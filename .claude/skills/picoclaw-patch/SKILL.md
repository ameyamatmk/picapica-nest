---
name: picoclaw-patch
description: PicoClaw fork (picoclaw-nest) へのパッチ作成・更新を行う
user-invocable: true
argument-hint: [apply | update <version>]
allowed-tools:
  - Bash
  - Read
  - Edit
  - Write
  - Glob
  - Grep
---

# PicoClaw Fork パッチスキル

PicoClaw の fork リポジトリ `ameyamatmk/picoclaw-nest` に対してパッチを作成・更新するスキル。

## リポジトリ構成

- **upstream**: `sipeed/picoclaw` (remote: `origin`)
- **fork**: `ameyamatmk/picoclaw-nest` (remote: `fork`)
- **ローカルパス**: `~/devel/picoclaw/`

## ブランチ運用

| ブランチ | 役割 | 追従先 |
|---------|------|--------|
| `main` | upstream 追従用（変更しない） | `origin/main` |
| `nest` | fork のデフォルトブランチ。upstream のリリースタグと同じ位置に置く | `v0.x.x` タグ |
| `feat/*` | パッチ用フィーチャーブランチ。`nest` から分岐し `nest` に PR を出す | - |

## タグ規則

- upstream タグ: `v0.2.0`
- nest タグ: `v0.2.0-nest.1`, `v0.2.0-nest.2`, ...
- picapica-nest の `go.mod` は nest タグを `replace` ディレクティブで参照

## 作業フロー

### 1. 新しい upstream バージョンへの追従

```bash
cd ~/devel/picoclaw/

# upstream の最新を取得
git fetch origin
git checkout main
git merge origin/main --ff-only

# nest ブランチを新しいバージョンに移動
git checkout nest
git reset --hard v0.x.x  # 新しい upstream タグ
git push fork nest --force

# フィーチャーブランチでパッチを適用
git checkout -b feat/nest-patches nest
# ... パッチ適用 ...
git push fork feat/nest-patches

# nest に PR を作成
gh-with-token pr create --repo ameyamatmk/picoclaw-nest \
  --base nest --head feat/nest-patches \
  --title "feat: vX.Y.Z ベースの nest パッチ適用"
```

### 2. 既存バージョンへのパッチ追加

```bash
cd ~/devel/picoclaw/

# nest から分岐
git checkout -b feat/some-feature nest
# ... 変更 ...
git push fork feat/some-feature

# nest に PR を作成
gh-with-token pr create --repo ameyamatmk/picoclaw-nest \
  --base nest --head feat/some-feature \
  --title "feat: 機能の説明"
```

### 3. マージ後のタグ打ち

```bash
# PR マージ後
git fetch fork nest
git checkout nest
git merge fork/nest --ff-only

# タグ打ち & push
git tag v0.x.x-nest.N
git push fork v0.x.x-nest.N

# picapica-nest の go.mod 更新
# replace github.com/sipeed/picoclaw => github.com/ameyamatmk/picoclaw-nest v0.x.x-nest.N
```

## 現行パッチ内容

nest パッチは以下の機能を提供する（upstream に無い機能）:

1. **中間コンテンツ配信**: ツールコール付き LLM レスポンスの Content を即座に PublishOutbound で配信
2. **DefaultResponse 設定可能化**: `config.AgentDefaults.DefaultResponse` フィールド
3. **セッション履歴の単純ドロップ**: LLM ベースの summarizeSession を削除し、MessageThreshold 超過時に KeepLastMessages 件を残してドロップ
4. **追加 config フィールド**: `DefaultResponse`, `MessageThreshold`, `KeepLastMessages`, `IdleTimeoutMinutes`

## 変更対象ファイル

- `pkg/config/config.go` — AgentDefaults フィールド追加
- `pkg/config/defaults.go` — デフォルト値
- `pkg/agent/loop.go` — 中間コンテンツ配信、ドロップ方式要約、DefaultResponse
- `pkg/agent/loop_test.go` — テスト

## 実行ルール

1. `$ARGUMENTS` を確認する
2. 引数が空または `apply` の場合、現在の nest パッチ内容を確認し、必要に応じて更新する
3. 引数が `update <version>` の場合、指定バージョンへの追従作業を行う
4. 作業は必ずフィーチャーブランチで行い、`nest` に直接コミットしない
5. テスト (`go test ./pkg/agent/`) を実行して PASS を確認する
6. PR 作成時は `--base nest` を指定する
7. GitHub CLI は `gh-with-token` を使う
