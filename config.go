package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	// Browser settings
	UserAgent    string `mapstructure:"user_agent"`
	WindowSizeX int    `mapstructure:"window_size_x"`
	WindowSizeY int    `mapstructure:"window_size_y"`
	
	// Cookie settings
	CookieFile string `mapstructure:"cookie_file"`
	CookieAutoSave bool `mapstructure:"cookie_auto_save"`
	
	// Session settings
	SessionFile       string `mapstructure:"session_file"`
	SessionAutoSave   bool   `mapstructure:"session_auto_save"`
	SessionHistorySize int   `mapstructure:"session_history_size"`
	
	// Network settings
	Proxy         string `mapstructure:"proxy"`
	RequestTimeout int    `mapstructure:"request_timeout"`
	MaxRedirects  int    `mapstructure:"max_redirects"`
	
	// Content settings
	MaxPageSize     int64 `mapstructure:"max_page_size"`
	MaxImageSize    int64 `mapstructure:"max_image_size"`
	EnableImages    bool  `mapstructure:"enable_images"`
	EnableScripts   bool  `mapstructure:"enable_scripts"` // For info, won't actually execute
	EnableCookies   bool  `mapstructure:"enable_cookies"`
	
	// UI settings
	Theme      string `mapstructure:"theme"`
	ShowImages bool   `mapstructure:"show_images"`
	WordWrap   bool   `mapstructure:"word_wrap"`
}

// GetDefaultConfig returns the default configuration
func GetDefaultConfig() Config {
	return Config{
		UserAgent:        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		WindowSizeX:      80,
		WindowSizeY:      24,
		CookieFile:       "cookies.json",
		CookieAutoSave:   true,
		SessionFile:      "session.json",
		SessionAutoSave:  true,
		SessionHistorySize: 50,
		Proxy:            "",
		RequestTimeout:   30,
		MaxRedirects:     10,
		MaxPageSize:      50 * 1024 * 1024, // 50MB
		MaxImageSize:     5 * 1024 * 1024,  // 5MB
		EnableImages:     true,
		EnableScripts:    false,
		EnableCookies:    true,
		Theme:           "dark", // Changed from "default" to "dark" as the default theme
		ShowImages:      true,
		WordWrap:        true,
	}
}

// GetConfigDir returns the appropriate config directory for the OS
func GetConfigDir() string {
	var configDir string
	
	switch runtime.GOOS {
	case "windows":
		configDir = filepath.Join(getWindowsConfigPath(), "t-browser")
	case "darwin": // macOS
		configDir = filepath.Join(getMacOSConfigPath(), "t-browser")
	default: // Linux and other Unix-like systems
		configDir = GetLinuxConfigPath()
	}
	
	return configDir
}

// GetLinuxConfigPath returns the config directory for Linux systems
func GetLinuxConfigPath() string {
	// Use XDG config directory or home directory
	configHome := viper.GetString("XDG_CONFIG_HOME")
	if configHome == "" {
		home, _ := getHomeDir()
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "t-browser")
}

// getWindowsConfigPath returns the config directory for Windows systems
func getWindowsConfigPath() string {
	appData, _ := getHomeDir() // Use USERPROFILE on Windows
	return filepath.Join(appData, "AppData", "Roaming")
}

// getMacOSConfigPath returns the config directory for macOS systems
func getMacOSConfigPath() string {
	home, _ := getHomeDir()
	return filepath.Join(home, "Library", "Application Support")
}

// getHomeDir returns the user's home directory
func getHomeDir() (string, error) {
	// Use os.UserHomeDir() which works cross-platform
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to environment variable
		home = os.Getenv("HOME") // Also works on Windows as fallback
		if home == "" {
			home = os.Getenv("USERPROFILE") // Windows
		}
	}
	return home, err
}

// InitializeConfig sets up Viper with configuration file
func InitializeConfig() error {
	// Set the name of the configuration file (without extension)
	viper.SetConfigName("config")

	// Add the config directory to search paths
	configDir := GetConfigDir()
	viper.AddConfigPath(configDir)

	// Also look in current directory
	viper.AddConfigPath(".")

	// Set default values
	defaultConfig := GetDefaultConfig()
	viper.SetDefault("user_agent", defaultConfig.UserAgent)
	viper.SetDefault("window_size_x", defaultConfig.WindowSizeX)
	viper.SetDefault("window_size_y", defaultConfig.WindowSizeY)
	viper.SetDefault("cookie_file", defaultConfig.CookieFile)
	viper.SetDefault("cookie_auto_save", defaultConfig.CookieAutoSave)
	viper.SetDefault("session_file", defaultConfig.SessionFile)
	viper.SetDefault("session_auto_save", defaultConfig.SessionAutoSave)
	viper.SetDefault("session_history_size", defaultConfig.SessionHistorySize)
	viper.SetDefault("proxy", defaultConfig.Proxy)
	viper.SetDefault("request_timeout", defaultConfig.RequestTimeout)
	viper.SetDefault("max_redirects", defaultConfig.MaxRedirects)
	viper.SetDefault("max_page_size", defaultConfig.MaxPageSize)
	viper.SetDefault("max_image_size", defaultConfig.MaxImageSize)
	viper.SetDefault("enable_images", defaultConfig.EnableImages)
	viper.SetDefault("enable_scripts", defaultConfig.EnableScripts)
	viper.SetDefault("enable_cookies", defaultConfig.EnableCookies)
	viper.SetDefault("theme", defaultConfig.Theme)
	viper.SetDefault("show_images", defaultConfig.ShowImages)
	viper.SetDefault("word_wrap", defaultConfig.WordWrap)

	// Try to read the config file
	err := viper.ReadInConfig()
	if err != nil {
		// If config file doesn't exist, create default one
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Create config directory if it doesn't exist
			configDir := GetConfigDir()
			if _, err := os.Stat(configDir); os.IsNotExist(err) {
				os.MkdirAll(configDir, 0755)
			}

			// Write the default config to file
			err = WriteDefaultConfig(configDir)
			if err != nil {
				return err
			}

			// Now read the config again
			viper.SetConfigFile(filepath.Join(configDir, "config.yaml"))
			err = viper.ReadInConfig()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// WriteDefaultConfig writes the default configuration to file
func WriteDefaultConfig(configDir string) error {
	viper.Set("user_agent", GetDefaultConfig().UserAgent)
	viper.Set("window_size_x", GetDefaultConfig().WindowSizeX)
	viper.Set("window_size_y", GetDefaultConfig().WindowSizeY)
	viper.Set("cookie_file", GetDefaultConfig().CookieFile)
	viper.Set("cookie_auto_save", GetDefaultConfig().CookieAutoSave)
	viper.Set("session_file", GetDefaultConfig().SessionFile)
	viper.Set("session_auto_save", GetDefaultConfig().SessionAutoSave)
	viper.Set("session_history_size", GetDefaultConfig().SessionHistorySize)
	viper.Set("proxy", GetDefaultConfig().Proxy)
	viper.Set("request_timeout", GetDefaultConfig().RequestTimeout)
	viper.Set("max_redirects", GetDefaultConfig().MaxRedirects)
	viper.Set("max_page_size", GetDefaultConfig().MaxPageSize)
	viper.Set("max_image_size", GetDefaultConfig().MaxImageSize)
	viper.Set("enable_images", GetDefaultConfig().EnableImages)
	viper.Set("enable_scripts", GetDefaultConfig().EnableScripts)
	viper.Set("enable_cookies", GetDefaultConfig().EnableCookies)
	viper.Set("theme", GetDefaultConfig().Theme)
	viper.Set("show_images", GetDefaultConfig().ShowImages)
	viper.Set("word_wrap", GetDefaultConfig().WordWrap)

	// Write to file
	configPath := filepath.Join(configDir, "config.yaml")
	return viper.WriteConfigAs(configPath)
}

// LoadConfig loads the configuration from viper
func LoadConfig() (Config, error) {
	var config Config

	// Unmarshal the configuration into the struct
	err := viper.Unmarshal(&config)
	if err != nil {
		return config, err
	}

	// Set defaults if values are not loaded properly
	defaultConfig := GetDefaultConfig()
	if config.UserAgent == "" {
		config.UserAgent = defaultConfig.UserAgent
	}
	if config.CookieFile == "" {
		config.CookieFile = defaultConfig.CookieFile
	}
	if config.SessionFile == "" {
		config.SessionFile = defaultConfig.SessionFile
	}

	return config, nil
}

// GetTimestamp returns a timestamp string in YYYY-MM-DD_HH-MM-SS format
func GetTimestamp() string {
	return time.Now().Format("2006-01-02_15-04-05")
}

// GetCookieFilePath returns the path for a new cookie file with timestamp
func GetCookieFilePath(configDir string) string {
	// Create cookies subdirectory if it doesn't exist
	cookiesDir := filepath.Join(configDir, "cookies")
	if _, err := os.Stat(cookiesDir); os.IsNotExist(err) {
		os.MkdirAll(cookiesDir, 0755)
	}

	// Create a filename with timestamp
	filename := fmt.Sprintf("cookies_%s.json", GetTimestamp())
	return filepath.Join(cookiesDir, filename)
}

// GetSessionFilePath returns the path for a new session file with timestamp
func GetSessionFilePath(configDir string) string {
	// Create sessions subdirectory if it doesn't exist
	sessionsDir := filepath.Join(configDir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		os.MkdirAll(sessionsDir, 0755)
	}

	// Create a filename with timestamp
	filename := fmt.Sprintf("session_%s.json", GetTimestamp())
	return filepath.Join(sessionsDir, filename)
}

// GetLatestCookieFile returns the path to the most recent cookie file
func GetLatestCookieFile(configDir string) string {
	cookiesDir := filepath.Join(configDir, "cookies")
	if _, err := os.Stat(cookiesDir); os.IsNotExist(err) {
		return ""
	}

	// Read all files in the cookies directory
	files, err := os.ReadDir(cookiesDir)
	if err != nil {
		return ""
	}

	// Find the most recent cookie file
	var latestFile string
	var latestTime time.Time

	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "cookies_") && strings.HasSuffix(file.Name(), ".json") {
			// Extract timestamp from filename
			// Format: cookies_YYYY-MM-DD_HH-MM-SS.json
			timeStr := strings.TrimPrefix(file.Name(), "cookies_")
			timeStr = strings.TrimSuffix(timeStr, ".json")

			// Parse the timestamp
			fileTime, err := time.Parse("2006-01-02_15-04-05", timeStr)
			if err == nil && fileTime.After(latestTime) {
				latestTime = fileTime
				latestFile = filepath.Join(cookiesDir, file.Name())
			}
		}
	}

	return latestFile
}

// GetLatestSessionFile returns the path to the most recent session file
func GetLatestSessionFile(configDir string) string {
	sessionsDir := filepath.Join(configDir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return ""
	}

	// Read all files in the sessions directory
	files, err := os.ReadDir(sessionsDir)
	if err != nil {
		return ""
	}

	// Find the most recent session file
	var latestFile string
	var latestTime time.Time

	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "session_") && strings.HasSuffix(file.Name(), ".json") {
			// Extract timestamp from filename
			// Format: session_YYYY-MM-DD_HH-MM-SS.json
			timeStr := strings.TrimPrefix(file.Name(), "session_")
			timeStr = strings.TrimSuffix(timeStr, ".json")

			// Parse the timestamp
			fileTime, err := time.Parse("2006-01-02_15-04-05", timeStr)
			if err == nil && fileTime.After(latestTime) {
				latestTime = fileTime
				latestFile = filepath.Join(sessionsDir, file.Name())
			}
		}
	}

	return latestFile
}

// WriteToFile writes the current configuration to the config file
func (c *Config) WriteToFile(configDir string) error {
	// Use Viper to set all config values
	viper.Set("user_agent", c.UserAgent)
	viper.Set("window_size_x", c.WindowSizeX)
	viper.Set("window_size_y", c.WindowSizeY)
	viper.Set("cookie_file", c.CookieFile)
	viper.Set("cookie_auto_save", c.CookieAutoSave)
	viper.Set("session_file", c.SessionFile)
	viper.Set("session_auto_save", c.SessionAutoSave)
	viper.Set("session_history_size", c.SessionHistorySize)
	viper.Set("proxy", c.Proxy)
	viper.Set("request_timeout", c.RequestTimeout)
	viper.Set("max_redirects", c.MaxRedirects)
	viper.Set("max_page_size", c.MaxPageSize)
	viper.Set("max_image_size", c.MaxImageSize)
	viper.Set("enable_images", c.EnableImages)
	viper.Set("enable_scripts", c.EnableScripts)
	viper.Set("enable_cookies", c.EnableCookies)
	viper.Set("theme", c.Theme)
	viper.Set("show_images", c.ShowImages)
	viper.Set("word_wrap", c.WordWrap)

	// Write to the config file
	configPath := filepath.Join(configDir, "config.yaml")
	return viper.WriteConfigAs(configPath)
}