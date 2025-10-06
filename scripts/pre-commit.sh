#!/usr/bin/env bash
set -euo pipefail

echo "[pre-commit] Formatting staged Go files..."

# Get a list of staged Go files
STAGED_GO_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$')

if [[ -z "$STAGED_GO_FILES" ]]; then
  echo "No Go files to format."
else
  # Format staged files
  if command -v gofumpt >/dev/null 2>&1; then
    echo "$STAGED_GO_FILES" | xargs gofumpt -w
  fi
  if command -v gci >/dev/null 2>&1; then
    echo "$STAGED_GO_FILES" | xargs gci write -s standard -s default -s "prefix(github.com/TonnyWong1052/aish)"
  fi
  if command -v goimports >/dev/null 2>&1; then
    echo "$STAGED_GO_FILES" | xargs goimports -w
  fi
  
  # Re-add the formatted files to the staging area
  echo "$STAGED_GO_FILES" | xargs git add
fi

if command -v golangci-lint >/dev/null 2>&1; then
  echo "[pre-commit] Running golangci-lint..."
  golangci-lint run --timeout=3m
else
  echo "[pre-commit] golangci-lint not found; skipping lint."
fi

echo "[pre-commit] Running short tests..."
go test ./... -short

echo "[pre-commit] OK"
