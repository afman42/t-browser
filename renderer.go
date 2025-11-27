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

	// Extract all links and images from the document
	var allLinks []Link
	linkCounter := 0

	// Parse HTML to extract links efficiently
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		// If parsing fails, fall back to the original method
		b.renderPageFallback(htmlContent)
		return
	}

	// Process links in a single pass to reduce memory usage
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
				allLinks = append(allLinks, Link{
					URL:      href,
					Text:     text,
					Position: linkCounter, // Position in order of appearance
				})
				linkCounter++
			}
		}
	})

	// Free the DOM document memory as soon as we're done with link extraction
	doc = nil

	// Extract all images from the document
	images := b.extractImagesFromHTML(htmlContent, parsedURL)

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
		// Add a nicely formatted title with better styling
		result.WriteString(fmt.Sprintf("[::b]%s[::-]\n\n", article.Title)) // Bold with extra spacing
	}

	// Clean up the extracted content to remove excessive whitespace
	cleanedContent := cleanExcessiveWhitespace(article.TextContent)

	// Sanitize the content to prevent formatting code injection
	sanitizedContent := b.sanitizeForTview(cleanedContent)

	// Filter links to only those that appear in the readability content
	visibleLinks := b.extractVisibleLinks(sanitizedContent, allLinks)

	// Add the sanitized content without link numbers
	result.WriteString(sanitizedContent)

	// Add image if available
	if article.Image != "" {
		result.WriteString(fmt.Sprintf("\n[Image: %s]", article.Image))
	}

	// Store the links and images in browser
	b.links = visibleLinks
	b.images = images
	b.currentLinkIndex = -1 // Start with no link selected

	// Set the content to the text view and store original content
	originalText := result.String()

	// Dynamically adjust word wrap based on content characteristics
	b.updateWordWrapBasedOnContent(originalText)

	b.originalContent = originalText
	b.textView.SetText(originalText)
	b.textView.ScrollToBeginning()

	// Explicitly release the result builder to help GC
	result.Reset()
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

	processedContent := result.String()

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

// Image represents an image element on the page
type Image struct {
	URL   string
	Alt   string
	Title string
	Src   string
}

// extractLinksFromContent extracts links efficiently from the content text
// This is a more memory-efficient approach than parsing full HTML DOM
func (b *Browser) extractLinksFromContent(content string, baseURL *url.URL) []Link {
	// This is a simplified version that might not capture all links
	// In a more sophisticated implementation, we might use regex or other methods
	// to identify potential links in the content

	// For now, we'll return an empty slice and maintain backward compatibility
	// by implementing the fallback in the renderPage method
	// A better implementation would parse the original HTML in a more efficient way
	return []Link{}
}

// extractImagesFromHTML extracts images from HTML document
func (b *Browser) extractImagesFromHTML(htmlContent string, baseURL *url.URL) []Image {
	var images []Image
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return images // Return empty if parsing fails
	}

	// Find all images in the document
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		// Get the src attribute
		src, srcExists := s.Attr("src")
		if !srcExists {
			return // Skip if no src attribute
		}

		// Get alt text
		alt, altExists := s.Attr("alt")
		if !altExists {
			alt = "" // Default to empty if no alt text
		}

		// Get title attribute
		title, titleExists := s.Attr("title")
		if !titleExists {
			title = "" // Default to empty if no title
		}

		// Resolve relative URLs
		imageURL := src
		if strings.HasPrefix(src, "/") {
			// Absolute path on the same host
			imageURL = fmt.Sprintf("%s://%s%s", baseURL.Scheme, baseURL.Host, src)
		} else if !strings.HasPrefix(src, "http") {
			// Relative path, combine with base URL
			baseURLPath := fmt.Sprintf("%s://%s", baseURL.Scheme, baseURL.Host)
			if strings.HasSuffix(baseURL.Path, "/") {
				imageURL = baseURLPath + baseURL.Path + src
			} else {
				// Get directory of current page
				dir := strings.TrimRight(baseURL.Path, "/")
				if lastSlash := strings.LastIndex(dir, "/"); lastSlash != -1 {
					dir = dir[:lastSlash+1]
				} else {
					dir = "/"
				}
				imageURL = baseURLPath + dir + src
			}
		}

		images = append(images, Image{
			URL:   imageURL,
			Alt:   alt,
			Title: title,
			Src:   src,
		})
	})

	// Free the DOM document memory as soon as we're done with image extraction
	doc = nil

	return images
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

	// Extract all images from the document (this will parse the content again, but it's for fallback)
	images := b.extractImagesFromHTML(htmlContent, currentURLParsed)

	var result strings.Builder
	tabs := 0

	// Process the document to get the raw content first
	doc.Find("body").Contents().Each(func(i int, s *goquery.Selection) {
		node := s.Get(0)
		b.renderNode(node, &result, &tabs)
	})

	// Free the DOM document memory as soon as we're done with content rendering
	doc = nil

	// Get the raw content before link numbering
	rawContent := result.String()

	// Filter links to only those that appear in the rendered content
	visibleLinks := b.extractVisibleLinks(rawContent, links)

	// Store the links and images in browser (content without link numbers)
	b.links = visibleLinks
	b.images = images
	b.currentLinkIndex = -1 // Start with no link selected

	processedContent := rawContent

	// Dynamically adjust word wrap based on content characteristics
	b.updateWordWrapBasedOnContent(processedContent)

	b.originalContent = processedContent
	b.textView.SetText(processedContent)
	b.textView.ScrollToBeginning()

	// Explicitly release the result builder to help GC
	result.Reset()
}

// renderNode renders an individual HTML node to text
func (b *Browser) renderNode(node *html.Node, result *strings.Builder, tabs *int) {
	switch node.Type {
	case html.TextNode:
		text := strings.TrimSpace(node.Data)
		if text != "" {
			// Add indentation if needed
			if result.Len() > 0 && result.String()[result.Len()-1] != ' ' && result.String()[result.Len()-1] != '\n' {
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

		// Handle special tags with improved formatting
		switch tag {
		case "h1":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("[::b]# ")
			*tabs += 2
		case "h2":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("[::b]## ")
			*tabs += 2
		case "h3":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("[::b]### ")
			*tabs += 2
		case "h4":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("[::b]#### ")
			*tabs += 2
		case "h5":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("[::b]##### ")
			*tabs += 2
		case "h6":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("[::b]###### ")
			*tabs += 2
		case "p":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			for i := 0; i < *tabs; i++ {
				result.WriteString("  ") // 2 spaces per tab level
			}
		case "b", "strong":
			result.WriteString("[::b]") // Bold formatting
		case "i", "em":
			result.WriteString("[::i]") // Italic formatting
		case "u", "ins":
			result.WriteString("[::b]") // Bold instead of underline for emphasis
		case "del", "s", "strike":
			result.WriteString("~~") // Strikethrough formatting
		case "code":
			result.WriteString("`") // Code formatting
		case "pre":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("```\n") // Code block start
		case "blockquote":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			for i := 0; i < *tabs; i++ {
				result.WriteString("  ")
			}
			result.WriteString("> ") // Blockquote indicator
			*tabs += 1
		case "a":
			// Add link formatting but keep the content readable
			// We'll handle the link reference separately in the browser
			break
		case "ul", "ol":
			*tabs += 1
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
		case "li":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			for i := 0; i < *tabs-1; i++ {
				result.WriteString("  ") // 2 spaces per indentation level
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
		case "hr":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("---\n") // Horizontal rule
		}

		// Process children
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			b.renderNode(child, result, tabs)
		}

		// Close tags with appropriate formatting
		switch tag {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			*tabs -= 2
			result.WriteString("[-]\n") // Close bold formatting and add newline
		case "b", "strong":
			result.WriteString("[-]") // Close bold formatting
		case "i", "em":
			result.WriteString("[-]") // Close italic formatting
		case "u", "ins":
			result.WriteString("[-]") // Close underline formatting
		case "del", "s", "strike":
			result.WriteString("~~") // Close strikethrough formatting
		case "code":
			result.WriteString("`") // Close code formatting
		case "pre":
			result.WriteString("\n```") // Code block end
		case "blockquote":
			*tabs -= 1
		case "a":
			if href, exists := b.getAttribute(node, "href"); exists {
				// Format the link in a more readable way
				result.WriteString(fmt.Sprintf(" [%s]", href))
			}
		case "ul", "ol":
			*tabs -= 1
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
		}

		// Add newline after block elements (except for list items which are handled individually)
		if isBlockElement && tag != "li" && tag != "h1" && tag != "h2" && tag != "h3" && tag != "h4" && tag != "h5" && tag != "h6" && tag != "pre" {
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
		}
	}
}

// sanitizeForTview sanitizes text to prevent tview formatting code injection
func (b *Browser) sanitizeForTview(text string) string {
	// First, protect any existing tview formatting by temporarily replacing them
	// This regex-like replacement looks for [:: followed by formatting chars and ]
	protectedText := text
	formattingRegex := [][2]string{
		{"[::b]", "TBFMTBOLD"},
		{"[::i]", "TBFMTITALIC"},
		{"[-]", "TBFMTEND"},
	}

	// Replace formatting codes with temporary markers
	for _, fmtPair := range formattingRegex {
		protectedText = strings.ReplaceAll(protectedText, fmtPair[0], fmtPair[1])
	}

	// Now escape any remaining [ and ] that aren't part of formatting
	protectedText = strings.ReplaceAll(protectedText, "[", "\\[")
	protectedText = strings.ReplaceAll(protectedText, "]", "\\]")

	// Restore the formatting codes
	for _, fmtPair := range formattingRegex {
		protectedText = strings.ReplaceAll(protectedText, fmtPair[1], fmtPair[0])
	}

	// Additional formatting characters that might be relevant
	protectedText = strings.ReplaceAll(protectedText, "*", "\\*")
	protectedText = strings.ReplaceAll(protectedText, "_", "\\_")
	protectedText = strings.ReplaceAll(protectedText, "`", "\\`")

	return protectedText
}
