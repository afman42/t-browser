package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
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

const maxImagePixels = 4096 * 4096 // anti-decompression-bomb ceiling

// transportForImages returns the app's transport (proxy/TLS/HSTS aware) or a
// fresh default when the browser has no client (tests, bare Browser).
func (b *Browser) transportForImages() http.RoundTripper {
	if b.client != nil {
		if t := b.client.transportSnapshot(); t != nil {
			return t
		}
	}
	return http.DefaultTransport
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
	imgCtx, imgCancel := context.WithCancel(context.Background())

	stopLoading := func() {
		select {
		case <-loadingStop:
		default:
			close(loadingStop)
		}
	}

	go b.showAnimatedImageLoading(imageInfo, imageURL, loadingStop)

	// Bounded client: 15 s timeout, app transport (proxy/pinning/HSTS), and a
	// context cancelled when the modal closes — no leaked goroutines, no
	// unbounded QueueUpdateDraw spam.
	imgClient := &http.Client{Transport: b.transportForImages(), Timeout: 15 * time.Second}

	go func() {
		defer imgCancel()
		defer stopLoading()

		// Same host policy as page fetches: image URLs are page-controlled and
		// must not reach internal/blocked addresses (SSRF via the modal).
		if imgParsed, err := url.Parse(imageURL); err != nil || (b.client != nil && b.client.checkRequestHost(imgParsed) != nil) {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText("[red]Blocked image URL: access to internal or blocked addresses not allowed[-]")
			})
			return
		}

		headReq, err := http.NewRequestWithContext(imgCtx, http.MethodHead, imageURL, nil)
		if err != nil {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error getting image info: %v", err))
			})
			return
		}
		headResp, err := imgClient.Do(headReq)
		if err != nil {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error getting image info: %v", err))
			})
			return
		}
		headResp.Body.Close()

		contentType := headResp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "image/") {
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
				b.app.QueueUpdateDraw(func() {
					imageInfo.SetText(fmt.Sprintf("Image is too large (%.2f MB > 5 MB)", float64(size)/(1024*1024)))
				})
				return
			}
		}

		getReq, err := http.NewRequestWithContext(imgCtx, http.MethodGet, imageURL, nil)
		if err != nil {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error loading image: %v", err))
			})
			return
		}
		resp, err := imgClient.Do(getReq)
		if err != nil {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error loading image: %v", err))
			})
			return
		}
		defer resp.Body.Close()

		imgData, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
		if err != nil {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error reading image: %v", err))
			})
			return
		}
		if len(imgData) >= 5*1024*1024 {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText("Image is too large (exceeds 5 MB limit)")
			})
			return
		}

		img, format, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Error decoding image (content-type: %s): %v", contentType, err))
			})
			return
		}
		// Decompression-bomb guard: a small compressed PNG can decode into a
		// multi-gigapixel bitmap; reject before handing it to the widget.
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		if int64(w)*int64(h) > maxImagePixels {
			b.app.QueueUpdateDraw(func() {
				imageInfo.SetText(fmt.Sprintf("Image dimensions too large (%dx%d)", w, h))
			})
			return
		}

		b.app.QueueUpdateDraw(func() {
			imgWidget.SetImage(img)
			imageInfo.SetText(fmt.Sprintf("Image: %s | Format: %s | Size: %dx%d | q=close",
				imageURL, format, w, h))
		})
	}()

	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'q') {
			imgCancel()
			stopLoading()
			b.showImagesModal()
			return nil
		}
		return event
	})

	b.app.SetRoot(flex, true)
}
