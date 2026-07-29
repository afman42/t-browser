package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// cacheTTL returns the client-side cache freshness lifetime. A value of 0
// means cached entries are always revalidated (the default, preserving the
// original conditional-request behaviour).
func (c *HTTPClient) cacheTTL() time.Duration {
	if c.config != nil && c.config.CacheTTLSeconds > 0 {
		return time.Duration(c.config.CacheTTLSeconds) * time.Second
	}
	return 0
}

// isTransientStatus reports whether a status code is a retryable transient
// failure: 429 Too Many Requests and the 502/503/504 gateway errors.
func isTransientStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// retryBackoff computes the sleep duration before the next retry attempt.  It
// honours a Retry-After response header (seconds or HTTP-date) when present,
// otherwise falls back to an exponential schedule (base * 2^retryCount).  The
// result is capped at 30 seconds.
func (c *HTTPClient) retryBackoff(resp *http.Response, retryCount int) time.Duration {
	const cap = 30 * time.Second

	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if parsed, ok := parseRetryAfter(ra); ok && parsed > 0 {
			if parsed > cap {
				return cap
			}
			return parsed
		}
	}

	base := c.retryBaseDelay
	if base <= 0 {
		base = 1 * time.Second
	}
	backoff := base << uint(retryCount) // base * 2^retryCount
	if backoff > cap || backoff < 0 {
		return cap
	}
	if backoff < base {
		return base
	}
	return backoff
}

// parseRetryAfter parses a Retry-After header value which may be either an
// integer number of seconds or an HTTP-date (RFC 7231).
func parseRetryAfter(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	// Integer seconds.
	if secs, err := strconv.Atoi(value); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	// HTTP-date.
	for _, layout := range []string{time.RFC1123, time.RFC850, time.ANSIC} {
		if t, err := time.Parse(layout, value); err == nil {
			d := time.Until(t)
			if d < 0 {
				return 0, true
			}
			return d, true
		}
	}
	return 0, false
}
