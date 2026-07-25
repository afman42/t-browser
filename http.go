package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// HTTPClient handles HTTP requests with proper headers, cookies, and encoding
type HTTPClient struct {
	client       *http.Client
	cookies      map[string]*Cookie  // Map of domain to cookies
	forceUA      string
	proxy        *url.URL
	maxRedirects int
	config       *Config
	hstsStore    *HSTSStore          // HSTS policy store (nil when HSTS disabled)
}



// NewHTTPClient creates a new HTTP client with proper configuration
func NewHTTPClient(config *Config) *HTTPClient {
	// Use provided config values or defaults
	timeout := time.Duration(30)
	if config != nil && config.RequestTimeout > 0 {
		timeout = time.Duration(config.RequestTimeout) * time.Second
	}

	maxRedirects := 10
	if config != nil && config.MaxRedirects > 0 {
		maxRedirects = config.MaxRedirects
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

	// ----- Certificate pinning -----
	if config != nil && config.EnablePinning {
		pins, err := parsePinnedKeys(config.PinnedKeys)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: invalid pinned_keys in config: %v\n", err)
		}
		if len(pins) > 0 {
			transport.TLSClientConfig = setupTLSConfig(pins, true)
		}
	}

	// ----- HSTS store -----
	var hstsStore *HSTSStore
	if config != nil && config.EnableHSTS {
		configDir := GetConfigDir()
		hstsPath := GetHSTSFilePath(configDir)
		if store, err := LoadHSTS(hstsPath); err == nil {
			hstsStore = store
			hstsStore.Cleanup()
		}
	}

	// Wrap the transport with HSTS upgrade logic.
	finalTransport := http.RoundTripper(transport)
	if hstsStore != nil {
		finalTransport = &hstsTransport{inner: transport, store: hstsStore}
	}

	client := &HTTPClient{
		client: &http.Client{
			Transport: finalTransport,
			Timeout:   timeout,
		},
		cookies:      make(map[string]*Cookie),
		forceUA:      userAgent,
		maxRedirects: maxRedirects,
		config:       config,
		hstsStore:    hstsStore,
	}

	// Load cookies from persistent storage if available
	client.loadCookiesFromFile()

	return client
}

// SetProxy sets the proxy for the HTTP client
func (c *HTTPClient) SetProxy(proxy *url.URL) {
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.Proxy = http.ProxyURL(proxy)
	}
}



// fetchPageWithRedirectLimit fetches the page content with a redirect limit
func (c *HTTPClient) fetchPageWithRedirectLimit(rawURL string, redirectCount int) (string, error) {
	// Check redirect limit
	if redirectCount > c.maxRedirects {
		return "", fmt.Errorf("maximum redirect limit (%d) exceeded", c.maxRedirects)
	}

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Create request
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}

	// Set headers
	req.Header.Set("User-Agent", c.forceUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// Add cookies if available and enabled in config
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

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check if the content type is actually text/html
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") &&
	   !strings.Contains(strings.ToLower(contentType), "text/plain") &&
	   !strings.Contains(strings.ToLower(contentType), "application/xhtml+xml") {
		return "", fmt.Errorf("content type not supported: %s", contentType)
	}

	// Handle redirects manually to maintain cookies across redirects
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			// Validate redirect location to prevent open redirects
			if !isValidRedirectLocation(location, parsedURL) {
				return "", fmt.Errorf("invalid redirect location: %s", location)
			}

			// Create absolute URL if location is relative
			if strings.HasPrefix(location, "/") {
				location = fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, location)
			} else if !strings.HasPrefix(location, "http") {
				location = fmt.Sprintf("%s://%s/%s", parsedURL.Scheme, parsedURL.Host, location)
			}
			return c.fetchPageWithRedirectLimit(location, redirectCount+1)
		}
	}

	// Store cookies from response
	for _, httpCookie := range resp.Cookies() {
		c.addCookie(httpCookie, parsedURL.Host)
	}

	// ----- HSTS header processing -----
	if c.hstsStore != nil {
		if hstsHeader := resp.Header.Get("Strict-Transport-Security"); hstsHeader != "" {
			c.hstsStore.RecordPolicy(parsedURL.Hostname(), hstsHeader)
			c.saveHSTSStore()
		}
	}

	// Extract charset from content type
	encodingName := "utf-8"
	if idx := strings.Index(contentType, "charset="); idx != -1 {
		encodingName = strings.TrimSpace(contentType[idx+8:])
		encodingName = strings.Trim(encodingName, "\"'")
	}

	// Check if body is compressed with gzip and decompress if needed
	var reader io.Reader = resp.Body
	var originalGzipReader *gzip.Reader // Keep reference to close later if needed
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", err
		}
		originalGzipReader = gzipReader
		reader = gzipReader
	}

	// Read response body with size limit to prevent excessive memory usage
	// Limit to 50MB to prevent memory exhaustion from very large documents
	maxSize := int64(50 * 1024 * 1024) // 50MB
	body, err := io.ReadAll(io.LimitReader(reader, maxSize))
	if err != nil {
		// Close the original gzip reader if it was created
		if originalGzipReader != nil {
			originalGzipReader.Close()
		}
		return "", err
	}

	// Close the original gzip reader if it was created
	if originalGzipReader != nil {
		originalGzipReader.Close()
	}

	// Check if we hit the size limit
	if int64(len(body)) >= maxSize {
		return "", fmt.Errorf("response body exceeds maximum size of %d bytes", maxSize)
	}

	// Verify the body is valid text by checking for binary content
	// Check the first few bytes to see if they look like binary
	if len(body) > 0 {
		// Check for common binary file signatures
		if len(body) >= 4 {
			// Check for common binary signatures (magic numbers)
			if (body[0] == 0x89 && body[1] == 0x50 && body[2] == 0x4E && body[3] == 0x47) || // PNG
				(body[0] == 0xFF && body[1] == 0xD8 && body[2] == 0xFF) || // JPEG
				(body[0] == 0x47 && body[1] == 0x49 && body[2] == 0x46) || // GIF
				(body[0] == 0x50 && body[1] == 0x4B) || // ZIP/PDF
				(body[0] == 0x25 && body[1] == 0x50 && body[2] == 0x44 && body[3] == 0x46) { // PDF
				return "", fmt.Errorf("binary content detected, not text/html")
			}
		}
	}

	// If encoding is not UTF-8, try to convert it
	if strings.Contains(strings.ToLower(encodingName), "iso-8859") ||
	   strings.Contains(strings.ToLower(encodingName), "latin") {
		enc := charmap.ISO8859_1
		if strings.Contains(strings.ToLower(encodingName), "iso-8859-2") {
			enc = charmap.ISO8859_2
		} else if strings.Contains(strings.ToLower(encodingName), "iso-8859-15") {
			enc = charmap.ISO8859_15
		}
		decoder := enc.NewDecoder()
		// Apply size limit to the converted content as well
		maxSize := int64(50 * 1024 * 1024) // 50MB
		convertedBody, err := io.ReadAll(io.LimitReader(transform.NewReader(bytes.NewReader(body), decoder), maxSize))
		if err != nil {
			// If conversion fails, return with original body
			return "", fmt.Errorf("encoding conversion error: %v", err)
		}
		// Check if we hit the size limit after conversion
		if int64(len(convertedBody)) >= maxSize {
			return "", fmt.Errorf("converted content exceeds maximum size of %d bytes", maxSize)
		}
		body = convertedBody
	} else if strings.Contains(strings.ToLower(encodingName), "utf-16") {
		// Handle UTF-16 encoding
		decoder := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
		// Apply size limit to the converted content as well
		maxSize := int64(50 * 1024 * 1024) // 50MB
		convertedBody, err := io.ReadAll(io.LimitReader(transform.NewReader(bytes.NewReader(body), decoder), maxSize))
		if err != nil {
			// If conversion fails, return with original body
			return "", fmt.Errorf("encoding conversion error: %v", err)
		}
		// Check if we hit the size limit after conversion
		if int64(len(convertedBody)) >= maxSize {
			return "", fmt.Errorf("converted content exceeds maximum size of %d bytes", maxSize)
		}
		body = convertedBody
	}

	return string(body), nil
}

// FetchPage fetches the page content from the given URL
func (c *HTTPClient) FetchPage(rawURL string) (string, error) {
	return c.fetchPageWithRedirectLimit(rawURL, 0)
}

// isValidRedirectLocation checks if the redirect location is safe and valid
func isValidRedirectLocation(location string, originalURL *url.URL) bool {
	// Check if location is a full URL
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		redirectURL, err := url.Parse(location)
		if err != nil {
			return false
		}
		// Only allow redirects to the same host or subdomains.
		// Use exact match OR ".domain" suffix to prevent "foo.com.evil.com" bypass.
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

	// For relative redirects (starting with / or #), they are safe
	return strings.HasPrefix(location, "/") || strings.HasPrefix(location, "#") || strings.HasPrefix(location, "?")
}