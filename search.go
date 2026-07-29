package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SearchMatch holds information about each search match
type SearchMatch struct {
	LineNum   int
	LineText  string
	CharStart int
	CharEnd   int
}

// startSearch starts the search functionality
func (b *Browser) startSearch() {
	// Initialize the mapping
	b.currentTab().displayToMatchIndex = make(map[int]int)

	// Create a modal search with input field and results list
	inputField := tview.NewInputField().
		SetLabel("Real-time search (case-sensitive): ").
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter || key == tcell.KeyEscape {
				// Return to main view when Enter or Escape is pressed
				// Only restore original content if we're not returning from a search result selection
				if !b.currentTab().returningFromSearchResult && b.currentTab().searchTerm == "" {
					b.currentTab().textView.SetText(b.currentTab().originalContent)
				} else if b.currentTab().returningFromSearchResult && b.currentTab().searchTerm != "" {
					// If returning from selection and search term exists, maintain highlighting
					highlightedText := b.highlightSearchTerm(b.currentTab().originalContent, b.currentTab().searchTerm, true)
					b.currentTab().textView.SetText(highlightedText)
				}

				// Reset the flag
				b.currentTab().returningFromSearchResult = false

				// Restore the proper flex layout with URL input
				b.app.SetRoot(b.mainFlex(), true)
				b.app.SetFocus(b.currentTab().textView) // Ensure content view has focus after search
			}
		})

	// Create a list for search results
	resultsList := tview.NewList()
	resultsList.SetBorder(true)
	resultsList.SetTitle("Search Results")

	// Layout container for both input and results
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(inputField, 3, 1, true).  // Input field takes 3 lines
		AddItem(resultsList, 0, 1, false) // Results list takes remaining space

	// Set up real-time search
	inputField.SetChangedFunc(func(text string) {
		// Update search in real-time as user types
		b.currentTab().searchTerm = text
		// By default, perform case-sensitive search
		matchCount, _ := b.performSearchWithMatches(text, true)

		// Find matches with context for the list
		b.currentTab().searchMatches = b.findSearchMatchesWithPositions(b.currentTab().originalContent, text, true)

		// Update the label to show match count
		if text != "" {
			inputField.SetLabel(fmt.Sprintf("Real-time search (case-sensitive) - %d matches: ", matchCount))
		} else {
			inputField.SetLabel("Real-time search (case-sensitive): ")
		}

		// Clear the results list
		resultsList.Clear()

		// Update results list with unique match titles
		if len(b.currentTab().searchMatches) > 0 {
			// Use a map to track which line text has already been added to avoid duplicates
			seenTexts := make(map[string]bool)
			itemIndex := 1 // Start from 1 for display purposes
			// Create a mapping to track which internal match index corresponds to each displayed item
			displayToMatchIndex := make(map[int]int)
			for i, match := range b.currentTab().searchMatches {
				// Truncate the line text for display if it's too long
				displayText := match.LineText
				if len(displayText) > 50 {
					displayText = displayText[:50] + "..."
				}
				// Remove any existing tview formatting codes and unwanted Unicode characters from the display text
				cleanText := removeUnwantedCharsFromDisplay(displayText)
				// Only add if this text hasn't been added before
				if !seenTexts[cleanText] {
					seenTexts[cleanText] = true
					displayToMatchIndex[itemIndex-1] = i // Store the mapping (0-indexed for internal match)
					// Add the search result with a prefix showing the index
					resultsList.AddItem(fmt.Sprintf("%d: %s", itemIndex, cleanText), "", rune('0'+(itemIndex%10)), nil)
					itemIndex++
				}
			}

			// Store the mapping for later use in navigation
			b.currentTab().displayToMatchIndex = displayToMatchIndex
		} else {
			resultsList.AddItem("No matches found", "", 0, nil)
		}

		// If we're returning from a search result selection, highlight all matches
		if b.currentTab().returningFromSearchResult && text != "" {
			highlightedText := b.highlightSearchTerm(b.currentTab().originalContent, text, true)
			b.currentTab().textView.SetText(highlightedText)
		}
	})

	// Set up input capture for the input field to handle Tab key
	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			// Switch focus to the results list
			b.app.SetFocus(resultsList)
			return nil // Consume the event
		}
		return event
	})

	// Set up input capture for the results list to handle j/k navigation and Tab key
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
			// Switch focus to the input field
			b.app.SetFocus(inputField)
			return nil // Consume the event
		case tcell.KeyEnter:
			// Get the selected item and navigate to it in the content
			displayIndex := resultsList.GetCurrentItem()
			if internalIdx, exists := b.currentTab().displayToMatchIndex[displayIndex]; exists && internalIdx < len(b.currentTab().searchMatches) {
				// Set flag to indicate we're returning from a search result selection
				b.currentTab().returningFromSearchResult = true

				// Store the selected match to highlight in the main content
				selectedMatch := b.currentTab().searchMatches[internalIdx]

				// Create highlighted text where the specific selected match has a different highlight
				highlightedText := b.highlightSelectedMatch(b.currentTab().originalContent, b.currentTab().searchTerm, selectedMatch)
				b.currentTab().textView.SetText(highlightedText)

				// Scroll to the position of the selected match
				b.scrollToMatch(selectedMatch)

				// Return to main view after navigating
				b.app.SetRoot(b.mainFlex(), true)
				b.app.SetFocus(b.currentTab().textView)
			}
			return nil
		}

		return event
	})

	// Reset the flag when starting search
	b.currentTab().returningFromSearchResult = false

	// Set focus to the input field when starting search
	b.app.SetRoot(layout, true)
	b.app.SetFocus(inputField)
}

// restoreFromSearch restores the main content view after search is dismissed.
func (b *Browser) restoreFromSearch() {
	if !b.currentTab().returningFromSearchResult && b.currentTab().searchTerm == "" {
		b.currentTab().textView.SetText(b.currentTab().originalContent)
	} else if b.currentTab().returningFromSearchResult && b.currentTab().searchTerm != "" {
		highlightedText := b.highlightSearchTerm(b.currentTab().originalContent, b.currentTab().searchTerm, true)
		b.currentTab().textView.SetText(highlightedText)
	}
	b.currentTab().returningFromSearchResult = false

	b.app.SetRoot(b.mainFlex(), true)
	b.app.SetFocus(b.currentTab().textView)
}

// navigateSearchMatch moves to the next or previous search match
func (b *Browser) navigateSearchMatch(direction int) {
	tab := b.currentTab()
	if tab.searchTerm == "" || len(tab.searchMatches) == 0 {
		return
	}

	currentIdx := -1
	for i, match := range tab.searchMatches {
		if match.CharStart == tab.currentMatchStart && match.CharEnd == tab.currentMatchEnd {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
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

	highlightedText := b.highlightSelectedMatch(tab.originalContent, tab.searchTerm, selectedMatch)
	tab.textView.SetText(highlightedText)
	b.scrollToMatch(selectedMatch)
}

// updateTextForSelectedSearchMatch highlights the currently selected search match
// in the results list and scrolls to it. Used by both j/k key handlers.
func (b *Browser) updateTextForSelectedSearchMatch(resultsList *tview.List) {
	tab := b.currentTab()
	displayIndex := resultsList.GetCurrentItem()
	if internalIdx, exists := tab.displayToMatchIndex[displayIndex]; exists && internalIdx < len(tab.searchMatches) {
		selectedMatch := tab.searchMatches[internalIdx]
		highlightedText := b.highlightSelectedMatch(tab.originalContent, tab.searchTerm, selectedMatch)
		tab.textView.SetText(highlightedText)
		b.scrollToMatch(selectedMatch)
	}
}

// performSearch performs the text search in the current page
func (b *Browser) performSearch(term string, caseSensitive bool) {
	b.performSearchWithMatches(term, caseSensitive)
}

// performSearchWithMatches performs the text search and returns the match count and matches
func (b *Browser) performSearchWithMatches(term string, caseSensitive bool) (int, []string) {
	tab := b.currentTab()
	text := tab.originalContent

	if term == "" {
		tab.textView.SetText(text)
		tab.searchTerm = ""
		tab.textView.ScrollToBeginning()
		return 0, []string{}
	}

	matchCount, matches := b.findSearchMatches(text, term, caseSensitive)
	highlightedText := b.highlightSearchTerm(text, term, caseSensitive)

	tab.textView.SetText(highlightedText)
	tab.textView.ScrollToBeginning()

	return matchCount, matches
}
