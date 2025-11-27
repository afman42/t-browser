package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// extractVisibleLinks filters links to only those that appear in the readability content
func (b *Browser) extractVisibleLinks(content string, allLinks []Link) []Link {
	var visibleLinks []Link
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

// getListItemIndex calculates the index of a list item in an ordered list
func (b *Browser) getListItemIndex(node *html.Node) int {
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
func (b *Browser) isParent(node *html.Node, tag string) bool {
	parent := node.Parent
	for parent != nil {
		if parent.DataAtom.String() == tag {
			return true
		}
		parent = parent.Parent
	}
	return false
}

// getAttribute gets an attribute value from an HTML node
func (b *Browser) getAttribute(node *html.Node, attrName string) (string, bool) {
	for _, attr := range node.Attr {
		if attr.Key == attrName {
			return attr.Val, true
		}
	}
	return "", false
}

// isBlockElement checks if the tag is a block-level element
func (b *Browser) isBlockElement(tag string) bool {
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