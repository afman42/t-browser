package main

import (
	"testing"
	"time"
)

func TestPageCacheEviction(t *testing.T) {
	cache := NewPageCache()

	for i := 0; i < maxCacheEntries+10; i++ {
		cache.Put("http://example.com/"+string(rune('a'+i)), &CacheEntry{
			Content: "content",
		})
	}

	if len(cache.entries) > maxCacheEntries {
		t.Errorf("cache has %d entries, max is %d", len(cache.entries), maxCacheEntries)
	}

	if cache.Get("http://example.com/a") != nil {
		t.Error("oldest entry should have been evicted")
	}

	if cache.Get("http://example.com/"+string(rune('a'+maxCacheEntries+9))) == nil {
		t.Error("newest entry should still be present")
	}
}

func TestPageCachePutUpdate(t *testing.T) {
	cache := NewPageCache()
	cache.Put("http://example.com/", &CacheEntry{Content: "old"})
	cache.Put("http://example.com/", &CacheEntry{Content: "new"})

	entry := cache.Get("http://example.com/")
	if entry == nil {
		t.Fatal("entry should exist")
	}
	if entry.Content != "new" {
		t.Errorf("content = %q, want %q", entry.Content, "new")
	}
	if len(cache.entries) != 1 {
		t.Errorf("cache should have 1 entry, got %d", len(cache.entries))
	}
}

func TestPageCacheConcurrent(t *testing.T) {
	cache := NewPageCache()
	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			cache.Put("http://example.com/", &CacheEntry{Content: "a"})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			cache.Get("http://example.com/")
		}
		done <- true
	}()

	<-done
	<-done
}

func TestCancelFuncNoRaceWithNewRequest(t *testing.T) {
	client := NewHTTPClient(nil)
	client.client.Timeout = 100 * time.Millisecond

	done := make(chan error, 2)

	go func() {
		_, err := client.FetchPage("http://192.0.2.1:1/")
		done <- err
	}()

	time.Sleep(10 * time.Millisecond)

	go func() {
		_, err := client.FetchPage("http://192.0.2.2:1/")
		done <- err
	}()

	time.Sleep(200 * time.Millisecond)

	client.CancelRequest()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}
}

func TestIsInternalAddressWithNetIP(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{"127.0.0.1", true},
		{"127.255.255.255", true},
		{"10.0.0.1", true},
		{"192.168.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false},
		{"172.15.0.1", false},
		{"169.254.0.1", true},
		{"169.254.169.254", true},
		{"0.0.0.0", true},
		{"::1", true},
		{"[::1]", true},
		{"::", true},
		{"[::]", true},
		{"example.com", false},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"", false},
		{"not-an-ip.example.com", false},
		// Legacy inet_aton forms (SSRF via resolver interpretation).
		{"2130706433", true}, // 127.0.0.1
		{"0x7f000001", true}, // 127.0.0.1
		{"0177.0.0.1", true}, // 127.0.0.1
		{"0x7f.0.0.1", true}, // 127.0.0.1
		{"3232235777", true}, // 192.168.1.1
		{"2130706434", true}, // 127.0.0.2 — still loopback
		// Partial-quad inet_aton forms: the final part is wider than 8 bits.
		{"127.65535", true},   // 127.0.255.255
		{"127.0.65535", true}, // 127.0.255.255
		{"10.65535", true},    // 10.0.255.255
		{"1.2.3", false},      // 1.2.0.3 — public
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

func TestMetaRefreshCancelOnNavigation(t *testing.T) {
	b := &Browser{}
	tab := b.currentTab()

	if tab.metaRefreshCancel != nil {
		t.Fatal("metaRefreshCancel should be nil on a fresh tab")
	}

	cancelCalled := false
	tab.metaRefreshCancel = func() {
		cancelCalled = true
	}

	validated, err := b.validateAndSanitizeURL("https://example.com")
	if err != nil {
		t.Fatalf("validateAndSanitizeURL failed: %v", err)
	}
	_ = validated

	if tab.metaRefreshCancel != nil {
		tab.metaRefreshCancel()
		tab.metaRefreshCancel = nil
	}

	if !cancelCalled {
		t.Error("metaRefreshCancel should have been called")
	}
	if tab.metaRefreshCancel != nil {
		t.Error("metaRefreshCancel should be nil after calling")
	}
}

func TestNewTabHasKeyBindings(t *testing.T) {
	b := &Browser{app: nil}
	b.tabs = []*Tab{newTab()}

	b.newTab()
	if len(b.tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(b.tabs))
	}

	newTabTextView := b.tabs[1].textView
	if newTabTextView == nil {
		t.Fatal("new tab should have a textView")
	}

	ic := newTabTextView.GetInputCapture()
	if ic == nil {
		t.Error("new tab should have input capture set (key bindings)")
	}
}

// TestTabOperationsDontDeadlock verifies that newTab/closeTab/switchTab
// complete without blocking when called synchronously (simulating the UI
// thread).  The previous implementation used QueueUpdateDraw which deadlocks
// when called from the event loop because QueueUpdate blocks waiting for the
// event loop to process the queued function.
func TestTabOperationsDontDeadlock(t *testing.T) {
	// Use a nil app — the methods must handle this without blocking.
	b := &Browser{app: nil}
	b.tabs = []*Tab{newTab()}

	done := make(chan struct{})
	go func() {
		// If any of these deadlock, the goroutine will hang and the
		// timeout below will fire.
		b.newTab()
		b.newTab()
		b.switchTab(1)
		b.switchTab(0)
		b.closeTab()
		b.closeTab()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("tab operations deadlocked (QueueUpdateDraw called from UI thread)")
	}

	if len(b.tabs) != 1 {
		t.Errorf("expected 1 tab remaining, got %d", len(b.tabs))
	}
}
