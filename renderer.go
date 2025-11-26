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

	// Parse HTML to extract links first
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		// If parsing fails, fall back to the original method
		b.renderPageFallback(htmlContent)
		return
	}

	// Extract all links from the document first
	var links []Link
	linkCounter := 0
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				// Resolve relative URLs
				if strings.HasPrefix(href, "/") {
					href = fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, href)
				} else if !strings.HasPrefix(href, "http") {
					// Handle relative URLs
					baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
					if strings.HasPrefix(href, "#") {
						href = baseURL + parsedURL.Path + href
					} else {
						href = baseURL + "/" + href
					}
				}
				links = append(links, Link{
					URL:      href,
					Text:     text,
					Position: linkCounter, // Position in order of appearance
				})
				linkCounter++
			}
		}
	})

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
		result.WriteString(fmt.Sprintf("[::b]%s[::-]\n", article.Title))
	}

	// Clean up the extracted content to remove excessive whitespace
	cleanedContent := cleanExcessiveWhitespace(article.TextContent)

	// Process content to embed link numbers directly in the text
	processedContent := b.embedLinkNumbers(cleanedContent, links)

	// Add the processed content
	result.WriteString(processedContent)

	// Store links for navigation
	if len(links) > 0 {
		// Add a separator before showing navigation info
		result.WriteString("\n\n[yellow]Use 'j'/'k' to navigate links[-]\n")
	}

	// Add image if available
	if article.Image != "" {
		result.WriteString(fmt.Sprintf("\n[Image: %s]", article.Image))
	}

	// Store the links in browser
	b.links = links
	b.currentLinkIndex = -1 // Start with no link selected

	// Set the content to the text view and store original content
	originalText := result.String()
	b.originalContent = originalText
	b.textView.SetText(originalText)
	b.textView.ScrollToBeginning()
}

// renderPageWithReadabilityContent renders content using only readability without link extraction
func (b *Browser) renderPageWithReadabilityContent(article readability.Article) {
	// Format the extracted content nicely
	var result strings.Builder

	// Add title if available
	if article.Title != "" {
		result.WriteString(fmt.Sprintf("[::b]%s[::-]\n", article.Title))
	}

	// Clean up the extracted content to remove excessive whitespace
	cleanedContent := cleanExcessiveWhitespace(article.TextContent)

	// Add the cleaned extracted content
	result.WriteString(cleanedContent)

	// Add image if available
	if article.Image != "" {
		result.WriteString(fmt.Sprintf("\n[Image: %s]", article.Image))
	}

	// Process content to embed link numbers directly in the text for fallback
	processedContent := b.originalContent
	if len(b.links) > 0 {
		processedContent = b.embedLinkNumbers(b.originalContent, b.links)
		// Add a separator before showing navigation info
		processedContent += "\n\n[yellow]Use 'j'/'k' to navigate links[-]\n"
	}

	// Set the content to the text view and store original content
	b.originalContent = processedContent
	b.textView.SetText(processedContent)
	b.textView.ScrollToBeginning()
}

// cleanExcessiveWhitespace removes excessive empty lines and normalizes spacing
func cleanExcessiveWhitespace(text string) string {
	// Split the text into lines
	lines := strings.Split(text, "\n")

	var cleanedLines []string
	previousWasEmpty := false

	for _, line := range lines {
		isEmpty := strings.TrimSpace(line) == ""

		// Skip consecutive empty lines
		if isEmpty && previousWasEmpty {
			continue
		}

		cleanedLines = append(cleanedLines, line)
		previousWasEmpty = isEmpty
	}

	// Join the lines back together
	result := strings.Join(cleanedLines, "\n")

	// Additional cleanup: remove multiple consecutive newlines in the content
	result = strings.ReplaceAll(result, "\n\n\n", "\n\n")

	return result
}

// embedLinkNumbers embeds link numbers directly in the content
func (b *Browser) embedLinkNumbers(content string, links []Link) string {
	processed := content

	// For each link, find its text in the content and add the link number
	// To avoid double numbering, process each link separately and mark them
	for i, link := range links {
		// Create a placeholder to temporarily mark this link's occurrences
		// We'll look for the original link text not followed by numbers in brackets
		// Use string scanning to process occurrences one by one
		processed = b.addLinkNumberSafely(processed, link.Text, i+1)
	}

	return processed
}

// addLinkNumberSafely adds a number to a link text without double numbering
func (b *Browser) addLinkNumberSafely(content, linkText string, linkNumber int) string {
	// Use a counter to only number the first N occurrences if we have N links
	// This prevents numbering other instances of the same text that aren't actual links
	result := content
	count := 0
	targetCount := 1 // For now, just replace the first occurrence we expect to be a link

	startIdx := 0
	for startIdx < len(result) && count < targetCount {
		pos := strings.Index(result[startIdx:], linkText)
		if pos == -1 {
			break
		}

		actualPos := startIdx + pos
		endPos := actualPos + len(linkText)

		// Check if this occurrence is already numbered
		alreadyNumbered := false
		if endPos < len(result) {
			remainder := result[endPos:]
			if len(remainder) >= 1 && remainder[0] == ' ' && len(remainder) > 1 {
				nextPart := remainder[1:]
				if len(nextPart) >= 3 && nextPart[0] == '[' {
					// Find the closing bracket
					closeBracket := strings.IndexRune(nextPart, ']')
					if closeBracket != -1 {
						insideBrackets := nextPart[1:closeBracket]
						// Check if it's a number
						isNumber := true
						for _, r := range insideBrackets {
							if r < '0' || r > '9' {
								isNumber = false
								break
							}
						}
						if isNumber {
							alreadyNumbered = true
						}
					}
				}
			}
		}

		if !alreadyNumbered {
			// Replace this occurrence
			before := result[:actualPos]
			after := result[endPos:]
			result = before + fmt.Sprintf("%s [%d]", linkText, linkNumber) + after
			count++
			startIdx = actualPos + len(fmt.Sprintf("%s [%d]", linkText, linkNumber))
		} else {
			startIdx = endPos
		}
	}

	return result
}

// renderPageFallback renders HTML content using the original method when readability fails
func (b *Browser) renderPageFallback(htmlContent string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		b.displayError(fmt.Sprintf("Error parsing HTML: %v", err))
		return
	}

	// Extract all links from the document
	var links []Link
	currentURLParsed, _ := url.Parse(b.currentURL) // Get current URL for relative URL resolution

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				// Resolve relative URLs
				if strings.HasPrefix(href, "/") {
					href = fmt.Sprintf("%s://%s%s", currentURLParsed.Scheme, currentURLParsed.Host, href)
				} else if !strings.HasPrefix(href, "http") && !strings.HasPrefix(href, "#") {
					// Handle relative URLs
					baseURL := fmt.Sprintf("%s://%s", currentURLParsed.Scheme, currentURLParsed.Host)
					href = baseURL + "/" + href
				} else if strings.HasPrefix(href, "#") && currentURLParsed != nil {
					// Handle anchor links
					href = fmt.Sprintf("%s://%s%s%s", currentURLParsed.Scheme, currentURLParsed.Host, currentURLParsed.Path, href)
				}
				links = append(links, Link{
					URL:      href,
					Text:     text,
					Position: len(links), // Simple position tracking
				})
			}
		}
	})

	// Store the links in browser
	b.links = links
	b.currentLinkIndex = -1 // Start with no link selected

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
				result.WriteString(" ")
			}
			// Escape special characters that might interfere with tview formatting
			text = strings.ReplaceAll(text, "[", "\\[")
			text = strings.ReplaceAll(text, "]", "\\]")
			text = strings.ReplaceAll(text, "*", "\\*")
			text = strings.ReplaceAll(text, "_", "\\_")
			text = strings.ReplaceAll(text, "`", "\\`")
			result.WriteString(text)
		}
	case html.ElementNode:
		tag := node.DataAtom.String()
		isBlockElement := b.isBlockElement(tag)

		// Handle special tags
		switch tag {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("## ")
			*tabs += 1
		case "p":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
		case "a":
			// Just process the content without special formatting for now
			break
		case "ul", "ol":
			*tabs += 1
		case "li":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			for i := 0; i < *tabs-1; i++ {
				result.WriteString(" ")
			}
			if b.isParent(node, "ol") {
				// Handle ordered list
				index := b.getListItemIndex(node)
				result.WriteString(fmt.Sprintf("%d. ", index))
			} else {
				result.WriteString("* ")
			}
		case "br":
			result.WriteString("\n")
		case "div":
			if isBlockElement && result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
		case "pre", "code":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
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
			result.WriteString("\n")
		case "a":
			if href, exists := b.getAttribute(node, "href"); exists {
				result.WriteString(fmt.Sprintf(" (%s)", href))
			}
		case "ul", "ol":
			*tabs -= 1
		}

		if isBlockElement && tag != "li" && tag != "h1" && tag != "h2" && tag != "h3" && tag != "h4" && tag != "h5" && tag != "h6" {
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
		}
	}
}