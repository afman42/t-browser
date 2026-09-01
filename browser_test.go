package main

import "testing"

func TestColorToTviewFormat(t *testing.T) {
	tests := []struct {
		name  string
		color string
		want  string
	}{
		{"yellow", "yellow", "yellow"},
		{"red", "red", "red"},
		{"green", "green", "green"},
		{"blue", "blue", "blue"},
		{"magenta", "magenta", "magenta"},
		{"cyan", "cyan", "cyan"},
		{"white", "white", "white"},
		{"black", "black", "black"},
		{"bold", "bold", "::b"},
		{"underline", "underline", "::u"},
		{"reverse", "reverse", "::r"},
		{"unknown defaults to yellow", "pink", "yellow"},
		{"empty defaults to yellow", "", "yellow"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ColorToTviewFormat(tc.color)
			if got != tc.want {
				t.Errorf("ColorToTviewFormat(%q) = %q, want %q", tc.color, got, tc.want)
			}
		})
	}
}

func TestShouldDisableWordWrap_ShortLines(t *testing.T) {
	b := &Browser{}
	content := "short line\nanother line\nthird line"
	if b.shouldDisableWordWrap(content) {
		t.Error("shouldDisableWordWrap should be false for short lines")
	}
}

func TestShouldDisableWordWrap_LongLines(t *testing.T) {
	b := &Browser{}
	content := ""
	for range 8 {
		content += "short line\n"
	}
	for range 2 {
		content += string(make([]byte, 501)) + "\n"
	}

	if !b.shouldDisableWordWrap(content) {
		t.Error("shouldDisableWordWrap should be true when >20% of lines are long")
	}
}

func TestShouldDisableWordWrap_ExtremeLine(t *testing.T) {
	b := &Browser{}
	content := string(make([]byte, 600))
	if !b.shouldDisableWordWrap(content) {
		t.Error("shouldDisableWordWrap should be true for a line >500 chars")
	}
}

func TestShouldDisableWordWrap_EmptyContent(t *testing.T) {
	b := &Browser{}
	if b.shouldDisableWordWrap("") {
		t.Error("shouldDisableWordWrap should be false for empty content")
	}
}

func TestValidateAndSanitizeURL_Valid(t *testing.T) {
	b := &Browser{}
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com", "https://example.com"},
		{"http://example.com/path", "http://example.com/path"},
		{"https://example.com:8080/path?q=1", "https://example.com:8080/path?q=1"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := b.validateAndSanitizeURL(tc.input)
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateAndSanitizeURL_Dangerous(t *testing.T) {
	b := &Browser{}
	dangerous := []string{
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"vbscript:msgbox",
		"file:///etc/passwd",
	}
	for _, input := range dangerous {
		t.Run(input[:10], func(t *testing.T) {
			_, err := b.validateAndSanitizeURL(input)
			if err == nil {
				t.Errorf("expected error for dangerous URL %q", input)
			}
		})
	}
}

func TestValidateAndSanitizeURL_LocalhostBlocked(t *testing.T) {
	b := &Browser{}
	local := []string{
		"http://localhost:8080",
		"http://127.0.0.1:3000",
		"http://10.0.0.1/admin",
		"http://192.168.1.1",
	}
	for _, input := range local {
		t.Run(input, func(t *testing.T) {
			_, err := b.validateAndSanitizeURL(input)
			if err == nil {
				t.Errorf("expected error for local address %q", input)
			}
		})
	}
}

func TestValidateAndSanitizeURL_Empty(t *testing.T) {
	b := &Browser{}
	_, err := b.validateAndSanitizeURL("")
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestValidateAndSanitizeURL_TooLong(t *testing.T) {
	b := &Browser{}
	long := string(make([]byte, 2049))
	_, err := b.validateAndSanitizeURL(long)
	if err == nil {
		t.Error("expected error for URL > 2048 chars")
	}
}

func TestValidateAndSanitizeURL_Suspicious(t *testing.T) {
	b := &Browser{}
	tests := []string{
		"https://example.com/..%2F..%2Fetc",
		"https://example.com/data0x00malicious",
	}
	for _, input := range tests {
		t.Run(input[:20], func(t *testing.T) {
			_, err := b.validateAndSanitizeURL(input)
			if err == nil {
				t.Errorf("expected error for suspicious URL %q", input)
			}
		})
	}
}

func TestValidateAndSanitizeURL_BadFormat(t *testing.T) {
	b := &Browser{}
	_, err := b.validateAndSanitizeURL("://invalid")
	if err == nil {
		t.Error("expected error for malformed URL")
	}
}

func TestGetHistoryCompletions(t *testing.T) {
	b := testBrowserWithUI()
	tab := b.currentTab()
	tab.history = []string{
		"https://example.com/a",
		"https://example.com/b",
		"https://example.com/a", // duplicate
		"https://other.org/x",
	}

	got := b.getHistoryCompletions("https://example.com/", 10)
	if len(got) != 2 {
		t.Fatalf("completions = %v, want 2 unique", got)
	}
	// Most-recent-first, deduplicated.
	if got[0] != "https://example.com/b" && got[0] != "https://example.com/a" {
		t.Errorf("first completion = %q", got[0])
	}
	if len(got) == 2 && got[0] == got[1] {
		t.Error("completions must not contain duplicates")
	}

	limited := b.getHistoryCompletions("https://", 1)
	if len(limited) != 1 {
		t.Errorf("limit 1 returned %d", len(limited))
	}

	if got := b.getHistoryCompletions("nomatch", 10); len(got) != 0 {
		t.Errorf("no-prefix matches should be empty, got %v", got)
	}
}

func TestMainFlexBuilds(t *testing.T) {
	b := testBrowserWithUI()
	flex := b.mainFlex()
	if flex == nil {
		t.Fatal("mainFlex returned nil")
	}
	if flex.GetItemCount() != 4 {
		t.Errorf("mainFlex items = %d, want 4 (tabbar, text, status, input)", flex.GetItemCount())
	}
}

func TestNewTabAddsActive(t *testing.T) {
	b := testBrowserWithUI()
	before := len(b.tabs)
	b.newTab()
	if len(b.tabs) != before+1 {
		t.Errorf("tabs = %d, want %d", len(b.tabs), before+1)
	}
	if b.activeTab != len(b.tabs)-1 {
		t.Errorf("activeTab = %d, want last", b.activeTab)
	}
}

func TestCloseTab(t *testing.T) {
	b := testBrowserWithUI()
	b.tabs = append(b.tabs, newTab())
	b.activeTab = 1

	b.closeTab()
	if len(b.tabs) != 1 {
		t.Errorf("tabs = %d, want 1", len(b.tabs))
	}
	if b.activeTab != 0 {
		t.Errorf("activeTab = %d, want 0 after closing last", b.activeTab)
	}
}

func TestCloseTabLastStopsApp(t *testing.T) {
	b := testBrowserWithUI()
	b.closeTab() // single tab → app.Stop() path; must not panic
}

func TestRedrawCurrentView(t *testing.T) {
	b := testBrowserWithUI()
	b.redrawCurrentView(b.currentTab().textView)
	// nil app is a no-op.
	(&Browser{tabBar: nil}).redrawCurrentView(nil)
}
