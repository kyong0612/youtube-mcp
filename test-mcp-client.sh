#!/bin/bash

# Test MCP client script

echo "Testing MCP server..." >&2

# Send initialize request
echo '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"0.1.0"}}}' | /Users/kimkiyong/go/bin/youtube-mcp 2>server.log

echo "Server logs:" >&2
cat server.log >&2