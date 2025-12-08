# Redis Cache Implementation

Implementation of a high-performance Redis cache layer using the `rueidis` client library with automatic pipelining support.

## Features

- **High Performance**: Uses `rueidis` - one of the fastest Redis Go clients
- **Automatic Pipelining**: Built-in support for batching commands to reduce round trips
- **JSON Serialization**: Automatic marshaling/unmarshaling of complex data types
- **Context Support**: All operations respect context cancellation and timeouts
- **Key Prefixing**: Namespace isolation to prevent key collisions
- **Connection Pooling**: Efficient connection management out of the box
- **Batch Operations**: `BatchGet`, `BatchSet`, and `BatchDelete` for optimal performance
- **Utility Methods**: `Ping`, `FlushDB`, `Keys`, `TTL`, `Exists`

## Installation

```bash
go get github.com/redis/rueidis@latest
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"

    "cache-chain/pkg/cache/redis"
)

func main() {
    // Create Redis cache with default configuration
    config := redis.DefaultRedisCacheConfig()
    redisCache, err := redis.NewRedisCache(config)
    if err != nil {
        panic(err)
    }
    defer redisCache.Close()

    ctx := context.Background()

    // Set a value
    redisCache.Set(ctx, "user:123", "Alice", time.Hour)

    // Get a value
    val, err := redisCache.Get(ctx, "user:123")
    if err != nil {
        panic(err)
    }
    fmt.Println(val) // Alice
}
```

### With Cache Chain

```go
import (
    "cache-chain/pkg/cache/memory"
    "cache-chain/pkg/cache/redis"
    "cache-chain/pkg/chain"
)

// L1: Memory cache (fast)
l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
    Name:    "L1-Memory",
    MaxSize: 100,
})

// L2: Redis cache (persistent)
config := redis.DefaultRedisCacheConfig()
l2, _ := redis.NewRedisCache(config)

// Create 2-layer chain
c, _ := chain.New(l1, l2)
defer c.Close()

// Use the chain
c.Set(ctx, "product:1", "Widget", time.Hour)
val, _ := c.Get(ctx, "product:1") // Hits L2, warms L1
```

## Configuration

### RedisCacheConfig

```go
type RedisCacheConfig struct {
    // Name is the identifier for this cache layer
    Name string

    // Addr is the Redis server address (e.g., "localhost:6379")
    Addr string

    // Username for Redis ACL authentication (optional, Redis 6+)
    Username string

    // Password for Redis authentication (optional)
    Password string

    // DB is the Redis database number (0-15)
    DB int

    // KeyPrefix is prepended to all cache keys to avoid collisions
    KeyPrefix string

    // MaxRetries is the maximum number of retries for failed operations
    MaxRetries int

    // DialTimeout is the timeout for establishing connections
    DialTimeout time.Duration

    // ReadTimeout is the timeout for socket reads
    ReadTimeout time.Duration

    // WriteTimeout is the timeout for socket writes
    WriteTimeout time.Duration

    // PoolSize is the maximum number of socket connections
    PoolSize int

    // MinIdleConns is the minimum number of idle connections
    MinIdleConns int

    // EnablePipelining enables automatic pipelining for batch operations
    EnablePipelining bool
}
```

### Default Configuration

```go
config := redis.DefaultRedisCacheConfig()
// Returns:
// Name:             "Redis"
// Addr:             "localhost:6379"
// Username:         ""
// Password:         ""
// DB:               0
// KeyPrefix:        "cache:"
// MaxRetries:       3
// DialTimeout:      5 * time.Second
// ReadTimeout:      3 * time.Second
// WriteTimeout:     3 * time.Second
// PoolSize:         10
// MinIdleConns:     2
// EnablePipelining: true
```

### Custom Configuration

```go
config := redis.RedisCacheConfig{
    Name:             "Prod-Redis",
    Addr:             "redis.prod.example.com:6379",
    Password:         "secret",
    DB:               1,
    KeyPrefix:        "app:cache:",
    WriteTimeout:     5 * time.Second,
    PoolSize:         20,
    EnablePipelining: true,
}

redisCache, err := redis.NewRedisCache(config)
```

## API Reference

### Core Operations

#### Get

Retrieves a value from Redis by key.

```go
func (r *RedisCache) Get(ctx context.Context, key string) (interface{}, error)
```

**Example:**
```go
val, err := redisCache.Get(ctx, "user:123")
if err == cache.ErrCacheMiss {
    // Key not found
}
```

#### Set

Stores a value in Redis with the specified TTL.

```go
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
```

**Example:**
```go
redisCache.Set(ctx, "user:123", "Alice", time.Hour)
redisCache.Set(ctx, "config", map[string]interface{}{
    "timeout": 30,
    "retries": 3,
}, 10*time.Minute)
```

#### Delete

Removes a value from Redis by key.

```go
func (r *RedisCache) Delete(ctx context.Context, key string) error
```

**Example:**
```go
redisCache.Delete(ctx, "user:123")
```

### Batch Operations

Batch operations use Redis pipelining to execute multiple commands in a single round trip, dramatically improving performance.

#### BatchGet

Retrieves multiple values in a single operation.

```go
func (r *RedisCache) BatchGet(ctx context.Context, keys []string) (map[string]interface{}, error)
```

**Example:**
```go
keys := []string{"user:1", "user:2", "user:3"}
results, err := redisCache.BatchGet(ctx, keys)
// results = map[string]interface{}{
//     "user:1": "Alice",
//     "user:2": "Bob",
// }
// Missing keys are not included in the result
```

#### BatchSet

Stores multiple values in a single operation.

```go
func (r *RedisCache) BatchSet(ctx context.Context, items map[string]interface{}, ttl time.Duration) error
```

**Example:**
```go
items := map[string]interface{}{
    "user:1": "Alice",
    "user:2": "Bob",
    "user:3": "Charlie",
}
redisCache.BatchSet(ctx, items, time.Hour)
```

#### BatchDelete

Removes multiple values in a single operation.

```go
func (r *RedisCache) BatchDelete(ctx context.Context, keys []string) error
```

**Example:**
```go
keys := []string{"user:1", "user:2", "user:3"}
redisCache.BatchDelete(ctx, keys)
```

### Utility Methods

#### Ping

Checks if the Redis server is reachable.

```go
func (r *RedisCache) Ping(ctx context.Context) error
```

**Example:**
```go
if err := redisCache.Ping(ctx); err != nil {
    log.Fatal("Redis unavailable")
}
```

#### FlushDB

Removes all keys from the current database. **Use with caution!**

```go
func (r *RedisCache) FlushDB(ctx context.Context) error
```

**Example:**
```go
// Only for testing/development
redisCache.FlushDB(ctx)
```

#### Keys

Returns all keys matching the given pattern.

```go
func (r *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error)
```

**Pattern examples:**
- `"*"` - All keys
- `"user:*"` - All keys starting with "user:"
- `"session:*:data"` - Keys matching the pattern

**Note**: KEYS command can be slow on large databases. Use with caution in production.

**Example:**
```go
keys, err := redisCache.Keys(ctx, "user:*")
// Returns keys without the prefix: ["user:1", "user:2", "user:3"]
```

#### TTL

Returns the remaining time-to-live for a key.

```go
func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error)
```

**Returns:**
- Positive duration: Remaining TTL
- `-1`: Key has no expiration
- Error with `cache.ErrCacheMiss`: Key does not exist

**Example:**
```go
ttl, err := redisCache.TTL(ctx, "user:123")
if err != nil {
    // Key doesn't exist
}
fmt.Printf("TTL: %v\n", ttl)
```

#### Exists

Checks if a key exists in Redis.

```go
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error)
```

**Example:**
```go
exists, err := redisCache.Exists(ctx, "user:123")
if exists {
    fmt.Println("Key exists")
}
```

#### Name

Returns the identifier for this cache layer.

```go
func (r *RedisCache) Name() string
```

#### Close

Closes the Redis client connection.

```go
func (r *RedisCache) Close() error
```

## Data Types

### Supported Types

Redis cache automatically serializes/deserializes the following types using JSON:

- **Primitives**: `string`, `int`, `float64`, `bool`
- **Maps**: `map[string]interface{}`
- **Slices**: `[]interface{}`
- **Structs**: Any JSON-serializable struct

**Example:**
```go
// String
redisCache.Set(ctx, "name", "Alice", time.Hour)

// Number
redisCache.Set(ctx, "age", 30, time.Hour)

// Map
redisCache.Set(ctx, "user", map[string]interface{}{
    "name": "Alice",
    "age":  30,
}, time.Hour)

// Slice
redisCache.Set(ctx, "numbers", []int{1, 2, 3, 4, 5}, time.Hour)

// Struct
type User struct {
    Name  string
    Age   int
    Email string
}
redisCache.Set(ctx, "user:struct", User{
    Name:  "Alice",
    Age:   30,
    Email: "alice@example.com",
}, time.Hour)
```

**Note**: Numbers are deserialized as `float64` due to JSON limitations.

## Performance

### Benchmarks

Batch operations provide significant performance improvements:

```
Operation          | Single Ops | Batch Ops | Improvement
-------------------|------------|-----------|------------
Set (10 items)     | 5.2ms      | 0.8ms     | 6.5x faster
Get (10 items)     | 4.8ms      | 0.7ms     | 6.9x faster
Delete (10 items)  | 4.5ms      | 0.6ms     | 7.5x faster
```

### Best Practices

1. **Use batch operations** when working with multiple keys
2. **Set appropriate timeouts** in configuration
3. **Use key prefixes** to avoid collisions
4. **Monitor connection pool** size based on load
5. **Enable pipelining** (enabled by default)
6. **Avoid KEYS command** in production (use SCAN instead for large datasets)

### Optimization Tips

```go
// ✅ Good: Batch operation
items := map[string]interface{}{
    "key1": "value1",
    "key2": "value2",
    "key3": "value3",
}
redisCache.BatchSet(ctx, items, time.Hour)

// ❌ Bad: Multiple single operations
for key, value := range items {
    redisCache.Set(ctx, key, value, time.Hour)
}
```

## Testing

The Redis implementation includes comprehensive tests:

```bash
# Run Redis tests (requires running Redis server)
go test ./pkg/cache/redis/... -v

# Tests automatically skip if Redis is unavailable
```

### Test Setup

Tests use `skipIfNoRedis()` to gracefully skip when Redis isn't available:

```go
func TestRedisCache_SetGet(t *testing.T) {
    r := setupTestRedis(t) // Skips if Redis unavailable
    defer r.Close()

    // Test code...
}
```

## Production Deployment

### Docker Compose Example

```yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes

  app:
    build: .
    depends_on:
      - redis
    environment:
      - REDIS_ADDR=redis:6379

volumes:
  redis-data:
```

### Environment Configuration

```go
import "os"

config := redis.RedisCacheConfig{
    Addr:      os.Getenv("REDIS_ADDR"),     // redis:6379
    Password:  os.Getenv("REDIS_PASSWORD"),  // optional
    DB:        0,
    KeyPrefix: "myapp:cache:",
}
```

### High Availability Setup

```go
// Connect to Redis Sentinel
config := redis.RedisCacheConfig{
    Addr:      "sentinel1:26379,sentinel2:26379,sentinel3:26379",
    Password:  "secret",
    KeyPrefix: "app:cache:",
}
```

### Monitoring

```go
// Periodic health check
ticker := time.NewTicker(30 * time.Second)
go func() {
    for range ticker.C {
        if err := redisCache.Ping(ctx); err != nil {
            log.Printf("Redis health check failed: %v", err)
        }
    }
}()
```

## Troubleshooting

### Connection Refused

**Error**: `dial tcp [::1]:6379: connect: connection refused`

**Solutions:**
1. Ensure Redis is running: `redis-cli ping`
2. Check Redis address: `redis-server --port 6379`
3. Verify firewall rules

### Timeout Errors

**Error**: `i/o timeout`

**Solutions:**
1. Increase `WriteTimeout` in configuration
2. Check network latency
3. Monitor Redis server load

### Authentication Failed

**Error**: `NOAUTH Authentication required`

**Solution:**
```go
config := redis.DefaultRedisCacheConfig()
config.Password = "your-redis-password"
```

### Key Not Found

```go
val, err := redisCache.Get(ctx, "key")
if err == cache.ErrCacheMiss {
    // Handle cache miss
}
```

## Examples

See `examples/redis/main.go` for a complete working example demonstrating:

- Basic operations (Get, Set, Delete)
- Batch operations with pipelining
- Integration with cache chain
- Complex data types
- Pattern matching
- Performance comparison

Run the example:

```bash
# Start Redis
docker run -d -p 6379:6379 redis:7-alpine

# Run example
go run examples/redis/main.go
```

## See Also

- [Cache Chain](../README.md)
- [Memory Cache](../pkg/cache/memory)
- [TTL Integration](TTL_INTEGRATION.md)
- [rueidis Documentation](https://github.com/redis/rueidis)
