# YouTube Transcript MCP Server

A high-performance Model Context Protocol (MCP) server for fetching YouTube video transcripts, implemented in Go.

## 🚀 Features

- **MCP Protocol 2024-11-05 Compliant**: Full implementation of the Model Context Protocol
- **5 Powerful Tools**:
  - `get_transcript`: Fetch transcript for a single video
  - `get_multiple_transcripts`: Batch process multiple videos
  - `translate_transcript`: Translate transcripts to different languages
  - `format_transcript`: Format transcripts (plain text, SRT, VTT, etc.)
  - `list_available_languages`: List available subtitle languages
- **High Performance**: Built with Go for speed and efficiency
- **Caching**: In-memory and Redis cache support
- **Rate Limiting**: Protect against YouTube API limits
- **Proxy Support**: Rotate through multiple proxies
- **Docker Ready**: Easy deployment with Docker Compose
- **Monitoring**: Built-in health checks and metrics

## 📋 Requirements

- Go 1.22 or higher
- Docker & Docker Compose (optional)
- Internet connection

## 🎯 MCP Client Setup

### Claude Desktop

To use this server with Claude Desktop, add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "go",
      "args": ["run", "/path/to/youtube-mcp/cmd/server/main.go"],
      "env": {
        "PORT": "8080",
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true",
        "YOUTUBE_DEFAULT_LANGUAGES": "en,ja"
      }
    }
  }
}
```

### Claude Code (claude.ai/code)

Claude Code automatically detects MCP servers. To configure:

1. **Using Go runtime:**
```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "go",
      "args": ["run", "/path/to/youtube-mcp/cmd/server/main.go"],
      "env": {
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true"
      }
    }
  }
}
```

2. **Using compiled binary:**
```bash
# First, build the binary
cd /path/to/youtube-mcp
make build

# Then add to Claude Code config
```
```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-transcript-mcp",
      "env": {
        "LOG_LEVEL": "info"
      }
    }
  }
}
```

### Cursor

Cursor supports MCP servers through its settings. To configure:

1. Open Cursor Settings (`Cmd+,` on macOS, `Ctrl+,` on Windows/Linux)
2. Search for "MCP" or "Model Context Protocol"
3. Add the server configuration:

```json
{
  "mcp.servers": {
    "youtube-transcript": {
      "command": "go",
      "args": ["run", "/path/to/youtube-mcp/cmd/server/main.go"],
      "env": {
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true",
        "YOUTUBE_DEFAULT_LANGUAGES": "en,ja"
      }
    }
  }
}
```

Or use the compiled binary:
```json
{
  "mcp.servers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-transcript-mcp",
      "env": {
        "LOG_LEVEL": "info"
      }
    }
  }
}
```

See [docs/mcp-client-setup.md](docs/mcp-client-setup.md) for detailed setup instructions.

## 🛠️ Installation

### Using Go

```bash
# Clone the repository
git clone https://github.com/yourusername/youtube-transcript-mcp.git
cd youtube-transcript-mcp

# Install dependencies
make deps

# Build the application
make build

# Run the server
make run
```

### Using Docker

```bash
# Clone the repository
git clone https://github.com/yourusername/youtube-transcript-mcp.git
cd youtube-transcript-mcp

# Setup environment
make env-setup
# Edit .env file with your configuration

# Start with Docker Compose
make up
```

## ⚙️ Configuration

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
```

Key configuration options:

- `PORT`: Server port (default: 8080)
- `YOUTUBE_DEFAULT_LANGUAGES`: Default languages for transcripts
- `CACHE_TYPE`: Cache type (memory/redis)
- `SECURITY_ENABLE_AUTH`: Enable API authentication
- `LOG_LEVEL`: Logging level (debug/info/warn/error)

## 🔧 Usage

### Initialize MCP

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize"
  }'
```

### List Available Tools

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'
```

### Get Transcript

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "get_transcript",
      "arguments": {
        "video_identifier": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
        "languages": ["en", "ja"],
        "preserve_formatting": false
      }
    }
  }'
```

### Get Multiple Transcripts

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "get_multiple_transcripts",
      "arguments": {
        "video_identifiers": ["dQw4w9WgXcQ", "jNQXAC9IVRw"],
        "languages": ["en"],
        "continue_on_error": true
      }
    }
  }'
```

## 🧪 Development

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run benchmarks
make benchmark
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Security scan
make security
```

### Hot Reload Development

```bash
# Install air for hot reload
go install github.com/air-verse/air@latest

# Run with hot reload
make dev
```

## 🐳 Docker Deployment

### Basic Deployment

```bash
# Build and start
make up-build

# View logs
make logs

# Stop services
make down
```

### With Redis Cache

```bash
# Start with Redis
make up-redis
```

### With Monitoring

```bash
# Start with Prometheus & Grafana
make up-monitoring
```

## 📊 Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

### Readiness Check

```bash
curl http://localhost:8080/ready
```

### Metrics

```bash
curl http://localhost:9090/metrics
```

## 🔒 Security

- **API Key Authentication**: Set `SECURITY_ENABLE_AUTH=true` and configure API keys
- **Rate Limiting**: Configurable per-IP rate limiting
- **IP Whitelisting/Blacklisting**: Control access by IP address
- **CORS**: Configurable CORS policies

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by [youtube-transcript-api](https://github.com/jdepoix/youtube-transcript-api)
- Built for the [Model Context Protocol](https://modelcontextprotocol.io/)

## ⚠️ Disclaimer

This tool is for educational and research purposes. Please respect YouTube's Terms of Service and copyright laws when using transcripts.
