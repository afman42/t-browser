package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestCleanExcessiveWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "no excessive whitespace",
			input: "hello\nworld",
			want:  "hello\nworld",
		},
		{
			name:  "collapses triple newlines to double",
			input: "a\n\n\nb",
			want:  "a\n\nb",
		},
		{
			name:  "removes consecutive empty lines",
			input: "line1\n\n\n\nline2",
			want:  "line1\n\nline2",
		},
		{
			name:  "single line unchanged",
			input: "just one line",
			want:  "just one line",
		},
		{
			name:  "leading and trailing whitespace lines",
			input: "\n\nhello\n\nworld\n\n",
			want:  "\nhello\n\nworld\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanExcessiveWhitespace(tc.input)
			if got != tc.want {
				t.Errorf("cleanExcessiveWhitespace(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSanitizeForTview(t *testing.T) {
	b := &Browser{}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "escapes brackets",
			input: "[not formatting]",
			want:  `\[not formatting\]`,
		},
		{
			name:  "preserves valid formatting codes",
			input: "[::b]bold[::-]",
			want:  "[::b]bold[::-]",
		},
		{
			name:  "asterisks are allowed through",
			input: "foo * bar",
			want:  "foo * bar",
		},
		{
			name:  "underscores are allowed through",
			input: "foo _ bar",
			want:  "foo _ bar",
		},
		{
			name:  "backticks are allowed through",
			input: "foo ` bar",
			want:  "foo ` bar",
		},
		{
			name:  "mixed: valid codes preserved, others escaped",
			input: "[::b]bold text with [brackets][::-]",
			want:  "[::b]bold text with \\[brackets\\][::-]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := b.sanitizeForTview(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeForTview(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// -------------------------------------------------------------------------
// extractImagesFromDoc tests
// -------------------------------------------------------------------------

func TestExtractImagesFromDocNilDoc(t *testing.T) {
	url, _ := url.Parse("https://example.com")
	images := extractImagesFromDoc(nil, url)
	if images != nil {
		t.Errorf("expected nil for nil doc, got %v", images)
	}
}

func TestExtractImagesFromDocNilURL(t *testing.T) {
	html := `<html><body><img src="/pic.png"></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	images := extractImagesFromDoc(doc, nil)
	if images != nil {
		t.Errorf("expected nil for nil url, got %v", images)
	}
}

func TestExtractImagesFromDocSkipsEmptySrc(t *testing.T) {
	html := `<html><body><img src=""><img src="/valid.png"></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	url, _ := url.Parse("https://example.com")
	images := extractImagesFromDoc(doc, url)
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if images[0].Src != "/valid.png" {
		t.Errorf("expected src '/valid.png', got %q", images[0].Src)
	}
}

func TestExtractImagesFromDocRelativeURLs(t *testing.T) {
	html := `<html><body><img src="/absolute.png"><img src="relative.png"></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	url, _ := url.Parse("https://example.com/sub/page")
	images := extractImagesFromDoc(doc, url)
	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(images))
	}
	// Absolute path.
	if images[0].URL != "https://example.com/absolute.png" {
		t.Errorf("absolute URL = %q, want %q", images[0].URL, "https://example.com/absolute.png")
	}
	// Relative path should resolve to the directory of baseURL.
	expectedRelative := "https://example.com/sub/relative.png"
	if images[1].URL != expectedRelative {
		t.Errorf("relative URL = %q, want %q", images[1].URL, expectedRelative)
	}
}

func TestExtractImagesFromDocCollectsAltAndTitle(t *testing.T) {
	html := `<html><body><img src="/pic.png" alt="a photo" title="My Photo"></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	url, _ := url.Parse("https://example.com")
	images := extractImagesFromDoc(doc, url)
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if images[0].Alt != "a photo" {
		t.Errorf("Alt = %q, want %q", images[0].Alt, "a photo")
	}
	if images[0].Title != "My Photo" {
		t.Errorf("Title = %q, want %q", images[0].Title, "My Photo")
	}
}

func TestExtractImagesFromDocEmptyDoc(t *testing.T) {
	html := `<html><body><p>no images here</p></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	url, _ := url.Parse("https://example.com")
	images := extractImagesFromDoc(doc, url)
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

func TestExtractLinksFromContent(t *testing.T) {
	b := &Browser{}
	url, _ := url.Parse("https://example.com")
	links := b.extractLinksFromContent("some content", url)
	if links == nil {
		t.Error("extractLinksFromContent should return empty slice, not nil")
	}
	if len(links) != 0 {
		t.Errorf("expected empty links, got %d", len(links))
	}
}

// --- Bug #5: resolveURL with protocol-relative URLs ---

func TestResolveURL(t *testing.T) {
	base, _ := url.Parse("https://example.com/path/page.html")

	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{"absolute https", "https://other.com/foo", "https://other.com/foo"},
		{"absolute http", "http://other.com/foo", "http://other.com/foo"},
		{"root-relative", "/about", "https://example.com/about"},
		{"path-relative", "subdir/page.html", "https://example.com/path/subdir/page.html"},
		{"parent-relative", "../about", "https://example.com/about"},
		{"protocol-relative", "//cdn.example.com/script.js", "https://cdn.example.com/script.js"},
		{"fragment", "#section1", "https://example.com/path/page.html#section1"},
		{"query", "?q=1", "https://example.com/path/page.html?q=1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveURL(tc.rawURL, base)
			if got != tc.want {
				t.Errorf("resolveURL(%q, %q) = %q, want %q", tc.rawURL, base.String(), got, tc.want)
			}
		})
	}
}

func TestResolveURLNilBase(t *testing.T) {
	got := resolveURL("https://example.com", nil)
	if got != "https://example.com" {
		t.Errorf("resolveURL with nil base should return raw URL, got %q", got)
	}
}

// --- Meta refresh detection ---

func TestDetectMetaRefresh(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		wantURL   string
		wantDelay int
		wantFound bool
	}{
		{
			name:      "standard meta refresh",
			html:      `<html><head><meta http-equiv="refresh" content="5;url=https://example.com/new"></head></html>`,
			wantURL:   "https://example.com/new",
			wantDelay: 5,
			wantFound: true,
		},
		{
			name:      "immediate redirect",
			html:      `<html><head><meta http-equiv="refresh" content="0;url=https://example.com/now"></head></html>`,
			wantURL:   "https://example.com/now",
			wantDelay: 0,
			wantFound: true,
		},
		{
			name:      "no meta refresh",
			html:      `<html><head><title>test</title></head></html>`,
			wantURL:   "",
			wantDelay: 0,
			wantFound: false,
		},
		{
			name:      "meta refresh with single quotes",
			html:      `<html><head><meta http-equiv='refresh' content='3;url=https://example.com/q'></head></html>`,
			wantURL:   "https://example.com/q",
			wantDelay: 3,
			wantFound: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotURL, gotDelay, gotFound := detectMetaRefresh(tc.html)
			if gotURL != tc.wantURL {
				t.Errorf("detectMetaRefresh() URL = %q, want %q", gotURL, tc.wantURL)
			}
			if gotDelay != tc.wantDelay {
				t.Errorf("detectMetaRefresh() delay = %d, want %d", gotDelay, tc.wantDelay)
			}
			if gotFound != tc.wantFound {
				t.Errorf("detectMetaRefresh() found = %v, want %v", gotFound, tc.wantFound)
			}
		})
	}
}
