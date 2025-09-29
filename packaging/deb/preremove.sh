#!/bin/sh
# Debian/RPM 移除前提示腳本（非互動式）
# 作用：提醒使用者移除自動安裝的 shell hook（如先前執行過 aish init）

set -eu

echo "[aish] 即將移除 aish。若你曾執行 'aish init'，建議手動檢查你的 shell 啟動檔（如 ~/.bashrc、~/.zshrc）中的 aish hook 設定並按需移除。"

exit 0

