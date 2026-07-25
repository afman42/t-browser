package main

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// NavigateTo navigates to the specified URL
func (b *Browser) NavigateTo(url string) {
	// Validate the URL before processing
	validatedURL, err := b.validateAndSanitizeURL(url)
	if err != nil {
		// Since we're potentially in the UI thread, ensure UI updates are queued
		b.app.QueueUpdateDraw(func() {
			b.displayError(fmt.Sprintf("Invalid URL: %v", err))
		})
		return
	}

	// Add to history
	if b.currentURL != "" && (len(b.history) == 0 || b.history[len(b.history)-1] != b.currentURL) {
		b.history = append(b.history, b.currentURL)
		b.historyIndex = len(b.history) - 1
	}

	// Prepare the fetch operation in a separate goroutine to not block UI
	go func() {
		// Show loading indicator from the UI thread
		b.app.QueueUpdateDraw(func() {
			b.showLoadingIndicator()
		})

		// Small delay to ensure the loading indicator appears before fetching
		time.Sleep(30 * time.Millisecond)

		// Fetch the page content
		content, err := b.client.FetchPage(validatedURL)

		// Hide loading indicator and handle the result from the UI thread
		b.app.QueueUpdateDraw(func() {
			b.hideLoadingIndicator()

			if err != nil {
				b.displayError(fmt.Sprintf("Error fetching page: %v", err))
				return
			}

			// Update current URL
			b.currentURL = url

			// Render the page content
			b.renderPage(content, url)

			// Clear the title bar when navigating to a new page
			// The links will be refreshed, so reset current link index
			b.currentLinkIndex = -1
			b.updateTitleBar(-1)
		})
	}()
}

// validateAndSanitizeURL validates and sanitizes the input URL
func (b *Browser) validateAndSanitizeURL(inputURL string) (string, error) {
	// Basic validation to check for potentially malicious schemes
	if strings.HasPrefix(inputURL, "javascript:") || strings.HasPrefix(inputURL, "data:") ||
		strings.HasPrefix(inputURL, "vbscript:") || strings.HasPrefix(inputURL, "file:") {
		return "", fmt.Errorf("unsupported or dangerous URL scheme")
	}

	// Check if the URL is empty
	if strings.TrimSpace(inputURL) == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	// Check for excessive length to prevent potential buffer overflow
	if len(inputURL) > 2048 {
		return "", fmt.Errorf("URL is too long")
	}

	// Parse the URL to validate its structure
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %v", err)
	}

	// Validate the host to ensure it's not pointing to internal addresses
	host := parsedURL.Hostname()
	if host == "localhost" || strings.HasPrefix(host, "127.") ||
		strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") ||
		(strings.HasPrefix(host, "172.") && len(host) > 4 &&
			host[4] >= '1' && host[4] <= '3' && host[5] == '.') {
		// Allow these only if explicitly enabled, for security
		return "", fmt.Errorf("access to local/internal addresses not allowed")
	}

	// Check for suspicious patterns in the URL
	if strings.Contains(inputURL, "..") || strings.Contains(inputURL, "0x00") {
		return "", fmt.Errorf("URL contains suspicious patterns")
	}

	// Return the validated URL
	return inputURL, nil
}

// displayError shows an error message in the text view
func (b *Browser) displayError(message string) {
	b.textView.SetText(fmt.Sprintf("[red]Error: %s[-]", message))
}

// shouldDisableWordWrap analyzes the content to decide if word wrap should be disabled
// Returns true if the content has very long lines that would benefit from disabling word wrap
func (b *Browser) shouldDisableWordWrap(content string) bool {
	lines := strings.Split(content, "\n")

	// Check if there are many long lines
	longLineCount := 0
	totalLines := len(lines)

	for _, line := range lines {
		// If line is significantly longer than common terminal width (80+ chars)
		if len(line) > 120 {
			longLineCount++
		}
	}

	// If more than 20% of lines are very long, disable word wrap for performance
	if totalLines > 0 && float64(longLineCount)/float64(totalLines) > 0.2 {
		return true
	}

	// Additional check: if there are any extremely long lines (>500 chars)
	for _, line := range lines {
		if len(line) > 500 {
			return true
		}
	}

	return false
}

// updateWordWrapBasedOnContent dynamically sets word wrap based on content characteristics
func (b *Browser) updateWordWrapBasedOnContent(content string) {
	shouldDisableWrap := b.shouldDisableWordWrap(content)

	// Only update if the setting has changed to avoid unnecessary UI updates
	needsWrap := !shouldDisableWrap
	b.textView.SetWordWrap(needsWrap)
}

// updateTitleBar updates the title bar with the current link's URL
func (b *Browser) updateTitleBar(linkIndex int) {
	baseTitle := "Terminal Browser - Press Ctrl+C to quit, / for search"

	if linkIndex >= 0 && linkIndex < len(b.links) {
		linkURL := b.links[linkIndex].URL
		// Truncate long URLs to fit in title
		if len(linkURL) > 50 {
			linkURL = linkURL[:50] + "..."
		}
		b.textView.SetTitle(fmt.Sprintf("%s | Current Link: %s", baseTitle, linkURL))
	} else {
		b.textView.SetTitle(baseTitle)
	}
}

// showLoadingIndicator shows the animated loading indicator
func (b *Browser) showLoadingIndicator() {
	if b.isLoading {
		return // Already showing loading indicator
	}

	b.isLoading = true

	// Create the loading view
	b.loadingView = tview.NewTextView()
	b.loadingView.SetDynamicColors(true)
	b.loadingView.SetTextAlign(tview.AlignCenter)
	b.loadingView.SetBorder(true)
	b.loadingView.SetBackgroundColor(tcell.ColorBlue)
	b.loadingView.SetTextColor(tcell.ColorWhite)
	b.loadingView.SetTitle("Loading")

	// Set initial loading text
	b.loadingView.SetText("[yellow]Loading...[white]")

	// Replace the current view with loading indicator
	b.app.SetRoot(b.loadingView, true)

	// Force a draw to show the loading indicator immediately
	go func() {
		time.Sleep(10 * time.Millisecond) // Brief pause to allow UI setup
		b.app.Draw()
	}()

	// Start the animation in a separate goroutine
	go b.animateLoading()
}

// animateLoading updates the loading indicator with animation
func (b *Browser) animateLoading() {
	// Animation sequence: Loading, Loading., Loading.., Loading...
	phases := []string{"Loading", "Loading.", "Loading..", "Loading..."}
	currentPhase := 0

	for {
		select {
		case <-b.loadingStop:
			// Stop animation when loading is done
			return
		default:
			// Update the loading text with the current phase
			animationText := fmt.Sprintf("[yellow]%s[white]", phases[currentPhase])

			// Update the text in the main goroutine to prevent race conditions
			b.app.QueueUpdateDraw(func() {
				if b.loadingView != nil {
					b.loadingView.SetText(animationText)
				}
			})

			// Move to the next phase
			currentPhase = (currentPhase + 1) % len(phases)

			// Wait before updating again
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// showLoadingModal shows a temporary loading modal for modals when needed
func (b *Browser) showLoadingModal(title, message string) *tview.TextView {
	loadingView := tview.NewTextView()
	loadingView.SetDynamicColors(true)
	loadingView.SetTextAlign(tview.AlignCenter)
	loadingView.SetBorder(true)
	loadingView.SetBackgroundColor(tcell.ColorBlue)
	loadingView.SetTextColor(tcell.ColorWhite)
	loadingView.SetTitle(title)
	loadingView.SetText(message)

	b.app.SetRoot(loadingView, true)

	return loadingView
}

// hideLoadingIndicator hides the loading indicator and returns to the main view
func (b *Browser) hideLoadingIndicator() {
	if !b.isLoading {
		return
	}

	// Stop the animation
	close(b.loadingStop)

	// Create a new stop channel for future use
	b.loadingStop = make(chan struct{})

	// Reset loading flag
	b.isLoading = false

	// Return to the main view (textView)
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(b.textView, 0, 1, false). // Main content area - takes remaining space
		AddItem(b.urlInput, 3, 0, false)  // URL input at the bottom - fixed height of 3

	b.app.SetRoot(flex, true)

	// Ensure focus goes back to the main content after loading
	b.app.SetFocus(b.textView)
}
