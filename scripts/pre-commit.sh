#!/usr/bin/env bash
set -euo pipefail

echo "[pre-commit] Formatting with gofumpt/gci/goimports (if available)..."
if command -v gofumpt >/dev/null 2>&1; then
  gofumpt -w .
fi
if command -v gci >/dev/null 2>&1; then
  # Align with project module import grouping
  gci write -s standard -s default -s "prefix(github.com/TonnyWong1052/aish)" -w .
fi
if command -v goimports >/dev/null 2>&1; then
  goimports -w .
fi

echo "[pre-commit] Checking gofmt diffs..."
DIFFS=$(gofmt -s -l . || true)
if [[ -n "$DIFFS" ]]; then
  echo "The following files are not gofmt-simplified:" >&2
  echo "$DIFFS" >&2
  exit 1
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
