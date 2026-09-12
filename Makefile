# Variables
BINARY_NAME=Elden Ring SaveForge
VERSION=1.8.1
BUILD_DIR=build/bin
WAILS ?= ~/go/bin/wails
OUTPUT ?= $(BINARY_NAME)
WAILS_PLATFORM_FLAG=$(if $(PLATFORM),-platform $(PLATFORM),)
CLEAN_GOCACHE=$(CURDIR)/.cache/go-build

.PHONY: all generate-version generate-bindings frontend-build build dev test test-go test-frontend lint clean deps help

all: deps build test

# Generate app version source from Makefile VERSION.
generate-version:
	go run ./scripts/generate_app_version.go

# Generate Wails bindings once, then normalize its known models.ts whitespace
# and runtime file-mode churn. build/dev use -skipbindings so Wails does not
# regenerate dirty output.
generate-bindings: frontend-build
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
	$(WAILS) build -skipbindings -m $(WAILS_PLATFORM_FLAG) -o "$(OUTPUT)"

# Build assets embedded by the root Go package.
frontend-build:
	npm --prefix frontend run build

# Run Wails in development mode (hot reload)
dev: generate-version generate-bindings
	$(WAILS) dev -skipbindings -m

# Run all tests without traversing ignored scratch packages under tmp/.
test: test-go test-frontend

test-go: frontend-build
	@echo "🧪 Running Go tests..."
	go test -count=1 -v . ./internal/application ./backend/... ./tests/...
	go test -count=1 ./scripts/clean-artifacts

test-frontend:
	@echo "🧪 Running frontend tests..."
	npm --prefix frontend test

# Run linter (requires golangci-lint installed) without traversing tmp/.
lint: frontend-build
	@echo "🔍 Running linter..."
	golangci-lint run . ./internal/application ./backend/... ./tests/... ./scripts/clean-artifacts

# Remove only known project-local generated files, build output and caches.
# tmp/ and tracked Wails bindings are intentionally outside this list.
clean:
	@echo "🧹 Cleaning generated files, build output and local caches..."
	GOCACHE="$(CLEAN_GOCACHE)" go run ./scripts/clean-artifacts

# Help command
help:
	@echo "Available commands:"
	@echo "  make generate-version - Generate app version source from Makefile"
	@echo "  make deps          - Install Go and Frontend dependencies"
	@echo "  make frontend-build - Build assets embedded by the Go application"
	@echo "  make build         - Build the app for current platform"
	@echo "  make dev           - Run app in development mode"
	@echo "  make test          - Run Go and frontend tests (excluding tmp/)"
	@echo "  make test-go       - Run Go tests (excluding tmp/)"
	@echo "  make test-frontend - Run frontend tests"
	@echo "  make lint          - Run linter (excluding tmp/)"
	@echo "  make clean         - Remove generated files, build output and local caches"
