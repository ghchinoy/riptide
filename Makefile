# Riptide Makefile

.PHONY: all build build-agent build-viewer frontend-build test clean help

BIN_DIR := bin

# Default target
all: build

help:
	@echo "Riptide Build System"
	@echo "Targets:"
	@echo "  build           Build everything (agent, frontend, viewer)"
	@echo "  build-agent     Build the Riptide agent"
	@echo "  build-viewer    Build the Session Viewer (backend + frontend)"
	@echo "  frontend-build  Build the Lit frontend"
	@echo "  test            Run Go tests"
	@echo "  clean           Clean build artifacts"
	@echo "  run-viewer      Start the Session Viewer"

build: build-agent build-viewer

build-agent: | $(BIN_DIR)
	go build -o $(BIN_DIR)/riptide main.go

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

run-viewer: build-viewer
	./$(BIN_DIR)/session-viewer

clean:
	rm -rf $(BIN_DIR)
	rm -rf frontend/dist
