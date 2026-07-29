package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// newTestServer is a small helper that starts an httptest server with the
// given handler and returns it.  Defer ts.Close() in the caller.
func newTestServer(handler func(http.ResponseWriter, *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(handler))
}

// --- Session integration tests ---

func TestSessionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "test_session.json")

	b := &Browser{
		forceUA: "test-agent/1.0",
	}
	b.currentTab().history = []string{"https://example.com", "https://example.com/page1"}
	b.currentTab().historyIndex = 1
	b.currentTab().currentURL = "https://example.com/page1"
	b.currentTab().searchTerm = "test"

	if err := b.SaveSession(sessionPath); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Verify the file exists and is non-empty
	info, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatalf("session file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("session file is empty")
	}

	// Load into a new Browser
	b2 := &Browser{}
	if err := b2.LoadSession(sessionPath); err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if len(b2.currentTab().history) != 2 {
		t.Errorf("history length = %d, want 2", len(b2.currentTab().history))
	}
	if b2.currentTab().historyIndex != 1 {
		t.Errorf("historyIndex = %d, want 1", b2.currentTab().historyIndex)
	}
	if b2.currentTab().currentURL != "https://example.com/page1" {
		t.Errorf("currentURL = %q, want %q", b2.currentTab().currentURL, "https://example.com/page1")
	}
	if b2.currentTab().searchTerm != "test" {
		t.Errorf("searchTerm = %q, want %q", b2.currentTab().searchTerm, "test")
	}
	if b2.forceUA != "test-agent/1.0" {
		t.Errorf("forceUA = %q, want %q", b2.forceUA, "test-agent/1.0")
	}
}

func TestSessionLoadNonexistent(t *testing.T) {
	b := &Browser{}
	err := b.LoadSession("/nonexistent/path/session.json")
	if err == nil {
		t.Error("expected error loading nonexistent session file")
	}
}

func TestSessionEmptyHistory(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "empty_session.json")

	b := &Browser{}
	b.currentTab().history = []string{}
	b.currentTab().historyIndex = -1

	if err := b.SaveSession(sessionPath); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	b2 := &Browser{}
	if err := b2.LoadSession(sessionPath); err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if len(b2.currentTab().history) != 0 {
		t.Errorf("history length = %d, want 0", len(b2.currentTab().history))
	}
	if b2.currentTab().historyIndex != -1 {
		t.Errorf("historyIndex = %d, want -1", b2.currentTab().historyIndex)
	}
}

// --- Navigation integration tests ---

func TestGoBackNoHistory(t *testing.T) {
	b := &Browser{}
	b.currentTab().history = []string{}
	b.currentTab().historyIndex = -1
	// Should not panic or crash
	b.GoBack()
	if b.currentTab().historyIndex != -1 {
		t.Errorf("historyIndex = %d, want -1", b.currentTab().historyIndex)
	}
}

func TestGoForwardNoHistory(t *testing.T) {
	b := &Browser{}
	b.currentTab().history = []string{}
	b.currentTab().historyIndex = -1
	b.GoForward()
	if b.currentTab().historyIndex != -1 {
		t.Errorf("historyIndex = %d, want -1", b.currentTab().historyIndex)
	}
}

func TestURLValidationEdgeCases(t *testing.T) {
	b := &Browser{}
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid http", "http://example.com", false},
		{"valid https with port", "https://example.com:8080/path", false},
		{"valid with query", "https://example.com/path?q=1&r=2", false},
		{"empty", "", true},
		{"javascript scheme", "javascript:alert(1)", true},
		{"data scheme", "data:text/html,<script>alert(1)</script>", true},
		{"file scheme", "file:///etc/passwd", true},
		{"localhost blocked", "http://localhost:3000", true},
		{"127 blocked", "http://127.0.0.1:8080", true},
		{"10.x blocked", "http://10.0.0.1/admin", true},
		{"192.168 blocked", "http://192.168.1.1", true},
		{"too long", string(make([]byte, 2049)), true},
		{"suspicious dots", "https://example.com/..%2F..%2Fetc", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := b.validateAndSanitizeURL(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateAndSanitizeURL(%q) error = %v, wantErr = %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

// --- Word wrap integration tests ---

func TestWordWrapIntegration(t *testing.T) {
	b := &Browser{}

	if b.shouldDisableWordWrap("") {
		t.Error("shouldDisableWordWrap('') should be false")
	}

	short := "short line\nanother line\nthird line"
	if b.shouldDisableWordWrap(short) {
		t.Error("shouldDisableWordWrap should be false for short lines")
	}

	long := string(make([]byte, 600))
	if !b.shouldDisableWordWrap(long) {
		t.Error("shouldDisableWordWrap should be true for a line >500 chars")
	}
}

// --- Session file path integration ---

func TestSessionFilePathGeneration(t *testing.T) {
	dir := t.TempDir()
	path := GetSessionFilePath(dir)

	if path == "" {
		t.Fatal("GetSessionFilePath returned empty")
	}

	// Directory should have been created
	sessionsDir := filepath.Join(dir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Error("sessions directory was not created")
	}

	// Filename should match pattern
	base := filepath.Base(path)
	if len(base) < 20 {
		t.Errorf("filename %q is too short", base)
	}
}

func TestLatestSessionFileSelection(t *testing.T) {
	dir := t.TempDir()
	sessionsDir := filepath.Join(dir, "sessions")
	os.MkdirAll(sessionsDir, 0755)

	// No files yet
	if got := GetLatestSessionFile(dir); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	// Create files in reverse chronological order
	os.WriteFile(filepath.Join(sessionsDir, "session_2024-01-01_00-00-00.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(sessionsDir, "session_2024-06-15_12-30-00.json"), []byte("{}"), 0644)

	got := GetLatestSessionFile(dir)
	wantSuffix := "session_2024-06-15_12-30-00.json"
	if filepath.Base(got) != wantSuffix {
		t.Errorf("got %q, want file %q", got, wantSuffix)
	}
}

// --- Cookie file path integration ---

func TestCookieFilePathGeneration(t *testing.T) {
	dir := t.TempDir()
	path := GetCookieFilePath(dir)

	if path == "" {
		t.Fatal("GetCookieFilePath returned empty")
	}

	cookiesDir := filepath.Join(dir, "cookies")
	if _, err := os.Stat(cookiesDir); os.IsNotExist(err) {
		t.Error("cookies directory was not created")
	}

	base := filepath.Base(path)
	if len(base) < 20 {
		t.Errorf("filename %q is too short", base)
	}
}

func TestLatestCookieFileSelection(t *testing.T) {
	dir := t.TempDir()
	cookiesDir := filepath.Join(dir, "cookies")
	os.MkdirAll(cookiesDir, 0755)

	// No files yet
	if got := GetLatestCookieFile(dir); got != "" {
		t.Errorf("expected empty, got %q", got)
	}

	os.WriteFile(filepath.Join(cookiesDir, "cookies_2024-03-10_08-15-00.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(cookiesDir, "cookies_2024-03-10_09-00-00.json"), []byte("{}"), 0644)

	got := GetLatestCookieFile(dir)
	wantSuffix := "cookies_2024-03-10_09-00-00.json"
	if filepath.Base(got) != wantSuffix {
		t.Errorf("got %q, want file %q", got, wantSuffix)
	}
}

// --- Highlight/formatting integration ---

func TestRemoveTviewFormattingIntegration(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"hello world", "hello world"},
		{"[red]hello[-]", "hello"},
		{"[::b]bold[::-]", "bold"},
		{"[red:blue:b]styled[::-]", "styled"},
		{"[red]a[-] [::b]b[::-]", "a b"},
	}
	for _, tc := range tests {
		got := removeTviewFormatting(tc.input)
		if got != tc.want {
			t.Errorf("removeTviewFormatting(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCurrentPosAtLineStartIntegration(t *testing.T) {
	text := "abc\ndefg\nhij"
	tests := []struct {
		line int
		want int
	}{
		{0, 0},
		{1, 4}, // len("abc") + 1
		{2, 9}, // len("abc\ndefg") + 1
	}
	for _, tc := range tests {
		got := currentPosAtLineStart(text, tc.line)
		if got != tc.want {
			t.Errorf("currentPosAtLineStart(line %d) = %d, want %d", tc.line, got, tc.want)
		}
	}
}

// --- Session save error path ---

func TestSessionSaveBadPath(t *testing.T) {
	b := &Browser{}
	b.currentTab().history = []string{"https://example.com"}
	b.currentTab().historyIndex = 0
	b.currentTab().currentURL = "https://example.com"
	// Saving to a path in a nonexistent directory should fail
	err := b.SaveSession("/nonexistent_parent_dir/session.json")
	if err == nil {
		t.Error("expected error saving session to bad path")
	}
}

func TestCleanExcessiveWhitespaceEdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"\n\n\n\n", ""},             // all empty lines become empty
		{"a\n\n\n\n\nb", "a\n\nb"},   // many middle newlines
		{"a\nb\n\n", "a\nb\n"},       // trailing newlines
		{"singleline", "singleline"}, // no newlines
		{"\nfirst\n\nsecond\n\n\nthird\n", "\nfirst\n\nsecond\n\nthird\n"},
	}
	for _, tc := range tests {
		got := cleanExcessiveWhitespace(tc.input)
		if got != tc.want {
			t.Errorf("cleanExcessiveWhitespace(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// -------------------------------------------------------------------------
// Content security integration tests
// -------------------------------------------------------------------------

func TestContentSecurityIntegrationSanitizeViaHTTP(t *testing.T) {
	// Start a test HTTP server that returns HTML with scripts.
	ts := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte(`<html><head><script>alert("xss")</script></head><body><p>safe content</p><img src="https://evil.com/pic.png" onload="evil()"></body></html>`))
	})
	defer ts.Close()

	cfg := GetDefaultConfig()
	cfg.EnableContentSecurity = true
	cfg.BlockExternalResources = true

	client := NewHTTPClient(&cfg)
	html, err := client.FetchPage(ts.URL)
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}

	// Apply content security sanitization.
	sanitized := sanitizeHTML(html)

	// Scripts should be stripped.
	if strings.Contains(sanitized, "<script>") {
		t.Error("sanitized HTML should not contain script tags")
	}

	// Safe content should be preserved.
	if !strings.Contains(sanitized, "safe content") {
		t.Error("sanitized HTML should preserve safe content")
	}

	// Event handlers should be stripped.
	if strings.Contains(sanitized, "onload=") {
		t.Error("sanitized HTML should not contain event handlers")
	}
}

func TestContentSecurityIntegrationDisabled(t *testing.T) {
	// When content security is disabled, scripts should pass through.
	ts := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte(`<html><script>alert(1)</script><p>text</p></html>`))
	})
	defer ts.Close()

	cfg := GetDefaultConfig()
	cfg.EnableContentSecurity = false
	cfg.BlockExternalResources = false

	client := NewHTTPClient(&cfg)
	html, err := client.FetchPage(ts.URL)
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}

	// Scripts should NOT be stripped when content security is disabled.
	if !strings.Contains(html, "<script>") {
		t.Error("scripts should pass through when content security is disabled")
	}
}

func TestContentSecurityBlockExternalResourcesHTTP(t *testing.T) {
	// Start a test server serving HTML with external images.
	ts := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte(`<html><body><img src="https://other.com/pic.png"><img src="/local.png"></body></html>`))
	})
	defer ts.Close()

	// Parse the HTML and apply blockExternalResources.
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(
		`<html><body><img src="https://other.com/pic.png"><img src="/local.png"></body></html>`))
	if err != nil {
		t.Fatal(err)
	}

	pageURL, _ := url.Parse(ts.URL)
	blockExternalResources(doc, pageURL)

	// The external image should have its src cleared and data-blocked-external set.
	var hasExternalBlocked, hasLocal bool
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		blocked, _ := s.Attr("data-blocked-external")
		switch {
		case blocked == "https://other.com/pic.png":
			hasExternalBlocked = true
			if src != "" {
				t.Errorf("external image src should be cleared, got %q", src)
			}
		case src == "/local.png":
			hasLocal = true
		default:
			t.Errorf("unexpected img: src=%q blocked=%q", src, blocked)
		}
	})

	if !hasExternalBlocked {
		t.Error("external image should be marked as blocked")
	}
	if !hasLocal {
		t.Error("local image should still be present")
	}
}

// -------------------------------------------------------------------------
// SameSite integration tests
// -------------------------------------------------------------------------

func TestSameSiteIntegrationStrictBlocksCrossDomain(t *testing.T) {
	// Cookie with SameSite=Strict should be blocked on cross-domain requests
	// when enforceSameSite=true.
	cfg := GetDefaultConfig()
	cfg.EnforceSameSite = true

	client := NewHTTPClient(&cfg)

	// Manually add a SameSite=Strict cookie.
	client.cookies["example.com_session"] = &Cookie{
		Name:     "session",
		Value:    "abc123",
		Domain:   "",
		Path:     "/",
		SameSite: "Strict",
	}

	// Simulate a cross-domain request.
	targetURL, _ := url.Parse("https://other.com/page")

	// The cookie should NOT match with enforceSameSite=true.
	if client.cookies["example.com_session"].Matches(targetURL, true) {
		t.Error("SameSite=Strict cookie should NOT match cross-domain with enforceSameSite=true")
	}

	// The cookie should match with enforceSameSite=false (backward compat).
	if !client.cookies["example.com_session"].Matches(targetURL, false) {
		t.Error("SameSite=Strict cookie should match cross-domain with enforceSameSite=false")
	}
}

func TestSameSiteIntegrationNoneAlwaysSends(t *testing.T) {
	// SameSite=None cookies should always send regardless of enforceSameSite.
	cookie := &Cookie{
		Name:     "tracker",
		Value:    "id123",
		Domain:   "",
		Path:     "/",
		SameSite: "None",
	}

	targetURL, _ := url.Parse("https://other.com/page")

	if !cookie.Matches(targetURL, true) {
		t.Error("SameSite=None cookie should match cross-domain with enforceSameSite=true")
	}
	if !cookie.Matches(targetURL, false) {
		t.Error("SameSite=None cookie should match cross-domain with enforceSameSite=false")
	}
}

// --- hasRealImageExtension edge cases ---

func TestHasRealImageExtensionMore(t *testing.T) {
	b := &Browser{}
	tests := []struct {
		url  string
		want bool
	}{
		{"https://example.com/image.jpg?w=800&h=600", true},
		{"https://example.com/image.png#fragment", true},
		{"https://example.com/image.JPG", true},
		{"https://example.com/image.PNG", true},
		{"https://example.com/image", false},
		{"https://example.com/image.html", false},
		{"https://example.com/image.php?img=1", false},
	}
	for _, tc := range tests {
		got := b.hasRealImageExtension(tc.url)
		if got != tc.want {
			t.Errorf("hasRealImageExtension(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}
