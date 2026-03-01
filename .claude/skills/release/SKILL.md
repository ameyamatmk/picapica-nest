---
name: release
description: picapica-nest のリリース（タグ打ち → push → CI でビルド・GitHub Release 作成）
user-invocable: true
argument-hint: <version> (e.g. v0.2.1)
allowed-tools:
  - Bash
  - Read
  - Grep
---

# リリーススキル

picapica-nest のリリースを行う。タグを打って push すると CI（`.github/workflows/release.yml`）がビルド・GitHub Release 作成を自動実行する。

## 実行フロー

1. `$ARGUMENTS` からバージョンを取得する（例: `v0.2.1`）
2. 引数が空の場合、直近のタグを確認して次のバージョンをユーザーに確認する
3. バージョンが `v` で始まり semver 形式であることを検証する
4. `git tag <version>` でタグを作成する
5. `git push origin <version>` でタグを push する
6. CI の実行状況を `gh-with-token run list --repo ameyamatmk/picapica-nest --limit 1` で確認する
7. CI の URL をユーザーに伝える

## 注意事項

- タグは必ず main ブランチの HEAD に打つ。別ブランチにいる場合はユーザーに確認する
- GitHub CLI は `gh-with-token` を使う
- 既に存在するタグを上書きしない。重複する場合はユーザーに報告する
