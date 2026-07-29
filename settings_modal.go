package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// settingsLeftColumnWidth returns the width for the left categories list,
// shrinking on narrow terminals so the right column has room to breathe.
func settingsLeftColumnWidth() int {
	return 24
}

// showSettingsModal displays the settings page with two-column layout
func (b *Browser) showSettingsModal() {
	flex := tview.NewFlex()

	leftList := tview.NewList()
	leftList.SetBorder(true)
	leftList.SetTitle("Settings Categories")
	leftList.ShowSecondaryText(false)

	categories := b.getSettingCategories()
	for i, category := range categories {
		leftList.AddItem(fmt.Sprintf("%s %s", category.Icon, category.Name), "", rune('1'+i), func(cat SettingCategory) func() {
			return func() {
				b.settingsChanged = true
				b.updateRightColumn(flex, leftList, cat)
			}
		}(category))
	}

	leftList.AddItem("Close", "Return to browser", 'c', func() {
		b.closeSettingsModal()
	})

	rightForm := tview.NewForm()
	rightForm.SetBorder(true)
	rightForm.SetTitle("Settings")

	flex.SetDirection(tview.FlexColumn).
		AddItem(leftList, settingsLeftColumnWidth(), 1, true).
		AddItem(rightForm, 0, 4, false)

	b.app.SetFocus(leftList)

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC, tcell.KeyEscape:
			b.closeSettingsModal()
			return nil
		case tcell.KeyCtrlS:
			b.saveSettings()
			b.closeSettingsModal()
			return nil
		case tcell.KeyCtrlQ:
			b.closeSettingsModal()
			return nil
		}
		return event
	})

	b.settingsActive = true
	b.settingsChanged = false
	b.rightColumnEmpty = false

	b.app.SetRoot(flex, true)
}

// buildSettingsRightColumn builds the right column for a settings category:
// a word-wrapping description TextView on top + the form below.  leftList is
// passed so the form's Tab handler can jump focus back to the categories.
func (b *Browser) buildSettingsRightColumn(category SettingCategory, leftList *tview.List) *tview.Flex {
	descView := tview.NewTextView()
	descView.SetDynamicColors(true)
	descView.SetWordWrap(true)
	descView.SetScrollable(true)
	descView.SetMaxLines(3)
	descView.SetTextColor(tcell.ColorYellow)
	descView.SetText("Select a setting to see its description.")

	rightForm := tview.NewForm()
	rightForm.SetBorder(true)
	rightForm.SetTitle(fmt.Sprintf("%s Settings", category.Name))

	settings := b.getSettingsForCategory(category.ID)

	for _, setting := range settings {
		label := setting.Name

		switch setting.Type {
		case "bool":
			value, ok := setting.Value.(bool)
			if !ok {
				value = false
			}
			rightForm.AddCheckbox(label, value, func(checked bool) {
				b.updateSettingValue(setting.ID, checked)
				b.settingsChanged = true
			})
		case "int":
			var valueStr string
			switch v := setting.Value.(type) {
			case int:
				valueStr = fmt.Sprintf("%d", v)
			case int64:
				valueStr = fmt.Sprintf("%d", v)
			default:
				valueStr = "0"
			}
			rightForm.AddInputField(label, valueStr, 20, nil, func(text string) {
				var intVal int
				if _, err := fmt.Sscanf(text, "%d", &intVal); err == nil {
					b.updateSettingValue(setting.ID, intVal)
					b.settingsChanged = true
				}
			})
		case "password":
			value, ok := setting.Value.(string)
			if !ok {
				value = ""
			}
			rightForm.AddPasswordField(label, value, 40, '*', nil)
		case "string":
			if setting.ID == "theme" {
				value, ok := setting.Value.(string)
				if !ok {
					value = "dark"
				}
				currentOption := 0
				if value == "light" {
					currentOption = 1
				}
				rightForm.AddDropDown(label, []string{"dark", "light"}, currentOption, func(option string, optionIndex int) {
					b.updateSettingValue(setting.ID, option)
					b.settingsChanged = true
				})
			} else {
				value, ok := setting.Value.(string)
				if !ok {
					value = ""
				}
				rightForm.AddInputField(label, value, 40, nil, func(text string) {
					b.updateSettingValue(setting.ID, text)
					b.settingsChanged = true
				})
			}
		}
	}

	rightForm.AddButton("Save", func() {
		b.saveSettings()
	})
	rightForm.AddButton("Cancel", func() {
		b.closeSettingsModal()
	})

	// Update the description view when the focused form item changes.
	updateDesc := func() {
		idx, _ := rightForm.GetFocusedItemIndex()
		if idx >= 0 && idx < len(settings) {
			s := settings[idx]
			text := s.Name
			if s.Description != "" {
				text += ": " + s.Description
			}
			descView.SetText(text)
		}
	}

	rightForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Tab (no Shift) jumps to the left categories list.
		// Shift+Tab also jumps to the left list.
		// We must intercept BEFORE the form's internal handler, which would
		// otherwise cycle between form fields and swallow the event.
		if isTabKey(event) && event.Modifiers()&tcell.ModShift == 0 {
			b.app.SetFocus(leftList)
			return nil
		}
		if isShiftTab(event) {
			b.app.SetFocus(leftList)
			return nil
		}

		switch event.Key() {
		case tcell.KeyUp:
			if event.Modifiers()&tcell.ModCtrl != 0 {
				formItem, btn := rightForm.GetFocusedItemIndex()
				current := formItem
				if formItem < 0 {
					current = rightForm.GetFormItemCount() + btn
				}
				if current > 0 {
					rightForm.SetFocus(current - 1)
				}
				updateDesc()
				return nil
			}
			updateDesc()
		case tcell.KeyDown:
			if event.Modifiers()&tcell.ModCtrl != 0 {
				formItem, btn := rightForm.GetFocusedItemIndex()
				current := formItem
				if formItem < 0 {
					current = rightForm.GetFormItemCount() + btn
				}
				total := rightForm.GetFormItemCount() + rightForm.GetButtonCount()
				if current < total-1 {
					rightForm.SetFocus(current + 1)
				}
				updateDesc()
				return nil
			}
			updateDesc()
		case tcell.KeyCtrlS:
			b.saveSettings()
			b.closeSettingsModal()
			return nil
		case tcell.KeyCtrlQ:
			b.closeSettingsModal()
			return nil
		}
		return event
	})

	// Show the first setting's description immediately.
	if len(settings) > 0 {
		s := settings[0]
		text := s.Name
		if s.Description != "" {
			text += ": " + s.Description
		}
		descView.SetText(text)
	}

	rightFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(descView, 4, 0, false).
		AddItem(rightForm, 0, 1, true)

	return rightFlex
}

// updateRightColumn updates the right column with settings for the selected category
func (b *Browser) updateRightColumn(flex *tview.Flex, leftList *tview.List, category SettingCategory) {
	leftWidth := settingsLeftColumnWidth()

	rightFlex := b.buildSettingsRightColumn(category, leftList)

	// Tab in the left list jumps to the right form.
	leftList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isTabKey(event) || isShiftTab(event) {
			if !b.rightColumnEmpty {
				if form := b.findFormInFlex(rightFlex); form != nil {
					b.app.SetFocus(form)
				}
				return nil
			}
		}
		return event
	})

	flex.Clear().
		AddItem(leftList, leftWidth, 1, true).
		AddItem(rightFlex, 0, 4, false)

	b.rightColumnEmpty = false
	if form := b.findFormInFlex(rightFlex); form != nil {
		b.app.SetFocus(form)
	}
}

// findFormInFlex searches a Flex's children for the first Form primitive.
func (b *Browser) findFormInFlex(f *tview.Flex) *tview.Form {
	for i := 0; i < f.GetItemCount(); i++ {
		item := f.GetItem(i)
		if form, ok := item.(*tview.Form); ok {
			return form
		}
	}
	return nil
}

// updateRightColumnForEmpty updates the right column to show empty content and sets focus to left column
func (b *Browser) updateRightColumnForEmpty(flex *tview.Flex, leftList *tview.List) {
	emptyForm := tview.NewForm()
	emptyForm.SetBorder(true)
	emptyForm.SetTitle("Settings")

	emptyForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if isTabKey(event) || isShiftTab(event) {
			b.app.SetFocus(leftList)
			return nil
		}
		return event
	})

	flex.Clear().
		AddItem(leftList, settingsLeftColumnWidth(), 1, true).
		AddItem(emptyForm, 0, 4, false)

	b.rightColumnEmpty = true
	b.app.SetFocus(leftList)
}

// closeSettingsModal closes the settings modal and returns to browser
func (b *Browser) closeSettingsModal() {
	b.settingsActive = false
	b.app.SetRoot(b.mainFlex(), true)
	b.app.SetFocus(b.currentTab().textView)
}
