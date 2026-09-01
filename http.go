package main

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net"
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
const maxCacheBytes = 64 << 20 // 64 MiB total page-content budget

type CacheEntry struct {
	ETag         string
	LastModified string
	Content      string
	CachedAt     time.Time
}

type PageCache struct {
	mu         sync.Mutex
	entries    map[string]*CacheEntry
	order      []string
	totalBytes int64
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
	if old, exists := c.entries[rawURL]; exists {
		c.totalBytes -= int64(len(old.Content))
	} else {
		c.order = append(c.order, rawURL)
	}
	c.totalBytes += int64(len(entry.Content))
	c.entries[rawURL] = entry

	// Evict by entry count AND total bytes: a few 50 MB pages must not pin
	// the whole cache budget.
	for len(c.order) > maxCacheEntries || c.totalBytes > maxCacheBytes {
		evict := c.order[0]
		c.order = c.order[1:]
		if old, ok := c.entries[evict]; ok {
			c.totalBytes -= int64(len(old.Content))
			if c.totalBytes < 0 {
				c.totalBytes = 0
			}
			delete(c.entries, evict)
		}
	}
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

	cookieMu  sync.RWMutex
	transport *http.Transport

	// confMu guards forceUA/maxRedirects/maxRetries and the transport/client
	// swap in applyConfig/SetProxy.  In-flight requests snapshot these under
	// RLock; config changes never mutate shared fields in place.
	confMu sync.RWMutex
}

func NewHTTPClient(config *Config) *HTTPClient {
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

	client := &HTTPClient{
		cookies:        make(map[string]*Cookie),
		forceUA:        userAgent,
		maxRedirects:   maxRedirects,
		maxRetries:     maxRetries,
		retryBaseDelay: 1 * time.Second,
		config:         config,
		pageCache:      NewPageCache(),
		transport:      transport,
	}
	client.ensureHSTSStore()
	client.client = client.buildClientLocked()
	client.loadCookiesFromFile()

	return client
}

// ensureHSTSStore loads the persisted HSTS store when HSTS is enabled and no
// store is attached yet.  A failed load leaves the store nil (HSTS off).
func (c *HTTPClient) ensureHSTSStore() {
	if c.config == nil || !c.config.EnableHSTS || c.hstsStore != nil {
		return
	}
	configDir := GetConfigDir()
	hstsPath := GetHSTSFilePath(configDir)
	if store, err := LoadHSTS(hstsPath); err == nil {
		store.Cleanup()
		c.hstsStore = store
	}
}

// buildClientLocked constructs a *http.Client carrying the current timeout and
// HSTS-wrapped transport.  Used by the constructor (before the client is
// shared) and by applyConfig/SetProxy, which call it under confMu.
func (c *HTTPClient) buildClientLocked() *http.Client {
	t := http.RoundTripper(c.transport)
	if c.config != nil && c.config.EnableHSTS && c.hstsStore != nil {
		t = &hstsTransport{inner: c.transport, store: c.hstsStore}
	}
	timeout := 30 * time.Second
	if c.config != nil && c.config.RequestTimeout > 0 {
		timeout = time.Duration(c.config.RequestTimeout) * time.Second
	}
	return &http.Client{
		Transport: t,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// applyConfig pushes live-configurable fields into the client so settings
// changes take effect without a restart.  Shared transport/client fields are
// never mutated in place: the transport is cloned, the client rebuilt, then
// both are swapped under confMu — an in-flight request sees the old or the
// new configuration, never a torn one.  Pinning and HSTS affect only new
// connections; pooled connections keep their old TLS config until idle.
func (c *HTTPClient) applyConfig() {
	if c.config == nil || c.transport == nil {
		return
	}
	c.confMu.Lock()
	defer c.confMu.Unlock()
	c.forceUA = c.config.UserAgent
	c.maxRedirects = c.config.MaxRedirects
	c.maxRetries = c.config.MaxRetries
	c.ensureHSTSStore()
	nc := c.transport.Clone()
	if c.config.Proxy != "" {
		if p, err := url.Parse(c.config.Proxy); err == nil {
			nc.Proxy = http.ProxyURL(p)
		}
	} else {
		nc.Proxy = nil
	}
	nc.TLSClientConfig = nil
	if c.config.EnablePinning && len(c.config.PinnedKeys) > 0 {
		if pins, err := parsePinnedKeys(c.config.PinnedKeys); err == nil {
			nc.TLSClientConfig = setupTLSConfig(pins, true)
		}
	}
	c.transport = nc
	c.client = c.buildClientLocked()
}

// SetUserAgent applies a user agent to new requests.
func (c *HTTPClient) SetUserAgent(ua string) {
	c.confMu.Lock()
	c.forceUA = ua
	c.confMu.Unlock()
}

// transportSnapshot returns the current base transport for side-channel
// requests (image preview) that share proxy/TLS configuration.
func (c *HTTPClient) transportSnapshot() *http.Transport {
	c.confMu.RLock()
	defer c.confMu.RUnlock()
	return c.transport
}

// checkRequestHost validates scheme, literal/legacy IP forms, the configured
// blocklist, and — for DNS names — that the host does not resolve to an
// internal address.  Runs at the start of every fetch and every redirect hop
// so a redirect chain can never land on a private/blocked target (SSRF).
func (c *HTTPClient) checkRequestHost(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}
	host := u.Hostname()
	if isInternalAddress(host) {
		return fmt.Errorf("access to internal address not allowed: %s", host)
	}
	if c.config != nil && len(c.config.BlockedDomains) > 0 && isBlockedDomain(host, c.config.BlockedDomains) {
		return fmt.Errorf("access to blocked domain not allowed: %s", host)
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	// DNS names: confirm every resolved address is public.  A lookup failure
	// falls through — the request itself will fail, and this keeps the check
	// usable in offline environments.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil
	}
	for _, a := range ips {
		if a.IP != nil && isInternalIP(a.IP) {
			return fmt.Errorf("host %s resolves to internal address %s", host, a.IP)
		}
	}
	return nil
}

func (c *HTTPClient) CancelRequest() {
	c.cancelMu.Lock()
	defer c.cancelMu.Unlock()
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
}

func (c *HTTPClient) SetProxy(proxy *url.URL) {
	if c.transport == nil {
		return
	}
	c.confMu.Lock()
	defer c.confMu.Unlock()
	nc := c.transport.Clone()
	if proxy == nil {
		nc.Proxy = nil
	} else {
		nc.Proxy = http.ProxyURL(proxy)
	}
	c.transport = nc
	c.client = c.buildClientLocked()
}

func (c *HTTPClient) fetchPageWithRedirectLimit(rawURL string, redirectCount, retryCount int) (string, error) {
	// Snapshot live-configurable state once per request: applyConfig/SetProxy
	// swap these under confMu, so each request sees one consistent config.
	c.confMu.RLock()
	maxRedirects := c.maxRedirects
	maxRetries := c.maxRetries
	ua := c.forceUA
	client := c.client
	hstsStore := c.hstsStore
	c.confMu.RUnlock()

	if redirectCount > maxRedirects {
		return "", fmt.Errorf("maximum redirect limit (%d) exceeded", maxRedirects)
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	if err := c.checkRequestHost(parsedURL); err != nil {
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

	req.Header.Set("User-Agent", ua)
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
		c.cookieMu.RLock()
		for _, cookie := range c.cookies {
			if cookie.Matches(parsedURL, enforceSameSite) {
				req.AddCookie(&http.Cookie{
					Name:  cookie.Name,
					Value: cookie.Value,
				})
			}
		}
		c.cookieMu.RUnlock()
	}

	resp, err := client.Do(req)
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
	if isTransientStatus(resp.StatusCode) && retryCount < maxRetries {
		backoff := c.retryBackoff(resp, retryCount)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(backoff):
		}
		return c.fetchPageWithRedirectLimit(rawURL, 0, retryCount+1)
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		for _, httpCookie := range resp.Cookies() {
			c.addCookie(httpCookie, parsedURL.Hostname())
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
			// Re-validate every hop: a redirect chain must never escape the
			// same-origin / public-address policy (SSRF).
			if err := c.checkRequestHost(resolved); err != nil {
				return "", fmt.Errorf("blocked redirect target %s: %w", resolved.String(), err)
			}
			return c.fetchPageWithRedirectLimit(resolved.String(), redirectCount+1, retryCount)
		}
	}

	// Surface server errors after retry exhaustion instead of rendering the
	// 5xx body as page content.
	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("server error: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") &&
		!strings.Contains(strings.ToLower(contentType), "text/plain") &&
		!strings.Contains(strings.ToLower(contentType), "application/xhtml+xml") {
		return "", fmt.Errorf("content type not supported: %s", contentType)
	}

	for _, httpCookie := range resp.Cookies() {
		c.addCookie(httpCookie, parsedURL.Hostname())
	}

	if hstsStore != nil {
		if hstsHeader := resp.Header.Get("Strict-Transport-Security"); hstsHeader != "" {
			hstsStore.RecordPolicy(parsedURL.Hostname(), hstsHeader)
			c.saveHSTSStore(hstsStore)
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
		// Some servers send raw DEFLATE (RFC 1951) instead of zlib-wrapped
		// (RFC 1950).  Distinguish by the zlib header before choosing.
		br := bufio.NewReader(resp.Body)
		hdr, err := br.Peek(2)
		if err != nil || len(hdr) < 2 || hdr[0]&0x0f != 8 || hdr[0]>>4 > 7 {
			fr := flate.NewReader(br)
			reader = fr
			closer = fr
		} else {
			zr, err := zlib.NewReader(br)
			if err != nil {
				return "", err
			}
			reader = zr
			closer = zr
		}
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

	// Protocol-relative ("//host") redirects are cross-host: reject them, they
	// would bypass the same-origin policy checked above.
	if strings.HasPrefix(location, "//") {
		return false
	}
	return strings.HasPrefix(location, "/") || strings.HasPrefix(location, "#") || strings.HasPrefix(location, "?")
}
