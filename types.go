package main

import (
	"context"
	"sync"

	"github.com/rivo/tview"
)

const ItemsPerPage = 20

type Link struct {
	URL      string
	Text     string
	Position int
}

type Tab struct {
	textView                   *tview.TextView
	history                    []string
	historyIndex               int
	currentURL                 string
	searchTerm                 string
	originalContent            string
	originalUnprocessedContent string
	links                      []Link
	images                     []Image
	currentLinkIndex           int
	searchMatches              []SearchMatch
	returningFromSearchResult  bool
	displayToMatchIndex        map[int]int
	currentMatchStart          int
	currentMatchEnd            int
	currentMatchIdx            int
	metaRefreshCancel          context.CancelFunc
	searchHistory              []string
	searchHistoryIndex         int
	searchCaseSensitive        bool
}

func newTab() *Tab {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetRegions(true)
	tv.SetWordWrap(true)
	tv.SetScrollable(true)
	tv.SetBorder(true)
	tv.SetTitle("Terminal Browser - Press Ctrl+C to quit, / for search")
	return &Tab{
		textView:            tv,
		history:             make([]string, 0),
		historyIndex:        -1,
		currentLinkIndex:    -1,
		searchCaseSensitive: true,
		searchHistoryIndex:  -1,
	}
}

type Browser struct {
	mu sync.Mutex

	app         *tview.Application
	urlInput    *tview.InputField
	statusBar   *tview.TextView
	tabBar      *tview.TextView
	client      *HTTPClient
	forceUA     string
	isLoading   bool
	loadingStop chan struct{}
	config      *Config

	tabs      []*Tab
	activeTab int

	settingsActive   bool
	settingsChanged  bool
	rightColumnEmpty bool
}

func (b *Browser) currentTab() *Tab {
	if len(b.tabs) == 0 {
		b.tabs = []*Tab{newTab()}
		b.activeTab = 0
	}
	if b.activeTab < 0 || b.activeTab >= len(b.tabs) {
		b.activeTab = 0
	}
	return b.tabs[b.activeTab]
}
