#!/bin/bash

# Test various MCP requests

echo "=== Test 1: Initialize without ID ===" >&2
echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"claude-ai","version":"0.1.0"}}}' | /Users/kimkiyong/go/bin/youtube-mcp 2>/dev/null | jq .

echo -e "\n=== Test 2: Initialize with ID 0 ===" >&2
echo '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"claude-ai","version":"0.1.0"}}}' | /Users/kimkiyong/go/bin/youtube-mcp 2>/dev/null | jq .

echo -e "\n=== Test 3: List tools ===" >&2
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | /Users/kimkiyong/go/bin/youtube-mcp 2>/dev/null | jq .

echo -e "\n=== Test 4: Invalid method ===" >&2
echo '{"jsonrpc":"2.0","id":2,"method":"invalid"}' | /Users/kimkiyong/go/bin/youtube-mcp 2>/dev/null | jq .