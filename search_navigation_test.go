package main

import (
	"testing"

	"github.com/rivo/tview"
)

func TestNavigateSearchMatchNoMatches(t *testing.T) {
	b := &Browser{}
	b.currentTab().searchTerm = "test"
	b.currentTab().searchMatches = []SearchMatch{}

	// Should not panic
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

	b.currentTab().textView.SetText("test one\ntest two")

	// Start from first
	b.currentTab().currentMatchStart = 0
	b.currentTab().currentMatchEnd = 4

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

func TestNavigateSearchMatchEmptyTerm(t *testing.T) {
	b := &Browser{}
	b.currentTab().searchTerm = ""
	b.currentTab().searchMatches = []SearchMatch{{LineNum: 0, LineText: "test", CharStart: 0, CharEnd: 4}}

	b.navigateSearchMatch(1) // Should not panic
}

func TestFindSearchMatchesWithPositionsDeduplicates(t *testing.T) {
	b := &Browser{}
	text := "hello world hello"
	term := "hello"

	matches := b.findSearchMatchesWithPositions(text, term, true)
	if len(matches) != 1 {
		t.Errorf("expected 1 unique match (same line), got %d", len(matches))
	}
	if matches[0].LineNum != 0 {
		t.Errorf("expected line 0, got %d", matches[0].LineNum)
	}
}

func TestFindSearchMatchesWithPositionsCaseInsensitive(t *testing.T) {
	b := &Browser{}
	text := "Hello WORLD hello"
	term := "hello"

	matches := b.findSearchMatchesWithPositions(text, term, false)
	// The function deduplicates by line, so matches on same line get collapsed
	if len(matches) < 1 {
		t.Errorf("expected at least 1 match, got %d", len(matches))
	}
}

func TestFindSearchMatchesWithPositionsSpecialChars(t *testing.T) {
	b := &Browser{}
	text := "func (b *Browser) test()"
	term := "func"

	matches := b.findSearchMatchesWithPositions(text, term, true)
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
}

func TestCurrentPosAtLineStartMultipleLines(t *testing.T) {
	text := "line1\nline2\nline3"
	pos := currentPosAtLineStart(text, 1)
	if pos != 6 {
		t.Errorf("expected pos 6 (len line1 + newline), got %d", pos)
	}
}

func TestCurrentPosAtLineStartLastLine(t *testing.T) {
	text := "line1\nline2\nline3"
	pos := currentPosAtLineStart(text, 2)
	if pos != 12 {
		t.Errorf("expected pos 12, got %d", pos)
	}
}
