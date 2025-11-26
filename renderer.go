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
		// Sanitize the title to prevent formatting code injection
		sanitizedTitle := b.sanitizeForTview(article.Title)
		result.WriteString(fmt.Sprintf("[::b]%s[::-]\n", sanitizedTitle))
	}

	// Clean up the extracted content to remove excessive whitespace
	cleanedContent := cleanExcessiveWhitespace(article.TextContent)

	// Sanitize the content to prevent formatting code injection
	sanitizedContent := b.sanitizeForTview(cleanedContent)

	// Process content to embed link numbers directly in the text
	processedContent := b.embedLinkNumbers(sanitizedContent, links)

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

	// Dynamically adjust word wrap based on content characteristics
	b.updateWordWrapBasedOnContent(originalText)

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

	// Dynamically adjust word wrap based on content characteristics
	b.updateWordWrapBasedOnContent(processedContent)

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

// embedLinkNumbers embeds link numbers directly in the content using a better algorithm
// This approach tries to match links based on their position and context to avoid misnumbering
func (b *Browser) embedLinkNumbers(content string, links []Link) string {
	if len(links) == 0 {
		return content
	}

	result := content

	// Create a mapping of link text to all its occurrences and corresponding link numbers
	// Use a more robust approach that considers link context
	usedPositions := make(map[int]bool) // Track positions already numbered

	for i, link := range links {
		linkNumber := i + 1

		// Find the next unused occurrence of the link text
		// Start searching from beginning of content
		startIdx := 0
		found := false

		for startIdx < len(result) && !found {
			pos := strings.Index(result[startIdx:], link.Text)
			if pos == -1 {
				break // No more occurrences found
			}

			actualPos := startIdx + pos

			// Check if this position is already used for numbering
			if usedPositions[actualPos] {
				startIdx = actualPos + 1
				continue
			}

			// Check if this occurrence is already numbered
			endPos := actualPos + len(link.Text)
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
				// Replace this occurrence with the link number
				before := result[:actualPos]
				after := result[endPos:]
				result = before + fmt.Sprintf("%s [%d]", link.Text, linkNumber) + after

				// Mark this position as used
				usedPositions[actualPos] = true
				found = true
			} else {
				startIdx = actualPos + 1
			}
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

	// Dynamically adjust word wrap based on content characteristics
	b.updateWordWrapBasedOnContent(originalText)

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

// sanitizeForTview sanitizes text to prevent tview formatting code injection
func (b *Browser) sanitizeForTview(text string) string {
	// Escape square brackets which are used for tview formatting
	result := strings.ReplaceAll(text, "[", "\\[")
	result = strings.ReplaceAll(result, "]", "\\]")

	// Additional formatting characters that might be relevant
	result = strings.ReplaceAll(result, "*", "\\*")
	result = strings.ReplaceAll(result, "_", "\\_")
	result = strings.ReplaceAll(result, "`", "\\`")

	return result
}