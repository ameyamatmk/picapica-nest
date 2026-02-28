# picapica-nest

日常の記録・整理・振り返りを行う個人用 AI エージェント。

## 技術スタック

- 言語: Go
- チャットエンジン: [PicoClaw](https://github.com/sipeed/picoclaw) (`pkg/` を Go ライブラリとして import)
- メッセージング: Discord
- PicoClaw ローカルコピー: `../picoclaw/`

## リポジトリ構成

```
picapica-nest/
├── cmd/picapica-nest/    # カスタムゲートウェイ (PicoClaw pkg/ を import)
├── internal/             # 内部パッケージ
│   ├── provider/         # カスタム Provider (Logging, PromptRewrite)
│   ├── session/          # セッション管理 (idle timeout)
│   ├── logging/          # 会話ログ (Message Bus 購読)
│   └── hindsight/        # 振り返りパイプライン前処理
├── workspace/            # PicoClaw ワークスペース
│   ├── SOUL.md
│   ├── AGENT.md
│   └── memory/
├── docs/                 # 設計書・ドキュメント
├── go.mod
└── go.sum
```

## 設計ドキュメント

- 基本設計書: `docs/00_personal_agent_basic_design.md`

## 開発方針

- PicoClaw 本体の改造はゼロを原則とする
- 拡張は Provider wrapper、カスタムツール登録、ワークスペースファイル、外部プロセスで実現
- PicoClaw 本体の改造が必要な場合は `go.mod` の `replace` ディレクティブで対応

## Go コマンド

- ビルド: `go build ./cmd/picapica-nest/`
- テスト: `go test ./...`
- リント: `go vet ./...`

## デプロイ

- デプロイ先: ホームラボ VM
- SSH ホスト名: `picapica-vm`
- 例: `ssh picapica-vm`

## GitHub CLI

- GitHub への操作（PR, Issue 等）は `gh` の代わりに `gh-with-token` を使う
- エージェント用の GitHub App トークンを自動取得して実行するラッパー
- 例: `gh-with-token pr create ...`, `gh-with-token issue view 1`
