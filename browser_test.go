package main

import (
	"testing"
)

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

func TestApplyTviewColor(t *testing.T) {
	got := ApplyTviewColor("hello", "red")
	want := "[red]hello[-]"
	if got != want {
		t.Errorf("ApplyTviewColor = %q, want %q", got, want)
	}
}

func TestApplyTviewStyle(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		fg      string
		bg      string
		attrs   string
		want    string
	}{
		{"fg only", "text", "red", "", "", "[red]text[-]"},
		{"fg and bg", "text", "red", "blue", "", "[red:blue]text[-]"},
		{"fg, bg, attrs", "text", "red", "blue", "b", "[red:blue:b]text[-]"},
		{"empty fg", "text", "", "blue", "", "[:blue]text[-]"},
		{"empty all", "text", "", "", "", "[]text[-]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyTviewStyle(tc.text, tc.fg, tc.bg, tc.attrs)
			if got != tc.want {
				t.Errorf("ApplyTviewStyle = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetColorFunc(t *testing.T) {
	// Just verify it returns a non-nil function and doesn't panic
	fn := GetColorFunc("yellow")
	if fn == nil {
		t.Fatal("GetColorFunc returned nil")
	}
	result := fn("test")
	if result == "" {
		t.Error("GetColorFunc returned empty string")
	}

	// Unknown color should default to yellow (no panic)
	fn2 := GetColorFunc("nonexistent")
	if fn2 == nil {
		t.Fatal("GetColorFunc(nonexistent) returned nil")
	}
	_ = fn2("test")
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
	// Create 10 lines, 8 short, 2 long (>500 each triggers the EXTREME check too)
	content := ""
	for range 8 {
		content += "short line\n"
	}
	// 2 extremely long lines — the >500 check fires
	for range 2 {
		content += string(make([]byte, 501)) + "\n"
	}

	if !b.shouldDisableWordWrap(content) {
		t.Error("shouldDisableWordWrap should be true when >20% of lines are long")
	}
}

func TestShouldDisableWordWrap_ExtremeLine(t *testing.T) {
	b := &Browser{}
	// Even one line over 500 should trigger it
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
