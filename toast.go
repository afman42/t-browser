package main

import (
	"time"
)

// cancelToast stops any currently active status-bar toast without restoring
// the status bar (the caller is responsible for redrawing). Safe to call when
// no toast is active.
func (b *Browser) cancelToast() {
	b.toastMu.Lock()
	defer b.toastMu.Unlock()
	if b.toastStop != nil {
		select {
		case <-b.toastStop:
		default:
			close(b.toastStop)
		}
		b.toastStop = nil
	}
}

// showStatusToast displays a transient message in the status bar that
// automatically dismisses after hold, restoring the normal status text.  Any
// previously active toast is cancelled first.  It is safe to call when the
// application or status bar are not yet initialised (no-op).
func (b *Browser) showStatusToast(message string, hold time.Duration) {
	if b.statusBar == nil {
		return
	}

	b.cancelToast()

	b.toastMu.Lock()
	stop := make(chan struct{})
	done := make(chan struct{})
	b.toastStop = stop
	b.toastDone = done
	b.lastToastText = message
	b.toastMu.Unlock()

	// showStatusToast is called from the UI thread (via NavigateTo), so the
	// initial text must be set directly — QueueUpdateDraw would deadlock
	// because it blocks until the event loop processes the function, but the
	// event loop is busy running this code.
	b.statusBar.SetText(message)

	go func() {
		defer close(done)
		select {
		case <-stop:
			return
		case <-time.After(hold):
			restore := func() { b.updateStatusBar() }
			if b.app != nil {
				// This runs on a background goroutine, so QueueUpdateDraw
				// is the correct way to touch the UI.
				b.app.QueueUpdateDraw(restore)
			} else {
				restore()
			}
		}
	}()
}
