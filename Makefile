.PHONY: validate test lint fmt vet tidy build help ready

## Primary target - run before completing any task
validate: fmt vet tidy lint test ## Run all validation checks
	@echo "✓ All validation checks passed"

## Build
build: ## Build the CLI
	go build -o bin/j4c ./cmd/j4c

## Testing
test: ## Run tests with race detector
	go test -race ./...

## Linting
lint: ## Run golangci-lint
	golangci-lint run ./...

## Formatting
fmt: ## Run gofmt
	@go fmt ./...

## Vet
vet: ## Run go vet
	go vet ./...

## Tidy
tidy: ## Ensure go.mod is tidy
	@go mod tidy

## Beads workflow
ready: ## Show tasks with no blockers
	@bd ready

list: ## List all beads tasks
	@bd list

## Help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
