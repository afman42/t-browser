package main

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
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
func searchBrowser() *Browser {
	b := testBrowserWithUI()
	tab := b.currentTab()
	tab.originalContent = "line one alpha\nline two beta alpha\ngamma line"
	tab.searchMatches = []SearchMatch{
		{LineNum: 0, LineText: "line one alpha", CharStart: 9, CharEnd: 14},
		{LineNum: 1, LineText: "line two beta alpha", CharStart: 18, CharEnd: 23},
	}
	tab.displayToMatchIndex = map[int]int{0: 0, 1: 1}
	return b
}

func TestStartSearchPreparesState(t *testing.T) {
	b := searchBrowser()
	tab := b.currentTab()

	b.startSearch()

	if tab.searchHistoryIndex != -1 {
		t.Errorf("searchHistoryIndex = %d, want -1", tab.searchHistoryIndex)
	}
	if tab.displayToMatchIndex == nil {
		t.Error("displayToMatchIndex should be initialized")
	}
	if tab.returningFromSearchResult {
		t.Error("returningFromSearchResult should reset to false")
	}
}

func TestSearchLabel(t *testing.T) {
	tab := newTab()
	b := &Browser{tabs: []*Tab{tab}}

	tab.searchCaseSensitive = true
	if !strings.Contains(b.searchLabel(), "case-sensitive") {
		t.Errorf("label = %q, want case-sensitive hint", b.searchLabel())
	}
	tab.searchCaseSensitive = false
	if !strings.Contains(b.searchLabel(), "case-insensitive") {
		t.Errorf("label = %q, want case-insensitive hint", b.searchLabel())
	}
}

func TestSaveSearchHistoryDedupAndCap(t *testing.T) {
	b := &Browser{}
	tab := newTab()
	tab.searchTerm = "alpha"
	b.saveSearchHistory(tab)
	b.saveSearchHistory(tab)
	if len(tab.searchHistory) != 1 {
		t.Errorf("duplicate terms must collapse, got %v", tab.searchHistory)
	}

	// Re-saved term moves to the end.
	tab.searchTerm = "beta"
	b.saveSearchHistory(tab)
	tab.searchTerm = "alpha"
	b.saveSearchHistory(tab)
	if tab.searchHistory[len(tab.searchHistory)-1] != "alpha" {
		t.Errorf("re-added term should move to the end: %v", tab.searchHistory)
	}

	// Empty term is ignored.
	tab.searchTerm = "  "
	n := len(tab.searchHistory)
	b.saveSearchHistory(tab)
	if len(tab.searchHistory) != n {
		t.Error("blank term must not be saved")
	}

	// History caps at maxSearchHistory.
	for i := 0; i < maxSearchHistory+5; i++ {
		tab.searchTerm = string(rune('a'+i%26)) + "term"
		b.saveSearchHistory(tab)
	}
	if len(tab.searchHistory) > maxSearchHistory {
		t.Errorf("history length %d exceeds cap %d", len(tab.searchHistory), maxSearchHistory)
	}
}

func TestCycleSearchHistory(t *testing.T) {
	tab := newTab()
	b := &Browser{tabs: []*Tab{tab}}

	if got := b.cycleSearchHistory(tab, 1); got != "" {
		t.Errorf("empty history should return empty, got %q", got)
	}

	tab.searchHistory = []string{"a", "b", "c"}
	if got := b.cycleSearchHistory(tab, 1); got != "a" {
		t.Errorf("first cycle up = %q, want a", got)
	}
	if got := b.cycleSearchHistory(tab, 1); got != "b" {
		t.Errorf("second cycle up = %q, want b", got)
	}
	if got := b.cycleSearchHistory(tab, -1); got != "a" {
		t.Errorf("cycle down = %q, want a", got)
	}
	// Wrap upward past the end.
	tab.searchHistoryIndex = len(tab.searchHistory) - 1
	if got := b.cycleSearchHistory(tab, 1); got != "a" {
		t.Errorf("wrap up = %q, want a", got)
	}
}

func TestUpdateMatchPositionStatus(t *testing.T) {
	b := searchBrowser()
	b.currentTab().searchTerm = "alpha"
	b.currentTab().links = []Link{{URL: "x", Text: "y"}}
	b.currentTab().images = []Image{{URL: "https://e/i.png"}}

	b.updateMatchPositionStatus(0, 2)
	got := b.statusBar.GetText(false)
	if !strings.Contains(got, "Match 1/2") {
		t.Errorf("status = %q, want Match 1/2", got)
	}
	if !strings.Contains(got, "alpha") {
		t.Errorf("status should contain the search term: %q", got)
	}
	if !strings.Contains(got, "1 links | 1 images") {
		t.Errorf("status should include counts: %q", got)
	}

	// nil statusBar is a no-op.
	(&Browser{}).updateMatchPositionStatus(0, 2)
}

func TestRestoreFromSearch(t *testing.T) {
	b := searchBrowser()
	tab := b.currentTab()
	tab.searchTerm = ""

	b.restoreFromSearch()
	if got := tab.textView.GetText(false); got != tab.originalContent {
		t.Error("empty term restore should show original content")
	}

	tab.searchTerm = "alpha"
	b.restoreFromSearch()
	if got := tab.textView.GetText(false); !strings.Contains(got, "alpha") {
		t.Errorf("restore with term should show highlighted content: %q", got)
	}
	if tab.returningFromSearchResult {
		t.Error("returningFromSearchResult should be false after restore")
	}
}

func TestNavigateSearchMatch(t *testing.T) {
	b := searchBrowser()
	tab := b.currentTab()
	tab.searchTerm = "alpha"
	tab.currentMatchIdx = -1

	b.navigateSearchMatch(1)
	if tab.currentMatchIdx != 0 {
		t.Errorf("first navigate = %d, want 0", tab.currentMatchIdx)
	}
	b.navigateSearchMatch(1)
	if tab.currentMatchIdx != 1 {
		t.Errorf("second navigate = %d, want 1", tab.currentMatchIdx)
	}
	// Wrap forward.
	b.navigateSearchMatch(1)
	if tab.currentMatchIdx != 0 {
		t.Errorf("wrap forward = %d, want 0", tab.currentMatchIdx)
	}
	// No matches → no-op.
	noMatch := &Browser{tabs: []*Tab{newTab()}}
	noMatch.currentTab().searchTerm = "zzz"
	noMatch.currentTab().searchMatches = nil
	noMatch.navigateSearchMatch(1)
	noMatch.currentTab().searchTerm = ""
	noMatch.navigateSearchMatch(-1)
}

func TestUpdateTextForSelectedSearchMatch(t *testing.T) {
	b := searchBrowser()
	tab := b.currentTab()
	tab.searchTerm = "alpha"
	list := tview.NewList()
	list.AddItem("item 0", "", 0, nil)
	list.AddItem("item 1", "", 0, nil)
	list.SetCurrentItem(1)

	b.updateTextForSelectedSearchMatch(list)
	if got := b.statusBar.GetText(false); !strings.Contains(got, "Match 2/2") {
		t.Errorf("status = %q, want Match 2/2", got)
	}
}
