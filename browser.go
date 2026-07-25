package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/fatih/color"
	"github.com/rivo/tview"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"os/signal"
	"syscall"
)

// NewBrowser creates a new browser instance
func NewBrowser() *Browser {
	// Initialize the configuration system
	if err := InitializeConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "config init warning: %v (continuing with defaults)\n", err)
	}

	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load warning: %v (using defaults)\n", err)
		config = GetDefaultConfig()
	}

	// Create an HTTP client with the configuration
	client := NewHTTPClient(&config)

	browser := &Browser{
		app:                       tview.NewApplication(),
		history:                   make([]string, 0),
		historyIndex:              -1,
		client:                    client,
		forceUA:                   config.UserAgent,
		loadingStop:               make(chan struct{}),
		returningFromSearchResult: false,
		config:                    &config,
	}

	// Handle proxy configuration - prioritize config file over environment variable
	if config.Proxy != "" {
		if proxy, err := url.Parse(config.Proxy); err == nil {
			browser.client.SetProxy(proxy)
			browser.proxy = config.Proxy
		}
	} else if proxyURL := os.Getenv("PROXY"); proxyURL != "" {
		if proxy, err := url.Parse(proxyURL); err == nil {
			browser.client.SetProxy(proxy)
			browser.proxy = proxyURL
		}
	}

	return browser
}

// ColorToTviewFormat converts a color name to tview compatible format
func ColorToTviewFormat(colorName string) string {
	// Map common fatih/color names to tview format
	switch colorName {
	case "yellow":
		return "yellow"
	case "red":
		return "red"
	case "green":
		return "green"
	case "blue":
		return "blue"
	case "magenta":
		return "magenta"
	case "cyan":
		return "cyan"
	case "white":
		return "white"
	case "black":
		return "black"
	case "bold":
		return "::b"
	case "underline":
		return "::u"
	case "reverse":
		return "::r"
	default:
		return "yellow" // default highlight color
	}
}

// ApplyTviewColor applies color formatting to text for use in tview
func ApplyTviewColor(text, colorName string) string {
	colorCode := ColorToTviewFormat(colorName)
	return fmt.Sprintf("[%s]%s[-]", colorCode, text)
}

// ApplyTviewStyle applies multi-attribute formatting to text for use in tview
func ApplyTviewStyle(text string, fgColor, bgColor, attrs string) string {
	var format string
	if fgColor != "" {
		format += fgColor
	}
	if bgColor != "" {
		format += ":" + bgColor
	}
	if attrs != "" {
		format += ":" + attrs
	}
	return fmt.Sprintf("[%s]%s[-]", format, text)
}

// GetColorFunc returns a fatih/color function for terminal output (not for tview but for other uses)
func GetColorFunc(colorName string) func(a ...interface{}) string {
	col := color.New()

	switch colorName {
	case "yellow":
		col.Add(color.FgYellow)
	case "red":
		col.Add(color.FgRed)
	case "green":
		col.Add(color.FgGreen)
	case "blue":
		col.Add(color.FgBlue)
	case "magenta":
		col.Add(color.FgMagenta)
	case "cyan":
		col.Add(color.FgCyan)
	case "white":
		col.Add(color.FgWhite)
	case "bold":
		col.Add(color.Bold)
	case "underline":
		col.Add(color.Underline)
	default:
		col.Add(color.FgYellow) // default highlight color
	}

	return col.SprintFunc()
}

// GetCookiesForDomain returns all cookies for a specific domain
func (b *Browser) GetCookiesForDomain(domain string) []*Cookie {
	var cookies []*Cookie
	for _, cookie := range b.client.cookies {
		if cookie.Domain == domain {
			cookies = append(cookies, cookie)
		}
	}
	return cookies
}

// GetAllCookies returns all stored cookies
func (b *Browser) GetAllCookies() []*Cookie {
	var cookies []*Cookie
	for _, cookie := range b.client.cookies {
		cookies = append(cookies, cookie)
	}
	return cookies
}

// ClearCookies removes all cookies
func (b *Browser) ClearCookies() {
	b.client.cookies = make(map[string]*Cookie)
	// Also clear the persistent storage
	if b.config != nil {
		// Clear the cookies subdirectory
		configDir := GetConfigDir()
		cookiesDir := filepath.Join(configDir, "cookies")
		if _, err := os.Stat(cookiesDir); err == nil {
			// Remove all files in the cookies directory
			files, _ := os.ReadDir(cookiesDir)
			for _, file := range files {
				if !file.IsDir() {
					os.Remove(filepath.Join(cookiesDir, file.Name()))
				}
			}
		}
	} else {
		// Fallback to old method
		cookieFile := "t-browser-cookies.json"
		if b.config != nil && b.config.CookieFile != "" {
			cookieFile = b.config.CookieFile
		}
		os.Remove(cookieFile)
	}
}

// ClearCookiesForDomain removes cookies for a specific domain
func (b *Browser) ClearCookiesForDomain(domain string) {
	for key, cookie := range b.client.cookies {
		if cookie.Domain == domain {
			delete(b.client.cookies, key)
		}
	}
	// Save the updated cookies
	b.client.saveCookiesToFile()
}

// Run starts the browser application
func (b *Browser) Run() error {
	// Load previous session if available
	if b.config != nil {
		// Use the config directory to find the latest session file
		configDir := GetConfigDir()
		sessionFile := GetLatestSessionFile(configDir)
		if sessionFile != "" {
			b.LoadSession(sessionFile)
		}
	} else {
		// Fallback to old method
		sessionFile := "t-browser-session.json"
		if b.config != nil && b.config.SessionFile != "" {
			sessionFile = b.config.SessionFile
		}
		b.LoadSession(sessionFile)
	}

	// Apply the selected theme based on config BEFORE creating UI
	b.ApplyTheme()

	// Create UI components
	b.createUI()

	// Set initial URL if provided as argument and no current URL is set from session
	if b.currentURL == "" {
		if len(os.Args) > 1 {
			initialURL := os.Args[1]
			if !strings.HasPrefix(initialURL, "http://") && !strings.HasPrefix(initialURL, "https://") {
				initialURL = "https://" + initialURL
			}
			b.NavigateTo(initialURL)
		} else {
			b.NavigateTo("https://example.com")
		}
	} else {
		// Navigate to the saved current URL from the session
		b.NavigateTo(b.currentURL)
	}

	// Create a flex layout to hold both content and input
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.textView, 0, 1, false). // Main content area - takes remaining space
		AddItem(b.urlInput, 3, 0, false)  // URL input at the bottom - fixed height of 3

	// Set up graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		b.app.Stop()
	}()
	defer signal.Stop(sigCh)

	// Start the application with the flex layout and ensure content view has focus
	b.app.SetRoot(flex, true)
	b.app.SetFocus(b.textView)
	if err := b.app.EnableMouse(true).Run(); err != nil {
		return err
	}

	// Save cookies when the app exits
	b.client.saveCookiesToFile()

	// Save the session state
	if b.config != nil && b.config.SessionAutoSave {
		// Use the config directory to get a new timestamped session file
		configDir := GetConfigDir()
		sessionFile := GetSessionFilePath(configDir)
		b.SaveSession(sessionFile)
	}
	// For backward compatibility, if config is nil, use the old method
	if b.config == nil {
		b.SaveSession("t-browser-session.json")
	}

	return nil
}


