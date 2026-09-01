package main

import (
	"strings"
	"testing"
)

func TestFilterLinksBy(t *testing.T) {
	links := []Link{
		{URL: "https://example.com/a", Text: "Alpha"},
		{URL: "https://example.com/beta", Text: "Beta"},
	}
	// Empty filter passes everything through (same slice).
	if got := filterLinksBy(links, "  "); len(got) != 2 {
		t.Errorf("empty filter should pass all links, got %d", len(got))
	}
	if got := filterLinksBy(links, "alpha"); len(got) != 1 || got[0].URL != "https://example.com/a" {
		t.Errorf("text filter alpha = %+v", got)
	}
	if got := filterLinksBy(links, "BETA"); len(got) != 1 || got[0].URL != "https://example.com/beta" {
		t.Errorf("case-insensitive URL filter beta = %+v", got)
	}
	if got := filterLinksBy(links, "zzz"); len(got) != 0 {
		t.Errorf("no-match filter should return empty, got %d", len(got))
	}
}

func TestFilterImagesBy(t *testing.T) {
	imgs := []Image{
		{URL: "https://example.com/a.png", Alt: "alpha pic"},
		{URL: "https://example.com/b.png", Title: "beta pic"},
		{URL: "https://example.com/c.png"},
	}
	if got := filterImagesBy(imgs, ""); len(got) != 3 {
		t.Errorf("empty filter should pass all images, got %d", len(got))
	}
	if got := filterImagesBy(imgs, "alpha"); len(got) != 1 {
		t.Errorf("alt filter alpha = %d, want 1", len(got))
	}
	if got := filterImagesBy(imgs, "BETA"); len(got) != 1 {
		t.Errorf("title filter beta = %d, want 1", len(got))
	}
	if got := filterImagesBy(imgs, "example.com/b.png"); len(got) != 1 {
		t.Errorf("URL filter = %d, want 1", len(got))
	}
	if got := filterImagesBy(imgs, "nope"); len(got) != 0 {
		t.Errorf("no-match filter = %d, want 0", len(got))
	}
}

func TestShowLinksModalNoLinks(t *testing.T) {
	b := testBrowserWithUI()
	// No links: both entry points are no-ops (no panic, no modal).
	b.currentTab().links = nil
	b.showLinksModal()
	b.showLinksModalPage(0)
	if b.settingsActive {
		t.Error("links modal should not toggle settings state")
	}
}

func TestShowLinksModalPageBuilds(t *testing.T) {
	b := testBrowserWithUI()
	var links []Link
	for i := 0; i < 45; i++ {
		links = append(links, Link{URL: "https://example.com/link/" + strings.Repeat("x", 80), Text: "link"})
	}
	b.currentTab().links = links

	// Smoke: modal builds (SetRoot/SetFocus on a non-running app are safe).
	b.showLinksModalPage(0)
}

func TestShowLinksModalLargeList(t *testing.T) {
	b := testBrowserWithUI()
	var links []Link
	for i := 0; i < 55; i++ {
		links = append(links, Link{URL: "https://example.com/l", Text: "l"})
	}
	b.currentTab().links = links
	b.showLinksModalPage(0) // >50 links → showLoadingModal path
}

func TestShowImagesModalNoImages(t *testing.T) {
	b := testBrowserWithUI()
	b.currentTab().images = nil
	b.showImagesModal()
	b.showImagesModalPage(0)
}

func TestShowImagesModalPageBuilds(t *testing.T) {
	b := testBrowserWithUI()
	var imgs []Image
	for i := 0; i < 3; i++ {
		imgs = append(imgs, Image{URL: "https://example.com/i.png", Alt: "pic"})
	}
	b.currentTab().images = imgs
	b.showImagesModalPage(0)
}

func TestCloseModalToMain(t *testing.T) {
	b := testBrowserWithUI()
	b.closeModalToMain() // must not panic
}

func TestItemsPerPage(t *testing.T) {
	if got := (&Browser{}).itemsPerPage(); got != ItemsPerPage {
		t.Errorf("nil config itemsPerPage = %d, want %d", got, ItemsPerPage)
	}
	if got := (&Browser{config: &Config{ItemsPerPage: 7}}).itemsPerPage(); got != 7 {
		t.Errorf("configured itemsPerPage = %d, want 7", got)
	}
	if got := (&Browser{config: &Config{ItemsPerPage: 0}}).itemsPerPage(); got != ItemsPerPage {
		t.Errorf("invalid itemsPerPage = %d, want %d", got, ItemsPerPage)
	}
}
