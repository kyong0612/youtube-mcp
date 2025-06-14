#!/bin/bash

echo "Testing enhanced YouTube MCP server with fallback support..."

# Test video IDs
VIDEO_ID1="YaE2GYegLNU"  # Working video
VIDEO_ID2="dQw4w9WgXcQ"  # Rick Astley - Never Gonna Give You Up (known problematic)

# Start server in background
echo "Starting server..."
./server &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Test 1: Get transcript for working video
echo -e "\n=== Test 1: Get transcript for video $VIDEO_ID1 ==="
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"id\": 1,
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"get_transcript\",
      \"arguments\": {
        \"video_id\": \"$VIDEO_ID1\",
        \"languages\": [\"en\"]
      }
    }
  }" | jq '.result.content[0].text' | jq -r | head -20

# Test 2: Get transcript for problematic video
echo -e "\n=== Test 2: Get transcript for video $VIDEO_ID2 ==="
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"id\": 2,
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"get_transcript\",
      \"arguments\": {
        \"video_id\": \"$VIDEO_ID2\",
        \"languages\": [\"en\"]
      }
    }
  }" | jq '.result.content[0].text' | jq -r | head -20

# Test 3: List available languages
echo -e "\n=== Test 3: List available languages for video $VIDEO_ID1 ==="
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"id\": 3,
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"list_available_languages\",
      \"arguments\": {
        \"video_id\": \"$VIDEO_ID1\"
      }
    }
  }" | jq '.result.content[0].text' | jq -r

# Cleanup
echo -e "\nStopping server..."
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null

echo "Done!"