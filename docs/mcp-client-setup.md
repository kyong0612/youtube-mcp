# MCP Client Setup Guide

このドキュメントでは、YouTube Transcript MCP ServerをClaude Desktop、Claude Code、CursorなどのMCPクライアントから使用する方法を説明します。

## Claude Desktop での設定

### 1. 設定ファイルの場所

Claude Desktopの設定ファイルは以下の場所にあります：

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

### 2. 設定方法

#### 方法1: Go実行環境がある場合

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "go",
      "args": ["run", "/path/to/youtube-mcp/cmd/server/main.go"],
      "env": {
        "PORT": "8080",
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true",
        "YOUTUBE_DEFAULT_LANGUAGES": "en,ja,es,fr,de"
      }
    }
  }
}
```

#### 方法2: ビルド済みバイナリを使用する場合

まず、バイナリをビルドします：

```bash
cd /path/to/youtube-mcp
make build
```

その後、設定ファイルに追加：

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-transcript-mcp",
      "env": {
        "PORT": "8080",
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true",
        "YOUTUBE_DEFAULT_LANGUAGES": "en,ja"
      }
    }
  }
}
```

#### 方法3: Dockerを使用する場合

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "-p", "8080:8080",
        "--env-file", "/path/to/youtube-mcp/.env",
        "youtube-transcript-mcp:latest"
      ]
    }
  }
}
```

### 3. 環境変数の設定

重要な環境変数：

- `YOUTUBE_DEFAULT_LANGUAGES`: デフォルトの字幕言語（カンマ区切り）
- `CACHE_ENABLED`: キャッシュの有効/無効
- `LOG_LEVEL`: ログレベル（debug, info, warn, error）
- `YOUTUBE_REQUEST_TIMEOUT`: リクエストタイムアウト
- `YOUTUBE_RATE_LIMIT_PER_MINUTE`: 分あたりのリクエスト上限

### 4. 高度な設定

プロキシを使用する場合：

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-transcript-mcp",
      "env": {
        "YOUTUBE_PROXY_URL": "http://proxy.example.com:8080",
        "YOUTUBE_ENABLE_PROXY_ROTATION": "true",
        "YOUTUBE_PROXY_LIST": "http://proxy1.com:8080,http://proxy2.com:8080"
      }
    }
  }
}
```

認証を有効にする場合：

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-transcript-mcp",
      "env": {
        "SECURITY_ENABLE_AUTH": "true",
        "SECURITY_API_KEYS": "your-secret-api-key-here"
      }
    }
  }
}
```

## Claude Code での設定

Claude Code（claude.ai/code）はMCPサーバーを自動的に検出します。

### 1. 設定方法

#### 方法1: Go実行環境を使用

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "go",
      "args": ["run", "/path/to/youtube-mcp/cmd/server/main.go"],
      "env": {
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true",
        "YOUTUBE_DEFAULT_LANGUAGES": "en,ja"
      }
    }
  }
}
```

#### 方法2: コンパイル済みバイナリを使用

```bash
# まずバイナリをビルド
cd /path/to/youtube-mcp
make build
```

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-transcript-mcp",
      "env": {
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true"
      }
    }
  }
}
```

### 2. Claude Codeの特徴

- MCPサーバーの自動検出と統合
- リアルタイムでの字幕取得と処理
- 複数の動画を並行処理可能

## Cursor での設定

CursorはMCPサーバーをサポートしています。

### 1. 設定方法

1. Cursorの設定を開く（macOS: `Cmd+,`、Windows/Linux: `Ctrl+,`）
2. "MCP"または"Model Context Protocol"を検索
3. 以下の設定を追加：

#### Go実行環境を使用する場合

```json
{
  "mcp.servers": {
    "youtube-transcript": {
      "command": "go",
      "args": ["run", "/path/to/youtube-mcp/cmd/server/main.go"],
      "env": {
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true",
        "YOUTUBE_DEFAULT_LANGUAGES": "en,ja"
      }
    }
  }
}
```

#### コンパイル済みバイナリを使用する場合

```json
{
  "mcp.servers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-transcript-mcp",
      "env": {
        "LOG_LEVEL": "info",
        "CACHE_ENABLED": "true"
      }
    }
  }
}
```

### 2. Cursorでの活用例

- コードコメントに動画の内容を自動追加
- チュートリアル動画からコードスニペットを抽出
- 技術解説動画の要約をドキュメントに反映

## 使用方法

MCPクライアント（Claude Desktop、Claude Code、Cursor）を再起動後、以下のツールが利用可能になります：

### 1. 動画の字幕を取得

```text
YouTubeの動画 https://www.youtube.com/watch?v=VIDEO_ID の字幕を日本語で取得してください
```

### 2. 複数動画の字幕を一括取得

```text
以下の動画の字幕をすべて取得してください：
- https://www.youtube.com/watch?v=VIDEO_ID1
- https://www.youtube.com/watch?v=VIDEO_ID2
```

### 3. 字幕を翻訳

```text
この動画の字幕を英語から日本語に翻訳してください：
https://www.youtube.com/watch?v=VIDEO_ID
```

### 4. 利用可能な言語を確認

```text
この動画で利用可能な字幕言語を教えてください：
https://www.youtube.com/watch?v=VIDEO_ID
```

### 5. 字幕をSRT形式で取得

```text
この動画の字幕をSRT形式で取得してください：
https://www.youtube.com/watch?v=VIDEO_ID
```

## トラブルシューティング

### サーバーが起動しない

1. Goがインストールされているか確認：
   ```bash
   go version
   ```

2. 依存関係がインストールされているか確認：
   ```bash
   cd /path/to/youtube-mcp
   make deps
   ```

3. ポートが使用されていないか確認：
   ```bash
   lsof -i :8080
   ```

### 字幕が取得できない

1. 動画に字幕が存在するか確認
2. プライベート動画や地域制限がないか確認
3. レート制限に引っかかっていないか確認

### ログの確認

デバッグログを有効にする：

```json
{
  "mcpServers": {
    "youtube-transcript": {
      "command": "/path/to/youtube-mcp/youtube-transcript-mcp",
      "env": {
        "LOG_LEVEL": "debug"
      }
    }
  }
}
```

## セキュリティに関する注意

- APIキーを使用する場合は、環境変数に直接記載せず、`.env`ファイルを使用することを推奨
- プロキシを使用する場合は、信頼できるプロキシサービスを使用すること
- YouTubeの利用規約を遵守すること

## サポート

問題が発生した場合は、以下を確認してください：

1. [README.md](../README.md) - 基本的な使用方法
2. [GitHub Issues](https://github.com/yourusername/youtube-transcript-mcp/issues) - 既知の問題
3. ログファイル - デバッグ情報の確認
