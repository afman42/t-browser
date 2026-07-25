package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
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

	// ----- Content Security -----
	processedHTML := htmlContent
	if b.config != nil && b.config.EnableContentSecurity {
		processedHTML = sanitizeHTML(processedHTML)
	}

	// Extract all links and images from the document
	var allLinks []Link
	linkCounter := 0

	// Parse HTML to extract links efficiently
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(processedHTML))
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

	// Block external resources in the goquery document.
	if b.config != nil && b.config.BlockExternalResources {
		blockExternalResources(doc, parsedURL)
	}

	// Extract images from the doc AFTER blockExternalResources (so external
	// images are excluded).
	images := extractImagesFromDoc(doc, parsedURL)

	// Free the DOM document memory as soon as we're done with link extraction
	doc = nil

	// Use go-readability to extract the main content
	article, err := readability.FromReader(strings.NewReader(processedHTML), parsedURL)
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
	originalUnprocessedText := result.String()

	// Dynamically adjust word wrap based on content characteristics
	b.updateWordWrapBasedOnContent(originalUnprocessedText)

	// Process content to ensure visibility based on current theme
	processedContent := b.ensureContentVisibilityForTheme(originalUnprocessedText)

	b.originalUnprocessedContent = originalUnprocessedText
	b.originalContent = processedContent
	b.textView.SetText(processedContent)
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

	originalUnprocessedContent := result.String()

	// Process content to ensure visibility based on current theme
	themeProcessedContent := b.ensureContentVisibilityForTheme(originalUnprocessedContent)

	// Set the content to the text view and store original content
	b.originalUnprocessedContent = originalUnprocessedContent
	b.originalContent = themeProcessedContent

	// Dynamically adjust word wrap based on content characteristics
	b.updateWordWrapBasedOnContent(originalUnprocessedContent)

	b.textView.SetText(themeProcessedContent)
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

// extractImagesFromDoc extracts images from an already-parsed goquery document.
// It respects any previous blocking done by blockExternalResources (images with
// empty src after blocking are skipped).
func extractImagesFromDoc(doc *goquery.Document, baseURL *url.URL) []Image {
	if doc == nil || baseURL == nil {
		return nil
	}

	var images []Image
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if src == "" {
			return // blocked by content security or no src
		}
		alt, _ := s.Attr("alt")
		title, _ := s.Attr("title")

		// Resolve relative URLs.
		imageURL := src
		if strings.HasPrefix(src, "/") {
			imageURL = fmt.Sprintf("%s://%s%s", baseURL.Scheme, baseURL.Host, src)
		} else if !strings.HasPrefix(src, "http") {
			baseURLPath := fmt.Sprintf("%s://%s", baseURL.Scheme, baseURL.Host)
			if strings.HasSuffix(baseURL.Path, "/") {
				imageURL = baseURLPath + baseURL.Path + src
			} else {
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
	// ----- Content Security -----
	processedHTML := htmlContent
	if b.config != nil && b.config.EnableContentSecurity {
		processedHTML = sanitizeHTML(processedHTML)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(processedHTML))
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

	// Block external resources BEFORE rendering, so external iframes/scripts
	// don't appear in the rendered output.
	if b.config != nil && b.config.BlockExternalResources {
		blockExternalResources(doc, currentURLParsed)
	}

	// Extract images from the doc AFTER blockExternalResources.
	images := extractImagesFromDoc(doc, currentURLParsed)

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

	// Process content to ensure visibility based on current theme
	processedContent := b.ensureContentVisibilityForTheme(rawContent)

	// Dynamically adjust word wrap based on content characteristics
	b.updateWordWrapBasedOnContent(rawContent)

	b.originalUnprocessedContent = rawContent
	b.originalContent = processedContent
	b.textView.SetText(processedContent)
	b.textView.ScrollToBeginning()

	// Explicitly release the result builder to help GC
	result.Reset()
}

