package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/fatih/color"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"io/ioutil"
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
	images          []Image // Store images found on the page
	currentLinkIndex int   // Index of currently highlighted link
	forceUA         string
	loadingView     *tview.TextView
	isLoading       bool
	loadingStop     chan struct{} // Channel to signal loading animation to stop
	searchMatches   []SearchMatch // Store search matches for navigation
	returningFromSearchResult bool // Flag to track if returning from a selected search result
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
			// For now, Enter doesn't follow links directly in content
			// Users should use the 'l' key to see the modal list of links
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
			case 'j': // Scroll down OR navigate to next link if modal is open
				// For scrolling content, just scroll down by 10 lines
				currentRow, _ := b.textView.GetScrollOffset()
				b.textView.ScrollTo(currentRow + 10, 0)
				return nil
			case 'k': // Scroll up OR navigate to previous link if modal is open
				// For scrolling content, just scroll up by 10 lines (with minimum of 0)
				currentRow, _ := b.textView.GetScrollOffset()
				newRow := currentRow - 10
				if newRow < 0 {
					newRow = 0
				}
				b.textView.ScrollTo(newRow, 0)
				return nil
			case 'l': // Show list of all links in a modal
				if len(b.links) > 0 {
					b.showLinksModal()
				} else if len(b.images) > 0 {
					// If no links but images exist, show images modal
					b.showImagesModal()
				}
				return nil
			case 'i': // Show list of all images in a modal
				if len(b.images) > 0 {
					b.showImagesModal()
				}
				return nil
			case 'J': // Alternative: just scroll down regardless of links
				currentRow, _ := b.textView.GetScrollOffset()
				b.textView.ScrollTo(currentRow + 10, 0)
				return nil
			case 'K': // Alternative: just scroll up regardless of links
				currentRow, _ := b.textView.GetScrollOffset()
				newRow := currentRow - 10
				if newRow < 0 {
					newRow = 0
				}
				b.textView.ScrollTo(newRow, 0)
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
	// Capture tab to switch back to content view and enhance text input handling
	b.urlInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			b.app.SetFocus(b.textView)
			return nil // Consume the event
		}
		// Handle 'p' key to paste from clipboard
		if event.Key() == tcell.KeyRune && event.Rune() == 'p' {
			// Get text from clipboard
			clipText, err := clipboard.ReadAll()
			if err == nil {
				// Set the clipboard content to the input field
				b.urlInput.SetText(clipText)
			}
			return nil // Consume the event
		}
		return event
	})

	// Layout is set in the Run function
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

// showHelp displays help and usage information
func (b *Browser) showHelp() {
	helpText := `Terminal Browser - Help & Usage

Navigation:
  j     - Scroll down content
  k     - Scroll up content
  l     - Show modal list of all links on page (or images if no links)
  i     - Show modal list of all images on page
  Enter - Confirm selection in modal
  b     - Go back in history
  f     - Go forward in history

Link Handling:
  - Links marked with [IMAGE] for detected images, [IMAGE*] for real extensions
  - Press Enter on any link to navigate to that page
  - Image links preview directly in terminal when selected

Image Handling:
  - Supports formats: JPG, PNG, GIF, BMP, WebP, SVG, ICO, TIFF
  - Press 'i' to view all images on the current page
  - Each image shows alt text, title, URL, and file extension
  - Automatic size checking (max 5MB) to prevent large downloads
  - Images render directly in terminal using block characters
  - Press Enter on any image to preview it in terminal

Search:
  /     - Real-time search with match highlighting

Interface:
  Tab   - Switch between content view and URL input box
  ?     - Show this help information
  q     - Quit browser
  Ctrl+C - Quit browser

URL Input:
  - Type URL in the bottom input box and press Enter
  - Automatically adds 'https://' if no protocol specified
  - Press Tab to return to content view
  - For long URLs: use ← and → arrow keys to navigate within the input field
  - The input field shows a portion of long URLs; use arrow keys to see more

Clipboard:
  - Press 'p' when in URL input field to paste URL from clipboard

Loading Indicators:
  - Animated "Loading..." indicator appears when fetching pages
  - Shows progress during page loading
  - Automatically disappears when content loads

Security Features:
  - Blocks dangerous URL schemes (javascript:, data:, etc.)
  - Prevents access to local/internal addresses
  - Limits redirect chains to prevent loops
  - Sanitizes input to prevent formatting code injection
  - Size protection limits image downloads to 5MB

Content Processing:
  - Extracts main content and removes ads/navigation
  - Preserves basic formatting and structure
  - Identifies and presents all visible links in modal list
  - Supports multiple character encodings (UTF-8, Latin-1, etc.)

Accessibility Features:
  - Full keyboard navigation
  - Clear visual indicators
  - High contrast highlighting
  - Readable text formatting
  - Responsive controls

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

// hasRealImageExtension checks if a URL has a real image file extension
func (b *Browser) hasRealImageExtension(url string) bool {
	// Check file extension first
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff", ".tif"}

	// Convert URL to lower case for comparison
	lowerURL := strings.ToLower(url)

	for _, ext := range imageExtensions {
		if strings.HasSuffix(lowerURL, ext) {
			return true
		}
	}

	return false
}

// isImageURL checks if a URL points to an image based on extension or content type
func (b *Browser) isImageURL(url string) bool {
	// First check if it has a real image extension
	if b.hasRealImageExtension(url) {
		return true
	}

	// If no extension found in URL, try to check the content type by making a HEAD request
	resp, err := http.Head(url)
	if err != nil {
		// If we can't make the request, fall back to the extension check
		return false
	}
	defer resp.Body.Close()

	// Check the content type header
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "image/") {
		return true
	}

	return false
}

// showImagePreview shows a modal with an actual image preview in terminal
func (b *Browser) showImagePreview(imageURL string) {
	// Create a TextView to show image info
	imageInfo := tview.NewTextView()
	imageInfo.SetTextColor(tcell.ColorWhite)
	imageInfo.SetBackgroundColor(tcell.ColorNavy)
	imageInfo.SetDynamicColors(true)
	imageInfo.SetText(fmt.Sprintf("Loading image: %s", imageURL))
	imageInfo.SetBorder(true)
	imageInfo.SetTitle("Image Preview")

	// Create the image widget
	imgWidget := tview.NewImage()
	imgWidget.SetBorder(true)
	imgWidget.SetTitle("Image Preview - Press 'q' or ESC to close")

	// Create a Flex layout for the image preview
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(imageInfo, 3, 0, false).  // Show image URL at top
		AddItem(imgWidget, 0, 1, true)    // Show image in middle

	// Update image info to show loading
	go func() {
		// First check content length with a HEAD request to avoid downloading large files
		headResp, err := http.Head(imageURL)
		if err != nil {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error getting image info: %v", err))
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}
		defer headResp.Body.Close()

		// Check if the response is actually an image
		contentType := headResp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("URL does not point to an image (content type: %s)", contentType))
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}

		// Check content length (size) - max 5MB (5 * 1024 * 1024 bytes = 5,242,880 bytes)
		contentLength := headResp.Header.Get("Content-Length")
		if contentLength != "" {
			var size int64
			fmt.Sscanf(contentLength, "%d", &size)
			if size > 5*1024*1024 { // 5MB limit
				b.app.QueueUpdateDraw(func() {
					imageInfo.SetText(fmt.Sprintf("Image is too large (%.2f MB > 5 MB)", float64(size)/(1024*1024)))
					imgWidget.SetImage(nil) // Clear image
				})
				return
			}
		}

		// Load the image in a goroutine to prevent blocking
		resp, err := http.Get(imageURL)
		if err != nil {
			// Show error using app.QueueUpdateDraw
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error loading image: %v", err))
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}
		defer resp.Body.Close()

		// Check if the response is actually an image again (in case it changed)
		// However, we should be more permissive since some servers return wrong content-types
		contentType = resp.Header.Get("Content-Type")

		// Read the image data with size limit
		imgData, err := ioutil.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
		if err != nil {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error reading image: %v", err))
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}

		// Check if we reached the size limit
		if len(imgData) >= 5*1024*1024 {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText("Image is too large (exceeds 5 MB limit)")
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}

		// Decode the image - we'll try to decode it regardless of content-type header
		// since some servers return incorrect content-type headers
		img, format, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error decoding image (content-type: %s): %v", contentType, err))
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}

		// Update the image widget with the decoded image
		b.app.QueueUpdateDraw(func() {
			imgWidget.SetImage(img)
			imageInfo.SetText(fmt.Sprintf("Image loaded: %s (Format: %s, Size: %dx%d)", imageURL, format, img.Bounds().Dx(), img.Bounds().Dy()))
		})
	}()

	// Set up key handling for the image preview
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			// Return to the images modal
			b.showImagesModal()
			return nil
		}
		return event
	})

	// Set the flex layout as root
	b.app.SetRoot(flex, true)
}

// downloadImage downloads an image from URL to a temporary location
func (b *Browser) downloadImage(imageURL string) error {
	// Create an HTTP client
	client := &http.Client{}

	// Make the GET request
	resp, err := client.Get(imageURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check if the response is an image
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("URL does not point to an image (content type: %s)", contentType)
	}

	// For now, just verify we can access the image - in a real implementation
	// you might save it to a temp file or the user's system
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// showImageErrorModal shows an error modal for image operations
func (b *Browser) showImageErrorModal(errorMessage string) {
	errorModal := tview.NewModal()
	errorModal.SetBorder(true)
	errorModal.SetTitle("Image Error")
	errorModal.SetText(errorMessage)
	errorModal.AddButtons([]string{"OK"})

	errorModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		// Return to the links modal after showing error
		b.showLinksModal()
	})

	// Set up key handling for the modal
	errorModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			// Return to links list
			b.showLinksModal()
			return nil
		}
		return event
	})

	b.app.SetRoot(errorModal, true)
}

// showImageSuccessModal shows a success modal for image operations
func (b *Browser) showImageSuccessModal(successMessage string) {
	successModal := tview.NewModal()
	successModal.SetBorder(true)
	successModal.SetTitle("Success")
	successModal.SetText(successMessage)
	successModal.AddButtons([]string{"OK"})

	successModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		// Return to the links modal after showing success
		b.showLinksModal()
	})

	// Set up key handling for the modal
	successModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			// Return to links list
			b.showLinksModal()
			return nil
		}
		return event
	})

	b.app.SetRoot(successModal, true)
}

// showImagesModal displays a modal with a list of all images on the page
func (b *Browser) showImagesModal() {
	if len(b.images) == 0 {
		return
	}

	// Create a new list for images
	imageList := tview.NewList()
	imageList.SetBorder(true)
	imageList.SetTitle("Images on this page")
	imageList.ShowSecondaryText(true)

	// Add each image to the list
	for i, img := range b.images {
		// Create title for the image
		imgTitle := img.Alt
		if imgTitle == "" {
			// If no alt text, use a generic description
			imgTitle = fmt.Sprintf("Image %d", i+1)
		}

		// Truncate long alt text to fit in the list
		if len(imgTitle) > 50 {
			imgTitle = imgTitle[:50] + "..."
		}

		// Extract the file extension from the URL
		ext := "unknown"
		lastDot := strings.LastIndex(img.URL, ".")
		if lastDot != -1 && lastDot < len(img.URL)-1 {
			ext = strings.ToLower(img.URL[lastDot+1:])
			// Handle query parameters that might follow the extension
			if queryIndex := strings.Index(ext, "?"); queryIndex != -1 {
				ext = ext[:queryIndex]
			}
		}

		// Format the image URL to show in secondary text with file extension, truncating long URLs
		urlToShow := fmt.Sprintf("%s [%s]", img.URL, ext)
		if len(urlToShow) > 70 {
			urlToShow = urlToShow[:70] + "..."
		}

		// Add the item with primary text as image title and secondary as URL with extension
		imageList.AddItem(imgTitle, urlToShow, 0, func(index int) func() {
			return func() {
				// Show image preview
				b.showImagePreview(b.images[index].URL)
			}
		}(i))
	}

	// Add a close option
	imageList.AddItem("Close", "Close the images list", 'c', func() {
		// Close the modal by returning to main view
		flex := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(b.textView, 0, 1, false).
			AddItem(b.urlInput, 3, 0, false)
		b.app.SetRoot(flex, true)
		b.app.SetFocus(b.textView)
	})

	// Add a link list option to return to the links modal
	imageList.AddItem("View Links", "Return to links list", 'l', func() {
		b.showLinksModal()
	})

	// Set up key handling for the modal
	imageList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			// Close the modal by returning to main view
			flex := tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(b.textView, 0, 1, false).
				AddItem(b.urlInput, 3, 0, false)
			b.app.SetRoot(flex, true)
			b.app.SetFocus(b.textView)
			return nil
		}
		return event
	})

	// Set the list as root
	b.app.SetRoot(imageList, true)
}

// showLinksModal displays a modal with a list of all links on the page
func (b *Browser) showLinksModal() {
	if len(b.links) == 0 {
		return
	}

	// Create a new list for links
	linkList := tview.NewList()
	linkList.SetBorder(true)
	linkList.SetTitle("Links on this page")
	linkList.ShowSecondaryText(true)

	// Add each link to the list
	for i, link := range b.links {
		linkText := link.Text
		// Truncate long link text to fit in the list
		if len(linkText) > 50 {
			linkText = linkText[:50] + "..."
		}

		// Format the URL to show just the domain and path, truncating long paths
		urlToShow := link.URL
		if len(urlToShow) > 70 {
			urlToShow = urlToShow[:70] + "..."
		}

		// Check if the link is an image
		isImage := b.isImageURL(link.URL)
		hasRealExt := b.hasRealImageExtension(link.URL)
		if isImage {
			if hasRealExt {
				linkText += " [IMAGE*]" // Indicate that this is an image with real extension
			} else {
				linkText += " [IMAGE]" // Indicate that this is an image (detected by content type)
			}
		}

		// Add the item with primary text as link text and secondary as URL
		linkList.AddItem(linkText, urlToShow, 0, func(index int, isImg bool, hasExt bool) func() {
			return func() {
				if isImg {
					// Show image preview instead of navigating
					b.showImagePreview(b.links[index].URL)
				} else {
					// Navigate to the selected link
					b.NavigateTo(b.links[index].URL)
					// Close the modal by returning to main view
					flex := tview.NewFlex().
						SetDirection(tview.FlexRow).
						AddItem(b.textView, 0, 1, false).
						AddItem(b.urlInput, 3, 0, false)
					b.app.SetRoot(flex, true)
					b.app.SetFocus(b.textView)
				}
			}
		}(i, isImage, hasRealExt))
	}

	// Add a close option
	linkList.AddItem("Close", "Close the links list", 'c', func() {
		// Close the modal by returning to main view
		flex := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(b.textView, 0, 1, false).
			AddItem(b.urlInput, 3, 0, false)
		b.app.SetRoot(flex, true)
		b.app.SetFocus(b.textView)
	})

	// Add an images option if there are images on the page
	if len(b.images) > 0 {
		linkList.AddItem("Show Images", fmt.Sprintf("View all %d images on this page", len(b.images)), 'i', func() {
			b.showImagesModal()
		})
	}

	// Add a go back option if there's history
	if b.historyIndex > 0 {
		linkList.AddItem("Go Back", "Return to previous page", 'b', func() {
			// Go back in history and return to main view
			b.GoBack()
			// The GoBack function will handle updating the UI
		})
	}

	// Set up key handling for the modal
	linkList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			// Close the modal by returning to main view
			flex := tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(b.textView, 0, 1, false).
				AddItem(b.urlInput, 3, 0, false)
			b.app.SetRoot(flex, true)
			b.app.SetFocus(b.textView)
			return nil
		}
		return event
	})

	// Set the list as root
	b.app.SetRoot(linkList, true)
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