# 發佈流程（自動化）

本專案已整合 GoReleaser 與 GitHub Actions，自動產生 GitHub Release 並更新 Homebrew Tap 公式。

## 先決條件

1. 在倉庫的 Secrets 中新增 `HOMEBREW_TAP_TOKEN`（PAT，需對 `TonnyWong1052/aish` 與 `TonnyWong1052/homebrew-aish` 皆有 `repo` 權限）。
2. 確認 `.goreleaser.yaml` 與 `.github/workflows/release.yml` 已於 main。

## 發佈步驟

1. 合併所有變更到 `main`。
2. 建立標籤並推送：

   ```bash
   git tag -a v0.0.2 -m "v0.0.2"
   git push origin v0.0.2
   ```

3. GitHub Actions 會自動觸發：
   - 建置二進位（注入 `-X main._version=v0.0.2`）
   - 建立 GitHub Release，產出對應資產
   - 生成並推送 Homebrew 公式到 `homebrew-aish` Tap（含正確 `sha256`）

## 驗證

```bash
brew update
brew tap TonnyWong1052/aish
brew reinstall aish
aish --version  # aish v0.0.2
```

## 注意事項

- 請勿重簽舊標籤，若需修正，請發佈新的 patch 版本（例如由 v0.0.2 → v0.0.4）。
- 版本字串預設由 ldflags 注入；若非釋出環境（本地 build），程式內的 fallback 版本可能不同步，屬正常現象。
