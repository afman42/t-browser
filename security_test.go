package main

import (
	"crypto/tls"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// saveHSTSStore tests
// ---------------------------------------------------------------------------

func TestSaveHSTSStoreNilStore(t *testing.T) {
	client := NewHTTPClient(nil)
	client.hstsStore = nil
	// Should not panic when hstsStore is nil.
	client.saveHSTSStore()
}

func TestSaveHSTSStoreNilConfig(t *testing.T) {
	client := &HTTPClient{
		cookies:   make(map[string]*Cookie),
		hstsStore: NewHSTSStore(),
		config:    nil,
	}
	// Should not panic when config is nil.
	client.saveHSTSStore()
}

func TestSaveHSTSStorePersists(t *testing.T) {
	dir := t.TempDir()
	store := NewHSTSStore()
	store.RecordPolicy("example.com", "max-age=3600")

	savePath := filepath.Join(dir, "hsts.json")
	if err := SaveHSTS(store, savePath); err != nil {
		t.Fatalf("SaveHSTS failed: %v", err)
	}

	loaded, err := LoadHSTS(savePath)
	if err != nil {
		t.Fatalf("LoadHSTS failed: %v", err)
	}
	if !loaded.ShouldUpgrade("example.com") {
		t.Error("expected loaded store to upgrade example.com")
	}
}

func TestSaveHSTSStoreViaClient(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.EnableHSTS = true
	client := NewHTTPClient(&cfg)

	// Record a policy.
	client.hstsStore.RecordPolicy("example.com", "max-age=3600")

	// saveHSTSStore is a no-op guard against nil store/config (tested above).
	// The actual save path depends on the real config dir, so we just verify
	// the method doesn't panic when called with a valid store.
	client.saveHSTSStore()
}

// ---------------------------------------------------------------------------
// Certificate pinning tests
// ---------------------------------------------------------------------------

func TestParsePinnedKeysValid(t *testing.T) {
	// A valid 32-byte key (all zeros is technically valid, just weak).
	raw := make([]byte, 32)
	encoded := base64.StdEncoding.EncodeToString(raw)

	pins, err := parsePinnedKeys([]string{encoded})
	if err != nil {
		t.Fatalf("parsePinnedKeys failed: %v", err)
	}
	if len(pins) != 1 {
		t.Fatalf("expected 1 pin, got %d", len(pins))
	}
	if pins[0].Fingerprint != [32]byte{} {
		t.Error("fingerprint mismatch")
	}
}

func TestParsePinnedKeysMultiple(t *testing.T) {
	raw1 := make([]byte, 32)
	raw1[0] = 1
	raw2 := make([]byte, 32)
	raw2[0] = 2

	pins, err := parsePinnedKeys([]string{
		base64.StdEncoding.EncodeToString(raw1),
		base64.StdEncoding.EncodeToString(raw2),
	})
	if err != nil {
		t.Fatalf("parsePinnedKeys failed: %v", err)
	}
	if len(pins) != 2 {
		t.Fatalf("expected 2 pins, got %d", len(pins))
	}
}

func TestParsePinnedKeysInvalidBase64(t *testing.T) {
	_, err := parsePinnedKeys([]string{"not-valid-base64!!!"})
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestParsePinnedKeysWrongLength(t *testing.T) {
	// 16 bytes instead of 32
	raw := make([]byte, 16)
	encoded := base64.StdEncoding.EncodeToString(raw)

	_, err := parsePinnedKeys([]string{encoded})
	if err == nil {
		t.Fatal("expected error for wrong key length")
	}
	if !strings.Contains(err.Error(), "expected 32 bytes") {
		t.Errorf("wrong error message: %v", err)
	}
}

func TestParsePinnedKeysEmpty(t *testing.T) {
	pins, err := parsePinnedKeys(nil)
	if err != nil {
		t.Fatalf("parsePinnedKeys(nil) failed: %v", err)
	}
	if len(pins) != 0 {
		t.Errorf("expected 0 pins, got %d", len(pins))
	}

	pins, err = parsePinnedKeys([]string{})
	if err != nil {
		t.Fatalf("parsePinnedKeys([]) failed: %v", err)
	}
	if len(pins) != 0 {
		t.Errorf("expected 0 pins, got %d", len(pins))
	}
}

func TestSetupTLSConfigPinningDisabled(t *testing.T) {
	cfg := setupTLSConfig(nil, false)
	if cfg == nil {
		t.Fatal("setupTLSConfig returned nil")
	}
	if cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false when pinning is disabled")
	}
	if cfg.VerifyConnection != nil {
		t.Error("VerifyConnection should be nil when pinning is disabled")
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS12)
	}
}

func TestSetupTLSConfigPinningEnabled(t *testing.T) {
	raw := make([]byte, 32)
	pins, _ := parsePinnedKeys([]string{base64.StdEncoding.EncodeToString(raw)})

	cfg := setupTLSConfig(pins, true)
	if cfg == nil {
		t.Fatal("setupTLSConfig returned nil")
	}
	if !cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true when pinning is enabled")
	}
	if cfg.VerifyConnection == nil {
		t.Error("VerifyConnection should be set when pinning is enabled")
	}
}

// ---------------------------------------------------------------------------
// HSTS store tests
// ---------------------------------------------------------------------------

func TestHSTSStoreRecordAndUpgrade(t *testing.T) {
	store := NewHSTSStore()

	// Initially no host should be upgraded.
	if store.ShouldUpgrade("example.com") {
		t.Error("expected no upgrade before any policy recorded")
	}

	// Record a policy for example.com.
	store.RecordPolicy("example.com", "max-age=3600")

	if !store.ShouldUpgrade("example.com") {
		t.Error("expected upgrade for example.com after recording policy")
	}

	// A different host should still not be upgraded.
	if store.ShouldUpgrade("other.com") {
		t.Error("expected no upgrade for other.com")
	}
}

func TestHSTSStoreIncludeSubDomains(t *testing.T) {
	store := NewHSTSStore()

	// Record a policy with includeSubDomains for example.com.
	store.RecordPolicy("example.com", "max-age=3600; includeSubDomains")

	// Subdomains should now be upgraded.
	if !store.ShouldUpgrade("sub.example.com") {
		t.Error("expected upgrade for sub.example.com with includeSubDomains")
	}
	if !store.ShouldUpgrade("deep.sub.example.com") {
		t.Error("expected upgrade for deep.sub.example.com with includeSubDomains")
	}

	// Unrelated domain should not.
	if store.ShouldUpgrade("other.com") {
		t.Error("expected no upgrade for other.com")
	}
}

func TestHSTSStoreMaxAgeZeroDeletes(t *testing.T) {
	store := NewHSTSStore()

	store.RecordPolicy("example.com", "max-age=3600")
	if !store.ShouldUpgrade("example.com") {
		t.Error("expected upgrade after recording policy")
	}

	// max-age=0 should remove the policy.
	store.RecordPolicy("example.com", "max-age=0")
	if store.ShouldUpgrade("example.com") {
		t.Error("expected no upgrade after max-age=0")
	}
}

func TestHSTSStoreExpiry(t *testing.T) {
	store := NewHSTSStore()

	// Record a very short-lived policy.
	store.RecordPolicy("example.com", "max-age=1")
	if !store.ShouldUpgrade("example.com") {
		t.Error("expected upgrade immediately after recording")
	}

	// Wait for it to expire. 2s to avoid flakiness on slow CI.
	time.Sleep(2 * time.Second)

	if store.ShouldUpgrade("example.com") {
		t.Error("expected no upgrade after expiry")
	}
}

func TestHSTSStoreCleanup(t *testing.T) {
	store := NewHSTSStore()

	store.RecordPolicy("short.com", "max-age=1")
	store.RecordPolicy("long.com", "max-age=3600")

	// 2s to avoid flakiness on slow CI.
	time.Sleep(2 * time.Second)

	store.Cleanup()

	if store.ShouldUpgrade("short.com") {
		t.Error("short.com should have been cleaned up")
	}
	if !store.ShouldUpgrade("long.com") {
		t.Error("long.com should still be valid")
	}
}

func TestHSTSStoreCaseInsensitive(t *testing.T) {
	store := NewHSTSStore()

	store.RecordPolicy("Example.COM", "max-age=3600")

	if !store.ShouldUpgrade("example.com") {
		t.Error("should match case-insensitively")
	}
	if !store.ShouldUpgrade("EXAMPLE.COM") {
		t.Error("should match case-insensitively")
	}
}

func TestHSTSPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hsts_test.json")

	// Create a store, record some policies, save it.
	store := NewHSTSStore()
	store.RecordPolicy("example.com", "max-age=3600")
	store.RecordPolicy("test.org", "max-age=7200; includeSubDomains")

	if err := SaveHSTS(store, path); err != nil {
		t.Fatalf("SaveHSTS failed: %v", err)
	}

	// Load it back.
	loaded, err := LoadHSTS(path)
	if err != nil {
		t.Fatalf("LoadHSTS failed: %v", err)
	}

	if !loaded.ShouldUpgrade("example.com") {
		t.Error("example.com should be upgraded after load")
	}
	if !loaded.ShouldUpgrade("sub.test.org") {
		t.Error("sub.test.org should be upgraded via includeSubDomains after load")
	}
}

func TestHSTSPersistenceCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupted.json")
	if err := os.WriteFile(path, []byte("{not valid json}"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadHSTS(path)
	if err == nil {
		t.Fatal("expected error loading corrupted file")
	}
}

func TestHSTSPersistenceNonExistentFile(t *testing.T) {
	store, err := LoadHSTS("/nonexistent/hsts_policies.json")
	if err != nil {
		t.Fatalf("expected nil store for nonexistent file, got error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store for nonexistent file")
	}
}

func TestGetHSTSFilePath(t *testing.T) {
	dir := t.TempDir()
	path := GetHSTSFilePath(dir)

	if path == "" {
		t.Fatal("GetHSTSFilePath returned empty")
	}

	// Directory should have been created.
	hstsDir := filepath.Join(dir, "hsts")
	if _, err := os.Stat(hstsDir); os.IsNotExist(err) {
		t.Error("hsts directory was not created")
	}

	base := filepath.Base(path)
	if base != "policies.json" {
		t.Errorf("expected 'policies.json', got %q", base)
	}
}

// ---------------------------------------------------------------------------
// HSTS header parsing edge cases
// ---------------------------------------------------------------------------

func TestHSTSStoreParsingEdgeCases(t *testing.T) {
	store := NewHSTSStore()

	tests := []struct {
		header string
		host   string
		want   bool
	}{
		{"max-age=3600", "example.com", true},
		{"max-age=3600; includeSubDomains", "example.com", true},
		{"max-age=3600; preload", "example.com", true},
		{"max-age=3600; includeSubDomains; preload", "example.com", true},
		{" max-age = 3600 ; includeSubDomains ", "example.com", true},
	}

	for _, tc := range tests {
		store.RecordPolicy(tc.host, tc.header)
		if got := store.ShouldUpgrade(tc.host); got != tc.want {
			t.Errorf("header %q: ShouldUpgrade(%q) = %v, want %v", tc.header, tc.host, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// hstsTransport tests
// ---------------------------------------------------------------------------

func TestHSTSTransportUpgradesHTTP(t *testing.T) {
	store := NewHSTSStore()
	store.RecordPolicy("example.com", "max-age=3600")

	var capturedScheme string
	transport := &hstsTransport{
		inner: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedScheme = req.URL.Scheme
			return &http.Response{StatusCode: 200, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
				Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
		store: store,
	}

	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	transport.RoundTrip(req)

	if capturedScheme != "https" {
		t.Errorf("expected scheme to be upgraded to https, got %q", capturedScheme)
	}
}

func TestHSTSTransportNoUpgradeForNonHSTS(t *testing.T) {
	store := NewHSTSStore()
	// No policy recorded.

	var capturedScheme string
	transport := &hstsTransport{
		inner: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedScheme = req.URL.Scheme
			return &http.Response{StatusCode: 200, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
				Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
		store: store,
	}

	req, _ := http.NewRequest("GET", "http://other.com/", nil)
	transport.RoundTrip(req)

	if capturedScheme != "http" {
		t.Errorf("expected scheme to remain http, got %q", capturedScheme)
	}
}

func TestHSTSTransportDoesNotDowngradeHTTPS(t *testing.T) {
	store := NewHSTSStore()
	store.RecordPolicy("example.com", "max-age=3600")

	var capturedScheme string
	transport := &hstsTransport{
		inner: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedScheme = req.URL.Scheme
			return &http.Response{StatusCode: 200, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
				Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
		store: store,
	}

	req, _ := http.NewRequest("GET", "https://example.com/", nil)
	transport.RoundTrip(req)

	if capturedScheme != "https" {
		t.Errorf("expected scheme to remain https, got %q", capturedScheme)
	}
}
