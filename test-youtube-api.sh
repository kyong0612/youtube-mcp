#!/bin/bash

VIDEO_ID="YaE2GYegLNU"
echo "Testing YouTube video page fetch for: $VIDEO_ID"

# Fetch the page and extract player response
curl -s "https://www.youtube.com/watch?v=$VIDEO_ID" \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36" \
  -H "Accept-Language: en-US,en;q=0.9" | \
  grep -o 'var ytInitialPlayerResponse = {.*};' | \
  sed 's/var ytInitialPlayerResponse = //' | \
  sed 's/;$//' | \
  jq '.captions.playerCaptionsTracklistRenderer.captionTracks[0]' 2>/dev/null || echo "Failed to extract caption tracks"