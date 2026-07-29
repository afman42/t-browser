package main

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (b *Browser) NavigateTo(targetURL string) {
	validatedURL, err := b.validateAndSanitizeURL(targetURL)
	if err != nil {
		b.app.QueueUpdateDraw(func() {
			b.displayError(fmt.Sprintf("Invalid URL: %v", err))
		})
		return
	}

	tab := b.currentTab()
	if tab.metaRefreshCancel != nil {
		tab.metaRefreshCancel()
		tab.metaRefreshCancel = nil
	}

	if tab.currentURL != "" && (len(tab.history) == 0 || tab.history[len(tab.history)-1] != tab.currentURL) {
		tab.history = append(tab.history, tab.currentURL)
		tab.historyIndex = len(tab.history) - 1
	}

	go func() {
		b.app.QueueUpdateDraw(func() {
			b.showLoadingIndicator()
		})

		time.Sleep(30 * time.Millisecond)

		content, err := b.client.FetchPage(validatedURL)

		b.app.QueueUpdateDraw(func() {
			b.hideLoadingIndicator()

			if err != nil {
				b.displayError(fmt.Sprintf("Error fetching page: %v", err))
				return
			}

			tab.currentURL = targetURL
			b.renderPage(content, targetURL)
			tab.currentLinkIndex = -1
			b.updateTitleBar(-1)
		})
	}()
}

func (b *Browser) validateAndSanitizeURL(inputURL string) (string, error) {
	if strings.HasPrefix(inputURL, "javascript:") || strings.HasPrefix(inputURL, "data:") ||
		strings.HasPrefix(inputURL, "vbscript:") || strings.HasPrefix(inputURL, "file:") {
		return "", fmt.Errorf("unsupported or dangerous URL scheme")
	}

	if strings.TrimSpace(inputURL) == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	if len(inputURL) > 2048 {
		return "", fmt.Errorf("URL is too long")
	}

	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %v", err)
	}

	host := parsedURL.Hostname()
	if isInternalAddress(host) {
		return "", fmt.Errorf("access to local/internal addresses not allowed")
	}

	if strings.Contains(inputURL, "..") || strings.Contains(inputURL, "0x00") {
		return "", fmt.Errorf("URL contains suspicious patterns")
	}

	return inputURL, nil
}

func isInternalAddress(host string) bool {
	if host == "" {
		return false
	}
	host = strings.ToLower(host)
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")

	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
		return true
	}

	return false
}

func (b *Browser) displayError(message string) {
	b.currentTab().textView.SetText(fmt.Sprintf("[red]Error: %s[-]", message))
}

func (b *Browser) shouldDisableWordWrap(content string) bool {
	lines := strings.Split(content, "\n")
	longLineCount := 0
	totalLines := len(lines)

	for _, line := range lines {
		if len(line) > 120 {
			longLineCount++
		}
	}

	if totalLines > 0 && float64(longLineCount)/float64(totalLines) > 0.2 {
		return true
	}

	for _, line := range lines {
		if len(line) > 500 {
			return true
		}
	}

	return false
}

func (b *Browser) updateWordWrapBasedOnContent(content string) {
	shouldDisableWrap := b.shouldDisableWordWrap(content)
	b.currentTab().textView.SetWordWrap(!shouldDisableWrap)
}

func (b *Browser) updateTitleBar(linkIndex int) {
	baseTitle := "Terminal Browser - Press Ctrl+C to quit, / for search"
	tab := b.currentTab()

	if linkIndex >= 0 && linkIndex < len(tab.links) {
		linkURL := tab.links[linkIndex].URL
		if len(linkURL) > 50 {
			linkURL = linkURL[:50] + "..."
		}
		tab.textView.SetTitle(fmt.Sprintf("%s | Current Link: %s", baseTitle, linkURL))
	} else {
		tab.textView.SetTitle(baseTitle)
	}
}

func (b *Browser) updateStatusBar() {
	if b.statusBar == nil {
		return
	}
	tab := b.currentTab()
	row, _ := tab.textView.GetScrollOffset()
	status := fmt.Sprintf(" URL: %s | Links: %d | Images: %d | Scroll: %d",
		truncateString(tab.currentURL, 40), len(tab.links), len(tab.images), row)
	b.statusBar.SetText(status)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (b *Browser) showLoadingIndicator() {
	if b.isLoading {
		return
	}
	b.isLoading = true
	go b.animateLoading()
}

func (b *Browser) animateLoading() {
	phases := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	currentPhase := 0

	for {
		select {
		case <-b.loadingStop:
			return
		default:
			animationText := fmt.Sprintf(" %s Loading...", phases[currentPhase])
			b.app.QueueUpdateDraw(func() {
				if b.statusBar != nil {
					b.statusBar.SetText(animationText)
				}
			})
			currentPhase = (currentPhase + 1) % len(phases)
			time.Sleep(120 * time.Millisecond)
		}
	}
}

func (b *Browser) showLoadingModal(title, message string) *tview.TextView {
	loadingView := tview.NewTextView()
	loadingView.SetDynamicColors(true)
	loadingView.SetTextAlign(tview.AlignCenter)
	loadingView.SetBorder(true)
	loadingView.SetBackgroundColor(tcell.ColorBlue)
	loadingView.SetTextColor(tcell.ColorWhite)
	loadingView.SetTitle(title)
	loadingView.SetText(message)
	b.app.SetRoot(loadingView, true)
	return loadingView
}

func (b *Browser) hideLoadingIndicator() {
	if !b.isLoading {
		return
	}
	close(b.loadingStop)
	b.loadingStop = make(chan struct{})
	b.isLoading = false
	b.updateStatusBar()
}
