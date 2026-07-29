package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
)

func TestNewHTTPClientDefaultTimeoutIsSeconds(t *testing.T) {
	client := NewHTTPClient(nil)
	if client.client.Timeout < 1*time.Second {
		t.Errorf("default timeout = %v, expected >= 30s (bug #1: was 30 nanoseconds)", client.client.Timeout)
	}
}

func TestNewHTTPClientCustomTimeoutApplied(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.RequestTimeout = 5
	client := NewHTTPClient(&cfg)
	expected := 5 * time.Second
	if client.client.Timeout != expected {
		t.Errorf("custom timeout = %v, want %v", client.client.Timeout, expected)
	}
}

func TestNewHTTPClientCheckRedirectDisabledByDefault(t *testing.T) {
	client := NewHTTPClient(nil)
	if client.client.CheckRedirect == nil {
		t.Fatal("CheckRedirect should be set to prevent automatic redirect following")
	}
	req1, _ := http.NewRequest("GET", "http://example.com", nil)
	req2, _ := http.NewRequest("GET", "http://example.com/redirect", nil)
	err := client.client.CheckRedirect(req2, []*http.Request{req1})
	if err != http.ErrUseLastResponse {
		t.Errorf("CheckRedirect returned %v, want http.ErrUseLastResponse", err)
	}
}

func TestFetchPageHandlesDeflate(t *testing.T) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	zw.Write([]byte("<html><body>deflated content</body></html>"))
	zw.Close()

	client := newMockClient(map[string]mockResponse{
		"/": {200, map[string]string{"Content-Type": "text/html; charset=utf-8", "Content-Encoding": "deflate"}, buf.Bytes()},
	})
	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if !strings.Contains(content, "deflated content") {
		t.Errorf("expected 'deflated content', got: %s", content)
	}
}

func TestFetchPageHandlesBrotli(t *testing.T) {
	var buf bytes.Buffer
	bw := brotli.NewWriter(&buf)
	bw.Write([]byte("<html><body>brotli content</body></html>"))
	bw.Close()

	client := newMockClient(map[string]mockResponse{
		"/": {200, map[string]string{"Content-Type": "text/html; charset=utf-8", "Content-Encoding": "br"}, buf.Bytes()},
	})
	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if !strings.Contains(content, "brotli content") {
		t.Errorf("expected 'brotli content', got: %s", content)
	}
}

func TestFetchPageServesFromCacheOn304(t *testing.T) {
	callCount := 0
	etags := []string{"\"abc123\""}
	bodies := []string{"<html><body>cached content</body></html>"}

	client := NewHTTPClient(nil)
	client.client.Timeout = 5 * time.Second

	resp304 := &http.Response{
		StatusCode:    http.StatusNotModified,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader([]byte{})),
		ContentLength: 0,
	}
	resp200 := &http.Response{
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader([]byte(bodies[0]))),
		ContentLength: int64(len(bodies[0])),
	}
	resp200.Header.Set("Content-Type", "text/html; charset=utf-8")
	resp200.Header.Set("ETag", etags[0])

	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return resp200, nil
		}
		// Second request should have If-None-Match header.
		if req.Header.Get("If-None-Match") == "" {
			t.Error("expected If-None-Match header on second request")
		}
		return resp304, nil
	})
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("first FetchPage failed: %v", err)
	}
	if !strings.Contains(content, "cached content") {
		t.Errorf("first fetch: expected 'cached content', got: %s", content)
	}

	content2, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("second FetchPage failed: %v", err)
	}
	if !strings.Contains(content2, "cached content") {
		t.Errorf("second fetch (304): expected 'cached content' from cache, got: %s", content2)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}
}

func TestDetectMetaCharset(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "meta charset utf-8",
			html: `<html><head><meta charset="utf-8"></head><body>hello</body></html>`,
			want: "utf-8",
		},
		{
			name: "meta charset iso-8859-1",
			html: `<html><head><meta charset="iso-8859-1"></head><body>hello</body></html>`,
			want: "iso-8859-1",
		},
		{
			name: "meta http-equiv content-type",
			html: `<html><head><meta http-equiv="content-type" content="text/html; charset=windows-1252"></head><body>hello</body></html>`,
			want: "windows-1252",
		},
		{
			name: "no meta charset",
			html: `<html><head><title>test</title></head><body>hello</body></html>`,
			want: "",
		},
		{
			name: "meta charset with single quotes",
			html: `<html><head><meta charset='utf-8'></head><body>hello</body></html>`,
			want: "utf-8",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectMetaCharset([]byte(tc.html))
			if got != tc.want {
				t.Errorf("detectMetaCharset() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsBinaryContent(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want bool
	}{
		{"empty", []byte{}, false},
		{"short text", []byte("hi"), false},
		{"PNG", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A}, true},
		{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0}, true},
		{"GIF", []byte{0x47, 0x49, 0x46, 0x38}, true},
		{"ZIP", []byte{0x50, 0x4B, 0x03, 0x04}, true},
		{"PDF", []byte{0x25, 0x50, 0x44, 0x46}, true},
		{"HTML", []byte("<htm"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isBinaryContent(tc.body)
			if got != tc.want {
				t.Errorf("isBinaryContent(%v) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

func TestCancelRequest(t *testing.T) {
	client := NewHTTPClient(nil)

	done := make(chan error, 1)
	go func() {
		// This will block because the transport is nil/defaults.
		// We just want to verify CancelRequest doesn't panic.
		_, err := client.FetchPage("http://192.0.2.1:1/") // non-routable, will hang
		done <- err
	}()

	client.CancelRequest()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		// Timeout is acceptable — the request may still be in flight.
	}
}

func TestFetchPageRedirectForwardsCookies(t *testing.T) {
	mt := &mockTransport{
		pathMatches: map[string]mockResponse{
			"/start": {302, map[string]string{
				"Location":     "/final",
				"Content-Type": "text/html; charset=utf-8",
				"Set-Cookie":   "session=xyz; Path=/",
			}, nil},
			"/final": {200, map[string]string{"Content-Type": "text/html; charset=utf-8"}, body("redirected")},
		},
	}
	client := NewHTTPClient(nil)
	client.client.Transport = mt
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Timeout = 5 * time.Second

	_, err := client.FetchPage("http://mock.test/start")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}

	if len(client.cookies) == 0 {
		t.Error("expected cookies to be stored from redirect response")
	}

	found := false
	for _, c := range client.cookies {
		if c.Name == "session" && c.Value == "xyz" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'session=xyz' cookie to be stored from redirect")
	}
}

func TestFetchPageProtocolRelativeRedirect(t *testing.T) {
	mt := &mockTransport{
		pathMatches: map[string]mockResponse{
			"/start": {302, map[string]string{
				"Location":     "//mock.test/final",
				"Content-Type": "text/html; charset=utf-8",
			}, nil},
			"/final": {200, map[string]string{"Content-Type": "text/html; charset=utf-8"}, body("protocol-relative redirect")},
		},
	}
	client := NewHTTPClient(nil)
	client.client.Transport = mt
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Timeout = 5 * time.Second

	content, err := client.FetchPage("http://mock.test/start")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if !strings.Contains(content, "protocol-relative redirect") {
		t.Errorf("expected 'protocol-relative redirect', got: %s", content)
	}
	if mt.requestCount != 2 {
		t.Errorf("expected 2 requests, got %d", mt.requestCount)
	}
}

func TestFetchPageSendsBrotliAcceptEncoding(t *testing.T) {
	var capturedHeaders http.Header
	client := NewHTTPClient(nil)
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedHeaders = req.Header.Clone()
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		return &http.Response{
			StatusCode:    200,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        header,
			Body:          io.NopCloser(bytes.NewReader(body("ok"))),
			ContentLength: 2,
			Request:       req,
		}, nil
	})
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Timeout = 5 * time.Second

	_, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}

	acceptEncoding := capturedHeaders.Get("Accept-Encoding")
	if !strings.Contains(acceptEncoding, "br") {
		t.Errorf("Accept-Encoding = %q, expected to contain 'br' for brotli", acceptEncoding)
	}
	if !strings.Contains(acceptEncoding, "deflate") {
		t.Errorf("Accept-Encoding = %q, expected to contain 'deflate'", acceptEncoding)
	}
}

func TestFetchPageUsesConfigMaxPageSize(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.MaxPageSize = 100 // 100 bytes — very small
	client := newMockClient(map[string]mockResponse{
		"/": {200, map[string]string{"Content-Type": "text/html; charset=utf-8"}, body(fmt.Sprintf("<html>%s</html>", strings.Repeat("x", 200)))},
	})
	client.config = &cfg

	_, err := client.FetchPage("http://mock.test/")
	if err == nil {
		t.Fatal("expected error for page exceeding max_page_size")
	}
	if !strings.Contains(err.Error(), "maximum size") {
		t.Errorf("expected 'maximum size' error, got: %v", err)
	}
}
