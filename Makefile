## Minimal developer helpers (Go 1.23+)
# Usage:
#   make tools       # Install local format tools (optional)
#   make fmt         # Apply gofumpt/gci/goimports
#   make lint        # Run golangci-lint (per .golangci.yml)
#   make vet         # Run go vet
#   make test        # Unit tests (race + coverage)
#   make build       # Build CLI to bin/
#   make ci          # Local CI (fmt check + lint + vet + test)

MODULE := github.com/TonnyWong1052/aish
PKG    := ./...
BIN    := bin/aish

.PHONY: all fmt lint vet test build tools ci tidy clean coverage-min

all: build

## Install local dev tools (optional)
# Note: requires network; CI installs via workflow
TOOLS := \
	mvdan.cc/gofumpt@latest \
	github.com/daixiang0/gci@latest \
	golang.org/x/tools/cmd/goimports@latest \
	github.com/golangci/golangci-lint/cmd/golangci-lint@latest

tools:
	@echo "Installing dev tools..."
	@for t in $(TOOLS); do \
		echo "go install $$t"; \
		GO111MODULE=on go install $$t; \
	done

## Apply formatting (no semantic rewrite)
fmt:
	@[ -x "$$(which gofumpt 2>/dev/null)" ] && gofumpt -w . || echo "gofumpt not found; skip"
	@[ -x "$$(which gci 2>/dev/null)" ] && gci write -s standard -s default -s "prefix($(MODULE))" -w . || echo "gci not found; skip"
	@[ -x "$$(which goimports 2>/dev/null)" ] && goimports -w . || echo "goimports not found; skip"

## Run configured linters per .golangci.yml (formatting focused)
lint:
	@golangci-lint run --timeout=5m

vet:
	@go vet $(PKG)

## Tests (with race and coverage)
test:
	@go test $(PKG) -race -covermode=atomic -coverprofile=coverage.out

## Check coverage threshold (default 60%)
COV_MIN ?= 60
coverage-min:
	@echo "==> Checking coverage threshold ($(COV_MIN)%)"
	@echo "Coverage summary:" && go tool cover -func=coverage.out | tail -n 1 || true
	@ACTUAL=$$(go tool cover -func=coverage.out | awk '/^total:/ { sub(/%/,"",$$3); print $$3 }'); \
	 if [ -z "$$ACTUAL" ]; then echo "Failed to parse coverage from coverage.out"; exit 1; fi; \
	 pass=$$(awk -v a="$$ACTUAL" -v m="$(COV_MIN)" 'BEGIN { print (a+0 >= m+0) ? 1 : 0 }'); \
	 if [ "$$pass" -ne 1 ]; then echo "Coverage $$ACTUAL% is below target $(COV_MIN)%"; exit 1; fi; \
	 echo "Coverage $$ACTUAL% meets target $(COV_MIN)%"

## Build CLI
build:
	@mkdir -p bin
	@go build -o $(BIN) ./cmd/aish
	@echo "Built $(BIN)"

## Local CI steps
ci:
	@echo "==> Checking formatting (diff should be clean)"
	@out=$$(gofmt -s -l .); if [ -n "$$out" ]; then echo "gofmt differences:"; echo "$$out"; exit 1; fi
	$(MAKE) lint
	$(MAKE) vet
	$(MAKE) test

## Dependency tidy (run after go.mod/go.sum changes)
tidy:
	@go mod tidy

clean:
	@rm -rf bin coverage.out
