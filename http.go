package main

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/brotli"
	"golang.org/x/net/http2"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const maxCacheEntries = 50

type CacheEntry struct {
	ETag         string
	LastModified string
	Content      string
	CachedAt     time.Time
}

type PageCache struct {
	mu      sync.Mutex
	entries map[string]*CacheEntry
	order   []string
}

func NewPageCache() *PageCache {
	return &PageCache{entries: make(map[string]*CacheEntry)}
}

func (c *PageCache) Get(rawURL string) *CacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entries[rawURL]
}

func (c *PageCache) Put(rawURL string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[rawURL]; !exists {
		c.order = append(c.order, rawURL)
		if len(c.order) > maxCacheEntries {
			evict := c.order[0]
			c.order = c.order[1:]
			delete(c.entries, evict)
		}
	}
	c.entries[rawURL] = entry
}

type HTTPClient struct {
	client         *http.Client
	cookies        map[string]*Cookie
	forceUA        string
	proxy          *url.URL
	maxRedirects   int
	maxRetries     int
	retryBaseDelay time.Duration
	config         *Config
	hstsStore      *HSTSStore
	pageCache      *PageCache
	cancelMu       sync.Mutex
	cancelFunc     context.CancelFunc
	reqID          int64
}

func NewHTTPClient(config *Config) *HTTPClient {
	timeout := 30 * time.Second
	if config != nil && config.RequestTimeout > 0 {
		timeout = time.Duration(config.RequestTimeout) * time.Second
	}

	maxRedirects := 10
	if config != nil && config.MaxRedirects > 0 {
		maxRedirects = config.MaxRedirects
	}

	maxRetries := 3
	if config != nil {
		maxRetries = config.MaxRetries
	}

	userAgent := "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	if config != nil && config.UserAgent != "" {
		userAgent = config.UserAgent
	}

	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if err := http2.ConfigureTransport(transport); err != nil {
		fmt.Fprintf(os.Stderr, "warning: HTTP/2 setup failed: %v\n", err)
	}

	if config != nil && config.EnablePinning {
		pins, err := parsePinnedKeys(config.PinnedKeys)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: invalid pinned_keys: %v\n", err)
		}
		if len(pins) > 0 {
			transport.TLSClientConfig = setupTLSConfig(pins, true)
		}
	}

	var hstsStore *HSTSStore
	if config != nil && config.EnableHSTS {
		configDir := GetConfigDir()
		hstsPath := GetHSTSFilePath(configDir)
		if store, err := LoadHSTS(hstsPath); err == nil {
			hstsStore = store
			hstsStore.Cleanup()
		}
	}

	finalTransport := http.RoundTripper(transport)
	if hstsStore != nil {
		finalTransport = &hstsTransport{inner: transport, store: hstsStore}
	}

	client := &HTTPClient{
		client: &http.Client{
			Transport: finalTransport,
			Timeout:   timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		cookies:        make(map[string]*Cookie),
		forceUA:        userAgent,
		maxRedirects:   maxRedirects,
		maxRetries:     maxRetries,
		retryBaseDelay: 1 * time.Second,
		config:         config,
		hstsStore:      hstsStore,
		pageCache:      NewPageCache(),
	}

	client.loadCookiesFromFile()

	return client
}

func (c *HTTPClient) CancelRequest() {
	c.cancelMu.Lock()
	defer c.cancelMu.Unlock()
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
}

func (c *HTTPClient) SetProxy(proxy *url.URL) {
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.Proxy = http.ProxyURL(proxy)
	}
}

func (c *HTTPClient) fetchPageWithRedirectLimit(rawURL string, redirectCount, retryCount int) (string, error) {
	if redirectCount > c.maxRedirects {
		return "", fmt.Errorf("maximum redirect limit (%d) exceeded", c.maxRedirects)
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	myReqID := atomic.AddInt64(&c.reqID, 1)
	ctx := context.Background()
	c.cancelMu.Lock()
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	ctx, cancel := context.WithCancel(ctx)
	c.cancelFunc = cancel
	c.cancelMu.Unlock()
	defer func() {
		c.cancelMu.Lock()
		if atomic.LoadInt64(&c.reqID) == myReqID {
			c.cancelFunc = nil
		}
		c.cancelMu.Unlock()
		cancel()
	}()

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", c.forceUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	var cachedEntry *CacheEntry
	if c.pageCache != nil {
		cachedEntry = c.pageCache.Get(rawURL)
		// Client-side cache TTL: serve entries younger than the configured TTL
		// directly from the cache without any network round-trip.
		if cachedEntry != nil && c.cacheTTL() > 0 && time.Since(cachedEntry.CachedAt) < c.cacheTTL() {
			return cachedEntry.Content, nil
		}
		if cachedEntry != nil {
			if cachedEntry.ETag != "" {
				req.Header.Set("If-None-Match", cachedEntry.ETag)
			}
			if cachedEntry.LastModified != "" {
				req.Header.Set("If-Modified-Since", cachedEntry.LastModified)
			}
		}
	}

	enforceSameSite := c.config != nil && c.config.EnforceSameSite
	if c.config == nil || c.config.EnableCookies {
		for _, cookie := range c.cookies {
			if cookie.Matches(parsedURL, enforceSameSite) {
				req.AddCookie(&http.Cookie{
					Name:  cookie.Name,
					Value: cookie.Value,
				})
			}
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && cachedEntry != nil {
		return cachedEntry.Content, nil
	}

	// Retry transient failures (429 Too Many Requests, 502/503/504) with an
	// exponential backoff that honours the server's Retry-After header.  This
	// runs before body processing so non-HTML error bodies are retried too.
	// The backoff is interruptible by the request context so Esc still cancels.
	if isTransientStatus(resp.StatusCode) && retryCount < c.maxRetries {
		backoff := c.retryBackoff(resp, retryCount)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(backoff):
		}
		return c.fetchPageWithRedirectLimit(rawURL, 0, retryCount+1)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") &&
		!strings.Contains(strings.ToLower(contentType), "text/plain") &&
		!strings.Contains(strings.ToLower(contentType), "application/xhtml+xml") {
		return "", fmt.Errorf("content type not supported: %s", contentType)
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		for _, httpCookie := range resp.Cookies() {
			c.addCookie(httpCookie, parsedURL.Host)
		}

		location := resp.Header.Get("Location")
		if location != "" {
			if !isValidRedirectLocation(location, parsedURL) {
				return "", fmt.Errorf("invalid redirect location: %s", location)
			}

			locURL, parseErr := url.Parse(location)
			if parseErr != nil {
				return "", fmt.Errorf("invalid redirect URL: %w", parseErr)
			}
			resolved := parsedURL.ResolveReference(locURL)
			return c.fetchPageWithRedirectLimit(resolved.String(), redirectCount+1, retryCount)
		}
	}

	for _, httpCookie := range resp.Cookies() {
		c.addCookie(httpCookie, parsedURL.Host)
	}

	if c.hstsStore != nil {
		if hstsHeader := resp.Header.Get("Strict-Transport-Security"); hstsHeader != "" {
			c.hstsStore.RecordPolicy(parsedURL.Hostname(), hstsHeader)
			c.saveHSTSStore()
		}
	}

	encodingName := "utf-8"
	if idx := strings.Index(contentType, "charset="); idx != -1 {
		encodingName = strings.TrimSpace(contentType[idx+8:])
		encodingName = strings.Trim(encodingName, "\"'")
	}

	maxSize := int64(50 * 1024 * 1024)
	if c.config != nil && c.config.MaxPageSize > 0 {
		maxSize = c.config.MaxPageSize
	}

	var reader io.Reader = resp.Body
	var closer io.Closer
	contentEncoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	switch contentEncoding {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", err
		}
		reader = gr
		closer = gr
	case "deflate":
		zr, err := zlib.NewReader(resp.Body)
		if err != nil {
			return "", err
		}
		reader = zr
		closer = zr
	case "br":
		reader = brotli.NewReader(resp.Body)
	}
	defer func() {
		if closer != nil {
			closer.Close()
		}
	}()

	body, err := io.ReadAll(io.LimitReader(reader, maxSize))
	if err != nil {
		return "", err
	}

	if int64(len(body)) >= maxSize {
		return "", fmt.Errorf("response body exceeds maximum size of %d bytes", maxSize)
	}

	if len(body) > 0 && isBinaryContent(body) {
		return "", fmt.Errorf("binary content detected, not text/html")
	}

	if !strings.Contains(strings.ToLower(contentType), "charset=") {
		if metaCharset := detectMetaCharset(body); metaCharset != "" {
			encodingName = metaCharset
		}
	}

	converted, err := convertEncoding(body, encodingName, maxSize)
	if err != nil {
		return "", err
	}
	if converted != nil {
		body = converted
	}

	result := string(body)

	if c.pageCache != nil && resp.StatusCode == 200 {
		c.pageCache.Put(rawURL, &CacheEntry{
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
			Content:      result,
			CachedAt:     time.Now(),
		})
	}

	return result, nil
}

func isBinaryContent(body []byte) bool {
	if len(body) < 4 {
		return false
	}
	if body[0] == 0x89 && body[1] == 0x50 && body[2] == 0x4E && body[3] == 0x47 {
		return true
	}
	if body[0] == 0xFF && body[1] == 0xD8 && body[2] == 0xFF {
		return true
	}
	if body[0] == 0x47 && body[1] == 0x49 && body[2] == 0x46 {
		return true
	}
	if body[0] == 0x50 && body[1] == 0x4B {
		return true
	}
	if body[0] == 0x25 && body[1] == 0x50 && body[2] == 0x44 && body[3] == 0x46 {
		return true
	}
	return false
}

var metaCharsetRegex = regexp.MustCompile(`(?i)<meta\s+[^>]*(?:charset=["']?([a-zA-Z0-9_\-]+)["']?|http-equiv=["']?content-type["']?\s+content=["'][^"']*charset=([a-zA-Z0-9_\-]+))`)

func detectMetaCharset(body []byte) string {
	scanLen := 1024
	if len(body) < scanLen {
		scanLen = len(body)
	}
	m := metaCharsetRegex.FindSubmatch(body[:scanLen])
	if m == nil {
		return ""
	}
	for _, g := range m[1:] {
		if len(g) > 0 {
			return strings.ToLower(string(g))
		}
	}
	return ""
}

func convertEncoding(body []byte, encodingName string, maxSize int64) ([]byte, error) {
	encLower := strings.ToLower(encodingName)

	if strings.Contains(encLower, "utf-8") || strings.Contains(encLower, "utf8") || encLower == "" {
		return nil, nil
	}

	var decoder *transform.Reader

	switch {
	case strings.Contains(encLower, "iso-8859-2"):
		decoder = transform.NewReader(bytes.NewReader(body), charmap.ISO8859_2.NewDecoder())
	case strings.Contains(encLower, "iso-8859-15"):
		decoder = transform.NewReader(bytes.NewReader(body), charmap.ISO8859_15.NewDecoder())
	case strings.Contains(encLower, "iso-8859") || strings.Contains(encLower, "latin"):
		decoder = transform.NewReader(bytes.NewReader(body), charmap.ISO8859_1.NewDecoder())
	case strings.Contains(encLower, "utf-16"):
		decoder = transform.NewReader(bytes.NewReader(body), unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder())
	default:
		return nil, nil
	}

	converted, err := io.ReadAll(io.LimitReader(decoder, maxSize))
	if err != nil {
		return nil, fmt.Errorf("encoding conversion error: %v", err)
	}
	if int64(len(converted)) >= maxSize {
		return nil, fmt.Errorf("converted content exceeds maximum size of %d bytes", maxSize)
	}
	return converted, nil
}

func (c *HTTPClient) FetchPage(rawURL string) (string, error) {
	return c.fetchPageWithRedirectLimit(rawURL, 0, 0)
}

func isValidRedirectLocation(location string, originalURL *url.URL) bool {
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		redirectURL, err := url.Parse(location)
		if err != nil {
			return false
		}
		host := originalURL.Hostname()
		rhost := redirectURL.Hostname()
		if rhost == host {
			return true
		}
		if strings.HasSuffix(rhost, "."+host) {
			return true
		}
		return false
	}

	return strings.HasPrefix(location, "/") || strings.HasPrefix(location, "#") || strings.HasPrefix(location, "?")
}
