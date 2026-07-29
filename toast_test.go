package main

import (
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
)

// statusBarForTest creates a lightweight tview.TextView for tests that don't
// run a real application.
func statusBarForTest() *tview.TextView { return tview.NewTextView() }

func TestSpinnerChar(t *testing.T) {
	b := &Browser{}
	got := b.spinnerChar()
	if got == "" {
		t.Error("spinnerChar should return a non-empty glyph")
	}
	// Phase 0 is the first braille glyph.
	if got != "⠋" {
		t.Errorf("spinnerChar phase 0 = %q, want %q", got, "⠋")
	}

	b.loadingPhase = 3
	if got := b.spinnerChar(); got != "⠸" {
		t.Errorf("spinnerChar phase 3 = %q, want %q", got, "⠸")
	}

	// Out-of-range phase resets to 0.
	b.loadingPhase = 999
	if got := b.spinnerChar(); got != "⠋" {
		t.Errorf("spinnerChar out-of-range = %q, want %q", got, "⠋")
	}
}

func TestSpinnerCharAllPhases(t *testing.T) {
	want := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	b := &Browser{}
	for i, w := range want {
		b.loadingPhase = i
		if got := b.spinnerChar(); got != w {
			t.Errorf("spinnerChar phase %d = %q, want %q", i, got, w)
		}
	}
}

func TestShowStatusToastNoStatusBar(t *testing.T) {
	b := &Browser{}
	// Should be a safe no-op when the status bar is not initialised.
	b.showStatusToast("hello", 10*time.Millisecond)
}

func TestShowStatusToastDisplaysThenAutoDismisses(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{statusBar: statusBarForTest(), config: &cfg}

	b.showStatusToast("[yellow]toast message[-]", 30*time.Millisecond)

	// Immediate display is recorded synchronously (no widget read races with
	// the background restore goroutine).
	if got := b.lastToastText; !strings.Contains(got, "toast message") {
		t.Errorf("toast should be recorded immediately, got %q", got)
	}

	// Wait for the restore goroutine to fully exit before touching the widget,
	// establishing a happens-before edge (race-free).
	select {
	case <-b.toastDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("toast goroutine did not exit in time")
	}

	if got := b.statusBar.GetText(true); strings.Contains(got, "toast message") {
		t.Errorf("toast should have been dismissed, got %q", got)
	}
}

func TestShowStatusToastReplacesPreviousToast(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{statusBar: statusBarForTest(), config: &cfg}

	// First toast with a long hold; a second toast must cancel it.
	b.showStatusToast("first", 2*time.Second)
	b.showStatusToast("second", 30*time.Millisecond)

	if got := b.lastToastText; !strings.Contains(got, "second") {
		t.Errorf("second toast should replace the first, got %q", got)
	}

	select {
	case <-b.toastDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second toast goroutine did not exit in time")
	}
	// After the second toast's hold elapses, neither toast text remains.
	if got := b.statusBar.GetText(true); strings.Contains(got, "first") || strings.Contains(got, "second") {
		t.Errorf("toasts should have been dismissed, got %q", got)
	}
}

func TestCancelToastSafeWhenNoneActive(t *testing.T) {
	b := &Browser{statusBar: statusBarForTest()}
	b.cancelToast() // must not panic with no active toast

	b.showStatusToast("x", 200*time.Millisecond)
	b.cancelToast() // must not panic when cancelling an active toast
	b.cancelToast() // double-cancel must also be safe

	// Ensure the cancelled goroutine has exited before the test ends.
	select {
	case <-b.toastDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cancelled toast goroutine did not exit in time")
	}
}

func TestTabLoadingSpinnerShownInTabBar(t *testing.T) {
	b := &Browser{tabBar: statusBarForTest(), loadingPhase: 0}
	b.tabs = []*Tab{newTab()}
	b.tabs[0].currentURL = "https://example.com"

	b.tabs[0].loading = true
	b.updateTabBar()
	if got := b.tabBar.GetText(true); !strings.Contains(got, "⠋") {
		t.Errorf("tab bar should show spinner for loading tab, got %q", got)
	}

	b.tabs[0].loading = false
	b.updateTabBar()
	if got := b.tabBar.GetText(true); strings.Contains(got, "⠋") {
		t.Errorf("tab bar should not show spinner when not loading, got %q", got)
	}
}

func TestTabBarLoadingSpinnerAcrossMultipleTabs(t *testing.T) {
	b := &Browser{tabBar: statusBarForTest(), loadingPhase: 4}
	b.tabs = []*Tab{newTab(), newTab()}
	b.tabs[0].currentURL = "https://a.test"
	b.tabs[1].currentURL = "https://b.test"

	// Only the second tab is loading.
	b.tabs[1].loading = true
	b.updateTabBar()
	got := b.tabBar.GetText(true)
	if !strings.Contains(got, "⠼ https://b.test") {
		t.Errorf("loading tab should show spinner glyph, got %q", got)
	}
	if strings.Contains(got, "⠼ https://a.test") {
		t.Errorf("non-loading tab should not show spinner, got %q", got)
	}
}
