# GoFr Makefile

# Variables
GOLANGCI_LINT_VERSION := v2.4.0

.PHONY: help install-lint lint test test-unit test-cover generate fmt tidy run-example

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

install-lint: ## Install golangci-lint v2.4.0 (matching CI)
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: ## Run golangci-lint on the project
	@if ! golangci-lint --version | grep -q "$(GOLANGCI_LINT_VERSION:v%=%)"; then \
		echo "Warning: golangci-lint $(GOLANGCI_LINT_VERSION) is required locally for consistency with CI."; \
		echo "Run 'make install-lint' to install the correct version."; \
	fi
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

test: ## Run all tests in the project
	@echo "Running all tests..."
	@go test -v ./...

test-unit: ## Run unit tests (skipping long-running ones)
	@echo "Running unit tests..."
	@go test -short -v ./...

test-cover: ## Run tests and generate coverage report
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out
	@echo "Coverage report generated at coverage.out"

generate: ## Run go generate to update mocks and other generated files
	@echo "Running go generate..."
	@go generate ./...

fmt: ## Format all go files using go fmt and goimports
	@echo "Formatting code..."
	@go fmt ./...
	@go install golang.org/x/tools/cmd/goimports@latest
	@goimports -w .

tidy: ## Clean up go.mod and go.sum
	@echo "Tidying up modules..."
	@go mod tidy

run-example: ## Run the basic http-server example
	@echo "Running http-server example..."
	@go run examples/http-server/main.go
