package main

import "github.com/gdamore/tcell/v2"

// isTabKey reports whether the event is a Tab key press, checking both the
// KeyTAB constant and the '\t' rune (most terminals send Tab as the rune
// '\t', not as KeyTAB).
func isTabKey(event *tcell.EventKey) bool {
	return event.Key() == tcell.KeyTAB ||
		(event.Key() == tcell.KeyRune && event.Rune() == '\t')
}

// isShiftTab reports whether the event is a Shift+Tab (BackTab) press.
func isShiftTab(event *tcell.EventKey) bool {
	return event.Key() == tcell.KeyBacktab ||
		(isTabKey(event) && event.Modifiers()&tcell.ModShift != 0)
}
