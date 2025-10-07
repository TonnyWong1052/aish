# Packaging Matrix

本文件彙整 AISH 目前的套件打包流程與維護重點，方便釋出時快速檢查。

## Linux

- **APT**：由 `.github/workflows/release.yml` 的 `update-apt-repo` 工作自動同步。
- **RPM**：GoReleaser 透過 `nfpms` 產生 `.rpm`，並在 `goreleaser` 工作中上傳成 CI artifact。若需要外部套件庫（如 COPR），可直接重用同一份二進位檔。
- **AUR (`aish-bin`)**：GoReleaser 的 `aurs` 設定會在 `dist/aur` 下輸出最新版 `PKGBUILD` 與 checksum。維護者可手動推送到 `aur@aur.archlinux.org:aish-bin.git`，或使用產出的檔案在本地 `makepkg -si` 驗證。

## Windows

- **Scoop**：`scoops` 配置會在 `dist/scoop` 下輸出 `aish.json`，並可推送至 `TonnyWong1052/scoop-aish` bucket。使用者可透過 `scoop bucket add aish https://github.com/TonnyWong1052/scoop-aish` 安裝。
- **winget**：`winget` 配置會產生 manifest。若需要提交至 `microsoft/winget-pkgs`，可從 `dist/winget` 取得檔案後送出 PR。

## Nix

- 根目錄的 `flake.nix` 使用 `buildGoModule`。於發佈前執行 `nix build .#aish` 取得真實 `vendorHash`，將輸出的雜湊貼回 flake 以確保 Nix 使用者能直接安裝。

## 流程建議

1. 發佈前執行 `go test ./...`、`make ci` 確認品質。
2. 若有介面變更，更新 `README.md` 對應的安裝章節。
3. 釋出後檢查 `dist/` 內是否生成 `rpm`、`aur`、`scoop`、`winget`、`nix` 資料夾；必要時依照上述說明推送至外部套件庫。
