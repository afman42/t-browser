package main

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ---------------------------------------------------------------------------
// Content Security — HTML sanitization & external resource blocking
// ---------------------------------------------------------------------------

var (
	// scriptTag matches entire <script> blocks (including inline).
	scriptTag = regexp.MustCompile(`(?is)<script[\s>].*?</script>`)

	// iframeTag matches <iframe> blocks.
	iframeTag = regexp.MustCompile(`(?is)<iframe[\s>].*?</iframe>`)

	// objectTag matches <object> and <embed> blocks, including self-closing and
	// HTML5 void-element forms (<embed ...> without </embed> or />).
	objectTag = regexp.MustCompile(`(?is)<(?:object|embed)(?:[\s>][^>]*)?>.*?</(?:object|embed)>|` +
		`<(?:object|embed)(?:[\s>][^>]*)?/?>`)

	// appletTag matches <applet> blocks.
	appletTag = regexp.MustCompile(`(?is)<applet[\s>].*?</applet>`)

	// eventHandler matches on*="..." and on*='...' attributes.
	eventHandler = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*"[^"]*"|` +
		`\s+on\w+\s*=\s*'[^']*'|` +
		`\s+on\w+\s*=\s*[^\s"'>]+`)

	// javascriptHref matches href="javascript:..." in anchor and area tags.
	javascriptHref = regexp.MustCompile(`(?i)\s+(?:href|action|formaction)\s*=\s*"(?:javascript|vbscript):[^"]*"|` +
		`\s+(?:href|action|formaction)\s*=\s*'(?:javascript|vbscript):[^']*'`)

	// javascriptSrc matches src="javascript:..." 
	javascriptSrc = regexp.MustCompile(`(?i)\s+src\s*=\s*"(?:javascript|vbscript):[^"]*"|` +
		`\s+src\s*=\s*'(?:javascript|vbscript):[^']*'`)
)

// sanitizeHTML strips dangerous elements and attributes from HTML content.
// It removes <script>, <iframe>, <object>, <embed>, <applet> tags,
// event handler attributes (onclick, onload, etc.), and javascript: URLs.
func sanitizeHTML(htmlContent string) string {
	if htmlContent == "" {
		return htmlContent
	}

	// Order matters: strip full elements before attributes to avoid
	// leaving partial tags behind.
	result := scriptTag.ReplaceAllString(htmlContent, "")
	result = iframeTag.ReplaceAllString(result, "")
	result = objectTag.ReplaceAllString(result, "")
	result = appletTag.ReplaceAllString(result, "")

	// Strip event handler attributes.
	result = eventHandler.ReplaceAllString(result, "")

	// Strip javascript: / vbscript: URLs in href, action, formaction.
	result = javascriptHref.ReplaceAllString(result, "")
	result = javascriptSrc.ReplaceAllString(result, "")

	// Clean up any empty anchor tags left behind.
	result = strings.ReplaceAll(result, "<a></a>", "")

	return result
}

// blockExternalResources removes or neutralises references to external
// resources (images, iframes, scripts, stylesheets) that point to a
// different origin than pageURL.
func blockExternalResources(doc *goquery.Document, pageURL *url.URL) {
	if pageURL == nil {
		return
	}

	pageHost := strings.ToLower(pageURL.Hostname())

	// Helper: returns true when rawURL points to an external host.
	isExternal := func(rawURL string) bool {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Host == "" {
			return false // relative URLs are fine
		}
		return strings.ToLower(parsed.Hostname()) != pageHost
	}

	// Block external images.
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok && isExternal(src) {
			// Replace the src with a placeholder so the image isn't fetched.
			s.SetAttr("src", "")
			s.SetAttr("data-blocked-external", src)
		}
	})

	// Block external iframes.
	doc.Find("iframe[src]").Each(func(i int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok && isExternal(src) {
			s.Remove()
		}
	})

	// Block external scripts.
	doc.Find("script[src]").Each(func(i int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok && isExternal(src) {
			s.Remove()
		}
	})

	// Block external stylesheets.
	doc.Find(`link[rel="stylesheet"][href]`).Each(func(i int, s *goquery.Selection) {
		if href, ok := s.Attr("href"); ok && isExternal(href) {
			s.Remove()
		}
	})

	// Block external <source> elements (used by <video>/<audio>).
	doc.Find("source[src]").Each(func(i int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok && isExternal(src) {
			s.Remove()
		}
	})
}
