# Variables
BINARY_NAME=Elden Ring SaveForge
VERSION=1.5.7
BUILD_DIR=build/bin
WAILS ?= ~/go/bin/wails
OUTPUT ?= $(BINARY_NAME)
WAILS_PLATFORM_FLAG=$(if $(PLATFORM),-platform $(PLATFORM),)

.PHONY: all generate-version generate-bindings build dev test lint clean deps help

all: deps build test

# Generate app version source from Makefile VERSION.
generate-version:
	go run ./scripts/generate_app_version.go

# Generate Wails bindings once, then normalize its known models.ts whitespace
# and runtime file-mode churn. build/dev use -skipbindings so Wails does not
# regenerate dirty output.
generate-bindings:
	$(WAILS) generate module
	go run ./scripts/normalize_wails_models.go

# Install dependencies for both Go and Frontend
deps:
	@echo "📥 Installing dependencies..."
	go mod tidy
	cd frontend && npm install

# Build the application for the current platform
build: generate-version generate-bindings
	@echo "🔨 Building $(BINARY_NAME) v$(VERSION)..."
	$(WAILS) build -skipbindings $(WAILS_PLATFORM_FLAG) -o "$(OUTPUT)"

# Run Wails in development mode (hot reload)
dev: generate-version generate-bindings
	$(WAILS) dev -skipbindings

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
	@echo "  make generate-version - Generate app version source from Makefile"
	@echo "  make deps         - Install Go and Frontend dependencies"
	@echo "  make build        - Build the app for current platform"
	@echo "  make dev          - Run app in development mode"
	@echo "  make test         - Run all tests"
	@echo "  make lint         - Run linter"
	@echo "  make clean        - Remove build artifacts"
