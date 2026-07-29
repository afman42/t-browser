package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/fatih/color"
)

// GetHighlightColor returns a colored string using fatih/color for terminal output
func GetHighlightColor(text string) string {
	return color.YellowString(text)
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
		re = regexp.MustCompile(escapedTerm)
	} else {
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
func (b *Browser) highlightSearchTerm(text, term string, caseSensitive bool) string {
	if term == "" || strings.TrimSpace(term) == "" {
		return text
	}

	// Escape special regex characters in the search term to prevent regex injection
	escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))

	var re *regexp.Regexp
	if caseSensitive {
		re = regexp.MustCompile(escapedTerm)
	} else {
		re = regexp.MustCompile("(?i)" + escapedTerm)
	}

	// Replace only the matching terms with highlighted versions
	highlighted := re.ReplaceAllStringFunc(text, func(match string) string {
		colorCode := ColorToTviewFormat("yellow")
		return fmt.Sprintf("[%s]%s[-]", colorCode, match)
	})

	return highlighted
}

// highlightSearchTermWithColor highlights search terms using a specific color
func (b *Browser) highlightSearchTermWithColor(text, term string, caseSensitive bool, colorName string) string {
	if term == "" || strings.TrimSpace(term) == "" {
		return text
	}

	// Escape special regex characters in the search term to prevent regex injection
	escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))

	var re *regexp.Regexp
	if caseSensitive {
		re = regexp.MustCompile(escapedTerm)
	} else {
		re = regexp.MustCompile("(?i)" + escapedTerm)
	}

	// Convert color name to tview format
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

	var result strings.Builder

	// Process each line
	for i, line := range lines {
		if i == selectedLineIndex {
			// This is the line containing the selected match
			escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))
			re := regexp.MustCompile("(?i)" + escapedTerm)

			// Find all matches in this specific line
			lineMatches := re.FindAllStringIndex(line, -1)

			if lineMatches != nil {
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
						// This is the selected match - highlight it with bold yellow
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
				result.WriteString(line)
			}
		} else {
			// Not the selected line - apply regular search term highlighting
			escapedTerm := regexp.QuoteMeta(strings.TrimSpace(term))
			re := regexp.MustCompile("(?i)" + escapedTerm)

			lineMatches := re.FindAllStringIndex(line, -1)

			if lineMatches != nil {
				var lineResult strings.Builder
				lastEnd := 0

				for _, match := range lineMatches {
					start, end := match[0], match[1]
					lineResult.WriteString(line[lastEnd:start])

					colorCode := ColorToTviewFormat("yellow")
					lineResult.WriteString(fmt.Sprintf("[%s]%s[-]", colorCode, line[start:end]))

					lastEnd = end
				}

				lineResult.WriteString(line[lastEnd:])
				result.WriteString(lineResult.String())
			} else {
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
	// Remove tview formatting codes like [::b], [yellow], [::-], etc.
	re := regexp.MustCompile(`\[[^]]*\]`)
	text = re.ReplaceAllString(text, "")

	// Remove unwanted Unicode characters (like ം and া)
	reUnicode := regexp.MustCompile(`[\x{0900}-\x{0DFF}\x{1CD0}-\x{1CFF}\x{A8E0}-\x{A8FF}\x{0300}-\x{036F}\x{1AB0}-\x{1AFF}\x{1DC0}-\x{1DFF}\x{20D0}-\x{20FF}\x{FE20}-\x{FE2F}]`)
	text = reUnicode.ReplaceAllString(text, "")

	return text
}

// removeUnwantedCharsFromDisplay removes tview formatting and unwanted Unicode characters for clean display
func removeUnwantedCharsFromDisplay(text string) string {
	// Remove tview formatting codes
	text = removeTviewFormatting(text)

	// Additional cleaning: remove zero-width characters and other problematic Unicode
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
		re = regexp.MustCompile(escapedTerm)
	} else {
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
	uniqueMatches := []SearchMatch{}
	usedLines := make(map[int]bool)

	for _, match := range allMatches {
		if !usedLines[match.LineNum] {
			usedLines[match.LineNum] = true
			uniqueMatches = append(uniqueMatches, match)
		}
	}

	return uniqueMatches
}

// scrollToMatch ensures the text view shows the highlighted content
func (b *Browser) scrollToMatch(match SearchMatch) {
	text := b.currentTab().textView.GetText(false)
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

	b.currentTab().textView.ScrollTo(approxLineNum, 0)
	b.app.SetFocus(b.currentTab().textView)
}
