package main

import (
	"context"
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func TestIsInternalAddress(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		// localhost
		{"localhost", true},
		{"LOCALHOST", true},

		// 127.* loopback
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"127.255.255.255", true},

		// 10.* private
		{"10.0.0.1", true},
		{"10.255.255.255", true},

		// 192.168.* private
		{"192.168.0.1", true},
		{"192.168.1.100", true},

		// 172.16-31 private (BUG FIX: old code allowed 172.32-39)
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		// 172.32+ should NOT be blocked (not private)
		{"172.32.0.1", false},
		{"172.33.0.1", false},
		{"172.15.0.1", false},
		// Old bug: host[4] check allowed 172.32-39 via single-digit check
		{"172.20.0.1", true},
		{"172.25.0.1", true},

		// 169.254.* link-local (NEW: was missing)
		{"169.254.0.1", true},
		{"169.254.169.254", true},

		// 0.0.0.0 (NEW: was missing)
		{"0.0.0.0", true},

		// IPv6 loopback (NEW: was missing)
		{"::1", true},
		{"[::1]", true},

		// IPv6 unspecified (NEW)
		{"::", true},
		{"[::]", true},

		// Public addresses — should NOT be blocked
		{"example.com", false},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"172.32.0.1", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			got := isInternalAddress(tc.host)
			if got != tc.want {
				t.Errorf("isInternalAddress(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestValidateAndSanitizeURLSSRFProtections(t *testing.T) {
	b := &Browser{}
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"IPv6 loopback", "http://[::1]:8080/", true},
		{"link-local 169.254", "http://169.254.169.254/latest/meta-data/", true},
		{"0.0.0.0", "http://0.0.0.0/", true},
		{"172.32 not private", "http://172.32.0.1/", false},
		{"172.16 private", "http://172.16.0.1/", true},
		{"public domain", "https://example.com/", false},
		{"public IP", "http://8.8.8.8/", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := b.validateAndSanitizeURL(tc.url)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateAndSanitizeURL(%q) error = %v, wantErr = %v", tc.url, err, tc.wantErr)
			}
		})
	}
}
func TestTruncateString(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"", 5, ""},
		{"short", 10, "short"},
		{"a", 1, "a"},
		{"abcdef", 6, "abcdef"},
		{"abcdef", 5, "abcde..."},
		{"abcdef", 3, "abc..."},
		{"abcdefghij", 3, "abc..."},
	}
	for _, tc := range tests {
		if got := truncateString(tc.in, tc.max); got != tc.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}

func TestPrepareNavigationValid(t *testing.T) {
	b := &Browser{config: &Config{}}
	tab := newTab()

	cleaned, validated, err := b.prepareNavigation(tab, "https://example.com/page")
	if err != nil {
		t.Fatalf("prepareNavigation: %v", err)
	}
	if cleaned != "https://example.com/page" || validated != "https://example.com/page" {
		t.Errorf("cleaned=%q validated=%q", cleaned, validated)
	}
	// First navigation: no currentURL, so nothing is pushed to history yet.
	if len(tab.history) != 0 {
		t.Errorf("history = %v, want empty", tab.history)
	}

	tab.currentURL = "https://example.com/page"
	_, _, err = b.prepareNavigation(tab, "https://example.com/next")
	if err != nil {
		t.Fatalf("prepareNavigation second call: %v", err)
	}
	if len(tab.history) != 1 || tab.history[0] != "https://example.com/page" {
		t.Errorf("history = %v, want [page]", tab.history)
	}
	// Same-page navigation is not duplicated.
	_, _, err = b.prepareNavigation(tab, "https://example.com/next")
	if err != nil {
		t.Fatal(err)
	}
	if len(tab.history) != 1 {
		t.Errorf("history = %v, want still 1 entry", tab.history)
	}
}

func TestPrepareNavigationStripsTrackingParams(t *testing.T) {
	b := &Browser{config: &Config{StripTrackingParams: true}}
	tab := newTab()

	cleaned, _, err := b.prepareNavigation(tab, "https://example.com/?utm_source=x&a=1")
	if err != nil {
		t.Fatalf("prepareNavigation: %v", err)
	}
	if strings.Contains(cleaned, "utm_source") {
		t.Errorf("tracking params should be stripped, got %q", cleaned)
	}
	if !strings.Contains(cleaned, "a=1") {
		t.Errorf("non-tracking params must survive, got %q", cleaned)
	}
}

func TestPrepareNavigationRejectsDangerousURL(t *testing.T) {
	b := &Browser{config: &Config{}}
	tab := newTab()
	cancelled := false
	tab.metaRefreshCancel = func() { cancelled = true }

	_, _, err := b.prepareNavigation(tab, "javascript:alert(1)")
	if err == nil {
		t.Fatal("expected error for javascript: URL")
	}
	// Validation fails before pending-refresh cancellation.
	if cancelled {
		t.Error("metaRefreshCancel must not run on an invalid URL")
	}
	if len(tab.history) != 0 {
		t.Error("history must not change on an invalid URL")
	}
}

func TestPrepareNavigationCancelsPendingRefresh(t *testing.T) {
	b := &Browser{config: &Config{}}
	tab := newTab()
	cancelled := false
	tab.metaRefreshCancel = context.CancelFunc(func() { cancelled = true })

	if _, _, err := b.prepareNavigation(tab, "https://example.com/"); err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Error("pending meta-refresh should be cancelled by a new navigation")
	}
}

func TestDisplayError(t *testing.T) {
	tab := newTab()
	b := &Browser{}
	b.displayError(tab, "boom")
	if got := tab.textView.GetText(false); got != "[red]Error: boom[-]" {
		t.Errorf("displayError text = %q", got)
	}
}

func TestUpdateTitleBar(t *testing.T) {
	b := &Browser{}
	tab := b.currentTab()

	b.updateTitleBar(-1)
	if !strings.Contains(tab.textView.GetTitle(), "Terminal Browser") {
		t.Errorf("title = %q, want base title", tab.textView.GetTitle())
	}

	longURL := "https://example.com/" + strings.Repeat("a", 60)
	tab.links = []Link{{URL: longURL, Text: "x"}}
	b.updateTitleBar(0)
	title := tab.textView.GetTitle()
	if !strings.Contains(title, "Current Link:") {
		t.Errorf("title = %q, want current-link title", title)
	}
	if len(longURL) > 50 {
		// The URL shown must be the truncated form.
		want := longURL[:50] + "..."
		if !strings.Contains(title, want) {
			t.Errorf("title link URL should be truncated to %q, got %q", want, title)
		}
	}
}

func TestUpdateTitleBarOutOfRange(t *testing.T) {
	b := &Browser{}
	tab := b.currentTab()
	tab.links = []Link{{URL: "https://example.com/", Text: "x"}}
	b.updateTitleBar(5) // out of range → base title
	if strings.Contains(tab.textView.GetTitle(), "Current Link") {
		t.Errorf("out-of-range index must fall back to base title: %q", tab.textView.GetTitle())
	}
}

func TestUpdateWordWrapBasedOnContent(t *testing.T) {
	b := &Browser{}
	tab := b.currentTab()

	// tview exposes no word-wrap getter; these calls exercise the enable and
	// disable branches (short content, >20% long lines, single >500-char line).
	b.updateWordWrapBasedOnContent("short\nlines")
	tab.textView.SetWordWrap(true)
	long := strings.Repeat("x", 200)
	b.updateWordWrapBasedOnContent(long + "\n" + long + "\n" + long + "\n" + "nope")
	tab.textView.SetWordWrap(true)
	b.updateWordWrapBasedOnContent(strings.Repeat("y", 600))
}

func TestUpdateStatusBar(t *testing.T) {
	b := &Browser{statusBar: tview.NewTextView()}
	tab := b.currentTab()
	tab.currentURL = "https://example.com/very/long/path/that/goes/on/and/on/forever"
	tab.links = []Link{{URL: "https://example.com/", Text: "a"}, {URL: "https://example.com/b", Text: "b"}}
	tab.images = []Image{{URL: "https://example.com/i.png"}, {URL: "https://example.com/j.png"}}

	b.updateStatusBar()
	got := b.statusBar.GetText(false)
	if !strings.Contains(got, "Links: 2") || !strings.Contains(got, "Images: 2") {
		t.Errorf("status bar = %q, want link/image counts", got)
	}
	if !strings.Contains(got, "...") {
		t.Errorf("status bar should truncate long URLs: %q", got)
	}
}

func TestUpdateStatusBarNil(t *testing.T) {
	(&Browser{}).updateStatusBar() // must not panic
}

func TestShowHideLoadingIndicator(t *testing.T) {
	// app must be non-nil: animateLoading calls QueueUpdateDraw from its loop.
	b := &Browser{app: tview.NewApplication(), loadingStop: make(chan struct{})}
	tab := b.currentTab()

	b.showLoadingIndicator()
	if !b.isLoading || !tab.loading {
		t.Error("showLoadingIndicator should set flags and animate")
	}
	// Second call is a no-op while loading.
	b.showLoadingIndicator()

	b.hideLoadingIndicator()
	if b.isLoading || tab.loading {
		t.Error("hideLoadingIndicator should clear flags")
	}
	// Hiding again is a no-op.
	b.hideLoadingIndicator()
}

func TestShowLoadingModal(t *testing.T) {
	app := tview.NewApplication()
	b := &Browser{app: app}
	view := b.showLoadingModal("Loading", "Please wait")
	if view == nil {
		t.Fatal("showLoadingModal returned nil")
	}
	if got := view.GetText(false); got != "Please wait" {
		t.Errorf("modal text = %q", got)
	}
	if got := view.GetTitle(); got != "Loading" {
		t.Errorf("modal title = %q", got)
	}
}

func TestShouldDisableWordWrap(t *testing.T) {
	b := &Browser{}
	if b.shouldDisableWordWrap("a\nb") {
		t.Error("short content must not disable wrap")
	}
	// 3 of 4 lines > 120 → ratio 0.75 > 0.2.
	lines := []string{strings.Repeat("x", 121), strings.Repeat("y", 121), strings.Repeat("z", 121), "ok"}
	if !b.shouldDisableWordWrap(strings.Join(lines, "\n")) {
		t.Error(">20% long lines should disable wrap")
	}
	if !b.shouldDisableWordWrap(strings.Repeat("q", 501)) {
		t.Error(">500-char line should disable wrap")
	}
}
