package main

import (
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// createUI creates the terminal UI components
func (b *Browser) createUI() {
	// Text view to display web content
	b.textView = tview.NewTextView()
	b.textView.SetDynamicColors(true)
	b.textView.SetRegions(true)
	b.textView.SetWordWrap(true)
	b.textView.SetScrollable(true)
	b.textView.SetBorder(true)
	b.textView.SetTitle("Terminal Browser - Press Ctrl+C to quit, / for search")

	// Handle key events
	b.textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			b.app.Stop()
			return nil
		case tcell.KeyEnter:
			// For now, Enter doesn't follow links directly in content
			// Users should use the 'l' key to see the modal list of links
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				b.app.Stop()
				return nil
			case 'b': // Back button
				b.GoBack()
				return nil
			case 'f': // Forward button
				b.GoForward()
				return nil
			case '/': // Search
				b.startSearch()
				return nil
			case 'j': // Scroll down OR navigate to next link if modal is open
				// For scrolling content, just scroll down by 10 lines
				currentRow, _ := b.textView.GetScrollOffset()
				b.textView.ScrollTo(currentRow+10, 0)
				return nil
			case 'k': // Scroll up OR navigate to previous link if modal is open
				// For scrolling content, just scroll up by 10 lines (with minimum of 0)
				currentRow, _ := b.textView.GetScrollOffset()
				newRow := currentRow - 10
				if newRow < 0 {
					newRow = 0
				}
				b.textView.ScrollTo(newRow, 0)
				return nil
			case 'l': // Show list of all links in a modal
				if len(b.links) > 0 {
					b.showLinksModal()
				} else if len(b.images) > 0 {
					// If no links but images exist, show images modal
					b.showImagesModal()
				}
				return nil
			case 'i': // Show list of all images in a modal
				if len(b.images) > 0 {
					b.showImagesModal()
				}
				return nil
			case 'J': // Alternative: just scroll down regardless of links
				currentRow, _ := b.textView.GetScrollOffset()
				b.textView.ScrollTo(currentRow+10, 0)
				return nil
			case 'K': // Alternative: just scroll up regardless of links
				currentRow, _ := b.textView.GetScrollOffset()
				newRow := currentRow - 10
				if newRow < 0 {
					newRow = 0
				}
				b.textView.ScrollTo(newRow, 0)
				return nil
			case '?': // Show help/usage information
				b.showHelp()
				return nil
			case '\t': // Tab key to switch to URL input
				b.app.SetFocus(b.urlInput)
				return nil
			}
		}

		// Also handle tcell.KeyTAB for consistency
		if event.Key() == tcell.KeyTAB {
			b.app.SetFocus(b.urlInput)
			return nil
		}

		return event
	})

	// URL input field
	b.urlInput = tview.NewInputField()
	b.urlInput.SetBorder(true)
	b.urlInput.SetTitle("Enter URL")
	b.urlInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			url := b.urlInput.GetText()
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				url = "https://" + url
			}
			b.NavigateTo(url)
			b.app.SetFocus(b.textView)
		} else if key == tcell.KeyEscape {
			b.app.SetFocus(b.textView)
		}
	})
	// Capture tab to switch back to content view and enhance text input handling
	b.urlInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			b.app.SetFocus(b.textView)
			return nil // Consume the event
		}
		// Handle 'p' key to paste from clipboard
		if event.Key() == tcell.KeyRune && event.Rune() == 'p' {
			// Get text from clipboard
			clipText, err := clipboard.ReadAll()
			if err == nil {
				// Set the clipboard content to the input field
				b.urlInput.SetText(clipText)
			}
			return nil // Consume the event
		}
		return event
	})

	// Layout is set in the Run function
}

// showHelp displays help and usage information
func (b *Browser) showHelp() {
	helpText := `Terminal Browser - Help & Usage

Navigation:
  j     - Scroll down content
  k     - Scroll up content
  l     - Show modal list of all links on page (or images if no links)
  i     - Show modal list of all images on page
  Enter - Confirm selection in modal
  b     - Go back in history
  f     - Go forward in history

Link Handling:
  - Links marked with [IMAGE] for detected images, [IMAGE*] for real extensions
  - Press Enter on any link to navigate to that page
  - Image links preview directly in terminal when selected

Image Handling:
  - Supports formats: JPG, PNG, GIF, BMP, WebP, SVG, ICO, TIFF
  - Press 'i' to view all images on the current page
  - Each image shows alt text, title, URL, and file extension
  - Automatic size checking (max 5MB) to prevent large downloads
  - Images render directly in terminal using block characters
  - Press Enter on any image to preview it in terminal

Search:
  /     - Real-time search with match highlighting

Interface:
  Tab   - Switch between content view and URL input box
  ?     - Show this help information
  q     - Quit browser
  Ctrl+C - Quit browser

URL Input:
  - Type URL in the bottom input box and press Enter
  - Automatically adds 'https://' if no protocol specified
  - Press Tab to return to content view
  - For long URLs: use ← and → arrow keys to navigate within the input field
  - The input field shows a portion of long URLs; use arrow keys to see more

Clipboard:
  - Press 'p' when in URL input field to paste URL from clipboard

Loading Indicators:
  - Animated "Loading..." indicator appears when fetching pages
  - Shows progress during page loading
  - Automatically disappears when content loads

Security Features:
  - Blocks dangerous URL schemes (javascript:, data:, etc.)
  - Prevents access to local/internal addresses
  - Limits redirect chains to prevent loops
  - Sanitizes input to prevent formatting code injection
  - Size protection limits image downloads to 5MB

Content Processing:
  - Extracts main content and removes ads/navigation
  - Preserves basic formatting and structure
  - Identifies and presents all visible links in modal list
  - Supports multiple character encodings (UTF-8, Latin-1, etc.)

Accessibility Features:
  - Full keyboard navigation
  - Clear visual indicators
  - High contrast highlighting
  - Readable text formatting
  - Responsive controls

Press any key to close this help.`

	// Create a modal help view
	helpView := tview.NewTextView()
	helpView.SetTextColor(tcell.ColorWhite)
	helpView.SetBackgroundColor(tcell.ColorNavy)
	helpView.SetDynamicColors(true)
	helpView.SetRegions(false)
	helpView.SetText(helpText)
	helpView.SetDoneFunc(func(key tcell.Key) {
		// Close help when any key is pressed
		// Restore the proper flex layout with URL input
		flex := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(b.textView, 0, 1, false). // Main content area - takes remaining space
			AddItem(b.urlInput, 3, 0, false)  // URL input at the bottom - fixed height of 3

		b.app.SetRoot(flex, true)
		b.app.SetFocus(b.textView) // Ensure content view has focus after help
	})

	// Set the help view as root
	b.app.SetRoot(helpView, true)
}
