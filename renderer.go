package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
	"golang.org/x/net/html"
)

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

	// Set the content to the text view and store original content
	originalText := result.String()
	b.originalContent = originalText
	b.textView.SetText(originalText)
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

	// Set the content to the text view and store original content
	originalText := result.String()
	b.originalContent = originalText
	b.textView.SetText(originalText)
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