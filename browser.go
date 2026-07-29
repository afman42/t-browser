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
	"github.com/gdamore/tcell/v2"
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
		app:         tview.NewApplication(),
		client:      client,
		forceUA:     config.UserAgent,
		loadingStop: make(chan struct{}),
		config:      &config,
		activeTab:   0,
	}
	// Create the initial tab
	browser.tabs = []*Tab{newTab()}

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

// getHistoryCompletions returns URL completions from history
func (b *Browser) getHistoryCompletions(prefix string, limit int) []string {
	var matches []string
	seen := make(map[string]bool)

	// Search backwards through history for most recent matches
	for i := len(b.currentTab().history) - 1; i >= 0 && len(matches) < limit; i-- {
		url := b.currentTab().history[i]
		if strings.HasPrefix(url, prefix) && !seen[url] {
			matches = append(matches, url)
			seen[url] = true
		}
	}
	return matches
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

	// Create a flex layout to hold both content and input
	// Create status bar
	b.statusBar = tview.NewTextView()
	b.statusBar.SetDynamicColors(true)
	b.statusBar.SetTextAlign(tview.AlignLeft)
	b.statusBar.SetBorder(false)
	b.statusBar.SetBackgroundColor(tcell.ColorDefault)
	b.statusBar.SetTextColor(tcell.ColorWhite)

	// Create tab bar
	b.tabBar = tview.NewTextView()
	b.tabBar.SetDynamicColors(true)
	b.tabBar.SetTextAlign(tview.AlignLeft)
	b.tabBar.SetBorder(false)
	b.tabBar.SetBackgroundColor(tcell.ColorDarkCyan)
	b.tabBar.SetTextColor(tcell.ColorWhite)
	b.updateTabBar()

	flex := b.mainFlex()

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
	b.app.SetFocus(b.currentTab().textView)

	// Navigate to the initial URL AFTER the UI is fully set up so that
	// showLoadingIndicator can write to the status bar immediately.
	go func() {
		if b.currentTab().currentURL == "" {
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
			b.NavigateTo(b.currentTab().currentURL)
		}
	}()

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

func (b *Browser) mainFlex() *tview.Flex {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.tabBar, 1, 0, false).
		AddItem(b.currentTab().textView, 0, 1, false).
		AddItem(b.statusBar, 1, 0, false).
		AddItem(b.urlInput, 3, 0, false)
}

func (b *Browser) updateTabBar() {
	if b.tabBar == nil {
		return
	}
	var sb strings.Builder
	for i, tab := range b.tabs {
		label := tab.currentURL
		if label == "" {
			label = "New Tab"
		} else {
			// Shorten URL for display
			if len(label) > 30 {
				label = label[:30] + "..."
			}
		}
		if i == b.activeTab {
			sb.WriteString(fmt.Sprintf(" [::b][ %d: %s ][::-] ", i+1, label))
		} else {
			sb.WriteString(fmt.Sprintf(" [ %d: %s ] ", i+1, label))
		}
	}
	if len(b.tabs) == 0 {
		sb.WriteString(" No tabs")
	}
	b.tabBar.SetText(sb.String())
}

func (b *Browser) newTab() {
	tab := newTab()
	b.setupKeyBindings(tab.textView)
	b.tabs = append(b.tabs, tab)
	b.activeTab = len(b.tabs) - 1
	b.ApplyTheme()
	if b.app != nil {
		b.app.QueueUpdateDraw(func() {
			b.app.SetRoot(b.mainFlex(), true)
			b.app.SetFocus(b.urlInput)
			b.updateTabBar()
		})
	}
}

func (b *Browser) closeTab() {
	if len(b.tabs) <= 1 {
		if b.app != nil {
			b.app.Stop()
		}
		return
	}
	old := b.tabs[b.activeTab]
	if old.metaRefreshCancel != nil {
		old.metaRefreshCancel()
	}
	b.tabs = append(b.tabs[:b.activeTab], b.tabs[b.activeTab+1:]...)
	if b.activeTab >= len(b.tabs) {
		b.activeTab = len(b.tabs) - 1
	}
	b.ApplyTheme()
	if b.app != nil {
		b.app.QueueUpdateDraw(func() {
			b.app.SetRoot(b.mainFlex(), true)
			b.app.SetFocus(b.currentTab().textView)
			b.updateTabBar()
		})
	}
}

func (b *Browser) switchTab(index int) {
	if index < 0 || index >= len(b.tabs) {
		return
	}
	b.activeTab = index
	b.ApplyTheme()
	if b.app != nil {
		b.app.QueueUpdateDraw(func() {
			b.app.SetRoot(b.mainFlex(), true)
			b.app.SetFocus(b.currentTab().textView)
			b.updateStatusBar()
			b.updateTabBar()
		})
	}
}

func (b *Browser) nextTab() {
	b.switchTab((b.activeTab + 1) % len(b.tabs))
}

func (b *Browser) prevTab() {
	idx := b.activeTab - 1
	if idx < 0 {
		idx = len(b.tabs) - 1
	}
	b.switchTab(idx)
}

func (b *Browser) pinCurrentSite() {
	currentURL := b.currentTab().currentURL
	if currentURL == "" {
		return
	}
	fingerprint, err := PinCurrentSiteKey(currentURL)
	if err != nil {
		return
	}
	if b.config == nil {
		return
	}

	for _, existing := range b.config.PinnedKeys {
		if existing == fingerprint {
			return
		}
	}
	b.config.PinnedKeys = append(b.config.PinnedKeys, fingerprint)
	b.config.EnablePinning = true
	configDir := GetConfigDir()
	if err := b.config.WriteToFile(configDir); err != nil {
	}
}
