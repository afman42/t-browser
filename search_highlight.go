package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

func compileSearchRegex(term string, caseSensitive bool) *regexp.Regexp {
	trimmed := strings.TrimSpace(term)
	if trimmed == "" {
		return nil
	}
	escapedTerm := regexp.QuoteMeta(trimmed)
	if caseSensitive {
		return regexp.MustCompile(escapedTerm)
	}
	return regexp.MustCompile("(?i)" + escapedTerm)
}

func (b *Browser) findSearchMatches(text string, re *regexp.Regexp) (int, []string) {
	if re == nil {
		return 0, []string{}
	}

	indices := re.FindAllStringIndex(text, -1)
	if indices == nil {
		return 0, []string{}
	}

	var contexts []string
	seen := make(map[string]bool)

	for _, loc := range indices {
		start, end := loc[0], loc[1]

		wordStart := start
		for wordStart > 0 {
			char := text[wordStart-1]
			if char == ' ' || char == '\n' || char == '\t' ||
				char == '.' || char == ',' || char == '!' ||
				char == '?' || char == ';' || char == ':' {
				break
			}
			if char == '[' {
				break
			}
			wordStart--
		}

		wordEnd := end
		for wordEnd < len(text) {
			char := text[wordEnd]
			if char == ' ' || char == '\n' || char == '\t' ||
				char == '.' || char == ',' || char == '!' ||
				char == '?' || char == ';' || char == ':' {
				break
			}
			if char == '[' {
				break
			}
			wordEnd++
		}

		word := text[wordStart:wordEnd]
		plainWord := removeTviewFormatting(word)

		if !seen[plainWord] {
			seen[plainWord] = true
			contexts = append(contexts, plainWord)
		}
	}

	return len(indices), contexts
}

func (b *Browser) highlightSearchTerm(text string, re *regexp.Regexp) string {
	if re == nil {
		return text
	}

	highlighted := re.ReplaceAllStringFunc(text, func(match string) string {
		return fmt.Sprintf("[yellow]%s[-]", match)
	})

	return highlighted
}

func (b *Browser) highlightSelectedMatch(text string, re *regexp.Regexp, selectedMatch SearchMatch) string {
	if re == nil {
		return text
	}

	lines := strings.Split(text, "\n")

	selectedLineIndex := -1
	currentPos := 0

	for i, line := range lines {
		lineEndPos := currentPos + len(line)
		if selectedMatch.CharStart >= currentPos && selectedMatch.CharEnd <= lineEndPos {
			selectedLineIndex = i
			break
		}
		currentPos = lineEndPos + 1
	}

	if selectedLineIndex == -1 {
		return b.highlightSearchTerm(text, re)
	}

	var result strings.Builder
	lineStartPos := 0

	for i, line := range lines {
		if i == selectedLineIndex {
			lineMatches := re.FindAllStringIndex(line, -1)

			if lineMatches != nil {
				var lineResult strings.Builder
				lastEnd := 0

				for _, match := range lineMatches {
					start, end := match[0], match[1]
					lineResult.WriteString(line[lastEnd:start])

					absoluteStart := lineStartPos + start
					absoluteEnd := lineStartPos + end

					if absoluteStart == selectedMatch.CharStart && absoluteEnd == selectedMatch.CharEnd {
						lineResult.WriteString(fmt.Sprintf("[yellow::b]%s[-]", line[start:end]))
					} else {
						lineResult.WriteString(fmt.Sprintf("[yellow]%s[-]", line[start:end]))
					}

					lastEnd = end
				}

				lineResult.WriteString(line[lastEnd:])
				result.WriteString(lineResult.String())
			} else {
				result.WriteString(line)
			}
		} else {
			lineMatches := re.FindAllStringIndex(line, -1)

			if lineMatches != nil {
				var lineResult strings.Builder
				lastEnd := 0

				for _, match := range lineMatches {
					start, end := match[0], match[1]
					lineResult.WriteString(line[lastEnd:start])
					lineResult.WriteString(fmt.Sprintf("[yellow]%s[-]", line[start:end]))
					lastEnd = end
				}

				lineResult.WriteString(line[lastEnd:])
				result.WriteString(lineResult.String())
			} else {
				result.WriteString(line)
			}
		}

		if i < len(lines)-1 {
			result.WriteString("\n")
		}
		lineStartPos += len(line) + 1
	}

	return result.String()
}

func currentPosAtLineStart(text string, lineIndex int) int {
	lines := strings.Split(text, "\n")
	pos := 0
	for i := 0; i < lineIndex && i < len(lines); i++ {
		pos += len(lines[i]) + 1
	}
	return pos
}

func removeTviewFormatting(text string) string {
	re := regexp.MustCompile(`\[[^]]*\]`)
	text = re.ReplaceAllString(text, "")

	reUnicode := regexp.MustCompile(`[\x{0900}-\x{0DFF}\x{1CD0}-\x{1CFF}\x{A8E0}-\x{A8FF}\x{0300}-\x{036F}\x{1AB0}-\x{1AFF}\x{1DC0}-\x{1DFF}\x{20D0}-\x{20FF}\x{FE20}-\x{FE2F}]`)
	text = reUnicode.ReplaceAllString(text, "")

	return text
}

func removeUnwantedCharsFromDisplay(text string) string {
	text = removeTviewFormatting(text)

	var cleanText strings.Builder
	for _, r := range text {
		if !unicode.Is(unicode.Mn, r) && !unicode.Is(unicode.Mc, r) && !unicode.Is(unicode.Me, r) {
			cleanText.WriteRune(r)
		}
	}

	return cleanText.String()
}

func (b *Browser) findSearchMatchesWithPositions(text string, re *regexp.Regexp) []SearchMatch {
	if re == nil {
		return []SearchMatch{}
	}

	indices := re.FindAllStringIndex(text, -1)
	if indices == nil {
		return []SearchMatch{}
	}

	lines := strings.Split(text, "\n")
	lineOffsets := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		lineOffsets[i] = offset
		offset += len(line) + 1
	}

	var allMatches []SearchMatch

	for _, loc := range indices {
		start, end := loc[0], loc[1]

		lineNum := 0
		for i := len(lineOffsets) - 1; i >= 0; i-- {
			if start >= lineOffsets[i] {
				lineNum = i
				break
			}
		}

		if lineNum < len(lines) {
			allMatches = append(allMatches, SearchMatch{
				LineNum:   lineNum,
				LineText:  lines[lineNum],
				CharStart: start,
				CharEnd:   end,
			})
		}
	}

	return allMatches
}

func (b *Browser) scrollToMatch(match SearchMatch) {
	tab := b.currentTab()
	text := tab.originalContent
	lines := strings.Split(text, "\n")

	approxLineNum := 0
	charCount := 0

	for i, line := range lines {
		nextCharCount := charCount + len(line) + 1
		if match.CharStart < nextCharCount {
			approxLineNum = i
			break
		}
		charCount = nextCharCount
	}

	tab.textView.ScrollTo(approxLineNum, 0)
}

func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
