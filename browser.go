package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/fatih/color"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// NewBrowser creates a new browser instance
func NewBrowser() *Browser {
	browser := &Browser{
		app:                       tview.NewApplication(),
		history:                   make([]string, 0),
		historyIndex:              -1,
		client:                    NewHTTPClient(),
		cookies:                   make(map[string]*Cookie),
		forceUA:                   "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		loadingStop:               make(chan struct{}),
		returningFromSearchResult: false,
	}

	// Handle proxy configuration
	if proxyURL := os.Getenv("PROXY"); proxyURL != "" {
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
	os.Remove(b.client.cookieFile)
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

// Session represents the state of a browser session
type Session struct {
	History           []string  `json:"history"`
	HistoryIndex      int       `json:"history_index"`
	CurrentURL        string    `json:"current_url"`
	SearchTerm        string    `json:"search_term"`
	ForceUA           string    `json:"force_ua"`
}

// SaveSession saves the current browser state to a file
func (b *Browser) SaveSession(filename string) error {
	session := &Session{
		History:      b.history,
		HistoryIndex: b.historyIndex,
		CurrentURL:   b.currentURL,
		SearchTerm:   b.searchTerm,
		ForceUA:      b.forceUA,
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0600)
}

// LoadSession loads a browser state from a file
func (b *Browser) LoadSession(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var session Session
	err = json.Unmarshal(data, &session)
	if err != nil {
		return err
	}

	b.history = session.History
	b.historyIndex = session.HistoryIndex
	b.currentURL = session.CurrentURL
	b.searchTerm = session.SearchTerm
	b.forceUA = session.ForceUA

	return nil
}

// Run starts the browser application
func (b *Browser) Run() error {
	// Load previous session if available
	b.LoadSession("t-browser-session.json")

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

	// Set up the before-draw function to handle graceful shutdown
	b.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		// This function runs before each draw - could be used for periodic cleanup
		// but we'll mainly use it to ensure cookies are saved before exit
		return false // Don't consume the draw, just run the cleanup
	})

	// Set up the after-draw function to handle cleanup if needed
	b.app.SetAfterDrawFunc(func(screen tcell.Screen) {
		// This ensures cookies are saved after each draw operation if needed
	})

	// Start the application with the flex layout and ensure content view has focus
	b.app.SetRoot(flex, true)
	b.app.SetFocus(b.textView)
	if err := b.app.EnableMouse(true).Run(); err != nil {
		return err
	}

	// Save cookies when the app exits
	b.client.saveCookiesToFile()

	// Save the session state
	b.SaveSession("t-browser-session.json")

	return nil
}

// shouldDisableWordWrap analyzes the content to decide if word wrap should be disabled
// Returns true if the content has very long lines that would benefit from disabling word wrap
func (b *Browser) shouldDisableWordWrap(content string) bool {
	lines := strings.Split(content, "\n")

	// Check if there are many long lines
	longLineCount := 0
	totalLines := len(lines)

	for _, line := range lines {
		// If line is significantly longer than common terminal width (80+ chars)
		if len(line) > 120 {
			longLineCount++
		}
	}

	// If more than 20% of lines are very long, disable word wrap for performance
	if totalLines > 0 && float64(longLineCount)/float64(totalLines) > 0.2 {
		return true
	}

	// Additional check: if there are any extremely long lines (>500 chars)
	for _, line := range lines {
		if len(line) > 500 {
			return true
		}
	}

	return false
}

// updateWordWrapBasedOnContent dynamically sets word wrap based on content characteristics
func (b *Browser) updateWordWrapBasedOnContent(content string) {
	shouldDisableWrap := b.shouldDisableWordWrap(content)

	// Only update if the setting has changed to avoid unnecessary UI updates
	needsWrap := !shouldDisableWrap
	b.textView.SetWordWrap(needsWrap)
}

// updateTitleBar updates the title bar with the current link's URL
func (b *Browser) updateTitleBar(linkIndex int) {
	baseTitle := "Terminal Browser - Press Ctrl+C to quit, / for search"

	if linkIndex >= 0 && linkIndex < len(b.links) {
		url := b.links[linkIndex].URL
		// Truncate long URLs to fit in title
		if len(url) > 50 {
			url = url[:50] + "..."
		}
		b.textView.SetTitle(fmt.Sprintf("%s | Current Link: %s", baseTitle, url))
	} else {
		b.textView.SetTitle(baseTitle)
	}
}

// NavigateTo navigates to the specified URL
func (b *Browser) NavigateTo(url string) {
	// Validate the URL before processing
	validatedURL, err := b.validateAndSanitizeURL(url)
	if err != nil {
		// Since we're potentially in the UI thread, ensure UI updates are queued
		b.app.QueueUpdateDraw(func() {
			b.displayError(fmt.Sprintf("Invalid URL: %v", err))
		})
		return
	}

	// Add to history
	if b.currentURL != "" && (len(b.history) == 0 || b.history[len(b.history)-1] != b.currentURL) {
		b.history = append(b.history, b.currentURL)
		b.historyIndex = len(b.history) - 1
	}

	// Prepare the fetch operation in a separate goroutine to not block UI
	go func() {
		// Show loading indicator from the UI thread
		b.app.QueueUpdateDraw(func() {
			b.showLoadingIndicator()
		})

		// Small delay to ensure the loading indicator appears before fetching
		time.Sleep(30 * time.Millisecond)

		// Fetch the page content
		content, err := b.client.FetchPage(validatedURL)

		// Hide loading indicator and handle the result from the UI thread
		b.app.QueueUpdateDraw(func() {
			b.hideLoadingIndicator()

			if err != nil {
				b.displayError(fmt.Sprintf("Error fetching page: %v", err))
				return
			}

			// Update current URL
			b.currentURL = url

			// Render the page content
			b.renderPage(content, url)

			// Clear the title bar when navigating to a new page
			// The links will be refreshed, so reset current link index
			b.currentLinkIndex = -1
			b.updateTitleBar(-1)
		})
	}()
}

// displayError shows an error message in the text view
func (b *Browser) displayError(message string) {
	b.textView.SetText(fmt.Sprintf("[red]Error: %s[-]", message))
}

// GoBack navigates back in history
func (b *Browser) GoBack() {
	if b.historyIndex > 0 {
		b.historyIndex--
		url := b.history[b.historyIndex]
		b.NavigateTo(url)
	}
}

// GoForward navigates forward in history
func (b *Browser) GoForward() {
	if b.historyIndex < len(b.history)-1 {
		b.historyIndex++
		url := b.history[b.historyIndex]
		b.NavigateTo(url)
	}
}

// showLoadingIndicator shows the animated loading indicator
func (b *Browser) showLoadingIndicator() {
	if b.isLoading {
		return // Already showing loading indicator
	}

	b.isLoading = true

	// Create the loading view
	b.loadingView = tview.NewTextView()
	b.loadingView.SetDynamicColors(true)
	b.loadingView.SetTextAlign(tview.AlignCenter)
	b.loadingView.SetBorder(true)
	b.loadingView.SetBackgroundColor(tcell.ColorBlue)
	b.loadingView.SetTextColor(tcell.ColorWhite)
	b.loadingView.SetTitle("Loading")

	// Set initial loading text
	b.loadingView.SetText("[yellow]Loading...[white]")

	// Replace the current view with loading indicator
	b.app.SetRoot(b.loadingView, true)

	// Force a draw to show the loading indicator immediately
	go func() {
		time.Sleep(10 * time.Millisecond) // Brief pause to allow UI setup
		b.app.Draw()
	}()

	// Start the animation in a separate goroutine
	go b.animateLoading()
}

// animateLoading updates the loading indicator with animation
func (b *Browser) animateLoading() {
	// Animation sequence: Loading, Loading., Loading.., Loading...
	phases := []string{"Loading", "Loading.", "Loading..", "Loading..."}
	currentPhase := 0

	for {
		select {
		case <-b.loadingStop:
			// Stop animation when loading is done
			return
		default:
			// Update the loading text with the current phase
			animationText := fmt.Sprintf("[yellow]%s[white]", phases[currentPhase])

			// Update the text in the main goroutine to prevent race conditions
			b.app.QueueUpdateDraw(func() {
				if b.loadingView != nil {
					b.loadingView.SetText(animationText)
				}
			})

			// Move to the next phase
			currentPhase = (currentPhase + 1) % len(phases)

			// Wait before updating again
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// showLoadingModal shows a temporary loading modal for modals when needed
func (b *Browser) showLoadingModal(title, message string) *tview.TextView {
	loadingView := tview.NewTextView()
	loadingView.SetDynamicColors(true)
	loadingView.SetTextAlign(tview.AlignCenter)
	loadingView.SetBorder(true)
	loadingView.SetBackgroundColor(tcell.ColorBlue)
	loadingView.SetTextColor(tcell.ColorWhite)
	loadingView.SetTitle(title)
	loadingView.SetText(message)

	b.app.SetRoot(loadingView, true)

	return loadingView
}

// hideLoadingIndicator hides the loading indicator and returns to the main view
func (b *Browser) hideLoadingIndicator() {
	if !b.isLoading {
		return
	}

	// Stop the animation
	close(b.loadingStop)

	// Create a new stop channel for future use
	b.loadingStop = make(chan struct{})

	// Reset loading flag
	b.isLoading = false

	// Return to the main view (textView)
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.textView, 0, 1, false). // Main content area - takes remaining space
		AddItem(b.urlInput, 3, 0, false)  // URL input at the bottom - fixed height of 3

	b.app.SetRoot(flex, true)

	// Ensure focus goes back to the main content after loading
	b.app.SetFocus(b.textView)
}

// validateAndSanitizeURL validates and sanitizes the input URL
func (b *Browser) validateAndSanitizeURL(inputURL string) (string, error) {
	// Basic validation to check for potentially malicious schemes
	if strings.HasPrefix(inputURL, "javascript:") || strings.HasPrefix(inputURL, "data:") ||
		strings.HasPrefix(inputURL, "vbscript:") || strings.HasPrefix(inputURL, "file:") {
		return "", fmt.Errorf("unsupported or dangerous URL scheme")
	}

	// Check if the URL is empty
	if strings.TrimSpace(inputURL) == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	// Check for excessive length to prevent potential buffer overflow
	if len(inputURL) > 2048 {
		return "", fmt.Errorf("URL is too long")
	}

	// Parse the URL to validate its structure
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %v", err)
	}

	// Validate the host to ensure it's not pointing to internal addresses
	host := parsedURL.Hostname()
	if host == "localhost" || strings.HasPrefix(host, "127.") ||
		strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") ||
		(strings.HasPrefix(host, "172.") && len(host) > 4 &&
			host[4] >= '1' && host[4] <= '3' && host[5] == '.') {
		// Allow these only if explicitly enabled, for security
		return "", fmt.Errorf("access to local/internal addresses not allowed")
	}

	// Check for suspicious patterns in the URL
	if strings.Contains(inputURL, "..") || strings.Contains(inputURL, "0x00") {
		return "", fmt.Errorf("URL contains suspicious patterns")
	}

	// Return the validated URL
	return inputURL, nil
}
