package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
	"os/signal"
	"syscall"
)

func NewBrowser() *Browser {
	if err := InitializeConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "config init warning: %v (continuing with defaults)\n", err)
	}

	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load warning: %v (using defaults)\n", err)
		config = GetDefaultConfig()
	}

	client := NewHTTPClient(&config)

	browser := &Browser{
		app:         tview.NewApplication(),
		client:      client,
		forceUA:     config.UserAgent,
		loadingStop: make(chan struct{}),
		config:      &config,
		activeTab:   0,
	}
	browser.tabs = []*Tab{newTab()}

	if config.Proxy != "" {
		if proxy, err := url.Parse(config.Proxy); err == nil {
			browser.client.SetProxy(proxy)
		}
	} else if proxyURL := os.Getenv("PROXY"); proxyURL != "" {
		if proxy, err := url.Parse(proxyURL); err == nil {
			browser.client.SetProxy(proxy)
		}
	}

	return browser
}

func ColorToTviewFormat(colorName string) string {
	switch colorName {
	case "yellow":
		return "yellow"
	case "red":
		return "red"
	case "green":
		return "green"
	case "blue":
		return "blue"
	case "magenta":
		return "magenta"
	case "cyan":
		return "cyan"
	case "white":
		return "white"
	case "black":
		return "black"
	case "bold":
		return "::b"
	case "underline":
		return "::u"
	case "reverse":
		return "::r"
	default:
		return "yellow"
	}
}

func (b *Browser) getHistoryCompletions(prefix string, limit int) []string {
	var matches []string
	seen := make(map[string]bool)

	for i := len(b.currentTab().history) - 1; i >= 0 && len(matches) < limit; i-- {
		u := b.currentTab().history[i]
		if strings.HasPrefix(u, prefix) && !seen[u] {
			matches = append(matches, u)
			seen[u] = true
		}
	}
	return matches
}

// itemsPerPage returns the configured pagination page size, falling back to
// the ItemsPerPage constant when the config is unset or invalid.
func (b *Browser) itemsPerPage() int {
	if b.config != nil && b.config.ItemsPerPage > 0 {
		return b.config.ItemsPerPage
	}
	return ItemsPerPage
}

// spinnerChar returns the braille glyph for the current loading animation
// phase. Kept simple so it can be used from the UI goroutine.
func (b *Browser) spinnerChar() string {
	phases := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	if b.loadingPhase < 0 || b.loadingPhase >= len(phases) {
		b.loadingPhase = 0
	}
	return phases[b.loadingPhase]
}

func (b *Browser) Run() error {
	if b.config != nil {
		configDir := GetConfigDir()
		sessionFile := GetLatestSessionFile(configDir)
		if sessionFile != "" {
			b.LoadSession(sessionFile)
		}
	} else {
		sessionFile := "t-browser-session.json"
		if b.config != nil && b.config.SessionFile != "" {
			sessionFile = b.config.SessionFile
		}
		b.LoadSession(sessionFile)
	}

	b.ApplyTheme()
	b.createUI()

	b.statusBar = tview.NewTextView()
	b.statusBar.SetDynamicColors(true)
	b.statusBar.SetTextAlign(tview.AlignLeft)
	b.statusBar.SetBorder(false)
	b.statusBar.SetBackgroundColor(tcell.ColorDefault)
	b.statusBar.SetTextColor(tcell.ColorWhite)

	b.tabBar = tview.NewTextView()
	b.tabBar.SetDynamicColors(true)
	b.tabBar.SetTextAlign(tview.AlignLeft)
	b.tabBar.SetBorder(false)
	b.tabBar.SetBackgroundColor(tcell.ColorDarkCyan)
	b.tabBar.SetTextColor(tcell.ColorWhite)
	b.updateTabBar()

	flex := b.mainFlex()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		b.app.Stop()
	}()
	defer signal.Stop(sigCh)

	b.app.SetRoot(flex, true)
	b.app.SetFocus(b.currentTab().textView)

	go func() {
		if b.currentTab().currentURL == "" {
			if len(os.Args) > 1 {
				initialURL := os.Args[1]
				if !strings.HasPrefix(initialURL, "http://") && !strings.HasPrefix(initialURL, "https://") {
					initialURL = "https://" + initialURL
				}
				b.NavigateTo(initialURL)
			} else {
				b.NavigateTo("https://example.com")
			}
		} else {
			b.NavigateTo(b.currentTab().currentURL)
		}
	}()

	if err := b.app.EnableMouse(true).Run(); err != nil {
		return err
	}

	b.client.saveCookiesToFile()

	if b.config != nil && b.config.SessionAutoSave {
		configDir := GetConfigDir()
		sessionFile := GetSessionFilePath(configDir)
		b.SaveSession(sessionFile)
	}
	if b.config == nil {
		b.SaveSession("t-browser-session.json")
	}

	return nil
}

func (b *Browser) mainFlex() *tview.Flex {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.tabBar, 1, 0, false).
		AddItem(b.currentTab().textView, 0, 1, false).
		AddItem(b.statusBar, 1, 0, false).
		AddItem(b.urlInput, 3, 0, false)
}

func (b *Browser) updateTabBar() {
	if b.tabBar == nil {
		return
	}
	var sb strings.Builder
	for i, tab := range b.tabs {
		label := tab.currentURL
		if label == "" {
			label = "New Tab"
		} else {
			if len(label) > 30 {
				label = label[:30] + "..."
			}
		}
		if tab.loading {
			label = b.spinnerChar() + " " + label
		}
		if i == b.activeTab {
			sb.WriteString(fmt.Sprintf(" [::b][ %d: %s ][::-] ", i+1, label))
		} else {
			sb.WriteString(fmt.Sprintf(" [ %d: %s ] ", i+1, label))
		}
	}
	if len(b.tabs) == 0 {
		sb.WriteString(" No tabs")
	}
	b.tabBar.SetText(sb.String())
}

// redrawCurrentView replaces the root view and redraws.  It must only be
// called from the UI/event-loop goroutine (e.g. from input captures).  Using
// QueueUpdateDraw or Draw from the event loop deadlocks because both block
// until the event loop processes the queued function — but the event loop is
// already busy running this code.  ForceDraw is the tview-sanctioned way to
// redraw during direct event handling.
func (b *Browser) redrawCurrentView(focus tview.Primitive) {
	if b.app == nil {
		return
	}
	b.app.SetRoot(b.mainFlex(), true)
	b.app.SetFocus(focus)
	b.updateTabBar()
	b.app.ForceDraw()
}

func (b *Browser) newTab() {
	tab := newTab()
	b.setupKeyBindings(tab.textView)
	b.tabs = append(b.tabs, tab)
	b.activeTab = len(b.tabs) - 1
	b.ApplyTheme()
	b.redrawCurrentView(b.urlInput)
}

func (b *Browser) closeTab() {
	if len(b.tabs) <= 1 {
		if b.app != nil {
			b.app.Stop()
		}
		return
	}
	old := b.tabs[b.activeTab]
	if old.metaRefreshCancel != nil {
		old.metaRefreshCancel()
	}
	b.tabs = append(b.tabs[:b.activeTab], b.tabs[b.activeTab+1:]...)
	if b.activeTab >= len(b.tabs) {
		b.activeTab = len(b.tabs) - 1
	}
	b.ApplyTheme()
	b.redrawCurrentView(b.currentTab().textView)
}

func (b *Browser) switchTab(index int) {
	if index < 0 || index >= len(b.tabs) {
		return
	}
	b.activeTab = index
	b.ApplyTheme()
	if b.app != nil {
		b.redrawCurrentView(b.currentTab().textView)
		b.updateStatusBar()
	}
}

func (b *Browser) nextTab() {
	b.switchTab((b.activeTab + 1) % len(b.tabs))
}

func (b *Browser) prevTab() {
	idx := b.activeTab - 1
	if idx < 0 {
		idx = len(b.tabs) - 1
	}
	b.switchTab(idx)
}
