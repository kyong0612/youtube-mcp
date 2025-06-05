#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Server URL
SERVER_URL="http://localhost:8080"

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    if [ "$status" = "success" ]; then
        echo -e "${GREEN}✓${NC} $message"
    elif [ "$status" = "error" ]; then
        echo -e "${RED}✗${NC} $message"
    else
        echo -e "${YELLOW}→${NC} $message"
    fi
}

# Function to check if server is running
check_server() {
    if curl -s -f "$SERVER_URL/health" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Function to make MCP request
make_mcp_request() {
    local method=$1
    local params=$2
    local id=$3
    
    local data="{\"jsonrpc\":\"2.0\",\"id\":$id,\"method\":\"$method\""
    if [ -n "$params" ]; then
        data="${data},\"params\":$params"
    fi
    data="${data}}"
    
    curl -s -X POST "$SERVER_URL/mcp" \
        -H "Content-Type: application/json" \
        -d "$data"
}

# Start verification
echo "=== YouTube MCP Server Verification ==="
echo ""

# Check if server is already running
print_status "info" "Checking if server is already running..."
if check_server; then
    print_status "success" "Server is already running"
    SERVER_PID=""
else
    print_status "info" "Starting server..."
    
    # Build and start server
    if [ -f "./server" ]; then
        ./server &
    else
        go run cmd/server/main.go &
    fi
    SERVER_PID=$!
    
    # Wait for server to start
    sleep 3
    
    if check_server; then
        print_status "success" "Server started successfully (PID: $SERVER_PID)"
    else
        print_status "error" "Failed to start server"
        exit 1
    fi
fi

echo ""

# Test 1: Health check
print_status "info" "Testing health endpoint..."
HEALTH_RESPONSE=$(curl -s "$SERVER_URL/health")
if [ $? -eq 0 ]; then
    print_status "success" "Health check passed: $HEALTH_RESPONSE"
else
    print_status "error" "Health check failed"
fi

echo ""

# Test 2: List tools
print_status "info" "Testing MCP tools/list..."
TOOLS_RESPONSE=$(make_mcp_request "tools/list" "" 1)
if echo "$TOOLS_RESPONSE" | grep -q "get_transcript"; then
    print_status "success" "Tools listed successfully"
    echo "$TOOLS_RESPONSE" | jq -r '.result.tools[].name' 2>/dev/null | sed 's/^/  - /'
else
    print_status "error" "Failed to list tools"
    echo "Response: $TOOLS_RESPONSE"
fi

echo ""

# Test 3: Get transcript
print_status "info" "Testing get_transcript..."
TRANSCRIPT_RESPONSE=$(make_mcp_request "tools/call" '{"name":"get_transcript","arguments":{"video_identifier":"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}}' 2)
if echo "$TRANSCRIPT_RESPONSE" | grep -q "result"; then
    print_status "success" "Transcript fetched successfully"
    # Show first few words
    FIRST_WORDS=$(echo "$TRANSCRIPT_RESPONSE" | jq -r '.result.content[0].text' 2>/dev/null | head -c 50)
    if [ -n "$FIRST_WORDS" ]; then
        echo "  First words: ${FIRST_WORDS}..."
    fi
else
    print_status "error" "Failed to fetch transcript"
    echo "Response: $TRANSCRIPT_RESPONSE"
fi

echo ""

# Test 4: Format transcript
print_status "info" "Testing format_transcript..."
SEARCH_RESPONSE=$(make_mcp_request "tools/call" '{"name":"format_transcript","arguments":{"video_identifier":"https://www.youtube.com/watch?v=dQw4w9WgXcQ","format_type":"plain_text"}}' 3)
if echo "$SEARCH_RESPONSE" | grep -q "result"; then
    print_status "success" "Format transcript completed successfully"
    # Show format type
    FORMAT_PREVIEW=$(echo "$SEARCH_RESPONSE" | jq -r '.result.content[0].text' 2>/dev/null | head -c 100)
    if [ -n "$FORMAT_PREVIEW" ]; then
        echo "  Format preview: ${FORMAT_PREVIEW}..."
    fi
else
    print_status "error" "Failed to format transcript"
    echo "Response: $SEARCH_RESPONSE"
fi

echo ""

# Test 5: List available languages
print_status "info" "Testing list_available_languages..."
CHANNEL_RESPONSE=$(make_mcp_request "tools/call" '{"name":"list_available_languages","arguments":{"video_identifier":"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}}' 4)
if echo "$CHANNEL_RESPONSE" | grep -q "result"; then
    print_status "success" "Languages listed successfully"
    LANG_COUNT=$(echo "$CHANNEL_RESPONSE" | jq '.result.content[0].text' 2>/dev/null | grep -o "languages available" | wc -l)
    if [ "$LANG_COUNT" -gt 0 ]; then
        echo "  Language information retrieved"
    fi
else
    print_status "error" "Failed to list languages"
    echo "Response: $CHANNEL_RESPONSE"
fi

echo ""

# Cleanup
if [ -n "$SERVER_PID" ]; then
    print_status "info" "Stopping server (PID: $SERVER_PID)..."
    kill $SERVER_PID 2>/dev/null
    wait $SERVER_PID 2>/dev/null
    print_status "success" "Server stopped"
else
    print_status "info" "Server was already running, leaving it running"
fi

echo ""
echo "=== Verification Complete ==="