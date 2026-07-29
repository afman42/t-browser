package main

import (
	"bytes"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsTransientStatus(t *testing.T) {
	transient := []int{429, 502, 503, 504}
	for _, code := range transient {
		if !isTransientStatus(code) {
			t.Errorf("isTransientStatus(%d) = false, want true", code)
		}
	}
	nonTransient := []int{200, 201, 301, 302, 400, 401, 403, 404, 500, 501}
	for _, code := range nonTransient {
		if isTransientStatus(code) {
			t.Errorf("isTransientStatus(%d) = true, want false", code)
		}
	}
}

func TestParseRetryAfterSeconds(t *testing.T) {
	d, ok := parseRetryAfter("10")
	if !ok {
		t.Error("parseRetryAfter(\"10\") should be ok")
	}
	if d != 10*time.Second {
		t.Errorf("parseRetryAfter(\"10\") = %v, want 10s", d)
	}

	if d, ok := parseRetryAfter("0"); !ok || d != 0 {
		t.Errorf("parseRetryAfter(\"0\") = (%v, %v), want (0s, true)", d, ok)
	}

	if _, ok := parseRetryAfter("-5"); ok {
		t.Error("parseRetryAfter(\"-5\") should not be ok")
	}

	if _, ok := parseRetryAfter(""); ok {
		t.Error("parseRetryAfter(\"\") should not be ok")
	}

	if _, ok := parseRetryAfter("not-a-number"); ok {
		t.Error("parseRetryAfter(\"not-a-number\") should not be ok")
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	future := time.Now().Add(3 * time.Second).UTC().Format(time.RFC1123)
	d, ok := parseRetryAfter(future)
	if !ok {
		t.Errorf("parseRetryAfter(http-date %q) should be ok", future)
	}
	if d <= 0 {
		t.Errorf("future HTTP-date should yield positive duration, got %v", d)
	}
	if d > 3*time.Second {
		t.Errorf("future HTTP-date duration too large: %v", d)
	}
}

func TestParseRetryAfterPastHTTPDate(t *testing.T) {
	// A date in the past should parse ok but yield a zero/negative duration
	// (clamped to 0 by the parseRetryAfter contract).
	past := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC1123)
	d, ok := parseRetryAfter(past)
	if !ok {
		t.Errorf("parseRetryAfter(past http-date %q) should be ok", past)
	}
	if d > 0 {
		t.Errorf("past HTTP-date should yield <= 0 duration, got %v", d)
	}
}

func TestRetryBackoffExponential(t *testing.T) {
	client := NewHTTPClient(nil)
	client.retryBaseDelay = 1 * time.Second

	resp := &http.Response{Header: make(http.Header)}

	if d := client.retryBackoff(resp, 0); d != 1*time.Second {
		t.Errorf("retryBackoff(0) = %v, want 1s", d)
	}
	if d := client.retryBackoff(resp, 1); d != 2*time.Second {
		t.Errorf("retryBackoff(1) = %v, want 2s", d)
	}
	if d := client.retryBackoff(resp, 2); d != 4*time.Second {
		t.Errorf("retryBackoff(2) = %v, want 4s", d)
	}
	// Capped at 30 seconds.
	if d := client.retryBackoff(resp, 10); d != 30*time.Second {
		t.Errorf("retryBackoff(10) = %v, want 30s (capped)", d)
	}
}

func TestRetryBackoffHonoursRetryAfter(t *testing.T) {
	client := NewHTTPClient(nil)
	client.retryBaseDelay = 1 * time.Second

	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "5")
	if d := client.retryBackoff(resp, 0); d != 5*time.Second {
		t.Errorf("retryBackoff with Retry-After: 5 = %v, want 5s", d)
	}

	// Retry-After beyond the cap is capped.
	resp.Header.Set("Retry-After", "120")
	if d := client.retryBackoff(resp, 0); d != 30*time.Second {
		t.Errorf("retryBackoff with Retry-After: 120 = %v, want 30s (capped)", d)
	}
}

func TestRetryBackoffHonoursRetryAfterHTTPDate(t *testing.T) {
	client := NewHTTPClient(nil)
	client.retryBaseDelay = 1 * time.Second

	resp := &http.Response{Header: make(http.Header)}
	future := time.Now().Add(4 * time.Second).UTC().Format(time.RFC1123)
	resp.Header.Set("Retry-After", future)
	d := client.retryBackoff(resp, 0)
	if d <= 0 || d > 4*time.Second {
		t.Errorf("retryBackoff with future HTTP-date should be in (0, 4s], got %v", d)
	}
}

func TestCacheTTLHelpers(t *testing.T) {
	// Default config: TTL 0 (always revalidate).
	client := NewHTTPClient(nil)
	if client.cacheTTL() != 0 {
		t.Errorf("cacheTTL() with nil config = %v, want 0", client.cacheTTL())
	}

	// Configured TTL is returned as a Duration.
	cfg := GetDefaultConfig()
	cfg.CacheTTLSeconds = 90
	c := NewHTTPClient(&cfg)
	if got := c.cacheTTL(); got != 90*time.Second {
		t.Errorf("cacheTTL() with 90s config = %v, want 90s", got)
	}
}

// newRetryMockClient builds an HTTPClient whose transport invokes respond for
// each call (call numbers start at 1). It returns the client and a pointer to
// the call counter so tests can assert on the number of network attempts.
func newRetryMockClient(t *testing.T, maxRetries int, respond func(call int, req *http.Request) (int, string)) (*HTTPClient, *int32) {
	t.Helper()
	var calls int32
	client := NewHTTPClient(nil)
	client.maxRetries = maxRetries
	client.retryBaseDelay = 1 * time.Millisecond
	client.client.Timeout = 5 * time.Second
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		n := int(atomic.AddInt32(&calls, 1))
		code, bodyStr := respond(n, req)
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{
			StatusCode:    code,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        header,
			Body:          io.NopCloser(bytes.NewReader([]byte(bodyStr))),
			ContentLength: int64(len(bodyStr)),
			Request:       req,
		}, nil
	})
	return client, &calls
}

func TestFetchPageRetriesOn503ThenSucceeds(t *testing.T) {
	client, calls := newRetryMockClient(t, 3, func(call int, _ *http.Request) (int, string) {
		if call == 1 {
			return 503, "down"
		}
		return 200, "ok"
	})

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if content != "ok" {
		t.Errorf("expected 'ok' after retry, got %q", content)
	}
	if got := atomic.LoadInt32(calls); got != 2 {
		t.Errorf("expected 2 network calls (1 + 1 retry), got %d", got)
	}
}

func TestFetchPageRetriesOn429ThenSucceeds(t *testing.T) {
	client, _ := newRetryMockClient(t, 3, func(call int, _ *http.Request) (int, string) {
		if call == 1 {
			return 429, "rate limited"
		}
		return 200, "ok"
	})

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if content != "ok" {
		t.Errorf("expected 'ok' after retry, got %q", content)
	}
}

func TestFetchPageRetriesOn502ThenSucceeds(t *testing.T) {
	client, calls := newRetryMockClient(t, 3, func(call int, _ *http.Request) (int, string) {
		if call == 1 {
			return 502, "bad gateway"
		}
		return 200, "ok"
	})

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if content != "ok" {
		t.Errorf("expected 'ok' after 502 retry, got %q", content)
	}
	if got := atomic.LoadInt32(calls); got != 2 {
		t.Errorf("expected 2 calls after 502 retry, got %d", got)
	}
}

func TestFetchPageRetriesOn504ThenSucceeds(t *testing.T) {
	client, calls := newRetryMockClient(t, 3, func(call int, _ *http.Request) (int, string) {
		if call == 1 {
			return 504, "gateway timeout"
		}
		return 200, "ok"
	})

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if content != "ok" {
		t.Errorf("expected 'ok' after 504 retry, got %q", content)
	}
	if got := atomic.LoadInt32(calls); got != 2 {
		t.Errorf("expected 2 calls after 504 retry, got %d", got)
	}
}

func TestFetchPageRetriesWithRetryAfterHeaderThenSucceeds(t *testing.T) {
	// The server returns 429 with a Retry-After: 1 header, then 200 on the
	// retry. Verifies the header is honoured end-to-end in FetchPage.
	client, calls := newRetryMockClient(t, 3, func(call int, _ *http.Request) (int, string) {
		if call == 1 {
			return 429, "rate limited"
		}
		return 200, "ok"
	})
	// Wrap the transport to inject the Retry-After header on the first call.
	inner := client.client.Transport
	var firstCall int32
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := inner.RoundTrip(req)
		if err == nil && atomic.AddInt32(&firstCall, 1) == 1 && resp.StatusCode == 429 {
			resp.Header.Set("Retry-After", "1")
		}
		return resp, err
	})

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if content != "ok" {
		t.Errorf("expected 'ok' after Retry-After, got %q", content)
	}
	if got := atomic.LoadInt32(calls); got != 2 {
		t.Errorf("expected 2 calls, got %d", got)
	}
}

func TestFetchPageRetriesUntilMaxThenReturnsBody(t *testing.T) {
	// Always 503: after maxRetries attempts it should fall through and return
	// the 503 response body (a reasonable "show the error page" behaviour).
	client, calls := newRetryMockClient(t, 2, func(call int, _ *http.Request) (int, string) {
		return 503, "service unavailable"
	})

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("expected 503 body returned after retries, got error: %v", err)
	}
	if content != "service unavailable" {
		t.Errorf("expected 'service unavailable', got %q", content)
	}
	// 1 initial + 2 retries = 3 calls.
	if got := atomic.LoadInt32(calls); got != 3 {
		t.Errorf("expected 3 network calls, got %d", got)
	}
}

func TestFetchPageMaxRetriesZeroDisablesRetry(t *testing.T) {
	client, calls := newRetryMockClient(t, 0, func(call int, _ *http.Request) (int, string) {
		return 503, "down"
	})

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "down" {
		t.Errorf("expected 'down', got %q", content)
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("maxRetries=0 should perform a single call, got %d", got)
	}
}

func TestFetchPageRetryThenRedirectSucceeds(t *testing.T) {
	// First call: 503 (transient, retried). Second call: 302 redirect to
	// /final. Third call: 200. Verifies retry + redirect compose.
	client, calls := newRetryMockClient(t, 3, func(call int, req *http.Request) (int, string) {
		switch call {
		case 1:
			return 503, "down"
		case 2:
			return http.StatusFound, "" // 302 — body unused, Location set below
		default:
			return 200, "final"
		}
	})
	// Wrap to inject the Location header on the 302 response.
	inner := client.client.Transport
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := inner.RoundTrip(req)
		if err == nil && resp.StatusCode == http.StatusFound {
			resp.Header.Set("Location", "/final")
		}
		return resp, err
	})

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if content != "final" {
		t.Errorf("expected 'final' after retry+redirect, got %q", content)
	}
	if got := atomic.LoadInt32(calls); got != 3 {
		t.Errorf("expected 3 calls (503 + 302 + 200), got %d", got)
	}
}

func TestFetchPageCacheTTLHitSkipsNetwork(t *testing.T) {
	var calls int32
	cfg := GetDefaultConfig()
	cfg.CacheTTLSeconds = 60
	client := NewHTTPClient(&cfg)
	client.client.Timeout = 5 * time.Second
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{
			StatusCode: 200, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
			Header: header, Body: io.NopCloser(bytes.NewReader(body("cached"))),
			ContentLength: 6, Request: req,
		}, nil
	})

	if _, err := client.FetchPage("http://mock.test/"); err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	// Second fetch within TTL should be served from cache with no network call.
	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	if content != "cached" {
		t.Errorf("expected cached content, got %q", content)
	}
	if calls != 1 {
		t.Errorf("expected 1 network call (TTL cache hit), got %d", calls)
	}
}

func TestFetchPageCacheTTLExpiryHitsNetwork(t *testing.T) {
	var calls int32
	cfg := GetDefaultConfig()
	cfg.CacheTTLSeconds = 60
	client := NewHTTPClient(&cfg)
	client.client.Timeout = 5 * time.Second
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{
			StatusCode: 200, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
			Header: header, Body: io.NopCloser(bytes.NewReader(body("fresh"))),
			ContentLength: 5, Request: req,
		}, nil
	})

	// Prime the cache with an entry whose CachedAt is well beyond the TTL.
	client.pageCache.Put("http://mock.test/", &CacheEntry{
		Content:  "stale",
		CachedAt: time.Now().Add(-2 * time.Minute),
	})

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if content != "fresh" {
		t.Errorf("expired TTL entry should fetch 'fresh', got %q", content)
	}
	if calls != 1 {
		t.Errorf("expired TTL entry should hit the network once, got %d", calls)
	}
}

func TestFetchPageCacheTTLZeroUsesConditionalRequest(t *testing.T) {
	// With TTL=0 the cache should not short-circuit; the conditional request
	// (If-None-Match) path is preserved, and a 304 returns the cached body.
	var calls int32
	cfg := GetDefaultConfig()
	cfg.CacheTTLSeconds = 0
	client := NewHTTPClient(&cfg)
	client.client.Timeout = 5 * time.Second
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	// Prime cache with an ETag.
	client.pageCache.Put("http://mock.test/", &CacheEntry{
		Content:  "cached-body",
		ETag:     "W/\"abc\"",
		CachedAt: time.Now(),
	})
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		if got := req.Header.Get("If-None-Match"); got != "W/\"abc\"" {
			t.Errorf("expected If-None-Match conditional, got %q", got)
		}
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{
			StatusCode: http.StatusNotModified, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
			Header: header, Body: io.NopCloser(bytes.NewReader(nil)),
			Request: req,
		}, nil
	})

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if content != "cached-body" {
		t.Errorf("304 should return cached body, got %q", content)
	}
	if calls != 1 {
		t.Errorf("TTL=0 conditional path should make 1 network call, got %d", calls)
	}
}
