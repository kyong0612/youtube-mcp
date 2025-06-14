#!/bin/bash

# URLからビデオIDを再取得
VIDEO_ID="YaE2GYegLNU"

# YouTube ページからURLを取得
URL=$(curl -s "https://www.youtube.com/watch?v=$VIDEO_ID" \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36" \
  -H "Accept-Language: en-US,en;q=0.9" | \
  grep -o 'var ytInitialPlayerResponse = {.*};' | \
  sed 's/var ytInitialPlayerResponse = //' | \
  sed 's/;$//' | \
  jq -r '.captions.playerCaptionsTracklistRenderer.captionTracks[0].baseUrl' 2>/dev/null)

if [ -z "$URL" ]; then
  echo "Failed to extract transcript URL"
  exit 1
fi

echo "Transcript URL: $URL"
echo -e "\nFetching transcript..."

# Transcriptを取得
curl -s "$URL" \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36" \
  -H "Accept-Language: en-US,en;q=0.9" | head -20