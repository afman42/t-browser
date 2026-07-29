package main

import "strings"

// wrapText wraps text to the given column width, breaking on word
// boundaries.  Used to make setting descriptions fit the right column.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var current strings.Builder
	currentLen := 0

	for _, word := range words {
		wordLen := len(word)
		if currentLen > 0 && currentLen+1+wordLen > width {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
			currentLen = wordLen
		} else {
			if currentLen > 0 {
				current.WriteString(" ")
				currentLen++
			}
			current.WriteString(word)
			currentLen += wordLen
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}

	return strings.Join(lines, "\n")
}
