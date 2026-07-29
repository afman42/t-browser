package main

import (
	"net/url"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (b *Browser) createUI() {
	b.setupKeyBindings(b.currentTab().textView)

	b.urlInput = tview.NewInputField()
	b.urlInput.SetBorder(true)
	b.urlInput.SetTitle("Enter URL (Tab to autocomplete)")

	var completions []string
	var completionIndex int

	b.urlInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			input := b.urlInput.GetText()
			navigURL := resolveInputURL(input, b.config)
			b.NavigateTo(navigURL)
			b.app.SetFocus(b.currentTab().textView)
			completions = nil
		} else if key == tcell.KeyEscape {
			b.app.SetFocus(b.currentTab().textView)
			completions = nil
		}
	})

	b.urlInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			current := b.urlInput.GetText()
			if current == "" {
				completions = b.getHistoryCompletions("", 5)
			} else {
				completions = b.getHistoryCompletions(current, 10)
			}
			completionIndex = 0
			if len(completions) > 0 {
				b.urlInput.SetText(completions[completionIndex])
			}
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			b.app.SetFocus(b.currentTab().textView)
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == 'p' {
			clipText, err := clipboard.ReadAll()
			if err == nil {
				b.urlInput.SetText(clipText)
			}
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == 's' {
			b.showSettingsModal()
			return nil
		}
		return event
	})
}

func (b *Browser) setupKeyBindings(tv *tview.TextView) {
	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		tab := b.currentTab()
		switch event.Key() {
		case tcell.KeyCtrlC:
			b.app.Stop()
			return nil
		case tcell.KeyEscape:
			b.client.CancelRequest()
			return nil
		case tcell.KeyEnter:
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				b.app.Stop()
				return nil
			case 'b':
				b.GoBack()
				b.updateStatusBar()
				return nil
			case 'f':
				b.GoForward()
				b.updateStatusBar()
				return nil
			case '/':
				b.startSearch()
				return nil
			case 'j':
				currentRow, _ := tab.textView.GetScrollOffset()
				tab.textView.ScrollTo(currentRow+10, 0)
				b.updateStatusBar()
				return nil
			case 'k':
				currentRow, _ := tab.textView.GetScrollOffset()
				newRow := currentRow - 10
				if newRow < 0 {
					newRow = 0
				}
				tab.textView.ScrollTo(newRow, 0)
				b.updateStatusBar()
				return nil
			case 'h':
				_, currentCol := tab.textView.GetScrollOffset()
				tab.textView.ScrollTo(0, currentCol-10)
				return nil
			case 'l':
				_, currentCol := tab.textView.GetScrollOffset()
				tab.textView.ScrollTo(0, currentCol+10)
				return nil
			case 'g':
				tab.textView.ScrollTo(0, 0)
				b.updateStatusBar()
				return nil
			case 'G':
				tab.textView.ScrollTo(1000000, 0)
				b.updateStatusBar()
				return nil
			case 'L':
				if len(tab.links) > 0 {
					b.showLinksModal()
				} else if len(tab.images) > 0 {
					b.showImagesModal()
				}
				return nil
			case 'i':
				if len(tab.images) > 0 {
					b.showImagesModal()
				}
				return nil
			case 'J':
				currentRow, _ := tab.textView.GetScrollOffset()
				tab.textView.ScrollTo(currentRow+10, 0)
				b.updateStatusBar()
				return nil
			case 'K':
				currentRow, _ := tab.textView.GetScrollOffset()
				newRow := currentRow - 10
				if newRow < 0 {
					newRow = 0
				}
				tab.textView.ScrollTo(newRow, 0)
				b.updateStatusBar()
				return nil
			case 's':
				b.showSettingsModal()
				return nil
			case '?':
				b.showHelp()
				return nil
			case 'n':
				b.navigateSearchMatch(1)
				return nil
			case 'N':
				b.navigateSearchMatch(-1)
				return nil
			case 't':
				b.newTab()
				return nil
			case 'T':
				b.closeTab()
				return nil
			case 'H':
				b.prevTab()
				return nil
			case '\t':
				b.app.SetFocus(b.urlInput)
				return nil
			}
		}

		if event.Key() == tcell.KeyTAB {
			if event.Modifiers()&tcell.ModCtrl != 0 {
				if event.Modifiers()&tcell.ModShift != 0 {
					b.prevTab()
				} else {
					b.nextTab()
				}
				return nil
			}
			b.app.SetFocus(b.urlInput)
			return nil
		}

		return event
	})
}

func (b *Browser) showHelp() {
	helpText := `Terminal Browser - Help & Usage

Navigation:
  j     - Scroll down content
  k     - Scroll up content
  h     - Scroll left
  l     - Scroll right
  g     - Go to top of page
  G     - Go to bottom of page
  b     - Go back in history
  f     - Go forward in history
  Esc   - Cancel loading the current page
  s     - Open settings page
  ?     - Show this help
  q     - Quit browser
  Ctrl+C - Quit browser

Links & Images:
  L     - Show modal list of all links on page
  i     - Show modal list of all images on page
  Enter - Confirm selection in modal

Search:
  /     - Real-time search with match highlighting
  n     - Jump to next search match
  N     - Jump to previous search match

Tabs:
  t     - Open a new tab
  T     - Close current tab
  H     - Switch to previous tab
  Ctrl+Tab    - Switch to next tab
  Ctrl+Shift+Tab - Switch to previous tab

Interface:
  Tab   - Switch between content view and URL input box

URL Input:
  - Type a URL or search query and press Enter
  - If it looks like a domain, https:// is added automatically
  - Otherwise, it is sent to your search engine
  - Tab to autocomplete from history
  - p to paste from clipboard

Settings:
  s     - Open settings page (two-column UI)

Press any key to close this help.`

	helpView := tview.NewTextView()
	helpView.SetTextColor(tcell.ColorWhite)
	helpView.SetBackgroundColor(tcell.ColorNavy)
	helpView.SetDynamicColors(true)
	helpView.SetRegions(false)
	helpView.SetText(helpText)
	helpView.SetDoneFunc(func(key tcell.Key) {
		b.app.SetRoot(b.mainFlex(), true)
		b.app.SetFocus(b.currentTab().textView)
	})

	b.app.SetRoot(helpView, true)
}

func resolveInputURL(input string, config *Config) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return "https://example.com"
	}

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return input
	}

	if strings.Contains(input, ".") && !strings.ContainsAny(input, " \t") {
		return "https://" + input
	}

	searchEngine := "https://duckduckgo.com/html?q="
	if config != nil && config.SearchEngine != "" {
		searchEngine = config.SearchEngine
	}
	return searchEngine + url.QueryEscape(input)
}
