.PHONY: help build up down restart logs clean seed test-api health run-local

# Variables
DOCKER_COMPOSE = docker-compose
GO = go

help: ## Show this help message
	@echo "Banking API - Makefile commands"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build Docker images
	@echo "🔨 Building Docker images..."
	$(DOCKER_COMPOSE) build

up: ## Start all services (PostgreSQL, Redis, API)
	@echo "🚀 Starting all services..."
	$(DOCKER_COMPOSE) up -d
	@echo "✅ Services started!"
	@echo "📝 API running at http://localhost:8080"
	@echo "💾 PostgreSQL running at localhost:5432"
	@echo "🔴 Redis running at localhost:6379"

down: ## Stop all services
	@echo "🛑 Stopping all services..."
	$(DOCKER_COMPOSE) down
	@echo "✅ Services stopped!"

down-volumes: ## Stop all services and remove volumes
	@echo "🛑 Stopping all services and removing volumes..."
	$(DOCKER_COMPOSE) down -v
	@echo "✅ Services stopped and volumes removed!"

restart: down up ## Restart all services

logs: ## Show logs from all services
	$(DOCKER_COMPOSE) logs -f

logs-api: ## Show logs from API only
	$(DOCKER_COMPOSE) logs -f api

logs-postgres: ## Show logs from PostgreSQL only
	$(DOCKER_COMPOSE) logs -f postgres

logs-redis: ## Show logs from Redis only
	$(DOCKER_COMPOSE) logs -f redis

ps: ## Show running services
	$(DOCKER_COMPOSE) ps

clean: ## Clean up Docker resources
	@echo "🧹 Cleaning up..."
	$(DOCKER_COMPOSE) down -v --remove-orphans
	@docker system prune -f
	@echo "✅ Cleanup complete!"

seed: ## Seed database with sample transactions
	@echo "🌱 Seeding database with sample transactions..."
	@sleep 2
	@curl -s -X POST http://localhost:8080/transactions \
		-H "Content-Type: application/json" \
		-d '{"account_id":"ACC001","type":"credit","amount":5000.00,"currency":"USD","description":"Initial deposit"}' | jq .
	@curl -s -X POST http://localhost:8080/transactions \
		-H "Content-Type: application/json" \
		-d '{"account_id":"ACC001","type":"debit","amount":150.50,"currency":"USD","description":"Coffee shop"}' | jq .
	@curl -s -X POST http://localhost:8080/transactions \
		-H "Content-Type: application/json" \
		-d '{"account_id":"ACC001","type":"debit","amount":89.99,"currency":"USD","description":"Restaurant"}' | jq .
	@curl -s -X POST http://localhost:8080/transactions \
		-H "Content-Type: application/json" \
		-d '{"account_id":"ACC001","type":"credit","amount":2500.00,"currency":"USD","description":"Salary"}' | jq .
	@curl -s -X POST http://localhost:8080/transactions \
		-H "Content-Type: application/json" \
		-d '{"account_id":"ACC002","type":"credit","amount":10000.00,"currency":"USD","description":"Business income"}' | jq .
	@curl -s -X POST http://localhost:8080/transactions \
		-H "Content-Type: application/json" \
		-d '{"account_id":"ACC002","type":"debit","amount":1200.00,"currency":"USD","description":"Office rent"}' | jq .
	@echo "✅ Database seeded successfully!"

health: ## Check API health
	@echo "🏥 Checking API health..."
	@curl -s http://localhost:8080/health | jq .

test-api: ## Run API test script
	@echo "🧪 Running API tests..."
	@chmod +x requests.sh
	@./requests.sh

run-local: ## Run API locally (requires PostgreSQL and Redis running)
	@echo "🚀 Running API locally..."
	@echo "⚠️  Make sure PostgreSQL and Redis are running!"
	$(GO) run main.go

dev: ## Start services in development mode (without API)
	@echo "🔧 Starting PostgreSQL and Redis for local development..."
	$(DOCKER_COMPOSE) up -d postgres redis
	@sleep 3
	@echo "✅ Development services ready!"
	@echo "💾 PostgreSQL: localhost:5432"
	@echo "🔴 Redis: localhost:6379"
	@echo ""
	@echo "Run 'make run-local' to start the API locally"

install: ## Install Go dependencies
	@echo "📦 Installing Go dependencies..."
	cd ../.. && $(GO) mod download
	@echo "✅ Dependencies installed!"

init: build up seed ## Initialize everything (build, start, seed)
	@echo ""
	@echo "🎉 Banking API is ready!"
	@echo "📝 API: http://localhost:8080"
	@echo "📚 Try 'make test-api' to run example requests"
	@echo "📋 Use 'make logs-api' to see API logs"

shell-postgres: ## Open PostgreSQL shell
	@echo "🐘 Opening PostgreSQL shell..."
	@docker exec -it banking-postgres psql -U postgres -d banking_db

shell-redis: ## Open Redis CLI
	@echo "🔴 Opening Redis CLI..."
	@docker exec -it banking-redis redis-cli

stats: ## Show cache statistics
	@echo "📊 Cache Statistics"
	@echo "===================="
	@echo ""
	@echo "Redis Keys:"
	@docker exec -it banking-redis redis-cli DBSIZE
	@echo ""
	@echo "PostgreSQL Transactions:"
	@docker exec -it banking-postgres psql -U postgres -d banking_db -c "SELECT COUNT(*) as total_transactions FROM transactions;"
	@echo ""
	@echo "Transactions by Account:"
	@docker exec -it banking-postgres psql -U postgres -d banking_db -c "SELECT account_id, COUNT(*) as count FROM transactions GROUP BY account_id;"

benchmark: ## Run simple benchmark
	@echo "⚡ Running benchmark..."
	@echo "Creating test transaction..."
	@TX_ID=$$(curl -s -X POST http://localhost:8080/transactions \
		-H "Content-Type: application/json" \
		-d '{"account_id":"ACC999","type":"credit","amount":100.00,"currency":"USD","description":"Benchmark test"}' | jq -r '.id'); \
	echo "Transaction ID: $$TX_ID"; \
	echo ""; \
	echo "First request (cache miss):"; \
	time curl -s http://localhost:8080/transactions/$$TX_ID > /dev/null; \
	echo ""; \
	echo "Second request (cache hit from Redis):"; \
	time curl -s http://localhost:8080/transactions/$$TX_ID > /dev/null; \
	echo ""; \
	echo "Third request (cache hit from Memory):"; \
	time curl -s http://localhost:8080/transactions/$$TX_ID > /dev/null

examples: ## Show example API calls
	@echo "📚 Example API Calls"
	@echo "===================="
	@echo ""
	@echo "1. Create Transaction:"
	@echo "   curl -X POST http://localhost:8080/transactions \\"
	@echo "     -H 'Content-Type: application/json' \\"
	@echo "     -d '{\"account_id\":\"ACC001\",\"type\":\"debit\",\"amount\":50.00,\"currency\":\"USD\",\"description\":\"Test\"}'"
	@echo ""
	@echo "2. Get Transaction:"
	@echo "   curl http://localhost:8080/transactions/{id}"
	@echo ""
	@echo "3. List Account Transactions:"
	@echo "   curl http://localhost:8080/transactions?account_id=ACC001"
	@echo ""
	@echo "4. Health Check:"
	@echo "   curl http://localhost:8080/health"

watch-logs: ## Watch all logs with color
	$(DOCKER_COMPOSE) logs -f --tail=100

rebuild: down-volumes build up ## Rebuild everything from scratch

quick-start: up seed ## Quick start (up + seed)
	@echo "⚡ Quick start complete!"
