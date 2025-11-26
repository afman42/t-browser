package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// startSearch starts the search functionality
func (b *Browser) startSearch() {
	// Create a modal search with input field and results area
	inputField := tview.NewInputField().
		SetLabel("Real-time search (case-sensitive): ").
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter || key == tcell.KeyEscape {
				// Return to main view when Enter or Escape is pressed
				// Restore the proper flex layout with URL input
				flex := tview.NewFlex().
					SetDirection(tview.FlexRow).
					AddItem(b.textView, 0, 1, false).  // Main content area - takes remaining space
					AddItem(b.urlInput, 3, 0, false)   // URL input at the bottom - fixed height of 3

				b.app.SetRoot(flex, true)
				b.app.SetFocus(b.textView)  // Ensure content view has focus after search
			}
		})

	// Create a text view to show search results
	resultsView := tview.NewTextView().
		SetScrollable(true).
		SetDynamicColors(true)

	// Layout container for both input and results
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(inputField, 3, 1, true).  // Input field takes 3 lines
		AddItem(resultsView, 0, 1, false) // Results area takes remaining space

	// Set up real-time search
	inputField.SetChangedFunc(func(text string) {
		// Update search in real-time as user types
		b.searchTerm = text
		// By default, perform case-sensitive search
		matchCount, matches := b.performSearchWithMatches(text, true)

		// Update the label to show match count
		if text != "" {
			inputField.SetLabel(fmt.Sprintf("Real-time search (case-sensitive) - %d matches: ", matchCount))
		} else {
			inputField.SetLabel("Real-time search (case-sensitive): ")
		}

		// Update results view with matching text
		if len(matches) > 0 {
			var resultText strings.Builder
			resultText.WriteString("Matches found:\n")
			for i, matchInfo := range matches {
				resultText.WriteString(fmt.Sprintf("%d. %s\n", i+1, matchInfo))
			}
			resultsView.SetText(resultText.String())
		} else {
			resultsView.SetText("No matches found")
		}
	})

	// Set focus to the input field
	b.app.SetRoot(layout, true)
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

// highlightSearchTerm highlights search terms in the text
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
	highlighted := re.ReplaceAllStringFunc(text, func(match string) string {
		return "[yellow]" + match + "[::-]"
	})
	
	return highlighted
}

// removeTviewFormatting removes tview formatting codes from text like [::b], [yellow], etc.
func removeTviewFormatting(text string) string {
	// Regex to match tview formatting codes like [::b], [yellow], [::-], etc.
	// This matches [ followed by any characters (non-greedy) and then ]
	re := regexp.MustCompile(`\[[^]]*\]`)
	return re.ReplaceAllString(text, "")
}