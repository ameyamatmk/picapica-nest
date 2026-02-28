# picapica-nest

日常の記録・整理・振り返りを行う個人用 AI エージェント。

Discord 経由で会話し、会話ログを自動記録・定期的に振り返り（hindsight）して活用する。

## 技術スタック

- **言語**: Go 1.25+
- **チャットエンジン**: [PicoClaw](https://github.com/sipeed/picoclaw) (`pkg/` を Go ライブラリとして import)
- **メッセージング**: Discord
- **LLM**: Anthropic Claude (Sonnet 4)
- **Web コンソール**: Go `html/template` + HTMX + Pico CSS

## リポジトリ構成

```
picapica-nest/
├── cmd/picapica-nest/    # エントリーポイント (serve / hindsight コマンド)
├── internal/
│   ├── applog/           # 構造化アプリケーションログ
│   ├── console/          # Web コンソール (HTMX + Pico CSS)
│   ├── hindsight/        # 振り返りパイプライン前処理
│   ├── logging/          # 会話ログ記録 (MessageBus 購読)
│   ├── provider/         # LLM Provider wrapper (Logging, PromptRewrite)
│   └── session/          # セッション管理 (Idle timeout)
├── workspace/            # PicoClaw ワークスペース
│   ├── SOUL.md           # エージェントの personality
│   ├── AGENTS.md         # エージェント定義
│   ├── memory/           # Hindsight 結果 (daily/weekly/monthly)
│   ├── prompts/          # Hindsight Prompt テンプレート
│   └── logs/             # 会話ログ (JSONL)
├── deploy/               # デプロイ設定 (systemd, スクリプト)
├── docs/                 # 設計書
└── Makefile
```

## セットアップ

### 前提条件

- Go 1.25 以上
- Anthropic API Key
- Discord Bot Token

### 設定ファイル

`~/.picapica-nest/config.json` を作成する。サンプル: [`deploy/config.example.json`](deploy/config.example.json)

```json
{
  "agents": {
    "defaults": {
      "workspace": "/path/to/workspace",
      "provider": "anthropic",
      "model": "claude-sonnet-4-20250514"
    }
  },
  "providers": {
    "anthropic": {}
  },
  "channels": {
    "discord": {
      "enabled": true
    }
  },
  "gateway": {
    "host": "0.0.0.0",
    "port": 8080
  }
}
```

Anthropic API Key は環境変数 `ANTHROPIC_API_KEY` で渡すか、`providers.anthropic.api_key` に設定する。
Discord Bot Token は環境変数または `channels.discord.token` に設定する。

## コマンド

### 開発

```bash
make build          # ローカルビルド → build/picapica-nest
make build-all      # クロスコンパイル (linux/amd64, linux/arm64)
make clean          # ビルド成果物を削除
go test ./...       # 全テスト実行
go vet ./...        # 静的解析
```

### 実行

```bash
# メインサーバー起動 (Discord Bot + Web コンソール)
picapica-nest serve

# Hindsight の手動実行
picapica-nest hindsight daily   [-date YYYY-MM-DD]
picapica-nest hindsight weekly  [-week YYYY-WNN]
picapica-nest hindsight monthly [-month YYYY-MM]
```

## サーバーポート

| サーバー | ポート | 備考 |
|---------|--------|------|
| Gateway (PicoClaw) | 8080 | `config.json` の `gateway.port` で変更可 |
| Web コンソール | 19100 | 固定 |

## Web コンソール

`http://localhost:19100` でアクセス。

| 画面 | パス | 内容 |
|------|------|------|
| ダッシュボード | `/dashboard` | Hindsight レポート概要、Usage サマリ |
| Hindsight | `/hindsight` | 日次（カレンダー）/ 週次 / 月次レポート閲覧 |
| 会話ログ | `/conversations` | チャンネル別の会話ログ閲覧 |
| ワークスペース | `/workspace` | Markdown ファイルの閲覧 |
| Usage | `/usage` | API 呼び出し回数・トークン数の日別集計 |
| アプリログ | `/logs` | アプリケーションログ（レベル・コンポーネントでフィルタ） |

## デプロイ

デプロイ先: ホームラボ VM (`picapica-vm`)

```bash
# 初回デプロイ
sudo bash deploy/deploy.sh

# アップデート (GitHub Release から最新版を取得して systemd 再起動)
sudo bash deploy/update.sh [VERSION]
```

systemd service: [`deploy/picapica-nest.service`](deploy/picapica-nest.service)

## アーキテクチャ

- PicoClaw 本体の改造はゼロを原則とし、拡張は Provider wrapper / カスタムツール / ワークスペースファイルで実現
- Provider chain: `PromptRewrite → Logging → Anthropic`
- Message Bus: channelBus（Discord 側）と agentBus（Agent 側）を ConversationLogger が Bridge
- Hindsight: 会話ログを日次→週次→月次で段階的に振り返り、`memory/` に保存
