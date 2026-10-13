# Build, test, and lint the oops module.
# Version metadata is injected with -ldflags; see internal/version.

MODULE  := github.com/lintsinghua/oops
BIN_DIR := bin
BINARY  := $(BIN_DIR)/oops
PKG     := ./cmd/oops

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)

GOLANGCI_LINT ?= golangci-lint

.DEFAULT_GOAL := build

.PHONY: build test lint fmt clean

build:
	mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

test:
	go test -race ./...

fmt:
	gofmt -w .

lint:
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then \
		echo "gofmt would change:"; \
		printf '%s\n' "$$out"; \
		exit 1; \
	fi
	go vet ./...
	$(GOLANGCI_LINT) run ./...

clean:
	rm -rf $(BIN_DIR) dist
