.PHONY: test test-integration test-all clean lint help

# Go parameters
GOCMD=go
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

## help: Display this help message
help:
	@echo "Available targets:"
	@echo "  test              - Run unit tests"
	@echo "  test-integration  - Run integration tests (with mock servers)"
	@echo "  test-all          - Run all tests (unit + integration)"
	@echo "  clean             - Remove build artifacts"
	@echo "  lint              - Run code formatting and linting"
	@echo "  tidy              - Tidy and vendor Go modules"
	@echo "  help              - Display this help message"

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

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -f coverage.out coverage-all.out tekton-results-mcp-server
	@echo "Clean completed"

## lint: Run code formatting and linting
lint:
	@echo "Running code formatting..."
	$(GOFMT) ./...
	@echo "Checking with go vet..."
	$(GOCMD) vet ./...
	@echo "Linting completed"

## tidy: Tidy and vendor Go modules
tidy:
	@echo "Tidying Go modules..."
	$(GOMOD) tidy
	$(GOMOD) vendor
	@echo "Modules tidied"

# Default target
.DEFAULT_GOAL := help

