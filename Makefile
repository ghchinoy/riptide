# Riptide Makefile

.PHONY: all build build-agent build-viewer frontend-build test clean help serve release release-dry-run

BIN_DIR := bin

# Version: prefer git tags; fall back to "dev" if the repo has no tags yet.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/ghchinoy/riptide/cmd.Version=$(VERSION)

# Default target
all: build

help:
	@echo "Riptide Build System"
	@echo "Targets:"
	@echo "  build             Build the riptide CLI and session-viewer"
	@echo "  build-agent       Build the riptide CLI binary (bin/riptide)"
	@echo "  build-viewer      Build the session-viewer binary (bin/session-viewer)"
	@echo "  frontend-build    Build the Lit frontend"
	@echo "  test              Run Go tests"
	@echo "  clean             Clean build artifacts"
	@echo "  serve             Start the Session Viewer (bin/riptide serve)"
	@echo "  release-dry-run   Build all release targets locally (no publish)"
	@echo "  release           Print instructions for cutting a release"
	@echo ""
	@echo "Quick start:"
	@echo "  make build && bin/riptide config init"
	@echo "  bin/riptide run --prompt \"Go to google.com\""

build: build-agent build-viewer

# Primary CLI — all commands: run, config, serve, sessions
build-agent: | $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/riptide .

build-viewer: frontend-build | $(BIN_DIR)
	go build -o $(BIN_DIR)/session-viewer cmd/session-viewer/main.go

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

frontend-build:
	@echo "Building frontend..."
	@cd frontend && [ -d node_modules ] || npm install
	@cd frontend && npm run build

test:
	go test -v ./pkg/...

test-short:
	go test -short ./pkg/...

serve: build-agent
	./$(BIN_DIR)/riptide serve

clean:
	rm -rf $(BIN_DIR)
	rm -rf frontend/dist
	rm -rf dist

# Build all release targets locally without publishing (requires goreleaser).
release-dry-run:
	goreleaser release --snapshot --clean

# Print instructions for cutting a tagged release.
release:
	@echo ""
	@echo "To cut a release:"
	@echo "  1. git tag -a vX.Y.Z -m 'Release vX.Y.Z'"
	@echo "  2. git push origin vX.Y.Z"
	@echo ""
	@echo "GitHub Actions will handle the rest."
	@echo "See docs/releasing.md for full details."
	@echo ""
	@echo "Current version: $(VERSION)"
