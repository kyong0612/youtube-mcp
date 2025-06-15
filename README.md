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

- Go 1.24 or higher
- Docker & Docker Compose (optional)
- Internet connection

## 🎯 MCP Client Setup

### Quick Install (Recommended)

Use the automatic installation script:

```bash
# Clone the repository
git clone https://github.com/kyong0612/youtube-mcp.git
cd youtube-mcp

# Run the installer
./scripts/install-mcp.sh
```

The installer will:
- Build the MCP server binary
- Configure Claude Desktop automatically
- Set up environment variables

### Manual Setup

#### Claude Desktop

To use this server with Claude Desktop, add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-mcp-stdio",
      "args": [],
      "env": {
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true",
        "YOUTUBE_DEFAULT_LANGUAGES": "en,ja"
      }
    }
  }
}
```

**Important**: Claude Desktop requires the stdio version of the server (`youtube-mcp-stdio`), not the HTTP server.

Build the stdio server:
```bash
go build -o youtube-mcp-stdio ./cmd/mcp/
```

#### Claude Code (claude.ai/code)

Claude Code automatically detects MCP servers. Use the same configuration as Claude Desktop.

#### Cursor

Cursor supports MCP servers through its settings. To configure:

1. Open Cursor Settings (`Cmd+,` on macOS, `Ctrl+,` on Windows/Linux)
2. Search for "MCP" or "Model Context Protocol"
3. Add the server configuration:

```json
{
  "mcp.servers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-mcp-stdio",
      "args": [],
      "env": {
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true",
        "YOUTUBE_DEFAULT_LANGUAGES": "en,ja"
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
git clone https://github.com/kyong0612/youtube-mcp.git
cd youtube-mcp

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
git clone https://github.com/kyong0612/youtube-mcp.git
cd youtube-mcp

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

### Using with MCP Clients (Claude Desktop, Cursor, etc.)

The MCP server will be automatically started by your MCP client. Once configured, you can use the tools directly in your conversations.

### Using as HTTP Server

For development or testing, you can also run the HTTP server version:

```bash
# Run HTTP server
make run
```

Then test with:

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

## 🐛 Troubleshooting

### Common Issues

#### "Empty transcript response" error
- **Cause**: The server is running in HTTP mode instead of stdio mode
- **Solution**: Ensure you're using `youtube-mcp-stdio` binary, not `youtube-transcript-mcp`

#### "Request timed out" error
- **Cause**: Claude Desktop timeout or server not responding
- **Solution**: 
  - Restart Claude Desktop
  - Check server logs: `LOG_LEVEL=debug` in environment
  - Verify network connectivity

#### "Failed to extract player response" in health checks
- **Cause**: YouTube page structure changes or rate limiting
- **Solution**: This is usually temporary. The server will retry automatically.

#### Server not connecting to Claude Desktop
- **Cause**: Incorrect configuration or binary path
- **Solution**:
  1. Verify the binary exists: `ls -la /path/to/youtube-mcp-stdio`
  2. Check Claude Desktop logs: Developer → Open logs
  3. Ensure the config file is valid JSON

### Debug Mode

Enable debug logging to see detailed information:

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-mcp-stdio",
      "args": [],
      "env": {
        "LOG_LEVEL": "debug",
        "CACHE_ENABLED": "true",
        "YOUTUBE_DEFAULT_LANGUAGES": "en,ja"
      }
    }
  }
}
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
