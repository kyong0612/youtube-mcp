// Package main implements the YouTube MCP server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/youtube-transcript-mcp/internal/cache"
	"github.com/youtube-transcript-mcp/internal/config"
	"github.com/youtube-transcript-mcp/internal/health"
	"github.com/youtube-transcript-mcp/internal/mcp"
	"github.com/youtube-transcript-mcp/internal/youtube"
)

// Version information (set during build)
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// Global server state
type serverState struct {
	ready         atomic.Bool
	healthy       atomic.Bool
	healthChecker *health.Checker
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Setup structured logging
	logger := setupLogger(cfg.Logging)
	slog.SetDefault(logger)

	// Log startup information
	logger.Info("Starting YouTube Transcript MCP Server",
		slog.String("version", Version),
		slog.String("build_time", BuildTime),
		slog.String("git_commit", GitCommit),
		slog.String("go_version", runtime.Version()),
		slog.Int("port", cfg.Server.Port),
	)

	// Create cache instance
	cacheInstance := setupCache(cfg.Cache, logger)
	defer cacheInstance.Close()

	// Initialize YouTube service
	youtubeService := youtube.NewService(cfg.YouTube, cacheInstance, logger)

	// Initialize MCP server
	mcpServer := mcp.NewServer(youtubeService, cfg.MCP, logger)

	// Initialize health checker
	healthChecker := health.NewChecker(cacheInstance, youtubeService)

	// Setup HTTP server
	srv := setupHTTPServer(cfg, mcpServer, logger)

	// Server state for health checks
	state = &serverState{
		healthChecker: healthChecker,
	}

	// Setup signal handler
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		logger.Info("HTTP server starting", slog.String("address", srv.Addr))
		state.healthy.Store(true)
		state.ready.Store(true)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Start periodic health checks
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				healthStatus := state.healthChecker.CheckHealth(ctx)
				cancel()

				// Update server state based on health checks
				state.healthy.Store(healthStatus.Status != "unhealthy")

				if healthStatus.Status != "healthy" {
					logger.Warn("Health check detected issues",
						slog.String("status", healthStatus.Status),
						slog.Any("checks", healthStatus.Checks))
				}
			case <-quit:
				return
			}
		}
	}()

	// Setup metrics server if enabled
	if cfg.Metrics.Enabled {
		go startMetricsServer(cfg.Metrics, mcpServer, logger)
	}

	// Wait for interrupt signal to gracefully shutdown the server
	<-quit

	logger.Info("Shutting down server...")
	state.ready.Store(false)

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("Server exited gracefully")
}

// setupLogger configures structured logging
func setupLogger(cfg config.LoggingConfig) *slog.Logger {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     parseLogLevel(cfg.Level),
		AddSource: cfg.EnableCaller,
	}

	switch cfg.Output {
	case "stdout":
		if cfg.Format == "json" {
			handler = slog.NewJSONHandler(os.Stdout, opts)
		} else {
			handler = slog.NewTextHandler(os.Stdout, opts)
		}
	case "stderr":
		if cfg.Format == "json" {
			handler = slog.NewJSONHandler(os.Stderr, opts)
		} else {
			handler = slog.NewTextHandler(os.Stderr, opts)
		}
	default:
		// File output - would need additional implementation for rotation
		if cfg.Format == "json" {
			handler = slog.NewJSONHandler(os.Stdout, opts)
		} else {
			handler = slog.NewTextHandler(os.Stdout, opts)
		}
	}

	return slog.New(handler)
}

// parseLogLevel converts string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// setupCache creates cache instance based on configuration
func setupCache(cfg config.CacheConfig, logger *slog.Logger) cache.Cache {
	if !cfg.Enabled {
		logger.Info("Cache disabled")
		return cache.NewMemoryCache(0, 0, time.Hour) // Minimal cache
	}

	switch cfg.Type {
	case "memory":
		logger.Info("Using memory cache",
			slog.Int("max_size", cfg.MaxSize),
			slog.Int("max_memory_mb", cfg.MaxMemoryMB),
		)
		return cache.NewMemoryCache(cfg.MaxSize, cfg.MaxMemoryMB, cfg.CleanupInterval)
	case "redis":
		// Redis cache implementation would go here
		logger.Warn("Redis cache not implemented, falling back to memory cache")
		return cache.NewMemoryCache(cfg.MaxSize, cfg.MaxMemoryMB, cfg.CleanupInterval)
	default:
		logger.Warn("Unknown cache type, using memory cache", slog.String("type", cfg.Type))
		return cache.NewMemoryCache(cfg.MaxSize, cfg.MaxMemoryMB, cfg.CleanupInterval)
	}
}

// setupHTTPServer configures the HTTP server with middleware and routes
func setupHTTPServer(cfg *config.Config, mcpServer *mcp.Server, logger *slog.Logger) *http.Server {
	router := chi.NewRouter()

	// Global middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(cfg.Server.ReadTimeout))

	// Custom middleware
	router.Use(loggingMiddleware(logger))
	router.Use(metricsMiddleware())

	if cfg.Server.EnableCORS {
		router.Use(corsMiddleware(cfg.Server.CORSOrigins))
	}

	if cfg.Server.EnableGzip {
		router.Use(middleware.Compress(5))
	}

	if cfg.Security.EnableRateLimit {
		router.Use(rateLimitMiddleware(cfg.Security))
	}

	if cfg.Security.EnableAuth {
		router.Use(authMiddleware(cfg.Security))
	}

	// Health check endpoints (no auth required)
	router.Group(func(r chi.Router) {
		r.Get("/health", handleHealth)
		r.Get("/ready", handleReady)
		r.Get("/version", handleVersion)
	})

	// MCP endpoints
	router.Route("/mcp", func(r chi.Router) {
		r.Post("/", mcpServer.HandleMCP)
		r.Options("/", handleOptions) // For CORS preflight
	})

	// API endpoints (future expansion)
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.SetHeader("Content-Type", "application/json"))

		// Stats endpoint
		r.Get("/stats", func(w http.ResponseWriter, r *http.Request) {
			stats := mcpServer.GetStats()
			json.NewEncoder(w).Encode(stats)
		})
	})

	// 404 handler
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Not found",
			"path":  r.URL.Path,
		})
	})

	return &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}
}

// Middleware functions

func loggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				logger.Info("HTTP request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", ww.Status()),
					slog.Int("bytes", ww.BytesWritten()),
					slog.Duration("duration", time.Since(start)),
					slog.String("remote", r.RemoteAddr),
					slog.String("user_agent", r.UserAgent()),
					slog.String("request_id", middleware.GetReqID(r.Context())),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

func metricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Metrics collection would go here
			next.ServeHTTP(w, r)
		})
	}
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false

			// Check if origin is allowed
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitMiddleware(cfg config.SecurityConfig) func(http.Handler) http.Handler {
	// Simple in-memory rate limiter (production would use Redis)
	type visitor struct {
		lastSeen time.Time
		count    int
	}

	var (
		visitors = make(map[string]*visitor)
		mu       = &sync.RWMutex{}
	)

	// Cleanup old entries periodically
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			mu.Lock()
			for ip, v := range visitors {
				if time.Since(v.lastSeen) > cfg.RateLimitWindow {
					delete(visitors, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			mu.Lock()
			v, exists := visitors[ip]
			if !exists {
				visitors[ip] = &visitor{lastSeen: time.Now(), count: 1}
				mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}

			// Reset count if window has passed
			if time.Since(v.lastSeen) > cfg.RateLimitWindow {
				v.count = 1
				v.lastSeen = time.Now()
				mu.Unlock()
				next.ServeHTTP(w, r)
				return
			}

			// Check rate limit
			if v.count >= cfg.RateLimitPerIP {
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Rate limit exceeded",
				})
				return
			}

			v.count++
			v.lastSeen = time.Now()
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

func authMiddleware(cfg config.SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health checks
			if r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/version" {
				next.ServeHTTP(w, r)
				return
			}

			// Check API key
			apiKey := r.Header.Get("Authorization")
			if apiKey == "" {
				apiKey = r.URL.Query().Get("api_key")
			}

			if apiKey == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Missing API key",
				})
				return
			}

			// Validate API key
			valid := false
			for _, key := range cfg.APIKeys {
				if apiKey == key || apiKey == "Bearer "+key {
					valid = true
					break
				}
			}

			if !valid {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Invalid API key",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Handler functions

var state = &serverState{}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	// Perform health checks
	ctx := r.Context()
	healthStatus := state.healthChecker.CheckHealth(ctx)

	// Add version to response
	healthStatus.Version = Version

	// Determine status code
	statusCode := http.StatusOK
	if healthStatus.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if healthStatus.Status == "degraded" {
		statusCode = http.StatusOK // Still return 200 for degraded
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(healthStatus)
}

func handleReady(w http.ResponseWriter, r *http.Request) {
	// Check if service is ready
	isReady := state.healthChecker.IsReady()

	status := "ready"
	statusCode := http.StatusOK

	if !isReady || !state.ready.Load() {
		status = "not ready"
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleOptions(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// startMetricsServer starts a separate metrics server
func startMetricsServer(cfg config.MetricsConfig, mcpServer *mcp.Server, logger *slog.Logger) {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.HandleFunc(cfg.Path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// Prometheus metrics would be exposed here
		fmt.Fprintf(w, "# YouTube Transcript MCP Server Metrics\n")
		fmt.Fprintf(w, "# TODO: Implement Prometheus metrics\n")

		// For now, return basic stats
		stats := mcpServer.GetStats()
		for key, value := range stats {
			fmt.Fprintf(w, "youtube_transcript_mcp_%s %v\n", key, value)
		}
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("Starting metrics server", slog.String("address", addr))

	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("Metrics server failed", slog.Any("error", err))
	}
}
