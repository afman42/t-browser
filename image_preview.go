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

func (b *Browser) showImagePreview(imageURL string) {
	imageInfo := tview.NewTextView()
	imageInfo.SetTextColor(tcell.ColorWhite)
	imageInfo.SetBackgroundColor(tcell.ColorNavy)
	imageInfo.SetDynamicColors(true)
	imageInfo.SetText(fmt.Sprintf("[yellow]Loading... %s[white]", imageURL))
	imageInfo.SetBorder(true)
	imageInfo.SetTitle("Image Preview")

	imgWidget := tview.NewImage()
	imgWidget.SetBorder(true)
	imgWidget.SetTitle("Image Preview - q/Esc to close")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(imageInfo, 3, 0, false).
		AddItem(imgWidget, 0, 1, true)

	loadingStop := make(chan struct{})

	go b.showAnimatedImageLoading(imageInfo, imageURL, loadingStop)

	go func() {
		headResp, err := http.Head(imageURL)
		if err != nil {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error getting image info: %v", err))
			})
			return
		}
		defer headResp.Body.Close()

		contentType := headResp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("URL does not point to an image (content type: %s)", contentType))
			})
			return
		}

		contentLength := headResp.Header.Get("Content-Length")
		if contentLength != "" {
			var size int64
			fmt.Sscanf(contentLength, "%d", &size)
			if size > 5*1024*1024 {
				close(loadingStop)
				b.app.QueueUpdateDraw(func() {
					imageInfo.SetText(fmt.Sprintf("Image is too large (%.2f MB > 5 MB)", float64(size)/(1024*1024)))
				})
				return
			}
		}

		resp, err := http.Get(imageURL)
		if err != nil {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error loading image: %v", err))
			})
			return
		}
		defer resp.Body.Close()

		imgData, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
		if err != nil {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error reading image: %v", err))
			})
			return
		}

		if len(imgData) >= 5*1024*1024 {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText("Image is too large (exceeds 5 MB limit)")
			})
			return
		}

		img, format, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			close(loadingStop)
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error decoding image (content-type: %s): %v", contentType, err))
			})
			return
		}

		close(loadingStop)

		b.app.QueueUpdateDraw(func() {
			imgWidget.SetImage(img)
			imageInfo.SetText(fmt.Sprintf("Image: %s | Format: %s | Size: %dx%d | q=close",
				imageURL, format, img.Bounds().Dx(), img.Bounds().Dy()))
		})
	}()

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			b.showImagesModal()
			return nil
		}
		return event
	})

	b.app.SetRoot(flex, true)
}
