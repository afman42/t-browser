package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

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
	forceUA         string
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
	content, err := b.client.FetchPage(url)
	if err != nil {
		b.displayError(fmt.Sprintf("Error fetching page: %v", err))
		return
	}

	// Update current URL
	b.currentURL = url
	
	// Render the page content
	b.renderPage(content, url)
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