package main

import (
	"testing"
)

func TestRemoveTviewFormatting(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"no formatting", "hello world", "hello world"},
		{"color code", "[red]hello[-]", "hello"},
		{"bold code", "[::b]bold[::-]", "bold"},
		{"complex formatting", "[red:blue:b]styled[::-]", "styled"},
		{"multiple codes", "[red]a[-] [::b]b[::-]", "a b"},
		{"nested", "[red]outer [::b]inner[::-] text[-]", "outer inner text"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := removeTviewFormatting(tc.input)
			if got != tc.want {
				t.Errorf("removeTviewFormatting(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRemoveUnwantedCharsFromDisplay(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain text", "hello world", "hello world"},
		{"removes tview color codes", "[red]hello[-]", "hello"},
		{"removes region tags", "[\"region\"]text[\"\"]", "text"},
		{"preserves normal printable", "abc123!@#$%", "abc123!@#$%"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := removeUnwantedCharsFromDisplay(tc.input)
			if got != tc.want {
				t.Errorf("removeUnwantedCharsFromDisplay(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestCurrentPosAtLineStart(t *testing.T) {
	text := "line0\nline1\nline2"
	tests := []struct {
		lineIndex int
		want      int
	}{
		{0, 0},
		{1, 6},
		{2, 12},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := currentPosAtLineStart(text, tc.lineIndex)
			if got != tc.want {
				t.Errorf("currentPosAtLineStart(line %d) = %d, want %d", tc.lineIndex, got, tc.want)
			}
		})
	}
}
