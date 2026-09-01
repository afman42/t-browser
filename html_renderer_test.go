package main

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func TestEndsWithNewline(t *testing.T) {
	var b strings.Builder
	if endsWithNewline(&b) {
		t.Error("empty builder should not end with newline")
	}
	b.WriteString("abc")
	if endsWithNewline(&b) {
		t.Error(`"abc" should not end with newline`)
	}
	b.WriteString("\n")
	if !endsWithNewline(&b) {
		t.Error(`"abc\n" should end with newline`)
	}
	// Multi-byte rune at the tail must not confuse byte indexing.
	b.Reset()
	b.WriteString("héllo")
	if endsWithNewline(&b) {
		t.Error(`"héllo" should not end with newline`)
	}
}

func TestEndsWithNewlineOrSpace(t *testing.T) {
	var b strings.Builder
	if endsWithNewlineOrSpace(&b) {
		t.Error("empty builder should be false")
	}
	for _, suffix := range []string{" ", "\n", "x"} {
		b.Reset()
		b.WriteString("ab" + suffix)
		want := suffix != "x"
		if got := endsWithNewlineOrSpace(&b); got != want {
			t.Errorf(`suffix %q: got %v, want %v`, suffix, got, want)
		}
	}
}

func TestIsInThead(t *testing.T) {
	thead := &html.Node{Type: html.ElementNode, DataAtom: atom.Thead, Data: "thead"}
	tr := &html.Node{Type: html.ElementNode, DataAtom: atom.Tr, Data: "tr", Parent: thead}
	cell := &html.Node{Type: html.ElementNode, DataAtom: atom.Td, Data: "td", Parent: tr}

	if !isInThead(cell) {
		t.Error("td inside thead should be detected")
	}
	if isInThead(thead) {
		t.Error("thead itself should not be in thead")
	}
	div := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	if isInThead(div) {
		t.Error("div should not be in thead")
	}
}

func el(tag string) *html.Node {
	return &html.Node{Type: html.ElementNode, DataAtom: atom.Lookup([]byte(tag)), Data: tag}
}

func TestRenderNodeEscapesTviewCodes(t *testing.T) {
	b := &Browser{}
	var out strings.Builder
	tabs := 0
	// Text with brackets and formatting characters must be escaped for tview.
	text := textNode("[alert] *stars* _under_ `tick` >")
	b.renderNode(text, &out, &tabs)
	for _, needle := range []string{"\\[", "\\]", "\\*", "\\_", "\\`"} {
		if !strings.Contains(out.String(), needle) {
			t.Errorf("expected escape %q in output %q", needle, out.String())
		}
	}
}

func TestRenderNodeHeadingPrefix(t *testing.T) {
	b := &Browser{}
	var out strings.Builder
	tabs := 0
	h1 := el("h1")
	h1.AppendChild(textNode("Title"))
	b.renderNode(h1, &out, &tabs)
	if !strings.Contains(out.String(), "[cyan::b]# Title") {
		t.Errorf("expected heading prefix + text, got %q", out.String())
	}
	// Entry adds +2, close unwinds −2: indent must be balanced afterwards.
	if tabs != 0 {
		t.Errorf("tabs = %d, want balanced 0", tabs)
	}
}

func textNode(s string) *html.Node {
	return &html.Node{Type: html.TextNode, Data: s}
}
