// Package config provides configuration management for the YouTube MCP server.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the complete application configuration
type Config struct {
	YouTube  YouTubeConfig  `json:"youtube"`
	Security SecurityConfig `json:"security"`
	Metrics  MetricsConfig  `json:"metrics"`
	Server   ServerConfig   `json:"server"`
	Cache    CacheConfig    `json:"cache"`
	MCP      MCPConfig      `json:"mcp"`
	Logging  LoggingConfig  `json:"logging"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host            string        `json:"host"`
	CORSOrigins     []string      `json:"cors_origins"`
	Port            int           `json:"port"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	IdleTimeout     time.Duration `json:"idle_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	MaxRequestSize  int64         `json:"max_request_size"`
	EnableCORS      bool          `json:"enable_cors"`
	EnableGzip      bool          `json:"enable_gzip"`
}

// YouTubeConfig represents YouTube-specific configuration
type YouTubeConfig struct {
	UserAgent           string        `json:"user_agent"`
	YoutubeDLPath       string        `json:"youtubedl_path"`
	CookieFile          string        `json:"cookie_file"`
	APIKey              string        `json:"api_key"`
	ProxyURL            string        `json:"proxy_url"`
	DefaultLanguages    []string      `json:"default_languages"`
	ProxyList           []string      `json:"proxy_list"`
	RetryDelay          time.Duration `json:"retry_delay"`
	RateLimitPerHour    int           `json:"rate_limit_per_hour"`
	RateLimitPerMinute  int           `json:"rate_limit_per_minute"`
	RetryBackoffFactor  float64       `json:"retry_backoff_factor"`
	MaxConcurrent       int           `json:"max_concurrent"`
	RetryAttempts       int           `json:"retry_attempts"`
	RequestTimeout      time.Duration `json:"request_timeout"`
	EnableProxyRotation bool          `json:"enable_proxy_rotation"`
	EnableCookies       bool          `json:"enable_cookies"`
	EnableYoutubeDL     bool          `json:"enable_youtubedl"`
}

// MCPConfig represents MCP-specific configuration
type MCPConfig struct {
	Tools           map[string]bool `json:"tools"`
	Version         string          `json:"version"`
	ServerName      string          `json:"server_name"`
	ServerVersion   string          `json:"server_version"`
	MaxConcurrent   int             `json:"max_concurrent"`
	RequestTimeout  time.Duration   `json:"request_timeout"`
	MaxRequestSize  int64           `json:"max_request_size"`
	EnableResources bool            `json:"enable_resources"`
	EnablePrompts   bool            `json:"enable_prompts"`
	EnableLogging   bool            `json:"enable_logging"`
}

// CacheConfig represents cache configuration
type CacheConfig struct {
	Type              string        `json:"type"`
	RedisPassword     string        `json:"redis_password"`
	RedisURL          string        `json:"redis_url"`
	MetadataTTL       time.Duration `json:"metadata_ttl"`
	LanguagesTTL      time.Duration `json:"languages_ttl"`
	ErrorTTL          time.Duration `json:"error_ttl"`
	MaxSize           int           `json:"max_size"`
	MaxMemoryMB       int           `json:"max_memory_mb"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	TranscriptTTL     time.Duration `json:"transcript_ttl"`
	RedisDB           int           `json:"redis_db"`
	RedisPoolSize     int           `json:"redis_pool_size"`
	Enabled           bool          `json:"enabled"`
	EnableCompression bool          `json:"enable_compression"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	JWTSecret         string        `json:"jwt_secret"`
	APIKeys           []string      `json:"api_keys"`
	IPWhitelist       []string      `json:"ip_whitelist"`
	IPBlacklist       []string      `json:"ip_blacklist"`
	JWTExpiry         time.Duration `json:"jwt_expiry"`
	RateLimitPerIP    int           `json:"rate_limit_per_ip"`
	RateLimitWindow   time.Duration `json:"rate_limit_window"`
	EnableAuth        bool          `json:"enable_auth"`
	EnableRateLimit   bool          `json:"enable_rate_limit"`
	EnableIPWhitelist bool          `json:"enable_ip_whitelist"`
	EnableIPBlacklist bool          `json:"enable_ip_blacklist"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level            string  `json:"level"`
	Format           string  `json:"format"` // "json", "text"
	Output           string  `json:"output"` // "stdout", "stderr", "file"
	FilePath         string  `json:"file_path"`
	MaxSizeMB        int     `json:"max_size_mb"`
	MaxBackups       int     `json:"max_backups"`
	MaxAgeDays       int     `json:"max_age_days"`
	Compress         bool    `json:"compress"`
	EnableCaller     bool    `json:"enable_caller"`
	EnableStacktrace bool    `json:"enable_stacktrace"`
	SamplingEnabled  bool    `json:"sampling_enabled"`
	SamplingRate     float64 `json:"sampling_rate"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Path            string `json:"path"`
	Port            int    `json:"port"`
	Enabled         bool   `json:"enabled"`
	EnableHistogram bool   `json:"enable_histogram"`
	EnableSummary   bool   `json:"enable_summary"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            8080,
			Host:            "0.0.0.0",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 30 * time.Second,
			MaxRequestSize:  10 * 1024 * 1024, // 10MB
			EnableCORS:      true,
			CORSOrigins:     []string{"*"},
			EnableGzip:      true,
		},
		YouTube: YouTubeConfig{
			DefaultLanguages:   []string{"en", "ja", "es", "fr", "de"},
			RequestTimeout:     30 * time.Second,
			RetryAttempts:      3,
			RetryDelay:         time.Second,
			RetryBackoffFactor: 2.0,
			RateLimitPerMinute: 60,
			RateLimitPerHour:   1000,
			UserAgent:          "YouTube-Transcript-MCP-Server/1.0.0",
			MaxConcurrent:      10,
			EnableCookies:      false,
			EnableYoutubeDL:    false,
		},
		MCP: MCPConfig{
			Version:        "2024-11-05",
			ServerName:     "youtube-transcript-server",
			ServerVersion:  "1.0.0",
			MaxConcurrent:  10,
			RequestTimeout: 60 * time.Second,
			MaxRequestSize: 5 * 1024 * 1024, // 5MB
			Tools: map[string]bool{
				"get_transcript":           true,
				"get_multiple_transcripts": true,
				"translate_transcript":     true,
				"format_transcript":        true,
				"list_available_languages": true,
			},
			EnableResources: false,
			EnablePrompts:   false,
			EnableLogging:   true,
		},
		Cache: CacheConfig{
			Type:              "memory",
			Enabled:           true,
			TranscriptTTL:     24 * time.Hour,
			MetadataTTL:       1 * time.Hour,
			LanguagesTTL:      6 * time.Hour,
			ErrorTTL:          15 * time.Minute,
			MaxSize:           1000,
			MaxMemoryMB:       512,
			CleanupInterval:   1 * time.Hour,
			RedisPoolSize:     10,
			EnableCompression: true,
		},
		Security: SecurityConfig{
			EnableAuth:        false,
			EnableRateLimit:   true,
			RateLimitPerIP:    100,
			RateLimitWindow:   time.Minute,
			EnableIPWhitelist: false,
			EnableIPBlacklist: false,
		},
		Logging: LoggingConfig{
			Level:            "info",
			Format:           "json",
			Output:           "stdout",
			MaxSizeMB:        100,
			MaxBackups:       3,
			MaxAgeDays:       7,
			Compress:         true,
			EnableCaller:     true,
			EnableStacktrace: false,
			SamplingEnabled:  false,
			SamplingRate:     1.0,
		},
		Metrics: MetricsConfig{
			Enabled:         true,
			Port:            9090,
			Path:            "/metrics",
			EnableHistogram: true,
			EnableSummary:   true,
		},
	}
}

// Load loads configuration from environment variables with defaults
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Server configuration
	cfg.Server.Port = getEnvInt("PORT", cfg.Server.Port)
	cfg.Server.Host = getEnvString("HOST", cfg.Server.Host)
	cfg.Server.ReadTimeout = getEnvDuration("SERVER_READ_TIMEOUT", cfg.Server.ReadTimeout)
	cfg.Server.WriteTimeout = getEnvDuration("SERVER_WRITE_TIMEOUT", cfg.Server.WriteTimeout)
	cfg.Server.IdleTimeout = getEnvDuration("SERVER_IDLE_TIMEOUT", cfg.Server.IdleTimeout)
	cfg.Server.ShutdownTimeout = getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", cfg.Server.ShutdownTimeout)
	cfg.Server.MaxRequestSize = getEnvInt64("SERVER_MAX_REQUEST_SIZE", cfg.Server.MaxRequestSize)
	cfg.Server.EnableCORS = getEnvBool("SERVER_ENABLE_CORS", cfg.Server.EnableCORS)
	cfg.Server.EnableGzip = getEnvBool("SERVER_ENABLE_GZIP", cfg.Server.EnableGzip)

	if corsOrigins := getEnvString("SERVER_CORS_ORIGINS", ""); corsOrigins != "" {
		cfg.Server.CORSOrigins = strings.Split(corsOrigins, ",")
	}

	// YouTube configuration
	cfg.YouTube.APIKey = getEnvString("YOUTUBE_API_KEY", cfg.YouTube.APIKey)
	if defaultLangs := getEnvString("YOUTUBE_DEFAULT_LANGUAGES", ""); defaultLangs != "" {
		cfg.YouTube.DefaultLanguages = strings.Split(defaultLangs, ",")
	}
	cfg.YouTube.RequestTimeout = getEnvDuration("YOUTUBE_REQUEST_TIMEOUT", cfg.YouTube.RequestTimeout)
	cfg.YouTube.RetryAttempts = getEnvInt("YOUTUBE_RETRY_ATTEMPTS", cfg.YouTube.RetryAttempts)
	cfg.YouTube.RetryDelay = getEnvDuration("YOUTUBE_RETRY_DELAY", cfg.YouTube.RetryDelay)
	cfg.YouTube.RetryBackoffFactor = getEnvFloat("YOUTUBE_RETRY_BACKOFF_FACTOR", cfg.YouTube.RetryBackoffFactor)
	cfg.YouTube.RateLimitPerMinute = getEnvInt("YOUTUBE_RATE_LIMIT_PER_MINUTE", cfg.YouTube.RateLimitPerMinute)
	cfg.YouTube.RateLimitPerHour = getEnvInt("YOUTUBE_RATE_LIMIT_PER_HOUR", cfg.YouTube.RateLimitPerHour)
	cfg.YouTube.UserAgent = getEnvString("USER_AGENT", cfg.YouTube.UserAgent)
	cfg.YouTube.ProxyURL = getEnvString("YOUTUBE_PROXY_URL", cfg.YouTube.ProxyURL)
	cfg.YouTube.EnableProxyRotation = getEnvBool("YOUTUBE_ENABLE_PROXY_ROTATION", cfg.YouTube.EnableProxyRotation)
	if proxyList := getEnvString("YOUTUBE_PROXY_LIST", ""); proxyList != "" {
		cfg.YouTube.ProxyList = strings.Split(proxyList, ",")
	}
	cfg.YouTube.MaxConcurrent = getEnvInt("YOUTUBE_MAX_CONCURRENT", cfg.YouTube.MaxConcurrent)
	cfg.YouTube.CookieFile = getEnvString("YOUTUBE_COOKIE_FILE", cfg.YouTube.CookieFile)
	cfg.YouTube.EnableCookies = getEnvBool("YOUTUBE_ENABLE_COOKIES", cfg.YouTube.EnableCookies)
	cfg.YouTube.YoutubeDLPath = getEnvString("YOUTUBE_DL_PATH", cfg.YouTube.YoutubeDLPath)
	cfg.YouTube.EnableYoutubeDL = getEnvBool("YOUTUBE_ENABLE_YOUTUBEDL", cfg.YouTube.EnableYoutubeDL)

	// MCP configuration
	cfg.MCP.Version = getEnvString("MCP_VERSION", cfg.MCP.Version)
	cfg.MCP.ServerName = getEnvString("MCP_SERVER_NAME", cfg.MCP.ServerName)
	cfg.MCP.ServerVersion = getEnvString("MCP_SERVER_VERSION", cfg.MCP.ServerVersion)
	cfg.MCP.MaxConcurrent = getEnvInt("MCP_MAX_CONCURRENT", cfg.MCP.MaxConcurrent)
	cfg.MCP.RequestTimeout = getEnvDuration("MCP_REQUEST_TIMEOUT", cfg.MCP.RequestTimeout)
	cfg.MCP.MaxRequestSize = getEnvInt64("MCP_MAX_REQUEST_SIZE", cfg.MCP.MaxRequestSize)
	cfg.MCP.EnableResources = getEnvBool("MCP_ENABLE_RESOURCES", cfg.MCP.EnableResources)
	cfg.MCP.EnablePrompts = getEnvBool("MCP_ENABLE_PROMPTS", cfg.MCP.EnablePrompts)
	cfg.MCP.EnableLogging = getEnvBool("MCP_ENABLE_LOGGING", cfg.MCP.EnableLogging)

	// Cache configuration
	cfg.Cache.Type = getEnvString("CACHE_TYPE", cfg.Cache.Type)
	cfg.Cache.Enabled = getEnvBool("CACHE_ENABLED", cfg.Cache.Enabled)
	cfg.Cache.TranscriptTTL = getEnvDuration("CACHE_TRANSCRIPT_TTL", cfg.Cache.TranscriptTTL)
	cfg.Cache.MetadataTTL = getEnvDuration("CACHE_METADATA_TTL", cfg.Cache.MetadataTTL)
	cfg.Cache.LanguagesTTL = getEnvDuration("CACHE_LANGUAGES_TTL", cfg.Cache.LanguagesTTL)
	cfg.Cache.ErrorTTL = getEnvDuration("CACHE_ERROR_TTL", cfg.Cache.ErrorTTL)
	cfg.Cache.MaxSize = getEnvInt("CACHE_MAX_SIZE", cfg.Cache.MaxSize)
	cfg.Cache.MaxMemoryMB = getEnvInt("CACHE_MAX_MEMORY_MB", cfg.Cache.MaxMemoryMB)
	cfg.Cache.CleanupInterval = getEnvDuration("CACHE_CLEANUP_INTERVAL", cfg.Cache.CleanupInterval)
	cfg.Cache.RedisURL = getEnvString("REDIS_URL", cfg.Cache.RedisURL)
	cfg.Cache.RedisPassword = getEnvString("REDIS_PASSWORD", cfg.Cache.RedisPassword)
	cfg.Cache.RedisDB = getEnvInt("REDIS_DB", cfg.Cache.RedisDB)
	cfg.Cache.RedisPoolSize = getEnvInt("REDIS_POOL_SIZE", cfg.Cache.RedisPoolSize)
	cfg.Cache.EnableCompression = getEnvBool("CACHE_ENABLE_COMPRESSION", cfg.Cache.EnableCompression)

	// Security configuration
	cfg.Security.EnableAuth = getEnvBool("SECURITY_ENABLE_AUTH", cfg.Security.EnableAuth)
	if apiKeys := getEnvString("SECURITY_API_KEYS", ""); apiKeys != "" {
		cfg.Security.APIKeys = strings.Split(apiKeys, ",")
	}
	cfg.Security.JWTSecret = getEnvString("SECURITY_JWT_SECRET", cfg.Security.JWTSecret)
	cfg.Security.JWTExpiry = getEnvDuration("SECURITY_JWT_EXPIRY", cfg.Security.JWTExpiry)
	cfg.Security.EnableRateLimit = getEnvBool("SECURITY_ENABLE_RATE_LIMIT", cfg.Security.EnableRateLimit)
	cfg.Security.RateLimitPerIP = getEnvInt("SECURITY_RATE_LIMIT_PER_IP", cfg.Security.RateLimitPerIP)
	cfg.Security.RateLimitWindow = getEnvDuration("SECURITY_RATE_LIMIT_WINDOW", cfg.Security.RateLimitWindow)
	cfg.Security.EnableIPWhitelist = getEnvBool("SECURITY_ENABLE_IP_WHITELIST", cfg.Security.EnableIPWhitelist)
	if ipWhitelist := getEnvString("SECURITY_IP_WHITELIST", ""); ipWhitelist != "" {
		cfg.Security.IPWhitelist = strings.Split(ipWhitelist, ",")
	}
	cfg.Security.EnableIPBlacklist = getEnvBool("SECURITY_ENABLE_IP_BLACKLIST", cfg.Security.EnableIPBlacklist)
	if ipBlacklist := getEnvString("SECURITY_IP_BLACKLIST", ""); ipBlacklist != "" {
		cfg.Security.IPBlacklist = strings.Split(ipBlacklist, ",")
	}

	// Logging configuration
	cfg.Logging.Level = getEnvString("LOG_LEVEL", cfg.Logging.Level)
	cfg.Logging.Format = getEnvString("LOG_FORMAT", cfg.Logging.Format)
	cfg.Logging.Output = getEnvString("LOG_OUTPUT", cfg.Logging.Output)
	cfg.Logging.FilePath = getEnvString("LOG_FILE_PATH", cfg.Logging.FilePath)
	cfg.Logging.MaxSizeMB = getEnvInt("LOG_MAX_SIZE_MB", cfg.Logging.MaxSizeMB)
	cfg.Logging.MaxBackups = getEnvInt("LOG_MAX_BACKUPS", cfg.Logging.MaxBackups)
	cfg.Logging.MaxAgeDays = getEnvInt("LOG_MAX_AGE_DAYS", cfg.Logging.MaxAgeDays)
	cfg.Logging.Compress = getEnvBool("LOG_COMPRESS", cfg.Logging.Compress)
	cfg.Logging.EnableCaller = getEnvBool("LOG_ENABLE_CALLER", cfg.Logging.EnableCaller)
	cfg.Logging.EnableStacktrace = getEnvBool("LOG_ENABLE_STACKTRACE", cfg.Logging.EnableStacktrace)
	cfg.Logging.SamplingEnabled = getEnvBool("LOG_SAMPLING_ENABLED", cfg.Logging.SamplingEnabled)
	cfg.Logging.SamplingRate = getEnvFloat("LOG_SAMPLING_RATE", cfg.Logging.SamplingRate)

	// Metrics configuration
	cfg.Metrics.Enabled = getEnvBool("METRICS_ENABLED", cfg.Metrics.Enabled)
	cfg.Metrics.Port = getEnvInt("METRICS_PORT", cfg.Metrics.Port)
	cfg.Metrics.Path = getEnvString("METRICS_PATH", cfg.Metrics.Path)
	cfg.Metrics.EnableHistogram = getEnvBool("METRICS_ENABLE_HISTOGRAM", cfg.Metrics.EnableHistogram)
	cfg.Metrics.EnableSummary = getEnvBool("METRICS_ENABLE_SUMMARY", cfg.Metrics.EnableSummary)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.YouTube.RetryAttempts < 0 {
		return fmt.Errorf("invalid retry attempts: %d", c.YouTube.RetryAttempts)
	}

	if c.YouTube.RateLimitPerHour < 0 {
		return fmt.Errorf("invalid rate limit per hour: %d", c.YouTube.RateLimitPerHour)
	}

	if c.Cache.MaxSize < 0 {
		return fmt.Errorf("invalid cache max size: %d", c.Cache.MaxSize)
	}

	if c.Cache.Type != "memory" && c.Cache.Type != "redis" && c.Cache.Type != "memcached" {
		return fmt.Errorf("invalid cache type: %s", c.Cache.Type)
	}

	if c.Logging.Format != "json" && c.Logging.Format != "text" {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	if c.Logging.SamplingRate < 0 || c.Logging.SamplingRate > 1 {
		return fmt.Errorf("invalid log sampling rate: %f", c.Logging.SamplingRate)
	}

	return nil
}

// Helper functions for environment variable parsing

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
