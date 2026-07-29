package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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
			return
		default:
			animationText := fmt.Sprintf("[yellow]%s %s[white]", phases[currentPhase], imageURL)

			b.app.QueueUpdateDraw(func() {
				if imageInfo != nil {
					imageInfo.SetText(animationText)
				}
			})

			currentPhase = (currentPhase + 1) % len(phases)
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

	// Create the image widget with 24-bit color support
	imgWidget := tview.NewImage()
	imgWidget.SetBorder(true)
	imgWidget.SetTitle("Image Preview - Press 'q' or ESC to close, 's' to scale")

	// Create a Flex layout for the image preview
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(imageInfo, 3, 0, false). // Show image URL at top
		AddItem(imgWidget, 0, 1, true)   // Show image in middle

	// Create a stop channel for the loading animation
	loadingStop := make(chan struct{})

	// Start the animated loading
	go b.showAnimatedImageLoading(imageInfo, imageURL, loadingStop)

	// Load the image in a goroutine to prevent blocking
	go func() {
		// First check content length with a HEAD request to avoid downloading large files
		headResp, err := http.Head(imageURL)
		if err != nil {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error getting image info: %v", err))
				imgWidget.SetImage(nil)
			})
			return
		}
		defer headResp.Body.Close()

		// Check if the response is actually an image
		contentType := headResp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("URL does not point to an image (content type: %s)", contentType))
				imgWidget.SetImage(nil)
			})
			return
		}

		// Check content length (size) - max 5MB
		contentLength := headResp.Header.Get("Content-Length")
		if contentLength != "" {
			var size int64
			fmt.Sscanf(contentLength, "%d", &size)
			if size > 5*1024*1024 {
				close(loadingStop)
				b.app.QueueUpdateDraw(func() {
					imageInfo.SetText(fmt.Sprintf("Image is too large (%.2f MB > 5 MB)", float64(size)/(1024*1024)))
					imgWidget.SetImage(nil)
				})
				return
			}
		}

		// Load the image
		resp, err := http.Get(imageURL)
		if err != nil {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error loading image: %v", err))
				imgWidget.SetImage(nil)
			})
			return
		}
		defer resp.Body.Close()

		// Read the image data with size limit
		imgData, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
		if err != nil {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error reading image: %v", err))
				imgWidget.SetImage(nil)
			})
			return
		}

		if len(imgData) >= 5*1024*1024 {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText("Image is too large (exceeds 5 MB limit)")
				imgWidget.SetImage(nil)
			})
			return
		}

		// Decode the image
		img, format, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error decoding image (content-type: %s): %v", contentType, err))
				imgWidget.SetImage(nil)
			})
			return
		}

		close(loadingStop)

		b.app.QueueUpdateDraw(func() {
			imgWidget.SetImage(img)
			imageInfo.SetText(fmt.Sprintf("Image loaded: %s (Format: %s, Size: %dx%d)", imageURL, format, img.Bounds().Dx(), img.Bounds().Dy()))
		})
	}()

	// Set up key handling for the image preview with scaling support
	var scaleFactor float64 = 1.0
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			b.showImagesModal()
			return nil
		}
		if event.Rune() == 's' {
			// Cycle scaling: 1.0 -> 1.5 -> 2.0 -> 0.5 -> 1.0
			if scaleFactor < 1.0 {
				scaleFactor = 1.0
			} else if scaleFactor < 1.5 {
				scaleFactor = 1.5
			} else if scaleFactor < 2.0 {
				scaleFactor = 2.0
			} else {
				scaleFactor = 0.5
			}
			// Scale support removed - tview.Image doesn't have SetScale
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Scale: %.1fx (not supported)", scaleFactor))
			})
			return nil
		}
		return event
	})

	b.app.SetRoot(flex, true)
}

// showImageErrorModal shows an error modal for image operations
func (b *Browser) showImageErrorModal(errorMessage string) {
	errorModal := tview.NewModal()
	errorModal.SetBorder(true)
	errorModal.SetTitle("Image Error")
	errorModal.SetText(errorMessage)
	errorModal.AddButtons([]string{"OK"})

	errorModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		b.showLinksModal()
	})

	errorModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
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
		b.showLinksModal()
	})

	successModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			b.showLinksModal()
			return nil
		}
		return event
	})

	b.app.SetRoot(successModal, true)
}
