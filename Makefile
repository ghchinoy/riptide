# Riptide Makefile

.PHONY: all build build-agent build-viewer frontend-build test clean help serve

BIN_DIR := bin

# Default target
all: build

help:
	@echo "Riptide Build System"
	@echo "Targets:"
	@echo "  build           Build the riptide CLI and session-viewer"
	@echo "  build-agent     Build the riptide CLI binary (bin/riptide)"
	@echo "  build-viewer    Build the session-viewer binary (bin/session-viewer)"
	@echo "  frontend-build  Build the Lit frontend"
	@echo "  test            Run Go tests"
	@echo "  clean           Clean build artifacts"
	@echo "  serve           Start the Session Viewer (bin/riptide serve)"
	@echo ""
	@echo "Quick start:"
	@echo "  make build && bin/riptide config init"
	@echo "  bin/riptide run --prompt \"Go to google.com\""

build: build-agent build-viewer

# Primary CLI — all commands: run, config, serve, sessions
build-agent: | $(BIN_DIR)
	go build -o $(BIN_DIR)/riptide .

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
