package main

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
)

func (b *Browser) renderPage(htmlContent, rawURL string) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		b.renderPageFallback(htmlContent)
		return
	}

	processedHTML := htmlContent
	if b.config != nil && b.config.EnableContentSecurity {
		processedHTML = sanitizeHTML(processedHTML)
	}

	if refreshURL, delay, found := detectMetaRefresh(processedHTML); found {
		ctx, cancel := context.WithCancel(context.Background())
		tab := b.currentTab()
		tab.metaRefreshCancel = cancel
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(delay) * time.Second):
				b.app.QueueUpdateDraw(func() {
					b.NavigateTo(refreshURL)
				})
			}
		}()
	}

	var allLinks []Link
	linkCounter := 0

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(processedHTML))
	if err != nil {
		b.renderPageFallback(htmlContent)
		return
	}

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				href = resolveURL(href, parsedURL)
				if href != "" {
					allLinks = append(allLinks, Link{
						URL:      href,
						Text:     text,
						Position: linkCounter,
					})
					linkCounter++
				}
			}
		}
	})

	if b.config != nil && b.config.BlockExternalResources {
		blockExternalResources(doc, parsedURL)
	}

	images := extractImagesFromDoc(doc, parsedURL)
	doc = nil

	article, err := readability.FromReader(strings.NewReader(processedHTML), parsedURL)
	if err != nil {
		b.renderPageFallback(htmlContent)
		return
	}

	var result strings.Builder

	if article.Title != "" {
		result.WriteString(fmt.Sprintf("[::b]%s[::-]\n\n", article.Title))
	}

	cleanedContent := cleanExcessiveWhitespace(article.TextContent)
	sanitizedContent := b.sanitizeForTview(cleanedContent)
	visibleLinks := b.extractVisibleLinks(sanitizedContent, allLinks)
	result.WriteString(sanitizedContent)

	if article.Image != "" {
		result.WriteString(fmt.Sprintf("\n[Image: %s]", article.Image))
	}

	tab := b.currentTab()
	tab.links = visibleLinks
	tab.images = images
	tab.currentLinkIndex = -1

	originalUnprocessedText := result.String()
	b.updateWordWrapBasedOnContent(originalUnprocessedText)
	processedContent := b.ensureContentVisibilityForTheme(originalUnprocessedText)

	tab.originalUnprocessedContent = originalUnprocessedText
	tab.originalContent = processedContent
	tab.textView.SetText(processedContent)
	tab.textView.ScrollToBeginning()

	b.updateStatusBar()
	result.Reset()
}

func cleanExcessiveWhitespace(text string) string {
	lines := strings.Split(text, "\n")
	var cleanedLines []string
	previousWasEmpty := false

	for _, line := range lines {
		isEmpty := strings.TrimSpace(line) == ""
		if isEmpty && previousWasEmpty {
			continue
		}
		cleanedLines = append(cleanedLines, line)
		previousWasEmpty = isEmpty
	}

	result := strings.Join(cleanedLines, "\n")
	result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	return result
}

type Image struct {
	URL   string
	Alt   string
	Title string
	Src   string
}

func extractImagesFromDoc(doc *goquery.Document, baseURL *url.URL) []Image {
	if doc == nil || baseURL == nil {
		return nil
	}

	var images []Image
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if src == "" {
			return
		}
		alt, _ := s.Attr("alt")
		title, _ := s.Attr("title")
		imageURL := resolveURL(src, baseURL)

		images = append(images, Image{
			URL:   imageURL,
			Alt:   alt,
			Title: title,
			Src:   src,
		})
	})
	return images
}

func resolveURL(rawURL string, base *url.URL) string {
	if base == nil {
		return rawURL
	}
	ref, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return base.ResolveReference(ref).String()
}

var metaRefreshRegex = regexp.MustCompile(`(?i)<meta\s+http-equiv=["']?refresh["']?\s+content=["'](\d+)\s*;\s*url=([^"']+)["']`)

func detectMetaRefresh(htmlContent string) (string, int, bool) {
	m := metaRefreshRegex.FindStringSubmatch(htmlContent)
	if m == nil {
		return "", 0, false
	}
	delay := 0
	if d, err := strconv.Atoi(strings.TrimSpace(m[1])); err == nil {
		delay = d
	}
	targetURL := strings.Trim(strings.TrimSpace(m[2]), "'\"")
	if targetURL == "" {
		return "", 0, false
	}
	return targetURL, delay, true
}

func (b *Browser) renderPageFallback(htmlContent string) {
	processedHTML := htmlContent
	if b.config != nil && b.config.EnableContentSecurity {
		processedHTML = sanitizeHTML(processedHTML)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(processedHTML))
	if err != nil {
		b.displayError(fmt.Sprintf("Error parsing HTML: %v", err))
		return
	}

	var links []Link
	currentURLParsed, _ := url.Parse(b.currentTab().currentURL)

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				href = resolveURL(href, currentURLParsed)
				if href != "" {
					links = append(links, Link{
						URL:      href,
						Text:     text,
						Position: len(links),
					})
				}
			}
		}
	})

	if b.config != nil && b.config.BlockExternalResources {
		blockExternalResources(doc, currentURLParsed)
	}

	images := extractImagesFromDoc(doc, currentURLParsed)

	var result strings.Builder
	tabs := 0

	doc.Find("body").Contents().Each(func(i int, s *goquery.Selection) {
		node := s.Get(0)
		b.renderNode(node, &result, &tabs)
	})
	doc = nil

	rawContent := result.String()
	visibleLinks := b.extractVisibleLinks(rawContent, links)

	tab := b.currentTab()
	tab.links = visibleLinks
	tab.images = images
	tab.currentLinkIndex = -1

	processedContent := b.ensureContentVisibilityForTheme(rawContent)
	b.updateWordWrapBasedOnContent(rawContent)

	tab.originalUnprocessedContent = rawContent
	tab.originalContent = processedContent
	tab.textView.SetText(processedContent)
	tab.textView.ScrollToBeginning()

	result.Reset()
}
