package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// showAnimatedImageLoading shows animated loading text for image preview
func (b *Browser) showAnimatedImageLoading(imageInfo *tview.TextView, imageURL string, stopChan chan struct{}) {
	// Animation sequence: Loading, Loading., Loading.., Loading...
	phases := []string{"Loading", "Loading.", "Loading..", "Loading..."}
	currentPhase := 0

	for {
		select {
		case <-stopChan:
			// Stop animation
			return
		default:
			// Update the loading text with the current phase
			animationText := fmt.Sprintf("[yellow]%s %s[white]", phases[currentPhase], imageURL)

			// Update the text in the main goroutine to prevent race conditions
			b.app.QueueUpdateDraw(func() {
				if imageInfo != nil {
					imageInfo.SetText(animationText)
				}
			})

			// Move to the next phase
			currentPhase = (currentPhase + 1) % len(phases)

			// Wait before updating again
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// showImagePreview shows a modal with an actual image preview in terminal
func (b *Browser) showImagePreview(imageURL string) {
	// Create a TextView to show image info
	imageInfo := tview.NewTextView()
	imageInfo.SetTextColor(tcell.ColorWhite)
	imageInfo.SetBackgroundColor(tcell.ColorNavy)
	imageInfo.SetDynamicColors(true)
	imageInfo.SetText(fmt.Sprintf("[yellow]Loading... %s[white]", imageURL))
	imageInfo.SetBorder(true)
	imageInfo.SetTitle("Image Preview")

	// Create the image widget
	imgWidget := tview.NewImage()
	imgWidget.SetBorder(true)
	imgWidget.SetTitle("Image Preview - Press 'q' or ESC to close")

	// Create a Flex layout for the image preview
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(imageInfo, 3, 0, false).  // Show image URL at top
		AddItem(imgWidget, 0, 1, true)    // Show image in middle

	// Create a stop channel for the loading animation
	loadingStop := make(chan struct{})

	// Start the animated loading
	go b.showAnimatedImageLoading(imageInfo, imageURL, loadingStop)

	// Update image info to show loading
	go func() {
		// First check content length with a HEAD request to avoid downloading large files
		headResp, err := http.Head(imageURL)
		if err != nil {
			// Stop the animation
			close(loadingStop)

			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error getting image info: %v", err))
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}
		defer headResp.Body.Close()

		// Check if the response is actually an image
		contentType := headResp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
			// Stop the animation
			close(loadingStop)

			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("URL does not point to an image (content type: %s)", contentType))
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}

		// Check content length (size) - max 5MB (5 * 1024 * 1024 bytes = 5,242,880 bytes)
		contentLength := headResp.Header.Get("Content-Length")
		if contentLength != "" {
			var size int64
			fmt.Sscanf(contentLength, "%d", &size)
			if size > 5*1024*1024 { // 5MB limit
				// Stop the animation
				close(loadingStop)

				b.app.QueueUpdateDraw(func() {
					imageInfo.SetText(fmt.Sprintf("Image is too large (%.2f MB > 5 MB)", float64(size)/(1024*1024)))
					imgWidget.SetImage(nil) // Clear image
				})
				return
			}
		}

		// Load the image in a goroutine to prevent blocking
		resp, err := http.Get(imageURL)
		if err != nil {
			// Stop the animation
			close(loadingStop)

			// Show error using app.QueueUpdateDraw
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error loading image: %v", err))
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}
		defer resp.Body.Close()

		// Check if the response is actually an image again (in case it changed)
		// However, we should be more permissive since some servers return wrong content-types
		contentType = resp.Header.Get("Content-Type")

		// Read the image data with size limit
		imgData, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
		if err != nil {
			// Stop the animation
			close(loadingStop)

			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error reading image: %v", err))
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}

		// Check if we reached the size limit
		if len(imgData) >= 5*1024*1024 {
			// Stop the animation
			close(loadingStop)

			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText("Image is too large (exceeds 5 MB limit)")
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}

		// Decode the image - we'll try to decode it regardless of content-type header
		// since some servers return incorrect content-type headers
		img, format, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			// Stop the animation
			close(loadingStop)

			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error decoding image (content-type: %s): %v", contentType, err))
				imgWidget.SetImage(nil) // Clear image
			})
			return
		}

		// Stop the animation
		close(loadingStop)

		// Update the image widget with the decoded image
		b.app.QueueUpdateDraw(func() {
			imgWidget.SetImage(img)
			imageInfo.SetText(fmt.Sprintf("Image loaded: %s (Format: %s, Size: %dx%d)", imageURL, format, img.Bounds().Dx(), img.Bounds().Dy()))
		})
	}()

	// Set up key handling for the image preview
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			// Return to the images modal
			b.showImagesModal()
			return nil
		}
		return event
	})

	// Set the flex layout as root
	b.app.SetRoot(flex, true)
}

// hasRealImageExtension checks if a URL has a real image file extension
func (b *Browser) hasRealImageExtension(url string) bool {
	// Check file extension first
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff", ".tif"}

	// Convert URL to lower case for comparison
	lowerURL := strings.ToLower(url)

	for _, ext := range imageExtensions {
		if strings.HasSuffix(lowerURL, ext) {
			return true
		}
	}

	return false
}

// isImageURL checks if a URL points to an image based on extension or content type
func (b *Browser) isImageURL(url string) bool {
	// First check if it has a real image extension
	if b.hasRealImageExtension(url) {
		return true
	}

	// If no extension found in URL, try to check the content type by making a HEAD request
	resp, err := http.Head(url)
	if err != nil {
		// If we can't make the request, fall back to the extension check
		return false
	}
	defer resp.Body.Close()

	// Check the content type header
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "image/") {
		return true
	}

	return false
}

// downloadImage downloads an image from URL to a temporary location
func (b *Browser) downloadImage(imageURL string) error {
	// Create an HTTP client
	client := &http.Client{}

	// Make the GET request
	resp, err := client.Get(imageURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check if the response is an image
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("URL does not point to an image (content type: %s)", contentType)
	}

	// Check content length from the response if available
	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		var size int64
		fmt.Sscanf(contentLength, "%d", &size)
		if size > 5*1024*1024 { // 5MB limit
			return fmt.Errorf("image too large (%d bytes > 5 MB)", size)
		}
	}

	// Read the image data with size limit to prevent downloading very large files
	// Use the same 5MB limit as in showImagePreview
	imgData, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
	if err != nil {
		return err
	}

	// Check if we reached the size limit
	if len(imgData) >= 5*1024*1024 {
		return fmt.Errorf("image is too large (exceeds 5 MB limit)")
	}

	return nil
}

// showImageErrorModal shows an error modal for image operations
func (b *Browser) showImageErrorModal(errorMessage string) {
	errorModal := tview.NewModal()
	errorModal.SetBorder(true)
	errorModal.SetTitle("Image Error")
	errorModal.SetText(errorMessage)
	errorModal.AddButtons([]string{"OK"})

	errorModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		// Return to the links modal after showing error
		b.showLinksModal()
	})

	// Set up key handling for the modal
	errorModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			// Return to links list
			b.showLinksModal()
			return nil
		}
		return event
	})

	b.app.SetRoot(errorModal, true)
}

// showImageSuccessModal shows a success modal for image operations
func (b *Browser) showImageSuccessModal(successMessage string) {
	successModal := tview.NewModal()
	successModal.SetBorder(true)
	successModal.SetTitle("Success")
	successModal.SetText(successMessage)
	successModal.AddButtons([]string{"OK"})

	successModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		// Return to the links modal after showing success
		b.showLinksModal()
	})

	// Set up key handling for the modal
	successModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			// Return to links list
			b.showLinksModal()
			return nil
		}
		return event
	})

	b.app.SetRoot(successModal, true)
}