# Riptide Makefile

.PHONY: all build build-agent build-viewer frontend-build test clean help

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

build-agent:
	go build -o riptide main.go

build-viewer: frontend-build
	go build -o session-viewer cmd/session-viewer/main.go

frontend-build:
	@echo "Building frontend..."
	@cd frontend && [ -d node_modules ] || npm install
	@cd frontend && npm run build

test:
	go test -v ./pkg/...

run-viewer: build-viewer
	./session-viewer

clean:
	rm -f riptide session-viewer
	rm -rf frontend/dist
