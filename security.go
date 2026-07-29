package main

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Certificate Pinning
// ---------------------------------------------------------------------------

// CertPin holds a SHA-256 fingerprint of a DER-encoded public key.
type CertPin struct {
	Fingerprint [32]byte
}

// parsePinnedKeys parses a slice of base64-encoded SHA-256 fingerprints into
// CertPin values.  Each input must be exactly 44 base64 characters (32 bytes).
func parsePinnedKeys(keys []string) ([]CertPin, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	pins := make([]CertPin, 0, len(keys))
	for i, s := range keys {
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("pinned key %d: invalid base64: %w", i, err)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("pinned key %d: expected 32 bytes, got %d", i, len(decoded))
		}
		var pin CertPin
		copy(pin.Fingerprint[:], decoded)
		pins = append(pins, pin)
	}
	return pins, nil
}

// setupTLSConfig returns a *tls.Config that performs public-key pinning via
// the VerifyConnection callback.  When enablePinning is false the default
// config is returned (no pinning applied).
func setupTLSConfig(pins []CertPin, enablePinning bool) *tls.Config {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if !enablePinning || len(pins) == 0 {
		return cfg
	}

	// We must disable the default PKI verification because we supply our own.
	cfg.InsecureSkipVerify = true

	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return fmt.Errorf("certificate pinning: no peer certificates provided")
		}

		// Extract and hash the leaf certificate's public key.
		cert := cs.PeerCertificates[0]
		pubKeyBytes, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
		if err != nil {
			return fmt.Errorf("certificate pinning: failed to marshal public key: %w", err)
		}
		hash := sha256.Sum256(pubKeyBytes)

		// Check against any pinned key.
		for _, pin := range pins {
			if hash == pin.Fingerprint {
				// Also run standard verification (expiry, hostname, chain).
				// We build a fresh verifier using the system roots.
				opts := x509.VerifyOptions{
					DNSName: cs.ServerName,
				}
				if _, err := cert.Verify(opts); err != nil {
					return fmt.Errorf("certificate pinning: standard verification failed: %w", err)
				}
				return nil
			}
		}

		return fmt.Errorf("certificate pinning: no pinned key matches the server's public key")
	}

	return cfg
}

// ---------------------------------------------------------------------------
// HSTS (HTTP Strict Transport Security)
// ---------------------------------------------------------------------------

// HSTSPolicy represents a cached HSTS policy for a single domain.
type HSTSPolicy struct {
	Domain            string `json:"domain"`
	MaxAge            int64  `json:"max_age"` // seconds
	IncludeSubDomains bool   `json:"include_subdomains"`
	ExpiresUnix       int64  `json:"expires_unix"` // Unix timestamp of expiry
}

// expired returns true when the policy has expired.
func (p *HSTSPolicy) expired() bool {
	return p.ExpiresUnix > 0 && time.Now().Unix() > p.ExpiresUnix
}

// HSTSStore is a concurrency-safe, optionally-persistent store of HSTS policies.
type HSTSStore struct {
	mu       sync.RWMutex
	policies map[string]*HSTSPolicy // keyed by lower-cased hostname
	dirty    bool
}

// NewHSTSStore creates an empty HSTS store.
func NewHSTSStore() *HSTSStore {
	return &HSTSStore{
		policies: make(map[string]*HSTSPolicy),
	}
}

// RecordPolicy parses a Strict-Transport-Security header value and stores
// (or removes) the policy for the given host.
func (s *HSTSStore) RecordPolicy(host, headerValue string) {
	host = strings.ToLower(host)

	maxAge := int64(0)
	includeSub := false

	// Parse directive list (semicolons per RFC 6797).
	for _, part := range strings.Split(headerValue, ";") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "max-age") && strings.Contains(part, "="):
			// Handle "max-age=3600" and "max-age = 3600" (with spaces around =).
			_, valStr, _ := strings.Cut(part, "=")
			valStr = strings.TrimSpace(valStr)
			if val, err := strconv.ParseInt(valStr, 10, 64); err == nil {
				maxAge = val
			}
		case part == "includeSubDomains":
			includeSub = true
		case part == "preload":
			// Preload is a hint for browser vendors; we note it but don't
			// need special handling.
		}
	}

	// max-age=0 means the policy should be removed immediately.
	if maxAge <= 0 {
		s.mu.Lock()
		delete(s.policies, host)
		s.dirty = true
		s.mu.Unlock()
		return
	}

	policy := &HSTSPolicy{
		Domain:            host,
		MaxAge:            maxAge,
		IncludeSubDomains: includeSub,
		ExpiresUnix:       time.Now().Unix() + maxAge,
	}

	s.mu.Lock()
	s.policies[host] = policy
	s.dirty = true
	s.mu.Unlock()
}

// ShouldUpgrade returns true when requests to the given host should be
// upgraded from HTTP to HTTPS.  It also checks includeSubDomains.
func (s *HSTSStore) ShouldUpgrade(host string) bool {
	host = strings.ToLower(host)

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Exact match.
	if p, ok := s.policies[host]; ok && !p.expired() {
		return true
	}

	// Check if any parent domain with includeSubDomains covers this host.
	parts := strings.Split(host, ".")
	for i := 1; i < len(parts)-1; i++ {
		parent := strings.Join(parts[i:], ".")
		if p, ok := s.policies[parent]; ok && !p.expired() && p.IncludeSubDomains {
			return true
		}
	}

	return false
}

// Cleanup removes expired policies from the store.
func (s *HSTSStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for host, p := range s.policies {
		if p.expired() {
			delete(s.policies, host)
			s.dirty = true
		}
	}
}

// LoadHSTS loads an HSTS store from a JSON file on disk.
func LoadHSTS(path string) (*HSTSStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewHSTSStore(), nil
		}
		return nil, err
	}

	var policies []*HSTSPolicy
	if err := json.Unmarshal(data, &policies); err != nil {
		return nil, err
	}

	store := NewHSTSStore()
	now := time.Now().Unix()
	for _, p := range policies {
		if p.ExpiresUnix > 0 && now > p.ExpiresUnix {
			continue // skip expired
		}
		store.policies[p.Domain] = p
	}
	return store, nil
}

// SaveHSTS persists the current HSTS policies to a JSON file.
func SaveHSTS(store *HSTSStore, path string) error {
	store.mu.RLock()
	policies := make([]*HSTSPolicy, 0, len(store.policies))
	for _, p := range store.policies {
		if !p.expired() {
			policies = append(policies, p)
		}
	}
	store.mu.RUnlock()

	data, err := json.MarshalIndent(policies, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// hstsTransport is an http.RoundTripper that upgrades HTTP requests to HTTPS
// when the target host has an active HSTS policy.
type hstsTransport struct {
	inner http.RoundTripper
	store *HSTSStore
}

func (t *hstsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == "http" && t.store.ShouldUpgrade(req.URL.Hostname()) {
		// Clone the request with an upgraded URL.
		upgraded := req.Clone(req.Context())
		upgraded.URL.Scheme = "https"
		upgraded.URL.Host = req.URL.Host // keep port if present
		// The Host header should remain the original value.
		return t.inner.RoundTrip(upgraded)
	}
	return t.inner.RoundTrip(req)
}

// saveHSTSStore persists the HSTS store to disk, if the store and config are
// both available. Errors are non-fatal (logged to stderr).
func (c *HTTPClient) saveHSTSStore() {
	if c.hstsStore == nil || c.config == nil {
		return
	}
	configDir := GetConfigDir()
	if err := SaveHSTS(c.hstsStore, GetHSTSFilePath(configDir)); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to persist HSTS policies: %v\n", err)
	}
}

// GetHSTSFilePath returns the path to the HSTS store file.
func GetHSTSFilePath(configDir string) string {
	if configDir == "" {
		configDir = GetConfigDir()
	}
	hstsDir := filepath.Join(configDir, "hsts")
	if err := os.MkdirAll(hstsDir, 0755); err != nil {
		return filepath.Join(configDir, "hsts_policies.json")
	}
	return filepath.Join(hstsDir, "policies.json")
}

// ---------------------------------------------------------------------------
// Pin Current Site Key
// ---------------------------------------------------------------------------

func PinCurrentSiteKey(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("pin: invalid URL: %w", err)
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		port = "443"
	}

	// Dial a TLS connection to extract the certificate.
	tlsConf := &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		ServerName:         host,
	}
	conn, err := tls.Dial("tcp", host+":"+port, tlsConf)
	if err != nil {
		return "", fmt.Errorf("pin: TLS dial failed: %w", err)
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return "", fmt.Errorf("pin: no peer certificates")
	}

	cert := state.PeerCertificates[0]
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return "", fmt.Errorf("pin: failed to marshal public key: %w", err)
	}

	hash := sha256.Sum256(pubKeyBytes)
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}
