# Cache-Chain

A flexible, production-ready Go library for building multi-layer cache systems with automatic fallback, warming, and resilience patterns.

## Overview

Cache-Chain enables you to build sophisticated caching architectures with **N layers** of cache, where each layer automatically falls back to the next on cache miss, and warms upper layers on hit. Think of it as a chain of responsibility pattern for caching.

```go
// Example: 3-layer cache system
L1 (in-memory) → L2 (Redis) → L3 (Database)

// On GetData(key):
// - L1 miss → query L2
// - L2 hit → return value + warm L1 asynchronously
// - L2 miss → query L3
// - L3 hit → return value + warm L2 + warm L1
```

## Key Features

### 🔗 N-Layer Chain Architecture
- Support for unlimited cache layers
- Automatic fallback traversal
- Intelligent warm-up of upper layers on hit

### ⚡ Performance
- **Async Writers**: Write operations are non-blocking with configurable buffered queues
- **Sync Readers**: Read operations are synchronous to the client but internally optimized
- **Single-flight Pattern**: Prevents thundering herd on cache misses

### 🛡️ Resilience
- **Circuit Breaker**: Per-layer circuit breakers prevent cascade failures
- **Timeouts**: Configurable timeouts for each layer operation
- **Graceful Degradation**: Skip failed layers automatically

### 📊 Observability
- **Comprehensive Metrics**: Hits, misses, errors, latencies per layer
- **Circuit Breaker Status**: Monitor circuit state transitions
- **Queue Metrics**: Writer queue depth and processing rates
- **Pluggable Exporters**: Support for Prometheus, StatsD, or custom backends

### ✅ Data Integrity
- **Validators**: Optional validation for data entering/exiting cache
- **Versioning**: Built-in support for value versioning to prevent race conditions
- **TTL Management**: Per-layer TTL configuration with hierarchical support

### 🎯 Flexibility
- **Pluggable Implementations**: Bring your own cache backend (in-memory, Redis, Memcached, etc.)
- **Configurable Policies**: Eviction, warm-up, degradation modes
- **Context-Aware**: Full context.Context support for cancellation and deadlines

## Architecture

### Core Components

1. **CacheLayer Interface**: Defines the contract for any cache implementation
2. **Chain**: Manages the sequence of cache layers and orchestrates fallback/warm-up
3. **AsyncWriter**: Handles non-blocking writes with bounded queues
4. **CircuitBreaker**: Per-layer resilience mechanism
5. **MetricsCollector**: Captures and exports observability data
6. **Validator**: Validates data integrity at layer boundaries

### Data Flow

```
┌─────────────────────────────────────────────────────────┐
│                     Client Request                       │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
            ┌────────────────┐
            │  Validator     │ (optional)
            └────────┬───────┘
                     │
                     ▼
            ┌────────────────┐
            │   L1 Cache     │──── Circuit Breaker ──┐
            │   (fastest)    │──── Timeout          │
            └────────┬───────┘──── Metrics          │
                     │                               │
                 miss │ hit                          │
                     │  │                            │
                     │  └──────────────────────┐    │
                     ▼                         │    │
            ┌────────────────┐                 │    │
            │   L2 Cache     │                 │    │
            └────────┬───────┘                 │    │
                     │                         │    │
                 miss │ hit                    │    │
                     │  │                      │    │
                     │  └─────> Warm L1 ◄─────┘    │
                     ▼          (async)             │
            ┌────────────────┐                      │
            │   L3 Cache     │                      │
            └────────┬───────┘                      │
                     │                              │
                 hit │                              │
                     │                              │
                     └─────> Warm L2 & L1 ◄────────┘
                             (async)
```

## Use Cases

- **High-Traffic APIs**: Reduce database load with intelligent multi-layer caching
- **Microservices**: Consistent caching strategy across service boundaries
- **Cost Optimization**: Minimize expensive external cache service calls
- **Resilient Systems**: Gracefully handle cache layer failures
- **Global Distribution**: Combine local (in-memory) + regional (Redis) + global (database) caches

## Design Principles

1. **Simplicity**: Easy to understand and use APIs
2. **Idiomatic Go**: Follows Go best practices and conventions
3. **Production-Ready**: Battle-tested resilience patterns
4. **Zero Dependencies** (core): Standard library only for core functionality
5. **Extensible**: Plugin architecture for custom implementations
6. **Observable**: Metrics and logging built-in, not bolted-on

## Quick Start

### Running Examples

The library includes several examples demonstrating different features and use cases:

#### 1. Banking Transactions API (Production-Ready Example)

Complete 3-layer cache API with Memory (L1), Redis (L2), and PostgreSQL (L3 read-only).

```bash
cd examples/banking-api

# Initialize and start the complete stack (PostgreSQL + Redis + API)
make init

# View available commands
make help

# Health check
make health

# Seed sample transactions
make seed

# Run API tests
make test-api

# View logs
make logs

# Stop all services
make down
```

**Features:**
- REST API for banking transactions
- 3-layer cache architecture (Memory → Redis → PostgreSQL)
- Docker Compose orchestration
- Comprehensive Makefile with 27+ commands
- Health checks and graceful shutdown
- PostgreSQL as read-only cache layer (L3)

See [examples/banking-api/README.md](examples/banking-api/README.md) for complete documentation.

#### 2. Basic Chain Integration

Simple example showing how to create and use a cache chain:

```bash
# Run from repository root
go run examples/chain_integration.go
```

Demonstrates:
- Creating memory cache layers
- Building a cache chain
- Basic Get/Set operations
- Cache warming behavior

#### 3. HTTP API Server

REST API exposing cache operations over HTTP:

```bash
# Run from repository root
go run examples/api/main.go

# API runs on http://localhost:8080
# Endpoints: GET/POST /cache/:key, GET /metrics
```

Demonstrates:
- HTTP API for cache operations
- In-memory metrics collection
- Graceful shutdown

#### 4. Redis Integration

Shows how to use Redis as a cache layer with pipelining:

```bash
# Start Redis
docker run -d -p 6379:6379 redis:7-alpine

# Run example
go run examples/redis/main.go
```

Demonstrates:
- Redis cache layer configuration
- Pipelining for batch operations
- Connection pooling
- Multi-layer chain with Redis

#### 5. Redis Cluster Mode

Configuration examples for Redis Cluster and Sentinel:

```bash
go run examples/redis-cluster/main.go
```

Demonstrates:
- Redis Cluster configuration
- Sentinel (HA) configuration
- Production deployment options

See [examples/redis-cluster/README.md](examples/redis-cluster/README.md) for cluster setup.

#### 6. Resilience Patterns

Circuit breaker, timeouts, and graceful degradation:

```bash
go run examples/resilience/main.go
```

Demonstrates:
- Automatic circuit breaker protection
- Timeout handling per layer
- Fallback behavior
- Error recovery

### Using the Library Makefile

The repository includes a comprehensive Makefile for development:

```bash
# Run all tests
make test

# Run benchmarks
make bench

# Run specific component tests
make test-memory
make test-redis
make test-chain

# Code quality
make fmt        # Format code
make lint       # Run linter
make vet        # Run go vet
make check      # Run all quality checks

# Run examples
make run-redis-example
make run-resilience-example

# Docker Redis management
make docker-redis        # Start Redis
make docker-redis-stop   # Stop Redis

# CI pipeline
make ci                  # Run full CI (format, vet, coverage)
make verify              # Quick verification
make quick               # Fast iteration (fmt + test)

# View all commands
make help
```

## Project Status

🚧 **Under Active Development** - Following phased implementation approach

See `rules/` directory for detailed development phases and prompts.

## License

MIT License - See LICENSE file for details

## Contributing

Contributions are welcome! Please read CONTRIBUTING.md for guidelines.

---

**Note**: This library is designed to be the foundation for production caching systems. Each component is carefully designed to handle real-world failure scenarios and scale requirements.
