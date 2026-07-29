package main

import (
	"fmt"
	"net/url"
	"strings"
)

// trackingParamPrefixes are query-parameter name prefixes that are purely used
// for analytics/tracking and are safe to strip without breaking page rendering.
var trackingParamPrefixes = []string{
	"utm_",
}

// trackingParamNames are exact query-parameter names that are tracking beacons.
var trackingParamNames = map[string]bool{
	"fbclid":      true,
	"gclid":       true,
	"dclid":       true,
	"msclkid":     true,
	"twclid":      true,
	"ysclid":      true,
	"mc_eid":      true,
	"mc_cid":      true,
	"_hsenc":      true,
	"_hsmi":       true,
	"igshid":      true,
	"pk_campaign": true,
	"pk_kwd":      true,
	"yclid":       true,
	"gbraid":      true,
	"wbraid":      true,
	"msid":        true,
	"ref_src":     true,
	"ref_url":     true,
	"ref":         true,
}

// isTrackingParam reports whether a single query-parameter name is a known
// tracking parameter (either an exact match or a utm_* prefix match).
func isTrackingParam(name string) bool {
	if trackingParamNames[strings.ToLower(name)] {
		return true
	}
	lower := strings.ToLower(name)
	for _, prefix := range trackingParamPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// stripTrackingParams returns rawURL with known analytics/tracking query
// parameters removed.  It preserves all other query parameters, the fragment,
// and the rest of the URL.  If the URL cannot be parsed it is returned
// unchanged so that callers (which will re-validate) still get a chance to
// report a proper error.
func stripTrackingParams(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	values := parsed.Query()
	changed := false
	for name := range values {
		if isTrackingParam(name) {
			values.Del(name)
			changed = true
		}
	}
	if !changed {
		return rawURL
	}

	parsed.RawQuery = values.Encode()
	// url.Values.Encode() omits empty values entirely (e.g. "?q=" becomes "?q").
	// That is acceptable for tracking-param stripping.
	return parsed.String()
}

// isBlockedDomain reports whether host matches any entry in blockedDomains.
// Matching is case-insensitive and treats a bare domain in the blocklist as
// covering all of its subdomains (e.g. "example.com" blocks "a.b.example.com").
func isBlockedDomain(host string, blockedDomains []string) bool {
	if host == "" || len(blockedDomains) == 0 {
		return false
	}
	host = strings.ToLower(strings.TrimSpace(host))
	for _, d := range blockedDomains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if host == d {
			return true
		}
		// subdomain match: block "x.example.com" when "example.com" is listed
		if strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// cleanURLForNavigation applies the configured URL privacy transforms
// (currently tracking-parameter stripping) and returns the cleaned URL.
// It is a no-op when strip_tracking_params is disabled.
func (b *Browser) cleanURLForNavigation(rawURL string) string {
	if b.config == nil || !b.config.StripTrackingParams {
		return rawURL
	}
	return stripTrackingParams(rawURL)
}

// checkBlockedDomain returns an error when the host of inputURL is on the
// configured blocklist.  It does nothing when no blocklist is configured.
func (b *Browser) checkBlockedDomain(inputURL string) error {
	if b.config == nil || len(b.config.BlockedDomains) == 0 {
		return nil
	}
	parsed, err := url.Parse(inputURL)
	if err != nil {
		return nil // let the caller's normal validation report parse errors
	}
	if isBlockedDomain(parsed.Hostname(), b.config.BlockedDomains) {
		return fmt.Errorf("access to blocked domain not allowed")
	}
	return nil
}
