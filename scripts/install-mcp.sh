#!/bin/bash

# YouTube Transcript MCP Server Installation Script for Claude Desktop

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}YouTube Transcript MCP Server Installer${NC}"
echo "========================================"

# Detect OS
OS="unknown"
CONFIG_DIR=""

if [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
    CONFIG_DIR="$HOME/Library/Application Support/Claude"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
    CONFIG_DIR="$HOME/.config/Claude"
elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "win32" ]]; then
    OS="windows"
    CONFIG_DIR="$APPDATA/Claude"
else
    echo -e "${RED}Unsupported OS: $OSTYPE${NC}"
    exit 1
fi

echo -e "${GREEN}Detected OS: $OS${NC}"
echo -e "${GREEN}Config directory: $CONFIG_DIR${NC}"

# Get installation directory
INSTALL_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"
echo -e "${GREEN}Installation directory: $INSTALL_DIR${NC}"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}Go is not installed. Would you like to build a binary instead? (y/n)${NC}"
    read -r response
    if [[ "$response" == "y" ]]; then
        BUILD_BINARY=true
    else
        echo -e "${RED}Go is required to run the server. Please install Go and try again.${NC}"
        exit 1
    fi
else
    BUILD_BINARY=false
fi

# Build binary if needed
if [[ "$BUILD_BINARY" == true ]]; then
    echo -e "${GREEN}Building binary...${NC}"
    cd "$INSTALL_DIR"
    make build
    BINARY_PATH="$INSTALL_DIR/youtube-transcript-mcp"
else
    BINARY_PATH=""
fi

# Create config directory if it doesn't exist
mkdir -p "$CONFIG_DIR"

# Check if claude_desktop_config.json exists
CONFIG_FILE="$CONFIG_DIR/claude_desktop_config.json"
if [[ -f "$CONFIG_FILE" ]]; then
    echo -e "${YELLOW}Config file already exists. Creating backup...${NC}"
    cp "$CONFIG_FILE" "$CONFIG_FILE.backup.$(date +%Y%m%d_%H%M%S)"
fi

# Generate MCP server configuration
if [[ "$BUILD_BINARY" == true ]]; then
    MCP_CONFIG=$(cat <<EOF
{
  "youtube-transcript": {
    "command": "$BINARY_PATH",
    "env": {
      "PORT": "8080",
      "LOG_LEVEL": "info",
      "CACHE_ENABLED": "true",
      "YOUTUBE_DEFAULT_LANGUAGES": "en,ja,es,fr,de"
    }
  }
}
EOF
)
else
    MCP_CONFIG=$(cat <<EOF
{
  "youtube-transcript": {
    "command": "go",
    "args": ["run", "$INSTALL_DIR/cmd/server/main.go"],
    "env": {
      "PORT": "8080",
      "LOG_LEVEL": "info",
      "CACHE_ENABLED": "true",
      "YOUTUBE_DEFAULT_LANGUAGES": "en,ja,es,fr,de"
    }
  }
}
EOF
)
fi

# Update or create config file
if [[ -f "$CONFIG_FILE" ]]; then
    # Parse existing config and add our server
    echo -e "${GREEN}Updating existing config file...${NC}"
    
    # This is a simple approach - for production, use jq or a proper JSON parser
    if command -v jq &> /dev/null; then
        # Use jq if available
        jq --argjson mcp "$MCP_CONFIG" '.mcpServers."youtube-transcript" = $mcp."youtube-transcript"' "$CONFIG_FILE" > "$CONFIG_FILE.tmp"
        mv "$CONFIG_FILE.tmp" "$CONFIG_FILE"
    else
        echo -e "${YELLOW}jq not found. Please manually add the following to your config file:${NC}"
        echo "$MCP_CONFIG"
    fi
else
    # Create new config file
    echo -e "${GREEN}Creating new config file...${NC}"
    cat > "$CONFIG_FILE" <<EOF
{
  "mcpServers": $MCP_CONFIG
}
EOF
fi

# Create .env file if it doesn't exist
if [[ ! -f "$INSTALL_DIR/.env" ]]; then
    echo -e "${GREEN}Creating .env file...${NC}"
    cp "$INSTALL_DIR/.env.example" "$INSTALL_DIR/.env"
    echo -e "${YELLOW}Please edit $INSTALL_DIR/.env to configure your settings${NC}"
fi

echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Next steps:"
echo "1. Restart Claude Desktop"
echo "2. The YouTube Transcript tools will be available"
echo ""
echo "Example usage:"
echo "- 'Get transcript for https://youtube.com/watch?v=VIDEO_ID'"
echo "- 'List available languages for VIDEO_ID'"
echo "- 'Translate transcript to Japanese for VIDEO_ID'"
echo ""
echo -e "${GREEN}Enjoy using YouTube Transcript MCP Server!${NC}"