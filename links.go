package main

import (
	"strings"
)

// extractVisibleLinks filters links to only those that appear in the readability content
func (b *Browser) extractVisibleLinks(content string, allLinks []Link) []Link {
	visibleLinks := make([]Link, 0)
	usedTexts := make(map[string]bool) // Track which link texts have been used

	for _, link := range allLinks {
		// Check if the link text appears in the readability content
		// We also want to avoid duplicates
		if strings.Contains(content, link.Text) && !usedTexts[link.Text] {
			// Additional check: ensure the link text is a meaningful part of the content
			// (not just a small fragment that happens to match)
			if len(link.Text) >= 2 { // At least 2 characters to be considered meaningful
				visibleLinks = append(visibleLinks, link)
				usedTexts[link.Text] = true
			}
		}
	}

	return visibleLinks
}
