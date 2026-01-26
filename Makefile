.PHONY: build test lint clean run dev docker helm

# Build variables
BINARY_NAME=chronos
CLI_NAME=chronosctl
VERSION?=0.1.0
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Directories
CMD_DIR=./cmd
BUILD_DIR=./build
WEB_DIR=./web

# Default target
all: lint test build

# Build the server binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/chronos

# Build the CLI binary
build-cli:
	@echo "Building $(CLI_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME) $(CMD_DIR)/chronosctl

# Build all binaries
build-all: build build-cli

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -race -tags=integration ./...

# Run end-to-end tests
test-e2e:
	@echo "Running E2E tests..."
	cd e2e && $(GOTEST) -v ./...

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Run the server locally
run: build
	@echo "Running $(BINARY_NAME)..."
	$(BUILD_DIR)/$(BINARY_NAME) --config chronos.yaml

# Run with hot reload (requires air)
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# Build web UI
web:
	@echo "Building web UI..."
	cd $(WEB_DIR) && npm install && npm run build

# Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t chronos:$(VERSION) -f deployments/docker/Dockerfile .

# Package Helm chart
helm:
	@echo "Packaging Helm chart..."
	helm package deployments/helm/chronos -d $(BUILD_DIR)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Generate API documentation
docs:
	@echo "Generating API docs..."
	@which swag > /dev/null || (echo "Installing swag..." && go install github.com/swaggo/swag/cmd/swag@latest)
	swag init -g cmd/chronos/main.go -o docs/api

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build the server binary"
	@echo "  build-cli      - Build the CLI binary"
	@echo "  build-all      - Build all binaries"
	@echo "  test           - Run unit tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-integration - Run integration tests"
	@echo "  test-e2e       - Run end-to-end tests"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  deps           - Download dependencies"
	@echo "  run            - Build and run the server"
	@echo "  dev            - Run with hot reload"
	@echo "  web            - Build web UI"
	@echo "  docker         - Build Docker image"
	@echo "  helm           - Package Helm chart"
	@echo "  clean          - Clean build artifacts"
	@echo "  docs           - Generate API documentation"
