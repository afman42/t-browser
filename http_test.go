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
		name   string
		cookie Cookie
		rawURL string
		want   bool
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
		host   string
		domain string
		want   bool
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
		// Protocol-relative — cross-host, blocked (SSRF)
		{"//evil.com/page", false},
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

func TestAddCookieRejectsForeignDomain(t *testing.T) {
	client := NewHTTPClient(nil)

	// Server at example.com tries to set a cookie scoped to a foreign domain:
	// it must fall back to a host-only cookie (RFC 6265 §5.3).
	client.addCookie(&http.Cookie{Name: "sid", Value: "1", Domain: ".bank.com", Path: "/"}, "example.com")
	if cookie, ok := client.cookies["example.com_sid"]; !ok {
		t.Fatal("expected host-only cookie stored under example.com_sid")
	} else if cookie.Domain != "example.com" {
		t.Errorf("cookie Domain = %q, want host-only fallback %q", cookie.Domain, "example.com")
	}

	// A legitimate subdomain-scoped cookie is preserved as-is.
	client.addCookie(&http.Cookie{Name: "pref", Value: "x", Domain: ".example.com", Path: "/"}, "sub.example.com")
	if cookie, ok := client.cookies[".example.com_pref"]; !ok {
		t.Fatal("expected subdomain cookie stored")
	} else if cookie.Domain != ".example.com" {
		t.Errorf("cookie Domain = %q, want %q", cookie.Domain, ".example.com")
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

func TestCheckRequestHost(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.BlockedDomains = []string{"evil.example"}
	client := NewHTTPClient(&cfg)

	tests := []struct {
		url  string
		want bool // true = rejected
	}{
		{"http://127.0.0.1/", true},
		{"http://localhost/", true},
		{"http://2130706433/", true}, // 127.0.0.1 legacy form
		{"http://[::1]/", true},
		{"http://10.0.0.5/", true},
		{"http://evil.example/", true},
		{"ftp://example.com/", true},
		{"http://8.8.8.8/", false},
		{"https://no-such-host.invalid/", false}, // lookup failure falls through
	}
	for _, tc := range tests {
		u, err := url.Parse(tc.url)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.url, err)
		}
		err = client.checkRequestHost(u)
		if (err != nil) != tc.want {
			t.Errorf("checkRequestHost(%q) err=%v, want rejected=%v", tc.url, err, tc.want)
		}
	}
}

func TestSetUserAgentReachesRequests(t *testing.T) {
	mt := &mockTransport{pathMatches: map[string]mockResponse{"/": htmlOK}}
	client := NewHTTPClient(nil)
	client.client.Transport = mt
	client.SetUserAgent("live-agent/1.0")

	if _, err := client.FetchPage("http://mock.test/"); err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if mt.captureUA != "live-agent/1.0" {
		t.Errorf("captured UA = %q, want live-agent/1.0", mt.captureUA)
	}
}

func TestTransportSnapshot(t *testing.T) {
	client := NewHTTPClient(nil)
	snap := client.transportSnapshot()
	if snap == nil {
		t.Fatal("transportSnapshot returned nil")
	}
	if snap != client.transport {
		t.Error("transportSnapshot should expose the current base transport")
	}
}

func TestCancelRequestNoPanic(t *testing.T) {
	client := NewHTTPClient(nil)
	// No request in flight: cancelFunc is nil, must be a no-op.
	client.CancelRequest()
	client.CancelRequest()
}

func TestNewHTTPClientCustomConfig(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.RequestTimeout = 5
	cfg.MaxRedirects = 7
	cfg.MaxRetries = 9
	cfg.UserAgent = "custom-agent/2.0"
	client := NewHTTPClient(&cfg)

	if client.client.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", client.client.Timeout)
	}
	if client.maxRedirects != 7 {
		t.Errorf("maxRedirects = %d, want 7", client.maxRedirects)
	}
	if client.maxRetries != 9 {
		t.Errorf("maxRetries = %d, want 9", client.maxRetries)
	}
	if client.forceUA != "custom-agent/2.0" {
		t.Errorf("forceUA = %q, want custom-agent/2.0", client.forceUA)
	}
}

func TestNewHTTPClientInvalidPinning(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.EnablePinning = true
	cfg.PinnedKeys = []string{"not-base64!!"}
	// Malformed pins must not panic and must not enable pinning.  (http2
	// setup always installs a TLS config, so check pinning flags, not nil.)
	client := NewHTTPClient(&cfg)
	if client.transport == nil || client.transport.TLSClientConfig == nil ||
		client.transport.TLSClientConfig.InsecureSkipVerify ||
		client.transport.TLSClientConfig.VerifyConnection != nil {
		t.Error("invalid pinned keys must leave pinning disabled")
	}
}

func TestConvertEncoding(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		body     []byte
		want     []byte
		isNil    bool
	}{
		{"utf8 declared", "utf-8", []byte("x"), nil, true},
		{"empty encoding", "", []byte("x"), nil, true},
		{"unknown encoding", "x-charset-foo", []byte("x"), nil, true},
		{"latin1", "iso-8859-1", []byte("caf\xe9"), []byte("café"), false},
		{"latin generic", "latin1", []byte("\xe9"), []byte("é"), false},
		{"iso-8859-2", "iso-8859-2", []byte{0xb1}, []byte("ą"), false},
		{"iso-8859-15", "iso-8859-15", []byte{0xa4}, []byte("€"), false},
		{"utf16le bom", "utf-16", []byte("\xff\xfec\x00a\x00f\x00\xe9\x00"), []byte("café"), false},
	}
	for _, tc := range tests {
		got, err := convertEncoding(tc.body, tc.encoding, 1<<20)
		if err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
			continue
		}
		if tc.isNil {
			if got != nil {
				t.Errorf("%s: want nil, got %q", tc.name, got)
			}
			continue
		}
		if string(got) != string(tc.want) {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestValidCookieDomain(t *testing.T) {
	tests := []struct {
		host, domain string
		want         bool
	}{
		{"example.com", "example.com", true},
		{"sub.example.com", ".example.com", true},
		{"example.com", ".example.com", true},
		{"example.com", "evil.com", false},
		{"example.com", "notexample.com", false},
		{"example.com:8080", "example.com", true},
		{"[::1]", "::1", true},
		{"EXAMPLE.com", "example.com", true},
		{"example.com", "", true},
		{"example.com", ".", true},
	}
	for _, tc := range tests {
		if got := validCookieDomain(tc.host, tc.domain); got != tc.want {
			t.Errorf("validCookieDomain(%q, %q) = %v, want %v", tc.host, tc.domain, got, tc.want)
		}
	}
}

func TestAddCookieLockedBranches(t *testing.T) {
	client := NewHTTPClient(nil)
	now := time.Now()

	// SameSite=Strict, explicit path, secure+httponly, expiry via Expires.
	client.addCookie(&http.Cookie{
		Name: "a", Value: "1", Domain: "example.com", Path: "/app",
		SameSite: http.SameSiteStrictMode, Secure: true, HttpOnly: true,
		Expires: now.Add(time.Hour),
	}, "example.com")
	c := client.cookies["example.com_a"]
	if c == nil {
		t.Fatal("expected cookie a stored")
	}
	if c.SameSite != "Strict" || c.Path != "/app" || !c.Secure || !c.HttpOnly {
		t.Errorf("cookie a fields wrong: %+v", c)
	}

	// Default path when empty; MaxAge sets Expires.
	client.addCookie(&http.Cookie{Name: "b", Value: "2", MaxAge: 60}, "example.com")
	c = client.cookies["example.com_b"]
	if c == nil {
		t.Fatal("expected cookie b stored")
	}
	if c.Path != "/" {
		t.Errorf("cookie b path = %q, want /", c.Path)
	}
	if c.MaxAge != 60 {
		t.Errorf("cookie b MaxAge = %d, want 60", c.MaxAge)
	}
	if c.Expires.Before(now.Add(50*time.Second)) || c.Expires.After(now.Add(90*time.Second)) {
		t.Errorf("cookie b Expires = %v, want ~now+60s", c.Expires)
	}

	// Negative MaxAge deletes the cookie.  The deletion key uses the
	// server-sent Domain, so it must match how the cookie was stored.
	client.cookies["example.com_c"] = &Cookie{Name: "c", Value: "x", Domain: "example.com"}
	client.addCookie(&http.Cookie{Name: "c", Value: "", Domain: "example.com", MaxAge: -1}, "example.com")
	if _, ok := client.cookies["example.com_c"]; ok {
		t.Error("cookie c should be deleted by negative MaxAge")
	}
}
