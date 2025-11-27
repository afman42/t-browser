package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
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
	config *Config
}

// Cookie represents an HTTP cookie
type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Expires  time.Time // When the cookie expires
	MaxAge   int       // Max age in seconds
	Secure   bool      // Only send over HTTPS
	HttpOnly bool      // Not accessible via JavaScript
	SameSite string    // SameSite policy
}

// Matches checks if a cookie should be included in a request to the given URL
func (c *Cookie) Matches(requestURL *url.URL) bool {
	// Check domain matching (RFC 6265)
	if !matchesDomain(requestURL.Host, c.Domain) {
		return false
	}

	// Check path matching (RFC 6265)
	if !matchesPath(requestURL.Path, c.Path) {
		return false
	}

	// Check secure flag if HTTPS is required
	if c.Secure && requestURL.Scheme != "https" {
		return false
	}

	// Check if cookie is expired
	if !c.Expires.IsZero() && time.Now().After(c.Expires) {
		return false
	}

	return true
}

// matchesDomain checks if the request host matches the cookie domain
func matchesDomain(requestHost, cookieDomain string) bool {
	if cookieDomain == "" {
		return true
	}

	// Remove port from requestHost if present
	hostWithoutPort := strings.Split(requestHost, ":")[0]

	// Case-insensitive comparison
	hostWithoutPort = strings.ToLower(hostWithoutPort)
	cookieDomain = strings.ToLower(cookieDomain)

	// If exact match
	if hostWithoutPort == cookieDomain {
		return true
	}

	// If cookie domain starts with a dot
	if strings.HasPrefix(cookieDomain, ".") {
		// Check if request host is a subdomain
		suffix := cookieDomain[1:] // Remove the leading dot
		if strings.HasSuffix(hostWithoutPort, suffix) {
			// Check that the request host is longer than the suffix
			// and the character before the suffix is a dot
			if len(hostWithoutPort) > len(suffix) {
				prefix := hostWithoutPort[:len(hostWithoutPort)-len(suffix)]
				return strings.HasSuffix(prefix, ".")
			}
		}
	}

	return false
}

// matchesPath checks if the request path matches the cookie path
func matchesPath(requestPath, cookiePath string) bool {
	if cookiePath == "" || cookiePath == "/" {
		return true
	}

	if requestPath == "" {
		requestPath = "/"
	}

	// Path must be a prefix of the request path
	if !strings.HasPrefix(requestPath, cookiePath) {
		return false
	}

	// If cookie path is not followed by / in the request path, it doesn't match
	remaining := requestPath[len(cookiePath):]
	return len(remaining) == 0 || strings.HasPrefix(remaining, "/")
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

	client := &HTTPClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		cookies:      make(map[string]*Cookie),
		forceUA:      userAgent,
		maxRedirects: maxRedirects,
		config:       config, // Store the config for later use
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

// loadCookiesFromFile loads cookies from persistent storage
func (c *HTTPClient) loadCookiesFromFile() {
	cookieFile := "t-browser-cookies.json" // default
	if c.config != nil && c.config.CookieFile != "" {
		cookieFile = c.config.CookieFile
	}

	data, err := os.ReadFile(cookieFile)
	if err != nil {
		// File may not exist yet, which is fine
		return
	}

	var cookies []*Cookie
	err = json.Unmarshal(data, &cookies)
	if err != nil {
		// Log the error but continue without loaded cookies
		return
	}

	// Filter out expired cookies and populate the cookies map
	for _, cookie := range cookies {
		if cookie != nil && (cookie.Expires.IsZero() || time.Now().Before(cookie.Expires)) {
			// Use domain as key for the map
			c.cookies[cookie.Domain+"_"+cookie.Name] = cookie
		}
	}
}

// saveCookiesToFile saves cookies to persistent storage
func (c *HTTPClient) saveCookiesToFile() {
	cookieFile := "t-browser-cookies.json" // default
	if c.config != nil && c.config.CookieFile != "" {
		cookieFile = c.config.CookieFile
	}

	// Clean up expired cookies before saving
	c.cleanupExpiredCookies()

	var cookies []*Cookie
	for _, cookie := range c.cookies {
		cookies = append(cookies, cookie)
	}

	data, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		// Log the error but continue
		return
	}

	err = os.WriteFile(cookieFile, data, 0600) // Only readable/writable by owner
	if err != nil {
		// Log the error but continue
	}
}

// cleanupExpiredCookies removes expired cookies from the storage
func (c *HTTPClient) cleanupExpiredCookies() {
	now := time.Now()
	for key, cookie := range c.cookies {
		if !cookie.Expires.IsZero() && now.After(cookie.Expires) {
			delete(c.cookies, key)
		}
	}
}

// addCookie adds a cookie to the storage with proper validation
func (c *HTTPClient) addCookie(httpCookie *http.Cookie, host string) {
	// Create our internal Cookie representation
	cookie := &Cookie{
		Name:   httpCookie.Name,
		Value:  httpCookie.Value,
		Domain: httpCookie.Domain,
		Path:   httpCookie.Path,
		Secure: httpCookie.Secure,
		HttpOnly: httpCookie.HttpOnly,
	}

	// Handle expiration - try Expires first, then MaxAge
	if !httpCookie.Expires.IsZero() {
		cookie.Expires = httpCookie.Expires
	} else if httpCookie.MaxAge > 0 {
		cookie.Expires = time.Now().Add(time.Duration(httpCookie.MaxAge) * time.Second)
		cookie.MaxAge = httpCookie.MaxAge
	} else if httpCookie.MaxAge < 0 {
		// Delete the cookie if MaxAge is negative
		delete(c.cookies, httpCookie.Domain+"_"+httpCookie.Name)
		return
	}

	// If no domain was specified, use the host
	if cookie.Domain == "" {
		cookie.Domain = host
	}

	// If no path was specified, set it to the directory of the request path
	if cookie.Path == "" {
		cookie.Path = "/"
	}

	// Store the cookie in the map
	key := cookie.Domain + "_" + cookie.Name
	c.cookies[key] = cookie

	// Save to persistent storage
	c.saveCookiesToFile()
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
	if c.config == nil || c.config.EnableCookies {
		for _, cookie := range c.cookies {
			if cookie.Matches(parsedURL) {
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
		// Only allow redirects to the same host or subdomains
		// This prevents open redirect vulnerabilities
		return strings.HasSuffix(redirectURL.Host, originalURL.Host) || redirectURL.Host == originalURL.Host
	}

	// For relative redirects (starting with / or #), they are safe
	return strings.HasPrefix(location, "/") || strings.HasPrefix(location, "#") || strings.HasPrefix(location, "?")
}