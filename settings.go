package main

// updateSettingValue updates a setting value temporarily
func (b *Browser) updateSettingValue(settingID string, value interface{}) {
	switch settingID {
	case "user_agent":
		if strVal, ok := value.(string); ok {
			b.config.UserAgent = strVal
		}
	case "request_timeout":
		if intVal, ok := value.(int); ok {
			b.config.RequestTimeout = intVal
		}
	case "proxy":
		if strVal, ok := value.(string); ok {
			b.config.Proxy = strVal
		}
	case "max_redirects":
		if intVal, ok := value.(int); ok {
			b.config.MaxRedirects = intVal
		}
	case "max_retries":
		if intVal, ok := value.(int); ok {
			b.config.MaxRetries = intVal
		}
	case "cache_ttl_seconds":
		if intVal, ok := value.(int); ok {
			b.config.CacheTTLSeconds = intVal
		}
	case "max_page_size":
		if intVal, ok := value.(int); ok {
			b.config.MaxPageSize = int64(intVal) * 1024 * 1024
		}
	case "max_image_size":
		if intVal, ok := value.(int); ok {
			b.config.MaxImageSize = int64(intVal) * 1024 * 1024
		}
	case "enable_images":
		if boolVal, ok := value.(bool); ok {
			b.config.EnableImages = boolVal
		}
	case "theme":
		if strVal, ok := value.(string); ok {
			b.config.Theme = strVal
		}
	case "show_images":
		if boolVal, ok := value.(bool); ok {
			b.config.ShowImages = boolVal
		}
	case "word_wrap":
		if boolVal, ok := value.(bool); ok {
			b.config.WordWrap = boolVal
		}
	case "items_per_page":
		if intVal, ok := value.(int); ok {
			b.config.ItemsPerPage = intVal
		}
	case "search_engine":
		if strVal, ok := value.(string); ok {
			b.config.SearchEngine = strVal
		}
	case "enable_cookies":
		if boolVal, ok := value.(bool); ok {
			b.config.EnableCookies = boolVal
		}
	case "enable_content_security":
		if boolVal, ok := value.(bool); ok {
			b.config.EnableContentSecurity = boolVal
		}
	case "block_external_resources":
		if boolVal, ok := value.(bool); ok {
			b.config.BlockExternalResources = boolVal
		}
	case "enforce_same_site":
		if boolVal, ok := value.(bool); ok {
			b.config.EnforceSameSite = boolVal
		}
	case "enable_pinning":
		if boolVal, ok := value.(bool); ok {
			b.config.EnablePinning = boolVal
		}
	case "enable_hsts":
		if boolVal, ok := value.(bool); ok {
			b.config.EnableHSTS = boolVal
		}
	case "strip_tracking_params":
		if boolVal, ok := value.(bool); ok {
			b.config.StripTrackingParams = boolVal
		}
	case "cookie_auto_save":
		if boolVal, ok := value.(bool); ok {
			b.config.CookieAutoSave = boolVal
		}
	case "session_auto_save":
		if boolVal, ok := value.(bool); ok {
			b.config.SessionAutoSave = boolVal
		}
	case "cookie_file":
		if strVal, ok := value.(string); ok {
			b.config.CookieFile = strVal
		}
	case "session_file":
		if strVal, ok := value.(string); ok {
			b.config.SessionFile = strVal
		}
	}

	// Push client-bound settings into the live HTTP client so password/proxy/
	// timeout/redirect/retry/pinning/HSTS changes take effect immediately.
	if b.client != nil {
		b.client.applyConfig()
	}
}

// saveSettings saves the current settings to the config file
func (b *Browser) saveSettings() {
	configDir := GetConfigDir()
	if err := b.config.WriteToFile(configDir); err != nil {
		// Could show an error message here if needed
	}

	b.ApplyTheme()
	b.closeSettingsModal()
}

// SettingCategory represents a category of settings
type SettingCategory struct {
	ID   string
	Name string
	Icon string
}

// Setting represents an individual setting
type Setting struct {
	ID          string
	Name        string
	Description string
	Value       interface{}
	Type        string // "bool", "string", "int", "password"
}

// getSettingCategories returns all available setting categories
func (b *Browser) getSettingCategories() []SettingCategory {
	return []SettingCategory{
		{ID: "browser", Name: "Browser Settings", Icon: "🌐"},
		{ID: "network", Name: "Network Settings", Icon: "🌐"},
		{ID: "content", Name: "Content Settings", Icon: "📄"},
		{ID: "ui", Name: "UI Settings", Icon: "🎨"},
		{ID: "privacy", Name: "Privacy Settings", Icon: "🛡️"},
		{ID: "advanced", Name: "Advanced Settings", Icon: "⚙️"},
	}
}

// getSettingsForCategory returns all settings for the specified category
func (b *Browser) getSettingsForCategory(categoryID string) []Setting {
	switch categoryID {
	case "browser":
		return []Setting{
			{ID: "user_agent", Name: "User Agent", Description: "Custom user agent string for HTTP requests", Value: b.config.UserAgent, Type: "string"},
			{ID: "request_timeout", Name: "Request Timeout", Description: "Time in seconds before request timeout", Value: b.config.RequestTimeout, Type: "int"},
		}
	case "network":
		return []Setting{
			{ID: "proxy", Name: "Proxy Server", Description: "Proxy server URL (e.g., http://proxy:port)", Value: b.config.Proxy, Type: "string"},
			{ID: "max_redirects", Name: "Max Redirects", Description: "Maximum number of HTTP redirects to follow", Value: b.config.MaxRedirects, Type: "int"},
			{ID: "max_retries", Name: "Max Retries", Description: "Retry attempts for transient errors (429/502/503/504)", Value: b.config.MaxRetries, Type: "int"},
			{ID: "cache_ttl_seconds", Name: "Cache TTL (s)", Description: "Client cache freshness; 0 = always revalidate", Value: b.config.CacheTTLSeconds, Type: "int"},
			{ID: "enable_pinning", Name: "Certificate Pinning", Description: "Verify server certificates against pinned public keys", Value: b.config.EnablePinning, Type: "bool"},
			{ID: "enable_hsts", Name: "HSTS", Description: "Enforce HTTPS for known HSTS hosts (Strict-Transport-Security)", Value: b.config.EnableHSTS, Type: "bool"},
		}
	case "content":
		return []Setting{
			{ID: "max_page_size", Name: "Max Page Size", Description: "Maximum size (in MB) for downloaded pages", Value: b.config.MaxPageSize / (1024 * 1024), Type: "int"},
			{ID: "max_image_size", Name: "Max Image Size", Description: "Maximum size (in MB) for images", Value: b.config.MaxImageSize / (1024 * 1024), Type: "int"},
			{ID: "enable_images", Name: "Enable Images", Description: "Enable or disable image loading", Value: b.config.EnableImages, Type: "bool"},
			{ID: "enable_content_security", Name: "Content Security", Description: "Strip scripts, iframes, event handlers from HTML", Value: b.config.EnableContentSecurity, Type: "bool"},
			{ID: "block_external_resources", Name: "Block External Resources", Description: "Block images/resources from external domains", Value: b.config.BlockExternalResources, Type: "bool"},
		}
	case "ui":
		return []Setting{
			{ID: "theme", Name: "Theme", Description: "Color theme for the interface", Value: b.config.Theme, Type: "string"},
			{ID: "show_images", Name: "Show Images", Description: "Display images in terminal", Value: b.config.ShowImages, Type: "bool"},
			{ID: "word_wrap", Name: "Word Wrap", Description: "Enable text word wrapping", Value: b.config.WordWrap, Type: "bool"},
			{ID: "items_per_page", Name: "Items Per Page", Description: "Page size for the links/images lists", Value: b.config.ItemsPerPage, Type: "int"},
			{ID: "search_engine", Name: "Search Engine", Description: "URL prefix for web search (e.g. https://duckduckgo.com/html?q=)", Value: b.config.SearchEngine, Type: "string"},
		}
	case "privacy":
		return []Setting{
			{ID: "enable_cookies", Name: "Enable Cookies", Description: "Enable or disable cookie storage", Value: b.config.EnableCookies, Type: "bool"},
			{ID: "cookie_auto_save", Name: "Auto Save Cookies", Description: "Enable automatic cookie saving", Value: b.config.CookieAutoSave, Type: "bool"},
			{ID: "enforce_same_site", Name: "Enforce SameSite", Description: "Enforce SameSite=Strict: block cookies on cross-domain requests", Value: b.config.EnforceSameSite, Type: "bool"},
			{ID: "strip_tracking_params", Name: "Strip Tracking Params", Description: "Remove utm_*/fbclid/gclid etc. from navigated URLs", Value: b.config.StripTrackingParams, Type: "bool"},
			{ID: "session_auto_save", Name: "Auto Save Session", Description: "Enable automatic session saving", Value: b.config.SessionAutoSave, Type: "bool"},
		}
	case "advanced":
		return []Setting{
			{ID: "cookie_file", Name: "Cookie File", Description: "Path to store persistent cookies", Value: b.config.CookieFile, Type: "string"},
			{ID: "session_file", Name: "Session File", Description: "Path to store session data", Value: b.config.SessionFile, Type: "string"},
		}
	default:
		return []Setting{}
	}
}
