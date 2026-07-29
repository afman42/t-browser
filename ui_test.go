package main

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func TestResolveInputURL(t *testing.T) {
	cfg := GetDefaultConfig()

	tests := []struct {
		name   string
		input  string
		config *Config
		want   string
	}{
		{
			name:   "https URL passes through",
			input:  "https://example.com",
			config: &cfg,
			want:   "https://example.com",
		},
		{
			name:   "http URL passes through",
			input:  "http://example.com",
			config: &cfg,
			want:   "http://example.com",
		},
		{
			name:   "domain without scheme gets https",
			input:  "example.com",
			config: &cfg,
			want:   "https://example.com",
		},
		{
			name:   "domain with path gets https",
			input:  "example.com/page",
			config: &cfg,
			want:   "https://example.com/page",
		},
		{
			name:   "search query goes to search engine",
			input:  "how to test in go",
			config: &cfg,
			want:   "https://duckduckgo.com/html?q=how+to+test+in+go",
		},
		{
			name:   "empty input returns default",
			input:  "",
			config: &cfg,
			want:   "https://example.com",
		},
		{
			name:   "nil config uses default search engine",
			input:  "golang testing",
			config: nil,
			want:   "https://duckduckgo.com/html?q=golang+testing",
		},
		{
			name:   "custom search engine",
			input:  "test query",
			config: &Config{SearchEngine: "https://search.example.com/?q="},
			want:   "https://search.example.com/?q=test+query",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveInputURL(tc.input, tc.config)
			if got != tc.want {
				t.Errorf("resolveInputURL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestCurrentTabAutoCreates(t *testing.T) {
	b := &Browser{}
	tab := b.currentTab()
	if tab == nil {
		t.Fatal("currentTab() should auto-create a tab")
	}
	if len(b.tabs) != 1 {
		t.Errorf("expected 1 tab after currentTab(), got %d", len(b.tabs))
	}
	if tab.textView == nil {
		t.Error("auto-created tab should have a non-nil textView")
	}
}

func TestCurrentTabReturnsExistingTab(t *testing.T) {
	b := &Browser{}
	tab1 := b.currentTab()
	tab2 := b.currentTab()
	if tab1 != tab2 {
		t.Error("currentTab() should return the same tab on repeated calls")
	}
	if len(b.tabs) != 1 {
		t.Errorf("expected 1 tab, got %d", len(b.tabs))
	}
}

func TestNewTabCreatesAdditionalTab(t *testing.T) {
	b := &Browser{app: tview.NewApplication()}
	b.tabs = []*Tab{newTab()}
	b.activeTab = 0

	tab := newTab()
	b.tabs = append(b.tabs, tab)
	b.activeTab = len(b.tabs) - 1

	if len(b.tabs) != 2 {
		t.Errorf("expected 2 tabs, got %d", len(b.tabs))
	}
	if b.activeTab != 1 {
		t.Errorf("expected activeTab = 1, got %d", b.activeTab)
	}
}

func TestSwitchTab(t *testing.T) {
	b := &Browser{}
	for i := 0; i < 3; i++ {
		b.tabs = append(b.tabs, newTab())
	}
	b.activeTab = 0

	b.switchTab(2)
	if b.activeTab != 2 {
		t.Errorf("after switchTab(2), activeTab = %d, want 2", b.activeTab)
	}

	b.switchTab(1)
	if b.activeTab != 1 {
		t.Errorf("after switchTab(1), activeTab = %d, want 1", b.activeTab)
	}

	b.switchTab(5)
	if b.activeTab != 1 {
		t.Errorf("after switchTab(5), activeTab should remain 1, got %d", b.activeTab)
	}

	b.switchTab(-1)
	if b.activeTab != 1 {
		t.Errorf("after switchTab(-1), activeTab should remain 1, got %d", b.activeTab)
	}
}

func TestNextTab(t *testing.T) {
	b := &Browser{}
	for i := 0; i < 3; i++ {
		b.tabs = append(b.tabs, newTab())
	}
	b.activeTab = 0

	b.nextTab()
	if b.activeTab != 1 {
		t.Errorf("after nextTab(), activeTab = %d, want 1", b.activeTab)
	}

	b.nextTab()
	if b.activeTab != 2 {
		t.Errorf("after nextTab(), activeTab = %d, want 2", b.activeTab)
	}

	b.nextTab()
	if b.activeTab != 0 {
		t.Errorf("after nextTab() wrap, activeTab = %d, want 0", b.activeTab)
	}
}

func TestPrevTab(t *testing.T) {
	b := &Browser{}
	for i := 0; i < 3; i++ {
		b.tabs = append(b.tabs, newTab())
	}
	b.activeTab = 2

	b.prevTab()
	if b.activeTab != 1 {
		t.Errorf("after prevTab(), activeTab = %d, want 1", b.activeTab)
	}

	b.activeTab = 0
	b.prevTab()
	if b.activeTab != 2 {
		t.Errorf("after prevTab() wrap, activeTab = %d, want 2", b.activeTab)
	}
}

func TestUpdateTabBar(t *testing.T) {
	b := &Browser{app: tview.NewApplication()}
	b.tabBar = tview.NewTextView()

	b.tabs = []*Tab{
		{currentURL: "https://example.com"},
		{currentURL: "https://other.com/page"},
	}
	b.activeTab = 0

	b.updateTabBar()
	text := b.tabBar.GetText(false)
	if text == "" {
		t.Error("tab bar text should not be empty")
	}
	if !strings.Contains(text, "example.com") {
		t.Errorf("tab bar should contain 'example.com', got: %s", text)
	}
}
