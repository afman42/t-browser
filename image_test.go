package main

import (
	"testing"
)

func TestHasRealImageExtension(t *testing.T) {
	b := &Browser{}

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"jpg", "https://example.com/image.jpg", true},
		{"jpeg", "https://example.com/image.jpeg", true},
		{"png", "https://example.com/image.png", true},
		{"gif", "https://example.com/image.gif", true},
		{"bmp", "https://example.com/image.bmp", true},
		{"webp", "https://example.com/image.webp", true},
		{"svg", "https://example.com/image.svg", true},
		{"ico", "https://example.com/favicon.ico", true},
		{"tiff", "https://example.com/image.tiff", true},
		{"tif", "https://example.com/image.tif", true},
		{"no extension", "https://example.com/image", false},
		{"html page", "https://example.com/page.html", false},
		{"query params", "https://example.com/image.jpg?w=800", true},
		{"case insensitive", "https://example.com/image.JPG", true},
		{"mixed case", "https://example.com/image.PnG", true},
		{"dot in path", "https://example.com/files/image.png", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := b.hasRealImageExtension(tc.url)
			if got != tc.want {
				t.Errorf("hasRealImageExtension(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestHasRealImageExtensionEdgeCases(t *testing.T) {
	b := &Browser{}

	if b.hasRealImageExtension("") {
		t.Error("empty URL should not be an image")
	}
	if b.hasRealImageExtension("not-a-url") {
		t.Error("non-URL should not be an image")
	}
	if b.hasRealImageExtension(".txt") {
		t.Error(".txt should not be an image")
	}
}
