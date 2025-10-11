# Repository Guidelines

## Project Structure & Module Organization
- `cmd/aish`: Cobra entrypoint 和子命令；`main.go` 負責初始化、歷史、與 shell hooks。
- `internal/`: 依職責分包 — `shell/`（hook 安裝/執行）、`llm/`（供應商）、`config/`（載入/驗證）、`history/`、`ui/`。測試與原始碼同目錄。
- `scripts/`: 安裝與封裝腳本；`web-bundles/` 前端資產快照；`bin/` 本地構建輸出；文件與示例見 `docs/`、`demo/`；釋出產物在 `dist/`。
- 使用者設定：`~/.config/aish/`（`config.json`、`history.json`、`logs/aish.log`）。

## Build, Test, and Development Commands
- `make build` — 構建 CLI 至 `bin/aish`（等同 `go build -o bin/aish ./cmd/aish`）。
- `make test` — 以 `-race` 與覆蓋率執行單元測試，輸出 `coverage.out`。
- `make fmt` / `make lint` / `make vet` — 套用格式化、以 `.golangci.yml` 執行靜態檢查、Go 原生檢查。
- `make ci` — 格式檢查 + lint + vet + tests 的本地 CI。
- `./scripts/install.sh --with-init` — 模擬安裝並驗證預設 hooks。

## Coding Style & Naming Conventions
- 語言：Go；縮排使用 tabs。
- 使用 `gofumpt`、`gci`、`goimports` 整理格式與 import；遵循專案既有排序。
- 套件命名短小、小寫；匯出錯誤以 `Err...` 前綴集中管理。
- 註解以 English 完整句；旗標/欄位採 `camelCase`。

## Testing Guidelines
- 採用標準 `testing` 與表驅動；測試命名 `TestFeature_Scenario`；安全時使用 `t.Parallel()`。
- 聚焦套件：`go test ./internal/shell -v` 檢驗 hook 行為（含跨 shell 情境）。
- 重大變更前建議：`go test ./... -race`；以 `make coverage-min COV_MIN=60` 驗證覆蓋率門檻。

## Commit & Pull Request Guidelines
- Commit：English 祈使句、單一主題（如 `Refine prompt caching`）；分支 `feature/<topic>`、`fix/<ticket>`。
- PR：說明目的、關聯議題與測試結果；必要時附截圖/影片；標註 hooks/設定綱要/外部依賴變更；提交前確保 `make ci` 無誤。

## Security & Configuration Tips
- 以環境變數或 `~/.config/aish/config.json` 提供金鑰/Token；切勿提交祕密。
- 日誌預設於 `~/.config/aish/logs/aish.log`；分享前請去識別敏感資訊。
- 擴充 LLM 供應商：於 `internal/llm` 工廠註冊並提供安全後備路徑。
