package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type SearchMatch struct {
	LineNum   int
	LineText  string
	CharStart int
	CharEnd   int
}

const maxSearchHistory = 20

func (b *Browser) startSearch() {
	tab := b.currentTab()
	tab.displayToMatchIndex = make(map[int]int)
	tab.searchHistoryIndex = -1

	inputField := tview.NewInputField().
		SetLabel(b.searchLabel())

	resultsList := tview.NewList()
	resultsList.SetBorder(true)
	resultsList.SetTitle("Search Results")

	searchPanel := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(inputField, 3, 0, true).
		AddItem(resultsList, 0, 1, false)

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.tabBar, 1, 0, false).
		AddItem(tab.textView, 0, 1, false).
		AddItem(searchPanel, 12, 0, true).
		AddItem(b.statusBar, 1, 0, false).
		AddItem(b.urlInput, 3, 0, false)

	runSearch := func(text string) {
		tab.searchTerm = text

		if text == "" {
			tab.textView.SetText(tab.originalContent)
			tab.searchMatches = nil
			inputField.SetLabel(b.searchLabel())
			resultsList.Clear()
			resultsList.AddItem("Type to search...", "", 0, nil)
			return
		}

		re := compileSearchRegex(text, tab.searchCaseSensitive)
		tab.searchMatches = b.findSearchMatchesWithPositions(tab.originalContent, re)
		matchCount := len(tab.searchMatches)

		inputField.SetLabel(fmt.Sprintf("%s - %d matches: ", b.searchLabel(), matchCount))

		resultsList.Clear()
		if matchCount > 0 {
			seenTexts := make(map[string]bool)
			itemIndex := 1
			displayToMatchIndex := make(map[int]int)
			for i, match := range tab.searchMatches {
				cleanText := removeUnwantedCharsFromDisplay(truncateRunes(match.LineText, 50))
				if !seenTexts[cleanText] {
					seenTexts[cleanText] = true
					displayToMatchIndex[itemIndex-1] = i
					resultsList.AddItem(fmt.Sprintf("%d: %s", itemIndex, cleanText), "", 0, nil)
					itemIndex++
				}
			}
			tab.displayToMatchIndex = displayToMatchIndex
		} else {
			resultsList.AddItem("No matches found", "", 0, nil)
		}

		highlightedText := b.highlightSearchTerm(tab.originalContent, re)
		tab.textView.SetText(highlightedText)
	}

	inputField.SetChangedFunc(func(text string) {
		runSearch(text)
	})

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter || key == tcell.KeyEscape {
			b.saveSearchHistory(tab)
			if key == tcell.KeyEscape && tab.searchTerm == "" {
				tab.textView.SetText(tab.originalContent)
			} else if tab.searchTerm != "" {
				re := compileSearchRegex(tab.searchTerm, tab.searchCaseSensitive)
				highlightedText := b.highlightSearchTerm(tab.originalContent, re)
				tab.textView.SetText(highlightedText)
			}
			tab.returningFromSearchResult = false
			b.app.SetRoot(b.mainFlex(), true)
			b.app.SetFocus(tab.textView)
		}
	})

	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTAB:
			b.app.SetFocus(resultsList)
			return nil
		case tcell.KeyUp:
			newText := b.cycleSearchHistory(tab, -1)
			if newText != "" {
				inputField.SetText(newText)
			}
			return nil
		case tcell.KeyDown:
			newText := b.cycleSearchHistory(tab, 1)
			if newText != "" {
				inputField.SetText(newText)
			}
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'i' {
				tab.searchCaseSensitive = !tab.searchCaseSensitive
				runSearch(inputField.GetText())
				return nil
			}
		}
		return event
	})

	resultsList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				currentIdx := resultsList.GetCurrentItem()
				if currentIdx < resultsList.GetItemCount()-1 {
					resultsList.SetCurrentItem(currentIdx + 1)
				}
				b.updateTextForSelectedSearchMatch(resultsList)
				return nil
			case 'k':
				currentIdx := resultsList.GetCurrentItem()
				if currentIdx > 0 {
					resultsList.SetCurrentItem(currentIdx - 1)
				}
				b.updateTextForSelectedSearchMatch(resultsList)
				return nil
			case 'q':
				b.restoreFromSearch()
				return nil
			}
		case tcell.KeyTAB:
			b.app.SetFocus(inputField)
			return nil
		case tcell.KeyEnter:
			displayIndex := resultsList.GetCurrentItem()
			if internalIdx, exists := tab.displayToMatchIndex[displayIndex]; exists && internalIdx < len(tab.searchMatches) {
				tab.returningFromSearchResult = true
				selectedMatch := tab.searchMatches[internalIdx]
				tab.currentMatchStart = selectedMatch.CharStart
				tab.currentMatchEnd = selectedMatch.CharEnd
				tab.currentMatchIdx = internalIdx

				re := compileSearchRegex(tab.searchTerm, tab.searchCaseSensitive)
				highlightedText := b.highlightSelectedMatch(tab.originalContent, re, selectedMatch)
				tab.textView.SetText(highlightedText)
				b.scrollToMatch(selectedMatch)
				b.updateMatchPositionStatus(internalIdx, len(tab.searchMatches))

				b.app.SetRoot(b.mainFlex(), true)
				b.app.SetFocus(tab.textView)
			}
			return nil
		}
		return event
	})

	tab.returningFromSearchResult = false

	if tab.searchTerm != "" {
		inputField.SetText(tab.searchTerm)
	}

	b.app.SetRoot(layout, true)
	b.app.SetFocus(inputField)
	if tab.searchTerm == "" {
		resultsList.AddItem("Type to search...", "", 0, nil)
	}
}

func (b *Browser) searchLabel() string {
	tab := b.currentTab()
	if tab.searchCaseSensitive {
		return "Search (case-sensitive, i to toggle)"
	}
	return "Search (case-insensitive, i to toggle)"
}

func (b *Browser) saveSearchHistory(tab *Tab) {
	term := strings.TrimSpace(tab.searchTerm)
	if term == "" {
		return
	}
	for i, h := range tab.searchHistory {
		if h == term {
			tab.searchHistory = append(tab.searchHistory[:i], tab.searchHistory[i+1:]...)
			break
		}
	}
	tab.searchHistory = append(tab.searchHistory, term)
	if len(tab.searchHistory) > maxSearchHistory {
		tab.searchHistory = tab.searchHistory[len(tab.searchHistory)-maxSearchHistory:]
	}
}

func (b *Browser) cycleSearchHistory(tab *Tab, direction int) string {
	if len(tab.searchHistory) == 0 {
		return ""
	}
	if tab.searchHistoryIndex < 0 || tab.searchHistoryIndex >= len(tab.searchHistory) {
		if direction > 0 {
			tab.searchHistoryIndex = 0
		} else {
			tab.searchHistoryIndex = len(tab.searchHistory) - 1
		}
	} else {
		tab.searchHistoryIndex += direction
		if tab.searchHistoryIndex < 0 {
			tab.searchHistoryIndex = len(tab.searchHistory) - 1
		} else if tab.searchHistoryIndex >= len(tab.searchHistory) {
			tab.searchHistoryIndex = 0
		}
	}
	return tab.searchHistory[tab.searchHistoryIndex]
}

func (b *Browser) updateMatchPositionStatus(current, total int) {
	if b.statusBar == nil {
		return
	}
	tab := b.currentTab()
	b.statusBar.SetText(fmt.Sprintf(" Match %d/%d for '%s' | %d links | %d images",
		current+1, total, tab.searchTerm, len(tab.links), len(tab.images)))
}

func (b *Browser) restoreFromSearch() {
	tab := b.currentTab()
	b.saveSearchHistory(tab)
	if tab.searchTerm != "" {
		re := compileSearchRegex(tab.searchTerm, tab.searchCaseSensitive)
		highlightedText := b.highlightSearchTerm(tab.originalContent, re)
		tab.textView.SetText(highlightedText)
	} else {
		tab.textView.SetText(tab.originalContent)
	}
	tab.returningFromSearchResult = false
	b.app.SetRoot(b.mainFlex(), true)
	b.app.SetFocus(tab.textView)
}

func (b *Browser) navigateSearchMatch(direction int) {
	tab := b.currentTab()
	if tab.searchTerm == "" || len(tab.searchMatches) == 0 {
		return
	}

	currentIdx := tab.currentMatchIdx
	if currentIdx < 0 || currentIdx >= len(tab.searchMatches) {
		if direction > 0 {
			currentIdx = 0
		} else {
			currentIdx = len(tab.searchMatches) - 1
		}
	} else {
		currentIdx += direction
		if currentIdx < 0 {
			currentIdx = len(tab.searchMatches) - 1
		} else if currentIdx >= len(tab.searchMatches) {
			currentIdx = 0
		}
	}

	selectedMatch := tab.searchMatches[currentIdx]
	tab.currentMatchStart = selectedMatch.CharStart
	tab.currentMatchEnd = selectedMatch.CharEnd
	tab.currentMatchIdx = currentIdx

	re := compileSearchRegex(tab.searchTerm, tab.searchCaseSensitive)
	highlightedText := b.highlightSelectedMatch(tab.originalContent, re, selectedMatch)
	tab.textView.SetText(highlightedText)
	b.scrollToMatch(selectedMatch)
	b.updateMatchPositionStatus(currentIdx, len(tab.searchMatches))
}

func (b *Browser) updateTextForSelectedSearchMatch(resultsList *tview.List) {
	tab := b.currentTab()
	displayIndex := resultsList.GetCurrentItem()
	if internalIdx, exists := tab.displayToMatchIndex[displayIndex]; exists && internalIdx < len(tab.searchMatches) {
		selectedMatch := tab.searchMatches[internalIdx]
		re := compileSearchRegex(tab.searchTerm, tab.searchCaseSensitive)
		highlightedText := b.highlightSelectedMatch(tab.originalContent, re, selectedMatch)
		tab.textView.SetText(highlightedText)
		b.scrollToMatch(selectedMatch)
		b.updateMatchPositionStatus(internalIdx, len(tab.searchMatches))
	}
}
