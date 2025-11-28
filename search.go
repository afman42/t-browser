package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

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
			case 'j': // Move down in the list
				currentIdx := resultsList.GetCurrentItem()
				if currentIdx < resultsList.GetItemCount()-1 {
					resultsList.SetCurrentItem(currentIdx + 1)
				}
				// Update the text view to reflect which match would be selected
				displayIndex := resultsList.GetCurrentItem()
				if internalIdx, exists := b.displayToMatchIndex[displayIndex]; exists && internalIdx < len(b.searchMatches) {
					selectedMatch := b.searchMatches[internalIdx]
					highlightedText := b.highlightSelectedMatch(b.originalContent, b.searchTerm, selectedMatch)
					b.textView.SetText(highlightedText)
					// Scroll to the position of the selected match
					b.scrollToMatch(selectedMatch)
				}
				return nil
			case 'k': // Move up in the list
				currentIdx := resultsList.GetCurrentItem()
				if currentIdx > 0 {
					resultsList.SetCurrentItem(currentIdx - 1)
				}
				// Update the text view to reflect which match would be selected
				displayIndex := resultsList.GetCurrentItem()
				if internalIdx, exists := b.displayToMatchIndex[displayIndex]; exists && internalIdx < len(b.searchMatches) {
					selectedMatch := b.searchMatches[internalIdx]
					highlightedText := b.highlightSelectedMatch(b.originalContent, b.searchTerm, selectedMatch)
					b.textView.SetText(highlightedText)
					// Scroll to the position of the selected match
					b.scrollToMatch(selectedMatch)
				}
				return nil
			case 'q': // Quit search
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
	// Using yellow text on dark background to match HTML styling (like "From" in the example)
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
	// Format as: foreground:background to get text in specified color with yellow background
	fgColor := ColorToTviewFormat(colorName)
	bgColor := ColorToTviewFormat("yellow")

	// Replace only the matching terms with highlighted versions
	highlighted := re.ReplaceAllStringFunc(text, func(match string) string {
		return fmt.Sprintf("[%s:%s]%s[-]", fgColor, bgColor, match)
	})

	return highlighted
}

// highlightSelectedMatch highlights the full line containing the selected match in the main content
func (b *Browser) highlightSelectedMatch(text, term string, selectedMatch SearchMatch) string {
	if term == "" || strings.TrimSpace(term) == "" {
		return text
	}

	// Split the text into lines
	lines := strings.Split(text, "\n")

	// Find which line contains the selected match
	selectedLineIndex := -1
	currentPos := 0

	for i, line := range lines {
		lineEndPos := currentPos + len(line)

		// Check if the selected match is in this line
		if selectedMatch.CharStart >= currentPos && selectedMatch.CharEnd <= lineEndPos {
			selectedLineIndex = i
			break
		}

		currentPos = lineEndPos + 1 // +1 for the newline character
	}

	if selectedLineIndex == -1 {
		// Fallback: use the original functionality if we can't find the line
		return b.highlightSearchTerm(text, term, true)
	}

	// Create a new string builder for the result
	var result strings.Builder

	// Process each line
	for i, line := range lines {
		if i == selectedLineIndex {
			// This is the line containing the selected match - highlight the entire line content
			// Find all occurrences of the search term in this line to highlight them specially
			escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))
			re := regexp.MustCompile("(?i)" + escapedTerm)

			// Find all matches in this specific line
			lineMatches := re.FindAllStringIndex(line, -1)

			if lineMatches != nil {
				// Process this line to highlight search terms with different formatting for the selected one
				var lineResult strings.Builder
				lastEnd := 0

				for _, match := range lineMatches {
					start, end := match[0], match[1]
					// Add text before the match
					lineResult.WriteString(line[lastEnd:start])

					// Check if this specific match is the selected one
					absoluteStart := currentPosAtLineStart(text, i) + start
					absoluteEnd := currentPosAtLineStart(text, i) + end

					if absoluteStart == selectedMatch.CharStart && absoluteEnd == selectedMatch.CharEnd {
						// This is the selected match - highlight it with bold yellow to make it stand out
						colorCode := ColorToTviewFormat("yellow") + ":" + ColorToTviewFormat("bold")
						lineResult.WriteString(fmt.Sprintf("[%s]%s[-]", colorCode, line[start:end]))
					} else {
						// Other matches in the same line - regular yellow highlight
						colorCode := ColorToTviewFormat("yellow")
						lineResult.WriteString(fmt.Sprintf("[%s]%s[-]", colorCode, line[start:end]))
					}

					lastEnd = end
				}

				// Add the remainder of the line
				lineResult.WriteString(line[lastEnd:])
				result.WriteString(lineResult.String())
			} else {
				// No matches in this line, just add it as is
				result.WriteString(line)
			}
		} else {
			// This is not the selected line - highlight normally
			// Apply regular search term highlighting to other lines
			escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))
			re := regexp.MustCompile("(?i)" + escapedTerm)

			// Find all matches in this line
			lineMatches := re.FindAllStringIndex(line, -1)

			if lineMatches != nil {
				var lineResult strings.Builder
				lastEnd := 0

				for _, match := range lineMatches {
					start, end := match[0], match[1]
					// Add text before the match
					lineResult.WriteString(line[lastEnd:start])

					// Highlight the matched text with regular yellow
					colorCode := ColorToTviewFormat("yellow")
					lineResult.WriteString(fmt.Sprintf("[%s]%s[-]", colorCode, line[start:end]))

					lastEnd = end
				}

				// Add the remainder of the line
				lineResult.WriteString(line[lastEnd:])
				result.WriteString(lineResult.String())
			} else {
				// No matches in this line, just add it as is
				result.WriteString(line)
			}
		}

		// Add newline if not the last line
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// currentPosAtLineStart calculates the character position at the start of a given line
func currentPosAtLineStart(text string, lineIndex int) int {
	lines := strings.Split(text, "\n")
	pos := 0
	for i := 0; i < lineIndex && i < len(lines); i++ {
		pos += len(lines[i]) + 1 // +1 for the newline character
	}
	return pos
}

// removeTviewFormatting removes tview formatting codes and unwanted Unicode characters from text
func removeTviewFormatting(text string) string {
	// First, remove tview formatting codes like [::b], [yellow], [::-], etc.
	// This matches [ followed by any characters (non-greedy) and then ]
	re := regexp.MustCompile(`\[[^]]*\]`)
	text = re.ReplaceAllString(text, "")

	// Remove unwanted Unicode characters (like ം and া)
	// We'll remove combining marks and other diacritics that might be problematic
	// This regex matches combining characters
	reUnicode := regexp.MustCompile(`[\x{0900}-\x{0DFF}\x{1CD0}-\x{1CFF}\x{A8E0}-\x{A8FF}\x{0300}-\x{036F}\x{1AB0}-\x{1AFF}\x{1DC0}-\x{1DFF}\x{20D0}-\x{20FF}\x{FE20}-\x{FE2F}]`)
	text = reUnicode.ReplaceAllString(text, "")

	return text
}

// removeUnwantedCharsFromDisplay removes tview formatting and unwanted Unicode characters for clean display
func removeUnwantedCharsFromDisplay(text string) string {
	// Remove tview formatting codes
	text = removeTviewFormatting(text)

	// Additional cleaning: remove zero-width characters and other problematic Unicode
	// This includes the characters mentioned (like ം and া) and similar combining marks
	var cleanText strings.Builder
	for _, r := range text {
		// Skip combining marks and other potentially problematic characters
		if !unicode.Is(unicode.Mn, r) && !unicode.Is(unicode.Mc, r) && !unicode.Is(unicode.Me, r) {
			cleanText.WriteRune(r)
		}
	}

	return cleanText.String()
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
	var allMatches []SearchMatch

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

			allMatches = append(allMatches, match)
		}
	}

	// Remove duplicates by checking if matches are on the same line with overlapping text
	// This ensures we don't have multiple matches from the same line that are too close together
	uniqueMatches := []SearchMatch{}
	usedLines := make(map[int]bool) // Track which lines have already been used

	for _, match := range allMatches {
		// If the line hasn't been used yet, add the match
		if !usedLines[match.LineNum] {
			usedLines[match.LineNum] = true
			uniqueMatches = append(uniqueMatches, match)
		}
	}

	return uniqueMatches
}

// scrollToMatch ensures the text view shows the highlighted content and attempts to position the selected match in view
func (b *Browser) scrollToMatch(match SearchMatch) {
	// The text is already highlighted with the selected match in the call that brought us here
	// tview doesn't have direct methods to scroll to specific text locations
	// We'll ensure focus is set properly and trigger a scroll to beginning for now
	// The user can then visually locate the highlighted match

	// Calculate approximate line number based on character position
	text := b.textView.GetText(false)
	lines := strings.Split(text, "\n")

	// Find the line where the match occurs
	approxLineNum := 0
	charCount := 0

	for i, line := range lines {
		nextCharCount := charCount + len(line) + 1 // +1 for newline character
		if match.CharStart < nextCharCount {
			approxLineNum = i
			break
		}
		charCount = nextCharCount
	}

	// Scroll to the approximate line of the match if possible
	// Set the relative offset to bring the match into view
	b.textView.ScrollTo(approxLineNum, 0)
	b.app.SetFocus(b.textView)
}