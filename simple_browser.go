package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// SimpleTextBrowser is a version without terminal UI for testing content processing
type SimpleTextBrowser struct {
	client  *http.Client
	cookies map[string]*http.Cookie
	forceUA string
}

// NewSimpleTextBrowser creates a new simple browser instance
func NewSimpleTextBrowser() *SimpleTextBrowser {
	browser := &SimpleTextBrowser{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cookies: make(map[string]*http.Cookie),
		forceUA: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}
	return browser
}

// FetchAndRender fetches a page and returns rendered text content
func (b *SimpleTextBrowser) FetchAndRender(rawURL string) (string, error) {
	htmlContent, err := b.fetchPage(rawURL)
	if err != nil {
		return "", err
	}
	
	return b.renderHTMLToText(htmlContent), nil
}

// fetchPage fetches the page content from the given URL
func (b *SimpleTextBrowser) fetchPage(rawURL string) (string, error) {
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

	return string(body), nil
}

// renderHTMLToText converts HTML to plain text
func (b *SimpleTextBrowser) renderHTMLToText(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return fmt.Sprintf("Error parsing HTML: %v", err)
	}

	var result strings.Builder
	tabs := 0

	// Process the document
	doc.Find("body").Contents().Each(func(i int, s *goquery.Selection) {
		node := s.Get(0)
		b.renderNode(node, &result, &tabs)
	})

	return result.String()
}

// renderNode renders an individual HTML node to text
func (b *SimpleTextBrowser) renderNode(node *html.Node, result *strings.Builder, tabs *int) {
	switch node.Type {
	case html.TextNode:
		text := strings.TrimSpace(node.Data)
		if text != "" {
			// Add indentation
			for i := 0; i < *tabs; i++ {
				result.WriteString("  ")
			}
			// Escape special characters that might interfere with display
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
			result.WriteString("### ") // Simple header marking
			*tabs += 1
		case "p":
			result.WriteString("\n")
			for i := 0; i < *tabs; i++ {
				result.WriteString("  ")
			}
		case "a":
			// Add link highlighting
			if _, exists := b.getAttribute(node, "href"); exists {
				// Just process the content without special formatting
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
			*tabs -= 1
		case "a":
			if href, exists := b.getAttribute(node, "href"); exists {
				result.WriteString(fmt.Sprintf(" (%s)", href))
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
func (b *SimpleTextBrowser) isBlockElement(tag string) bool {
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
func (b *SimpleTextBrowser) getAttribute(node *html.Node, attrName string) (string, bool) {
	for _, attr := range node.Attr {
		if attr.Key == attrName {
			return attr.Val, true
		}
	}
	return "", false
}

// isParent checks if any ancestor has the specified tag
func (b *SimpleTextBrowser) isParent(node *html.Node, tag string) bool {
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
func (b *SimpleTextBrowser) getListItemIndex(node *html.Node) int {
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

