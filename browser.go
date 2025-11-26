package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Link represents a hyperlink on the page
type Link struct {
	URL  string
	Text string
	Position int // Position in content for navigation
}

// Browser represents the terminal browser instance
type Browser struct {
	app             *tview.Application
	textView        *tview.TextView
	urlInput        *tview.InputField
	history         []string
	historyIndex    int
	client          *HTTPClient
	cookies         map[string]*Cookie
	proxy           string
	currentURL      string
	searchTerm      string
	originalContent string // Store original content for search
	links           []Link  // Store links found on the page
	currentLinkIndex int   // Index of currently highlighted link
	forceUA         string
	loadingView     *tview.TextView
	isLoading       bool
	loadingStop     chan struct{} // Channel to signal loading animation to stop
}

// NewBrowser creates a new browser instance
func NewBrowser() *Browser {
	browser := &Browser{
		app:          tview.NewApplication(),
		history:      make([]string, 0),
		historyIndex: -1,
		client:       NewHTTPClient(),
		cookies:      make(map[string]*Cookie),
		forceUA:      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		loadingStop:  make(chan struct{}),
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

// Run starts the browser application
func (b *Browser) Run() error {
	// Create UI components
	b.createUI()

	// Set initial URL if provided as argument
	if len(os.Args) > 1 {
		initialURL := os.Args[1]
		if !strings.HasPrefix(initialURL, "http://") && !strings.HasPrefix(initialURL, "https://") {
			initialURL = "https://" + initialURL
		}
		b.NavigateTo(initialURL)
	} else {
		b.NavigateTo("https://example.com")
	}

	// Create a flex layout to hold both content and input
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.textView, 0, 1, false).  // Main content area - takes remaining space
		AddItem(b.urlInput, 3, 0, false)   // URL input at the bottom - fixed height of 3

	// Start the application with the flex layout and ensure content view has focus
	b.app.SetRoot(flex, true)
	b.app.SetFocus(b.textView)
	if err := b.app.EnableMouse(true).Run(); err != nil {
		return err
	}
	return nil
}

// createUI creates the terminal UI components
func (b *Browser) createUI() {
	// Text view to display web content
	b.textView = tview.NewTextView()
	b.textView.SetDynamicColors(true)
	b.textView.SetRegions(true)
	b.textView.SetWordWrap(true)
	b.textView.SetScrollable(true)
	b.textView.SetBorder(true)
	b.textView.SetTitle("Terminal Browser - Press Ctrl+C to quit, / for search")

	// Handle key events
	b.textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			b.app.Stop()
			return nil
		case tcell.KeyEnter:
			// Follow the currently highlighted link
			if b.currentLinkIndex >= 0 && b.currentLinkIndex < len(b.links) {
				b.NavigateTo(b.links[b.currentLinkIndex].URL)
				b.currentLinkIndex = -1 // Reset highlight
				b.updateTitleBar(-1) // Clear current link from title
				b.renderPageWithHighlightedLink() // Re-render without highlight
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				b.app.Stop()
				return nil
			case 'b': // Back button
				b.GoBack()
				return nil
			case 'f': // Forward button
				b.GoForward()
				return nil
			case '/': // Search
				b.startSearch()
				return nil
			case 'j': // Move to next link
				b.selectNextLink()
				b.updateTitleBar(b.currentLinkIndex) // Update title bar with current link
				return nil
			case 'k': // Move to previous link
				b.selectPreviousLink()
				b.updateTitleBar(b.currentLinkIndex) // Update title bar with current link
				return nil
			case '?': // Show help/usage information
				b.showHelp()
				return nil
			case '\t': // Tab key to switch to URL input
				b.app.SetFocus(b.urlInput)
				return nil
			}
		}

		// Also handle tcell.KeyTAB for consistency
		if event.Key() == tcell.KeyTAB {
			b.app.SetFocus(b.urlInput)
			return nil
		}

		return event
	})

	// URL input field
	b.urlInput = tview.NewInputField()
	b.urlInput.SetBorder(true)
	b.urlInput.SetTitle("Enter URL")
	b.urlInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			url := b.urlInput.GetText()
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				url = "https://" + url
			}
			b.NavigateTo(url)
			b.app.SetFocus(b.textView)
		} else if key == tcell.KeyEscape {
			b.app.SetFocus(b.textView)
		}
	})
	// Capture tab to switch back to content view
	b.urlInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			b.app.SetFocus(b.textView)
			return nil // Consume the event
		}
		return event
	})

	// Layout is set in the Run function
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

// selectNextLink moves to the next link in the document
func (b *Browser) selectNextLink() {
	if len(b.links) == 0 {
		return
	}

	b.currentLinkIndex++
	if b.currentLinkIndex >= len(b.links) {
		b.currentLinkIndex = 0 // Wrap around to first link
	}

	b.renderPageWithHighlightedLink()
}

// selectPreviousLink moves to the previous link in the document
func (b *Browser) selectPreviousLink() {
	if len(b.links) == 0 {
		return
	}

	b.currentLinkIndex--
	if b.currentLinkIndex < 0 {
		b.currentLinkIndex = len(b.links) - 1 // Wrap around to last link
	}

	b.renderPageWithHighlightedLink()
}

// renderPageWithHighlightedLink renders the page with the currently selected link highlighted
func (b *Browser) renderPageWithHighlightedLink() {
	originalText := b.originalContent

	// Create a copy of the original text with only the link number highlighted
	var displayText string

	if b.currentLinkIndex >= 0 && b.currentLinkIndex < len(b.links) && len(b.links) > 0 {
		currentLinkNumber := b.currentLinkIndex + 1
		linkText := b.links[b.currentLinkIndex].Text

		// Create the target string to search for: "link text [number]"
		targetStr := fmt.Sprintf("%s [%d]", linkText, currentLinkNumber)

		// Also handle the case where it might be formatted differently
		// Replace only the first occurrence to highlight the current link
		displayText = strings.Replace(originalText, targetStr,
			fmt.Sprintf("%s [blue][%d][::-]", linkText, currentLinkNumber), 1)

		// If the above replacement didn't work (meaning the format was different),
		// try a more general replacement for just the number
		if displayText == originalText {
			// If no replacement happened, try to just highlight the number [n] wherever it appears
			linkNumberStr := fmt.Sprintf("[%d]", currentLinkNumber)
			displayText = strings.Replace(originalText, linkNumberStr,
				fmt.Sprintf("[blue]%s[::-]", linkNumberStr), 1)
		}
	} else {
		displayText = originalText
	}

	b.textView.SetText(displayText)

	// Update the title bar with current link info
	b.updateTitleBar(b.currentLinkIndex)
}

// showHelp displays help and usage information
func (b *Browser) showHelp() {
	helpText := `Terminal Browser - Help & Usage

Navigation:
  j     - Move to next link
  k     - Move to previous link
  Enter - Follow current highlighted link
  b     - Go back in history
  f     - Go forward in history

Search:
  /     - Real-time search with match highlighting

Other:
  ?     - Show this help information
  q     - Quit browser
  Ctrl+C - Quit browser

Link Navigation:
  - Links appear in content as "text [n]"
  - Current link number is highlighted in blue
  - URL of current link shown in title bar

Accessibility Features:
  - Keyboard navigation only
  - Clear visual indicators
  - High contrast highlighting
  - Readable text formatting

Press any key to close this help.`

	// Create a modal help view
	helpView := tview.NewTextView()
	helpView.SetTextColor(tcell.ColorWhite)
	helpView.SetBackgroundColor(tcell.ColorNavy)
	helpView.SetDynamicColors(true)
	helpView.SetRegions(false)
	helpView.SetText(helpText)
	helpView.SetDoneFunc(func(key tcell.Key) {
		// Close help when any key is pressed
		// Restore the proper flex layout with URL input
		flex := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(b.textView, 0, 1, false).  // Main content area - takes remaining space
			AddItem(b.urlInput, 3, 0, false)   // URL input at the bottom - fixed height of 3

		b.app.SetRoot(flex, true)
		b.app.SetFocus(b.textView)  // Ensure content view has focus after help
	})

	// Set the help view as root
	b.app.SetRoot(helpView, true)
}

// isBlockElement checks if the tag is a block-level element
func (b *Browser) isBlockElement(tag string) bool {
	blockElements := map[string]bool{
		"address": true, "article": true, "aside": true, "blockquote": true,
		"details": true, "dialog": true, "dd": true, "div": true,
		"dl": true, "dt": true, "fieldset": true, "figcaption": true,
		"figure": true, "footer": true, "form": true, "h1": true,
		"h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
		"header": true, "hgroup": true, "hr": true, "li": true,
		"main": true, "nav": true, "ol": true, "p": true,
		"pre": true, "section": true, "table": true, "ul": true,
		"tr": true, "td": true, "th": true, "thead": true,
		"tbody": true, "tfoot": true,
	}
	return blockElements[tag]
}

// getAttribute gets an attribute value from an HTML node
func (b *Browser) getAttribute(node *html.Node, attrName string) (string, bool) {
	for _, attr := range node.Attr {
		if attr.Key == attrName {
			return attr.Val, true
		}
	}
	return "", false
}

// isParent checks if any ancestor has the specified tag
func (b *Browser) isParent(node *html.Node, tag string) bool {
	parent := node.Parent
	for parent != nil {
		if parent.DataAtom.String() == tag {
			return true
		}
		parent = parent.Parent
	}
	return false
}

// getListItemIndex calculates the index of a list item in an ordered list
func (b *Browser) getListItemIndex(node *html.Node) int {
	index := 1
	current := node.PrevSibling
	
	for current != nil {
		if current.Type == html.ElementNode && current.DataAtom == atom.Li {
			index++
		}
		current = current.PrevSibling
	}
	
	return index
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
		AddItem(b.textView, 0, 1, false).  // Main content area - takes remaining space
		AddItem(b.urlInput, 3, 0, false)   // URL input at the bottom - fixed height of 3

	b.app.SetRoot(flex, true)

	// Ensure focus goes back to the main content after loading
	b.app.SetFocus(b.textView)
}