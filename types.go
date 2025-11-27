package main

import "github.com/rivo/tview"

const ItemsPerPage = 20

// Link represents a hyperlink on the page
type Link struct {
	URL      string
	Text     string
	Position int // Position in content for navigation
}

// Browser represents the terminal browser instance
type Browser struct {
	app                       *tview.Application
	textView                  *tview.TextView
	urlInput                  *tview.InputField
	history                   []string
	historyIndex              int
	client                    *HTTPClient
	cookies                   map[string]*Cookie
	proxy                     string
	currentURL                string
	searchTerm                string
	originalContent           string  // Store original content for search
	links                     []Link  // Store links found on the page
	images                    []Image // Store images found on the page
	currentLinkIndex          int     // Index of currently highlighted link
	forceUA                   string
	loadingView               *tview.TextView
	isLoading                 bool
	loadingStop               chan struct{} // Channel to signal loading animation to stop
	searchMatches             []SearchMatch // Store search matches for navigation
	returningFromSearchResult bool          // Flag to track if returning from a selected search result
	config                    *Config       // Configuration for the browser
	settingsActive            bool          // Flag to track if settings page is active
	settingsChanged           bool          // Flag to track if settings have been changed
	rightColumnEmpty          bool          // Flag to track if right column is empty
}
