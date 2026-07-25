package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// ---------------------------------------------------------------------------
// sanitizeHTML tests
// ---------------------------------------------------------------------------

func TestSanitizeHTMLEmpty(t *testing.T) {
	if got := sanitizeHTML(""); got != "" {
		t.Errorf("sanitizeHTML('') = %q, want ''", got)
	}
}

func TestSanitizeHTMLRemovesScriptTags(t *testing.T) {
	input := `<html><head><script>alert("xss")</script></head><body><p>hello</p><script src="http://evil.com/x.js"></script></body></html>`
	got := sanitizeHTML(input)
	if strings.Contains(got, "<script") {
		t.Errorf("sanitizeHTML should remove script tags, got: %s", got)
	}
	if !strings.Contains(got, "<p>hello</p>") {
		t.Errorf("sanitizeHTML should preserve non-dangerous content: %s", got)
	}
}

func TestSanitizeHTMLRemovesIframe(t *testing.T) {
	input := `<p>text</p><iframe src="https://evil.com"></iframe><p>more</p>`
	got := sanitizeHTML(input)
	if strings.Contains(got, "<iframe") {
		t.Errorf("sanitizeHTML should remove iframe tags, got: %s", got)
	}
	if !strings.Contains(got, "<p>text</p>") || !strings.Contains(got, "<p>more</p>") {
		t.Errorf("sanitizeHTML should preserve surrounding content: %s", got)
	}
}

func TestSanitizeHTMLRemovesObjectAndEmbed(t *testing.T) {
	tests := []string{
		`<object data="evil.swf"></object>`,
		`<embed src="evil.swf">`,               // HTML5 void element (no />)
		`<embed src="evil.swf"/>`,               // XHTML self-closing
	}
	for _, input := range tests {
		got := sanitizeHTML(input)
		if strings.Contains(got, "<object") || strings.Contains(got, "<embed") {
			t.Errorf("sanitizeHTML should remove object/embed tags, got: %s", got)
		}
	}
}

func TestSanitizeHTMLRemovesApplet(t *testing.T) {
	input := `<applet code="evil.class"></applet>`
	got := sanitizeHTML(input)
	if strings.Contains(got, "<applet") {
		t.Errorf("sanitizeHTML should remove applet tags, got: %s", got)
	}
}

func TestSanitizeHTMLRemovesEventHandlers(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"onclick double-quoted", `<button onclick="alert(1)">click</button>`},
		{"onclick single-quoted", `<button onclick='alert(1)'>click</button>`},
		{"onload", `<img src="x.png" onload="alert(1)">`},
		{"onerror", `<img src="x.png" onerror="evil()">`},
		{"onmouseover", `<div onmouseover="doEvil()">hover</div>`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeHTML(tc.input)
			if strings.Contains(got, "onclick") || strings.Contains(got, "onload") ||
				strings.Contains(got, "onerror") || strings.Contains(got, "onmouseover") {
				t.Errorf("sanitizeHTML should remove event handlers, got: %s", got)
			}
		})
	}
}

func TestSanitizeHTMLRemovesJavascriptHref(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"href javascript", `<a href="javascript:alert(1)">link</a>`},
		{"href vbscript", `<a href="vbscript:msgbox(1)">link</a>`},
		{"action javascript", `<form action="javascript:void(0)"></form>`},
		{"formaction", `<button formaction="javascript:doEvil()">submit</button>`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeHTML(tc.input)
			if strings.Contains(got, "javascript:") || strings.Contains(got, "vbscript:") {
				t.Errorf("sanitizeHTML should remove javascript: URLs, got: %s", got)
			}
		})
	}
}

func TestSanitizeHTMLPreservesValidHTML(t *testing.T) {
	input := `<p>Hello <b>world</b></p><a href="https://example.com">link</a><ul><li>item</li></ul>`
	got := sanitizeHTML(input)
	if !strings.Contains(got, "<p>Hello") {
		t.Errorf("sanitizeHTML should preserve valid HTML: %s", got)
	}
	if !strings.Contains(got, "<a href=\"https://example.com\">") {
		t.Errorf("sanitizeHTML should preserve normal links: %s", got)
	}
}

// ---------------------------------------------------------------------------
// blockExternalResources tests
// ---------------------------------------------------------------------------

func TestBlockExternalResourcesNilPageURL(t *testing.T) {
	html := `<img src="https://evil.com/pic.png">`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	// Should not panic when pageURL is nil.
	blockExternalResources(doc, nil)
}

func TestBlockExternalResourcesBlocksExternalImage(t *testing.T) {
	html := `<html><body><img src="https://evil.com/pic.png"><img src="/local.png"><img src="relative.png"></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	pageURL, _ := url.Parse("https://example.com/page")

	blockExternalResources(doc, pageURL)

	// Check that external image src was cleared and data-blocked-external set.
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		blocked, hasBlocked := s.Attr("data-blocked-external")
		if hasBlocked {
			// External image: src should be empty, data-blocked-external should contain original URL.
			if src != "" {
				t.Errorf("external image src should be cleared, got %q", src)
			}
			if blocked != "https://evil.com/pic.png" {
				t.Errorf("data-blocked-external = %q, want %q", blocked, "https://evil.com/pic.png")
			}
		} else {
			// Same-origin or relative images should keep their src.
			if src == "" {
				t.Errorf("same-origin image should keep its src")
			}
		}
	})
}

func TestBlockExternalResourcesBlocksExternalIframe(t *testing.T) {
	html := `<html><body><iframe src="https://evil.com/frame"></iframe><iframe src="/same-origin"></iframe></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	pageURL, _ := url.Parse("https://example.com/page")

	blockExternalResources(doc, pageURL)

	if doc.Find("iframe").Length() != 1 {
		t.Errorf("expected 1 iframe (same-origin), got %d", doc.Find("iframe").Length())
	}
	// The remaining iframe should be the same-origin one.
	src, _ := doc.Find("iframe").Attr("src")
	if src != "/same-origin" {
		t.Errorf("expected remaining iframe src to be '/same-origin', got %q", src)
	}
}

func TestBlockExternalResourcesBlocksExternalScript(t *testing.T) {
	html := `<html><body><script src="https://evil.com/hack.js"></script><script src="/local.js"></script></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	pageURL, _ := url.Parse("https://example.com/page")

	blockExternalResources(doc, pageURL)

	if doc.Find("script").Length() != 1 {
		t.Errorf("expected 1 script (same-origin), got %d", doc.Find("script").Length())
	}
}

func TestBlockExternalResourcesBlocksExternalStylesheet(t *testing.T) {
	html := `<html><head><link rel="stylesheet" href="https://evil.com/style.css"><link rel="stylesheet" href="/local.css"></head></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	pageURL, _ := url.Parse("https://example.com/page")

	blockExternalResources(doc, pageURL)

	if doc.Find(`link[rel="stylesheet"]`).Length() != 1 {
		t.Errorf("expected 1 stylesheet (same-origin), got %d", doc.Find(`link[rel="stylesheet"]`).Length())
	}
}

func TestBlockExternalResourcesBlocksExternalSource(t *testing.T) {
	html := `<html><body><video><source src="https://evil.com/video.mp4"><source src="/local.mp4"></video></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	pageURL, _ := url.Parse("https://example.com/page")

	blockExternalResources(doc, pageURL)

	if doc.Find("source").Length() != 1 {
		t.Errorf("expected 1 source (same-origin), got %d", doc.Find("source").Length())
	}
}
