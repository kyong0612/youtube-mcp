# YouTube Transcript MCP STDIO Server

This is the STDIO mode server for MCP clients like Claude Desktop, Claude Code, and Cursor.

## Overview

This server implements the Model Context Protocol (MCP) over STDIO, which is required for integration with MCP clients. Unlike the HTTP server (`cmd/server/`), this version:

- Communicates via standard input/output (STDIO)
- Uses line-delimited JSON-RPC 2.0 messages
- Logs only to stderr to keep stdout clean for MCP communication
- Is designed to be launched directly by MCP clients

## Building

From project root:

```bash
# Build the STDIO server
go build -o youtube-mcp-stdio ./cmd/mcp/

# Or build with version info
go build -ldflags "-X main.Version=1.0.0" -o youtube-mcp-stdio ./cmd/mcp/
```

## Usage

This server is not meant to be run directly. It should be configured in your MCP client:

### Claude Desktop Configuration

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp-stdio",
      "env": {
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true",
        "YOUTUBE_DEFAULT_LANGUAGES": "en,ja"
      }
    }
  }
}
```

## Differences from HTTP Server

| Feature | STDIO Server (`cmd/mcp/`) | HTTP Server (`cmd/server/`) |
|---------|---------------------------|----------------------------|
| Protocol | STDIO (stdin/stdout) | HTTP REST API |
| Use Case | MCP clients | Development, testing, direct API access |
| Communication | Line-delimited JSON-RPC | HTTP JSON-RPC |
| Logging | stderr only | Configurable (file, stdout) |
| Port | N/A | Configurable (default: 8080) |

## Environment Variables

The STDIO server uses the same environment variables as the HTTP server, except:

- `PORT` is ignored (no network listener)
- `SERVER_*` variables are ignored (no HTTP server configuration)
- Logging always goes to stderr regardless of `LOG_OUTPUT`

## Testing

To test the STDIO server manually:

```bash
# Send initialize request
echo '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | ./youtube-mcp-stdio

# List tools
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | ./youtube-mcp-stdio
```

Note: The server expects one JSON-RPC message per line and will continue running until EOF or error.