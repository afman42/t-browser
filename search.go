package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// GetHighlightColor returns a colored string using fatih/color for terminal output
func GetHighlightColor(text string) string {
	// Example usage of fatih/color for terminal output (though not used in tview)
	// This demonstrates integration of fatih/color library
	return color.YellowString(text)
}

// SearchMatch holds information about each search match
type SearchMatch struct {
	LineNum   int
	LineText  string
	CharStart int
	CharEnd   int
}

// startSearch starts the search functionality
func (b *Browser) startSearch() {
	// Create a modal search with input field and results list
	inputField := tview.NewInputField().
		SetLabel("Real-time search (case-sensitive): ").
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter || key == tcell.KeyEscape {
				// Return to main view when Enter or Escape is pressed
				// Only restore original content if we're not returning from a search result selection
				if !b.returningFromSearchResult {
					b.textView.SetText(b.originalContent)
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

		// Update results list with matching text
		if len(b.searchMatches) > 0 {
			for i, match := range b.searchMatches {
				// Truncate the line text for display if it's too long
				displayText := match.LineText
				if len(displayText) > 50 {
					displayText = displayText[:50] + "..."
				}
				// Add the search result with a prefix showing the index
				resultsList.AddItem(fmt.Sprintf("%d: %s", i+1, displayText), "", rune('0'+(i+1)%10), nil)
			}
		} else {
			resultsList.AddItem("No matches found", "", 0, nil)
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
			case 'j': // Move down in the list
				currentIdx := resultsList.GetCurrentItem()
				if currentIdx < resultsList.GetItemCount()-1 {
					resultsList.SetCurrentItem(currentIdx + 1)
				}
				return nil
			case 'k': // Move up in the list
				currentIdx := resultsList.GetCurrentItem()
				if currentIdx > 0 {
					resultsList.SetCurrentItem(currentIdx - 1)
				}
				return nil
			case 'q': // Quit search
				// Only restore original content if we're not returning from a search result selection
				if !b.returningFromSearchResult {
					b.textView.SetText(b.originalContent)
				}

				// Reset the flag
				b.returningFromSearchResult = false

				flex := tview.NewFlex().
					SetDirection(tview.FlexRow).
					AddItem(b.textView, 0, 1, false).  // Main content area - takes remaining space
					AddItem(b.urlInput, 3, 0, false)   // URL input at the bottom - fixed height of 3

				b.app.SetRoot(flex, true)
				b.app.SetFocus(b.textView)  // Ensure content view has focus after search
				return nil
			}
		case tcell.KeyTAB:
			// Switch focus to the input field
			b.app.SetFocus(inputField)
			return nil // Consume the event
		case tcell.KeyEnter:
			// Get the selected item and navigate to it in the content
			currentIdx := resultsList.GetCurrentItem()
			if currentIdx >= 0 && currentIdx < len(b.searchMatches) {
				// Set flag to indicate we're returning from a search result selection
				b.returningFromSearchResult = true

				// Store the selected match to highlight in the main content
				selectedMatch := b.searchMatches[currentIdx]

				// Create highlighted text where the specific selected match has a different highlight
				highlightedText := b.highlightSelectedMatch(b.originalContent, b.searchTerm, selectedMatch)
				b.textView.SetText(highlightedText)

				// Return to main view after navigating
				flex := tview.NewFlex().
					SetDirection(tview.FlexRow).
					AddItem(b.textView, 0, 1, false).  // Main content area - takes remaining space
					AddItem(b.urlInput, 3, 0, false)   // URL input at the bottom - fixed height of 3

				b.app.SetRoot(flex, true)

				// Set focus to the text view and ensure the selected match is prominent
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

// performSearch performs the text search in the current page
func (b *Browser) performSearch(term string, caseSensitive bool) {
	// Use the original content for searching to avoid searching highlighted text
	b.performSearchWithMatches(term, caseSensitive)
}

// performSearchWithMatches performs the text search and returns the match count and matches
func (b *Browser) performSearchWithMatches(term string, caseSensitive bool) (int, []string) {
	// Use the original content for searching to avoid searching highlighted text
	text := b.originalContent

	// If term is empty, just show original content without highlighting
	if term == "" {
		b.textView.SetText(text)
		b.searchTerm = ""
		b.textView.ScrollToBeginning()
		return 0, []string{}
	}

	// Get matches before highlighting
	matchCount, matches := b.findSearchMatches(text, term, caseSensitive)

	// Find occurrences and highlight them
	highlightedText := b.highlightSearchTerm(text, term, caseSensitive)

	// Update the text view with highlighted content
	b.textView.SetText(highlightedText)
	b.textView.ScrollToBeginning()

	return matchCount, matches
}

// findSearchMatches finds all matches and returns count and text contexts
func (b *Browser) findSearchMatches(text, term string, caseSensitive bool) (int, []string) {
	if term == "" || strings.TrimSpace(term) == "" {
		return 0, []string{}
	}
	
	// Escape special regex characters in the search term to prevent regex injection
	escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))
	
	var re *regexp.Regexp
	if caseSensitive {
		// Create a case-sensitive regex to find the search term
		re = regexp.MustCompile(escapedTerm)
	} else {
		// Create a case-insensitive regex to find the search term
		re = regexp.MustCompile("(?i)" + escapedTerm)
	}
	
	// Find all match locations (start and end indices)
	indices := re.FindAllStringIndex(text, -1)
	
	if indices == nil {
		return 0, []string{}
	}
	
	// Get surrounding context for each match
	var contexts []string
	seen := make(map[string]bool) // To avoid duplicate contexts
	
	for _, loc := range indices {
		start, end := loc[0], loc[1]

		// Expand backward to find word start, being careful about formatting codes
		wordStart := start
		for wordStart > 0 {
			char := text[wordStart-1]
			if char == ' ' || char == '\n' || char == '\t' ||
				char == '.' || char == ',' || char == '!' ||
				char == '?' || char == ';' || char == ':' {
				break
			}
			// Stop if we encounter a potential formatting bracket
			if char == '[' {
				// Look ahead to see if this is a formatting code and adjust accordingly
				break
			}
			wordStart--
		}

		// Expand forward to find word end
		wordEnd := end
		for wordEnd < len(text) {
			char := text[wordEnd]
			if char == ' ' || char == '\n' || char == '\t' ||
				char == '.' || char == ',' || char == '!' ||
				char == '?' || char == ';' || char == ':' {
				break
			}
			// Stop if we encounter a potential formatting bracket
			if char == '[' {
				break
			}
			wordEnd++
		}

		// Extract the specific word or phrase that contains the match
		word := text[wordStart:wordEnd]

		// Remove any existing tview formatting codes from the extracted word
		plainWord := removeTviewFormatting(word)

		// Add to results if not a duplicate
		if !seen[plainWord] {
			seen[plainWord] = true
			contexts = append(contexts, plainWord)
		}
	}

	return len(indices), contexts
}

// highlightSearchTerm highlights search terms in the text using tview color format
// but using color definitions from our color management system based on fatih/color
func (b *Browser) highlightSearchTerm(text, term string, caseSensitive bool) string {
	if term == "" || strings.TrimSpace(term) == "" {
		return text
	}

	// Escape special regex characters in the search term to prevent regex injection
	escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))

	var re *regexp.Regexp
	if caseSensitive {
		// Create a case-sensitive regex to find the search term
		re = regexp.MustCompile(escapedTerm)
	} else {
		// Create a case-insensitive regex to find the search term
		re = regexp.MustCompile("(?i)" + escapedTerm)
	}

	// Replace only the matching terms with highlighted versions
	// Using yellow as the highlight color via our color management system
	highlighted := re.ReplaceAllStringFunc(text, func(match string) string {
		colorCode := ColorToTviewFormat("yellow")
		return fmt.Sprintf("[%s]%s[-]", colorCode, match)
	})

	return highlighted
}

// highlightSearchTermWithColor highlights search terms using a specific color from our color system
func (b *Browser) highlightSearchTermWithColor(text, term string, caseSensitive bool, colorName string) string {
	if term == "" || strings.TrimSpace(term) == "" {
		return text
	}

	// Escape special regex characters in the search term to prevent regex injection
	escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))

	var re *regexp.Regexp
	if caseSensitive {
		// Create a case-sensitive regex to find the search term
		re = regexp.MustCompile(escapedTerm)
	} else {
		// Create a case-insensitive regex to find the search term
		re = regexp.MustCompile("(?i)" + escapedTerm)
	}

	// Use our color management system to convert color name to tview format
	tviewColor := ColorToTviewFormat(colorName)

	// Replace only the matching terms with highlighted versions
	highlighted := re.ReplaceAllStringFunc(text, func(match string) string {
		return fmt.Sprintf("[%s]%s[-]", tviewColor, match)
	})

	return highlighted
}

// highlightSelectedMatch highlights the search term differently for the selected match vs other matches
func (b *Browser) highlightSelectedMatch(text, term string, selectedMatch SearchMatch) string {
	if term == "" || strings.TrimSpace(term) == "" {
		return text
	}

	// Find all matches first, before applying any formatting
	escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))
	re := regexp.MustCompile("(?i)" + escapedTerm) // Case insensitive

	// Find all match positions
	indices := re.FindAllStringIndex(text, -1)

	if indices == nil {
		return text
	}

	// Find which index in the regex result corresponds to our selected match
	selectedIndex := -1
	for i, idx := range indices {
		start, end := idx[0], idx[1]
		// Check if this match corresponds to our selected match by comparing position
		if start == selectedMatch.CharStart && end == selectedMatch.CharEnd {
			selectedIndex = i
			break
		}
	}

	// If we couldn't find the match in our new search, default to highlighting the first occurrence
	if selectedIndex == -1 {
		// Try to find by matching the text content instead
		// Make sure the selectedMatch positions are valid
		if selectedMatch.CharStart >= 0 && selectedMatch.CharEnd <= len(text) {
			selectedText := text[selectedMatch.CharStart:selectedMatch.CharEnd]
			for i, idx := range indices {
				start, end := idx[0], idx[1]
				if text[start:end] == selectedText {
					selectedIndex = i
					break
				}
			}
		}
	}

	// Apply highlighting: regular for all matches except the selected one
	var result strings.Builder
	lastEnd := 0
	matchIndex := 0

	for _, idx := range indices {
		start, end := idx[0], idx[1]

		// Add text before this match
		result.WriteString(text[lastEnd:start])

		// Get the matched text
		matchText := text[start:end]

		// Check if this is the selected match by index
		if matchIndex == selectedIndex {
			// This is the selected match - use highly distinctive highlighting
			// Use black text on yellow background with bold to make it extremely visible
			// Format: foreground:background:attributes
			selectedColorCode := ColorToTviewFormat("black") + ":" + ColorToTviewFormat("yellow") + ":" + ColorToTviewFormat("bold")
			result.WriteString(fmt.Sprintf("[%s]%s[-]", selectedColorCode, matchText))
		} else {
			// Regular match - use normal highlighting
			regularColorCode := ColorToTviewFormat("yellow")
			result.WriteString(fmt.Sprintf("[%s]%s[-]", regularColorCode, matchText))
		}

		lastEnd = end
		matchIndex++
	}

	// Add remaining text after the last match
	result.WriteString(text[lastEnd:])

	return result.String()
}

// removeTviewFormatting removes tview formatting codes from text like [::b], [yellow], etc.
func removeTviewFormatting(text string) string {
	// Regex to match tview formatting codes like [::b], [yellow], [::-], etc.
	// This matches [ followed by any characters (non-greedy) and then ]
	re := regexp.MustCompile(`\[[^]]*\]`)
	return re.ReplaceAllString(text, "")
}

// findSearchMatchesWithPositions finds all matches and returns position information for navigation
func (b *Browser) findSearchMatchesWithPositions(text, term string, caseSensitive bool) []SearchMatch {
	if term == "" || strings.TrimSpace(term) == "" {
		return []SearchMatch{}
	}

	// Escape special regex characters in the search term to prevent regex injection
	escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))

	var re *regexp.Regexp
	if caseSensitive {
		// Create a case-sensitive regex to find the search term
		re = regexp.MustCompile(escapedTerm)
	} else {
		// Create a case-insensitive regex to find the search term
		re = regexp.MustCompile("(?i)" + escapedTerm)
	}

	// Find all match locations (start and end indices)
	indices := re.FindAllStringIndex(text, -1)

	if indices == nil {
		return []SearchMatch{}
	}

	// Split content into lines to provide context
	lines := strings.Split(text, "\n")
	var searchMatches []SearchMatch

	for _, loc := range indices {
		start, end := loc[0], loc[1]

		// Find which line the match is in
		lineNum := 0
		charCount := 0

		for i, line := range lines {
			nextCharCount := charCount + len(line) + 1 // +1 for newline character
			if start < nextCharCount {
				lineNum = i
				break
			}
			charCount = nextCharCount
		}

		// Get the line text containing the match
		if lineNum < len(lines) {
			lineText := lines[lineNum]

			// Create a SearchMatch with the information
			match := SearchMatch{
				LineNum:   lineNum,
				LineText:  lineText,
				CharStart: start,
				CharEnd:   end,
			}

			searchMatches = append(searchMatches, match)
		}
	}

	// Remove duplicates by checking if matches are too close to each other
	uniqueMatches := []SearchMatch{}
	seenPositions := make(map[int]bool)

	for _, match := range searchMatches {
		// Use the character start position as a unique identifier
		if !seenPositions[match.CharStart] {
			seenPositions[match.CharStart] = true
			uniqueMatches = append(uniqueMatches, match)
		}
	}

	return uniqueMatches
}

// scrollToMatch ensures the text view shows the highlighted content and focuses on the selected match
func (b *Browser) scrollToMatch(match SearchMatch) {
	// The text is already highlighted with the selected match in the call that brought us here
	// Just ensure focus is set properly
	b.app.SetFocus(b.textView)
}