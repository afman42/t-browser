package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/charmap"
)

// mockResponse represents a canned HTTP response for the mock transport.
type mockResponse struct {
	statusCode int
	headers    map[string]string
	body       []byte
}

// mockTransport implements http.RoundTripper by returning canned responses.
type mockTransport struct {
	pathMatches  map[string]mockResponse // path → response
	captureUA    string                  // captures User-Agent from last request
	requestCount int
}

func (mt *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	mt.requestCount++
	mt.captureUA = req.UserAgent()

	resp, ok := mt.pathMatches[req.URL.Path]
	if !ok {
		return nil, fmt.Errorf("no mock response for %s", req.URL.Path)
	}

	header := make(http.Header)
	for k, v := range resp.headers {
		header.Set(k, v)
	}

	return &http.Response{
		StatusCode:    resp.statusCode,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(resp.body)),
		ContentLength: int64(len(resp.body)),
		Request:       req,
	}, nil
}

// newMockClient creates an HTTPClient backed by a mock transport.
// It disables the Go HTTP client's automatic redirect following so that
// fetchPageWithRedirectLimit can handle redirects itself.
func newMockClient(responses map[string]mockResponse) *HTTPClient {
	client := NewHTTPClient(nil)
	client.client.Transport = &mockTransport{pathMatches: responses}
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Timeout = 5 * time.Second
	return client
}

func body(s string) []byte { return []byte(s) }

var htmlOK = mockResponse{200, map[string]string{"Content-Type": "text/html; charset=utf-8"}, body("<html><body>Hello World</body></html>")}
var textOK = mockResponse{200, map[string]string{"Content-Type": "text/plain; charset=utf-8"}, body("plain text content")}

// --- Basic FetchPage tests ---

func TestFetchPageReturnsContent(t *testing.T) {
	client := newMockClient(map[string]mockResponse{
		"/": htmlOK,
	})
	bodyContent, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if !strings.Contains(bodyContent, "Hello World") {
		t.Errorf("expected 'Hello World', got: %s", bodyContent)
	}
}

func TestFetchPageRejectsNonTextContent(t *testing.T) {
	client := newMockClient(map[string]mockResponse{
		"/": {200, map[string]string{"Content-Type": "application/octet-stream"}, body("binary data")},
	})
	_, err := client.FetchPage("http://mock.test/")
	if err == nil {
		t.Fatal("expected error for non-text content type")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("expected 'not supported' error, got: %v", err)
	}
}

func TestFetchPageAcceptsTextPlain(t *testing.T) {
	client := newMockClient(map[string]mockResponse{
		"/": textOK,
	})
	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if content != "plain text content" {
		t.Errorf("expected 'plain text content', got: %s", content)
	}
}

// --- Redirect tests ---

func TestFetchPageFollowsRedirect(t *testing.T) {
	mt := &mockTransport{
		pathMatches: map[string]mockResponse{
			"/start": {302, map[string]string{"Location": "/final", "Content-Type": "text/html; charset=utf-8"}, nil},
			"/final": {200, map[string]string{"Content-Type": "text/html; charset=utf-8"}, body("redirected content")},
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
	if content != "redirected content" {
		t.Errorf("expected 'redirected content', got: %s", content)
	}
	if mt.requestCount != 2 {
		t.Errorf("expected 2 requests (1 redirect), got %d", mt.requestCount)
	}
}

func TestFetchPageRedirectLimitExceeded(t *testing.T) {
	mt := &mockTransport{
		pathMatches: map[string]mockResponse{
			"/loop": {302, map[string]string{"Location": "/loop", "Content-Type": "text/html; charset=utf-8"}, nil},
		},
	}
	client := NewHTTPClient(nil)
	client.client.Transport = mt
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.maxRedirects = 3
	client.client.Timeout = 5 * time.Second

	_, err := client.FetchPage("http://mock.test/loop")
	if err == nil {
		t.Fatal("expected error for exceeding redirect limit")
	}
	if !strings.Contains(err.Error(), "maximum redirect limit") {
		t.Errorf("expected redirect limit error, got: %v", err)
	}
}

func TestFetchPageRejectsCrossDomainRedirect(t *testing.T) {
	mt := &mockTransport{
		pathMatches: map[string]mockResponse{
			"/start": {302, map[string]string{"Location": "https://evil.com/phish", "Content-Type": "text/html; charset=utf-8"}, nil},
		},
	}
	client := NewHTTPClient(nil)
	client.client.Transport = mt
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Timeout = 5 * time.Second

	_, err := client.FetchPage("http://mock.test/start")
	if err == nil {
		t.Fatal("expected error for cross-domain redirect")
	}
	if !strings.Contains(err.Error(), "invalid redirect location") {
		t.Errorf("expected 'invalid redirect location' error, got: %v", err)
	}
}

// --- Cookie tests ---

func TestFetchPageSendsAndStoresCookies(t *testing.T) {
	mt := &mockTransport{
		pathMatches: map[string]mockResponse{
			"/": {200, map[string]string{
				"Content-Type": "text/html; charset=utf-8",
				"Set-Cookie":   "session=abc123; Path=/",
			}, body("ok")},
		},
	}
	client := NewHTTPClient(nil)
	client.client.Transport = mt
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Timeout = 5 * time.Second

	_, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("first FetchPage failed: %v", err)
	}
	if len(client.cookies) == 0 {
		t.Fatal("expected cookies to be stored")
	}

	_, err = client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("second FetchPage failed: %v", err)
	}
	if len(client.cookies) == 0 {
		t.Error("cookies should persist across requests")
	}
}

func TestFetchPageCookieRoundTrip(t *testing.T) {
	// Use separate Set-Cookie header values (Go's http.Header handles this)
	header := make(http.Header)
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Add("Set-Cookie", "a=1; Path=/")
	header.Add("Set-Cookie", "b=2; Path=/")
	resp := &http.Response{
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body("ok"))),
		ContentLength: 2,
	}

	callCount := 0
	client := NewHTTPClient(nil)
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		return resp, nil
	})
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Timeout = 5 * time.Second

	_, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 request, got %d", callCount)
	}
	// Verify at least one cookie was stored (count may vary by Go version cookie parsing)
	if len(client.cookies) == 0 {
		t.Error("expected cookies to be stored, got none")
	}
}

// --- Binary content detection ---

func TestFetchPageRejectsBinaryContent(t *testing.T) {
	pngData := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, []byte("...")...)
	client := newMockClient(map[string]mockResponse{
		"/": {200, map[string]string{"Content-Type": "text/html; charset=utf-8"}, pngData},
	})
	_, err := client.FetchPage("http://mock.test/")
	if err == nil {
		t.Fatal("expected error for binary (PNG) content")
	}
	if !strings.Contains(err.Error(), "binary content") {
		t.Errorf("expected 'binary content' error, got: %v", err)
	}
}

func TestFetchPageRejectsJPEGContent(t *testing.T) {
	client := newMockClient(map[string]mockResponse{
		"/": {200, map[string]string{"Content-Type": "text/html; charset=utf-8"}, []byte{0xFF, 0xD8, 0xFF, 0xE0}},
	})
	_, err := client.FetchPage("http://mock.test/")
	if err == nil {
		t.Fatal("expected error for binary (JPEG) content")
	}
}

func TestFetchPageAcceptsNormalTextContent(t *testing.T) {
	client := newMockClient(map[string]mockResponse{
		"/": {200, map[string]string{"Content-Type": "text/html; charset=utf-8"}, body("<html>normal content</html>")},
	})
	_, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Errorf("normal HTML should not be rejected: %v", err)
	}
}

// --- Gzip encoding ---

func TestFetchPageHandlesGzip(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte("<html><body>gzipped content</body></html>"))
	gz.Close()

	client := newMockClient(map[string]mockResponse{
		"/": {200, map[string]string{"Content-Type": "text/html; charset=utf-8", "Content-Encoding": "gzip"}, buf.Bytes()},
	})
	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if !strings.Contains(content, "gzipped content") {
		t.Errorf("expected 'gzipped content', got: %s", content)
	}
}

// --- ISO-8859-1 encoding ---

func TestFetchPageHandlesLatin1Encoding(t *testing.T) {
	enc := charmap.ISO8859_1.NewEncoder()
	latin1Bytes, _ := enc.Bytes([]byte("café résumé"))

	client := newMockClient(map[string]mockResponse{
		"/": {200, map[string]string{"Content-Type": "text/html; charset=iso-8859-1"}, latin1Bytes},
	})
	content, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if !strings.Contains(content, "café") {
		t.Errorf("expected 'café', got: %s", content)
	}
}

// --- Custom config tests ---

func TestFetchPageUsesCustomUserAgent(t *testing.T) {
	mt := &mockTransport{
		pathMatches: map[string]mockResponse{
			"/": htmlOK,
		},
	}
	cfg := GetDefaultConfig()
	cfg.UserAgent = "test-browser/1.0"
	client := NewHTTPClient(&cfg)
	client.client.Transport = mt
	client.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client.client.Timeout = 5 * time.Second

	_, err := client.FetchPage("http://mock.test/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if mt.captureUA != "test-browser/1.0" {
		t.Errorf("expected UA 'test-browser/1.0', got: %s", mt.captureUA)
	}
}

func TestFetchPageSendsCookieOnRequest(t *testing.T) {
	client := NewHTTPClient(nil)
	client.cookies["domainkey_session"] = &Cookie{
		Name: "session", Value: "abc123", Domain: "domainkey", Path: "/",
	}
	client.client.Timeout = 5 * time.Second

	var seenCookies string
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seenCookies = req.Header.Get("Cookie")
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

	_, err := client.FetchPage("http://domainkey/")
	if err != nil {
		t.Fatalf("FetchPage failed: %v", err)
	}
	if !strings.Contains(seenCookies, "session=abc123") {
		t.Errorf("expected 'session=abc123' in Cookie header, got: %q", seenCookies)
	}
}

// roundTripFunc adapts a function to the http.RoundTripper interface.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
