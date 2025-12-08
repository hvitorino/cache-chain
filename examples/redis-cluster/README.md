# Redis Cluster Example

This example demonstrates using cache-chain with Redis Cluster for high availability and horizontal scaling.

## Features

- **Cluster Mode**: Automatic data sharding across multiple nodes
- **High Availability**: Master-replica architecture with automatic failover
- **Horizontal Scaling**: Distribute load across multiple Redis instances
- **Pipelining Support**: Batch operations work seamlessly across shards

## Quick Start

### 1. Start Redis Cluster (Docker Compose)

```bash
cd examples/redis-cluster
docker-compose up -d
```

This will create a 6-node Redis cluster (3 masters + 3 replicas).

### 2. Verify Cluster

```bash
docker exec redis-node-1 redis-cli cluster info
docker exec redis-node-1 redis-cli cluster nodes
```

### 3. Run the Example

```bash
go run main.go
```

## Configuration Examples

### Basic Cluster Configuration

```go
config := redis.ClusterCacheConfig(
    "Redis-Cluster",
    []string{
        "localhost:6379",
        "localhost:6380",
        "localhost:6381",
        "localhost:6382",
        "localhost:6383",
        "localhost:6384",
    },
    "", // password (empty for local development)
)

cache, err := redis.NewRedisCache(config)
if err != nil {
    log.Fatal(err)
}
defer cache.Close()
```

### With Cache Chain

```go
// L1: Memory cache
l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
    Name:    "L1-Memory",
    MaxSize: 1000,
})

// L2: Redis Cluster
l2, _ := redis.NewRedisCache(redis.ClusterCacheConfig(
    "L2-Cluster",
    []string{"node1:6379", "node2:6379", "node3:6379"},
    "password",
))

// Create chain
chain, _ := chain.New(l1, l2)
defer chain.Close()

// Use it
chain.Set(ctx, "key", "value", time.Hour)
val, _ := chain.Get(ctx, "key")
```

### Production Environment

```go
config := redis.ClusterCacheConfig(
    "Prod-Cluster",
    strings.Split(os.Getenv("REDIS_CLUSTER_NODES"), ","),
    os.Getenv("REDIS_PASSWORD"),
)
config.PoolSize = 30
config.WriteTimeout = 5 * time.Second
config.KeyPrefix = "app:cache:"

cache, err := redis.NewRedisCache(config)
```

## Sentinel Mode (Alternative HA Setup)

For Sentinel-based high availability instead of cluster:

```go
config := redis.SentinelCacheConfig(
    "Redis-HA",
    []string{
        "sentinel1:26379",
        "sentinel2:26379",
        "sentinel3:26379",
    },
    "mymaster", // master set name
    "password",
)

cache, err := redis.NewRedisCache(config)
```

## Testing Cluster Operations

```go
ctx := context.Background()

// Test basic operations
cache.Set(ctx, "user:123", "Alice", time.Hour)
val, _ := cache.Get(ctx, "user:123")

// Test batch operations (uses pipelining)
items := map[string]interface{}{
    "user:1": "Alice",
    "user:2": "Bob",
    "user:3": "Charlie",
}
cache.BatchSet(ctx, items, time.Hour)

results, _ := cache.BatchGet(ctx, []string{"user:1", "user:2", "user:3"})

// Test cluster health
if err := cache.Ping(ctx); err != nil {
    log.Fatal("Cluster unavailable:", err)
}
```

## Cluster Benefits

### Data Sharding
- Automatic distribution of keys across nodes using hash slots
- 16384 hash slots divided among master nodes
- Keys with hash tags `{user:123}` stay on same node

### High Availability
- Each master has one or more replicas
- Automatic failover when master goes down
- No single point of failure

### Scalability
- Add nodes dynamically to increase capacity
- Rebalance hash slots automatically
- Linear scalability for read operations

### Performance
- Parallel operations across multiple nodes
- Pipelining works across cluster
- Reduced latency with data locality

## Cluster Topology

```
Master 1 (6379) ← Replica 4 (6382)
Master 2 (6380) ← Replica 5 (6383)
Master 3 (6381) ← Replica 6 (6384)
```

Each master handles ~5461 hash slots.

## Monitoring Cluster

```bash
# Cluster info
docker exec redis-node-1 redis-cli cluster info

# Node status
docker exec redis-node-1 redis-cli cluster nodes

# Check specific key location
docker exec redis-node-1 redis-cli cluster keyslot "user:123"

# Get node serving a slot
docker exec redis-node-1 redis-cli cluster nodes | grep master
```

## Troubleshooting

### Cluster Not Forming

```bash
# Check cluster state on each node
docker exec redis-node-1 redis-cli cluster info

# Reset cluster (CAUTION: deletes data)
docker exec redis-node-1 redis-cli cluster reset soft
```

### Connection Issues

```go
// Increase timeout for cluster discovery
config := redis.ClusterCacheConfig(...)
config.WriteTimeout = 10 * time.Second

// Test connectivity to all nodes
for _, addr := range config.ClusterAddrs {
    conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
    if err != nil {
        log.Printf("Cannot reach %s: %v", addr, err)
    }
    conn.Close()
}
```

### Failover Testing

```bash
# Stop a master node
docker stop redis-node-1

# Watch automatic failover (should promote replica)
docker exec redis-node-2 redis-cli cluster nodes

# Restart node (will rejoin as replica)
docker start redis-node-1
```

## Cleanup

```bash
# Stop cluster
docker-compose down

# Remove volumes (clears data)
docker-compose down -v
```

## Best Practices

1. **Use hash tags** for related keys to keep them on same node
2. **Monitor cluster health** with periodic `CLUSTER INFO`
3. **Plan capacity** based on expected data growth
4. **Use replicas** for read scaling
5. **Set appropriate timeouts** for cluster operations
6. **Test failover scenarios** in staging
7. **Use key prefixes** to organize data
8. **Monitor slot distribution** for balance

## See Also

- [Redis Cluster Tutorial](https://redis.io/docs/manual/scaling/)
- [Redis Cluster Specification](https://redis.io/docs/reference/cluster-spec/)
- [Main Redis Cache Documentation](../../docs/REDIS_CACHE.md)
