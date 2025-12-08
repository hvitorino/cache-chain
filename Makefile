.PHONY: help test test-verbose test-coverage test-unit test-integration bench clean fmt lint vet build install run-examples run-redis-example run-chain-example check deps

# Variables
GO = go
GOTEST = $(GO) test
GOVET = $(GO) vet
GOFMT = gofmt
GOLINT = golangci-lint

help: ## Show this help message
	@echo "Cache Chain Library - Makefile commands"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

test: ## Run all tests
	@echo "🧪 Running all tests..."
	$(GOTEST) -v ./...

test-verbose: ## Run tests with verbose output
	@echo "🧪 Running tests (verbose)..."
	$(GOTEST) -v -race -count=1 ./...

test-coverage: ## Run tests with coverage report
	@echo "📊 Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

test-unit: ## Run unit tests only
	@echo "🧪 Running unit tests..."
	$(GOTEST) -v -short ./...

test-integration: ## Run integration tests only
	@echo "🧪 Running integration tests..."
	$(GOTEST) -v -run Integration ./...

test-memory: ## Run memory cache tests
	@echo "🧪 Testing memory cache..."
	$(GOTEST) -v ./pkg/cache/memory/...

test-redis: ## Run Redis cache tests
	@echo "🧪 Testing Redis cache..."
	$(GOTEST) -v ./pkg/cache/redis/...

test-chain: ## Run chain tests
	@echo "🧪 Testing cache chain..."
	$(GOTEST) -v ./pkg/chain/...

bench: ## Run benchmarks
	@echo "⚡ Running benchmarks..."
	$(GOTEST) -bench=. -benchmem -run=^$$ ./...

bench-memory: ## Run memory cache benchmarks
	@echo "⚡ Benchmarking memory cache..."
	$(GOTEST) -bench=. -benchmem -run=^$$ ./pkg/cache/memory/...

bench-redis: ## Run Redis cache benchmarks
	@echo "⚡ Benchmarking Redis cache..."
	$(GOTEST) -bench=. -benchmem -run=^$$ ./pkg/cache/redis/...

bench-chain: ## Run chain benchmarks
	@echo "⚡ Benchmarking cache chain..."
	$(GOTEST) -bench=. -benchmem -run=^$$ ./pkg/chain/...

clean: ## Clean build artifacts and cache
	@echo "🧹 Cleaning..."
	@rm -f coverage.out coverage.html
	@$(GO) clean -cache -testcache -modcache
	@find . -name "*.test" -delete
	@find . -name "*.out" -delete
	@echo "✅ Clean complete!"

fmt: ## Format Go code
	@echo "✨ Formatting code..."
	@$(GOFMT) -s -w .
	@echo "✅ Code formatted!"

fmt-check: ## Check if code is formatted
	@echo "🔍 Checking code format..."
	@output=$$($(GOFMT) -l .); \
	if [ -n "$$output" ]; then \
		echo "❌ The following files are not formatted:"; \
		echo "$$output"; \
		exit 1; \
	else \
		echo "✅ All files are properly formatted!"; \
	fi

lint: ## Run linter (requires golangci-lint)
	@echo "🔍 Running linter..."
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run ./...; \
	else \
		echo "⚠️  golangci-lint not installed. Install with:"; \
		echo "   brew install golangci-lint"; \
		echo "   or visit: https://golangci-lint.run/usage/install/"; \
	fi

vet: ## Run go vet
	@echo "🔍 Running go vet..."
	$(GOVET) ./...
	@echo "✅ Vet complete!"

check: fmt-check vet ## Run all checks (format + vet)
	@echo "✅ All checks passed!"

build: ## Build the library
	@echo "🔨 Building..."
	$(GO) build -v ./...
	@echo "✅ Build complete!"

install: ## Install dependencies
	@echo "📦 Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "✅ Dependencies installed!"

deps: ## Show dependency tree
	@echo "📦 Dependency tree:"
	$(GO) mod graph

deps-update: ## Update dependencies
	@echo "⬆️  Updating dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "✅ Dependencies updated!"

run-examples: ## Run all examples
	@echo "🚀 Running examples..."
	@$(MAKE) run-memory-example
	@$(MAKE) run-redis-example
	@$(MAKE) run-chain-example

run-memory-example: ## Run memory cache example
	@echo "📝 Running memory cache example..."
	$(GO) run examples/memory/main.go

run-redis-example: ## Run Redis cache example (requires Redis)
	@echo "📝 Running Redis cache example..."
	@echo "⚠️  Make sure Redis is running on localhost:6379"
	$(GO) run examples/redis/main.go

run-chain-example: ## Run cache chain example (requires Redis)
	@echo "📝 Running chain integration example..."
	@echo "⚠️  Make sure Redis is running on localhost:6379"
	$(GO) run examples/chain_integration.go

verify: check test ## Verify code (format, vet, test)
	@echo "✅ Verification complete!"

ci: fmt-check vet test-coverage ## CI pipeline (format check, vet, coverage)
	@echo "✅ CI pipeline complete!"

quick: fmt vet test-unit ## Quick check (format, vet, unit tests)
	@echo "✅ Quick check complete!"

docker-redis: ## Start Redis in Docker
	@echo "🔴 Starting Redis..."
	@docker run -d --name cache-chain-redis -p 6379:6379 redis:7-alpine
	@echo "✅ Redis started at localhost:6379"

docker-redis-stop: ## Stop Redis Docker container
	@echo "🛑 Stopping Redis..."
	@docker stop cache-chain-redis
	@docker rm cache-chain-redis
	@echo "✅ Redis stopped!"

docker-redis-cluster: ## Start Redis Cluster in Docker
	@echo "🔴 Starting Redis Cluster..."
	@cd examples/redis-cluster && docker-compose up -d
	@echo "✅ Redis Cluster started!"

docker-redis-cluster-stop: ## Stop Redis Cluster
	@echo "🛑 Stopping Redis Cluster..."
	@cd examples/redis-cluster && docker-compose down
	@echo "✅ Redis Cluster stopped!"

show-coverage: ## Show coverage report in browser
	@if [ -f coverage.html ]; then \
		open coverage.html || xdg-open coverage.html; \
	else \
		echo "❌ No coverage report found. Run 'make test-coverage' first."; \
	fi

mod-init: ## Initialize go module
	@echo "📦 Initializing Go module..."
	$(GO) mod init cache-chain || true
	$(GO) mod tidy
	@echo "✅ Module initialized!"

mod-vendor: ## Vendor dependencies
	@echo "�� Vendoring dependencies..."
	$(GO) mod vendor
	@echo "✅ Dependencies vendored!"

release-check: ## Check if ready for release
	@echo "🔍 Checking release readiness..."
	@$(MAKE) ci
	@echo ""
	@echo "✅ Release checks passed!"
	@echo "📦 Ready to release!"

info: ## Show project information
	@echo "�� Cache Chain Library Information"
	@echo "===================================="
	@echo "Go version:       $$(go version)"
	@echo "Module:           $$(go list -m)"
	@echo "Packages:         $$(go list ./... | wc -l | tr -d ' ')"
	@echo "Dependencies:     $$(go list -m all | wc -l | tr -d ' ')"
	@echo ""
	@echo "📁 Project structure:"
	@echo "  pkg/cache/        - Cache interface and implementations"
	@echo "  pkg/chain/        - Cache chain orchestration"
	@echo "  pkg/metrics/      - Metrics collection"
	@echo "  pkg/resilience/   - Circuit breaker and timeout"
	@echo "  examples/         - Usage examples"

lines: ## Count lines of code
	@echo "📊 Lines of Code:"
	@echo "Total Go files:   $$(find . -name "*.go" -not -path "./vendor/*" | wc -l | tr -d ' ')"
	@echo "Lines of code:    $$(find . -name "*.go" -not -path "./vendor/*" -exec wc -l {} + | tail -1 | awk '{print $$1}')"
	@echo "Test files:       $$(find . -name "*_test.go" -not -path "./vendor/*" | wc -l | tr -d ' ')"

watch: ## Watch for changes and run tests (requires fswatch)
	@if command -v fswatch >/dev/null 2>&1; then \
		echo "👀 Watching for changes..."; \
		fswatch -o . -e ".*" -i "\\.go$$" | xargs -n1 -I{} make test-unit; \
	else \
		echo "❌ fswatch not installed. Install with:"; \
		echo "   brew install fswatch"; \
	fi

todo: ## Show TODO comments in code
	@echo "📝 TODO items:"
	@grep -rn "TODO" --include="*.go" . || echo "No TODOs found!"

fixme: ## Show FIXME comments in code
	@echo "🔧 FIXME items:"
	@grep -rn "FIXME" --include="*.go" . || echo "No FIXMEs found!"
