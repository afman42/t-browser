package main

import (
	"strings"
	"testing"
)

func TestIsTrackingParam(t *testing.T) {
	tracking := []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
		"fbclid", "gclid", "GCLID", "msclkid", "twclid", "yclid", "mc_eid"}
	for _, p := range tracking {
		if !isTrackingParam(p) {
			t.Errorf("isTrackingParam(%q) = false, want true", p)
		}
	}
	legit := []string{"q", "id", "page", "sort", "dir", "token", "auth", "size", "utm", ""}
	for _, p := range legit {
		if isTrackingParam(p) {
			t.Errorf("isTrackingParam(%q) = true, want false", p)
		}
	}
}

func TestIsTrackingParamUTMPrefixVariants(t *testing.T) {
	// Every utm_* prefix variant should be detected (case-insensitive).
	for _, p := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "UTM_SOURCE", "Utm_Medium"} {
		if !isTrackingParam(p) {
			t.Errorf("isTrackingParam(%q) = false, want true", p)
		}
	}
	// "utm" alone (no underscore) is NOT a tracking param.
	if isTrackingParam("utm") {
		t.Error("isTrackingParam(\"utm\") should be false (no underscore)")
	}
}

func TestStripTrackingParamsRemovesKnownParams(t *testing.T) {
	in := "https://example.com/page?utm_source=newsletter&utm_medium=email&id=42&fbclid=abc"
	out := stripTrackingParams(in)
	if strings.Contains(out, "utm_") {
		t.Errorf("utm_* params should be stripped, got %q", out)
	}
	if strings.Contains(out, "fbclid") {
		t.Errorf("fbclid should be stripped, got %q", out)
	}
	if !strings.Contains(out, "id=42") {
		t.Errorf("legit 'id' param should be preserved, got %q", out)
	}
}

func TestStripTrackingParamsPreservesFragment(t *testing.T) {
	in := "https://example.com/page?utm_source=x&q=1#section"
	out := stripTrackingParams(in)
	if !strings.Contains(out, "#section") {
		t.Errorf("fragment should be preserved, got %q", out)
	}
	if !strings.Contains(out, "q=1") {
		t.Errorf("legit query param should be preserved, got %q", out)
	}
}

func TestStripTrackingParamsNoTrackingReturnsUnchanged(t *testing.T) {
	in := "https://example.com/page?q=1&sort=asc"
	if out := stripTrackingParams(in); out != in {
		t.Errorf("URL without tracking params should be unchanged, got %q", out)
	}
}

func TestStripTrackingParamsInvalidURLReturnsUnchanged(t *testing.T) {
	in := "://not-a-url"
	if out := stripTrackingParams(in); out != in {
		t.Errorf("invalid URL should be returned unchanged, got %q", out)
	}
}

func TestStripTrackingParamsAllTrackingReturnsCleanURL(t *testing.T) {
	in := "https://example.com/?utm_source=a&fbclid=b"
	out := stripTrackingParams(in)
	// All params stripped → no query string.
	if strings.Contains(out, "?") {
		t.Errorf("URL with only tracking params should end up with no query, got %q", out)
	}
	if !strings.HasPrefix(out, "https://example.com/") {
		t.Errorf("base URL should be preserved, got %q", out)
	}
}

func TestStripTrackingParamsPreservesValuelessParams(t *testing.T) {
	in := "https://example.com/p?flag&utm_source=x&sort"
	out := stripTrackingParams(in)
	if !strings.Contains(out, "flag") {
		t.Errorf("valueless 'flag' param should be preserved, got %q", out)
	}
	if !strings.Contains(out, "sort") {
		t.Errorf("valueless 'sort' param should be preserved, got %q", out)
	}
	if strings.Contains(out, "utm_") {
		t.Errorf("utm param should be stripped, got %q", out)
	}
}

func TestStripTrackingParamsStripsAllUTMVariants(t *testing.T) {
	in := "https://example.com/p?utm_source=a&utm_medium=b&utm_campaign=c&utm_term=d&utm_content=e&q=1"
	out := stripTrackingParams(in)
	for _, p := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"} {
		if strings.Contains(out, p) {
			t.Errorf("%s should be stripped, got %q", p, out)
		}
	}
	if !strings.Contains(out, "q=1") {
		t.Errorf("legit 'q' param should survive, got %q", out)
	}
}

func TestIsBlockedDomain(t *testing.T) {
	blocked := []string{"example.com", "bad.test"}
	tests := []struct {
		host string
		want bool
	}{
		{"example.com", true},
		{"EXAMPLE.COM", true},
		{"sub.example.com", true},
		{"a.b.example.com", true},
		{"bad.test", true},
		{"good.com", false},
		{"example.com.evil.com", false}, // suffix attack must not match
		{"notexample.com", false},       // partial suffix must not match
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			if got := isBlockedDomain(tc.host, blocked); got != tc.want {
				t.Errorf("isBlockedDomain(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestIsBlockedDomainEmptyList(t *testing.T) {
	if isBlockedDomain("example.com", nil) {
		t.Error("nil blocklist should not block anything")
	}
	if isBlockedDomain("example.com", []string{}) {
		t.Error("empty blocklist should not block anything")
	}
}

func TestIsBlockedDomainTrimsWhitespace(t *testing.T) {
	// Entries with surrounding whitespace should still match.
	if !isBlockedDomain("example.com", []string{"  example.com  "}) {
		t.Error("blocklist entry with whitespace should match exact host")
	}
	if !isBlockedDomain("sub.example.com", []string{" example.com "}) {
		t.Error("blocklist entry with whitespace should match subdomain")
	}
}

func TestCleanURLForNavigation(t *testing.T) {
	// Enabled (default): strips tracking params.
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}
	in := "https://example.com/p?utm_source=x&q=1"
	out := b.cleanURLForNavigation(in)
	if strings.Contains(out, "utm_") {
		t.Errorf("cleanURLForNavigation should strip tracking params, got %q", out)
	}
	if !strings.Contains(out, "q=1") {
		t.Errorf("legit param should survive, got %q", out)
	}

	// Disabled: returns unchanged.
	cfg.StripTrackingParams = false
	if got := b.cleanURLForNavigation(in); got != in {
		t.Errorf("with strip disabled, URL should be unchanged, got %q", got)
	}
}

func TestCleanURLForNavigationNoConfig(t *testing.T) {
	// With no config the helper is a no-op (returns input unchanged).
	b := &Browser{}
	in := "https://example.com/p?utm_source=x"
	if got := b.cleanURLForNavigation(in); got != in {
		t.Errorf("cleanURLForNavigation with no config should be a no-op, got %q", got)
	}
}

func TestCheckBlockedDomain(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.BlockedDomains = []string{"evil.test"}
	b := &Browser{config: &cfg}

	if err := b.checkBlockedDomain("https://evil.test/page"); err == nil {
		t.Error("checkBlockedDomain should reject a blocked host")
	}
	if err := b.checkBlockedDomain("https://good.test/page"); err != nil {
		t.Errorf("checkBlockedDomain should allow non-blocked host, got %v", err)
	}
	if err := b.checkBlockedDomain("https://sub.evil.test/page"); err == nil {
		t.Error("checkBlockedDomain should reject a subdomain of a blocked host")
	}

	// No blocklist configured → always allow.
	cfg.BlockedDomains = nil
	if err := b.checkBlockedDomain("https://anything.test/"); err != nil {
		t.Errorf("with empty blocklist, no error expected, got %v", err)
	}
}

func TestCheckBlockedDomainNoConfig(t *testing.T) {
	// With no config the check is always a pass.
	b := &Browser{}
	if err := b.checkBlockedDomain("https://anything.test/"); err != nil {
		t.Errorf("checkBlockedDomain with no config should pass, got %v", err)
	}
}

func TestValidateAndSanitizeURL_RejectsBlockedDomain(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.BlockedDomains = []string{"evil.test"}
	b := &Browser{config: &cfg}

	_, err := b.validateAndSanitizeURL("https://evil.test/page")
	if err == nil {
		t.Fatal("expected error for blocked domain")
	}
	if !strings.Contains(err.Error(), "blocked domain") {
		t.Errorf("expected 'blocked domain' error, got %v", err)
	}
}

func TestValidateAndSanitizeURL_AllowsNonBlockedDomain(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.BlockedDomains = []string{"evil.test"}
	b := &Browser{config: &cfg}

	got, err := b.validateAndSanitizeURL("https://example.test/page")
	if err != nil {
		t.Errorf("non-blocked domain should pass validation, got %v", err)
	}
	if got != "https://example.test/page" {
		t.Errorf("validated URL = %q, want %q", got, "https://example.test/page")
	}
}

func TestValidateAndSanitizeURL_BlockedSubdomainRejected(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.BlockedDomains = []string{"evil.test"}
	b := &Browser{config: &cfg}

	_, err := b.validateAndSanitizeURL("https://sub.evil.test/page")
	if err == nil {
		t.Fatal("expected error for blocked subdomain")
	}
}
