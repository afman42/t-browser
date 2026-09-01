package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// endsWithNewline reports whether the builder's last rune is '\n'.  Byte-
// indexing the tail would misread multi-byte runes.
func endsWithNewline(b *strings.Builder) bool {
	if b.Len() == 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(b.String())
	return r == '\n'
}

// endsWithNewlineOrSpace is endsWithNewline, also true for a trailing space.
func endsWithNewlineOrSpace(b *strings.Builder) bool {
	if b.Len() == 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(b.String())
	return r == '\n' || r == ' '
}

// renderNode renders an individual HTML node to text
func (b *Browser) renderNode(node *html.Node, result *strings.Builder, tabs *int) {
	switch node.Type {
	case html.TextNode:
		text := strings.TrimSpace(node.Data)
		if text != "" {
			// Add indentation if needed
			if result.Len() > 0 && !endsWithNewlineOrSpace(result) {
				result.WriteString(" ")
			}
			// Escape special characters that might interfere with tview formatting
			text = strings.ReplaceAll(text, "[", "\\[")
			text = strings.ReplaceAll(text, "]", "\\]")
			text = strings.ReplaceAll(text, "*", "\\*")
			text = strings.ReplaceAll(text, "_", "\\_")
			text = strings.ReplaceAll(text, "`", "\\`")
			result.WriteString(text)
		}
	case html.ElementNode:
		tag := node.DataAtom.String()
		isBlockElement := isBlockElement(tag)

		// Handle special tags with improved formatting
		switch tag {
		case "h1":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("\n[cyan::b]# ")
			*tabs += 2
		case "h2":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("\n[yellow::b]## ")
			*tabs += 2
		case "h3":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("\n[green::b]### ")
			*tabs += 2
		case "h4":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("[blue::b]#### ")
			*tabs += 2
		case "h5":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("[magenta::b]##### ")
			*tabs += 2
		case "h6":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("[red::b]###### ")
			*tabs += 2
		case "p":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			for i := 0; i < *tabs; i++ {
				result.WriteString("  ") // 2 spaces per tab level
			}
		case "b", "strong":
			result.WriteString("[::b]") // Bold formatting
		case "i", "em":
			result.WriteString("[::i]") // Italic formatting
		case "u", "ins":
			result.WriteString("[::b]") // Bold instead of underline for emphasis
		case "del", "s", "strike":
			result.WriteString("~~") // Strikethrough formatting
		case "code":
			result.WriteString("[::b]`[::-]") // Code formatting with bold monospace
		case "pre":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString("[::b]```[::-]\n") // Code block start with monospace styling
		case "blockquote":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			for i := 0; i < *tabs; i++ {
				result.WriteString("  ")
			}
			result.WriteString("[cyan]> [-]") // Coloured blockquote marker
			*tabs += 1
		case "a":
			// Add link formatting but keep the content readable
			break
		case "ul", "ol":
			*tabs += 1
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
		case "li":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			for i := 0; i < *tabs-1; i++ {
				result.WriteString("  ") // 2 spaces per indentation level
			}
			if isParent(node, "ol") {
				// Handle ordered list
				index := getListItemIndex(node)
				result.WriteString(fmt.Sprintf("%d. ", index))
			} else {
				result.WriteString("* ")
			}
		case "br":
			result.WriteString("\n")
		case "div":
			if isBlockElement && !endsWithNewline(result) {
				result.WriteString("\n")
			}
		case "hr":
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
			result.WriteString(strings.Repeat("─", 40) + "\n") // Full-width horizontal rule
		case "table":
			b.renderTable(node, result, tabs)
			return // children already processed by renderTable
		}

		// Process children
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			b.renderNode(child, result, tabs)
		}

		// Close tags with appropriate formatting
		switch tag {
		case "h1":
			*tabs -= 2
			result.WriteString("[-]\n")
			result.WriteString("────────────────────────────────────────\n")
		case "h2":
			*tabs -= 2
			result.WriteString("[-]\n")
			result.WriteString("────────────────────────────────\n")
		case "h3":
			*tabs -= 2
			result.WriteString("[-]\n")
		case "h4", "h5", "h6":
			*tabs -= 2
			result.WriteString("[-]\n")
		case "b", "strong":
			result.WriteString("[-]") // Close bold formatting
		case "i", "em":
			result.WriteString("[-]") // Close italic formatting
		case "u", "ins":
			result.WriteString("[-]") // Close underline formatting
		case "del", "s", "strike":
			result.WriteString("~~") // Close strikethrough formatting
		case "code":
			result.WriteString("`") // Close code formatting
		case "pre":
			result.WriteString("\n[::b]```[::-]") // Code block end
		case "blockquote":
			*tabs -= 1
		case "a":
			if href, exists := getAttribute(node, "href"); exists {
				// Format the link in a more readable way
				result.WriteString(fmt.Sprintf(" [%s]", href))
			}
		case "ul", "ol":
			*tabs -= 1
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
		}

		// Add newline after block elements (except for list items which are handled individually)
		if isBlockElement && tag != "li" && tag != "h1" && tag != "h2" && tag != "h3" && tag != "h4" && tag != "h5" && tag != "h6" && tag != "pre" {
			if result.Len() > 0 && result.String()[result.Len()-1] != '\n' {
				result.WriteString("\n")
			}
		}
	}
}

func (b *Browser) renderTable(tableNode *html.Node, result *strings.Builder, tabs *int) {
	if !endsWithNewline(result) {
		result.WriteString("\n")
	}

	var rows [][]string
	var headerRow []string

	collectCells := func(rowNode *html.Node) []string {
		var cells []string
		for cell := rowNode.FirstChild; cell != nil; cell = cell.NextSibling {
			if cell.Type == html.ElementNode {
				ctag := cell.DataAtom.String()
				if ctag == "td" || ctag == "th" {
					var cellText strings.Builder
					for child := cell.FirstChild; child != nil; child = child.NextSibling {
						b.renderNode(child, &cellText, tabs)
					}
					cells = append(cells, strings.TrimSpace(removeTviewFormatting(cellText.String())))
				}
			}
		}
		return cells
	}

	for child := tableNode.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		childTag := child.DataAtom.String()
		switch childTag {
		case "thead", "tbody", "tfoot":
			for tr := child.FirstChild; tr != nil; tr = tr.NextSibling {
				if tr.Type == html.ElementNode && tr.DataAtom.String() == "tr" {
					cells := collectCells(tr)
					if len(cells) > 0 {
						if childTag == "thead" {
							headerRow = cells
						} else {
							rows = append(rows, cells)
						}
					}
				}
			}
		case "tr":
			cells := collectCells(child)
			if len(cells) > 0 {
				if isInThead(child) {
					headerRow = cells
				} else {
					rows = append(rows, cells)
				}
			}
		}
	}

	allRows := [][]string{}
	if len(headerRow) > 0 {
		allRows = append(allRows, headerRow)
	}
	allRows = append(allRows, rows...)
	if len(allRows) == 0 {
		return
	}

	maxCols := 0
	for _, row := range allRows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	const maxColWidth = 40
	colWidths := make([]int, maxCols)
	for _, row := range allRows {
		for i, cell := range row {
			if i < maxCols {
				if w := utf8.RuneCountInString(cell); w > colWidths[i] {
					colWidths[i] = w
				}
			}
		}
	}
	for i := range colWidths {
		if colWidths[i] > maxColWidth {
			colWidths[i] = maxColWidth
		}
	}

	drawHBorder := func(left, mid, right string) {
		result.WriteString(left)
		for i := 0; i < maxCols; i++ {
			for j := 0; j < colWidths[i]+2; j++ {
				result.WriteString("─")
			}
			if i < maxCols-1 {
				result.WriteString(mid)
			}
		}
		result.WriteString(right + "\n")
	}

	drawRow := func(row []string, bold bool) {
		result.WriteString("│")
		for i := 0; i < maxCols; i++ {
			cellText := ""
			if i < len(row) {
				cellText = row[i]
				if utf8.RuneCountInString(cellText) > maxColWidth {
					cellText = truncateRunes(cellText, maxColWidth-3) + "..."
				}
			}
			if bold {
				result.WriteString(fmt.Sprintf(" [::b]%-*s[::-] ", colWidths[i], cellText))
			} else {
				result.WriteString(fmt.Sprintf(" %-*s ", colWidths[i], cellText))
			}
			if i < maxCols-1 {
				result.WriteString("│")
			}
		}
		result.WriteString("│\n")
	}

	drawHBorder("┌", "┬", "┐")
	if len(headerRow) > 0 {
		drawRow(headerRow, true)
		drawHBorder("├", "┼", "┤")
	}
	for _, row := range rows {
		drawRow(row, false)
	}
	drawHBorder("└", "┴", "┘")
}

// sanitizeForTview sanitizes text to prevent tview formatting code injection
func (b *Browser) sanitizeForTview(text string) string {
	// Known valid tview formatting codes we want to preserve.
	preserved := map[string]string{
		"[::b]": "\x00FMTB\x00",
		"[::i]": "\x00FMTI\x00",
		"[::-]": "\x00FMTRST\x00",
		"[-]":   "\x00FMTE\x00",
	}

	protectedText := text

	// Preserve known valid codes
	for code, token := range preserved {
		protectedText = strings.ReplaceAll(protectedText, code, token)
	}

	// Escape remaining [ and ] that aren't part of preserved codes
	protectedText = strings.ReplaceAll(protectedText, "[", "\\[")
	protectedText = strings.ReplaceAll(protectedText, "]", "\\]")

	// Restore preserved codes
	for code, token := range preserved {
		protectedText = strings.ReplaceAll(protectedText, token, code)
	}

	return protectedText
}

// getListItemIndex calculates the index of a list item in an ordered list
func getListItemIndex(node *html.Node) int {
	index := 1
	current := node.PrevSibling

	for current != nil {
		if current.Type == html.ElementNode && current.DataAtom == atom.Li {
			index++
		}
		current = current.PrevSibling
	}

	return index
}

// isParent checks if any ancestor has the specified tag
func isParent(node *html.Node, tag string) bool {
	parent := node.Parent
	for parent != nil {
		if parent.DataAtom.String() == tag {
			return true
		}
		parent = parent.Parent
	}
	return false
}

// isInThead checks if a node is inside a thead element
func isInThead(node *html.Node) bool {
	parent := node.Parent
	for parent != nil {
		if parent.DataAtom.String() == "thead" {
			return true
		}
		parent = parent.Parent
	}
	return false
}

// getAttribute gets an attribute value from an HTML node
func getAttribute(node *html.Node, attrName string) (string, bool) {
	for _, attr := range node.Attr {
		if attr.Key == attrName {
			return attr.Val, true
		}
	}
	return "", false
}

// isBlockElement checks if the tag is a block-level element
func isBlockElement(tag string) bool {
	blockElements := map[string]bool{
		"address": true, "article": true, "aside": true, "blockquote": true,
		"details": true, "dialog": true, "dd": true, "div": true,
		"dl": true, "dt": true, "fieldset": true, "figcaption": true,
		"figure": true, "footer": true, "form": true, "h1": true,
		"h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
		"header": true, "hgroup": true, "hr": true, "li": true,
		"main": true, "nav": true, "ol": true, "p": true,
		"pre": true, "section": true, "table": true, "ul": true,
		"tr": true, "td": true, "th": true, "thead": true,
		"tbody": true, "tfoot": true,
	}
	return blockElements[tag]
}
