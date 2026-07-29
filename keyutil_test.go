package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestIsTabKey(t *testing.T) {
	// KeyTAB constant
	if !isTabKey(tcell.NewEventKey(tcell.KeyTAB, 0, tcell.ModNone)) {
		t.Error("isTabKey should be true for KeyTAB")
	}
	// '\t' rune (how terminals send Tab)
	if !isTabKey(tcell.NewEventKey(tcell.KeyRune, '\t', tcell.ModNone)) {
		t.Error("isTabKey should be true for KeyRune '\\t'")
	}
	// Not a tab
	if isTabKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)) {
		t.Error("isTabKey should be false for 'a'")
	}
	if isTabKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)) {
		t.Error("isTabKey should be false for Enter")
	}
}

func TestIsShiftTab(t *testing.T) {
	// KeyBacktab (Shift+Tab as a key constant)
	if !isShiftTab(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)) {
		t.Error("isShiftTab should be true for KeyBacktab")
	}
	// '\t' rune with ModShift
	if !isShiftTab(tcell.NewEventKey(tcell.KeyRune, '\t', tcell.ModShift)) {
		t.Error("isShiftTab should be true for '\\t' with ModShift")
	}
	// KeyTAB with ModShift
	if !isShiftTab(tcell.NewEventKey(tcell.KeyTAB, 0, tcell.ModShift)) {
		t.Error("isShiftTab should be true for KeyTAB with ModShift")
	}
	// Plain tab (no shift) is NOT shift-tab
	if isShiftTab(tcell.NewEventKey(tcell.KeyTAB, 0, tcell.ModNone)) {
		t.Error("isShiftTab should be false for plain Tab")
	}
	if isShiftTab(tcell.NewEventKey(tcell.KeyRune, '\t', tcell.ModNone)) {
		t.Error("isShiftTab should be false for plain '\\t'")
	}
}
