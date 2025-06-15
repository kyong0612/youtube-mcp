# YouTube Transcript MCP Server - Development Guide

## 🚀 Quick Start

### Prerequisites
- Go 1.24+
- Docker & Docker Compose
- Make (optional)

### Local Development

```bash
# Clone repository
git clone https://github.com/kyong0612/youtube-mcp
cd youtube-mcp

# Install dependencies
go mod download

# Run tests
go test ./...

# Run server
go run cmd/server/main.go

# Or use make
make run
```

### Docker Development

```bash
# Build image
docker build -t youtube-mcp-server .

# Run container
docker run -p 8080:8080 -p 9090:9090 youtube-mcp-server

# Or use Docker Compose
docker compose up -d
```

## 🏗️ Project Structure

```
youtube-mcp/
├── cmd/
│   ├── server/
│   │   ├── main.go          # HTTP server entry point
│   │   └── README.md        # HTTP server documentation
│   └── mcp/
│       ├── main.go          # STDIO MCP server for Claude Desktop
│       └── README.md        # STDIO server documentation
├── internal/
│   ├── cache/
│   │   ├── interface.go     # Cache interface
│   │   ├── memory.go        # In-memory cache implementation
│   │   └── memory_test.go
│   ├── config/
│   │   ├── config.go        # Configuration management
│   │   └── config_test.go
│   ├── mcp/
│   │   ├── server.go        # MCP protocol implementation
│   │   ├── interfaces.go    # Service interfaces
│   │   └── server_test.go
│   ├── models/
│   │   ├── types.go         # Data models
│   │   └── types_test.go
│   └── youtube/
│       ├── service.go       # YouTube service implementation
│       └── service_test.go
├── docs/
│   ├── requirements.md      # Original requirements
│   ├── implementation-status.md
│   └── development-guide.md # This file
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod
└── go.sum
```

## 🔧 Configuration

### Environment Variables

```bash
# Server Configuration
PORT=8080
HOST=0.0.0.0
LOG_LEVEL=info

# YouTube Configuration
YOUTUBE_DEFAULT_LANGUAGES=en,ja,es
YOUTUBE_REQUEST_TIMEOUT=30s
YOUTUBE_RETRY_ATTEMPTS=3
YOUTUBE_RATE_LIMIT_PER_MINUTE=60

# Cache Configuration
CACHE_ENABLED=true
CACHE_TYPE=memory
CACHE_MAX_SIZE=1000
CACHE_TRANSCRIPT_TTL=24h

# Security Configuration
SECURITY_ENABLE_AUTH=false
SECURITY_API_KEYS=key1,key2,key3
```

### Configuration Priority
1. Environment variables
2. Default values in code

## 🧪 Testing

### Run All Tests
```bash
go test ./... -v
```

### Run Specific Package Tests
```bash
go test ./internal/mcp/... -v
```

### Run with Coverage
```bash
go test ./... -cover
```

### Run with Race Detection
```bash
go test ./... -race
```

## 🐛 Debugging

### Enable Debug Logging
```bash
LOG_LEVEL=debug go run cmd/server/main.go
```

### Test MCP Protocol
```bash
# Initialize
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "initialize"}'

# List tools
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}'

# Get transcript
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "get_transcript",
      "arguments": {
        "video_identifier": "dQw4w9WgXcQ",
        "languages": ["en"]
      }
    }
  }'
```

## 📝 Adding New Features

### 1. Adding a New Tool

1. Define the tool parameters in `internal/models/types.go`:
```go
type NewToolParams struct {
    VideoIdentifier string   `json:"video_identifier" validate:"required"`
    // Add fields
}
```

2. Add the tool to `internal/mcp/tools.go`:
```go
var toolDefinitions = map[string]ToolDefinition{
    "new_tool": {
        Name:        "new_tool",
        Description: "Description of the new tool",
        InputSchema: // JSON schema
    },
}
```

3. Implement the handler in `internal/mcp/server.go`:
```go
case "new_tool":
    var params models.NewToolParams
    // Implementation
```

4. Add tests in `internal/mcp/server_test.go`

### 2. Adding a New Cache Backend

1. Implement the Cache interface in `internal/cache/interface.go`:
```go
type RedisCache struct {
    client *redis.Client
}

func (r *RedisCache) Get(ctx context.Context, key string) (interface{}, bool) {
    // Implementation
}
// Implement other methods
```

2. Add configuration in `internal/config/config.go`
3. Update cache factory in `cmd/server/main.go`

## 🚨 Common Issues

### 1. "Request too large" Error
- Increase `MCP_MAX_REQUEST_SIZE` environment variable
- Default is 5MB

### 2. Health Check Failing
- Check if all dependencies are accessible
- Verify cache initialization
- Check logs for specific errors

### 3. YouTube API Errors
- Verify video ID format
- Check rate limits
- Ensure network connectivity

## 🔄 Git Workflow

```bash
# Create feature branch
git checkout -b feature/your-feature

# Make changes and test
go test ./...

# Commit with conventional commits
git commit -m "feat: add new feature"
git commit -m "fix: resolve issue"
git commit -m "docs: update documentation"

# Push and create PR
git push origin feature/your-feature
```

## 📊 Performance Tuning

### 1. Cache Optimization
- Adjust `CACHE_MAX_SIZE` based on memory
- Set appropriate `CACHE_TRANSCRIPT_TTL`
- Monitor cache hit rates

### 2. Concurrent Processing
- Adjust `YOUTUBE_MAX_CONCURRENT`
- Monitor goroutine counts
- Use rate limiting to prevent overload

### 3. Memory Usage
- Profile with `pprof`
- Adjust Docker memory limits
- Monitor GC statistics

## 🔍 Monitoring

### Metrics Endpoint
```bash
curl http://localhost:9090/metrics
```

### Health Check
```bash
curl http://localhost:8080/health
```

### Logs
```bash
# Docker logs
docker logs youtube-transcript-mcp -f

# Docker Compose logs
docker compose logs -f youtube-transcript-mcp
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

### Code Style
- Follow Go conventions
- Use `gofmt` and `golint`
- Write descriptive commit messages
- Add comments for exported functions

## 📚 Resources

- [Go Documentation](https://golang.org/doc/)
- [MCP Specification](https://modelcontextprotocol.io/)
- [Chi Router](https://github.com/go-chi/chi)
- [Docker Best Practices](https://docs.docker.com/develop/dev-best-practices/)