.PHONY: build test test-integration test-all clean fmt lint tidy help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOFMTBIN=gofmt

# Binary name
BINARY_NAME=tekton-results-mcp-server
MAIN_PATH=./cmd/tekton-results-mcp-server

## help: Display this help message
help:
	@echo "Available targets:"
	@echo "  build             - Build the main binary"
	@echo "  test              - Run unit tests"
	@echo "  test-integration  - Run integration tests (with mock servers)"
	@echo "  test-all          - Run all tests (unit + integration)"
	@echo "  fmt               - Format Go code (excludes vendor)"
	@echo "  clean             - Remove build artifacts (optional)"
	@echo "  lint              - Run code formatting and linting"
	@echo "  tidy              - Tidy and vendor Go modules (only if you added/removed imports)"
	@echo "  help              - Display this help message"

## build: Build the main binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -v -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "Build completed: ./$(BINARY_NAME)"

## test: Run unit tests
test:
	@echo "Running unit tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Unit tests completed"

## test-integration: Run integration tests with mock servers
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -race -tags=integration ./internal/tektonresults/...
	@echo "Integration tests completed"

## test-all: Run all tests (unit + integration)
test-all:
	@echo "Running all tests..."
	$(GOTEST) -v -race -tags=integration -coverprofile=coverage-all.out ./...
	@echo "All tests completed"

## fmt: Format Go code (excludes vendor directory)
fmt:
	@echo "Formatting Go code..."
	@find . -name '*.go' -not -path './vendor/*' -exec $(GOFMTBIN) -w {} \;
	@echo "Code formatted"

## clean: Remove build artifacts (optional, binary is git-ignored)
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -f coverage.out coverage-all.out tekton-results-mcp-server
	@echo "Clean completed"
	@echo "Note: Binary is ignored by git, so cleaning is optional"

## lint: Run code formatting and linting
lint:
	@echo "Running code formatting..."
	$(GOFMT) ./...
	@echo "Checking with go vet..."
	$(GOCMD) vet ./...
	@echo "Linting completed"

## tidy: Tidy and vendor Go modules (only run if you added/removed imports)
tidy:
	@echo "Tidying Go modules..."
	$(GOMOD) tidy
	$(GOMOD) vendor
	@echo "Modules tidied and vendored"
	@echo "Note: Only run this if you added or removed imports in your changes"

# Default target
.DEFAULT_GOAL := help

