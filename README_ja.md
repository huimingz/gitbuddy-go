# GitBuddy-Go

<p align="center">
  <strong>🤖 AI駆動のGitワークフローアシスタント</strong>
</p>

<p align="center">
  <a href="#機能">機能</a> •
  <a href="#インストール">インストール</a> •
  <a href="#クイックスタート">クイックスタート</a> •
  <a href="#設定">設定</a> •
  <a href="#使用方法">使用方法</a> •
  <a href="#対応llm">対応LLM</a>
</p>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README_zh.md">简体中文</a> |
  <a href="README_ja.md">日本語</a>
</p>

---

GitBuddy-Goは、日常のGitワークフローを自動化・強化するAI駆動のコマンドラインツールです。大規模言語モデル（LLM）をエージェントとして活用し、コード変更をインテリジェントに分析して、高品質なコミットメッセージ、PR説明、開発レポートを生成します。

## 機能

- **🎯 スマートコミットメッセージ**: ステージングされた変更を自動分析し、[Conventional Commits](https://www.conventionalcommits.org/)準拠のメッセージを生成
- **📝 PR説明ジェネレーター**: 概要、変更内容、動機、影響分析を含む包括的なPR説明を作成
- **🔍 コードレビュー**: バグ、セキュリティ問題、パフォーマンス問題、スタイル提案を識別するAI駆動のコードレビュー
- **🐛 問題デバッグ**: コード問題を体系的に分析・デバッグするインタラクティブなAIアシスタント
- **📊 開発レポート**: コミット履歴から構造化された週次/月次レポートを生成
- **🔄 自動リトライ**: 一時的なLLM APIの障害に対応する指数バックオフ付きスマートリトライ機構
- **💾 セッション管理**: Ctrl+Cサポート付きで長時間実行のデバッグ/レビューセッションを保存・再開
- **🌍 多言語対応**: 任意の言語で出力可能（日本語、英語、中国語など）
- **🔧 複数LLMプロバイダー対応**: OpenAI、DeepSeek、Ollama、Grok、Google Geminiをサポート
- **📡 リアルタイムストリーミング**: AIの分析プロセスをリアルタイムで確認
- **🤖 エージェントワークフロー**: LLMが自律的にGitツールを使用してコンテキストを収集

## インストール

### Go Installを使用（推奨）

```bash
go install github.com/huimingz/gitbuddy-go/cmd/gitbuddy@latest
```

これにより`gitbuddy`が`$GOPATH/bin`ディレクトリにインストールされます。このディレクトリが`PATH`に含まれていることを確認してください。

### ソースからビルド

```bash
git clone https://github.com/huimingz/gitbuddy-go.git
cd gitbuddy-go
go build -o gitbuddy ./cmd/gitbuddy
```

### 動作要件

- Go 1.21以上
- Git

## クイックスタート

1. **設定を初期化**:

```bash
gitbuddy init
```

`~/.gitbuddy.yaml`に設定ファイルが作成されます。

2. **LLMプロバイダーを設定**（`~/.gitbuddy.yaml`を編集）:

```yaml
default_model: deepseek

models:
  deepseek:
    provider: deepseek
    api_key: your-api-key-here
    model: deepseek-chat

language: ja  # デフォルトで日本語を使用
```

3. **コミットメッセージを生成**:

```bash
# まず変更をステージング
git add .

# 生成してコミット
gitbuddy commit
```

## 設定

GitBuddyはYAML設定ファイルを使用します。デフォルトの場所は`~/.gitbuddy.yaml`です。

### 設定ファイルの例

```yaml
# デフォルトで使用するモデル
default_model: deepseek

# 利用可能なモデル設定
models:
  deepseek:
    provider: deepseek
    api_key: sk-your-api-key
    model: deepseek-chat
    base_url: https://api.deepseek.com/v1  # オプション

  openai:
    provider: openai
    api_key: sk-your-openai-key
    model: gpt-4o

  ollama:
    provider: ollama
    model: qwen2.5:14b
    base_url: http://localhost:11434/v1

  gemini:
    provider: gemini
    api_key: your-gemini-api-key
    model: gemini-2.0-flash

# デフォルト出力言語
language: ja

# コードレビュー設定（オプション）
review:
  max_lines_per_read: 1000      # ファイル操作ごとの最大読み取り行数
  grep_max_file_size: 10        # grep の最大ファイルサイズ（MB）
  grep_timeout: 10              # grep 操作のタイムアウト（秒）
  grep_max_results: 100         # grep の最大結果数

# デバッグ設定（オプション）
debug:
  issues_dir: ./issues           # デバッグレポートの保存ディレクトリ
  max_iterations: 50             # 継続を確認する前の最大エージェント反復回数
  enable_compression: true       # メッセージ履歴圧縮を有効化
  compression_threshold: 20      # この数を超えるメッセージで圧縮
  compression_keep_recent: 10    # 圧縮後に保持する最近のメッセージ数
  show_compression_summary: false # 圧縮サマリーをユーザーに表示（デフォルト：false）
  max_lines_per_read: 1000       # ファイル操作ごとの最大読み取り行数
  grep_max_file_size: 10         # grep の最大ファイルサイズ（MB）
  grep_timeout: 10               # grep 操作のタイムアウト（秒）
  grep_max_results: 100          # grep の最大結果数

# リトライ設定（オプション）
retry:
  enabled: true                  # LLM API呼び出しの自動リトライを有効化
  max_attempts: 3                # 最大リトライ試行回数
  backoff_base: 1.0              # ベースバックオフ期間（秒）
  backoff_max: 30.0              # 最大バックオフ期間（秒）

# セッション設定（オプション）
session:
  save_dir: ~/.gitbuddy/sessions # セッションファイルの保存ディレクトリ
  auto_save: true                # 中断時にセッションを自動保存
  max_sessions: 50               # 保持する最大セッション数
```

### 設定の優先順位

1. コマンドライン引数（最優先）
2. 設定ファイル
3. 環境変数
4. デフォルト値

## 使用方法

### コミットメッセージの生成

```bash
# 基本的な使用法 - ステージングされた変更を分析してコミットメッセージを生成
gitbuddy commit

# 言語を指定
gitbuddy commit -l ja

# 追加のコンテキストを提供
gitbuddy commit -c "issue #123で報告されたログインバグを修正"

# 特定のモデルを使用
gitbuddy commit -m openai

# プロンプトなしで自動確認
gitbuddy commit -y
```

### PR説明の生成

```bash
# mainブランチと比較
gitbuddy pr --base main

# 言語とコンテキストを指定
gitbuddy pr --base develop -l ja -c "APIエンドポイントのパフォーマンス最適化"

# 特定のモデルを使用
gitbuddy pr --base main -m gemini
```

### 開発レポートの生成

```bash
# 日付範囲を指定してレポートを生成
gitbuddy report --since 2024-12-01 --until 2024-12-31

# 作成者でフィルター
gitbuddy report --since 2024-12-01 --author "john@example.com"

# 言語を指定
gitbuddy report --since 2024-12-01 -l ja
```

### コードレビュー

```bash
# ステージングされたすべての変更をレビュー
gitbuddy review

# 追加のコンテキストを提供
gitbuddy review -c "これは認証モジュールです"

# 特定のファイルのみレビュー
gitbuddy review --files "auth.go,crypto.go"

# エラーのみ表示（警告と情報を除外）
gitbuddy review --severity error

# セキュリティとパフォーマンスの問題に焦点を当てる
gitbuddy review --focus security,performance

# 日本語で出力
gitbuddy review -l ja
```

コードレビューは以下の種類の問題を識別します：
- 🔴 **エラー**: バグ、クラッシュ、重大な問題
- 🟡 **警告**: 潜在的なバグ、パフォーマンス問題
- 🔵 **情報**: スタイル提案、リファクタリングの機会

### 問題デバッグ

```bash
# AIアシスタントで特定の問題をデバッグ
gitbuddy debug "ログインが500エラーで失敗する"

# 追加のコンテキストを提供
gitbuddy debug "バックグラウンドワーカーでメモリリーク" -c "24時間実行後に発生"

# 特定のファイルに焦点を当てる
gitbuddy debug "TestUserAuthテストが失敗する" --files "auth_test.go,auth.go"

# インタラクティブモードを有効化（エージェントが入力を求めることができる）
gitbuddy debug "APIが間違ったデータを返す" --interactive

# 日本語でインタラクティブモードでデバッグ
gitbuddy debug "パフォーマンス問題" -l ja --interactive

# カスタム問題ディレクトリを指定
gitbuddy debug "データベース接続タイムアウト" --issues-dir ./debug-reports

# 最大反復回数を設定
gitbuddy debug "複雑な問題" --max-iterations 50

# インタラクティブモードでは、最大反復回数に達したときに継続するか尋ねられます

# 以前に中断されたセッションを再開
gitbuddy debug --resume debug-20240127-120000-abc123
```

デバッグコマンドの特徴：
- 🔍 **体系的な分析**: ファイルシステム、検索、Gitツールを使用して問題を分析
- 🤖 **自律的な探索**: コードベースを自律的に探索して問題を理解
- 💬 **インタラクティブな質問**: 必要に応じてユーザーの入力を求める（`--interactive`フラグ使用時）
- 📋 **詳細なレポート生成**: 根本原因分析と修正提案を含む詳細なレポートを生成
- 💾 **レポートの保存**: `./issues`ディレクトリにレポートを保存して将来の参照用に
- 🔄 **セッション再開サポート**: Ctrl+Cで中断し、後で`--resume`で再開可能

### セッション管理

```bash
# 保存されたすべてのセッションを一覧表示
gitbuddy sessions list

# 特定のセッションの詳細を表示
gitbuddy sessions show debug-20240127-120000-abc123

# セッションを削除
gitbuddy sessions delete debug-20240127-120000-abc123

# 古いセッションをクリーンアップし、最新の10個のみを保持
gitbuddy sessions clean --max 10
```

セッションは、Ctrl+Cでデバッグまたはレビューコマンドを中断すると自動的に保存されます。後で`--resume`フラグを使用して再開できます。

### その他のコマンド

```bash
# バージョン情報を表示
gitbuddy version

# 設定済みモデルを一覧表示
gitbuddy models list

# 設定ファイルを初期化
gitbuddy init
```

### グローバルフラグ

| フラグ | 説明 |
|--------|------|
| `--config` | 設定ファイルのパス（デフォルト：`~/.gitbuddy.yaml`） |
| `--debug` | デバッグモードを有効にして詳細なログを出力 |
| `-m, --model` | 使用するLLMモデルを指定 |

## 対応LLM

| プロバイダー | モデル | 備考 |
|--------------|--------|------|
| **DeepSeek** | deepseek-chat, deepseek-reasoner | おすすめ、コストパフォーマンス最高 |
| **OpenAI** | gpt-4o, gpt-4o-mini, gpt-3.5-turbo | OpenAI APIキーが必要 |
| **Ollama** | 任意のローカルモデル | ローカル実行、APIキー不要 |
| **Grok** | grok-beta | xAI APIキーが必要 |
| **Gemini** | gemini-2.0-flash, gemini-1.5-pro | Google AI APIキーが必要 |

## 動作原理

GitBuddyは**エージェントアプローチ**を採用しており、LLMが自律的にどのGitコマンドを実行するかを決定します：

1. **コミットメッセージ生成時**:
   - LLMが`git status`を呼び出して概要を取得
   - LLMが`git diff --cached`を呼び出して変更を分析
   - 必要に応じて`git log`を呼び出してコンテキストを取得
   - `submit_commit`ツールで構造化されたコミットメッセージを生成

2. **PR説明生成時**:
   - LLMが`git log`を呼び出してブランチ間のコミットを確認
   - LLMが`git diff`を呼び出してコード変更を分析
   - `submit_pr`ツールでPR説明を生成

3. **レポート生成時**:
   - LLMが日付フィルター付きで`git log`を呼び出し
   - コミットを分析・分類
   - `submit_report`ツールでレポートを生成

4. **コードレビュー時**:
   - LLMが`git diff --cached`を呼び出してステージングされた変更を分析
   - LLMが`grep_file`を使用してファイル内の特定の関数やパターンを素早く検索
   - LLMが`grep_directory`を使用して複数のファイルからコードパターンを検索
   - 必要に応じてLLMが`read_file`を呼び出して完全なソースコードのコンテキストを調査
   - バグ、セキュリティ問題、パフォーマンス問題を識別
   - `submit_review`ツールでレビューを生成

このエージェントアプローチにより、LLMは必要なコンテキストを正確に収集でき、より正確で関連性の高い出力を生成できます。

## 自動リトライとエラーハンドリング

GitBuddyは一時的なLLM APIの障害に対応するインテリジェントなリトライ機構を備えています：

- **スマートエラー分類**: リトライ可能なエラー（ネットワーク問題、タイムアウト、503、429）とリトライ不可能なエラー（400、401、コンテキスト超過）を自動的に区別
- **指数バックオフ**: APIに過負荷をかけないよう指数バックオフ戦略を実装
- **設定可能なリトライ**: 設定を通じてリトライ動作をカスタマイズ（最大試行回数、バックオフ期間）
- **ユーザーフレンドリーなメッセージ**: リトライ中に明確なフィードバックを提供

LLM API呼び出しがリトライ可能なエラーで失敗した場合、GitBuddyは試行間隔を徐々に増やしながら自動的にリトライします。

## デバッグモード

詳細情報を表示するにはデバッグモードを有効にします：

```bash
gitbuddy commit --debug
```

デバッグモードでは以下が表示されます：
- 設定の詳細
- 使用中のLLMプロバイダーとモデル
- ツール呼び出しとその結果
- リトライ試行とバックオフタイミング
- トークン使用統計
- 実行時間

## プロキシサポート

GitBuddyは標準のプロキシ環境変数をサポートしています：

```bash
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

## コントリビューション

コントリビューションを歓迎します！お気軽にPull Requestを提出してください。

## ライセンス

MIT License - 詳細は[LICENSE](LICENSE)を参照してください。

## 謝辞

- [CloudWeGo Eino](https://github.com/cloudwego/eino) - AIエージェントフレームワーク
- [Cobra](https://github.com/spf13/cobra) - CLIフレームワーク
- [Viper](https://github.com/spf13/viper) - 設定管理

