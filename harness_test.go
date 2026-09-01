package main

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// runWithSimScreen starts a tview Application on a simulation screen,
// runs the given function, then stops the app.  The screen is already
// initialized; callers can inject keys via screen.InjectKey.
func runWithSimScreen(t *testing.T, fn func(app *tview.Application, screen tcell.SimulationScreen)) {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("simulation screen init: %v", err)
	}
	app := tview.NewApplication().SetScreen(screen)
	done := make(chan struct{})
	go func() {
		_ = app.Run()
		close(done)
	}()
	// Give Run a moment to start the event loop.
	time.Sleep(20 * time.Millisecond)
	fn(app, screen)
	app.Stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("app did not stop")
	}
}

func TestShowAnimatedImageLoading(t *testing.T) {
	runWithSimScreen(t, func(app *tview.Application, _ tcell.SimulationScreen) {
		b := &Browser{app: app}
		info := tview.NewTextView()
		stop := make(chan struct{})
		go b.showAnimatedImageLoading(info, "https://example.com/pic.png", stop)
		time.Sleep(40 * time.Millisecond)
		close(stop)
		time.Sleep(20 * time.Millisecond)
		// Read on the UI goroutine to avoid racing the queued SetText.
		done := make(chan string, 1)
		app.QueueUpdateDraw(func() { done <- info.GetText(false) })
		select {
		case got := <-done:
			if got == "" {
				t.Error("animated loading should set text")
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for GetText")
		}
	})
}

func TestTransportForImages(t *testing.T) {
	// Nil client falls back to DefaultTransport.
	if got := (&Browser{}).transportForImages(); got == nil {
		t.Error("nil client should return a non-nil transport")
	}
	// With a client, the snapshot transport is returned.
	client := NewHTTPClient(nil)
	b := &Browser{client: client}
	if got := b.transportForImages(); got != client.transportSnapshot() {
		t.Error("transportForImages should return the snapshot transport")
	}
}

func TestShowImagePreviewSSRFBlocked(t *testing.T) {
	runWithSimScreen(t, func(app *tview.Application, _ tcell.SimulationScreen) {
		client := NewHTTPClient(nil)
		b := &Browser{
			app:       app,
			client:    client,
			config:    &Config{},
			urlInput:  tview.NewInputField(),
			statusBar: tview.NewTextView(),
			tabBar:    tview.NewTextView(),
		}

		// showImagePreview mutates the app (SetRoot/SetFocus), so it must run
		// on the UI goroutine — production calls it from an input capture.
		// The URL is internal: the SSRF guard must reject it before any fetch.
		opened := make(chan struct{})
		app.QueueUpdateDraw(func() {
			b.showImagePreview("http://127.0.0.1/secret.png")
			close(opened)
		})
		select {
		case <-opened:
		case <-time.After(time.Second):
			t.Fatal("showImagePreview did not run on the UI goroutine")
		}

		// Let the validation goroutine report the block, then tear the modal
		// down (also on the UI goroutine) so its goroutines exit.
		time.Sleep(80 * time.Millisecond)
		closed := make(chan struct{})
		app.QueueUpdateDraw(func() {
			b.closeModalToMain()
			close(closed)
		})
		select {
		case <-closed:
		case <-time.After(time.Second):
			t.Fatal("modal teardown did not run")
		}
	})
}

func TestRunCreatesUI(t *testing.T) {
	// Run is the entry point that wires ApplyTheme + createUI + status/tab
	// bars.  With a simulation screen it should start and stop cleanly.
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen init: %v", err)
	}
	b := &Browser{
		app:    tview.NewApplication().SetScreen(screen),
		config: &Config{Theme: "dark"},
		client: NewHTTPClient(nil),
	}
	done := make(chan error, 1)
	go func() { done <- b.Run() }()
	time.Sleep(80 * time.Millisecond)
	b.app.Stop()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Stop")
	}
	if b.statusBar == nil || b.tabBar == nil {
		t.Error("Run should create statusBar and tabBar")
	}
}
