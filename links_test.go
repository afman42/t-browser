package main

import (
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func TestIsBlockElement(t *testing.T) {
	blockTags := []string{"div", "p", "h1", "h2", "article", "section", "ul", "ol", "li", "table", "tr", "pre", "blockquote"}
	for _, tag := range blockTags {
		if !isBlockElement(tag) {
			t.Errorf("isBlockElement(%q) should be true", tag)
		}
	}

	inlineTags := []string{"span", "a", "strong", "em", "img", "code", "br", "input"}
	for _, tag := range inlineTags {
		if isBlockElement(tag) {
			t.Errorf("isBlockElement(%q) should be false", tag)
		}
	}

	// Unknown tag
	if isBlockElement("unknown") {
		t.Error("isBlockElement('unknown') should be false")
	}
}

func TestGetAttribute(t *testing.T) {
	node := &html.Node{
		Type: html.ElementNode,
		Attr: []html.Attribute{
			{Key: "href", Val: "https://example.com"},
			{Key: "class", Val: "link"},
			{Key: "id", Val: "main-link"},
		},
	}

	tests := []struct {
		attrName string
		wantVal  string
		wantOk   bool
	}{
		{"href", "https://example.com", true},
		{"class", "link", true},
		{"id", "main-link", true},
		{"nonexistent", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.attrName, func(t *testing.T) {
			val, ok := getAttribute(node, tc.attrName)
			if ok != tc.wantOk {
				t.Errorf("getAttribute(%q) ok = %v, want %v", tc.attrName, ok, tc.wantOk)
			}
			if val != tc.wantVal {
				t.Errorf("getAttribute(%q) = %q, want %q", tc.attrName, val, tc.wantVal)
			}
		})
	}
}

func TestIsParent(t *testing.T) {
	// Build a minimal tree: div > p > span
	div := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Lookup([]byte("div"))}
	p := &html.Node{Type: html.ElementNode, Data: "p", DataAtom: atom.Lookup([]byte("p"))}
	span := &html.Node{Type: html.ElementNode, Data: "span", DataAtom: atom.Lookup([]byte("span"))}

	div.AppendChild(p)
	p.Parent = div
	p.AppendChild(span)
	span.Parent = p

	if !isParent(span, "div") {
		t.Error("span should have div as ancestor")
	}
	if !isParent(span, "p") {
		t.Error("span should have p as ancestor")
	}
	if isParent(span, "ul") {
		t.Error("span should not have ul as ancestor")
	}
	if isParent(div, "span") {
		t.Error("div should not have span as ancestor")
	}
}

func TestGetListItemIndex(t *testing.T) {
	// Build an ol with three lis. html.Node.AppendChild sets PrevSibling.
	ol := &html.Node{Type: html.ElementNode, Data: "ol"}
	li1 := &html.Node{Type: html.ElementNode, Data: "li", DataAtom: atom.Li}
	li2 := &html.Node{Type: html.ElementNode, Data: "li", DataAtom: atom.Li}
	li3 := &html.Node{Type: html.ElementNode, Data: "li", DataAtom: atom.Li}

	ol.AppendChild(li1)
	ol.AppendChild(li2)
	ol.AppendChild(li3)

	if idx := getListItemIndex(li1); idx != 1 {
		t.Errorf("first li index = %d, want 1", idx)
	}
	if idx := getListItemIndex(li2); idx != 2 {
		t.Errorf("second li index = %d, want 2", idx)
	}
	if idx := getListItemIndex(li3); idx != 3 {
		t.Errorf("third li index = %d, want 3", idx)
	}
}

func TestExtractVisibleLinks(t *testing.T) {
	b := &Browser{}

	allLinks := []Link{
		{URL: "https://example.com/1", Text: "First Link"},
		{URL: "https://example.com/2", Text: "Second Link"},
		{URL: "https://example.com/3", Text: "Missing Text"},
		{URL: "https://example.com/4", Text: "X"},          // too short, length < 2
		{URL: "https://example.com/5", Text: "First Link"}, // duplicate text
	}

	content := "Some content with First Link and Second Link and X"
	visible := b.extractVisibleLinks(content, allLinks)

	if len(visible) != 2 {
		t.Fatalf("expected 2 visible links, got %d", len(visible))
	}

	if visible[0].URL != "https://example.com/1" {
		t.Errorf("first visible link URL = %q, want %q", visible[0].URL, "https://example.com/1")
	}
	if visible[1].URL != "https://example.com/2" {
		t.Errorf("second visible link URL = %q, want %q", visible[1].URL, "https://example.com/2")
	}
}

func TestExtractVisibleLinksEmpty(t *testing.T) {
	b := &Browser{}
	visible := b.extractVisibleLinks("no links here", nil)
	if visible == nil {
		t.Error("should return empty slice, not nil")
	}
	if len(visible) != 0 {
		t.Errorf("expected 0, got %d", len(visible))
	}
}
