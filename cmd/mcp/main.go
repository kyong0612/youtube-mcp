// Package main implements the YouTube MCP server in stdio mode.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"

	"github.com/youtube-transcript-mcp/internal/cache"
	"github.com/youtube-transcript-mcp/internal/config"
	"github.com/youtube-transcript-mcp/internal/mcp"
	"github.com/youtube-transcript-mcp/internal/youtube"
)

// Version information (set during build)
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	// Load configuration first to get log level
	cfg, err := config.Load()
	if err != nil {
		// Fallback logger
		fallbackLogger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))
		fallbackLogger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup logging to stderr with configured log level
	logLevel := slog.LevelInfo
	switch cfg.Logging.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Don't log startup information immediately - MCP requires clean stdio
	// Startup logs will be sent after first request

	// Override logging output to stderr for MCP mode
	cfg.Logging.Output = "stderr"

	// Create cache instance
	cacheInstance := setupCache(cfg.Cache, logger)
	defer func() {
		if err := cacheInstance.Close(); err != nil {
			logger.Error("Failed to close cache", "error", err)
		}
	}()

	// Initialize YouTube service with fallback support
	baseService := youtube.NewService(cfg.YouTube, cacheInstance, logger)
	youtubeService := youtube.NewEnhancedService(baseService)

	// Initialize MCP server
	mcpServer := mcp.NewServer(youtubeService, cfg.MCP, logger)

	// Start processing stdin/stdout
	if err := runStdioMode(mcpServer, logger); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}

func runStdioMode(mcpServer *mcp.Server, logger *slog.Logger) error {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	firstRequest := true

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Log startup info on first request
		if firstRequest {
			logger.Info("YouTube Transcript MCP Server started",
				slog.String("version", Version),
				slog.String("build_time", BuildTime),
				slog.String("git_commit", GitCommit),
				slog.String("go_version", runtime.Version()),
			)
			firstRequest = false
		}

		// Log incoming request to stderr
		logger.Debug("Received request", "data", string(line))

		// Parse to check for ID
		var rawRequest map[string]interface{}
		if err := json.Unmarshal(line, &rawRequest); err != nil {
			logger.Error("Failed to parse request", "error", err)
			// Send parse error response without ID
			errorResp := map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -32700,
					"message": "Parse error",
					"data":    err.Error(),
				},
			}
			if err := encoder.Encode(errorResp); err != nil {
				logger.Error("Failed to encode error response", "error", err)
			}
			continue
		}

		// Process the request
		response, err := mcpServer.HandleRawMessage(context.Background(), line)
		if err != nil {
			logger.Error("Failed to handle message", "error", err)
			// Send error response with ID if available
			errorResp := map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -32603,
					"message": "Internal error",
					"data":    err.Error(),
				},
			}
			if id, ok := rawRequest["id"]; ok {
				errorResp["id"] = id
			}
			if err := encoder.Encode(errorResp); err != nil {
				logger.Error("Failed to encode error response", "error", err)
			}
			continue
		}

		// Only send response if not nil (notifications don't get responses)
		if response != nil {
			// Send response to stdout
			if err := encoder.Encode(response); err != nil {
				logger.Error("Failed to encode response", "error", err)
				return err
			}

			// Log response to stderr
			if respBytes, err := json.Marshal(response); err == nil {
				logger.Debug("Sent response", "data", string(respBytes))
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func setupCache(cfg config.CacheConfig, logger *slog.Logger) cache.Cache {
	// Setup cache based on type
	switch cfg.Type {
	case "memory":
		// Don't log during initialization - MCP requires clean stdio
		return cache.NewMemoryCache(
			cfg.MaxSize,
			cfg.MaxMemoryMB*1024*1024, // Convert MB to bytes
			cfg.CleanupInterval,
		)
	default:
		// Default to memory cache - don't log during initialization
		return cache.NewMemoryCache(
			cfg.MaxSize,
			cfg.MaxMemoryMB*1024*1024,
			cfg.CleanupInterval,
		)
	}
}