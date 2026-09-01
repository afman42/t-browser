package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestDarkThemePalette(t *testing.T) {
	th := darkTheme()
	if th.PrimitiveBackgroundColor != tcell.ColorBlack {
		t.Error("dark background should be black")
	}
	if th.PrimaryTextColor != tcell.ColorWhite {
		t.Error("dark primary text should be white")
	}
	if th.InverseTextColor != tcell.ColorYellow {
		t.Error("dark inverse text should be yellow")
	}
	if th.TertiaryTextColor != tcell.ColorGreen {
		t.Error("dark tertiary text should be green")
	}
}

func TestLightThemePalette(t *testing.T) {
	th := lightTheme()
	if th.PrimitiveBackgroundColor != tcell.ColorWhite {
		t.Error("light background should be white")
	}
	if th.PrimaryTextColor != tcell.ColorBlack {
		t.Error("light primary text should be black")
	}
	if th.SecondaryTextColor != tcell.ColorBlue {
		t.Error("light secondary text should be blue")
	}
}

func TestThemeForName(t *testing.T) {
	if themeForName("light").PrimitiveBackgroundColor != tcell.ColorWhite {
		t.Error(`themeForName("light") should return the light palette`)
	}
	for _, name := range []string{"", "dark", "solarized", "unknown"} {
		if themeForName(name).PrimitiveBackgroundColor != tcell.ColorBlack {
			t.Errorf("themeForName(%q) should default to dark", name)
		}
	}
}

func TestIsLightTheme(t *testing.T) {
	if (&Browser{}).isLightTheme() {
		t.Error("nil config should not be light")
	}
	if !(&Browser{config: &Config{Theme: "light"}}).isLightTheme() {
		t.Error(`Theme "light" should be light`)
	}
	if (&Browser{config: &Config{Theme: "dark"}}).isLightTheme() {
		t.Error(`Theme "dark" should not be light`)
	}
}

func TestEnsureContentVisibilityForTheme(t *testing.T) {
	in := "[cyan::b]heading[::u]under[red::b]x"
	light := (&Browser{config: &Config{Theme: "light"}}).ensureContentVisibilityForTheme(in)
	if !strings.Contains(light, "[black::b]") {
		t.Errorf("light theme should pin colored bold to black, got %q", light)
	}
	if strings.Contains(light, "[cyan::b]") || strings.Contains(light, "[red::b]") {
		t.Errorf("light theme must not retain colored bold codes: %q", light)
	}
	if !strings.Contains(light, "[black::u]") {
		t.Errorf("light theme should pin underline to black: %q", light)
	}

	dark := (&Browser{config: &Config{Theme: "dark"}}).ensureContentVisibilityForTheme(in)
	if !strings.Contains(dark, "[cyan::b]") {
		t.Errorf("dark theme must keep colored bold codes: %q", dark)
	}
	if !strings.Contains(dark, "[white::u]") {
		t.Errorf("dark theme should pin underline to white: %q", dark)
	}
}

func TestApplyThemeAppliesStyles(t *testing.T) {
	saved := tview.Styles
	defer func() { tview.Styles = saved }()

	cfg := Config{Theme: "light"}
	b := &Browser{config: &cfg, urlInput: tview.NewInputField()}
	tab := b.currentTab()
	tab.originalUnprocessedContent = "[cyan::b]hi[::u]there"

	b.ApplyTheme()

	if tview.Styles.PrimitiveBackgroundColor != tcell.ColorWhite {
		t.Error("ApplyTheme(light) should switch global tview.Styles to light")
	}
	if got := tab.textView.GetBackgroundColor(); got != tcell.ColorWhite {
		t.Errorf("textView background = %v, want white", got)
	}
	// SetTitleColor has no getter; the re-rendered content proves it applied.
	if got := tab.textView.GetText(false); !strings.Contains(got, "[black::b]hi") {
		t.Errorf("light theme should re-render content with black bold, got %q", got)
	}

	cfg.Theme = "dark"
	b.ApplyTheme()

	if got := tab.textView.GetBackgroundColor(); got != tcell.ColorBlack {
		t.Errorf("textView background after dark = %v, want black", got)
	}
	if got := b.urlInput.GetBackgroundColor(); got != tcell.ColorBlack {
		t.Errorf("urlInput background after dark = %v, want black", got)
	}
}

func TestApplyThemeNilConfig(t *testing.T) {
	b := &Browser{}
	b.ApplyTheme() // must not panic
}
