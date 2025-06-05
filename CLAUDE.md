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

# Run only short tests (quick feedback)
make test-short

# Run integration tests
make test-integration

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

# Run Go vet for static analysis
make vet

# Run all checks (fmt, lint, test)
make check

# Check for outdated dependencies
make mod-check

# Show dependency graph
make mod-graph
```

### Build & Deploy

```bash
# Build binary
make build

# Build for all platforms (Linux, macOS, Windows with AMD64/ARM64)
make build-all

# Create release package
make release

# Build Docker image
make docker-build

# Run with Docker Compose
make up

# Run with Redis profile
make up-redis

# Run with monitoring (Prometheus/Grafana)
make up-monitoring

# View logs
make logs

# Clean build artifacts
make clean
```

### Testing & Verification

```bash
# Test live API endpoints (requires running server)
make test-api

# Test MCP initialization
make test-mcp-init

# Test MCP tool listing
make test-mcp-tools

# Test transcript fetching
make test-transcript

# Run comprehensive verification script
./scripts/verify-server.sh
```

### Monitoring & Health

```bash
# Check application health (formatted)
make health

# Check readiness
make ready

# Show Prometheus metrics
make metrics

# Show application statistics
make stats

# Combined status view
make status
```

## Architecture & Key Patterns

### Layered Architecture

1. **Entry Point Layer** (`cmd/server/main.go`)
   - Configuration loading and dependency injection
   - Middleware pipeline setup (request ID, logging, rate limiting, CORS, auth, compression)
   - Graceful shutdown handling

2. **Protocol Layer** (`internal/mcp/server.go`)
   - JSON-RPC 2.0 protocol implementation
   - Tool registration with schema validation
   - Request routing and statistics tracking
   - MCP-compliant error responses

3. **Business Logic Layer** (`internal/youtube/service.go`)
   - Adaptive rate limiting with exponential backoff
   - Proxy rotation support
   - Multi-language transcript handling
   - Concurrent request processing

4. **Infrastructure Layer**
   - Cache abstraction with size/memory limits
   - Parallel health checking system
   - Configuration management with environment variables

### MCP Protocol Implementation

The MCP server (`internal/mcp/server.go`) handles JSON-RPC 2.0 requests and routes them to appropriate tool handlers. Each tool is registered with input validation schemas. The server maintains request statistics and supports concurrent request processing.

### YouTube Service Architecture

The YouTube service (`internal/youtube/service.go`) follows these key patterns:

- **Video Data Extraction**: HTML parsing extracts `ytInitialPlayerResponse` from YouTube pages
- **Caption Track Selection**: Language preference logic with fallback support
- **XML Parsing**: Handles both `<transcript>` and `<timedtext>` formats
- **Caching Strategy**: Cache keys include video ID and language preferences
- **Rate Limiting**: Adaptive rate limiting tracks failures and adjusts delays

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

## MCP Tools

The server exposes 5 MCP tools:

- `get_transcript`: Fetch transcript for a single video
- `get_multiple_transcripts`: Batch fetch transcripts (max 50 videos)
- `translate_transcript`: Translate transcript to target language
- `format_transcript`: Format transcript (plain_text, paragraphs, sentences, srt, vtt, json)
- `list_available_languages`: List available transcript languages

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
3. **internal/youtube/service.go**: Core YouTube functionality with rate limiting
4. **internal/config/config.go**: Configuration loading and defaults
5. **internal/health/health.go**: Health check implementation
6. **docker-compose.yml**: Three profiles (default, with-redis, monitoring)

## Environment Variables

Key variables that affect behavior:

- `LOG_LEVEL`: Set to "debug" for detailed logging including HTTP responses
- `YOUTUBE_DEFAULT_LANGUAGES`: Comma-separated language codes (e.g., "en,ja,es")
- `CACHE_TYPE`: "memory" or "redis" (currently only memory is implemented)
- `SECURITY_ENABLE_AUTH`: Enables API key authentication
- `YOUTUBE_USER_AGENT`: Custom user agent for YouTube requests
- `CACHE_TRANSCRIPT_TTL`: How long to cache transcripts (default: 24h)
- `METRICS_ENABLED`: Enable Prometheus metrics endpoint on port 9090
- `YOUTUBE_RATE_LIMIT`: Requests per second limit (default: 10)
- `YOUTUBE_PROXY_URLS`: Comma-separated proxy URLs for rotation

## Development Scripts

### install-mcp.sh
Automatically installs the MCP server and updates Claude Desktop configuration. Supports both binary installation and Go runtime execution.

### verify-server.sh
Comprehensive verification script that tests all MCP tools and health endpoints. Provides colored output and handles server lifecycle.

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.