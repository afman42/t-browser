package main

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// =============================================================================
// Config defaults for the new enhancement fields
// =============================================================================

func TestGetDefaultConfigNewEnhancementFields(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("default MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.CacheTTLSeconds != 0 {
		t.Errorf("default CacheTTLSeconds = %d, want 0 (always revalidate)", cfg.CacheTTLSeconds)
	}
	if cfg.ItemsPerPage != 20 {
		t.Errorf("default ItemsPerPage = %d, want 20", cfg.ItemsPerPage)
	}
	if !cfg.StripTrackingParams {
		t.Error("default StripTrackingParams should be true")
	}
	if cfg.BlockedDomains != nil {
		t.Errorf("default BlockedDomains should be nil, got %v", cfg.BlockedDomains)
	}
}

// =============================================================================
// Pagination: items-per-page helper + link/image filter logic
// =============================================================================

func TestItemsPerPageHelper(t *testing.T) {
	// No config → fallback to the ItemsPerPage constant.
	b := &Browser{}
	if got := b.itemsPerPage(); got != ItemsPerPage {
		t.Errorf("itemsPerPage() with no config = %d, want %d", got, ItemsPerPage)
	}

	// Configured value wins.
	cfg := GetDefaultConfig()
	cfg.ItemsPerPage = 7
	b2 := &Browser{config: &cfg}
	if got := b2.itemsPerPage(); got != 7 {
		t.Errorf("itemsPerPage() with config = %d, want 7", got)
	}

	// Non-positive config value falls back to the constant.
	cfg.ItemsPerPage = 0
	b3 := &Browser{config: &cfg}
	if got := b3.itemsPerPage(); got != ItemsPerPage {
		t.Errorf("itemsPerPage() with 0 config = %d, want %d", got, ItemsPerPage)
	}

	// Negative config value also falls back.
	cfg.ItemsPerPage = -5
	b4 := &Browser{config: &cfg}
	if got := b4.itemsPerPage(); got != ItemsPerPage {
		t.Errorf("itemsPerPage() with -5 config = %d, want %d", got, ItemsPerPage)
	}
}

func TestFilterLinksByEmptyFilterReturnsAll(t *testing.T) {
	links := []Link{
		{URL: "https://a.com/", Text: "Alpha"},
		{URL: "https://b.com/", Text: "Beta"},
	}
	if got := filterLinksBy(links, ""); len(got) != 2 {
		t.Errorf("empty filter should return all, got %d", len(got))
	}
	if got := filterLinksBy(links, "   "); len(got) != 2 {
		t.Errorf("whitespace filter should return all, got %d", len(got))
	}
}

func TestFilterLinksByMatchesTextCaseInsensitive(t *testing.T) {
	links := []Link{
		{URL: "https://a.com/", Text: "Alpha"},
		{URL: "https://b.com/", Text: "Beta"},
		{URL: "https://c.com/", Text: "alphabet"},
	}
	got := filterLinksBy(links, "ALPH")
	if len(got) != 2 {
		t.Errorf("case-insensitive text match should return 2, got %d (%v)", len(got), got)
	}
}

func TestFilterLinksByMatchesURL(t *testing.T) {
	links := []Link{
		{URL: "https://example.com/page", Text: "Page"},
		{URL: "https://other.com/", Text: "Other"},
	}
	got := filterLinksBy(links, "example.com")
	if len(got) != 1 || got[0].Text != "Page" {
		t.Errorf("URL match should return the example.com link, got %v", got)
	}
}

func TestFilterLinksByNoMatch(t *testing.T) {
	links := []Link{{URL: "https://a.com/", Text: "Alpha"}}
	if got := filterLinksBy(links, "zzz"); len(got) != 0 {
		t.Errorf("no match should return empty slice, got %v", got)
	}
}

func TestFilterLinksByEmptySlice(t *testing.T) {
	if got := filterLinksBy(nil, "x"); len(got) != 0 {
		t.Errorf("filter on nil slice should return empty, got %v", got)
	}
}

func TestFilterImagesByEmptyFilterReturnsAll(t *testing.T) {
	images := []Image{
		{URL: "https://a.com/1.png", Alt: "First"},
		{URL: "https://b.com/2.jpg", Alt: "Second"},
	}
	if got := filterImagesBy(images, ""); len(got) != 2 {
		t.Errorf("empty filter should return all, got %d", len(got))
	}
}

func TestFilterImagesByMatchesAltAndURL(t *testing.T) {
	images := []Image{
		{URL: "https://a.com/cat.png", Alt: "A cat", Title: ""},
		{URL: "https://b.com/dog.jpg", Alt: "Dog", Title: "Good boy"},
		{URL: "https://c.com/x.gif", Alt: "", Title: "Banner"},
	}
	if got := filterImagesBy(images, "cat"); len(got) != 1 {
		t.Errorf("alt match 'cat' should return 1, got %d", len(got))
	}
	if got := filterImagesBy(images, "dog.jpg"); len(got) != 1 {
		t.Errorf("url match 'dog.jpg' should return 1, got %d", len(got))
	}
	if got := filterImagesBy(images, "banner"); len(got) != 1 {
		t.Errorf("title match 'banner' should return 1, got %d", len(got))
	}
}

func TestFilterImagesByNoMatch(t *testing.T) {
	images := []Image{{URL: "https://a.com/1.png", Alt: "cat"}}
	if got := filterImagesBy(images, "zzz"); len(got) != 0 {
		t.Errorf("no match should return empty, got %v", got)
	}
}

func TestFilterImagesByEmptySlice(t *testing.T) {
	if got := filterImagesBy(nil, "x"); len(got) != 0 {
		t.Errorf("filter on nil slice should return empty, got %v", got)
	}
}

// =============================================================================
// Rendering enhancements: heading colours, blockquote marker, horizontal rule
// =============================================================================

// findFirstElement returns the first descendant element with the given tag.
func findFirstElement(root *html.Node, tag string) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && root.Data == tag {
		return root
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if n := findFirstElement(c, tag); n != nil {
			return n
		}
	}
	return nil
}

func renderFirstTag(t *testing.T, htmlContent, tag string) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		t.Fatalf("html.Parse failed: %v", err)
	}
	node := findFirstElement(doc, tag)
	if node == nil {
		t.Fatalf("no <%s> element found in %q", tag, htmlContent)
	}
	b := &Browser{}
	var result strings.Builder
	tabs := 0
	b.renderNode(node, &result, &tabs)
	return result.String()
}

func TestRenderNodeHeadingColors(t *testing.T) {
	tests := []struct {
		tag      string
		html     string
		wantTag  string
		wantText string
	}{
		{"h1", "<h1>Title One</h1>", "[cyan::b]# ", "Title One"},
		{"h2", "<h2>Title Two</h2>", "[yellow::b]## ", "Title Two"},
		{"h3", "<h3>Title Three</h3>", "[green::b]### ", "Title Three"},
		{"h4", "<h4>Title Four</h4>", "[blue::b]#### ", "Title Four"},
		{"h5", "<h5>Title Five</h5>", "[magenta::b]##### ", "Title Five"},
		{"h6", "<h6>Title Six</h6>", "[red::b]###### ", "Title Six"},
	}
	for _, tc := range tests {
		t.Run(tc.tag, func(t *testing.T) {
			out := renderFirstTag(t, tc.html, tc.tag)
			if !strings.Contains(out, tc.wantTag) {
				t.Errorf("heading %s should contain color tag %q, got %q", tc.tag, tc.wantTag, out)
			}
			if !strings.Contains(out, tc.wantText) {
				t.Errorf("heading %s should contain text %q, got %q", tc.tag, tc.wantText, out)
			}
			if !strings.Contains(out, "[-]") {
				t.Errorf("heading %s should close formatting with [-], got %q", tc.tag, out)
			}
		})
	}
}

func TestRenderNodeBlockquoteColoredMarker(t *testing.T) {
	out := renderFirstTag(t, "<blockquote>Quoted text</blockquote>", "blockquote")
	if !strings.Contains(out, "[cyan]> [-]") {
		t.Errorf("blockquote should render coloured '> ' marker, got %q", out)
	}
	if !strings.Contains(out, "Quoted text") {
		t.Errorf("blockquote should preserve inner text, got %q", out)
	}
}

func TestRenderNodeHorizontalRuleFullWidth(t *testing.T) {
	out := renderFirstTag(t, "<html><body><hr></body></html>", "hr")
	// Should be a line of box-drawing chars, not the old '---' marker.
	if !strings.Contains(out, "─") {
		t.Errorf("hr should render box-drawing rule, got %q", out)
	}
	if strings.Contains(out, "---") {
		t.Errorf("hr should not render the old '---' marker, got %q", out)
	}
	if got := strings.Count(out, "─"); got < 10 {
		t.Errorf("hr rule should be a long line, got %d '─' runes", got)
	}
}

// =============================================================================
// Settings: new enhancement settings are wired into the settings model
// =============================================================================

func TestUpdateSettingValueEnhancementFields(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}

	b.updateSettingValue("max_retries", 5)
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}

	b.updateSettingValue("cache_ttl_seconds", 120)
	if cfg.CacheTTLSeconds != 120 {
		t.Errorf("CacheTTLSeconds = %d, want 120", cfg.CacheTTLSeconds)
	}

	b.updateSettingValue("items_per_page", 15)
	if cfg.ItemsPerPage != 15 {
		t.Errorf("ItemsPerPage = %d, want 15", cfg.ItemsPerPage)
	}

	b.updateSettingValue("strip_tracking_params", false)
	if cfg.StripTrackingParams != false {
		t.Errorf("StripTrackingParams = %v, want false", cfg.StripTrackingParams)
	}

	// Type mismatch should leave the value unchanged.
	b.updateSettingValue("max_retries", "not an int")
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries should be unchanged after type mismatch, got %d", cfg.MaxRetries)
	}
}

func TestGetSettingsForCategoryNetworkIncludesRetriesAndTTL(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}

	settings := b.getSettingsForCategory("network")
	ids := make(map[string]bool, len(settings))
	for _, s := range settings {
		ids[s.ID] = true
	}
	if !ids["max_retries"] {
		t.Error("network settings should include 'max_retries'")
	}
	if !ids["cache_ttl_seconds"] {
		t.Error("network settings should include 'cache_ttl_seconds'")
	}
}

func TestGetSettingsForCategoryUIIncludesItemsPerPage(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}

	settings := b.getSettingsForCategory("ui")
	ids := make(map[string]bool, len(settings))
	for _, s := range settings {
		ids[s.ID] = true
	}
	if !ids["items_per_page"] {
		t.Error("ui settings should include 'items_per_page'")
	}
}

func TestGetSettingsForCategoryPrivacyIncludesStripTracking(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}

	settings := b.getSettingsForCategory("privacy")
	ids := make(map[string]bool, len(settings))
	for _, s := range settings {
		ids[s.ID] = true
	}
	if !ids["strip_tracking_params"] {
		t.Error("privacy settings should include 'strip_tracking_params'")
	}
}
