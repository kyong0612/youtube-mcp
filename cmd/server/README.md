# YouTube Transcript MCP Server

This is the main server application for the YouTube Transcript MCP Server.

## Structure

- `main.go` - Entry point for the server application
  - HTTP server setup with Chi router
  - Middleware configuration (logging, CORS, rate limiting, auth)
  - Health check and monitoring endpoints
  - Graceful shutdown handling

## Running

From project root:

```bash
# Run directly
go run ./cmd/server/main.go

# Or use make
make run

# Build and run
make build
./youtube-transcript-mcp
```

## Configuration

The server is configured via environment variables. See `.env.example` for all available options.

Key configurations:
- `PORT` - Server port (default: 8080)
- `LOG_LEVEL` - Logging level (debug/info/warn/error)
- `CACHE_TYPE` - Cache type (memory/redis)
- `SECURITY_ENABLE_AUTH` - Enable API authentication