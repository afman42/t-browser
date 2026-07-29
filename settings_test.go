package main

import (
	"testing"
)

func TestGetSettingCategories(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}

	categories := b.getSettingCategories()
	if len(categories) != 6 {
		t.Errorf("expected 6 categories, got %d", len(categories))
	}

	// Verify all expected IDs are present
	expectedIDs := map[string]bool{"browser": false, "network": false, "content": false, "ui": false, "privacy": false, "advanced": false}
	for _, cat := range categories {
		if _, ok := expectedIDs[cat.ID]; ok {
			expectedIDs[cat.ID] = true
		} else {
			t.Errorf("unexpected category ID: %s", cat.ID)
		}
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("missing category ID: %s", id)
		}
	}
}

func TestGetSettingsForCategoryBrowser(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.UserAgent = "test-ua/1.0"
	cfg.RequestTimeout = 15
	b := &Browser{config: &cfg}

	settings := b.getSettingsForCategory("browser")
	if len(settings) != 2 {
		t.Fatalf("expected 2 browser settings, got %d", len(settings))
	}

	if settings[0].ID != "user_agent" {
		t.Errorf("expected first setting to be 'user_agent', got %q", settings[0].ID)
	}
	if settings[0].Value != "test-ua/1.0" {
		t.Errorf("expected user_agent value 'test-ua/1.0', got %v", settings[0].Value)
	}
	if settings[1].ID != "request_timeout" {
		t.Errorf("expected second setting to be 'request_timeout', got %q", settings[1].ID)
	}
	if settings[1].Value != 15 {
		t.Errorf("expected request_timeout 15, got %v", settings[1].Value)
	}
}

func TestGetSettingsForCategoryUI(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.Theme = "light"
	cfg.ShowImages = false
	cfg.WordWrap = false
	b := &Browser{config: &cfg}

	settings := b.getSettingsForCategory("ui")
	if len(settings) != 4 {
		t.Fatalf("expected 4 UI settings, got %d", len(settings))
	}

	if settings[0].Value != "light" {
		t.Errorf("expected theme 'light', got %v", settings[0].Value)
	}
	if settings[1].Value != false {
		t.Errorf("expected show_images false, got %v", settings[1].Value)
	}
	if settings[2].Value != false {
		t.Errorf("expected word_wrap false, got %v", settings[2].Value)
	}
	if settings[3].ID != "search_engine" {
		t.Errorf("expected search_engine setting, got %s", settings[3].ID)
	}
}

func TestGetSettingsForCategoryContent(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.MaxPageSize = 10 * 1024 * 1024 // 10 MB
	cfg.MaxImageSize = 2 * 1024 * 1024 // 2 MB
	cfg.EnableImages = true
	b := &Browser{config: &cfg}

	settings := b.getSettingsForCategory("content")
	if len(settings) != 5 {
		t.Fatalf("expected 5 content settings, got %d", len(settings))
	}

	// MaxPageSize should be expressed in MB
	if v, ok := settings[0].Value.(int64); !ok || v != 10 {
		t.Errorf("expected max_page_size (MB) as 10 (int64), got %T=%v", settings[0].Value, settings[0].Value)
	}
	if v, ok := settings[1].Value.(int64); !ok || v != 2 {
		t.Errorf("expected max_image_size (MB) as 2 (int64), got %T=%v", settings[1].Value, settings[1].Value)
	}
	if v, ok := settings[2].Value.(bool); !ok || v != true {
		t.Errorf("expected enable_images true, got %T=%v", settings[2].Value, settings[2].Value)
	}
}

func TestGetSettingsForCategoryInvalid(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}

	settings := b.getSettingsForCategory("nonexistent")
	if len(settings) != 0 {
		t.Errorf("expected 0 settings for invalid category, got %d", len(settings))
	}
}

func TestGetSettingsForCategoryPrivacy(t *testing.T) {
	cfg := GetDefaultConfig()
	cfg.EnableCookies = false
	b := &Browser{config: &cfg}

	settings := b.getSettingsForCategory("privacy")
	if len(settings) != 4 {
		t.Fatalf("expected 4 privacy settings, got %d", len(settings))
	}

	if settings[0].ID != "enable_cookies" {
		t.Errorf("expected 'enable_cookies', got %q", settings[0].ID)
	}
	if settings[0].Value != false {
		t.Errorf("expected enable_cookies false, got %v", settings[0].Value)
	}
}

func TestUpdateSettingValueStrings(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}

	b.updateSettingValue("user_agent", "custom-agent/2.0")
	if cfg.UserAgent != "custom-agent/2.0" {
		t.Errorf("UserAgent = %q, want %q", cfg.UserAgent, "custom-agent/2.0")
	}

	b.updateSettingValue("proxy", "http://proxy.example:8080")
	if cfg.Proxy != "http://proxy.example:8080" {
		t.Errorf("Proxy = %q, want %q", cfg.Proxy, "http://proxy.example:8080")
	}

	b.updateSettingValue("theme", "light")
	if cfg.Theme != "light" {
		t.Errorf("Theme = %q, want %q", cfg.Theme, "light")
	}

	b.updateSettingValue("cookie_file", "/tmp/cookies.json")
	if cfg.CookieFile != "/tmp/cookies.json" {
		t.Errorf("CookieFile = %q, want %q", cfg.CookieFile, "/tmp/cookies.json")
	}

	b.updateSettingValue("session_file", "/tmp/session.json")
	if cfg.SessionFile != "/tmp/session.json" {
		t.Errorf("SessionFile = %q, want %q", cfg.SessionFile, "/tmp/session.json")
	}
}

func TestUpdateSettingValueInts(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}

	b.updateSettingValue("request_timeout", 60)
	if cfg.RequestTimeout != 60 {
		t.Errorf("RequestTimeout = %d, want %d", cfg.RequestTimeout, 60)
	}

	b.updateSettingValue("max_redirects", 5)
	if cfg.MaxRedirects != 5 {
		t.Errorf("MaxRedirects = %d, want %d", cfg.MaxRedirects, 5)
	}

	// MaxPageSize is stored in bytes, input is MB
	b.updateSettingValue("max_page_size", 20)
	if cfg.MaxPageSize != int64(20)*1024*1024 {
		t.Errorf("MaxPageSize = %d, want %d", cfg.MaxPageSize, int64(20)*1024*1024)
	}

	// MaxImageSize is stored in bytes, input is MB
	b.updateSettingValue("max_image_size", 3)
	if cfg.MaxImageSize != int64(3)*1024*1024 {
		t.Errorf("MaxImageSize = %d, want %d", cfg.MaxImageSize, int64(3)*1024*1024)
	}
}

func TestUpdateSettingValueBools(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}

	b.updateSettingValue("enable_images", false)
	if cfg.EnableImages != false {
		t.Errorf("EnableImages = %v, want %v", cfg.EnableImages, false)
	}

	b.updateSettingValue("show_images", false)
	if cfg.ShowImages != false {
		t.Errorf("ShowImages = %v, want %v", cfg.ShowImages, false)
	}

	b.updateSettingValue("word_wrap", false)
	if cfg.WordWrap != false {
		t.Errorf("WordWrap = %v, want %v", cfg.WordWrap, false)
	}

	b.updateSettingValue("enable_cookies", false)
	if cfg.EnableCookies != false {
		t.Errorf("EnableCookies = %v, want %v", cfg.EnableCookies, false)
	}

	b.updateSettingValue("cookie_auto_save", false)
	if cfg.CookieAutoSave != false {
		t.Errorf("CookieAutoSave = %v, want %v", cfg.CookieAutoSave, false)
	}

	b.updateSettingValue("session_auto_save", false)
	if cfg.SessionAutoSave != false {
		t.Errorf("SessionAutoSave = %v, want %v", cfg.SessionAutoSave, false)
	}
}

func TestUpdateSettingValueTypeMismatch(t *testing.T) {
	cfg := GetDefaultConfig()
	b := &Browser{config: &cfg}

	original := cfg.UserAgent
	// Passing int where string is expected should not change the value
	b.updateSettingValue("user_agent", 12345)
	if cfg.UserAgent != original {
		t.Errorf("expected UserAgent unchanged after type mismatch, got %q", cfg.UserAgent)
	}
}
