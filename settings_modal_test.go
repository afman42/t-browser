package main

import (
	"testing"

	"github.com/rivo/tview"
)

func testBrowserWithUI() *Browser {
	return &Browser{
		app:       tview.NewApplication(),
		tabs:      []*Tab{newTab()},
		urlInput:  tview.NewInputField(),
		statusBar: tview.NewTextView(),
		tabBar:    tview.NewTextView(),
		config:    &Config{},
	}
}

func TestShowSettingsModal(t *testing.T) {
	b := testBrowserWithUI()
	cfg := GetDefaultConfig()
	b.config = &cfg

	b.showSettingsModal()
	if !b.settingsActive {
		t.Error("showSettingsModal should set settingsActive")
	}
	if b.settingsChanged {
		t.Error("settingsChanged should start false")
	}
	if b.rightColumnEmpty {
		t.Error("rightColumnEmpty should start false")
	}
}

func TestBuildSettingsRightColumnAllCategories(t *testing.T) {
	b := testBrowserWithUI()
	cfg := GetDefaultConfig()
	b.config = &cfg
	left := tview.NewList()

	for _, cat := range b.getSettingCategories() {
		flex := b.buildSettingsRightColumn(cat, left)
		if flex == nil {
			t.Fatalf("category %q produced nil flex", cat.ID)
		}
		if b.findFormInFlex(flex) == nil {
			t.Errorf("category %q should produce a form in the right column", cat.ID)
		}
	}
}

func TestUpdateRightColumnSwapsContent(t *testing.T) {
	b := testBrowserWithUI()
	left := tview.NewList()
	flex := tview.NewFlex()

	cat := b.getSettingCategories()[0]
	b.updateRightColumn(flex, left, cat)
	if b.rightColumnEmpty {
		t.Error("updateRightColumn must clear rightColumnEmpty")
	}
	// findFormInFlex only scans one level; the form sits inside the nested
	// right-column flex (after the description view), so scan explicitly.
	found := false
	for i := 0; i < flex.GetItemCount(); i++ {
		if sub, ok := flex.GetItem(i).(*tview.Flex); ok {
			for j := 0; j < sub.GetItemCount(); j++ {
				if _, isForm := sub.GetItem(j).(*tview.Form); isForm {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("updated flex should contain a form inside the right column")
	}
}

func TestUpdateRightColumnForEmpty(t *testing.T) {
	b := testBrowserWithUI()
	left := tview.NewList()
	flex := tview.NewFlex()

	b.updateRightColumnForEmpty(flex, left)
	if !b.rightColumnEmpty {
		t.Error("updateRightColumnForEmpty should set rightColumnEmpty")
	}
	if b.findFormInFlex(flex) == nil {
		t.Error("empty column should still contain a form")
	}
}

func TestFindFormInFlex(t *testing.T) {
	form := tview.NewForm()
	flex := tview.NewFlex().AddItem(form, 0, 1, false)
	if b := testBrowserWithUI(); b.findFormInFlex(flex) != form {
		t.Error("findFormInFlex should find the nested form")
	}
	if got := testBrowserWithUI().findFormInFlex(tview.NewFlex()); got != nil {
		t.Error("findFormInFlex on form-less flex should return nil")
	}
}

func TestCloseSettingsModal(t *testing.T) {
	b := testBrowserWithUI()
	b.settingsActive = true

	b.closeSettingsModal()
	if b.settingsActive {
		t.Error("closeSettingsModal should clear settingsActive")
	}
}
