#!/bin/bash
set -e

echo "Building Frontend..."
cd frontend
npm install
npm run build
cd ..

echo "Starting Session Viewer..."
go run cmd/session-viewer/main.go
