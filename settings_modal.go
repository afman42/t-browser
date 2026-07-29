package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showSettingsModal displays the settings page with two-column layout
func (b *Browser) showSettingsModal() {
	// Create the main flex container for two-column layout
	flex := tview.NewFlex()

	// Create left column with list of setting categories
	leftList := tview.NewList()
	leftList.SetBorder(true)
	leftList.SetTitle("Settings Categories")
	leftList.ShowSecondaryText(false)

	// Add categories to the list
	categories := b.getSettingCategories()
	for i, category := range categories {
		leftList.AddItem(fmt.Sprintf("%s %s", category.Icon, category.Name), "", rune('1'+i), func(cat SettingCategory) func() {
			return func() {
				b.settingsChanged = true
				b.updateRightColumn(flex, leftList, cat)
			}
		}(category))
	}

	// Add close button at the bottom
	leftList.AddItem("Close", "Return to browser", 'c', func() {
		b.closeSettingsModal()
	})

	// Create right column with form for settings
	rightForm := tview.NewForm()
	rightForm.SetBorder(true)
	rightForm.SetTitle("Settings")

	// Set up the flex layout with left and right columns
	flex.SetDirection(tview.FlexColumn).
		AddItem(leftList, 30, 1, true). // Left column (30% width, focusable)
		AddItem(rightForm, 0, 4, false) // Right column (remaining width, not focusable initially)

	// Set initial focus to left list
	b.app.SetFocus(leftList)

	// Set up keyboard shortcuts
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC, tcell.KeyEscape:
			b.closeSettingsModal()
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'q' {
				b.closeSettingsModal()
				return nil
			}
		}
		return event
	})

	// Store the current state
	b.settingsActive = true
	b.settingsChanged = false
	b.rightColumnEmpty = false

	// Set the layout as root and start
	b.app.SetRoot(flex, true)
}

// updateRightColumn updates the right column with settings for the selected category
func (b *Browser) updateRightColumn(flex *tview.Flex, leftList *tview.List, category SettingCategory) {
	rightForm := tview.NewForm()
	rightForm.SetBorder(true)
	rightForm.SetTitle(fmt.Sprintf("%s Settings", category.Name))

	settings := b.getSettingsForCategory(category.ID)

	for _, setting := range settings {
		switch setting.Type {
		case "bool":
			value, ok := setting.Value.(bool)
			if !ok {
				value = false
			}
			rightForm.AddCheckbox(setting.Name, value, func(checked bool) {
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
			rightForm.AddInputField(setting.Name, valueStr, 20, nil, func(text string) {
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
			rightForm.AddPasswordField(setting.Name, value, 40, '*', nil)
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
				rightForm.AddDropDown(setting.Name, []string{"dark", "light"}, currentOption, func(option string, optionIndex int) {
					b.updateSettingValue(setting.ID, option)
					b.settingsChanged = true
				})
			} else {
				value, ok := setting.Value.(string)
				if !ok {
					value = ""
				}
				rightForm.AddInputField(setting.Name, value, 40, nil, func(text string) {
					b.updateSettingValue(setting.ID, text)
					b.settingsChanged = true
				})
			}
		}
		rightForm.AddTextView("", setting.Description, 0, 1, false, false)
	}

	rightForm.AddButton("Save", func() {
		b.saveSettings()
	})
	rightForm.AddButton("Cancel", func() {
		b.closeSettingsModal()
	})

	itemCount := rightForm.GetFormItemCount() + rightForm.GetButtonCount()

	rightForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTAB:
			if event.Modifiers() == tcell.ModShift {
				return event
			}
			b.app.SetFocus(leftList)
			return nil
		case tcell.KeyUp:
			formItem, btn := rightForm.GetFocusedItemIndex()
			current := formItem
			if formItem < 0 {
				current = rightForm.GetFormItemCount() + btn
			}
			if current > 0 {
				rightForm.SetFocus(current - 1)
			}
			return nil
		case tcell.KeyDown:
			formItem, btn := rightForm.GetFocusedItemIndex()
			current := formItem
			if formItem < 0 {
				current = rightForm.GetFormItemCount() + btn
			}
			if current < itemCount-1 {
				rightForm.SetFocus(current + 1)
			}
			return nil
		case tcell.KeyEnter:
			formItem, _ := rightForm.GetFocusedItemIndex()
			if formItem < 0 {
				b.saveSettings()
				b.closeSettingsModal()
				return nil
			}
		}
		return event
	})

	leftList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTAB:
			if !b.rightColumnEmpty {
				b.app.SetFocus(rightForm)
			}
			return nil
		}
		return event
	})

	flex.Clear().
		AddItem(leftList, 30, 1, true).
		AddItem(rightForm, 0, 4, false)

	b.rightColumnEmpty = false
	b.app.SetFocus(rightForm)
}

// updateRightColumnForEmpty updates the right column to show empty content and sets focus to left column
func (b *Browser) updateRightColumnForEmpty(flex *tview.Flex, leftList *tview.List) {
	// Create an empty form for the right column
	emptyForm := tview.NewForm()
	emptyForm.SetBorder(true)
	emptyForm.SetTitle("Settings")

	// Set up Tab key capture for the empty form to switch back to the left list
	emptyForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			b.app.SetFocus(leftList)
			return nil
		}
		return event
	})

	// Remove the old right column and add the empty one
	flex.Clear().
		AddItem(leftList, 30, 1, true).
		AddItem(emptyForm, 0, 4, false)

	// Set focus to the left list as requested and mark right column as empty
	b.rightColumnEmpty = true
	b.app.SetFocus(leftList)
}

// closeSettingsModal closes the settings modal and returns to browser
func (b *Browser) closeSettingsModal() {
	b.settingsActive = false
	b.app.SetRoot(b.mainFlex(), true)
	b.app.SetFocus(b.currentTab().textView)
}
