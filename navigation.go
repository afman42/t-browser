package main

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (b *Browser) NavigateTo(targetURL string) {
	b.navigateTab(b.currentTab(), targetURL)
}

// prepareNavigation validates a destination for an explicit tab and updates
// that tab's history.  Pure state, no UI effects: error display and the
// tracking-strip toast stay in navigateTab.  Returns the cleaned URL (after
// tracking-param stripping) and the validated URL to fetch.
func (b *Browser) prepareNavigation(tab *Tab, targetURL string) (cleanedURL, validatedURL string, err error) {
	cleanedURL = b.cleanURLForNavigation(targetURL)
	validatedURL, err = b.validateAndSanitizeURL(cleanedURL)
	if err != nil {
		return "", "", err
	}

	if tab.metaRefreshCancel != nil {
		tab.metaRefreshCancel()
		tab.metaRefreshCancel = nil
	}

	if tab.currentURL != "" && (len(tab.history) == 0 || tab.history[len(tab.history)-1] != tab.currentURL) {
		tab.history = append(tab.history, tab.currentURL)
		tab.historyIndex = len(tab.history) - 1
	}
	return cleanedURL, validatedURL, nil
}

// navigateTab navigates the given tab to targetURL.  Navigation must target an
// explicit tab: background triggers (meta refresh, history) must never hijack
// whatever tab happens to be current when the fetch completes.
func (b *Browser) navigateTab(tab *Tab, targetURL string) {
	cleanedURL, validatedURL, err := b.prepareNavigation(tab, targetURL)
	if err != nil {
		// NavigateTo is called from the UI thread (input captures / DoneFunc),
		// so we must NOT use QueueUpdateDraw or Draw here — both block until
		// the event loop processes the queued function, but the event loop is
		// busy running this code.  ForceDraw is safe during event handling.
		b.displayError(tab, fmt.Sprintf("Invalid URL: %v", err))
		b.app.ForceDraw()
		return
	}

	// Notify the user when tracking parameters were silently removed.
	if cleanedURL != targetURL && b.config != nil && b.config.StripTrackingParams {
		b.showStatusToast("[yellow]Stripped tracking parameters from URL[-]", 2500*time.Millisecond)
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
				b.displayError(tab, fmt.Sprintf("Error fetching page: %v", err))
				return
			}

			tab.currentURL = cleanedURL
			b.renderPage(tab, content, cleanedURL)
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

	if err := b.checkBlockedDomain(inputURL); err != nil {
		return "", err
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
		// Legacy numeric forms (2130706433, 0x7f000001, 0177.0.0.1) that only
		// the resolver would otherwise interpret as IPv4 (SSRF bypass).
		if legacy, ok := parseLegacyIPv4(host); ok {
			ip = legacy
		}
	}
	if ip == nil {
		return false
	}
	return isInternalIP(ip)
}

// isInternalIP reports whether an IP is a loopback, unspecified, link-local,
// or RFC 1918 private address.
func isInternalIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsPrivate()
}

// net.ParseIP rejects: a 32-bit integer (decimal/octal/hex) or a dotted form
// whose parts may be decimal, octal, or hex.  The final part of a short form
// is wider than 8 bits (a.b.c → 16-bit, a.b → 24-bit), matching inet_aton.
func parseLegacyIPv4(host string) (net.IP, bool) {
	parts := strings.Split(host, ".")
	if len(parts) == 0 || len(parts) > 4 {
		return nil, false
	}
	if len(parts) == 1 {
		n, err := strconv.ParseUint(host, 0, 32)
		if err != nil {
			return nil, false
		}
		return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n)), true
	}
	var octets [4]byte
	idx := 0
	for _, p := range parts[:len(parts)-1] {
		n, err := strconv.ParseUint(p, 0, 8)
		if err != nil {
			return nil, false
		}
		octets[idx] = byte(n)
		idx++
	}
	lastBits := 8 * (5 - len(parts))
	n, err := strconv.ParseUint(parts[len(parts)-1], 0, lastBits)
	if err != nil {
		return nil, false
	}
	for b := 0; b < lastBits/8; b++ {
		octets[len(octets)-1-b] = byte(n >> (8 * b))
	}
	return net.IPv4(octets[0], octets[1], octets[2], octets[3]), true
}

func (b *Browser) displayError(tab *Tab, message string) {
	tab.textView.SetText(fmt.Sprintf("[red]Error: %s[-]", message))
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
	b.currentTab().loading = true
	b.loadingStop = make(chan struct{})
	go b.animateLoading()
}

func (b *Browser) animateLoading() {
	// Capture the stop channel once: hideLoadingIndicator closes it but never
	// replaces it, so the goroutine and the hider share one channel (no race).
	stop := b.loadingStop
	phases := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	currentPhase := 0

	for {
		select {
		case <-stop:
			return
		default:
			animationText := fmt.Sprintf(" %s Loading...", phases[currentPhase])
			phase := currentPhase
			b.app.QueueUpdateDraw(func() {
				b.loadingPhase = phase
				if b.statusBar != nil {
					b.statusBar.SetText(animationText)
				}
				if b.tabBar != nil {
					b.updateTabBar()
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
	b.isLoading = false
	for _, tab := range b.tabs {
		tab.loading = false
	}
	if b.tabBar != nil {
		b.updateTabBar()
	}
	b.updateStatusBar()
}
