# AISH - AI Shell

**[English](./README.md) | [繁體中文](./README_zh.md) | [简体中文](./README_zh_cn.md) | [日本語](#日本語)**

Google Gemini AI と統合された強力なコマンドラインツールで、インテリジェントなターミナル支援を提供します。AISH は自動的にコマンドエラーをキャプチャし、AI で分析してスマートな提案と修正を提供します。

> 最新安定版：**v0.0.1**

![AISH デモ](./demo/demo.gif)

## コア機能とアーキテクチャ

AISH はあなたのシェル環境と LLM プロバイダーと統合し、インテリジェントなコマンド支援を提供します：

### 主要機能

-   **🤖 インテリジェントエラー分析**：コマンド実行エラーを自動的にキャプチャし、AI 駆動のエラー分類と分析を提供し、インテリジェントな修正提案と説明を提供します。
-   **🔧 マルチ LLM プロバイダーサポート**：OpenAI (GPT シリーズ)、Google Gemini (公式 API)、Gemini CLI (Cloud Code プライベート API) など、さまざまな LLM プロバイダーをサポートします。
-   **📝 自然言語コマンド生成**：自然言語プロンプト（英語、中国語、日本語をサポート）から Shell コマンドを生成し、オフラインモードで基本的なコマンド提案も提供します。
-   **📊 履歴追跡とリプレイ**：エラー分析履歴を保存し、過去のエラーの再分析を可能にし、履歴クリア機能も含みます。
-   **🎯 スマートシェルフック**：シェル (bash/zsh) に自動的に統合され、コマンド出力とエラーをリアルタイムでキャプチャし、手動介入なしでシームレスな AI アシスタンスを提供します。

### システムコンポーネント

以下の図は、AISH がシェル環境と LLM プロバイダーと統合してインテリジェントなコマンド支援を提供する方法を示しています：

![AISH システムアーキテクチャ](./demo/system_architecture.png)

### アーキテクチャ概要

AISH は複数の重要なコンポーネントが連携して動作します：

- **🔗 Shell Hook**：ターミナルでのコマンド実行とエラーを自動的にキャプチャ
- **🧠 エラー分類器**：異なるタイプのコマンド失敗をインテリジェントに分類
- **🤖 LLM プロバイダー**：インテリジェントな分析のための複数の AI プロバイダー（OpenAI、Gemini、Gemini CLI）
- **📚 履歴マネージャー**：コマンド履歴と分析結果の永続化ストレージ
- **⚙️ 設定システム**：ユーザー設定とプロバイダー設定の管理
- **🛡️ セキュリティ層**：AI 分析前に機密情報を自動的にマスク

## インストールガイド

AISH をインストールする方法は 3 つあります：Homebrew を使用する（最も簡単）、インストールスクリプトを使用する、またはソースから手動でビルドするかです。

### 1. Homebrew の使用（最も簡単）

macOS または Homebrew がインストールされた Linux システムの場合：

```bash
brew tap TonnyWong1052/aish
brew install aish
```

インストール後、`aish init` を実行してシェルフックを設定し、LLM プロバイダーを設定します。

### 2. インストールスクリプトの使用（推奨）

このスクリプトはバイナリをビルドし、`~/bin` にインストールし、`PATH` に追加する手順を提供します。

```bash
# リポジトリをクローン
git clone https://github.com/TonnyWong1052/aish.git
cd aish

# インストールスクリプトを実行
./scripts/install.sh

# インストール後に自動的に初期化を実行したい場合（hook + 設定のインストール）
./scripts/install.sh --with-init
```

インストール後、ターミナルを再起動するか、シェル設定ファイルを読み込みます（例：`source ~/.zshrc`）。その後、`aish init` を実行してシェルフックを設定し、LLM プロバイダーを設定します。

## 🎯 シェルフック - AISH の魔法の裏側

**シェルフック** は AISH を真にインテリジェントでシームレスにする中核コンポーネントです。シェル環境に自動的に統合され、手動介入なしでリアルタイムの AI アシスタンスを提供します。

### フックの機能

シェルフックは自動的に：

- **🔍 コマンド出力をキャプチャ**：実行するすべてのコマンドの stdout と stderr を監視
- **🚨 エラーを検出**：コマンドがいつ失敗したかをインテリジェントに識別（非ゼロ終了コード）
- **🛡️ ノイズをフィルタリング**：ユーザー主導の中断（Ctrl+C、Ctrl+\）と AISH 自身のコマンドをスキップ
- **🔒 機密データをサニタイズ**：AI に送信する前に API キー、トークン、パスワード、その他の機密情報を自動的にマスク
- **⚡ AI 分析をトリガー**：エラーが検出されたときに自動的に AISH を呼び出し、即座にフィードバックを提供

### 動作原理

1. **コマンド前キャプチャ**：各コマンドが実行される前に、フックがコマンドをキャプチャし、出力リダイレクションを準備
2. **出力監視**：コマンド実行中、すべての出力（stdout/stderr）が一時ファイルにキャプチャ
3. **コマンド後分析**：コマンド完了後、フックが終了コードをチェックし、必要に応じて AI 分析をトリガー
4. **スマートフィルタリング**：意味のあるエラーのみが分析のために送信され、意図的な中断からのスパムを回避

### サポートされるシェル

- **🐚 Bash**：`trap DEBUG` と `PROMPT_COMMAND` を使用してコマンドインターセプト
- **🐚 Zsh**：`preexec` と `precmd` フックを使用してシームレスな統合を実現
- **🪟 PowerShell**：プロファイル変更を使用して Windows 環境をサポート

### セキュリティ機能

- **🔐 自動マスキング**：`--api-key`、`--token`、`--password` などの機密パラメータが自動的にマスク
- **🛡️ 環境変数保護**：`SECRET`、`TOKEN`、`PASSWORD`、`API_KEY` を含む変数がマスク
- **🚫 自己保護**：AISH 自身のコマンドを無視することで無限ループを防止
- **📁 安全なストレージ**：すべての一時ファイルが `~/.config/aish/` に適切な権限で保存

### フックインストール

フックは `aish init` を実行すると自動的にインストールされます。シェル設定ファイルを変更します：

- **Bash**：`~/.bashrc` または `~/.bash_profile` に追加
- **Zsh**：`~/.zshrc` に追加
- **PowerShell**：PowerShell プロファイルに追加

### 🏷️ エラー分類システム

フックには、よりターゲットを絞った AI 分析のために異なるタイプのコマンド失敗を分類するインテリジェントなエラー分類システムが含まれています：

#### **CommandNotFound** 🔍
- **トリガー条件**：`command not found` エラー
- **例**：
  ```bash
  $ unknowncmd
  bash: unknowncmd: command not found
  ```
- **AI レスポンス**：類似コマンド、インストール手順、またはタイプミス修正を提案

#### **FileNotFoundOrDirectory** 📁
- **トリガー条件**：`No such file or directory` エラー
- **例**：
  ```bash
  $ cat /nonexistent/file
  cat: /nonexistent/file: No such file or directory
  ```
- **AI レスポンス**：正しいファイルパス、ディレクトリリスト、またはファイル作成を提案

#### **PermissionDenied** 🔒
- **トリガー条件**：`Permission denied` エラー
- **例**：
  ```bash
  $ cat /root/secret
  cat: /root/secret: Permission denied
  ```
- **AI レスポンス**：権限修正、sudo 使用、または代替アプローチを提案

#### **CannotExecute** ⚠️
- **トリガー条件**：`cannot execute binary file` エラー
- **例**：
  ```bash
  $ ./script
  bash: ./script: cannot execute binary file
  ```
- **AI レスポンス**：ファイルを実行可能にする、ファイルタイプをチェックする、またはインタープリターの問題を提案

#### **InvalidArgumentOrOption** ❌
- **トリガー条件**：`invalid argument` または `invalid option` エラー
- **例**：
  ```bash
  $ ls -Z
  ls: invalid option -- 'Z'
  ```
- **AI レスポンス**：正しいコマンド構文、利用可能なオプション、または使用例を提案

#### **ResourceExists** 📄
- **トリガー条件**：`File exists` エラー
- **例**：
  ```bash
  $ mkdir /tmp/test
  mkdir: /tmp/test: File exists
  ```
- **AI レスポンス**：上書きオプション、異なる名前、または削除戦略を提案

#### **NotADirectory** 📂
- **トリガー条件**：`is not a directory` エラー
- **例**：
  ```bash
  $ cd /etc/passwd
  cd: /etc/passwd: is not a directory
  ```
- **AI レスポンス**：正しいディレクトリパスまたはファイルとディレクトリの操作を提案

#### **TerminatedBySignal** ⏹️
- **トリガー条件**：終了コード > 128（シグナル終了）
- **例**：
  ```bash
  $ long-running-command
  ^C  # Ctrl+C 中断（終了コード 130）
  ```
- **AI レスポンス**：シグナル終了を説明し、再開または代替アプローチを提案

#### **GenericError** 🔧
- **トリガー条件**：その他のすべてのエラータイプ
- **例**：カスタムアプリケーションエラー、ネットワーク問題など
- **AI レスポンス**：一般的なトラブルシューティングアドバイスとコンテキスト固有のソリューション

### 🎯 分類の利点

- **🎯 ターゲットを絞ったレスポンス**：各エラータイプが専門的な AI 分析を受信
- **📚 学習コンテキスト**：AI が各失敗の具体的な性質を理解
- **⚡ より迅速な解決**：エラーカテゴリに基づくより正確な提案
- **🔄 一貫した処理**：一般的なエラーパターンへの標準化されたアプローチ

## LLM プロバイダー設定

AISH は複数の LLM プロバイダーをサポートしています。以下は推奨設定です：

### 🚀 Gemini CLI（推奨）
最高の体験を得るために、**Gemini CLI** の使用を推奨します。以下の利点があります：
- **無料アクセス** Google の Gemini モデルへ
- **API キー不要**（Google アカウント認証を使用）
- **より高いレート制限** 公式 API と比較して
- **より良い統合** Google エコシステムとの

Gemini CLI の設定：
```bash
# Gemini CLI をインストール（まだインストールしていない場合）
# 手順はこちらを参照：https://github.com/google/generative-ai-cli

# AISH を Gemini CLI を使用するように設定
aish init
# LLM プロバイダーの選択を求められたら "gemini-cli" を選択
```

### 🔑 代替案：公式 Gemini API
公式 API を好む場合：
```bash
# 以下の URL から API キーを取得：https://aistudio.google.com/app/apikey
aish init
# LLM プロバイダーの選択を求められたら "gemini" を選択
# プロンプトが表示されたら API キーを入力
```

### 🤖 OpenAI GPT（代替案）
OpenAI ユーザー向け：
```bash
aish init
# LLM プロバイダーの選択を求められたら "openai" を選択
# プロンプトが表示されたら OpenAI API キーを入力
```

### 3. 手動インストール

手動でビルドとインストールを好む場合：

```bash
# 1. アプリケーションをビルド
# CLI のメインエントリーポイントは cmd/aish にあります
go build -o aish ./cmd/aish

# 2. バイナリを PATH 内のディレクトリに移動
# 例：~/bin
mkdir -p ~/bin
mv aish ~/bin

# 3. シェルフックをインストールして設定
aish init
```

## 実際のデモ

AISH の強力な機能を体験してください。これらの実際の使用例をご覧ください：

### 🚨 自動エラー分析
間違いを犯したとき、AISH は即座にインテリジェントなフィードバックを提供します：

```bash
$ ls /nonexistent
ls: cannot access '/nonexistent': No such file or directory

🧠 AISH 分析：
┌─ エラー説明 ─────────────────────────────────────────┐
│ 'ls' コマンドが失敗したのは、パス '/nonexistent' が  │
│ ファイルシステムに存在しないためです。              │
└────────────────────────────────────────────────────┘

💡 提案：ディレクトリパスが正しいか確認してください。
   ルートディレクトリの内容を確認するには 'ls /' を使用できます。

🔧 修正されたコマンド：
   ls /

[Enter] を押して修正されたコマンドを実行するか、他のキーを押してキャンセルしてください。
```

### 🤖 自然言語コマンド生成
普通の英語からシェルコマンドを生成：

```bash
tomleung@LeungdeMacBook-Air Powerful-CLI % aish -p "find all .go files in the current directory"
Generating command...                                                           
 SUCCESS  Generating command...                                                 
                           
     Generated Command     
                           
Explanation:
プロンプト "find all .go files in the current directory" に基づいて、現在のディレクトリ内の Go ファイルを検索するコマンドを生成します。

Suggested Command:
find . -name "*.go"

Options:
  [Enter] - 提案されたコマンドを実行
  [n/no]  - 拒否して終了
  [other] - 異なる提案のための新しいプロンプトを提供

Select an option: 
```

```bash
tomleung@LeungdeMacBook-Air Powerful-CLI % aish -p "show me the git status in a nice format"
Generating command...                                                           
 SUCCESS  Generating command...                                                 
                           
     Generated Command     
                           
Explanation:
絵文字と明確なインジケーターを含む、より読みやすい形式で git ステータスを表示するコマンドを作成します。

Suggested Command:
git status --porcelain | while read status file; do
  case $status in
    M) echo "📝 変更済み: $file" ;;
    A) echo "➕ 追加済み: $file" ;;
    D) echo "🗑️  削除済み: $file" ;;
    ?) echo "❓ 未追跡: $file" ;;
  esac
done

Options:
  [Enter] - 提案されたコマンドを実行
  [n/no]  - 拒否して終了
  [other] - 異なる提案のための新しいプロンプトを提供

Select an option: 
```

### 📊 履歴とリプレイ
過去のエラーを確認して再分析：

```bash
$ aish history
📋 最近のエラー分析履歴：
   1. [2 分前] ls /nonexistent - ファイルが見つかりません
   2. [15 分前] git push origin main - 認証に失敗しました
   3. [1 時間前] docker run nginx - ポートが既に使用されています

$ aish history 2
🔄 エラー #2 を再分析中...
[git push エラーの詳細分析を表示]
```

### 🔒 セキュリティ機能の実演 - 機密データ保護
Hook は自動的に機密情報を保護します：

```bash
# 機密データを含むコマンド
$ curl -H "Authorization: Bearer sk-1234567890abcdef" https://api.example.com

# Hook がキャプチャする内容（機密データはマスク済み）：
# curl -H "Authorization: Bearer ***REDACTED***" https://api.example.com

# 環境変数も保護されます：
$ API_KEY=secret123 npm run deploy
# Hook がキャプチャ：API_KEY=***REDACTED*** npm run deploy
```

### 🛡️ スマートフィルタリングの例
Hook はインテリジェントにノイズをフィルタリングします：

```bash
# ✅ これらは AISH 分析をトリガーします：
$ git push origin main
# エラー：認証に失敗しました

$ docker run nginx
# エラー：ポート 80 が既に使用されています

$ npm install
# エラー：パッケージが見つかりません

# ❌ これらは AISH をトリガーしません（意図的にフィルタリング）：
$ ^C  # Ctrl+C 中断
$ ^\  # Ctrl+\ 終了
$ aish capture  # AISH 自身のコマンド
```

## クイックスタート

-   **エラーキャプチャ（自動）**：コマンドが失敗したとき、AISH は自動的にエラーを分析して提案を提供します。
    ```bash
    # 間違ったコマンドを実行
    ls /nonexistent
    # AISH が自動的に分析して修正案を提供します。
    ```
-   **自然言語コマンド**：`-p` フラグを使用して自然言語からコマンドを生成します。
    ```bash
    aish -p "現在のディレクトリ内のすべてのファイルを一覧表示"
    aish -p "すべての .go ファイルを検索"
    ```

## 設定

インストール後、お好みの LLM プロバイダーで AISH を設定する必要があります：

```bash
# AISH 設定を初期化
aish init
```

これにより以下が案内されます：
- LLM プロバイダーの選択（Gemini CLI、Gemini API、または OpenAI）
- API キーの設定（必要な場合）
- 自動エラーキャプチャのための shell hook のインストール

## 使用方法

### 自動エラー分析
設定が完了すると、AISH は自動的にコマンドエラーをキャプチャして分析します：

```bash
# 間違いを犯す - AISH が自動的にヘルプします
$ ls /nonexistent
ls: cannot access '/nonexistent': No such file or directory

# AISH が自動的に分析と提案を提供します
```

### 手動コマンド生成
自然言語からコマンドを生成：

```bash
aish -p "現在のディレクトリ内のすべての .go ファイルを検索"
aish -p "美しい形式で git ステータスを表示"
```

### 履歴の表示
過去のエラー分析を確認：

```bash
aish history
```

## 貢献

貢献を歓迎します！詳細については[貢献ガイドライン](CONTRIBUTING.md)をご覧ください。

## ライセンス

このプロジェクトは MIT ライセンスの下でライセンスされています - 詳細については [LICENSE](LICENSE) ファイルをご覧ください。
