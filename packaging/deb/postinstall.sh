#!/bin/sh
# Debian/RPM 安裝後提示腳本（非互動式）
# 作用：提示使用者執行 aish 初始化以安裝 Shell Hook 與設定 LLM 提供者

set -eu

echo "[aish] 安裝完成。"
echo "[aish] 建議執行：aish init 以完成初始設定（選擇 LLM 提供者、安裝 shell hook）。"
echo "[aish] 文件：/usr/share/doc/aish 或線上 README，取得 APT 倉庫與疑難排解資訊。"

exit 0

