# CI/CD クイックリファレンス

## 🚀 基本的な使い方

### ローカル開発

```bash
# 開発環境のセットアップ
make deps        # 依存関係とツールのインストール
make dev         # ホットリロード付き開発サーバー起動

# コード品質チェック
make fmt         # コードフォーマット
make lint        # リンターの実行
make test        # テストの実行
make security    # セキュリティスキャン

# ビルドと実行
make build       # バイナリビルド
make run         # アプリケーション実行
make docker-build # Dockerイメージビルド
```

### CI/CDワークフロー

```bash
# プルリクエスト作成時に自動実行
- 依存関係チェック
- 静的解析（50+リンター）
- セキュリティスキャン
- マルチプラットフォームテスト
- 統合テスト
- ビルド検証

# リリース作成（v*タグプッシュ時）
git tag v1.2.3
git push origin v1.2.3
```

## 📋 Makefileターゲット一覧

### 開発ツール
| コマンド | 説明 |
|---------|------|
| `make tools` | すべての開発ツールをインストール |
| `make tools-update` | ツールを最新版に更新 |
| `make tools-verify` | ツールのインストール確認 |
| `make tools-clean` | ツールをクリーンアップ |

### テスト
| コマンド | 説明 |
|---------|------|
| `make test` | 単体テストを実行 |
| `make test-short` | 短時間テストのみ実行 |
| `make test-coverage` | カバレッジレポート生成 |
| `make test-integration` | 統合テストを実行 |
| `make benchmark` | ベンチマークテスト |

### コード品質
| コマンド | 説明 |
|---------|------|
| `make lint` | golangci-lintを実行 |
| `make fmt` | コードフォーマット |
| `make vet` | go vetを実行 |
| `make security` | セキュリティスキャン |
| `make check` | すべてのチェックを実行 |

### ビルド・デプロイ
| コマンド | 説明 |
|---------|------|
| `make build` | バイナリをビルド |
| `make build-all` | 全プラットフォーム向けビルド |
| `make docker-build` | Dockerイメージビルド |
| `make release` | リリースパッケージ作成 |

### Docker Compose
| コマンド | 説明 |
|---------|------|
| `make up` | サービスを起動 |
| `make down` | サービスを停止 |
| `make logs` | ログを表示 |
| `make up-redis` | Redis付きで起動 |
| `make up-monitoring` | 監視ツール付きで起動 |

### MCP テスト
| コマンド | 説明 |
|---------|------|
| `make test-mcp-init` | MCP初期化テスト |
| `make test-mcp-tools` | MCPツール一覧テスト |
| `make test-transcript` | トランスクリプト取得テスト |

## 🛠️ ツール一覧

Go 1.24の`tool`ディレクティブで管理されているツール：

| ツール | 用途 |
|--------|------|
| `golangci-lint` | 統合リンター（50+のリンター） |
| `gosec` | セキュリティ脆弱性スキャナー |
| `goimports` | インポート文の整形 |
| `air` | ホットリロード開発サーバー |
| `git-chglog` | 変更履歴生成 |
| `go-licenses` | ライセンスチェック |
| `ineffassign` | 無効な代入検出 |
| `revive` | 拡張可能なGoリンター |
| `godoc` | ドキュメント生成 |
| `govulncheck` | 脆弱性チェック |
| `staticcheck` | 高度な静的解析 |
| `nancy` | 依存関係の脆弱性スキャン |

## 📊 CI/CDパイプライン構成

```
┌─────────────────┐
│ dependency-check│
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌────────┐ ┌────────────┐ ┌───────────┐
│  test  │ │static-analysis│ │security-scan│
└────┬───┘ └──────┬──────┘ └─────┬─────┘
     │            │               │
     └────┬───────┴───────────────┘
          ▼
    ┌─────────────┐
    │integration-test│
    └──────┬──────┘
           │
    ┌──────┴──────┐
    ▼             ▼
┌───────┐    ┌────────┐
│ build │    │ docker │
└───┬───┘    └────┬───┘
    │             │
    └──────┬──────┘
           ▼
      ┌────────┐
      │release │
      └────────┘
```

## 🔧 トラブルシューティング

### よくあるエラーと対処法

#### ツールが見つからない
```bash
# ツールを再インストール
make tools-clean
make tools
```

#### リンターのタイムアウト
```bash
# タイムアウトを延長して実行
golangci-lint run --timeout=15m
```

#### テストの失敗
```bash
# 詳細なログで実行
go test -v -race ./...
```

#### Dockerビルドエラー
```bash
# キャッシュをクリアして再ビルド
docker system prune -a
make docker-build
```

## 🏷️ リリース手順

1. **バージョンタグを作成**
   ```bash
   git tag -a v1.2.3 -m "Release version 1.2.3"
   git push origin v1.2.3
   ```

2. **自動実行される処理**
   - マルチプラットフォームビルド
   - Dockerイメージ作成・プッシュ
   - リリースノート生成
   - GitHub Release作成
   - チェックサム生成

3. **リリース後の確認**
   - GitHub Releasesページで確認
   - Docker Hubでイメージ確認
   - ダウンロードリンクの動作確認

## 📝 コミットメッセージ規約

```bash
# 機能追加
git commit -m "feat: YouTube APIの新しいエンドポイントを追加"

# バグ修正
git commit -m "fix: トランスクリプト取得時のエラーを修正"

# ドキュメント
git commit -m "docs: READMEに使用例を追加"

# リファクタリング
git commit -m "refactor: キャッシュロジックを最適化"

# テスト
git commit -m "test: YouTubeサービスのテストカバレッジを向上"

# その他
git commit -m "chore: 依存関係を更新"
```

## 🔐 セキュリティ

### シークレット管理
- GitHub Secretsで管理
- 環境変数で参照
- ログに出力しない

### 必要なシークレット
| シークレット | 説明 |
|-------------|------|
| `CODECOV_TOKEN` | Codecovアップロード用 |
| `GITHUB_TOKEN` | 自動で提供される |

### セキュリティスキャン
- Gosec: Goコードの脆弱性
- Trivy: コンテナイメージの脆弱性
- CodeQL: セマンティック分析
- Dependabot: 依存関係の更新