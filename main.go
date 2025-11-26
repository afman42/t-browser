package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// Browser represents the terminal browser instance
type Browser struct {
	app          *tview.Application
	textView     *tview.TextView
	urlInput     *tview.InputField
	history      []string
	historyIndex int
	client       *http.Client
	cookies      map[string]*http.Cookie
	proxy        string
	currentURL   string
	searchTerm   string
	forceUA      string
}

// NewBrowser creates a new browser instance
func NewBrowser() *Browser {
	// Create HTTP client with proper configuration
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	
	browser := &Browser{
		app:          tview.NewApplication(),
		history:      make([]string, 0),
		historyIndex: -1,
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		cookies: make(map[string]*http.Cookie),
		forceUA: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}

	// Handle proxy configuration
	if proxyURL := os.Getenv("PROXY"); proxyURL != "" {
		if proxy, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(proxy)
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

	// Start the application
	if err := b.app.SetRoot(b.textView, true).EnableMouse(true).Run(); err != nil {
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
			// Handle link selection
			if link := b.getCurrentLink(); link != "" {
				b.NavigateTo(link)
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
			}
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

	// Layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.textView, 0, 1, false).
		AddItem(b.urlInput, 3, 1, false)

	b.app.SetRoot(flex, true)
}

// NavigateTo navigates to the specified URL
func (b *Browser) NavigateTo(url string) {
	// Add to history
	if b.currentURL != "" && (len(b.history) == 0 || b.history[len(b.history)-1] != b.currentURL) {
		b.history = append(b.history, b.currentURL)
		b.historyIndex = len(b.history) - 1
	}

	// Fetch the page
	content, err := b.fetchPage(url)
	if err != nil {
		b.displayError(fmt.Sprintf("Error fetching page: %v", err))
		return
	}

	// Update current URL
	b.currentURL = url
	
	// Render the page content
	b.renderPage(content, url)
}

// fetchPage fetches the page content from the given URL
func (b *Browser) fetchPage(rawURL string) (string, error) {
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Create request
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}

	// Set headers
	req.Header.Set("User-Agent", b.forceUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// Add cookies if available
	for domain, cookie := range b.cookies {
		if strings.Contains(parsedURL.Host, domain) {
			req.AddCookie(cookie)
		}
	}

	// Execute request
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check if the content type is actually text/html
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") &&
	   !strings.Contains(strings.ToLower(contentType), "text/plain") &&
	   !strings.Contains(strings.ToLower(contentType), "application/xhtml+xml") {
		return "", fmt.Errorf("content type not supported: %s", contentType)
	}

	// Handle redirects manually to maintain cookies across redirects
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			// Create absolute URL if location is relative
			if strings.HasPrefix(location, "/") {
				location = fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, location)
			} else if !strings.HasPrefix(location, "http") {
				location = fmt.Sprintf("%s://%s/%s", parsedURL.Scheme, parsedURL.Host, location)
			}
			return b.fetchPage(location)
		}
	}

	// Store cookies from response
	for _, cookie := range resp.Cookies() {
		b.cookies[cookie.Domain] = cookie
	}

	// Extract charset from content type
	encodingName := "utf-8"
	if idx := strings.Index(contentType, "charset="); idx != -1 {
		encodingName = strings.TrimSpace(contentType[idx+8:])
		encodingName = strings.Trim(encodingName, "\"'")
	}

	// Check if body is compressed with gzip and decompress if needed
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read response body
	body, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	// If encoding is not UTF-8, try to convert it
	if strings.Contains(strings.ToLower(encodingName), "iso-8859") ||
	   strings.Contains(strings.ToLower(encodingName), "latin") {
		enc := charmap.ISO8859_1
		if strings.Contains(strings.ToLower(encodingName), "iso-8859-2") {
			enc = charmap.ISO8859_2
		} else if strings.Contains(strings.ToLower(encodingName), "iso-8859-15") {
			enc = charmap.ISO8859_15
		}
		decoder := enc.NewDecoder()
		body, err = io.ReadAll(transform.NewReader(bytes.NewReader(body), decoder))
		if err != nil {
			// If conversion fails, continue with original body
			// Error handling is already done, just use the original body
		}
	} else if strings.Contains(strings.ToLower(encodingName), "utf-16") {
		// Handle UTF-16 encoding
		decoder := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
		body, err = io.ReadAll(transform.NewReader(bytes.NewReader(body), decoder))
		if err != nil {
			// If conversion fails, continue with original body
		}
	}

	// Verify the body is valid text by checking for binary content
	// Check the first few bytes to see if they look like binary
	if len(body) > 0 {
		// Check for common binary file signatures
		if len(body) >= 4 {
			// Check for common binary signatures (magic numbers)
			if (body[0] == 0x89 && body[1] == 0x50 && body[2] == 0x4E && body[3] == 0x47) || // PNG
				(body[0] == 0xFF && body[1] == 0xD8 && body[2] == 0xFF) || // JPEG
				(body[0] == 0x47 && body[1] == 0x49 && body[2] == 0x46) || // GIF
				(body[0] == 0x50 && body[1] == 0x4B) || // ZIP/PDF
				(body[0] == 0x25 && body[1] == 0x50 && body[2] == 0x44 && body[3] == 0x46) { // PDF
				return "", fmt.Errorf("binary content detected, not text/html")
			}
		}
	}

	return string(body), nil
}

// renderPage renders the HTML content to plain text
func (b *Browser) renderPage(htmlContent, rawURL string) {
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// If URL parsing fails, fall back to the original method
		b.renderPageFallback(htmlContent)
		return
	}

	// Use go-readability to extract the main content
	article, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err != nil {
		// If readability fails, fall back to the original method
		b.renderPageFallback(htmlContent)
		return
	}

	// Format the extracted content nicely
	var result strings.Builder

	// Add title if available
	if article.Title != "" {
		result.WriteString(fmt.Sprintf("[::b]%s[::-]\n\n", article.Title))
	}

	// Add the extracted content
	result.WriteString(article.TextContent)

	// Add image if available
	if article.Image != "" {
		result.WriteString(fmt.Sprintf("\n\n[Image: %s]", article.Image))
	}

	// Set the content to the text view
	b.textView.SetText(result.String())
	b.textView.ScrollToBeginning()
}

// renderPageFallback renders HTML content using the original method when readability fails
func (b *Browser) renderPageFallback(htmlContent string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		b.displayError(fmt.Sprintf("Error parsing HTML: %v", err))
		return
	}

	var result strings.Builder
	tabs := 0

	// Process the document
	doc.Find("body").Contents().Each(func(i int, s *goquery.Selection) {
		node := s.Get(0)
		b.renderNode(node, &result, &tabs)
	})

	// Set the content to the text view
	b.textView.SetText(result.String())
	b.textView.ScrollToBeginning()
}

// renderNode renders an individual HTML node to text
func (b *Browser) renderNode(node *html.Node, result *strings.Builder, tabs *int) {
	switch node.Type {
	case html.TextNode:
		text := strings.TrimSpace(node.Data)
		if text != "" {
			// Add indentation
			for i := 0; i < *tabs; i++ {
				result.WriteString("  ")
			}
			// Escape special characters that might interfere with tview formatting
			text = strings.ReplaceAll(text, "[", "\\[")
			text = strings.ReplaceAll(text, "]", "\\]")
			text = strings.ReplaceAll(text, "*", "\\*")
			text = strings.ReplaceAll(text, "_", "\\_")
			text = strings.ReplaceAll(text, "`", "\\`")
			result.WriteString(text)
			result.WriteString("\n")
		}
	case html.ElementNode:
		tag := node.DataAtom.String()
		isBlockElement := b.isBlockElement(tag)
		
		if isBlockElement {
			result.WriteString("\n")
		}
		
		// Handle special tags
		switch tag {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			// Add header formatting
			for i := 0; i < *tabs; i++ {
				result.WriteString("  ")
			}
			result.WriteString("[::b]") // Bold
			*tabs += 1
		case "p":
			result.WriteString("\n")
			for i := 0; i < *tabs; i++ {
				result.WriteString("  ")
			}
		case "a":
			// Add link highlighting
			if _, exists := b.getAttribute(node, "href"); exists {
				// Add link with special formatting
				result.WriteString("[::u]") // Underline
			}
		case "ul", "ol":
			*tabs += 1
		case "li":
			result.WriteString("\n")
			for i := 0; i < *tabs-1; i++ {
				result.WriteString("  ")
			}
			if b.isParent(node, "ol") {
				// Handle ordered list
				index := b.getListItemIndex(node)
				result.WriteString(fmt.Sprintf("%d. ", index))
			} else {
				result.WriteString("• ")
			}
		case "br":
			result.WriteString("\n")
		case "div":
			if isBlockElement {
				result.WriteString("\n")
			}
		}

		// Process children
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			b.renderNode(child, result, tabs)
		}

		// Close tags
		switch tag {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			result.WriteString("[::-]")
			*tabs -= 1
		case "a":
			href, exists := b.getAttribute(node, "href")
			if exists {
				result.WriteString(fmt.Sprintf(" (%s)", href))
				result.WriteString("[::-]") // Reset formatting
			}
		case "ul", "ol":
			*tabs -= 1
		}

		if isBlockElement && tag != "li" {
			result.WriteString("\n")
		}
	}
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

// getCurrentLink gets the currently selected link in the text view
func (b *Browser) getCurrentLink() string {
	// This is a simplified implementation
	// In a real implementation, we'd need to track cursor position and find the corresponding link
	// For now, we'll return empty string as an indication that we need to improve this
	return ""
}

// startSearch starts the search functionality
func (b *Browser) startSearch() {
	// Create a modal search input
	var searchInput *tview.InputField
	searchInput = tview.NewInputField().
		SetLabel("Search: ").
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				term := searchInput.GetText()
				b.searchTerm = term
				b.performSearch(term)
				b.app.SetFocus(b.textView)
			} else if key == tcell.KeyEscape {
				b.app.SetFocus(b.textView)
			}
		})

	b.app.SetRoot(tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(searchInput, 1, 1, true).
			AddItem(nil, 0, 1, false), 0, 1, true).
		AddItem(nil, 0, 1, false), true)
}

// performSearch performs the text search in the current page
func (b *Browser) performSearch(term string) {
	if term == "" {
		return
	}
	
	// Get current text content
	text := b.textView.GetText(false)
	
	// Find occurrences and highlight them
	highlightedText := b.highlightSearchTerm(text, term)
	
	// Update the text view with highlighted content
	b.textView.SetText(highlightedText)
}

// highlightSearchTerm highlights search terms in the text
func (b *Browser) highlightSearchTerm(text, term string) string {
	// This is a simple implementation that highlights search terms
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(term))
	highlighted := re.ReplaceAllString(text, "[yellow]$0[::-]")
	return highlighted
}

func main() {
	browser := NewBrowser()
	
	if err := browser.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running browser: %v\n", err)
		os.Exit(1)
	}
}