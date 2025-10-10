#!/bin/bash

# Test Script: Verify automatic token refresh for web authentication
#
# Prerequisites:
# 1. Already logged in via web authentication (gemini_oauth_creds.json exists)
# 2. Token is close to expiration (within 2 hours)
#
# Test Steps:
# 1. Check if AISH credentials exist
# 2. Display current token expiration time
# 3. Execute a command requiring LLM (should automatically refresh token)
# 4. Check if token has been updated

set -e

AISH_CREDS="$HOME/.config/aish/gemini_oauth_creds.json"

echo "=== Token Auto-Refresh Test ==="
echo

# Check credentials file
if [ ! -f "$AISH_CREDS" ]; then
    echo "❌ AISH credentials file does not exist: $AISH_CREDS"
    echo "Please run 'aish init' and select web authentication"
    exit 1
fi

echo "✓ AISH credentials file exists"
echo

# Display current token information
echo "Current token information:"
if command -v jq >/dev/null 2>&1; then
    EXPIRY_MS=$(jq -r '.expiry_date // 0' "$AISH_CREDS")
    if [ "$EXPIRY_MS" != "0" ]; then
        EXPIRY_DATE=$(date -r $((EXPIRY_MS / 1000)) 2>/dev/null || echo "Unable to parse")
        NOW=$(date +%s)
        EXPIRY_S=$((EXPIRY_MS / 1000))
        REMAINING=$((EXPIRY_S - NOW))
        REMAINING_HOURS=$((REMAINING / 3600))

        echo "  Expiration time: $EXPIRY_DATE"
        echo "  Time remaining: ${REMAINING_HOURS} hours"

        if [ $REMAINING -lt 7200 ]; then
            echo "  ⚠️  Token will expire within 2 hours, should auto-refresh when executing command"
        else
            echo "  ℹ️  Token is still valid, may not trigger refresh (unless AISH_GEMINI_REFRESH_THRESHOLD is set)"
        fi
    fi
else
    echo "  (Install jq to view detailed information)"
    cat "$AISH_CREDS" | grep -E "(access_token|expiry_date)" || true
fi
echo

# Backup current credentials (for comparison)
BACKUP="/tmp/aish_creds_before_test.json"
cp "$AISH_CREDS" "$BACKUP"
echo "✓ Backed up current credentials to: $BACKUP"
echo

# Execute command requiring LLM
echo "Executing test command (with debug mode enabled)..."
echo "$ AISH_GEMINI_DEBUG=1 ./aish -p \"echo hello\""
echo
AISH_GEMINI_DEBUG=1 ./aish -p "echo hello" || {
    echo
    echo "❌ Command execution failed"
    echo "Please check error messages and ensure:"
    echo "  1. Token information is correct"
    echo "  2. Network connection is working"
    echo "  3. OAuth client credentials are valid"
    exit 1
}
echo

# Compare credential changes
echo "Checking if token has been updated..."
if command -v jq >/dev/null 2>&1; then
    BEFORE_TOKEN=$(jq -r '.access_token // ""' "$BACKUP" | cut -c1-20)
    AFTER_TOKEN=$(jq -r '.access_token // ""' "$AISH_CREDS" | cut -c1-20)

    if [ "$BEFORE_TOKEN" != "$AFTER_TOKEN" ]; then
        echo "✅ Token has been updated!"
        echo "  Before: ${BEFORE_TOKEN}..."
        echo "  After: ${AFTER_TOKEN}..."
    else
        echo "ℹ️  Token unchanged (may not be expired or close to expiration yet)"
    fi

    # Display new expiration time
    NEW_EXPIRY_MS=$(jq -r '.expiry_date // 0' "$AISH_CREDS")
    if [ "$NEW_EXPIRY_MS" != "0" ]; then
        NEW_EXPIRY_DATE=$(date -r $((NEW_EXPIRY_MS / 1000)) 2>/dev/null || echo "Unable to parse")
        echo "  New expiration time: $NEW_EXPIRY_DATE"
    fi
else
    diff "$BACKUP" "$AISH_CREDS" && echo "ℹ️  Credentials unchanged" || echo "✅ Credentials updated"
fi
echo

echo "=== Test Complete ==="
echo
echo "Notes:"
echo "  - If token expires within 2 hours, system should automatically refresh"
echo "  - On successful refresh, you will see 'Token refreshed via HTTP' or similar message"
echo "  - access_token and expiry_date will be updated"
echo
echo "Tips:"
echo "  - Set AISH_GEMINI_REFRESH_THRESHOLD=10m to test more aggressive refresh strategy"
echo "  - Manually modify expiry_date to past time to force refresh trigger"
