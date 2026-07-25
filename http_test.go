package main

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestCookieMatches(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		cookie  Cookie
		rawURL  string
		want    bool
	}{
		{
			name:   "exact domain match",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: "example.com", Path: "/"},
			rawURL: "https://example.com/page",
			want:   true,
		},
		{
			name:   "subdomain match with dot prefix",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: ".example.com", Path: "/"},
			rawURL: "https://sub.example.com/page",
			want:   true,
		},
		{
			name:   "wrong domain",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: "other.com", Path: "/"},
			rawURL: "https://example.com/page",
			want:   false,
		},
		{
			name:   "path prefix match",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: "example.com", Path: "/app"},
			rawURL: "https://example.com/app/dashboard",
			want:   true,
		},
		{
			name:   "path does not match",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: "example.com", Path: "/admin"},
			rawURL: "https://example.com/app",
			want:   false,
		},
		{
			name:   "root path matches everything",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: "example.com", Path: "/"},
			rawURL: "https://example.com/anything/here",
			want:   true,
		},
		{
			name:   "secure cookie requires https",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: "example.com", Path: "/", Secure: true},
			rawURL: "http://example.com/page",
			want:   false,
		},
		{
			name:   "secure cookie works with https",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: "example.com", Path: "/", Secure: true},
			rawURL: "https://example.com/page",
			want:   true,
		},
		{
			name:   "expired cookie",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: "example.com", Path: "/", Expires: now.Add(-1 * time.Hour)},
			rawURL: "https://example.com/page",
			want:   false,
		},
		{
			name:   "not expired cookie",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: "example.com", Path: "/", Expires: now.Add(1 * time.Hour)},
			rawURL: "https://example.com/page",
			want:   true,
		},
		{
			name:   "zero expiry never expires",
			cookie: Cookie{Name: "sess", Value: "abc", Domain: "example.com", Path: "/"},
			rawURL: "https://example.com/page",
			want:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := url.Parse(tc.rawURL)
			if err != nil {
				t.Fatalf("bad test URL %q: %v", tc.rawURL, err)
			}
			got := tc.cookie.Matches(parsed, false) // enforceSameSite=false for backward compat
			if got != tc.want {
				t.Errorf("Matches(%q) = %v, want %v (enforceSameSite=false)", tc.rawURL, got, tc.want)
			}
		})
	}
}

func TestMatchesDomain(t *testing.T) {
	tests := []struct {
		host       string
		domain     string
		want       bool
	}{
		{"example.com", "example.com", true},
		{"sub.example.com", ".example.com", true},
		{"sub.example.com", "example.com", false},
		{"other.com", ".example.com", false},
		{"example.com.evil.com", "example.com", false},
		{"example.com", "", true},
		{"SUB.EXAMPLE.COM", ".example.com", true},
		{"example.com", ".example.com", true},
	}

	for _, tc := range tests {
		t.Run(tc.host+"_"+tc.domain, func(t *testing.T) {
			got := matchesDomain(tc.host, tc.domain)
			if got != tc.want {
				t.Errorf("matchesDomain(%q, %q) = %v, want %v", tc.host, tc.domain, got, tc.want)
			}
		})
	}
}

func TestMatchesPath(t *testing.T) {
	tests := []struct {
		reqPath    string
		cookiePath string
		want       bool
	}{
		{"/", "/", true},
		{"/app", "/", true},
		{"/app", "/app", true},
		{"/app/dashboard", "/app", true},
		{"/app", "/app/", false},
		{"/app/", "/app/", true},
		{"/admin", "/app", false},
		{"", "/", true},
		{"/something", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.reqPath+"_"+tc.cookiePath, func(t *testing.T) {
			got := matchesPath(tc.reqPath, tc.cookiePath)
			if got != tc.want {
				t.Errorf("matchesPath(%q, %q) = %v, want %v", tc.reqPath, tc.cookiePath, got, tc.want)
			}
		})
	}
}

func TestIsValidRedirectLocation(t *testing.T) {
	orig, _ := url.Parse("https://example.com/page")

	tests := []struct {
		location string
		want     bool
	}{
		// Same host
		{"https://example.com/other", true},
		// Subdomain
		{"https://sub.example.com/page", true},
		// Deep subdomain
		{"https://a.b.example.com/page", true},
		// Different host — blocked
		{"https://evil.com/page", false},
		// Equal hostname but different port — still OK
		{"https://example.com:8443/page", true},
		// CRITICAL: suffix match bypass
		{"https://example.com.evil.com/page", false},
		{"https://evil.example.com.comeviltest.com/page", false},
		// Relative redirects
		{"/other", true},
		{"#fragment", true},
		{"?query=1", true},
		// Non-relative non-absolute — blocked
		{"javascript:alert(1)", false},
	}

	for _, tc := range tests {
		t.Run(tc.location, func(t *testing.T) {
			got := isValidRedirectLocation(tc.location, orig)
			if got != tc.want {
				t.Errorf("isValidRedirectLocation(%q) = %v, want %v", tc.location, got, tc.want)
			}
		})
	}
}

func TestCleanupExpiredCookies(t *testing.T) {
	client := NewHTTPClient(nil)
	now := time.Now()

	// Add a fresh cookie
	fresh := &Cookie{Name: "fresh", Value: "a", Domain: "example.com", Path: "/", Expires: now.Add(1 * time.Hour)}
	client.cookies["fresh"] = fresh

	// Add an expired cookie
	expired := &Cookie{Name: "expired", Value: "b", Domain: "example.com", Path: "/", Expires: now.Add(-1 * time.Hour)}
	client.cookies["expired"] = expired

	// Add a cookie with no expiry (session cookie)
	session := &Cookie{Name: "session", Value: "c", Domain: "example.com", Path: "/"}
	client.cookies["session"] = session

	client.cleanupExpiredCookies()

	if _, ok := client.cookies["fresh"]; !ok {
		t.Error("fresh cookie was incorrectly removed")
	}
	if _, ok := client.cookies["expired"]; ok {
		t.Error("expired cookie was not removed")
	}
	if _, ok := client.cookies["session"]; !ok {
		t.Error("session cookie (no expiry) was incorrectly removed")
	}
}

func TestNewHTTPClientDefaults(t *testing.T) {
	client := NewHTTPClient(nil)

	if client == nil {
		t.Fatal("NewHTTPClient returned nil")
	}
	if client.maxRedirects != 10 {
		t.Errorf("default maxRedirects = %d, want 10", client.maxRedirects)
	}
	if client.cookies == nil {
		t.Error("cookies map should be initialized")
	}
}

func TestNewHTTPClientCustomTimeout(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.RequestTimeout = 5
	client := NewHTTPClient(&cfg)

	if client == nil {
		t.Fatal("NewHTTPClient returned nil")
	}
}

func TestNewHTTPClientCustomUserAgent(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.UserAgent = "custom-test/1.0"
	client := NewHTTPClient(&cfg)

	if client.forceUA != "custom-test/1.0" {
		t.Errorf("forceUA = %q, want %q", client.forceUA, "custom-test/1.0")
	}
}

func TestNewHTTPClientCustomMaxRedirects(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.MaxRedirects = 3
	client := NewHTTPClient(&cfg)

	if client.maxRedirects != 3 {
		t.Errorf("maxRedirects = %d, want 3", client.maxRedirects)
	}
}

// -------------------------------------------------------------------------
// isSameSite tests
// -------------------------------------------------------------------------

func TestIsSameSiteExactMatch(t *testing.T) {
	if !isSameSite("example.com", "example.com") {
		t.Error("isSameSite('example.com', 'example.com') should be true")
	}
}

func TestIsSameSiteDotPrefixMatch(t *testing.T) {
	tests := []struct {
		host, domain string
	}{
		{"sub.example.com", ".example.com"},
		{"deep.sub.example.com", ".example.com"},
		{"example.com", ".example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.host+"_"+tc.domain, func(t *testing.T) {
			if !isSameSite(tc.host, tc.domain) {
				t.Errorf("isSameSite(%q, %q) should be true", tc.host, tc.domain)
			}
		})
	}
}

func TestIsSameSiteDifferentDomain(t *testing.T) {
	if isSameSite("example.com", "other.com") {
		t.Error("isSameSite('example.com', 'other.com') should be false")
	}
	if isSameSite("other.com", ".example.com") {
		t.Error("isSameSite('other.com', '.example.com') should be false")
	}
}

func TestIsSameSiteCaseInsensitive(t *testing.T) {
	if !isSameSite("EXAMPLE.COM", "example.com") {
		t.Error("isSameSite should be case-insensitive")
	}
	if !isSameSite("Sub.Example.COM", ".example.com") {
		t.Error("isSameSite should be case-insensitive with dot prefix")
	}
}

func TestMatchesEnforcesSameSiteStrict(t *testing.T) {
	// SameSite=Strict cookie should NOT match a different domain.
	cookie := Cookie{Name: "sess", Value: "abc", Domain: "example.com", Path: "/", SameSite: "Strict"}
	parsed, _ := url.Parse("https://other.com/page")
	if cookie.Matches(parsed, true) {
		t.Error("SameSite=Strict cookie should not match different domain")
	}
	// Same-site request should still match.
	parsed2, _ := url.Parse("https://example.com/page")
	if !cookie.Matches(parsed2, true) {
		t.Error("SameSite=Strict cookie should match same domain")
	}
}

func TestMatchesEnforceSameSiteFalsePreservesExisting(t *testing.T) {
	// With enforceSameSite=false, SameSite=Strict should still match different
	// domain (backward compat — the SameSite field is ignored).
	// Use empty Domain so matchesDomain passes for any request host.
	cookie := Cookie{Name: "sess", Value: "abc", Domain: "", Path: "/", SameSite: "Strict"}
	parsed, _ := url.Parse("https://other.com/page")
	if !cookie.Matches(parsed, false) {
		t.Error("With enforceSameSite=false, Strict cookie should still match different domain")
	}
	// With enforceSameSite=true, same cookie should be blocked for other.com.
	if cookie.Matches(parsed, true) {
		t.Error("With enforceSameSite=true, Strict cookie should NOT match different domain")
	}
}

func TestSetProxy(t *testing.T) {
	client := NewHTTPClient(nil)
	proxyURL, _ := url.Parse("http://proxy.local:8080")
	client.SetProxy(proxyURL)

	// Verify the transport has the proxy set
	if transport, ok := client.client.Transport.(*http.Transport); ok {
		if transport.Proxy == nil {
			t.Error("proxy function should not be nil after SetProxy")
		}
	} else {
		t.Error("expected *http.Transport")
	}
}
