# Makefile for Milo - A coding agent CLI

# Variables
BINARY_NAME=milo
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*')
GOFLAGS=-ldflags="-s -w"

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build: $(BINARY_NAME)

$(BINARY_NAME): $(GO_FILES) go.mod go.sum
	go build $(GOFLAGS) -o $(BINARY_NAME) .

# Run tests
.PHONY: test
test:
	go test ./...

# Run tests with verbose output
.PHONY: test-verbose
test-verbose:
	go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	go test -cover ./...

# Run tests with coverage report
.PHONY: test-coverage-report
test-coverage-report:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Format Go code
.PHONY: fmt
fmt:
	gofmt -w .

# Tidy dependencies
.PHONY: tidy
tidy:
	go mod tidy

# Run static analysis
.PHONY: vet
vet:
	go vet ./...

# Run all quality checks
.PHONY: check
check: fmt vet test

# Install dependencies
.PHONY: deps
deps:
	go mod download

# Run the binary (useful for development)
.PHONY: run
run: build
	./$(BINARY_NAME)

# Development watch mode (requires entr: brew install entr)
.PHONY: watch
watch:
	find . -name '*.go' | entr -r make run

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build              Build the binary (default)"
	@echo "  test               Run tests"
	@echo "  test-verbose       Run tests with verbose output"
	@echo "  test-coverage      Run tests with coverage"
	@echo "  test-coverage-report Generate HTML coverage report"
	@echo "  clean              Clean build artifacts"
	@echo "  fmt                Format Go code"
	@echo "  tidy               Tidy dependencies"
	@echo "  vet                Run static analysis"
	@echo "  check              Run fmt, vet, and test"
	@echo "  deps               Install dependencies"
	@echo "  run                Build and run the binary"
	@echo "  watch              Watch for changes and rebuild (requires entr)"
	@echo "  help               Show this help message"

# Ensure the binary is rebuilt if go.mod or go.sum changes
$(BINARY_NAME): go.mod go.sum