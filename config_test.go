package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestGetDefaultConfig(t *testing.T) {
	cfg := GetDefaultConfig()

	if cfg.UserAgent == "" {
		t.Error("default UserAgent should not be empty")
	}
	if cfg.CookieFile != "cookies.json" {
		t.Errorf("default CookieFile = %q, want %q", cfg.CookieFile, "cookies.json")
	}
	if cfg.Theme != "dark" {
		t.Errorf("default Theme = %q, want %q", cfg.Theme, "dark")
	}
	if cfg.MaxPageSize != 50*1024*1024 {
		t.Errorf("default MaxPageSize = %d, want %d", cfg.MaxPageSize, 50*1024*1024)
	}
	if cfg.EnableImages != true {
		t.Error("default EnableImages should be true")
	}
	if cfg.WordWrap != true {
		t.Error("default WordWrap should be true")
	}
}

func TestGetDefaultConfigStable(t *testing.T) {
	// Ensure calling twice returns the same values (no hidden mutation)
	a := GetDefaultConfig()
	b := GetDefaultConfig()
	if a.UserAgent != b.UserAgent {
		t.Error("GetDefaultConfig is not idempotent")
	}
}

func TestApplyConfigToViper(t *testing.T) {
	// applyConfigToViper uses Viper's global singleton, so reset between tests
	viper.Reset()
	cfg := GetDefaultConfig()
	cfg.UserAgent = "test-agent/1.0"
	cfg.Theme = "light"
	cfg.MaxPageSize = 1_000_000
	cfg.EnableImages = false

	applyConfigToViper(cfg)

	if got := viper.GetString("user_agent"); got != "test-agent/1.0" {
		t.Errorf("user_agent = %q, want %q", got, "test-agent/1.0")
	}
	if got := viper.GetString("theme"); got != "light" {
		t.Errorf("theme = %q, want %q", got, "light")
	}
	if got := viper.GetInt64("max_page_size"); got != 1_000_000 {
		t.Errorf("max_page_size = %d, want %d", got, 1_000_000)
	}
	if got := viper.GetBool("enable_images"); got != false {
		t.Errorf("enable_images = %v, want %v", got, false)
	}
}

func TestApplyConfigToViperRoundTrip(t *testing.T) {
	// Write then read back via unmarshal
	viper.Reset()
	cfg := GetDefaultConfig()
	cfg.UserAgent = "roundtrip-test/1.0"
	applyConfigToViper(cfg)

	var decoded Config
	if err := viper.Unmarshal(&decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded.UserAgent != "roundtrip-test/1.0" {
		t.Errorf("round-trip UserAgent = %q, want %q", decoded.UserAgent, "roundtrip-test/1.0")
	}
}

func TestGetTimestamp(t *testing.T) {
	ts := GetTimestamp()
	// Expect format YYYY-MM-DD_HH-MM-SS
	if len(ts) != 19 {
		t.Errorf("timestamp length = %d, want 19 (got %q)", len(ts), ts)
	}
	// Should parse as our format
	_, err := time.Parse("2006-01-02_15-04-05", ts)
	if err != nil {
		t.Errorf("timestamp %q doesn't match format: %v", ts, err)
	}
}

func TestGetCookieFilePath(t *testing.T) {
	d := t.TempDir()
	path := GetCookieFilePath(d)

	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
	if !filepath.HasPrefix(path, d) {
		t.Errorf("path %q should be under dir %q", path, d)
	}
	// Should be inside a cookies/ subdirectory
	if filepath.Base(filepath.Dir(path)) != "cookies" {
		t.Errorf("expected cookies/ subdirectory, got %q", filepath.Dir(path))
	}
	// Filename should start with cookies_ and end with .json
	base := filepath.Base(path)
	if len(base) < 20 || base[:8] != "cookies_" || base[len(base)-5:] != ".json" {
		t.Errorf("filename %q doesn't match cookies_*.json pattern", base)
	}
	// Directory should have been created
	if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
		t.Error("cookies directory was not created")
	}
}

func TestGetSessionFilePath(t *testing.T) {
	d := t.TempDir()
	path := GetSessionFilePath(d)

	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
	if filepath.Base(filepath.Dir(path)) != "sessions" {
		t.Errorf("expected sessions/ subdirectory, got %q", filepath.Dir(path))
	}
	base := filepath.Base(path)
	if len(base) < 20 || base[:8] != "session_" || base[len(base)-5:] != ".json" {
		t.Errorf("filename %q doesn't match session_*.json pattern", base)
	}
}

func TestGetLatestCookieFileEmpty(t *testing.T) {
	d := t.TempDir()
	// No cookies directory at all
	if got := GetLatestCookieFile(d); got != "" {
		t.Errorf("expected empty for missing dir, got %q", got)
	}
}

func TestGetLatestCookieFile(t *testing.T) {
	d := t.TempDir()
	cookiesDir := filepath.Join(d, "cookies")
	os.MkdirAll(cookiesDir, 0755)

	// Create two cookie files at different timestamps
	os.WriteFile(filepath.Join(cookiesDir, "cookies_2024-01-01_00-00-00.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(cookiesDir, "cookies_2024-06-15_12-30-00.json"), []byte("{}"), 0644)

	got := GetLatestCookieFile(d)
	wantSuffix := "cookies_2024-06-15_12-30-00.json"
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("expected latest file ending in %q, got %q", wantSuffix, got)
	}
}

func TestGetLatestCookieFileIgnoresNonMatching(t *testing.T) {
	d := t.TempDir()
	cookiesDir := filepath.Join(d, "cookies")
	os.MkdirAll(cookiesDir, 0755)
	// Create a file that doesn't match the pattern
	os.WriteFile(filepath.Join(cookiesDir, "random.txt"), []byte("{}"), 0644)

	if got := GetLatestCookieFile(d); got != "" {
		t.Errorf("expected empty when no matching files, got %q", got)
	}
}

func TestGetLatestSessionFileEmpty(t *testing.T) {
	d := t.TempDir()
	if got := GetLatestSessionFile(d); got != "" {
		t.Errorf("expected empty for missing dir, got %q", got)
	}
}

func TestGetLatestSessionFile(t *testing.T) {
	d := t.TempDir()
	sessionsDir := filepath.Join(d, "sessions")
	os.MkdirAll(sessionsDir, 0755)

	os.WriteFile(filepath.Join(sessionsDir, "session_2024-03-10_08-15-00.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(sessionsDir, "session_2024-03-10_09-00-00.json"), []byte("{}"), 0644)

	got := GetLatestSessionFile(d)
	wantSuffix := "session_2024-03-10_09-00-00.json"
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("expected latest file ending in %q, got %q", wantSuffix, got)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// When no config file exists, LoadConfig should return zero-ish values
	// that the LoadConfig function fills in with defaults
	viper.Reset()
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() returned error: %v", err)
	}
	// Empty string fields get filled from GetDefaultConfig in LoadConfig
	if config.UserAgent == "" {
		t.Error("UserAgent should not be empty after LoadConfig default fill")
	}
	if config.CookieFile == "" {
		t.Error("CookieFile should not be empty after LoadConfig default fill")
	}
	if config.SessionFile == "" {
		t.Error("SessionFile should not be empty after LoadConfig default fill")
	}
}

func TestConfigWriteToFile(t *testing.T) {
	d := t.TempDir()
	cfg := GetDefaultConfig()
	cfg.UserAgent = "write-test/1.0"

	err := cfg.WriteToFile(d)
	if err != nil {
		t.Fatalf("WriteToFile failed: %v", err)
	}

	configPath := filepath.Join(d, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("config file was not written to %q", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if len(data) == 0 {
		t.Error("config file is empty")
	}
}
