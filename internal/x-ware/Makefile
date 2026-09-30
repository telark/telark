.PHONY: help build test lint fmt vet deps check

help: ## List targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-10s %s\n", $$1, $$2}'

build: ## Build the data Go module
	go build ./...

test: ## Run data module tests
	go test ./...

fmt: ## Format the data module
	gofmt -w .

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run

deps: ## Download and verify Go dependencies
	go mod download
	go mod verify

check: fmt vet lint test ## Format, vet, lint and test