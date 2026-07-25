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
	b.displayToMatchIndex = make(map[int]int)

	// Create a modal search with input field and results list
	inputField := tview.NewInputField().
		SetLabel("Real-time search (case-sensitive): ").
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter || key == tcell.KeyEscape {
				// Return to main view when Enter or Escape is pressed
				// Only restore original content if we're not returning from a search result selection
				if !b.returningFromSearchResult && b.searchTerm == "" {
					b.textView.SetText(b.originalContent)
				} else if b.returningFromSearchResult && b.searchTerm != "" {
					// If returning from selection and search term exists, maintain highlighting
					highlightedText := b.highlightSearchTerm(b.originalContent, b.searchTerm, true)
					b.textView.SetText(highlightedText)
				}

				// Reset the flag
				b.returningFromSearchResult = false

				// Restore the proper flex layout with URL input
				flex := tview.NewFlex().
					SetDirection(tview.FlexRow).
					AddItem(b.textView, 0, 1, false).  // Main content area - takes remaining space
					AddItem(b.urlInput, 3, 0, false)   // URL input at the bottom - fixed height of 3

				b.app.SetRoot(flex, true)
				b.app.SetFocus(b.textView)  // Ensure content view has focus after search
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
		b.searchTerm = text
		// By default, perform case-sensitive search
		matchCount, _ := b.performSearchWithMatches(text, true)

		// Find matches with context for the list
		b.searchMatches = b.findSearchMatchesWithPositions(b.originalContent, text, true)

		// Update the label to show match count
		if text != "" {
			inputField.SetLabel(fmt.Sprintf("Real-time search (case-sensitive) - %d matches: ", matchCount))
		} else {
			inputField.SetLabel("Real-time search (case-sensitive): ")
		}

		// Clear the results list
		resultsList.Clear()

		// Update results list with unique match titles
		if len(b.searchMatches) > 0 {
			// Use a map to track which line text has already been added to avoid duplicates
			seenTexts := make(map[string]bool)
			itemIndex := 1 // Start from 1 for display purposes
			// Create a mapping to track which internal match index corresponds to each displayed item
			displayToMatchIndex := make(map[int]int)
			for i, match := range b.searchMatches {
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
			b.displayToMatchIndex = displayToMatchIndex
		} else {
			resultsList.AddItem("No matches found", "", 0, nil)
		}

		// If we're returning from a search result selection, highlight all matches
		if b.returningFromSearchResult && text != "" {
			highlightedText := b.highlightSearchTerm(b.originalContent, text, true)
			b.textView.SetText(highlightedText)
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
			if internalIdx, exists := b.displayToMatchIndex[displayIndex]; exists && internalIdx < len(b.searchMatches) {
				// Set flag to indicate we're returning from a search result selection
				b.returningFromSearchResult = true

				// Store the selected match to highlight in the main content
				selectedMatch := b.searchMatches[internalIdx]

				// Create highlighted text where the specific selected match has a different highlight
				highlightedText := b.highlightSelectedMatch(b.originalContent, b.searchTerm, selectedMatch)
				b.textView.SetText(highlightedText)

				// Scroll to the position of the selected match
				b.scrollToMatch(selectedMatch)

				// Return to main view after navigating
				flex := tview.NewFlex().
					SetDirection(tview.FlexRow).
					AddItem(b.textView, 0, 1, false).  // Main content area - takes remaining space
					AddItem(b.urlInput, 3, 0, false)   // URL input at the bottom - fixed height of 3

				b.app.SetRoot(flex, true)
				b.app.SetFocus(b.textView)
			}
			return nil
		}

		return event
	})

	// Reset the flag when starting search
	b.returningFromSearchResult = false

	// Set focus to the input field when starting search
	b.app.SetRoot(layout, true)
	b.app.SetFocus(inputField)
}

// restoreFromSearch restores the main content view after search is dismissed.
func (b *Browser) restoreFromSearch() {
	if !b.returningFromSearchResult && b.searchTerm == "" {
		b.textView.SetText(b.originalContent)
	} else if b.returningFromSearchResult && b.searchTerm != "" {
		highlightedText := b.highlightSearchTerm(b.originalContent, b.searchTerm, true)
		b.textView.SetText(highlightedText)
	}
	b.returningFromSearchResult = false

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.textView, 0, 1, false).
		AddItem(b.urlInput, 3, 0, false)
	b.app.SetRoot(flex, true)
	b.app.SetFocus(b.textView)
}

// updateTextForSelectedSearchMatch highlights the currently selected search match
// in the results list and scrolls to it. Used by both j/k key handlers.
func (b *Browser) updateTextForSelectedSearchMatch(resultsList *tview.List) {
	displayIndex := resultsList.GetCurrentItem()
	if internalIdx, exists := b.displayToMatchIndex[displayIndex]; exists && internalIdx < len(b.searchMatches) {
		selectedMatch := b.searchMatches[internalIdx]
		highlightedText := b.highlightSelectedMatch(b.originalContent, b.searchTerm, selectedMatch)
		b.textView.SetText(highlightedText)
		b.scrollToMatch(selectedMatch)
	}
}

// performSearch performs the text search in the current page
func (b *Browser) performSearch(term string, caseSensitive bool) {
	b.performSearchWithMatches(term, caseSensitive)
}

// performSearchWithMatches performs the text search and returns the match count and matches
func (b *Browser) performSearchWithMatches(term string, caseSensitive bool) (int, []string) {
	text := b.originalContent

	if term == "" {
		b.textView.SetText(text)
		b.searchTerm = ""
		b.textView.ScrollToBeginning()
		return 0, []string{}
	}

	matchCount, matches := b.findSearchMatches(text, term, caseSensitive)
	highlightedText := b.highlightSearchTerm(text, term, caseSensitive)

	b.textView.SetText(highlightedText)
	b.textView.ScrollToBeginning()

	return matchCount, matches
}