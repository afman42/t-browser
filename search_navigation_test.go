package main

import (
	"testing"

	"github.com/rivo/tview"
)

func TestNavigateSearchMatchNoMatches(t *testing.T) {
	b := &Browser{}
	b.currentTab().searchTerm = "test"
	b.currentTab().searchMatches = []SearchMatch{}

	b.navigateSearchMatch(1)
	b.navigateSearchMatch(-1)
}

func TestNavigateSearchMatchSingleMatch(t *testing.T) {
	b := &Browser{app: tview.NewApplication()}
	b.currentTab().searchTerm = "test"
	b.currentTab().searchMatches = []SearchMatch{
		{LineNum: 0, LineText: "test line", CharStart: 0, CharEnd: 4},
	}
	b.currentTab().originalContent = "test line"
	b.currentTab().currentMatchIdx = -1

	b.currentTab().textView.SetText("test line")

	b.navigateSearchMatch(1)
	if b.currentTab().currentMatchStart != 0 || b.currentTab().currentMatchEnd != 4 {
		t.Errorf("expected match 0-4, got %d-%d", b.currentTab().currentMatchStart, b.currentTab().currentMatchEnd)
	}

	b.navigateSearchMatch(-1)
	if b.currentTab().currentMatchStart != 0 || b.currentTab().currentMatchEnd != 4 {
		t.Errorf("expected wrap to match 0-4, got %d-%d", b.currentTab().currentMatchStart, b.currentTab().currentMatchEnd)
	}
}

func TestNavigateSearchMatchMultipleMatches(t *testing.T) {
	b := &Browser{app: tview.NewApplication()}
	b.currentTab().searchTerm = "test"
	b.currentTab().searchMatches = []SearchMatch{
		{LineNum: 0, LineText: "test one", CharStart: 0, CharEnd: 4},
		{LineNum: 1, LineText: "test two", CharStart: 10, CharEnd: 14},
	}
	b.currentTab().originalContent = "test one\ntest two"
	b.currentTab().currentMatchIdx = 0

	b.currentTab().textView.SetText("test one\ntest two")

	b.navigateSearchMatch(1)
	if b.currentTab().currentMatchStart != 10 || b.currentTab().currentMatchEnd != 14 {
		t.Errorf("expected second match 10-14, got %d-%d", b.currentTab().currentMatchStart, b.currentTab().currentMatchEnd)
	}

	b.navigateSearchMatch(1)
	if b.currentTab().currentMatchStart != 0 || b.currentTab().currentMatchEnd != 4 {
		t.Errorf("expected wrap to first match 0-4, got %d-%d", b.currentTab().currentMatchStart, b.currentTab().currentMatchEnd)
	}

	b.navigateSearchMatch(-1)
	if b.currentTab().currentMatchStart != 10 || b.currentTab().currentMatchEnd != 14 {
		t.Errorf("expected wrap to second match 10-14, got %d-%d", b.currentTab().currentMatchStart, b.currentTab().currentMatchEnd)
	}
}

func TestNavigateSearchMatchSameLineMultiple(t *testing.T) {
	b := &Browser{app: tview.NewApplication()}
	b.currentTab().searchTerm = "foo"
	b.currentTab().searchMatches = []SearchMatch{
		{LineNum: 0, LineText: "foo bar foo", CharStart: 0, CharEnd: 3},
		{LineNum: 0, LineText: "foo bar foo", CharStart: 8, CharEnd: 11},
	}
	b.currentTab().originalContent = "foo bar foo"
	b.currentTab().currentMatchIdx = 0

	b.currentTab().textView.SetText("foo bar foo")

	b.navigateSearchMatch(1)
	if b.currentTab().currentMatchStart != 8 || b.currentTab().currentMatchEnd != 11 {
		t.Errorf("expected second match 8-11, got %d-%d", b.currentTab().currentMatchStart, b.currentTab().currentMatchEnd)
	}
	if b.currentTab().currentMatchIdx != 1 {
		t.Errorf("expected currentMatchIdx 1, got %d", b.currentTab().currentMatchIdx)
	}

	b.navigateSearchMatch(1)
	if b.currentTab().currentMatchStart != 0 || b.currentTab().currentMatchEnd != 3 {
		t.Errorf("expected wrap to first match 0-3, got %d-%d", b.currentTab().currentMatchStart, b.currentTab().currentMatchEnd)
	}
}

func TestNavigateSearchMatchEmptyTerm(t *testing.T) {
	b := &Browser{}
	b.currentTab().searchTerm = ""
	b.currentTab().searchMatches = []SearchMatch{{LineNum: 0, LineText: "test", CharStart: 0, CharEnd: 4}}

	b.navigateSearchMatch(1)
}

func TestFindSearchMatchesWithPositionsAllMatchesKept(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("hello", true)
	matches := b.findSearchMatchesWithPositions("hello world hello", re)

	if len(matches) != 2 {
		t.Errorf("expected 2 matches (no line dedup), got %d", len(matches))
	}
	if matches[0].CharStart != 0 {
		t.Errorf("first match at 0, got %d", matches[0].CharStart)
	}
	if matches[1].CharStart != 12 {
		t.Errorf("second match at 12, got %d", matches[1].CharStart)
	}
}

func TestFindSearchMatchesWithPositionsCaseInsensitive(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("hello", false)
	matches := b.findSearchMatchesWithPositions("Hello WORLD hello", re)

	if len(matches) < 2 {
		t.Errorf("expected at least 2 matches, got %d", len(matches))
	}
}

func TestFindSearchMatchesWithPositionsSpecialChars(t *testing.T) {
	b := &Browser{}
	re := compileSearchRegex("func", true)
	matches := b.findSearchMatchesWithPositions("func (b *Browser) test()", re)

	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
}

func TestCurrentPosAtLineStartMultipleLines(t *testing.T) {
	text := "line1\nline2\nline3"
	pos := currentPosAtLineStart(text, 1)
	if pos != 6 {
		t.Errorf("expected pos 6, got %d", pos)
	}
}

func TestCurrentPosAtLineStartLastLine(t *testing.T) {
	text := "line1\nline2\nline3"
	pos := currentPosAtLineStart(text, 2)
	if pos != 12 {
		t.Errorf("expected pos 12, got %d", pos)
	}
}

func TestSearchHistorySaveAndCycle(t *testing.T) {
	b := &Browser{}
	tab := b.currentTab()

	tab.searchTerm = "golang"
	b.saveSearchHistory(tab)
	tab.searchTerm = "testing"
	b.saveSearchHistory(tab)
	tab.searchTerm = "golang"
	b.saveSearchHistory(tab)

	if len(tab.searchHistory) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(tab.searchHistory))
	}
	if tab.searchHistory[0] != "testing" {
		t.Errorf("expected 'testing' first, got %q", tab.searchHistory[0])
	}
	if tab.searchHistory[1] != "golang" {
		t.Errorf("expected 'golang' last (moved), got %q", tab.searchHistory[1])
	}

	tab.searchHistoryIndex = -1
	result := b.cycleSearchHistory(tab, 1)
	if result != "testing" {
		t.Errorf("first cycle = %q, want %q", result, "testing")
	}

	result = b.cycleSearchHistory(tab, 1)
	if result != "golang" {
		t.Errorf("second cycle = %q, want %q", result, "golang")
	}

	result = b.cycleSearchHistory(tab, 1)
	if result != "testing" {
		t.Errorf("wrap cycle = %q, want %q", result, "testing")
	}
}

func TestSearchHistoryMaxLimit(t *testing.T) {
	b := &Browser{}
	tab := b.currentTab()

	for i := 0; i < maxSearchHistory+5; i++ {
		tab.searchTerm = "term"
		tab.searchTerm += string(rune('a' + i))
		b.saveSearchHistory(tab)
	}

	if len(tab.searchHistory) > maxSearchHistory {
		t.Errorf("history has %d entries, max is %d", len(tab.searchHistory), maxSearchHistory)
	}
}

func TestSearchCaseSensitiveToggle(t *testing.T) {
	b := &Browser{}
	tab := b.currentTab()

	if !tab.searchCaseSensitive {
		t.Error("default should be case-sensitive")
	}

	tab.searchCaseSensitive = !tab.searchCaseSensitive
	if tab.searchCaseSensitive {
		t.Error("after toggle should be case-insensitive")
	}

	label := b.searchLabel()
	if !contains(label, "case-insensitive") {
		t.Errorf("label should say 'case-insensitive', got: %s", label)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
