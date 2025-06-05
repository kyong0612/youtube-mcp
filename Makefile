# YouTube Transcript MCP Server Makefile

# Variables
APP_NAME = youtube-transcript-mcp
DOCKER_IMAGE = $(APP_NAME):latest
DOCKER_CONTAINER = $(APP_NAME)
GO_VERSION = 1.22

# Build variables
VERSION ?= 1.0.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo 'unknown')
LDFLAGS := -ldflags "-w -s -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Tools
TOOLS_DIR := $(shell pwd)/.tools
TOOLS_BIN := $(TOOLS_DIR)/bin
export PATH := $(TOOLS_BIN):$(PATH)

# Include tools makefile
include Makefile.tools

# Colors for output
CYAN := \033[0;36m
GREEN := \033[0;32m
RED := \033[0;31m
YELLOW := \033[0;33m
NC := \033[0m # No Color

# Default target
.PHONY: help
help: ## Show this help message
	@echo "$(CYAN)YouTube Transcript MCP Server$(NC)"
	@echo "$(GREEN)Available commands:$(NC)"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make $(CYAN)<target>$(NC)\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(CYAN)%-20s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Development

.PHONY: deps
deps: tools ## Install Go dependencies and tools
	@echo "$(GREEN)Installing dependencies...$(NC)"
	go mod download
	go mod tidy
	go mod verify

.PHONY: build
build: ## Build the application
	@echo "$(GREEN)Building $(APP_NAME)...$(NC)"
	CGO_ENABLED=0 go build $(LDFLAGS) -a -installsuffix cgo -o $(APP_NAME) ./cmd/server/main.go
	@echo "$(GREEN)Build complete!$(NC)"

.PHONY: build-all
build-all: ## Build for multiple platforms
	@echo "$(GREEN)Building for multiple platforms...$(NC)"
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(APP_NAME)-linux-amd64 ./cmd/server/main.go
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(APP_NAME)-linux-arm64 ./cmd/server/main.go
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(APP_NAME)-darwin-amd64 ./cmd/server/main.go
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(APP_NAME)-darwin-arm64 ./cmd/server/main.go
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(APP_NAME)-windows-amd64.exe ./cmd/server/main.go
	@echo "$(GREEN)Multi-platform build complete!$(NC)"

.PHONY: run
run: ## Run the application locally
	@echo "$(GREEN)Running $(APP_NAME)...$(NC)"
	go run $(LDFLAGS) ./cmd/server/main.go

.PHONY: dev
dev: install-air ## Run in development mode with hot reload
	@echo "$(GREEN)Running in development mode...$(NC)"
	$(TOOLS_BIN)/air

.PHONY: clean
clean: ## Clean build artifacts and caches
	@echo "$(YELLOW)Cleaning...$(NC)"
	go clean
	rm -f $(APP_NAME)
	rm -rf dist/
	rm -f coverage.out coverage.html
	rm -rf logs/ data/
	@echo "$(GREEN)Clean complete!$(NC)"

##@ Testing

.PHONY: test
test: ## Run unit tests
	@echo "$(GREEN)Running tests...$(NC)"
	go test -v -race ./...

.PHONY: test-short
test-short: ## Run short tests
	@echo "$(GREEN)Running short tests...$(NC)"
	go test -v -short ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(GREEN)Running integration tests...$(NC)"
	go test -v -tags=integration ./tests/integration/...

.PHONY: test-api
test-api: ## Test API endpoints (requires running server)
	@echo "$(GREEN)Testing API endpoints...$(NC)"
	@echo "Testing health endpoint..."
	@curl -sf http://localhost:8080/health | jq '.' || (echo "$(RED)Health check failed$(NC)" && exit 1)
	@echo "Testing ready endpoint..."
	@curl -sf http://localhost:8080/ready | jq '.' || (echo "$(RED)Ready check failed$(NC)" && exit 1)
	@echo "Testing version endpoint..."
	@curl -sf http://localhost:8080/version | jq '.' || (echo "$(RED)Version check failed$(NC)" && exit 1)
	@echo "$(GREEN)API tests passed!$(NC)"

.PHONY: benchmark
benchmark: ## Run benchmark tests
	@echo "$(GREEN)Running benchmarks...$(NC)"
	go test -bench=. -benchmem ./...

##@ Code Quality

.PHONY: lint
lint: install-golangci-lint ## Run linter
	@echo "$(GREEN)Running linter...$(NC)"
	$(TOOLS_BIN)/golangci-lint run

.PHONY: fmt
fmt: install-goimports ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	go fmt ./...
	$(TOOLS_BIN)/goimports -w .

.PHONY: vet
vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	go vet ./...

.PHONY: security
security: install-gosec ## Run security scan
	@echo "$(GREEN)Running security scan...$(NC)"
	$(TOOLS_BIN)/gosec -fmt=json -out=security-report.json ./... || true
	@echo "$(GREEN)Security report generated: security-report.json$(NC)"

.PHONY: mod-check
mod-check: ## Check for outdated dependencies
	@echo "$(GREEN)Checking for outdated dependencies...$(NC)"
	go list -u -m all

.PHONY: mod-graph
mod-graph: ## Show dependency graph
	@echo "$(GREEN)Dependency graph:$(NC)"
	go mod graph

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t $(DOCKER_IMAGE) --build-arg VERSION=$(VERSION) .
	@echo "$(GREEN)Docker build complete!$(NC)"

.PHONY: docker-run
docker-run: ## Run Docker container
	@echo "$(GREEN)Running Docker container...$(NC)"
	docker run --rm -it \
		-p 8080:8080 \
		-p 9090:9090 \
		--env-file .env \
		--name $(DOCKER_CONTAINER) \
		$(DOCKER_IMAGE)

.PHONY: docker-run-detached
docker-run-detached: ## Run Docker container in detached mode
	@echo "$(GREEN)Running Docker container in detached mode...$(NC)"
	docker run -d \
		-p 8080:8080 \
		-p 9090:9090 \
		--env-file .env \
		--name $(DOCKER_CONTAINER) \
		$(DOCKER_IMAGE)

.PHONY: docker-stop
docker-stop: ## Stop Docker container
	@echo "$(YELLOW)Stopping Docker container...$(NC)"
	docker stop $(DOCKER_CONTAINER) 2>/dev/null || true
	docker rm $(DOCKER_CONTAINER) 2>/dev/null || true

.PHONY: docker-logs
docker-logs: ## Show Docker container logs
	@echo "$(GREEN)Showing Docker logs...$(NC)"
	docker logs -f $(DOCKER_CONTAINER)

.PHONY: docker-shell
docker-shell: ## Get shell access to running container
	@echo "$(GREEN)Accessing container shell...$(NC)"
	docker exec -it $(DOCKER_CONTAINER) sh

.PHONY: docker-push
docker-push: ## Push Docker image to registry
	@echo "$(GREEN)Pushing Docker image...$(NC)"
	docker push $(DOCKER_IMAGE)

##@ Docker Compose

.PHONY: up
up: ## Start all services with docker-compose
	@echo "$(GREEN)Starting services...$(NC)"
	docker-compose up -d

.PHONY: up-build
up-build: ## Build and start services with docker-compose
	@echo "$(GREEN)Building and starting services...$(NC)"
	docker-compose up -d --build

.PHONY: down
down: ## Stop all services with docker-compose
	@echo "$(YELLOW)Stopping services...$(NC)"
	docker-compose down

.PHONY: down-volumes
down-volumes: ## Stop services and remove volumes
	@echo "$(YELLOW)Stopping services and removing volumes...$(NC)"
	docker-compose down -v

.PHONY: restart
restart: ## Restart all services
	@echo "$(GREEN)Restarting services...$(NC)"
	docker-compose restart

.PHONY: logs
logs: ## Show logs from all services
	@echo "$(GREEN)Showing logs...$(NC)"
	docker-compose logs -f

.PHONY: logs-app
logs-app: ## Show logs from app service only
	@echo "$(GREEN)Showing app logs...$(NC)"
	docker-compose logs -f youtube-transcript-mcp

.PHONY: ps
ps: ## Show running services
	@echo "$(GREEN)Running services:$(NC)"
	docker-compose ps

.PHONY: up-redis
up-redis: ## Start with Redis profile
	@echo "$(GREEN)Starting with Redis...$(NC)"
	docker-compose --profile with-redis up -d

.PHONY: up-monitoring
up-monitoring: ## Start with monitoring profile
	@echo "$(GREEN)Starting with monitoring...$(NC)"
	docker-compose --profile monitoring up -d

##@ Environment

.PHONY: env-setup
env-setup: ## Setup environment files
	@if [ ! -f .env ]; then \
		echo "$(GREEN)Creating .env file...$(NC)"; \
		cp .env.example .env; \
		echo "$(YELLOW)Please edit .env file with your configuration$(NC)"; \
	else \
		echo "$(YELLOW).env file already exists$(NC)"; \
	fi

.PHONY: env-check
env-check: ## Check environment configuration
	@echo "$(GREEN)Checking environment configuration...$(NC)"
	@if [ -f .env ]; then \
		echo "✓ .env file exists"; \
		echo "$(CYAN)Current configuration:$(NC)"; \
		grep -v '^#' .env | grep -v '^$$' | sort; \
	else \
		echo "$(RED)✗ .env file not found. Run 'make env-setup' first$(NC)"; \
	fi

.PHONY: env-validate
env-validate: ## Validate environment variables
	@echo "$(GREEN)Validating environment variables...$(NC)"
	@go run scripts/validate-env.go

##@ Monitoring

.PHONY: health
health: ## Check application health
	@echo "$(GREEN)Checking health...$(NC)"
	@curl -s http://localhost:8080/health | jq '.' || echo "$(RED)Service unavailable$(NC)"

.PHONY: ready
ready: ## Check application readiness
	@echo "$(GREEN)Checking readiness...$(NC)"
	@curl -s http://localhost:8080/ready | jq '.' || echo "$(RED)Service not ready$(NC)"

.PHONY: metrics
metrics: ## Show application metrics
	@echo "$(GREEN)Fetching metrics...$(NC)"
	@curl -s http://localhost:9090/metrics || echo "$(RED)Metrics unavailable$(NC)"

.PHONY: stats
stats: ## Show application statistics
	@echo "$(GREEN)Fetching statistics...$(NC)"
	@curl -s http://localhost:8080/api/v1/stats | jq '.' || echo "$(RED)Stats unavailable$(NC)"

.PHONY: status
status: ## Show overall application status
	@echo "$(CYAN)=== Application Status ===$(NC)"
	@make -s health
	@echo ""
	@make -s ready
	@echo ""
	@echo "$(CYAN)=== Docker Status ===$(NC)"
	@docker ps --filter name=$(DOCKER_CONTAINER) --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

##@ Utilities

.PHONY: version
version: ## Show version information
	@echo "$(CYAN)Version Information:$(NC)"
	@echo "App Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Go Version: $(shell go version)"

.PHONY: size
size: ## Show binary size
	@if [ -f $(APP_NAME) ]; then \
		echo "$(GREEN)Binary size:$(NC)"; \
		ls -lh $(APP_NAME); \
	else \
		echo "$(RED)Binary not found. Run 'make build' first$(NC)"; \
	fi

.PHONY: docs
docs: install-godoc ## Generate and serve documentation
	@echo "$(GREEN)Starting documentation server...$(NC)"
	$(TOOLS_BIN)/godoc -http=:6060 &
	@echo "$(GREEN)Documentation available at http://localhost:6060$(NC)"

.PHONY: docs-stop
docs-stop: ## Stop documentation server
	@echo "$(YELLOW)Stopping documentation server...$(NC)"
	@pkill -f "godoc -http=:6060" || true

.PHONY: generate
generate: ## Run go generate
	@echo "$(GREEN)Running go generate...$(NC)"
	go generate ./...

.PHONY: install
install: ## Install the binary to GOPATH/bin
	@echo "$(GREEN)Installing $(APP_NAME)...$(NC)"
	go install $(LDFLAGS) .

.PHONY: uninstall
uninstall: ## Uninstall the binary from GOPATH/bin
	@echo "$(YELLOW)Uninstalling $(APP_NAME)...$(NC)"
	rm -f $(GOPATH)/bin/$(APP_NAME)

##@ Release

.PHONY: release
release: clean test lint build-all ## Create a new release
	@echo "$(GREEN)Creating release $(VERSION)...$(NC)"
	@mkdir -p releases/$(VERSION)
	@cp -r dist/* releases/$(VERSION)/
	@cp README.md releases/$(VERSION)/
	@echo "$(GREEN)Release $(VERSION) created in releases/$(VERSION)$(NC)"

.PHONY: changelog
changelog: install-git-chglog ## Generate changelog
	@echo "$(GREEN)Generating changelog...$(NC)"
	$(TOOLS_BIN)/git-chglog -o CHANGELOG.md

##@ MCP Tools Testing

.PHONY: test-mcp-init
test-mcp-init: ## Test MCP initialize
	@echo "$(GREEN)Testing MCP initialize...$(NC)"
	@curl -X POST http://localhost:8080/mcp \
		-H "Content-Type: application/json" \
		-d '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | jq '.'

.PHONY: test-mcp-tools
test-mcp-tools: ## Test MCP list tools
	@echo "$(GREEN)Testing MCP list tools...$(NC)"
	@curl -X POST http://localhost:8080/mcp \
		-H "Content-Type: application/json" \
		-d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq '.'

.PHONY: test-transcript
test-transcript: ## Test get transcript tool
	@echo "$(GREEN)Testing get transcript...$(NC)"
	@curl -X POST http://localhost:8080/mcp \
		-H "Content-Type: application/json" \
		-d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_transcript","arguments":{"video_identifier":"dQw4w9WgXcQ","languages":["en"]}}}' | jq '.'

# Catch-all target
.DEFAULT_GOAL := help