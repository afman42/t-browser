package main

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestRenderTableBasic(t *testing.T) {
	htmlContent := `<table><thead><tr><th>Name</th><th>Age</th></tr></thead><tbody><tr><td>Alice</td><td>30</td></tr><tr><td>Bob</td><td>25</td></tr></tbody></table>`
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		t.Fatalf("html.Parse failed: %v", err)
	}

	b := &Browser{}
	var result strings.Builder
	tabs := 0

	var findTable func(*html.Node) *html.Node
	findTable = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "table" {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if t := findTable(c); t != nil {
				return t
			}
		}
		return nil
	}

	tableNode := findTable(doc)
	if tableNode == nil {
		t.Fatal("no table node found in HTML")
	}

	b.renderTable(tableNode, &result, &tabs)

	output := result.String()
	if !strings.Contains(output, "Alice") {
		t.Errorf("expected 'Alice' in table output, got: %s", output)
	}
	if !strings.Contains(output, "Bob") {
		t.Errorf("expected 'Bob' in table output, got: %s", output)
	}
	if !strings.Contains(output, "Name") {
		t.Errorf("expected 'Name' (header) in table output, got: %s", output)
	}
	if !strings.Contains(output, "Age") {
		t.Errorf("expected 'Age' (header) in table output, got: %s", output)
	}
	if !strings.Contains(output, "│") {
		t.Errorf("expected border character '│' in table output")
	}
	if !strings.Contains(output, "┌") {
		t.Errorf("expected top-left border '┌' in table output")
	}
	if !strings.Contains(output, "└") {
		t.Errorf("expected bottom-left border '└' in table output")
	}
}

func TestRenderTableEmpty(t *testing.T) {
	htmlContent := `<table></table>`
	doc, _ := html.Parse(strings.NewReader(htmlContent))

	b := &Browser{}
	var result strings.Builder
	tabs := 0

	var findTable func(*html.Node) *html.Node
	findTable = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "table" {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if t := findTable(c); t != nil {
				return t
			}
		}
		return nil
	}

	tableNode := findTable(doc)
	if tableNode == nil {
		t.Fatal("no table node found")
	}

	b.renderTable(tableNode, &result, &tabs)
	output := result.String()
	if strings.Contains(output, "Alice") {
		t.Errorf("empty table should not contain data, got: %s", output)
	}
}

func TestRenderTableColumnAlignment(t *testing.T) {
	htmlContent := `<table><tr><td>short</td><td>very long text here</td></tr></table>`
	doc, _ := html.Parse(strings.NewReader(htmlContent))

	b := &Browser{}
	var result strings.Builder
	tabs := 0

	var findTable func(*html.Node) *html.Node
	findTable = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "table" {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if t := findTable(c); t != nil {
				return t
			}
		}
		return nil
	}

	tableNode := findTable(doc)
	b.renderTable(tableNode, &result, &tabs)

	output := result.String()
	if !strings.Contains(output, "short") {
		t.Errorf("expected 'short' in output")
	}
	if !strings.Contains(output, "very long text here") {
		t.Errorf("expected 'very long text here' in output")
	}
}

func TestSanitizeHTMLWithReport(t *testing.T) {
	html := `<html><body>
		<script>alert(1)</script>
		<iframe src="evil.com"></iframe>
		<div onclick="alert(1)">click me</div>
		<a href="javascript:alert(1)">link</a>
		<p>safe content</p>
	</body></html>`

	cleaned, report := sanitizeHTMLWithReport(html)

	if strings.Contains(cleaned, "<script") {
		t.Error("script tag should be removed")
	}
	if strings.Contains(cleaned, "<iframe") {
		t.Error("iframe tag should be removed")
	}
	if strings.Contains(cleaned, "onclick") {
		t.Error("event handler should be removed")
	}
	if strings.Contains(cleaned, "javascript:") {
		t.Error("javascript: URL should be removed")
	}
	if !strings.Contains(cleaned, "safe content") {
		t.Error("safe content should be preserved")
	}

	if report.ScriptsRemoved != 1 {
		t.Errorf("ScriptsRemoved = %d, want 1", report.ScriptsRemoved)
	}
	if report.IframesRemoved != 1 {
		t.Errorf("IframesRemoved = %d, want 1", report.IframesRemoved)
	}
	if report.EventHandlersRemoved != 1 {
		t.Errorf("EventHandlersRemoved = %d, want 1", report.EventHandlersRemoved)
	}
}

func TestSanitizeHTMLWithReportEmpty(t *testing.T) {
	cleaned, report := sanitizeHTMLWithReport("")
	if cleaned != "" {
		t.Errorf("empty input should return empty output, got %q", cleaned)
	}
	if report.ScriptsRemoved != 0 {
		t.Errorf("ScriptsRemoved = %d, want 0 for empty input", report.ScriptsRemoved)
	}
}

func TestSanitizeHTMLWithReportNoIssues(t *testing.T) {
	html := `<html><body><p>clean content</p></body></html>`
	cleaned, report := sanitizeHTMLWithReport(html)
	if cleaned != html {
		t.Errorf("clean HTML should be unchanged")
	}
	if report.ScriptsRemoved != 0 || report.IframesRemoved != 0 || report.EventHandlersRemoved != 0 {
		t.Errorf("no issues should be reported for clean HTML: %+v", report)
	}
}
