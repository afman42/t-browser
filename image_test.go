package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestIsImageURLByExtension(t *testing.T) {
	b := &Browser{}
	for _, u := range []string{
		"https://example.com/photo.jpg",
		"https://example.com/photo.JPEG",
		"http://example.com/a.PNG?size=1",
		"https://example.com/x.webp#frag",
		"https://example.com/x.svg",
		"https://example.com/x.tiff",
	} {
		if !b.isImageURL(u) {
			t.Errorf("isImageURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{
		"https://example.com/page.html",
		"https://example.com/",
		"",
	} {
		if b.isImageURL(u) {
			t.Errorf("isImageURL(%q) = true, want false", u)
		}
	}
}

func TestIsImageURLByContentType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/img" {
			w.Header().Set("Content-Type", "image/png")
		} else {
			w.Header().Set("Content-Type", "text/html")
		}
	}))
	defer ts.Close()

	b := &Browser{}
	if !b.isImageURL(ts.URL + "/img") {
		t.Error("URL without extension served as image/png should be an image")
	}
	if b.isImageURL(ts.URL + "/page") {
		t.Error("URL served as text/html should not be an image")
	}
}

func TestIsImageURLConnectionError(t *testing.T) {
	b := &Browser{}
	if b.isImageURL("http://127.0.0.1:1/noext") {
		t.Error("unreachable URL should not be an image")
	}
}

func TestDownloadImage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/img.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("PNGDATA"))
		case "/text":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("hello"))
		case "/bigdeclared":
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", 6*1024*1024))
		case "/bigbody":
			w.Header().Set("Content-Type", "image/png")
			w.Write(make([]byte, 5*1024*1024+1))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	b := &Browser{}
	if err := b.downloadImage(ts.URL + "/img.png"); err != nil {
		t.Errorf("downloadImage(img) = %v, want nil", err)
	}
	if err := b.downloadImage(ts.URL + "/text"); err == nil {
		t.Error("downloadImage(text) should fail on non-image content type")
	}
	if err := b.downloadImage(ts.URL + "/bigdeclared"); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("downloadImage(bigdeclared) = %v, want too-large error", err)
	}
	if err := b.downloadImage(ts.URL + "/bigbody"); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("downloadImage(bigbody) = %v, want too-large error", err)
	}
}
