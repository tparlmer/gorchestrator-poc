BINARY_NAME=overnight-llm
VERSION=0.1.0
BUILD_TIME=$(shell date +%FT%T%z)
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build the binary
.PHONY: build
build:
	CGO_ENABLED=1 $(GOBUILD) -o $(BINARY_NAME) $(LDFLAGS) ./cmd/generator

# Build for multiple platforms
.PHONY: build-all
build-all: build-darwin-arm64 build-darwin-amd64 build-linux-amd64

.PHONY: build-darwin-arm64
build-darwin-arm64:
	@echo "Building for Mac ARM64..."
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GOBUILD) -o dist/$(BINARY_NAME)-mac-arm64 $(LDFLAGS) ./cmd/generator

.PHONY: build-darwin-amd64
build-darwin-amd64:
	@echo "Building for Mac AMD64..."
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) -o dist/$(BINARY_NAME)-mac-amd64 $(LDFLAGS) ./cmd/generator

.PHONY: build-linux-amd64
build-linux-amd64:
	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) -o dist/$(BINARY_NAME)-linux-amd64 $(LDFLAGS) ./cmd/generator

# Run tests
.PHONY: test
test:
	$(GOTEST) -v -cover ./...

# Run tests with coverage report
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Download dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run the application with default settings
.PHONY: run
run: build
	./$(BINARY_NAME) -output ./demo

# Run with custom model
.PHONY: run-small
run-small: build
	./$(BINARY_NAME) -output ./demo -model deepseek-coder:1.3b

# Clean build artifacts and generated files
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -rf generated/
	rm -rf demo/
	rm -f poc.db
	rm -f coverage.out coverage.html
	rm -f status.json

# Format code
.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

# Run linter
.PHONY: lint
lint:
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "golangci-lint not installed. Install it from https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi
	golangci-lint run

# Vet code for suspicious constructs
.PHONY: vet
vet:
	$(GOCMD) vet ./...

# Install the binary to GOPATH/bin
.PHONY: install
install:
	CGO_ENABLED=1 $(GOCMD) install $(LDFLAGS) ./cmd/generator

# Check if Ollama is running
.PHONY: check-ollama
check-ollama:
	@echo "Checking Ollama status..."
	@curl -s http://localhost:11434/api/tags > /dev/null && echo "OK: Ollama is running" || echo "ERROR: Ollama is not running. Start it with: ollama serve"

# Pull required models
.PHONY: setup-models
setup-models:
	@echo "Pulling recommended models..."
	ollama pull codellama:7b
	@echo "OK: Models ready"

# Full setup: deps, models, and verification
.PHONY: setup
setup: deps setup-models check-ollama
	@echo ""
	@echo "OK: Setup complete! You can now run: make run"

# Development mode - rebuild and run on changes (requires entr)
.PHONY: dev
dev:
	@if ! command -v entr &> /dev/null; then \
		echo "entr not installed. Install it with: brew install entr (Mac) or apt-get install entr (Linux)"; \
		exit 1; \
	fi
	find . -name '*.go' | entr -r make run

# Show help
.PHONY: help
help:
	@echo "Overnight LLM Code Generator - Makefile targets"
	@echo ""
	@echo "Setup & Dependencies:"
	@echo "  make setup          - Complete setup (deps, models, check)"
	@echo "  make deps           - Download Go dependencies"
	@echo "  make setup-models   - Pull required Ollama models"
	@echo "  make check-ollama   - Check if Ollama is running"
	@echo ""
	@echo "Building:"
	@echo "  make build          - Build the binary"
	@echo "  make build-all      - Build for all platforms"
	@echo "  make install        - Install to GOPATH/bin"
	@echo ""
	@echo "Running:"
	@echo "  make run            - Run with default settings"
	@echo "  make run-small      - Run with smaller model"
	@echo "  make dev            - Development mode (auto-rebuild)"
	@echo ""
	@echo "Testing & Quality:"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Generate coverage report"
	@echo "  make fmt            - Format code"
	@echo "  make vet            - Run go vet"
	@echo "  make lint           - Run linter (requires golangci-lint)"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean          - Remove build artifacts"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  BINARY_NAME=$(BINARY_NAME)"

# Default target
.DEFAULT_GOAL := help