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

	// Parse HTML to extract links and images first
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		// If parsing fails, fall back to the original method
		b.renderPageFallback(htmlContent)
		return
	}

	// Extract all links from the document first
	var allLinks []Link
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
				allLinks = append(allLinks, Link{
					URL:      href,
					Text:     text,
					Position: linkCounter, // Position in order of appearance
				})
				linkCounter++
			}
		}
	})

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
		// Sanitize the title to prevent formatting code injection
		sanitizedTitle := b.sanitizeForTview(article.Title)
		result.WriteString(fmt.Sprintf("[::b]%s[::-]\n", sanitizedTitle))
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
	URL     string
	Alt     string
	Title   string
	Src     string
}

// extractVisibleLinks filters links to only those that appear in the readability content
func (b *Browser) extractVisibleLinks(content string, allLinks []Link) []Link {
	var visibleLinks []Link
	usedTexts := make(map[string]bool) // Track which link texts have been used

	for _, link := range allLinks {
		// Check if the link text appears in the readability content
		// We also want to avoid duplicates
		if strings.Contains(content, link.Text) && !usedTexts[link.Text] {
			// Additional check: ensure the link text is a meaningful part of the content
			// (not just a small fragment that happens to match)
			if len(link.Text) >= 2 { // At least 2 characters to be considered meaningful
				visibleLinks = append(visibleLinks, link)
				usedTexts[link.Text] = true
			}
		}
	}

	return visibleLinks
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

	// Extract all images from the document
	images := b.extractImagesFromHTML(htmlContent, currentURLParsed)

	var result strings.Builder
	tabs := 0

	// Process the document to get the raw content first
	doc.Find("body").Contents().Each(func(i int, s *goquery.Selection) {
		node := s.Get(0)
		b.renderNode(node, &result, &tabs)
	})

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