# picapica-nest 基本設計書

> プロジェクト名: **picapica-nest**
> 作成日: 2026-02-26
> ステータス: Draft


---

## 1. 目的

日々の出来事を集約・整理し、振り返りやインサイトを得る手助けをする個人用 AI エージェントを構築する。

主な価値:
- 会話を通じた日常の記録・整理
- セッションログから日次→週次→月次への段階的 Hindsight
- Hindsight で得た知識の自動コンテキスト反映
- 利用状況の可視化・監視

---

## 2. 技術方針

### 2.1 チャットエンジン

**PicoClaw** を採用する。

| 選定理由 | 詳細 |
|---------|------|
| 軽量性 | RAM <10MB、起動 <1s、シングルバイナリ |
| 拡張性 | `pkg/` を Go ライブラリとして import し、独自ゲートウェイを構成可能 |
| Fork 不要 | Provider wrapper パターンにより本体改造ゼロで拡張できる |
| ファイルベース | ほぼ全永続データがファイル。外部プロセスからの読み書きが容易 |

詳細な比較はエンジン選定時の調査資料を参照。

### 2.2 リポジトリ構成

Go module 依存（A方式）を基本とする。

```
picapica-nest/
├── cmd/
│   └── picapica-nest/
│       └── main.go          # カスタムゲートウェイ (PicoClaw pkg/ を import)
├── internal/
│   ├── provider/             # カスタム Provider (Logging, PromptRewrite)
│   └── hindsight/            # 振り返りパイプライン前処理
├── workspace/                # PicoClaw ワークスペース
│   ├── SOUL.md
│   ├── AGENTS.md
│   ├── MEMORY.md
│   └── memory/
├── go.mod                    # require github.com/sipeed/picoclaw
└── go.sum
```

PicoClaw 本体のソース修正が必要になった場合は `go.mod` の `replace` ディレクティブでローカルコピーに切り替え可能。

### 2.3 メッセージングチャンネル

**Discord のみ**。PicoClaw の Discord チャンネルをそのまま利用する。
複数チャンネルに対応し、チャンネル ID 単位でセッションを分離する。ペルソナは全チャンネル共通。

将来的にはチャンネルごとに Completion の方向性（トーン、応答スタイル等）をチューニング可能にすることを想定するが、初期フェーズのスコープ外とする。

---

## 3. 機能一覧

| ID | 機能 | Phase | 概要 |
|----|------|-------|------|
| F-01 | カスタムゲートウェイ | P0 | PicoClaw `pkg/` を import した独自 `cmd/` |
| F-02 | Discord 連携 | P0 | PicoClaw 標準の Discord チャンネルで会話 |
| F-03 | ペルソナ設定 | P0 | SOUL.md / AGENTS.md / MEMORY.md の初期設計 |
| F-13 | セッション idle timeout | P0 | 一定時間無活動でセッションをクリア（トークン節約） |
| F-14 | 会話ログ | P0 | Message Bus 購読による append-only の永続会話ログ |
| F-04 | Logging Provider | P1 | API usage / コスト / レイテンシをファイル出力 |
| F-07 | Prompt Rewrite Provider | P1 | 動的コンテキストの末尾追加 |
| F-05 | 日次 Hindsight | P2 | セッションログ → 前処理 → LLM → 日次レポート |
| F-08 | 週次・月次 Hindsight | P2 | 日次 → 週次 → 月次の段階的集約 |
| F-06 | MEMORY.md 自動更新 | P2 | Hindsight 結果を「直近セクション」に反映 |
| F-09 | MCP サーバー | P2 | 参照カウント付きメモリ検索 |
| F-10 | Web コンソール | P3 | ファイル watch ベースの監視・可視化 UI |

F-11 (API ブリッジ), F-12 (参照カウントによる自動昇格) は P4 以降で検討。

---

## 4. フェーズ定義

### Phase 0: 最小構成 — 「会話できる」

**ゴール**: PicoClaw をほぼ素のまま Discord 経由で動かす。会話ログとセッション管理も組み込む。

**スコープ**:
- F-01: PicoClaw `pkg/` を import したカスタムゲートウェイの最小実装
- F-02: Discord Bot の接続・メッセージ送受信
- F-03: ワークスペースファイル (SOUL.md, AGENTS.md, MEMORY.md) の初期内容作成
- F-14: 会話ログ（Message Bus 購読）
- F-13: セッション idle timeout

#### F-14: 会話ログの設計

PicoClaw のセッション管理は LLM のコンテキストウィンドウ管理が目的であり、要約・圧縮で過去のメッセージが失われる。
Hindsight パイプライン（P2）や Web コンソール（P3）で完全な会話履歴が必要なため、
**PicoClaw のセッションとは独立した永続会話ログ**を Message Bus 購読で実現する。

```
Message Bus
  ├── Agent Loop（PicoClaw 標準）  → セッション管理（揮発的、要約で消える）
  └── Conversation Logger（追加） → 会話ログ（永続的、追記のみ）
```

ログ形式: 日付単位の JSONL ファイル（append-only）

```
logs/
└── discord_12345/
    ├── 2026-02-25.jsonl
    └── 2026-02-26.jsonl
```

```jsonl
{"ts":"2026-02-26T10:23:00Z","dir":"in","sender":"user#1234","content":"今日の会議どうだった？"}
{"ts":"2026-02-26T10:23:05Z","dir":"out","content":"会議の件だね！..."}
```

用途:
- **P2 Hindsight パイプライン**: `logs/*/*.jsonl` を日付で集めて前処理 → LLM → 日次レポート
- **P3 Web コンソール**: 過去の会話履歴の完全な閲覧
- **デバッグ**: エージェントの応答確認

#### F-13: セッション idle timeout の設計

PicoClaw 標準にはセッション自動リセット機能がないため、カスタムゲートウェイで実装する。
目的は**トークン消費の抑制**のみ。会話の永続化は F-14 が担うため、アーカイブの責務は持たない。

```
バックグラウンド goroutine (定期チェック)
  │
  ├── 各セッションの最終活動時刻を確認
  ├── idleTimeout 超過？
  │     ├── Yes → セッション情報（履歴・サマリー）をクリアし、永続化
  │     └── No  → スキップ
```

セッションキーは Discord チャンネル単位 (`discord:{chatID}`) で管理する。
クリア処理では、インメモリ状態とファイルの整合性を保証すること（永続化失敗時にインメモリだけクリアされない等）。

#### P0 の責務分離

```
PicoClaw セッション  →  LLM が「今の会話」を覚えておくため（揮発的）
                        要約・圧縮は PicoClaw に任せる
                        idle timeout でトークン消費を抑制

独自会話ログ (F-14)  →  Hindsight・Web コンソール・振り返りのため（永続的）
                        PicoClaw の要約に影響されない
                        append-only で完全な履歴を保持
```

**カスタムコード量**: 150-200行（wiring + 会話ログ + idle timeout）

**成果物**:
```
cmd/picapica-nest/main.go         # PicoClaw 起動 + Discord チャンネル登録
internal/session/idle.go          # idle timeout ロジック
internal/logging/conversation.go  # Message Bus 購読の会話ログ
workspace/SOUL.md                 # ペルソナ定義
workspace/AGENTS.md               # 行動指針
workspace/MEMORY.md               # 初期メモリ（空 or 基本情報）
```

### Phase 1: カスタム Provider — 「計測と制御」

**ゴール**: Provider wrapper で LLM 呼び出しに介入できる基盤を作る。

**スコープ**:
- F-04: Logging Provider
  - リクエスト/レスポンスの usage (トークン数), レイテンシ, モデル名, エラー有無
  - `usage.jsonl` にファイル出力
- F-07: Prompt Rewrite Provider
  - system prompt 末尾に動的セクションを追加
  - 初期実装: 時刻情報、直近メモリのサマリ等

**アーキテクチャ**:
```
Discord ──→ Message Bus ──→ Agent Loop ──→ [PromptRewrite] ──→ [Logging] ──→ Inner LLM Provider
                                               │ 末尾追加           │ usage記録
                                               │                    │
                                          workspace/            usage.jsonl
                                          MEMORY.md 参照
```

Provider は Decorator パターンで連鎖する:
```go
inner    := providers.NewOpenAIProvider(cfg)
logging  := NewLoggingProvider(inner, usageLogger)
rewriter := NewPromptRewriteProvider(logging, rewriteConfig)
// rewriter → logging → inner の順で実行
```

**カスタムコード量**: 各 Provider 100行前後

### Phase 2: Hindsight パイプライン — 「記憶の整理」

**ゴール**: セッションログを段階的に振り返り、長期メモリを構築する。

**スコープ**:
- F-05: 日次 Hindsight
- F-08: 週次・月次 Hindsight
- F-06: MEMORY.md 自動更新
- F-09: MCP サーバー（参照カウント付き検索）

**Hindsight パイプラインの構成**:

P0 で作成される会話ログ（`logs/*/*.jsonl`）が Hindsight の入力になる。
会話ログは既にユーザー/アシスタントのメッセージのみで、system prompt や tool calls を含まないため、
前処理は日付単位の結合とフォーマット変換のみで済む。

**LLM の利用方式**: Hindsight 処理は **Claude Code CLI (`claude --print`)** 経由で LLM を利用する。
picapica-nest から直接 LLM API を呼び出すことはせず、CLI 呼び出しに委譲することで
API キー管理やモデル選択を Claude Code 側に一元化する。

```
logs/discord_12345/2026-02-26.jsonl  ──→ [前処理] ──→ Clean transcript ──→ [claude --print] ──→ Hindsight レポート
                                            │
                                            ├── 日付単位で JSONL を集約
                                            ├── Markdown 形式に変換
                                            └── メタデータ（メッセージ数、時間帯）付与
```

**実行トリガー**: picapica-nest 自身が持つスケジューラ（内蔵 cron）でトリガーする。
PicoClaw の Cron 機能（エージェントとしての実行）ではなく、アプリケーションレベルのスケジューラとして実装する。

Hindsight の階層構造:

| レベル | 入力 | 出力 | 頻度 | 処理内容 |
|--------|------|------|------|----------|
| 日次 | 会話ログ JSONL（当日分） | `memory/daily/YYYY-MM-DD.md` | 毎日 | メッセージの収集・要約・情報整理 |
| 週次 | 直近7日の日次レポート | `memory/weekly/YYYY-WNN.md` | 毎週 | 重要度に基づく情報の選別・整理 |
| 月次 | 当月の週次レポート | `memory/monthly/YYYY-MM.md` | 毎月 | 俯瞰的な整理、MEMORY.md へのフィードバック |

Hindsight 結果の反映先:
- 日次レポート → PicoClaw の日次ノート (`YYYYMM/YYYYMMDD.md`) として自動コンテキストロード
- 最新サマリ → MEMORY.md の「直近の出来事」セクションを更新
- 恒久的な知識 → MEMORY.md の「恒久的な知識」セクションに昇格（将来的には自動化 F-12）

**MCP サーバー**:
- メモリファイルの検索 API
- 参照カウントのトラッキング（検索ヒット時にカウント++）
- メタデータ管理 (`metadata.json`)

**実装言語**: 未定（Go 統一 or Python を検討）

### Phase 3: Web コンソール — 「可視化」

**ゴール**: エージェントの動作状況をブラウザで確認できるようにする。

**スコープ**:
- F-10: ファイル watch ベースの監視 UI

**方針**: PicoClaw プロセスとは別プロセスで構築する。
PicoClaw のほぼ全永続データがファイルベースのため、ファイル watch + 静的 HTTP サーバーで完結可能。

**表示項目**:

| 情報 | ソース | 取得方法 |
|------|--------|---------|
| 会話履歴（完全） | `logs/*/*.jsonl` (P0 で作成) | ファイル読み取り |
| メモリファイル | `memory/*.md`, `MEMORY.md` | ファイル読み取り |
| Usage / コスト | `usage.jsonl` (P1 で作成) | ファイル読み取り |
| Cron ジョブ一覧 | `cron/jobs.json` | ファイル読み取り |
| ワークスペースファイル | `SOUL.md`, `AGENTS.md` 等 | ファイル読み取り |
| Hindsight レポート | `memory/daily/`, `memory/weekly/` | ファイル読み取り |

**取得できないもの** (ランタイム状態):
- チャンネル接続状態
- 処理中リクエスト
- LLM リクエスト進行中フラグ

これらはログからの推測、またはヘルスチェックエンドポイントで代替する。

**技術選定**: 未定（フロントエンド技術は P3 着手時に検討）

---

## 5. 横断的な設計方針

### 5.1 プロンプト制御の2段構え

```
[静的な指示]  SOUL.md / AGENTS.md       ← PicoClaw 標準の仕組みで読み込み
                                          Hindsight パイプラインから定期更新も可能

[動的な指示]  Prompt Rewrite Provider    ← 実行時に末尾追加
              ├── ## Priority Override   （最優先指示）
              ├── ## Recent Context      （直近N日のサマリ）
              └── ## Current Situation   （時間帯・状況）
```

LLM は後方の指示を重視する傾向があるため、既存プロンプトの解析不要で末尾追加だけで有効な制御が可能。

### 5.2 MEMORY.md の2層構造

```markdown
# MEMORY.md

## 恒久的な知識
- ユーザーの好み、方針、パターン
- 確立されたワークフロー

## 直近の出来事（Hindsight パイプラインが自動更新）
- 2/25: ○○について調査、△△が判明
- 2/24: □□の設定を変更
```

### 5.3 PicoClaw 本体の改造方針

**原則: 改造ゼロ**。全ての拡張を以下の手段で実現する:
- Provider wrapper (Logging, PromptRewrite)
- カスタムツール登録 (`RegisterTool()`)
- ワークスペースファイル (SOUL.md, AGENTS.md 等)
- 外部プロセス (Hindsight パイプライン, Web コンソール, MCP サーバー)

改造が必要になった場合は `go.mod` の `replace` ディレクティブでローカルコピーに切り替え。

---

## 6. 未決事項

| # | 項目 | 関連 Phase | メモ |
|---|------|-----------|------|
| TBD-01 | Hindsight パイプラインの実装言語 | P2 | Go 統一 vs Python。テキスト処理の書きやすさと依存管理のトレードオフ |
| TBD-02 | Web コンソールの技術スタック | P3 | フロントエンド技術、サーバー側の言語 |
| TBD-03 | MCP サーバーの実装言語 | P2 | Hindsight パイプラインと同一言語が望ましい |
| ~~TBD-04~~ | ~~Hindsight の実行方式~~ | P2 | **解決済み**: picapica-nest 内蔵スケジューラでトリガー。LLM は Claude Code CLI (`--print`) 経由で利用 |
| TBD-05 | F-11 API ブリッジの要否 | P4+ | OpenClaw エコシステムとの互換性が必要になった場合 |
| TBD-06 | F-12 参照カウント自動昇格のロジック | P4+ | 閾値、昇格条件、降格ポリシー |

