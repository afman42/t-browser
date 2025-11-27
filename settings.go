package main

import (
	"fmt"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SettingCategory represents a category of settings
type SettingCategory struct {
	ID   string
	Name string
	Icon string
}

// Setting represents an individual setting
type Setting struct {
	ID          string
	Name        string
	Description string
	Value       interface{}
	Type        string // "bool", "string", "int", "password"
}

// getSettingCategories returns all available setting categories
func (b *Browser) getSettingCategories() []SettingCategory {
	return []SettingCategory{
		{ID: "browser", Name: "Browser Settings", Icon: "🌐"},
		{ID: "network", Name: "Network Settings", Icon: "🌐"},
		{ID: "content", Name: "Content Settings", Icon: "📄"},
		{ID: "ui", Name: "UI Settings", Icon: "🎨"},
		{ID: "privacy", Name: "Privacy Settings", Icon: "🛡️"},
		{ID: "advanced", Name: "Advanced Settings", Icon: "⚙️"},
	}
}

// getSettingsForCategory returns all settings for the specified category
func (b *Browser) getSettingsForCategory(categoryID string) []Setting {
	switch categoryID {
	case "browser":
		return []Setting{
			{ID: "user_agent", Name: "User Agent", Description: "Custom user agent string for HTTP requests", Value: b.config.UserAgent, Type: "string"},
			{ID: "request_timeout", Name: "Request Timeout", Description: "Time in seconds before request timeout", Value: b.config.RequestTimeout, Type: "int"},
		}
	case "network":
		return []Setting{
			{ID: "proxy", Name: "Proxy Server", Description: "Proxy server URL (e.g., http://proxy:port)", Value: b.config.Proxy, Type: "string"},
			{ID: "max_redirects", Name: "Max Redirects", Description: "Maximum number of HTTP redirects to follow", Value: b.config.MaxRedirects, Type: "int"},
		}
	case "content":
		return []Setting{
			{ID: "max_page_size", Name: "Max Page Size", Description: "Maximum size (in MB) for downloaded pages", Value: b.config.MaxPageSize / (1024 * 1024), Type: "int"},
			{ID: "max_image_size", Name: "Max Image Size", Description: "Maximum size (in MB) for images", Value: b.config.MaxImageSize / (1024 * 1024), Type: "int"},
			{ID: "enable_images", Name: "Enable Images", Description: "Enable or disable image loading", Value: b.config.EnableImages, Type: "bool"},
		}
	case "ui":
		return []Setting{
			{ID: "theme", Name: "Theme", Description: "Color theme for the interface", Value: b.config.Theme, Type: "string"},
			{ID: "show_images", Name: "Show Images", Description: "Display images in terminal", Value: b.config.ShowImages, Type: "bool"},
			{ID: "word_wrap", Name: "Word Wrap", Description: "Enable text word wrapping", Value: b.config.WordWrap, Type: "bool"},
		}
	case "privacy":
		return []Setting{
			{ID: "enable_cookies", Name: "Enable Cookies", Description: "Enable or disable cookie storage", Value: b.config.EnableCookies, Type: "bool"},
			{ID: "cookie_auto_save", Name: "Auto Save Cookies", Description: "Enable automatic cookie saving", Value: b.config.CookieAutoSave, Type: "bool"},
			{ID: "session_auto_save", Name: "Auto Save Session", Description: "Enable automatic session saving", Value: b.config.SessionAutoSave, Type: "bool"},
		}
	case "advanced":
		return []Setting{
			{ID: "cookie_file", Name: "Cookie File", Description: "Path to store persistent cookies", Value: b.config.CookieFile, Type: "string"},
			{ID: "session_file", Name: "Session File", Description: "Path to store session data", Value: b.config.SessionFile, Type: "string"},
		}
	default:
		return []Setting{}
	}
}

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
				// Mark that there are now buttons available (selected a category)
				b.settingsChanged = true
				// Update the right column with settings for selected category
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
	// Create a new form for the right column
	rightForm := tview.NewForm()
	rightForm.SetBorder(true)
	rightForm.SetTitle(fmt.Sprintf("%s Settings", category.Name))

	// Get settings for the selected category
	settings := b.getSettingsForCategory(category.ID)

	// Add form items for each setting
	for _, setting := range settings {
		switch setting.Type {
		case "bool":
			// Add a checkbox for boolean settings
			value, ok := setting.Value.(bool)
			if !ok {
				value = false
			}
			rightForm.AddCheckbox(setting.Name, value, func(checked bool) {
				// Store the changed value temporarily and mark settings as changed
				b.updateSettingValue(setting.ID, checked)
				b.settingsChanged = true
				// Rebuild the form to show the save/cancel buttons
				b.updateRightColumn(flex, leftList, category)
			})
		case "int":
			// Add an input field for integer settings
			var valueStr string
			switch v := setting.Value.(type) {
			case int:
				valueStr = strconv.Itoa(v)
			case int64:
				valueStr = strconv.FormatInt(v, 10)
			default:
				valueStr = "0"
			}
			rightForm.AddInputField(setting.Name, valueStr, 20, nil, func(text string) {
				// Try to parse the integer value
				if intVal, err := strconv.Atoi(text); err == nil {
					b.updateSettingValue(setting.ID, intVal)
					b.settingsChanged = true
					// Rebuild the form to show the save/cancel buttons
					b.updateRightColumn(flex, leftList, category)
				}
			})
		case "password":
			// Add a password field
			value, ok := setting.Value.(string)
			if !ok {
				value = ""
			}
			rightForm.AddPasswordField(setting.Name, value, 40, '*', nil)
		default: // string
			// Add an input field for string settings
			value, ok := setting.Value.(string)
			if !ok {
				value = ""
			}
			rightForm.AddInputField(setting.Name, value, 40, nil, func(text string) {
				b.updateSettingValue(setting.ID, text)
				b.settingsChanged = true
				// Rebuild the form to show the save/cancel buttons
				b.updateRightColumn(flex, leftList, category)
			})
		}
		// Add the description as a static text item
		rightForm.AddTextView(setting.Name, setting.Description, 0, 1, false, false)
	}

	// Add save and cancel buttons only if settings have been changed
	if b.settingsChanged {
		rightForm.AddButton("Save", func() {
			b.saveSettings()
			b.closeSettingsModal()
		})
		rightForm.AddButton("Cancel", func() {
			// Reset the settingsChanged flag to hide the buttons
			b.settingsChanged = false
			// Update the right column to show empty content
			b.updateRightColumnForEmpty(flex, leftList)
		})
	}

	// Set up input capture for the form to handle navigation between form elements
	rightForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTAB:
			if event.Modifiers() == tcell.ModShift {
				// Shift+Tab moves focus to previous element or to left list
				return event
			} else {
				// Plain Tab moves focus to left list
				b.app.SetFocus(leftList)
				return nil
			}
		}
		return event
	})

	// Set up input capture for the left list to handle navigation between categories and switching to form
	leftList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTAB:
			// Only switch to right form if it's not empty
			if !b.rightColumnEmpty {
				b.app.SetFocus(rightForm)
			}
			return nil
		}
		return event
	})

	// Remove the old right column and add the new one
	flex.Clear().
		AddItem(leftList, 30, 1, true).
		AddItem(rightForm, 0, 4, false)

	// Set focus to the form so user can immediately start editing
	b.rightColumnEmpty = false  // Mark that the right column is no longer empty
	b.app.SetFocus(rightForm)
}

// updateSettingValue updates a setting value temporarily
func (b *Browser) updateSettingValue(settingID string, value interface{}) {
	// For now, we'll just store the changes in a temporary config
	// In a real implementation, we'd want to store these changes until user saves
	switch settingID {
	case "user_agent":
		if strVal, ok := value.(string); ok {
			b.config.UserAgent = strVal
		}
	case "request_timeout":
		if intVal, ok := value.(int); ok {
			b.config.RequestTimeout = intVal
		}
	case "proxy":
		if strVal, ok := value.(string); ok {
			b.config.Proxy = strVal
		}
	case "max_redirects":
		if intVal, ok := value.(int); ok {
			b.config.MaxRedirects = intVal
		}
	case "max_page_size":
		if intVal, ok := value.(int); ok {
			b.config.MaxPageSize = int64(intVal) * 1024 * 1024 // Convert MB to bytes
		}
	case "max_image_size":
		if intVal, ok := value.(int); ok {
			b.config.MaxImageSize = int64(intVal) * 1024 * 1024 // Convert MB to bytes
		}
	case "enable_images":
		if boolVal, ok := value.(bool); ok {
			b.config.EnableImages = boolVal
		}
	case "theme":
		if strVal, ok := value.(string); ok {
			b.config.Theme = strVal
		}
	case "show_images":
		if boolVal, ok := value.(bool); ok {
			b.config.ShowImages = boolVal
		}
	case "word_wrap":
		if boolVal, ok := value.(bool); ok {
			b.config.WordWrap = boolVal
		}
	case "enable_cookies":
		if boolVal, ok := value.(bool); ok {
			b.config.EnableCookies = boolVal
		}
	case "cookie_auto_save":
		if boolVal, ok := value.(bool); ok {
			b.config.CookieAutoSave = boolVal
		}
	case "session_auto_save":
		if boolVal, ok := value.(bool); ok {
			b.config.SessionAutoSave = boolVal
		}
	case "cookie_file":
		if strVal, ok := value.(string); ok {
			b.config.CookieFile = strVal
		}
	case "session_file":
		if strVal, ok := value.(string); ok {
			b.config.SessionFile = strVal
		}
	}
}

// saveSettings saves the current settings to the config file
func (b *Browser) saveSettings() {
	// Save the configuration to file
	configDir := GetConfigDir()
	if err := b.config.WriteToFile(configDir); err != nil {
		// Could show an error message here if needed
	}
}

// closeSettingsModal closes the settings modal and returns to browser
func (b *Browser) closeSettingsModal() {
	b.settingsActive = false

	// Restore the main browser UI
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.textView, 0, 1, false).
		AddItem(b.urlInput, 3, 0, false)

	b.app.SetRoot(flex, true)
	b.app.SetFocus(b.textView)
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
