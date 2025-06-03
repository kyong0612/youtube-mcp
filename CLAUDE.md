# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based MCP (Model Context Protocol) server that fetches YouTube video transcripts. The server implements 5 MCP tools for transcript operations and is designed for high performance with caching, rate limiting, and health monitoring.

## Essential Commands

### Development

```bash
# Run the server locally
make run

# Run with hot reload (requires air)
make dev

# Run a single test file
go test -v ./internal/youtube/service_test.go

# Run tests for a specific package
go test -v ./internal/youtube/...

# Run all tests with coverage
make test-coverage

# Run benchmarks
make benchmark
```

### Code Quality

```bash
# Format code (required before commits)
make fmt

# Run linter (golangci-lint)
make lint

# Run security scan
make security

# Run all checks (fmt, lint, test)
make check
```

### Build & Deploy

```bash
# Build binary
make build

# Build Docker image
make docker-build

# Run with Docker Compose
make up

# View logs
make logs

# Clean build artifacts
make clean
```

## Architecture & Key Patterns

### MCP Protocol Implementation

The MCP server (`internal/mcp/server.go`) handles JSON-RPC 2.0 requests and routes them to appropriate tool handlers. Each tool is registered with input validation schemas. The server maintains request statistics and supports concurrent request processing.

### YouTube Service Architecture

The YouTube service (`internal/youtube/service.go`) follows these key patterns:

- **Video Data Extraction**: HTML parsing extracts `ytInitialPlayerResponse` from YouTube pages
- **Caption Track Selection**: Language preference logic with fallback support
- **XML Parsing**: Handles both `<transcript>` and `<timedtext>` formats
- **Caching Strategy**: Cache keys include video ID and language preferences

### Health Check System

The health checker (`internal/health/health.go`) runs parallel checks for:

- Cache connectivity (set/get test)
- YouTube service availability (language list fetch)
- Network connectivity (HEAD request to YouTube)

Health checks run every 30 seconds and update server state atomically.

### Configuration Flow

1. Environment variables are loaded via `config.Load()`
2. Config structs use time.Duration for intervals
3. Default values are set in the Load function
4. Sensitive values (API keys) are never logged

## Known Issues & Workarounds

### XML Parsing Failures

Some videos (e.g., dQw4w9WgXcQ) return XML that fails parsing. The issue appears to be related to empty response bodies or different XML formats. The `parseTranscriptXML` function handles both `<transcript>` and `<timedtext>` formats, but some videos may return unexpected formats. Debug logging has been added to track response sizes and content.

### Timestamp Issues

Some videos return zero or missing duration values. The code handles this but timestamps may be incorrect. This was observed in "Me at the zoo" (jNQXAC9IVRw) where all segments had duration=0.

## Testing Approach

### Unit Tests

- Mock interfaces for external dependencies (cache, HTTP client)
- Table-driven tests for parsing functions
- Test files are colocated with implementation
- Coverage target: 80%+ (currently at 92.1%)

### Integration Testing

Use the test script pattern:

```bash
# Start server in background
./server &
# or
go run cmd/server/main.go &

# Wait for startup
sleep 2

# Test MCP endpoints
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Kill server when done
pkill -f "./server"
```

## Critical Files to Understand

1. **cmd/server/main.go**: Entry point, sets up all dependencies and middleware
2. **internal/mcp/server.go**: MCP protocol handler and tool registration
3. **internal/youtube/service.go**: Core YouTube functionality
4. **internal/config/config.go**: Configuration loading and defaults
5. **internal/health/health.go**: Health check implementation

## Environment Variables

Key variables that affect behavior:

- `LOG_LEVEL`: Set to "debug" for detailed logging including HTTP responses
- `YOUTUBE_DEFAULT_LANGUAGES`: Comma-separated language codes (e.g., "en,ja,es")
- `CACHE_TYPE`: "memory" or "redis" (currently only memory is implemented)
- `SECURITY_ENABLE_AUTH`: Enables API key authentication
- `YOUTUBE_USER_AGENT`: Custom user agent for YouTube requests
- `CACHE_TRANSCRIPT_TTL`: How long to cache transcripts (default: 24h)
- `METRICS_ENABLED`: Enable Prometheus metrics endpoint on port 9090
