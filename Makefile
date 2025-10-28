.PHONY: build test lint clean install run help

# Variables
BINARY_NAME=terranovate
MAIN_PATH=.
BUILD_DIR=bin
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"
GOPATH?=$(shell go env GOPATH)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get
GOCLEAN=$(GOCMD) clean

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

install: ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

test-coverage: test ## Run tests with coverage report
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run linter
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin" && exit 1)
	golangci-lint run --timeout 5m
	@echo "Linting complete"

fmt: ## Format code
	@echo "Formatting code..."
	$(GOCMD) fmt ./...
	@echo "Formatting complete"

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOCMD) vet ./...
	@echo "Vetting complete"

tidy: ## Tidy go modules
	@echo "Tidying go modules..."
	$(GOMOD) tidy
	@echo "Tidy complete"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

run: build ## Build and run the application
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

run-scan: build ## Run scan command
	./$(BUILD_DIR)/$(BINARY_NAME) scan --path .

run-check: build ## Run check command
	./$(BUILD_DIR)/$(BINARY_NAME) check --path .

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .
	docker tag $(BINARY_NAME):$(VERSION) $(BINARY_NAME):latest
	@echo "Docker build complete"

docker-run: docker-build ## Run Docker container
	docker run --rm -it $(BINARY_NAME):latest

ci: lint test ## Run CI checks (lint + test)
	@echo "CI checks complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	@echo "Dependencies downloaded"

verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	$(GOMOD) verify
	@echo "Dependencies verified"

all: clean deps build test lint ## Run all checks and build
	@echo "All tasks complete"
