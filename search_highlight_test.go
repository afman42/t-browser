package main

import (
	"strings"
	"testing"
)

func TestHighlightSearchTermCaseSensitive(t *testing.T) {
	b := &Browser{}
	result := b.highlightSearchTerm("hello world hello", "hello", true)

	// Should highlight both occurrences of "hello"
	expected := "[yellow]hello[-] world [yellow]hello[-]"
	if result != expected {
		t.Errorf("highlightSearchTerm = %q, want %q", result, expected)
	}
}

func TestHighlightSearchTermCaseInsensitive(t *testing.T) {
	b := &Browser{}
	result := b.highlightSearchTerm("Hello HELLO hello", "hello", false)

	// All three should be highlighted regardless of case
	count := strings.Count(result, "[yellow]")
	if count != 3 {
		t.Errorf("expected 3 highlights, got %d in: %s", count, result)
	}
}

func TestHighlightSearchTermNoMatch(t *testing.T) {
	b := &Browser{}
	result := b.highlightSearchTerm("hello world", "xyz", true)
	if result != "hello world" {
		t.Errorf("expected unchanged text, got: %s", result)
	}
}

func TestHighlightSearchTermEmptyTerm(t *testing.T) {
	b := &Browser{}
	result := b.highlightSearchTerm("hello world", "", true)
	if result != "hello world" {
		t.Errorf("expected unchanged text for empty term, got: %s", result)
	}
}

func TestHighlightSearchTermEmptyText(t *testing.T) {
	b := &Browser{}
	result := b.highlightSearchTerm("", "hello", true)
	if result != "" {
		t.Errorf("expected empty result, got: %s", result)
	}
}

func TestHighlightSearchTermWithColor(t *testing.T) {
	b := &Browser{}
	result := b.highlightSearchTermWithColor("foo bar foo", "foo", true, "red")

	// Should use red foreground with yellow background
	if !strings.Contains(result, "[red:yellow]") {
		t.Errorf("expected [red:yellow] formatting, got: %s", result)
	}
	if !strings.Contains(result, "foo") {
		t.Errorf("expected 'foo' to be present in result")
	}
}

func TestFindSearchMatchesBasic(t *testing.T) {
	b := &Browser{}
	count, contexts := b.findSearchMatches("the quick brown fox jumps over the lazy dog", "the", true)
	if count != 2 {
		t.Errorf("expected 2 matches, got %d", count)
	}
	if len(contexts) == 0 {
		t.Error("expected non-empty contexts")
	}
}

func TestFindSearchMatchesCaseInsensitive(t *testing.T) {
	b := &Browser{}
	count, contexts := b.findSearchMatches("The THE the", "the", false)
	if count != 3 {
		t.Errorf("expected 3 matches for case-insensitive, got %d", count)
	}
	if len(contexts) == 0 {
		t.Error("expected non-empty contexts")
	}
}

func TestFindSearchMatchesNoResults(t *testing.T) {
	b := &Browser{}
	count, contexts := b.findSearchMatches("hello world", "xyz", true)
	if count != 0 {
		t.Errorf("expected 0 matches, got %d", count)
	}
	if contexts == nil {
		t.Error("contexts should be empty slice, not nil")
	}
	if len(contexts) != 0 {
		t.Errorf("expected 0 contexts, got %d", len(contexts))
	}
}

func TestFindSearchMatchesEmptyText(t *testing.T) {
	b := &Browser{}
	count, contexts := b.findSearchMatches("", "hello", true)
	if count != 0 {
		t.Errorf("expected 0 matches for empty text, got %d", count)
	}
	if len(contexts) != 0 {
		t.Errorf("expected 0 contexts, got %d", len(contexts))
	}
}

func TestFindSearchMatchesEmptyTerm(t *testing.T) {
	b := &Browser{}
	count, contexts := b.findSearchMatches("hello", "", true)
	if count != 0 {
		t.Errorf("expected 0 matches for empty term, got %d", count)
	}
	if len(contexts) != 0 {
		t.Errorf("expected 0 contexts, got %d", len(contexts))
	}
}

func TestFindSearchMatchesDeduplicatesContexts(t *testing.T) {
	b := &Browser{}
	// "fox" appears twice but the context word is the same
	_, contexts := b.findSearchMatches("fox and fox", "fox", true)
	if len(contexts) != 1 {
		t.Errorf("expected 1 unique context, got %d: %v", len(contexts), contexts)
	}
}

func TestFindSearchMatchesWithPositions(t *testing.T) {
	b := &Browser{}
	matches := b.findSearchMatchesWithPositions("hello world\nfoo bar", "hello", true)

	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}

	m := matches[0]
	if m.LineNum != 0 {
		t.Errorf("expected line 0, got %d", m.LineNum)
	}
	if m.CharStart != 0 {
		t.Errorf("expected CharStart 0, got %d", m.CharStart)
	}
	if m.CharEnd != 5 {
		t.Errorf("expected CharEnd 5, got %d", m.CharEnd)
	}
	if !strings.Contains(m.LineText, "hello") {
		t.Errorf("expected LineText to contain 'hello', got: %s", m.LineText)
	}
}

func TestFindSearchMatchesWithPositionsSecondLine(t *testing.T) {
	b := &Browser{}
	matches := b.findSearchMatchesWithPositions("line one\nfoo bar\nline three", "foo", true)

	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}

	m := matches[0]
	if m.LineNum != 1 {
		t.Errorf("expected line 1, got %d", m.LineNum)
	}
	if m.CharStart != 9 { // len("line one\n") = 9
		t.Errorf("expected CharStart 9, got %d", m.CharStart)
	}
}

func TestFindSearchMatchesWithPositionsMultipleMatches(t *testing.T) {
	b := &Browser{}
	matches := b.findSearchMatchesWithPositions("foo bar foo\nbaz foo", "foo", true)

	// Should return one match per unique line (deduplication)
	if len(matches) != 2 {
		t.Errorf("expected 2 matches (one per line), got %d", len(matches))
	}
}

func TestFindSearchMatchesWithPositionsEmptyTerm(t *testing.T) {
	b := &Browser{}
	matches := b.findSearchMatchesWithPositions("hello", "", true)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for empty term, got %d", len(matches))
	}
}

func TestFindSearchMatchesWithPositionsNoMatch(t *testing.T) {
	b := &Browser{}
	matches := b.findSearchMatchesWithPositions("hello", "xyz", true)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestHighlightSelectedMatch(t *testing.T) {
	b := &Browser{}
	text := "first line\nsecond line with match\nthird line"
	term := "match"

	// Create a SearchMatch pointing to "match" in the second line
	// "first line\n" = 11 chars, so "match" starts at len("first line\nsecond line with ") = 30
	selected := SearchMatch{
		LineNum:   1,
		LineText:  "second line with match",
		CharStart: 11 + 1 + 16, // "first line\n" (11) + "second line with " (16) = 28
		CharEnd:   11 + 1 + 21, // 28 + 5 = 33
	}

	result := b.highlightSelectedMatch(text, term, selected)

	// The result should contain the highlighting markup
	if !strings.Contains(result, "[yellow::b]") && !strings.Contains(result, "[yellow:") && !strings.Contains(result, "[yellow]") {
		t.Errorf("expected some yellow highlighting in result, got: %s", result)
	}

	// The match text should still be present
	if !strings.Contains(result, "match") {
		t.Errorf("expected 'match' to be in result, got: %s", result)
	}
}

func TestHighlightSelectedMatchFallback(t *testing.T) {
	b := &Browser{}
	text := "simple text"
	term := "text"

	// Selected match with positions that don't match any line should fallback
	selected := SearchMatch{
		LineNum:   99,
		LineText:  "nonexistent",
		CharStart: 999,
		CharEnd:   1004,
	}

	result := b.highlightSelectedMatch(text, term, selected)

	// Should fallback to regular highlighting
	if !strings.Contains(result, "[yellow]") {
		t.Errorf("expected fallback highlighting, got: %s", result)
	}
}

func TestHighlightSelectedMatchEmptyTerm(t *testing.T) {
	b := &Browser{}
	result := b.highlightSelectedMatch("hello", "", SearchMatch{})
	if result != "hello" {
		t.Errorf("expected unchanged text for empty term, got: %s", result)
	}
}

func TestFindSearchMatchesWithFormattingCodes(t *testing.T) {
	b := &Browser{}
	// Text with tview formatting codes
	text := "[::b]bold text[::-] and [red]red text[-]"
	count, contexts := b.findSearchMatches(text, "text", true)

	if count != 2 {
		t.Errorf("expected 2 matches, got %d", count)
	}
	if len(contexts) == 0 {
		t.Error("expected non-empty contexts")
	}
}

func TestHighlightSearchTermSpecialRegexChars(t *testing.T) {
	b := &Browser{}
	// Test with characters that have special regex meaning
	result := b.highlightSearchTerm("price is $10.00", "$10.00", true)
	if !strings.Contains(result, "[yellow]") {
		t.Errorf("expected highlighting with regex-special chars, got: %s", result)
	}
}

func TestHighlightSearchTermMultipleLines(t *testing.T) {
	b := &Browser{}
	text := "hello world\nhello again"
	result := b.highlightSearchTerm(text, "hello", true)

	count := strings.Count(result, "[yellow]")
	if count != 2 {
		t.Errorf("expected 2 highlights across lines, got %d in: %s", count, result)
	}
}
