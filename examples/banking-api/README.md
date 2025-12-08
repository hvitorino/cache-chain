# Banking API - 3-Layer Cache Example

This example demonstrates a complete production-ready REST API using a 3-layer cache architecture for banking transactions.

## Architecture

```
┌─────────────────────────────────────────────────┐
│              Banking REST API                   │
├─────────────────────────────────────────────────┤
│                                                 │
│  ┌──────────────────────────────────────────┐  │
│  │  L1: Memory Cache (In-Memory)            │  │
│  │  - Fastest layer (~1µs)                  │  │
│  │  - 1000 max entries with LRU eviction    │  │
│  │  - READ + WRITE                          │  │
│  └──────────────────────────────────────────┘  │
│                     ↓ miss                      │
│  ┌──────────────────────────────────────────┐  │
│  │  L2: Redis Cache (Network)               │  │
│  │  - Fast layer (~1ms)                     │  │
│  │  - Distributed, shared between instances │  │
│  │  - READ + WRITE                          │  │
│  └──────────────────────────────────────────┘  │
│                     ↓ miss                      │
│  ┌──────────────────────────────────────────┐  │
│  │  L3: PostgreSQL (Database)               │  │
│  │  - Persistent layer (~10ms)              │  │
│  │  - Source of truth for all data          │  │
│  │  - READ ONLY (for cache layer)           │  │
│  └──────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘

Write Flow:
  Create Transaction → PostgreSQL.CreateTransaction()
                    → Cache in L1 + L2

Read Flow:
  Request → L1 → L2 → L3.Get() (reads from DB)
                   ↓ warm up L1 + L2
```

## Cache Flow

1. **Read Request**:
   - Check L1 (Memory) → if hit, return immediately
   - If miss, check L2 (Redis) → if hit, warm up L1 and return
   - If miss, check L3 (PostgreSQL) → read transaction data, warm up L2 and L1, then return

2. **Write Request**:
   - Write to PostgreSQL database (transactions table)
   - Cache in L1 (Memory) and L2 (Redis)
   - **L3 does not cache writes** - it's read-only for the cache layer

### Why L3 is Read-Only?

The PostgreSQL L3 layer acts as the **source of truth** for transaction data:

- **Read Path**: When L1 and L2 miss, L3 reads directly from the `transactions` table
- **Write Path**: New transactions are written via `CreateTransaction()` directly to the database
- **No Cache Table**: L3 doesn't maintain a separate `cache_entries` table
- **Automatic Warmup**: After reading from L3, data is automatically cached in L1 and L2

This design ensures:
- ✅ PostgreSQL remains the single source of truth
- ✅ No cache inconsistency between cache table and transactions table
- ✅ Simpler database schema (no cache_entries table needed)
- ✅ Direct access to transaction data when needed

## Features

- ✅ **3-Layer caching** with automatic fallback and warmup
- ✅ **PostgreSQL** as persistent database and read-only L3 cache layer
- ✅ **Redis** for distributed caching (L2)
- ✅ **In-Memory** cache for ultra-fast access (L1)
- ✅ **L3 Read-Only**: PostgreSQL reads transaction data directly from database
- ✅ **RESTful API** with JSON responses
- ✅ **Docker Compose** setup for easy deployment
- ✅ **Health checks** for all services
- ✅ **Cache hit/miss** logging with performance metrics

## Quick Start

### Using Makefile (Recommended)

```bash
cd examples/banking-api

# Show all available commands
make help

# Initialize everything (build, start services, seed data)
make init

# Or quick start (just up + seed)
make quick-start

# Check API health
make health

# Run test requests
make test-api

# View logs
make logs-api

# Stop services
make down
```

### Using Docker Compose Directly

```bash
# Start all services (PostgreSQL, Redis, API)
cd examples/banking-api
docker-compose up -d

# Check logs
docker-compose logs -f api

# Stop all services
docker-compose down
```

### Manual Setup

```bash
# 1. Start PostgreSQL
docker run -d --name postgres \
  -e POSTGRES_DB=banking_db \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  postgres:15-alpine

# 2. Start Redis
docker run -d --name redis \
  -p 6379:6379 \
  redis:7-alpine

# 3. Run the API
cd examples/banking-api
go run main.go
```

## API Endpoints

### Health Check
```bash
curl http://localhost:8080/health
```

### Create Transaction
```bash
curl -X POST http://localhost:8080/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "ACC001",
    "type": "debit",
    "amount": 150.50,
    "currency": "USD",
    "description": "Coffee shop payment"
  }'
```

Response:
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "account_id": "ACC001",
  "type": "debit",
  "amount": 150.50,
  "currency": "USD",
  "description": "Coffee shop payment",
  "status": "completed",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

### Get Transaction by ID
```bash
curl http://localhost:8080/transactions/123e4567-e89b-12d3-a456-426614174000
```

Response includes cache performance headers:
```
X-Cache-Hit: true
X-Response-Time: 123µs
```

## Testing Cache Performance

```bash
# Create a test transaction
TRANSACTION_ID=$(curl -s -X POST http://localhost:8080/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "ACC001",
    "type": "credit",
    "amount": 1000.00,
    "currency": "USD",
    "description": "Salary deposit"
  }' | jq -r '.id')

# First request - cache miss (loads from PostgreSQL)
curl -i http://localhost:8080/transactions/$TRANSACTION_ID
# Expected: X-Cache-Hit: false, slower response time

# Second request - cache hit from Memory (L1)
curl -i http://localhost:8080/transactions/$TRANSACTION_ID
# Expected: X-Cache-Hit: true, much faster response time
```

## Cache Behavior Examples

### Scenario 1: Cold Start
```
Request → L1 miss → L2 miss → L3 hit (PostgreSQL)
Result: ~10-50ms response time
Cache warmup: Data stored in L1 and L2
```

### Scenario 2: Warm L2 Cache
```
Request → L1 miss → L2 hit (Redis)
Result: ~1-5ms response time
Cache warmup: Data stored in L1
```

### Scenario 3: Hot L1 Cache
```
Request → L1 hit (Memory)
Result: ~100µs-1ms response time
No warmup needed
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | API server port |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_USER` | `postgres` | PostgreSQL username |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password |
| `POSTGRES_DB` | `banking_db` | PostgreSQL database name |
| `REDIS_ADDR` | `localhost:6379` | Redis address |

## Database Schema

### transactions table
```sql
CREATE TABLE transactions (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    type TEXT NOT NULL,
    amount NUMERIC(15,2) NOT NULL,
    currency TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);
```

**Note**: L3 (PostgreSQL) acts as a read-only cache layer. It reads transaction data directly from the `transactions` table without maintaining a separate cache_entries table. New transactions are written directly to the `transactions` table and then cached in L1 (Memory) and L2 (Redis).

## Performance Expectations

| Cache Layer | Typical Latency | Use Case |
|-------------|----------------|----------|
| L1 (Memory) | ~100µs | Ultra-fast access for hot data |
| L2 (Redis) | ~1-5ms | Shared cache across instances |
| L3 (PostgreSQL) | ~10-50ms | Source of truth, cold data |

## Monitoring

The API logs cache hits/misses with performance metrics:

```
✓ Cache HIT for transaction:123... (took 156µs)
✗ Cache MISS for transaction:456...
✓ Loaded from database (took 23ms)
```

## Production Considerations

1. **TTL Configuration**: Adjust cache TTLs based on your data volatility
2. **Memory Limits**: Configure L1 cache size based on available RAM
3. **Redis Persistence**: Enable RDB or AOF for Redis data durability
4. **Connection Pooling**: Tune PostgreSQL connection pool settings
5. **Metrics**: Add Prometheus metrics for cache hit rates
6. **Distributed Caching**: Use Redis Cluster for horizontal scaling

## Makefile Commands

```bash
# Basic commands
make help              # Show all available commands
make init              # Initialize everything (build, start, seed)
make quick-start       # Quick start (up + seed)
make up                # Start all services
make down              # Stop all services
make restart           # Restart all services
make rebuild           # Rebuild everything from scratch

# Development
make dev               # Start only PostgreSQL and Redis
make run-local         # Run API locally
make install           # Install Go dependencies

# Database
make seed              # Seed database with sample data
make shell-postgres    # Open PostgreSQL shell
make shell-redis       # Open Redis CLI
make stats             # Show cache statistics

# Testing & Monitoring
make health            # Check API health
make test-api          # Run API test script
make benchmark         # Run simple performance benchmark
make logs              # Show all logs
make logs-api          # Show API logs only
make ps                # Show running services

# Maintenance
make clean             # Clean up Docker resources
make down-volumes      # Stop and remove volumes
```

## Clean Up

```bash
# Using Makefile
make down-volumes      # Stop and remove all data
make clean             # Full cleanup

# Or using Docker Compose directly
docker-compose down -v
docker volume prune
```

## Next Steps

- Add authentication and authorization
- Implement rate limiting
- Add request/response logging middleware
- Set up Prometheus metrics
- Configure circuit breakers for external dependencies
- Add integration tests
