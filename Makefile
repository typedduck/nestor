# Makefile for nestor provisioning tool

# Build configuration
BINARY_NAME=nestor
AGENT_BINARY=nestor-agent
BUILD_DIR=build
GO=go
GOFLAGS=-v

# Version information
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u -Iseconds)
# BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%S')

# Linker flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: all clean test test-integration build build-controller build-agent install help

all: build

help:
	@echo "Available targets:"
	@echo "  build             - Build both controller and agent"
	@echo "  build-controller  - Build controller binary"
	@echo "  build-agent       - Build agent binary"
	@echo "  test              - Run unit tests"
	@echo "  test-integration  - Run integration tests (requires Docker or Podman)"
	@echo "  test-verbose      - Run tests with verbose output"
	@echo "  coverage          - Generate test coverage report"
	@echo "  clean             - Remove build artifacts"
	@echo "  install           - Install binaries to GOPATH/bin"
	@echo "  example-package   - Run the package example"
	@echo "  example-file      - Run the file example"
	@echo "  examples          - Run all examples"

# Build both controller and agent
build: build-controller build-agent

# Build controller
build-controller:
	@echo "Building controller..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/controller/main.go

# Build agent
build-agent:
	@echo "Building agent for native platform..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) \
	-o $(BUILD_DIR)/$(AGENT_BINARY) ./cmd/agent/main.go
	@echo "Building agent for Linux on amd64..."
	GOOS=linux GOARCH=amd64 \
	$(GO) build $(GOFLAGS) $(LDFLAGS) \
	-o $(BUILD_DIR)/$(AGENT_BINARY)-linux-amd64 ./cmd/agent/main.go
	@echo "Building agent for Linux on arm64..."
	GOOS=linux GOARCH=arm64 \
	$(GO) build $(GOFLAGS) $(LDFLAGS) \
	-o $(BUILD_DIR)/$(AGENT_BINARY)-linux-arm64 ./cmd/agent/main.go

# Run all tests
test:
	@echo "Running tests..."
	$(GO) test ./... -v

# Run integration tests (requires Docker or Podman)
test-integration:
	@echo "Running integration tests..."
	@eval `cat .env`
	$(GO) test -tags integration -v -timeout 10m ./tests/integration/...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	$(GO) test ./... -v -cover

# Generate coverage report
coverage:
	@echo "Generating coverage report..."
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run the package example
example-package:
	@echo "Running package example..."
	$(GO) run examples/package/main.go

# Run the file example
example-file:
	@echo "Running file example..."
	$(GO) run examples/file/main.go

# Run all examples
examples: example-package example-file

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Install binaries
install: build
	@echo "Installing binaries..."
	$(GO) install ./cmd/controller
	$(GO) install ./cmd/agent

# Development helpers
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

vet:
	@echo "Running go vet..."
	$(GO) vet ./...

# Static analysis
lint: fmt vet
	@echo "Code formatting and vetting complete"
