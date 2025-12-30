# Contributing to Tekton Results MCP Server

Thank you for your interest in contributing! This document provides guidelines and instructions for contributing to the project.

## Before You Start

- Read the [README.md](README.md) to understand the project's purpose and architecture
- Check existing [issues](https://github.com/enarha/tekton-results-mcp-server/issues) to see if your idea or bug has been discussed
- For major changes, open an issue first to discuss your proposed changes

## Quick Start for Contributors

**Standard pre-PR workflow** (run this before every PR):
```bash
make build && make test-all && make lint
```

**Only if you added/removed imports:**
```bash
# After the above passes, tidy dependencies
make tidy
```

## Development Setup

### Prerequisites

- Go 1.24.3 or later
- Access to a Kubernetes cluster with Tekton Pipelines and Tekton Results installed (for manual testing)
- `kubectl` configured to access your cluster

### Getting Started

1. Fork and clone the repository:
   ```bash
   git clone https://github.com/YOUR_USERNAME/tekton-results-mcp-server.git
   cd tekton-results-mcp-server
   ```

2. Build the project:
   ```bash
   make build
   ```

## Running Tests

### Local Testing

Before submitting a pull request, **always** run the full test suite locally:

```bash
# Check: build, all tests, and linting
make build && make test-all && make lint
```

### Individual Test Commands

```bash
# Build the binary (verifies main.go compiles)
make build

# Run unit tests only
make test

# Run integration tests only (uses mock HTTP servers)
make test-integration

# Run all tests with coverage
make test-all

# Check code formatting and run go vet
make lint

# Clean up build artifacts (optional, not required for routine work)
make clean
```

**Note**: You don't need to run `make clean` after building or testing. The binary is ignored by git and won't interfere with your work. Run `make clean` only if you want to free up disk space or force a complete rebuild.

### When to Tidy and Vendor Dependencies

**Run `make tidy` if**:
- You added new imports to your code
- You removed imports from your code
- Your PR explicitly involves updating dependencies

This will:
1. Run `go mod tidy` to update `go.mod` and `go.sum`
2. Run `go mod vendor` to update the `vendor/` directory

## Making Changes

### Code Style

- Follow standard Go conventions
- Run `go fmt ./...` before committing (or use `make lint`)
- Run `go vet ./...` to catch common mistakes
- Use meaningful variable and function names
- Add comments for exported functions and complex logic

### Writing Tests

**All new code must include tests.** Follow these guidelines:

1. **Unit tests** for new functions in `internal/tektonresults/`:
   - Test happy path and error cases
   - Use table-driven tests for multiple scenarios
   - Mock external dependencies

2. **Integration tests** for API interactions:
   - Use `httptest.NewServer` to mock the Tekton Results API
   - Test the full request/response cycle
   - Verify correct API paths and parameters

3. **Tool handler tests** for new MCP tools:
   - Mock the service layer using the `Service` interface
   - Test parameter validation
   - Test error handling
   - Test output formatting (YAML/JSON)

### Test Coverage

- Aim for reasonable test coverage of new code
- Focus on testing behavior, not implementation details
- Don't sacrifice test quality for coverage percentage

## Submitting a Pull Request

### Pre-submission Checklist

Before opening a PR, ensure:

- [ ] Code builds successfully: `make build`
- [ ] All tests pass: `make test-all`
- [ ] Code is properly formatted: `make lint`
- [ ] New tests are added for new functionality
- [ ] Existing tests still pass
- [ ] Documentation is updated (README.md, code comments)
- [ ] Commit messages are clear and descriptive
- [ ] Dependencies tidied only if you added/removed imports: `make tidy` (optional)

## Continuous Integration

All pull requests trigger automated CI checks via GitHub Actions:

## Getting Help

- **Questions?** Open an issue with the `question` label
- **Bug reports?** Open an issue with the `bug` label
- **Feature requests?** Open an issue with the `enhancement` label

## Code of Conduct

Be respectful and constructive in all interactions. We're here to build great software together!

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.
