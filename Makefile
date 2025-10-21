# JIRA CDC Git Sync - Production Makefile
# Supports development workflow and CI/CD pipeline

# Build configuration
BINARY_NAME=jira-sync
API_BINARY_NAME=api-server
OPERATOR_BINARY_NAME=operator
DOCKER_IMAGE=jira-cdc-git
VERSION?=v0.4.1
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

# Container runtime configuration (prefer podman, fallback to docker)
CONTAINER_RUNTIME=$(shell command -v podman >/dev/null 2>&1 && echo podman || echo docker)

# Kind configuration
KIND_CLUSTER_NAME?=jira-sync-v041-demo

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
OPERATOR_CMD_DIR=./cmd/operator
COVERAGE_DIR=./coverage

# Git information for build metadata
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Default target
.PHONY: all
all: clean deps lint test build build-api build-operator

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
	SKIP_K8S_INTEGRATION=1 $(GOTEST) -v -p 1 -count=1 -coverprofile=$(COVERAGE_DIR)/coverage.out ./...

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

.PHONY: test-operator
test-operator:
	@echo "☸️ Running operator tests..."
	$(GOTEST) -v ./internal/operator/...

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

.PHONY: build-operator
build-operator: clean-build
	@echo "🔨 Building $(OPERATOR_BINARY_NAME)..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(OPERATOR_BINARY_NAME) $(OPERATOR_CMD_DIR)

.PHONY: build-linux
build-linux: clean-build
	@echo "🐧 Building for Linux..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(API_BINARY_NAME)-linux-amd64 $(API_CMD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(OPERATOR_BINARY_NAME)-linux-amd64 $(OPERATOR_CMD_DIR)

.PHONY: build-darwin
build-darwin: clean-build
	@echo "🍎 Building for macOS..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(API_BINARY_NAME)-darwin-amd64 $(API_CMD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(OPERATOR_BINARY_NAME)-darwin-amd64 $(OPERATOR_CMD_DIR)

.PHONY: build-windows
build-windows: clean-build
	@echo "🪟 Building for Windows..."
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(API_BINARY_NAME)-windows-amd64.exe $(API_CMD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(OPERATOR_BINARY_NAME)-windows-amd64.exe $(OPERATOR_CMD_DIR)

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

# API Server container targets
.PHONY: api-image-build
api-image-build: build-api
	@echo "📦 Building API server container image with $(CONTAINER_RUNTIME)..."
	$(CONTAINER_RUNTIME) build -t localhost/jira-sync-api:$(VERSION) -f deployments/api-server/Dockerfile .
	$(CONTAINER_RUNTIME) tag localhost/jira-sync-api:$(VERSION) localhost/jira-sync-api:latest

.PHONY: api-image-load
api-image-load: api-image-build
	@echo "📦 Loading API server image into kind cluster..."
	kind load docker-image localhost/jira-sync-api:$(VERSION) --name $(KIND_CLUSTER_NAME)

.PHONY: api-image-push
api-image-push: api-image-build
	@echo "📤 Pushing API server image..."
	$(CONTAINER_RUNTIME) push localhost/jira-sync-api:$(VERSION)
	$(CONTAINER_RUNTIME) push localhost/jira-sync-api:latest

# Operator container targets
.PHONY: operator-image-build
operator-image-build: build-operator
	@echo "📦 Building operator container image with $(CONTAINER_RUNTIME)..."
	$(CONTAINER_RUNTIME) build -t localhost/jira-sync-operator:$(VERSION) -f deployments/operator/Dockerfile .
	$(CONTAINER_RUNTIME) tag localhost/jira-sync-operator:$(VERSION) localhost/jira-sync-operator:latest

.PHONY: operator-image-load
operator-image-load: operator-image-build
	@echo "📦 Loading operator image into kind cluster..."
	kind load docker-image localhost/jira-sync-operator:$(VERSION) --name $(KIND_CLUSTER_NAME)

.PHONY: operator-image-push
operator-image-push: operator-image-build
	@echo "📤 Pushing operator image..."
	$(CONTAINER_RUNTIME) push localhost/jira-sync-operator:$(VERSION)
	$(CONTAINER_RUNTIME) push localhost/jira-sync-operator:latest

# Combined image targets
.PHONY: images-build
images-build: api-image-build operator-image-build
	@echo "✅ All component images built successfully"

.PHONY: images-load
images-load: api-image-load operator-image-load
	@echo "✅ All component images loaded into kind cluster"

.PHONY: images-push
images-push: api-image-push operator-image-push
	@echo "✅ All component images pushed to registry"

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

# Kubernetes deployment targets
.PHONY: k8s-deploy
k8s-deploy:
	@echo "☸️ Deploying to Kubernetes..."
	kubectl apply -f deployments/

.PHONY: k8s-undeploy
k8s-undeploy:
	@echo "☸️ Removing from Kubernetes..."
	kubectl delete -f deployments/

# Kind cluster management
.PHONY: kind-cluster-create
kind-cluster-create:
	@echo "🚀 Creating kind cluster: $(KIND_CLUSTER_NAME)"
	kind create cluster --name $(KIND_CLUSTER_NAME) --config deployments/kind/cluster.yaml

.PHONY: kind-cluster-delete
kind-cluster-delete:
	@echo "🧹 Deleting kind cluster: $(KIND_CLUSTER_NAME)"
	kind delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: kind-cluster-reset
kind-cluster-reset: kind-cluster-delete kind-cluster-create
	@echo "🔄 Kind cluster reset complete"

# Operator deployment workflow
.PHONY: operator-deploy-prep
operator-deploy-prep:
	@echo "📋 Preparing operator deployment..."
	kubectl create namespace jira-sync-v040 --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f crds/v1alpha1/

.PHONY: operator-deploy
operator-deploy: images-load operator-deploy-prep
	@echo "☸️ Deploying JIRA Sync Operator..."
	helm upgrade --install jira-operator-demo deployments/operator/ \
		--namespace jira-sync-v040 \
		--set image.tag=$(VERSION) \
		--set apiServer.image.tag=$(VERSION) \
		--wait --timeout=300s
	@echo "✅ Operator deployment complete"

.PHONY: operator-undeploy
operator-undeploy:
	@echo "🧹 Undeploying JIRA Sync Operator..."
	helm uninstall jira-operator-demo --namespace jira-sync-v040 || true
	kubectl delete namespace jira-sync-v040 --timeout=60s || true

.PHONY: operator-status
operator-status:
	@echo "📊 Checking operator status..."
	kubectl get pods -n jira-sync-v040
	kubectl get jirasync -A 2>/dev/null || echo "No JIRASync resources found"

.PHONY: operator-logs
operator-logs:
	@echo "📋 Operator logs..."
	kubectl logs -n jira-sync-v040 -l app.kubernetes.io/name=jira-sync-operator --tail=50

# Complete demo workflow
.PHONY: demo-setup
demo-setup: kind-cluster-create images-load operator-deploy
	@echo "🎪 Demo environment setup complete!"
	@echo "📋 Next steps:"
	@echo "  1. Create JIRA credentials: kubectl create secret generic jira-credentials --from-literal=base-url=https://your-domain.atlassian.net --from-literal=email=your.email@domain.com --from-literal=token=your-token -n jira-sync-v040"
	@echo "  2. Create a JIRASync resource to test"

.PHONY: demo-teardown
demo-teardown: operator-undeploy kind-cluster-delete
	@echo "🧹 Demo environment teardown complete"

# Runtime targets
.PHONY: run
run: build
	@echo "🚀 Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME) --help

.PHONY: run-api
run-api: build-api
	@echo "🚀 Starting API server..."
	./$(BUILD_DIR)/$(API_BINARY_NAME) serve --help

.PHONY: run-operator
run-operator: build-operator
	@echo "🚀 Starting operator..."
	./$(BUILD_DIR)/$(OPERATOR_BINARY_NAME) --help

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
	$(GOTEST) -v -race -p 1 -count=1 -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
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
	@echo "Container & Images:"
	@echo "  container-build - Build container image (podman/docker)"
	@echo "  container-run   - Run container"
	@echo "  container-info  - Show container runtime info"
	@echo "  api-image-build - Build API server container image"
	@echo "  operator-image-build - Build operator container image"
	@echo "  images-build    - Build all component images"
	@echo "  images-load     - Build and load all images into kind cluster"
	@echo ""
	@echo "Kubernetes & Deployment:"
	@echo "  kind-cluster-create - Create kind cluster for development"
	@echo "  kind-cluster-delete - Delete kind cluster"
	@echo "  operator-deploy     - Deploy operator (builds images, deploys to kind)"
	@echo "  operator-undeploy   - Remove operator deployment"
	@echo "  operator-status     - Check operator and resource status"
	@echo "  operator-logs       - View operator logs"
	@echo "  demo-setup         - Complete demo environment setup"
	@echo "  demo-teardown      - Complete demo environment cleanup"
	@echo "  k8s-deploy         - Deploy to Kubernetes"
	@echo ""
	@echo "CI/CD:"
	@echo "  ci-pipeline  - Run full CI pipeline"
	@echo "  validate-all - Run all validation checks"
	@echo "  pre-commit   - Run pre-commit checks"
	@echo ""
	@echo "Cleanup:"
	@echo "  clean        - Clean all artifacts"