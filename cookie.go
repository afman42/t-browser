package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

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

// Matches checks if a cookie should be included in a request to the given URL.
// enforceSameSite controls whether SameSite=Lax/Strict is enforced.
func (c *Cookie) Matches(requestURL *url.URL, enforceSameSite bool) bool {
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

	// Enforce SameSite when enabled.
	if enforceSameSite && c.SameSite != "" {
		switch c.SameSite {
		case "Strict":
			// Strict: cookie must be sent only for requests to the cookie's own
			// registrable domain. Since this terminal browser only makes
			// top-level navigations (no sub-requests), Strict means the
			// request host must exactly match the cookie domain.
			if !isSameSite(requestURL.Hostname(), c.Domain) {
				return false
			}
		case "Lax":
			// Lax: allowed for top-level navigations (which is all this browser
			// does). No additional restriction beyond domain/path/secure/expiry.
		default:
			// None: always send (no restriction).
		}
	}

	return true
}

// validCookieDomain reports whether the server-set cookie domain is an exact
// match for the host or a parent-domain suffix of it (RFC 6265 §5.3).  A
// cookie claiming a domain outside the setting host's control is rejected;
// the caller falls back to a host-only cookie, which is what real browsers do.
func validCookieDomain(host, domain string) bool {
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	domain = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "."))
	if domain == "" {
		return true
	}
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// isSameSite returns true when the request host is the same site as the
// cookie domain. Two hosts are same-site when the cookie domain is either
// an exact match for the host or is a parent domain with a leading dot.
func isSameSite(requestHost, cookieDomain string) bool {
	rh := strings.ToLower(requestHost)
	cd := strings.ToLower(cookieDomain)

	if strings.HasPrefix(cd, ".") {
		// Cookie scoped to a domain + all subdomains.
		// e.g. cookie Domain=.example.com, request host sub.example.com
		suffix := cd[1:] // strip leading dot
		return rh == suffix || strings.HasSuffix(rh, "."+suffix)
	}
	// Exact match required.
	return rh == cd
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
		// Check if request host is a subdomain or the bare domain itself
		suffix := cookieDomain[1:] // Remove the leading dot
		if hostWithoutPort == suffix {
			return true
		}
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

// loadCookiesFromFile loads cookies from persistent storage
func (c *HTTPClient) loadCookiesFromFile() {
	c.cookieMu.Lock()
	defer c.cookieMu.Unlock()
	var cookieFile string

	if c.config != nil {
		// Use the config directory to find the latest cookie file
		configDir := GetConfigDir()
		cookieFile = GetLatestCookieFile(configDir)
	}

	// If no file found or config is not available, use the old method as fallback
	if cookieFile == "" {
		cookieFile = "t-browser-cookies.json"
		if c.config != nil && c.config.CookieFile != "" {
			cookieFile = c.config.CookieFile
		}
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
	c.cleanupExpiredCookies()
	var cookieFile string

	if c.config != nil {
		// Use the config directory to get a new timestamped cookie file
		configDir := GetConfigDir()
		cookieFile = GetCookieFilePath(configDir)
	}

	// If config is not available, use fallback
	if cookieFile == "" {
		cookieFile = "t-browser-cookies.json"
		if c.config != nil && c.config.CookieFile != "" {
			cookieFile = c.config.CookieFile
		}
	}

	c.cookieMu.RLock()
	var cookies []*Cookie
	for _, cookie := range c.cookies {
		cookies = append(cookies, cookie)
	}
	data, err := json.MarshalIndent(cookies, "", "  ")
	c.cookieMu.RUnlock()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not encode cookies: %v\n", err)
		return
	}

	if err := os.WriteFile(cookieFile, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not persist cookies to %s: %v\n", cookieFile, err)
	}
}

// cleanupExpiredCookies removes expired cookies from the storage
func (c *HTTPClient) cleanupExpiredCookies() {
	c.cookieMu.Lock()
	defer c.cookieMu.Unlock()
	now := time.Now()
	for key, cookie := range c.cookies {
		if !cookie.Expires.IsZero() && now.After(cookie.Expires) {
			delete(c.cookies, key)
		}
	}
}

// addCookie adds a cookie to the storage with proper validation.
func (c *HTTPClient) addCookie(httpCookie *http.Cookie, host string) {
	c.cookieMu.Lock()
	addCookieLocked(c, httpCookie, host)
	c.cookieMu.Unlock()
	c.saveCookiesToFile()
}

// addCookieLocked adds a validated cookie to the storage; the caller must
// hold c.cookieMu.
func addCookieLocked(c *HTTPClient, httpCookie *http.Cookie, host string) {
	// Convert http.SameSite enum to our string representation.
	sameSiteStr := ""
	switch httpCookie.SameSite {
	case http.SameSiteLaxMode:
		sameSiteStr = "Lax"
	case http.SameSiteStrictMode:
		sameSiteStr = "Strict"
	case http.SameSiteNoneMode:
		sameSiteStr = "None"
	}

	// Create our internal Cookie representation
	cookie := &Cookie{
		Name:     httpCookie.Name,
		Value:    httpCookie.Value,
		Domain:   httpCookie.Domain,
		Path:     httpCookie.Path,
		Secure:   httpCookie.Secure,
		HttpOnly: httpCookie.HttpOnly,
		SameSite: sameSiteStr,
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

	// Reject domains outside the setting host's control (host-only fallback).
	if !validCookieDomain(host, cookie.Domain) {
		cookie.Domain = host
	}

	// If no path was specified, set it to the directory of the request path
	if cookie.Path == "" {
		cookie.Path = "/"
	}

	// Store the cookie in the map
	key := cookie.Domain + "_" + cookie.Name
	c.cookies[key] = cookie
}
