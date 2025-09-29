# AISH - AI Shell

**[English](./README.md) | [繁體中文](#繁體中文) | [简体中文](./README_zh_cn.md) | [日本語](./README_ja.md)**

一個功能強大的命令行工具，集成 Google Gemini AI，提供智能終端協助。AISH 自動捕獲命令錯誤，通過 AI 分析並提供智能建議和修正。

> 最新穩定版本：**v0.0.2**

![AISH 演示](./demo/demo.gif)

## 核心功能與架構

AISH 與您的 shell 環境和 LLM 提供商整合，提供智能命令協助：

![AISH 系統架構](./demo/system_architecture.png)

### 主要功能

-   **🤖 智能錯誤分析**：自動捕獲命令執行錯誤、提供 AI 驅動的錯誤分類與分析，並提供智能修正建議與解釋。
-   **🔧 多 LLM 提供商支持**：支援多種 LLM 提供商，包括 OpenAI (GPT 系列)、Google Gemini (官方 API) 以及 Gemini CLI (Cloud Code 私有 API)。
-   **📝 自然語言命令生成**：能從自然語言提示（支援英文、中文、日文）生成 Shell 命令，並在離線模式下提供基礎命令建議。
-   **📊 歷史追蹤與重放**：保存錯誤分析歷史、允許重新分析歷史錯誤，並包含清理歷史記錄的功能。
-   **🎯 智能 Shell Hook**：自動整合到您的 shell (bash/zsh) 中，即時捕獲命令輸出和錯誤，提供無縫的 AI 協助而無需手動干預。

### 系統組件

- **🔗 Shell Hook**：自動捕獲終端中的命令執行和錯誤
- **🧠 錯誤分類器**：智能分類不同類型的命令失敗
- **🤖 LLM 提供商**：多個 AI 提供商（OpenAI、Gemini、Gemini CLI）進行智能分析
- **📚 歷史管理器**：持久化存儲命令歷史和分析結果
- **⚙️ 配置系統**：管理用戶偏好和提供商設置
- **🛡️ 安全層**：在 AI 分析前自動遮蔽敏感信息

## 安裝與配置

您可以透過三種方式安裝 AISH：使用 Homebrew（最簡單）、使用安裝腳本或手動從源碼構建。

### 1. 使用 Homebrew（最簡單）

如果您在 macOS 或已安裝 Homebrew 的 Linux 系統上：

```bash
brew tap TonnyWong1052/aish
brew install aish
```

安裝完成後，執行 `aish init` 來設定 shell hook 並配置您的 LLM 提供商。

### 2. 使用安裝腳本（推薦）

此腳本將會構建執行檔，將其安裝到 `~/bin`，並提供將其加入 `PATH` 的指引。

```bash
# 克隆專案
git clone https://github.com/TonnyWong1052/aish.git
cd aish

# 運行安裝腳本◊
./scripts/install.sh

# 如果希望在安裝後自動執行初始化（安裝 hook + 配置）
./scripts/install.sh --with-init
```

安裝完成後，請重啟您的終端機，或加載您的 shell 設定檔（例如：`source ~/.zshrc`）。然後，執行 `aish init` 來設定 shell hook 並配置您的 LLM 提供商。

## 🎯 Shell Hook - AISH 背後的魔法

**Shell Hook** 是讓 AISH 真正智能和無縫的核心組件。它自動整合到您的 shell 環境中，提供即時 AI 協助而無需任何手動干預。

### Hook 的功能

Shell Hook 自動：

- **🔍 捕獲命令輸出**：監控您執行的每個命令的 stdout 和 stderr
- **🚨 檢測錯誤**：智能識別命令何時失敗（非零退出碼）
- **🛡️ 過濾噪音**：跳過用戶主動中斷（Ctrl+C、Ctrl+\）和 AISH 自身的命令
- **🔒 清理敏感資料**：在發送給 AI 之前自動遮罩 API 金鑰、tokens、密碼和其他敏感資訊
- **⚡ 觸發 AI 分析**：檢測到錯誤時自動調用 AISH，提供即時反饋

### 工作原理

1. **命令前捕獲**：在每個命令執行前，hook 捕獲命令並準備輸出重定向
2. **輸出監控**：在命令執行期間，所有輸出（stdout/stderr）被捕獲到臨時檔案
3. **命令後分析**：命令完成後，hook 檢查退出碼並在需要時觸發 AI 分析
4. **智能過濾**：只有有意義的錯誤才會發送進行分析，避免來自故意中斷的垃圾訊息

### 支援的 Shell

- **🐚 Bash**：使用 `trap DEBUG` 和 `PROMPT_COMMAND` 進行命令攔截
- **🐚 Zsh**：使用 `preexec` 和 `precmd` hooks 實現無縫整合
- **🪟 PowerShell**：使用 profile 修改支援 Windows 環境

### 安全功能

- **🔐 自動遮罩**：敏感參數如 `--api-key`、`--token`、`--password` 會自動遮罩
- **🛡️ 環境變數保護**：包含 `SECRET`、`TOKEN`、`PASSWORD`、`API_KEY` 的變數會被遮罩
- **🚫 自我保護**：通過忽略 AISH 自身的命令防止無限循環
- **📁 安全儲存**：所有臨時檔案都儲存在 `~/.config/aish/` 中，具有適當的權限

### 避免互動式指令衝突

部分高度互動的 CLI（例如全螢幕 TUI、需要真實 TTY 的工具）在輸出被 hook 以 `tee` 鏡像時可能異常。AISH 內建略過 `claude`，你也可以透過下列方式控制略過行為：

- 自訂樣式略過（同時比對第一個 token 與整行命令）：
  ```bash
  export AISH_SKIP_COMMAND_PATTERNS="claude gh* fzf npm run *:watch"
  ```
- 略過所有使用者安裝指令，只捕捉系統指令（建議）：
  ```bash
  export AISH_SKIP_ALL_USER_COMMANDS=1
  # 可選：自訂系統白名單目錄（以冒號分隔）
  export AISH_SYSTEM_DIR_WHITELIST="/bin:/usr/bin:/sbin:/usr/sbin:/usr/libexec:/System/Library:/lib:/usr/lib"
  ```
  開啟後，凡是不在白名單目錄下的可執行檔（如 `/opt/homebrew/bin`、`/usr/local/bin`、`~/.bun/bin`、`~/.local/bin`、`~/go/bin` 等）會被視為使用者安裝並跳過。
- 單次繞過（本次完全不捕捉）：
  ```bash
  AISH_CAPTURE_OFF=1 <your-command>
  ```
- 完全不安裝 hook（本次 shell session）：
  ```bash
  export AISH_HOOK_DISABLED=1
  ```

執行 `aish init` 時也會提供相關互動設定，並將偏好寫入 `~/.config/aish/` 下的環境設定檔（POSIX 與 PowerShell），hook 啟動時自動載入。

### Hook 安裝

Hook 在您執行 `aish init` 時自動安裝。它會修改您的 shell 配置檔案：

- **Bash**：添加到 `~/.bashrc` 或 `~/.bash_profile`
- **Zsh**：添加到 `~/.zshrc`
- **PowerShell**：添加到您的 PowerShell profile

### 🏷️ 錯誤分類系統

Hook 包含一個智能錯誤分類系統，將不同類型的命令失敗進行分類，以提供更有針對性的 AI 分析：

#### **CommandNotFound** 🔍
- **觸發條件**：`command not found` 錯誤
- **範例**：
  ```bash
  $ unknowncmd
  bash: unknowncmd: command not found
  ```
- **AI 回應**：建議類似命令、安裝說明或拼寫錯誤修正

#### **FileNotFoundOrDirectory** 📁
- **觸發條件**：`No such file or directory` 錯誤
- **範例**：
  ```bash
  $ cat /nonexistent/file
  cat: /nonexistent/file: No such file or directory
  ```
- **AI 回應**：建議正確的檔案路徑、目錄列表或檔案建立

#### **PermissionDenied** 🔒
- **觸發條件**：`Permission denied` 錯誤
- **範例**：
  ```bash
  $ cat /root/secret
  cat: /root/secret: Permission denied
  ```
- **AI 回應**：建議權限修正、sudo 使用或替代方法

#### **CannotExecute** ⚠️
- **觸發條件**：`cannot execute binary file` 錯誤
- **範例**：
  ```bash
  $ ./script
  bash: ./script: cannot execute binary file
  ```
- **AI 回應**：建議使檔案可執行、檢查檔案類型或解釋器問題

#### **InvalidArgumentOrOption** ❌
- **觸發條件**：`invalid argument` 或 `invalid option` 錯誤
- **範例**：
  ```bash
  $ ls -Z
  ls: invalid option -- 'Z'
  ```
- **AI 回應**：建議正確的命令語法、可用選項或使用範例

#### **ResourceExists** 📄
- **觸發條件**：`File exists` 錯誤
- **範例**：
  ```bash
  $ mkdir /tmp/test
  mkdir: /tmp/test: File exists
  ```
- **AI 回應**：建議覆蓋選項、不同名稱或移除策略

#### **NotADirectory** 📂
- **觸發條件**：`is not a directory` 錯誤
- **範例**：
  ```bash
  $ cd /etc/passwd
  cd: /etc/passwd: is not a directory
  ```
- **AI 回應**：建議正確的目錄路徑或檔案與目錄操作

#### **TerminatedBySignal** ⏹️
- **觸發條件**：退出碼 > 128（信號終止）
- **範例**：
  ```bash
  $ long-running-command
  ^C  # Ctrl+C 中斷（退出碼 130）
  ```
- **AI 回應**：解釋信號終止，建議恢復或替代方法

#### **GenericError** 🔧
- **觸發條件**：所有其他錯誤類型
- **範例**：自訂應用程式錯誤、網路問題等
- **AI 回應**：一般故障排除建議和上下文特定解決方案

### 🎯 分類優勢

- **🎯 針對性回應**：每種錯誤類型都會收到專門的 AI 分析
- **📚 學習上下文**：AI 理解每種失敗的具體性質
- **⚡ 更快解決**：基於錯誤類別的更準確建議
- **🔄 一致處理**：對常見錯誤模式的標準化方法

### 3. 手動安裝

如果您偏好手動構建與安裝：

```bash
# 1. 構建應用程式
# CLI 主要入口點位於 cmd/aish
go build -o aish ./cmd/aish

# 2. 將執行檔移動到您 PATH 中的目錄
# 例如 ~/bin
mkdir -p ~/bin
mv aish ~/bin

# 3. 安裝 shell hook 並進行配置
aish init
```

### LLM 提供商配置

安裝完成後，配置 AISH 使用您偏好的 LLM 提供商：

```bash
# 初始化 AISH 配置
aish init
```

#### 🚀 Gemini CLI（推薦）
- **免費存取** Google 的 Gemini 模型
- **無需 API 金鑰**（使用您的 Google 帳戶驗證）
- **更高的速率限制** 相比官方 API

```bash
# 安裝 Gemini CLI: https://github.com/google/generative-ai-cli
aish init  # 選擇 "gemini-cli" 當提示時
```

#### 🔑 替代方案：官方 Gemini API
```bash
# 取得 API 金鑰: https://aistudio.google.com/app/apikey
aish init  # 選擇 "gemini" 並輸入您的 API 金鑰
```

#### 🤖 OpenAI GPT（替代方案）
```bash
aish init  # 選擇 "openai" 並輸入您的 API 金鑰
```

設定精靈將引導您完成提供商選擇、API 金鑰設定和 shell hook 安裝。

## 使用與範例

體驗 AISH 的強大功能，看看這些真實使用場景：

### 🚨 自動錯誤分析
當你犯錯時，AISH 立即提供智能反饋：

```bash
$ ls /nonexistent
ls: cannot access '/nonexistent': No such file or directory

🧠 AISH 分析：
┌─ 錯誤解釋 ─────────────────────────────────────────┐
│ 'ls' 命令失敗是因為路徑 '/nonexistent' 在您的         │
│ 檔案系統中不存在。                                   │
└───────────────────────────────────────────────────┘

💡 建議：檢查目錄路徑是否正確。
   您可以使用 'ls /' 來查看根目錄的內容。

🔧 修正後的命令：
   ls /

按 [Enter] 執行修正後的命令，或按其他鍵取消。
```

### 🤖 自然語言命令生成
從普通英文生成 shell 命令：

```bash
tomleung@LeungdeMacBook-Air Powerful-CLI % aish -p "find all .go files in the current directory"
Generating command...                                                           
 SUCCESS  Generating command...                                                 
                           
     Generated Command     
                           
Explanation:
基於您的提示 "find all .go files in the current directory"，我將生成一個命令來搜尋當前目錄中的 Go 文件。

Suggested Command:
find . -name "*.go"

Options:
  [Enter] - 執行建議的命令
  [n/no]  - 拒絕並退出
  [other] - 提供新的提示以獲得不同的建議

Select an option: 
```

```bash
tomleung@LeungdeMacBook-Air Powerful-CLI % aish -p "show me the git status in a nice format"
Generating command...                                                           
 SUCCESS  Generating command...                                                 
                           
     Generated Command     
                           
Explanation:
我將創建一個命令，以更易讀的格式顯示 git 狀態，包含表情符號和清晰的指示器。

Suggested Command:
git status --porcelain | while read status file; do
  case $status in
    M) echo "📝 已修改: $file" ;;
    A) echo "➕ 已新增: $file" ;;
    D) echo "🗑️  已刪除: $file" ;;
    ?) echo "❓ 未追蹤: $file" ;;
  esac
done

Options:
  [Enter] - 執行建議的命令
  [n/no]  - 拒絕並退出
  [other] - 提供新的提示以獲得不同的建議

Select an option: 
```

### 📊 歷史記錄與重放
查看和重新分析過去的錯誤：

```bash
$ aish history
📋 最近的錯誤分析歷史：
   1. [2 分鐘前] ls /nonexistent - 檔案不存在
   2. [15 分鐘前] git push origin main - 認證失敗
   3. [1 小時前] docker run nginx - 端口已被使用

$ aish history 2
🔄 重新分析錯誤 #2...
[顯示 git push 錯誤的詳細分析]
```

### 🔒 安全功能演示 - 敏感數據保護
Hook 自動保護您的敏感信息：

```bash
# 包含敏感數據的命令
$ curl -H "Authorization: Bearer sk-1234567890abcdef" https://api.example.com

# Hook 捕獲的內容（敏感數據已遮蔽）：
# curl -H "Authorization: Bearer ***REDACTED***" https://api.example.com

# 甚至環境變數也會被保護：
$ API_KEY=secret123 npm run deploy
# Hook 捕獲：API_KEY=***REDACTED*** npm run deploy
```

### 🛡️ 智能過濾示例
Hook 智能過濾噪音：

```bash
# ✅ 這些會觸發 AISH 分析：
$ git push origin main
# 錯誤：認證失敗

$ docker run nginx
# 錯誤：端口 80 已被使用

$ npm install
# 錯誤：找不到套件

# ❌ 這些不會觸發 AISH（故意過濾）：
$ ^C  # Ctrl+C 中斷
$ ^\  # Ctrl+\ 終止
$ aish capture  # AISH 自己的命令
```

## 使用

### 自動錯誤分析
配置完成後，AISH 會自動捕獲和分析命令錯誤：

```bash
# 犯個錯誤 - AISH 會自動幫助
$ ls /nonexistent
ls: cannot access '/nonexistent': No such file or directory

# AISH 自動提供分析和建議
```

### 手動命令生成
從自然語言生成命令：

```bash
aish -p "查找當前目錄中的所有 .go 文件"
aish -p "以美觀格式顯示 git 狀態"
```

### 查看歷史
查看過去的錯誤分析：

```bash
aish history
```

## 貢獻

我們歡迎貢獻！詳情請參閱我們的[貢獻指南](CONTRIBUTING.md)。

## 授權

本專案採用 MIT 授權條款 - 詳情請參閱 [LICENSE](LICENSE) 檔案。
