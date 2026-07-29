package main

import (
	"strings"
	"testing"
)

func TestHighlightSearchTermCaseSensitive(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("hello", true)
	result := b.highlightSearchTerm("hello world hello", re)

	expected := "[yellow]hello[-] world [yellow]hello[-]"
	if result != expected {
		t.Errorf("highlightSearchTerm = %q, want %q", result, expected)
	}
}

func TestHighlightSearchTermCaseInsensitive(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("hello", false)
	result := b.highlightSearchTerm("Hello HELLO hello", re)

	count := strings.Count(result, "[yellow]")
	if count != 3 {
		t.Errorf("expected 3 highlights, got %d in: %s", count, result)
	}
}

func TestHighlightSearchTermNoMatch(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("xyz", true)
	result := b.highlightSearchTerm("hello world", re)
	if result != "hello world" {
		t.Errorf("expected unchanged text, got: %s", result)
	}
}

func TestHighlightSearchTermEmptyText(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("hello", true)
	result := b.highlightSearchTerm("", re)
	if result != "" {
		t.Errorf("expected empty result, got: %s", result)
	}
}

func TestFindSearchMatchesBasic(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("the", true)
	count, contexts := b.findSearchMatches("the quick brown fox jumps over the lazy dog", re)
	if count != 2 {
		t.Errorf("expected 2 matches, got %d", count)
	}
	if len(contexts) == 0 {
		t.Error("expected non-empty contexts")
	}
}

func TestFindSearchMatchesCaseInsensitive(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("the", false)
	count, _ := b.findSearchMatches("The THE the", re)
	if count != 3 {
		t.Errorf("expected 3 matches for case-insensitive, got %d", count)
	}
}

func TestFindSearchMatchesNoResults(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("xyz", true)
	count, contexts := b.findSearchMatches("hello world", re)
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
	re := compileSearchRegex("hello", true)
	count, _ := b.findSearchMatches("", re)
	if count != 0 {
		t.Errorf("expected 0 matches for empty text, got %d", count)
	}
}

func TestFindSearchMatchesDeduplicatesContexts(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("fox", true)
	_, contexts := b.findSearchMatches("fox and fox", re)
	if len(contexts) != 1 {
		t.Errorf("expected 1 unique context, got %d: %v", len(contexts), contexts)
	}
}

func TestFindSearchMatchesWithPositions(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("hello", true)
	matches := b.findSearchMatchesWithPositions("hello world\nfoo bar", re)

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
	re := compileSearchRegex("foo", true)
	matches := b.findSearchMatchesWithPositions("line one\nfoo bar\nline three", re)

	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}

	m := matches[0]
	if m.LineNum != 1 {
		t.Errorf("expected line 1, got %d", m.LineNum)
	}
	if m.CharStart != 9 {
		t.Errorf("expected CharStart 9, got %d", m.CharStart)
	}
}

func TestFindSearchMatchesWithPositionsMultipleMatchesSameLine(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("foo", true)
	matches := b.findSearchMatchesWithPositions("foo bar foo\nbaz foo", re)

	if len(matches) != 3 {
		t.Errorf("expected 3 matches (all kept, no line dedup), got %d", len(matches))
	}
	if matches[0].CharStart != 0 {
		t.Errorf("first match at 0, got %d", matches[0].CharStart)
	}
	if matches[1].CharStart != 8 {
		t.Errorf("second match at 8, got %d", matches[1].CharStart)
	}
	if matches[2].LineNum != 1 {
		t.Errorf("third match on line 1, got %d", matches[2].LineNum)
	}
}

func TestFindSearchMatchesWithPositionsEmptyTerm(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("", true)
	matches := b.findSearchMatchesWithPositions("hello", re)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for empty term, got %d", len(matches))
	}
}

func TestFindSearchMatchesWithPositionsNoMatch(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("xyz", true)
	matches := b.findSearchMatchesWithPositions("hello", re)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestHighlightSelectedMatch(t *testing.T) {
	b := &Browser{}
	text := "first line\nsecond line with match\nthird line"
	re := compileSearchRegex("match", true)

	selected := SearchMatch{
		LineNum:   1,
		LineText:  "second line with match",
		CharStart: 11 + 1 + 16,
		CharEnd:   11 + 1 + 21,
	}

	result := b.highlightSelectedMatch(text, re, selected)

	if !strings.Contains(result, "[yellow") {
		t.Errorf("expected yellow highlighting in result, got: %s", result)
	}
	if !strings.Contains(result, "match") {
		t.Errorf("expected 'match' to be in result, got: %s", result)
	}
}

func TestHighlightSelectedMatchFallback(t *testing.T) {
	b := &Browser{}
	text := "simple text"
	re := compileSearchRegex("text", true)

	selected := SearchMatch{
		LineNum:   99,
		LineText:  "nonexistent",
		CharStart: 999,
		CharEnd:   1004,
	}

	result := b.highlightSelectedMatch(text, re, selected)

	if !strings.Contains(result, "[yellow]") {
		t.Errorf("expected fallback highlighting, got: %s", result)
	}
}

func TestHighlightSelectedMatchEmptyTerm(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("", true)
	result := b.highlightSelectedMatch("hello", re, SearchMatch{})
	if result != "hello" {
		t.Errorf("expected unchanged text for empty term, got: %s", result)
	}
}

func TestHighlightSelectedMatchCaseSensitive(t *testing.T) {
	b := &Browser{}
	text := "Match match MATCH"
	re := compileSearchRegex("match", true)

	selected := SearchMatch{
		LineNum:   0,
		LineText:  text,
		CharStart: 6,
		CharEnd:   11,
	}

	result := b.highlightSelectedMatch(text, re, selected)

	count := strings.Count(result, "[yellow")
	if count != 1 {
		t.Errorf("case-sensitive: expected 1 highlight, got %d in: %s", count, result)
	}
}

func TestFindSearchMatchesWithFormattingCodes(t *testing.T) {
	b := &Browser{}
	text := "[::b]bold text[::-] and [red]red text[-]"
	re := compileSearchRegex("text", true)
	count, contexts := b.findSearchMatches(text, re)

	if count != 2 {
		t.Errorf("expected 2 matches, got %d", count)
	}
	if len(contexts) == 0 {
		t.Error("expected non-empty contexts")
	}
}

func TestHighlightSearchTermSpecialRegexChars(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("$10.00", true)
	result := b.highlightSearchTerm("price is $10.00", re)
	if !strings.Contains(result, "[yellow]") {
		t.Errorf("expected highlighting with regex-special chars, got: %s", result)
	}
}

func TestHighlightSearchTermMultipleLines(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("hello", true)
	result := b.highlightSearchTerm("hello world\nhello again", re)

	count := strings.Count(result, "[yellow]")
	if count != 2 {
		t.Errorf("expected 2 highlights across lines, got %d in: %s", count, result)
	}
}

func TestCompileSearchRegex(t *testing.T) {
	re := compileSearchRegex("test", true)
	if re == nil {
		t.Fatal("compileSearchRegex returned nil")
	}
	if !re.MatchString("test") {
		t.Error("should match 'test'")
	}
	if re.MatchString("TEST") {
		t.Error("should not match 'TEST' in case-sensitive mode")
	}

	reCI := compileSearchRegex("test", false)
	if !reCI.MatchString("TEST") {
		t.Error("should match 'TEST' in case-insensitive mode")
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"héllo wörld", 5, "héllo..."},
		{"日本語テスト", 3, "日本語..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := truncateRunes(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}
