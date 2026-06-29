# Variables
BINARY_NAME=Elden Ring SaveForge
VERSION=1.0.0-beta6
BUILD_DIR=build/bin
WAILS ?= ~/go/bin/wails
NODE ?= node
OUTPUT ?= $(BINARY_NAME)
WAILS_PLATFORM_FLAG=$(if $(PLATFORM),-platform $(PLATFORM),)

.PHONY: all sync-version build dev test lint clean deps help

all: deps build test

# Sync VERSION into app metadata and frontend display.
sync-version:
	@$(NODE) scripts/sync-version.mjs "$(VERSION)"

# Install dependencies for both Go and Frontend
deps:
	@echo "📥 Installing dependencies..."
	go mod tidy
	cd frontend && npm install

# Build the application for the current platform
build: sync-version
	@echo "🔨 Building $(BINARY_NAME) v$(VERSION)..."
	$(WAILS) build $(WAILS_PLATFORM_FLAG) -o "$(OUTPUT)"

# Run Wails in development mode (hot reload)
dev: sync-version
	$(WAILS) dev

# Run all tests
test:
	@echo "🧪 Running unit tests..."
	go test -v ./backend/...
	@echo "🧪 Running round-trip validation tests..."
	go test -v ./tests/roundtrip_test.go

# Run linter (requires golangci-lint installed)
lint:
	@echo "🔍 Running linter..."
	golangci-lint run ./...

# Clean build artifacts
clean:
	@echo "🧹 Cleaning up..."
	rm -rf build/bin/*
	rm -rf frontend/dist

# Help command
help:
	@echo "Available commands:"
	@echo "  make sync-version - Sync VERSION into metadata and frontend"
	@echo "  make deps         - Install Go and Frontend dependencies"
	@echo "  make build        - Build the app for current platform"
	@echo "  make dev          - Run app in development mode"
	@echo "  make test         - Run all tests"
	@echo "  make lint         - Run linter"
	@echo "  make clean        - Remove build artifacts"
