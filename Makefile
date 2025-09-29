# JIRA CDC Git Sync - Production Makefile
# Supports development workflow and CI/CD pipeline

# Build configuration
BINARY_NAME=jira-sync
API_BINARY_NAME=api-server
DOCKER_IMAGE=jira-cdc-git
VERSION?=v0.4.0
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

# Container runtime configuration (prefer podman, fallback to docker)
CONTAINER_RUNTIME=$(shell command -v podman >/dev/null 2>&1 && echo podman || echo docker)

# Go configuration
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Directories
BUILD_DIR=./build
CMD_DIR=./cmd/jira-sync
API_CMD_DIR=./cmd/api-server
COVERAGE_DIR=./coverage

# Git information for build metadata
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Default target
.PHONY: all
all: clean deps lint test build build-api

# Development workflow targets
.PHONY: dev
dev: deps lint test build build-api run

.PHONY: quick
quick: test build build-api

# Dependency management
.PHONY: deps
deps:
	@echo "📦 Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

.PHONY: deps-update
deps-update:
	@echo "🔄 Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Code quality
.PHONY: fmt
fmt:
	@echo "🎨 Formatting code..."
	$(GOFMT) -s -w .

.PHONY: lint
lint:
	@echo "🔍 Running linters..."
	$(GOLINT) run ./...

.PHONY: lint-fix
lint-fix:
	@echo "🔧 Auto-fixing lint issues..."
	$(GOLINT) run --fix ./...

# Testing
.PHONY: test
test:
	@echo "🧪 Running tests..."
	mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -coverprofile=$(COVERAGE_DIR)/coverage.out ./...

.PHONY: test-race
test-race:
	@echo "🧪 Running tests with race detection (core functionality only)..."
	mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -race -skip="Performance" -coverprofile=$(COVERAGE_DIR)/coverage.out ./...

.PHONY: test-coverage
test-coverage: test
	@echo "📊 Generating coverage report..."
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report: $(COVERAGE_DIR)/coverage.html"

.PHONY: test-unit
test-unit:
	@echo "🧪 Running unit tests..."
	$(GOTEST) -v -short ./...

.PHONY: test-integration
test-integration:
	@echo "🔗 Running integration tests..."
	$(GOTEST) -v -run Integration ./test/...

.PHONY: test-watch
test-watch:
	@echo "👀 Watching for changes and running tests..."
	find . -name "*.go" | entr -c make test-unit

# Build targets
.PHONY: build
build: fmt lint clean-build
	@echo "🔨 Building $(BINARY_NAME)..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

.PHONY: build-api
build-api: clean-build
	@echo "🔨 Building $(API_BINARY_NAME)..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(API_BINARY_NAME) $(API_CMD_DIR)

.PHONY: build-linux
build-linux: clean-build
	@echo "🐧 Building for Linux..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(API_BINARY_NAME)-linux-amd64 $(API_CMD_DIR)

.PHONY: build-darwin
build-darwin: clean-build
	@echo "🍎 Building for macOS..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(API_BINARY_NAME)-darwin-amd64 $(API_CMD_DIR)

.PHONY: build-windows
build-windows: clean-build
	@echo "🪟 Building for Windows..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(API_BINARY_NAME)-windows-amd64.exe $(API_CMD_DIR)

.PHONY: build-all
build-all: build-linux build-darwin build-windows

# Container targets (supports both podman and docker)
.PHONY: container-build
container-build:
	@echo "📦 Building container image with $(CONTAINER_RUNTIME)..."
	$(CONTAINER_RUNTIME) build -t $(DOCKER_IMAGE):$(VERSION) .
	$(CONTAINER_RUNTIME) tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

.PHONY: container-run
container-run:
	@echo "📦 Running container with $(CONTAINER_RUNTIME)..."
	$(CONTAINER_RUNTIME) run --rm -it $(DOCKER_IMAGE):latest

# Legacy Docker targets (for compatibility)
.PHONY: docker-build
docker-build: container-build

.PHONY: docker-run
docker-run: container-run

# Podman-specific targets
.PHONY: podman-build
podman-build:
	@echo "🐳 Building image with Podman..."
	podman build -t $(DOCKER_IMAGE):$(VERSION) .
	podman tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

.PHONY: podman-run
podman-run:
	@echo "🐳 Running container with Podman..."
	podman run --rm -it $(DOCKER_IMAGE):latest

# Container registry operations
.PHONY: container-push
container-push:
	@echo "📤 Pushing container image with $(CONTAINER_RUNTIME)..."
	$(CONTAINER_RUNTIME) push $(DOCKER_IMAGE):$(VERSION)
	$(CONTAINER_RUNTIME) push $(DOCKER_IMAGE):latest

.PHONY: container-info
container-info:
	@echo "Container Runtime: $(CONTAINER_RUNTIME)"
	@$(CONTAINER_RUNTIME) --version

# Kubernetes targets (for future use)
.PHONY: k8s-deploy
k8s-deploy:
	@echo "☸️ Deploying to Kubernetes..."
	kubectl apply -f deployments/

.PHONY: k8s-undeploy
k8s-undeploy:
	@echo "☸️ Removing from Kubernetes..."
	kubectl delete -f deployments/

# Runtime targets
.PHONY: run
run: build
	@echo "🚀 Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME) --help

.PHONY: run-api
run-api: build-api
	@echo "🚀 Starting API server..."
	./$(BUILD_DIR)/$(API_BINARY_NAME) serve --help

.PHONY: run-example
run-example: build
	@echo "💡 Running example sync..."
	@echo "Note: Configure .env file first"
	# ./$(BUILD_DIR)/$(BINARY_NAME) --issue=PROJ-123 --repo=./test-repo

# Cleanup
.PHONY: clean
clean: clean-build clean-test

.PHONY: clean-build
clean-build:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)

.PHONY: clean-test
clean-test:
	@echo "🧹 Cleaning test artifacts..."
	rm -rf $(COVERAGE_DIR)

.PHONY: clean-deps
clean-deps:
	@echo "🧹 Cleaning dependency cache..."
	$(GOCLEAN) -modcache

# CI/CD pipeline targets
.PHONY: ci-deps
ci-deps:
	@echo "🔄 CI: Installing dependencies..."
	$(GOMOD) download

.PHONY: ci-lint
ci-lint:
	@echo "🔍 CI: Running linters..."
	$(GOLINT) run --timeout=5m ./...

.PHONY: ci-test
ci-test:
	@echo "🧪 CI: Running tests with coverage..."
	mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GOCMD) tool cover -func=$(COVERAGE_DIR)/coverage.out

.PHONY: ci-build
ci-build:
	@echo "🔨 CI: Building application..."
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

.PHONY: ci-security
ci-security:
	@echo "🔒 CI: Running security scan..."
	# Add security scanning tool here (gosec, nancy, etc.)

.PHONY: ci-pipeline
ci-pipeline: ci-deps ci-lint ci-test ci-security ci-build

# Validation targets
.PHONY: validate-all
validate-all: deps fmt lint test build
	@echo "✅ All validation checks passed!"

.PHONY: pre-commit
pre-commit: fmt lint test-unit
	@echo "✅ Pre-commit checks passed!"

# Help
.PHONY: help
help:
	@echo "JIRA CDC Git Sync - Available Commands:"
	@echo ""
	@echo "Development:"
	@echo "  dev          - Full development workflow (deps, lint, test, build, run)"
	@echo "  quick        - Quick build and test"
	@echo "  deps         - Install dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linters"
	@echo "  test         - Run all tests with coverage"
	@echo "  build        - Build binary"
	@echo "  run          - Build and run application"
	@echo ""
	@echo "Testing:"
	@echo "  test-unit    - Run unit tests only"
	@echo "  test-integration - Run integration tests"
	@echo "  test-coverage - Generate coverage report"
	@echo "  test-watch   - Watch for changes and run tests"
	@echo ""
	@echo "Building:"
	@echo "  build-all    - Build for all platforms"
	@echo "  build-linux  - Build for Linux"
	@echo "  build-darwin - Build for macOS"
	@echo "  build-windows - Build for Windows"
	@echo ""
	@echo "Container & Kubernetes:"
	@echo "  container-build - Build container image (podman/docker)"
	@echo "  container-run   - Run container"
	@echo "  container-info  - Show container runtime info"
	@echo "  docker-build    - Legacy docker build (uses container-build)"
	@echo "  podman-build    - Force Podman build"
	@echo "  k8s-deploy      - Deploy to Kubernetes"
	@echo ""
	@echo "CI/CD:"
	@echo "  ci-pipeline  - Run full CI pipeline"
	@echo "  validate-all - Run all validation checks"
	@echo "  pre-commit   - Run pre-commit checks"
	@echo ""
	@echo "Cleanup:"
	@echo "  clean        - Clean all artifacts"