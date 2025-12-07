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

## Project Status

🚧 **Under Active Development** - Following phased implementation approach

See `rules/` directory for detailed development phases and prompts.

## License

MIT License - See LICENSE file for details

## Contributing

Contributions are welcome! Please read CONTRIBUTING.md for guidelines.

---

**Note**: This library is designed to be the foundation for production caching systems. Each component is carefully designed to handle real-world failure scenarios and scale requirements.
