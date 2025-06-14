#!/bin/bash

echo "=== Test 1: Notification (no ID, no response expected) ===" >&2
echo '{"jsonrpc":"2.0","method":"test"}' | /Users/kimkiyong/go/bin/youtube-mcp 2>/dev/null

echo -e "\n=== Test 2: Request with ID (response expected) ===" >&2
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' | /Users/kimkiyong/go/bin/youtube-mcp 2>/dev/null | jq .