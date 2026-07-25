package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// hasRealImageExtension checks if a URL has a real image file extension
func (b *Browser) hasRealImageExtension(url string) bool {
	// Strip query params and fragment before checking extension
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}
	if idx := strings.Index(url, "#"); idx != -1 {
		url = url[:idx]
	}

	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff", ".tif"}

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
		return false
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "image/") {
		return true
	}

	return false
}

// downloadImage downloads an image from URL to a temporary location
func (b *Browser) downloadImage(imageURL string) error {
	client := &http.Client{}

	resp, err := client.Get(imageURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("URL does not point to an image (content type: %s)", contentType)
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		var size int64
		fmt.Sscanf(contentLength, "%d", &size)
		if size > 5*1024*1024 {
			return fmt.Errorf("image too large (%d bytes > 5 MB)", size)
		}
	}

	imgData, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return err
	}

	if len(imgData) >= 5*1024*1024 {
		return fmt.Errorf("image is too large (exceeds 5 MB limit)")
	}

	return nil
}

