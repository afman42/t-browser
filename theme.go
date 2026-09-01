package main

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// darkTheme returns the tview theme for dark mode.
func darkTheme() tview.Theme {
	return tview.Theme{
		PrimitiveBackgroundColor:    tcell.ColorBlack,
		ContrastBackgroundColor:     tcell.ColorBlue,
		MoreContrastBackgroundColor: tcell.ColorGreen,
		BorderColor:                 tcell.ColorWhite,
		TitleColor:                  tcell.ColorWhite,
		GraphicsColor:               tcell.ColorWhite,
		PrimaryTextColor:            tcell.ColorWhite,
		SecondaryTextColor:          tcell.ColorYellow,
		TertiaryTextColor:           tcell.ColorGreen,
		InverseTextColor:            tcell.ColorYellow,
		ContrastSecondaryTextColor:  tcell.ColorNavy,
	}
}

// lightTheme returns the tview theme for light mode.
func lightTheme() tview.Theme {
	return tview.Theme{
		PrimitiveBackgroundColor:    tcell.ColorWhite,
		ContrastBackgroundColor:     tcell.ColorLightGray,
		MoreContrastBackgroundColor: tcell.ColorGray,
		BorderColor:                 tcell.ColorBlack,
		TitleColor:                  tcell.ColorBlack,
		GraphicsColor:               tcell.ColorBlack,
		PrimaryTextColor:            tcell.ColorBlack,
		SecondaryTextColor:          tcell.ColorBlue,
		TertiaryTextColor:           tcell.ColorGreen,
		InverseTextColor:            tcell.ColorBlue,
		ContrastSecondaryTextColor:  tcell.ColorNavy,
	}
}

// themeForName returns the theme for the given name, defaulting to dark.
func themeForName(name string) tview.Theme {
	switch name {
	case "light":
		return lightTheme()
	default:
		return darkTheme()
	}
}

// isLightTheme returns true if the configured theme is light.
func (b *Browser) isLightTheme() bool {
	return b.config != nil && b.config.Theme == "light"
}

// ensureContentVisibilityForTheme ensures content is readable in the current theme.
func (b *Browser) ensureContentVisibilityForTheme(content string) string {
	if b.isLightTheme() {
		// Coloured heading/quote codes read poorly on white; pin them to black.
		for _, col := range []string{"cyan", "yellow", "green", "blue", "magenta", "red"} {
			content = strings.ReplaceAll(content, "["+col+"::b]", "[black::b]")
		}
		return strings.ReplaceAll(content, "[::u]", "[black::u]")
	}
	return strings.ReplaceAll(content, "[::u]", "[white::u]")
}

// ApplyTheme applies the selected theme to the application.
func (b *Browser) ApplyTheme() {
	if b.config == nil {
		return
	}
	tview.Styles = themeForName(b.config.Theme)

	// Update textView appearance based on theme
	if b.currentTab().textView != nil {
		if b.isLightTheme() {
			b.currentTab().textView.SetBackgroundColor(tcell.ColorWhite)
		} else {
			b.currentTab().textView.SetBackgroundColor(tcell.ColorBlack)
		}
		// Refresh content to make sure it's visible in current theme
		if b.currentTab().originalUnprocessedContent != "" {
			processedContent := b.ensureContentVisibilityForTheme(b.currentTab().originalUnprocessedContent)
			b.currentTab().originalContent = processedContent
			b.currentTab().textView.SetText(processedContent)
		}

		borderColor := tcell.ColorWhite
		titleColor := tcell.ColorWhite
		if b.isLightTheme() {
			borderColor = tcell.ColorBlack
			titleColor = tcell.ColorBlack
		}
		b.currentTab().textView.SetBorderColor(borderColor)
		b.currentTab().textView.SetTitleColor(titleColor)
	}

	// Update urlInput appearance based on theme
	if b.urlInput != nil {
		if b.isLightTheme() {
			b.urlInput.SetBackgroundColor(tcell.ColorWhite)
			b.urlInput.SetBorderColor(tcell.ColorBlack)
			b.urlInput.SetTitleColor(tcell.ColorBlack)
			b.urlInput.SetFieldBackgroundColor(tcell.ColorLightGray)
			b.urlInput.SetFieldTextColor(tcell.ColorBlack)
		} else {
			b.urlInput.SetBackgroundColor(tcell.ColorBlack)
			b.urlInput.SetBorderColor(tcell.ColorWhite)
			b.urlInput.SetTitleColor(tcell.ColorWhite)
			b.urlInput.SetFieldBackgroundColor(tcell.ColorBlue)
			b.urlInput.SetFieldTextColor(tcell.ColorWhite)
		}
	}
}
