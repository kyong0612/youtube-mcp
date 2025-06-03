package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test server defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected default read timeout 30s, got %v", cfg.Server.ReadTimeout)
	}

	// Test YouTube defaults
	if len(cfg.YouTube.DefaultLanguages) != 5 {
		t.Errorf("Expected 5 default languages, got %d", len(cfg.YouTube.DefaultLanguages))
	}
	if cfg.YouTube.RetryAttempts != 3 {
		t.Errorf("Expected 3 retry attempts, got %d", cfg.YouTube.RetryAttempts)
	}
	if cfg.YouTube.UserAgent != "YouTube-Transcript-MCP-Server/1.0.0" {
		t.Errorf("Unexpected user agent: %s", cfg.YouTube.UserAgent)
	}

	// Test MCP defaults
	if cfg.MCP.Version != "2024-11-05" {
		t.Errorf("Expected MCP version 2024-11-05, got %s", cfg.MCP.Version)
	}
	if cfg.MCP.ServerName != "youtube-transcript-server" {
		t.Errorf("Unexpected server name: %s", cfg.MCP.ServerName)
	}
	if !cfg.MCP.Tools["get_transcript"] {
		t.Error("Expected get_transcript tool to be enabled")
	}

	// Test cache defaults
	if !cfg.Cache.Enabled {
		t.Error("Expected cache to be enabled by default")
	}
	if cfg.Cache.Type != "memory" {
		t.Errorf("Expected default cache type 'memory', got %s", cfg.Cache.Type)
	}
	if cfg.Cache.TranscriptTTL != 24*time.Hour {
		t.Errorf("Expected transcript TTL 24h, got %v", cfg.Cache.TranscriptTTL)
	}
}

func TestLoadWithEnvironmentVariables(t *testing.T) {
	// Set test environment variables
	testEnvVars := map[string]string{
		"PORT":                     "9090",
		"LOG_LEVEL":                "debug",
		"YOUTUBE_API_KEY":          "test-api-key",
		"YOUTUBE_DEFAULT_LANGUAGES": "en,fr,de",
		"CACHE_ENABLED":            "false",
		"CACHE_TYPE":               "redis",
		"REDIS_URL":                "redis://test:6379",
		"SECURITY_ENABLE_AUTH":     "true",
		"SECURITY_API_KEYS":        "key1,key2,key3",
	}

	// Set environment variables
	for key, value := range testEnvVars {
		os.Setenv(key, value)
		defer os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment variables were loaded
	if cfg.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug', got %s", cfg.Logging.Level)
	}
	if cfg.YouTube.APIKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got %s", cfg.YouTube.APIKey)
	}
	if len(cfg.YouTube.DefaultLanguages) != 3 {
		t.Errorf("Expected 3 languages, got %d", len(cfg.YouTube.DefaultLanguages))
	}
	if cfg.Cache.Enabled {
		t.Error("Expected cache to be disabled")
	}
	if cfg.Cache.Type != "redis" {
		t.Errorf("Expected cache type 'redis', got %s", cfg.Cache.Type)
	}
	if cfg.Cache.RedisURL != "redis://test:6379" {
		t.Errorf("Expected Redis URL 'redis://test:6379', got %s", cfg.Cache.RedisURL)
	}
	if !cfg.Security.EnableAuth {
		t.Error("Expected auth to be enabled")
	}
	if len(cfg.Security.APIKeys) != 3 {
		t.Errorf("Expected 3 API keys, got %d", len(cfg.Security.APIKeys))
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*Config)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "invalid port - too low",
			setupFunc: func(cfg *Config) {
				cfg.Server.Port = 0
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "invalid port - too high",
			setupFunc: func(cfg *Config) {
				cfg.Server.Port = 70000
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "invalid retry attempts",
			setupFunc: func(cfg *Config) {
				cfg.YouTube.RetryAttempts = -1
			},
			wantErr: true,
			errMsg:  "invalid retry attempts",
		},
		{
			name: "invalid rate limit",
			setupFunc: func(cfg *Config) {
				cfg.YouTube.RateLimitPerHour = -100
			},
			wantErr: true,
			errMsg:  "invalid rate limit per hour",
		},
		{
			name: "invalid cache size",
			setupFunc: func(cfg *Config) {
				cfg.Cache.MaxSize = -10
			},
			wantErr: true,
			errMsg:  "invalid cache max size",
		},
		{
			name: "invalid cache type",
			setupFunc: func(cfg *Config) {
				cfg.Cache.Type = "invalid"
			},
			wantErr: true,
			errMsg:  "invalid cache type",
		},
		{
			name: "invalid log format",
			setupFunc: func(cfg *Config) {
				cfg.Logging.Format = "invalid"
			},
			wantErr: true,
			errMsg:  "invalid log format",
		},
		{
			name: "invalid sampling rate - negative",
			setupFunc: func(cfg *Config) {
				cfg.Logging.SamplingRate = -0.5
			},
			wantErr: true,
			errMsg:  "invalid log sampling rate",
		},
		{
			name: "invalid sampling rate - too high",
			setupFunc: func(cfg *Config) {
				cfg.Logging.SamplingRate = 1.5
			},
			wantErr: true,
			errMsg:  "invalid log sampling rate",
		},
		{
			name: "valid config",
			setupFunc: func(cfg *Config) {
				// Use defaults
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.setupFunc(cfg)

			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected validation error but got none")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestGetEnvHelpers(t *testing.T) {
	// Test getEnvString
	os.Setenv("TEST_STRING", "hello")
	defer os.Unsetenv("TEST_STRING")
	if v := getEnvString("TEST_STRING", "default"); v != "hello" {
		t.Errorf("Expected 'hello', got '%s'", v)
	}
	if v := getEnvString("MISSING_STRING", "default"); v != "default" {
		t.Errorf("Expected 'default', got '%s'", v)
	}

	// Test getEnvInt
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")
	if v := getEnvInt("TEST_INT", 10); v != 42 {
		t.Errorf("Expected 42, got %d", v)
	}
	if v := getEnvInt("MISSING_INT", 10); v != 10 {
		t.Errorf("Expected 10, got %d", v)
	}

	// Test getEnvBool
	os.Setenv("TEST_BOOL", "true")
	defer os.Unsetenv("TEST_BOOL")
	if v := getEnvBool("TEST_BOOL", false); !v {
		t.Error("Expected true, got false")
	}
	if v := getEnvBool("MISSING_BOOL", true); !v {
		t.Error("Expected true, got false")
	}

	// Test getEnvDuration
	os.Setenv("TEST_DURATION", "5m")
	defer os.Unsetenv("TEST_DURATION")
	if v := getEnvDuration("TEST_DURATION", time.Second); v != 5*time.Minute {
		t.Errorf("Expected 5m, got %v", v)
	}
	if v := getEnvDuration("MISSING_DURATION", time.Hour); v != time.Hour {
		t.Errorf("Expected 1h, got %v", v)
	}

	// Test getEnvFloat
	os.Setenv("TEST_FLOAT", "3.14")
	defer os.Unsetenv("TEST_FLOAT")
	if v := getEnvFloat("TEST_FLOAT", 2.0); v != 3.14 {
		t.Errorf("Expected 3.14, got %f", v)
	}
	if v := getEnvFloat("MISSING_FLOAT", 2.0); v != 2.0 {
		t.Errorf("Expected 2.0, got %f", v)
	}

	// Test getEnvInt64
	os.Setenv("TEST_INT64", "9223372036854775807")
	defer os.Unsetenv("TEST_INT64")
	if v := getEnvInt64("TEST_INT64", 100); v != 9223372036854775807 {
		t.Errorf("Expected max int64, got %d", v)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}