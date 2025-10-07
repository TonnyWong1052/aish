#!/bin/bash

# 測試腳本：驗證網頁驗證的 token 自動刷新功能
#
# 前提條件：
# 1. 已經通過網頁驗證登入（gemini_oauth_creds.json 存在）
# 2. token 接近過期時間（在 2 小時內）
#
# 測試步驟：
# 1. 檢查 AISH 憑證是否存在
# 2. 顯示當前 token 過期時間
# 3. 執行一個需要 LLM 的命令（應該自動刷新 token）
# 4. 檢查 token 是否已更新

set -e

AISH_CREDS="$HOME/.config/aish/gemini_oauth_creds.json"

echo "=== Token 自動刷新測試 ==="
echo

# 檢查憑證檔案
if [ ! -f "$AISH_CREDS" ]; then
    echo "❌ AISH 憑證檔案不存在: $AISH_CREDS"
    echo "請先執行 'aish init' 並選擇網頁驗證"
    exit 1
fi

echo "✓ AISH 憑證檔案存在"
echo

# 顯示當前 token 資訊
echo "當前 token 資訊："
if command -v jq >/dev/null 2>&1; then
    EXPIRY_MS=$(jq -r '.expiry_date // 0' "$AISH_CREDS")
    if [ "$EXPIRY_MS" != "0" ]; then
        EXPIRY_DATE=$(date -r $((EXPIRY_MS / 1000)) 2>/dev/null || echo "無法解析")
        NOW=$(date +%s)
        EXPIRY_S=$((EXPIRY_MS / 1000))
        REMAINING=$((EXPIRY_S - NOW))
        REMAINING_HOURS=$((REMAINING / 3600))

        echo "  過期時間: $EXPIRY_DATE"
        echo "  剩餘時間: ${REMAINING_HOURS} 小時"

        if [ $REMAINING -lt 7200 ]; then
            echo "  ⚠️  Token 將在 2 小時內過期，執行命令時應該自動刷新"
        else
            echo "  ℹ️  Token 還有效，可能不會觸發刷新（除非您設定了 AISH_GEMINI_REFRESH_THRESHOLD）"
        fi
    fi
else
    echo "  (安裝 jq 以查看詳細資訊)"
    cat "$AISH_CREDS" | grep -E "(access_token|expiry_date)" || true
fi
echo

# 備份當前憑證（以便比較）
BACKUP="/tmp/aish_creds_before_test.json"
cp "$AISH_CREDS" "$BACKUP"
echo "✓ 已備份當前憑證到: $BACKUP"
echo

# 執行需要 LLM 的命令
echo "執行測試命令 (啟用 debug 模式)..."
echo "$ AISH_GEMINI_DEBUG=1 ./aish -p \"echo hello\""
echo
AISH_GEMINI_DEBUG=1 ./aish -p "echo hello" || {
    echo
    echo "❌ 命令執行失敗"
    echo "請檢查錯誤訊息並確保："
    echo "  1. token 資訊正確"
    echo "  2. 網路連線正常"
    echo "  3. OAuth client 憑證有效"
    exit 1
}
echo

# 比較憑證變化
echo "檢查 token 是否已更新..."
if command -v jq >/dev/null 2>&1; then
    BEFORE_TOKEN=$(jq -r '.access_token // ""' "$BACKUP" | cut -c1-20)
    AFTER_TOKEN=$(jq -r '.access_token // ""' "$AISH_CREDS" | cut -c1-20)

    if [ "$BEFORE_TOKEN" != "$AFTER_TOKEN" ]; then
        echo "✅ Token 已更新！"
        echo "  前: ${BEFORE_TOKEN}..."
        echo "  後: ${AFTER_TOKEN}..."
    else
        echo "ℹ️  Token 未改變（可能尚未過期或接近過期）"
    fi

    # 顯示新的過期時間
    NEW_EXPIRY_MS=$(jq -r '.expiry_date // 0' "$AISH_CREDS")
    if [ "$NEW_EXPIRY_MS" != "0" ]; then
        NEW_EXPIRY_DATE=$(date -r $((NEW_EXPIRY_MS / 1000)) 2>/dev/null || echo "無法解析")
        echo "  新過期時間: $NEW_EXPIRY_DATE"
    fi
else
    diff "$BACKUP" "$AISH_CREDS" && echo "ℹ️  憑證未改變" || echo "✅ 憑證已更新"
fi
echo

echo "=== 測試完成 ==="
echo
echo "說明："
echo "  - 如果 token 在 2 小時內過期，系統應該自動刷新"
echo "  - 刷新成功時會看到 'Token refreshed via HTTP' 或類似訊息"
echo "  - access_token 和 expiry_date 會更新"
echo
echo "提示："
echo "  - 設定 AISH_GEMINI_REFRESH_THRESHOLD=10m 可以測試更積極的刷新策略"
echo "  - 手動修改 expiry_date 為過去時間可以強制觸發刷新"
