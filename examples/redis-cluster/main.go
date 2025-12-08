package main

import (
	"fmt"
	"time"

	"cache-chain/pkg/cache/redis"
)

func main() {
	fmt.Println("=== Redis Cluster Mode Example ===")
	fmt.Println()

	// Example 1: Redis Cluster Configuration
	fmt.Println("--- Redis Cluster ---")
	clusterConfig := redis.ClusterCacheConfig(
		"Redis-Cluster",
		[]string{
			"node1.cluster.local:6379",
			"node2.cluster.local:6379",
			"node3.cluster.local:6379",
			"node4.cluster.local:6379",
			"node5.cluster.local:6379",
			"node6.cluster.local:6379",
		},
		"cluster-password",
	)
	clusterConfig.KeyPrefix = "app:cache:"
	clusterConfig.PoolSize = 20

	fmt.Printf("Cluster Addresses: %v\n", clusterConfig.ClusterAddrs)
	fmt.Printf("Key Prefix: %s\n", clusterConfig.KeyPrefix)
	fmt.Println()

	// Note: This will fail without actual cluster nodes
	clusterCache, err := redis.NewRedisCache(clusterConfig)
	if err != nil {
		fmt.Printf("✓ Cluster config created (actual connection requires cluster nodes)\n")
		fmt.Printf("  Error (expected): %v\n", err)
	} else {
		defer clusterCache.Close()
		fmt.Println("✓ Connected to Redis Cluster")
	}

	fmt.Println()

	// Example 2: Redis Sentinel Configuration for High Availability
	fmt.Println("--- Redis Sentinel (HA) ---")
	sentinelConfig := redis.SentinelCacheConfig(
		"Redis-Sentinel",
		[]string{
			"sentinel1.example.com:26379",
			"sentinel2.example.com:26379",
			"sentinel3.example.com:26379",
		},
		"mymaster",
		"sentinel-password",
	)
	sentinelConfig.KeyPrefix = "app:cache:"
	sentinelConfig.SentinelUsername = "sentinel-user"
	sentinelConfig.SentinelPassword = "sentinel-pass"

	fmt.Printf("Sentinel Addresses: %v\n", sentinelConfig.SentinelAddrs)
	fmt.Printf("Master Set: %s\n", sentinelConfig.SentinelMasterSet)
	fmt.Println()

	sentinelCache, err := redis.NewRedisCache(sentinelConfig)
	if err != nil {
		fmt.Printf("✓ Sentinel config created (actual connection requires sentinel setup)\n")
		fmt.Printf("  Error (expected): %v\n", err)
	} else {
		defer sentinelCache.Close()
		fmt.Println("✓ Connected to Redis via Sentinel")
	}

	fmt.Println()

	// Example 3: Manual Cluster Configuration with Custom Settings
	fmt.Println("--- Custom Cluster Configuration ---")
	customConfig := redis.RedisCacheConfig{
		Name: "Custom-Cluster",
		ClusterAddrs: []string{
			"10.0.1.10:6379",
			"10.0.1.11:6379",
			"10.0.1.12:6379",
		},
		Username:         "admin",
		Password:         "secret",
		KeyPrefix:        "myapp:",
		WriteTimeout:     5 * time.Second,
		PoolSize:         30,
		EnablePipelining: true,
	}

	fmt.Printf("Custom Cluster: %d nodes\n", len(customConfig.ClusterAddrs))
	fmt.Printf("Pool Size: %d\n", customConfig.PoolSize)
	fmt.Printf("Pipelining: %v\n", customConfig.EnablePipelining)
	fmt.Println()

	// Example 4: Using Cluster Cache in Production
	fmt.Println("--- Production Usage Example ---")
	fmt.Println(`
Production configuration example:

// Docker Compose / Kubernetes environment
config := redis.ClusterCacheConfig(
    "Redis-Cluster",
    strings.Split(os.Getenv("REDIS_CLUSTER_NODES"), ","),
    os.Getenv("REDIS_PASSWORD"),
)

// Or for Sentinel HA setup
config := redis.SentinelCacheConfig(
    "Redis-HA",
    strings.Split(os.Getenv("SENTINEL_ADDRESSES"), ","),
    os.Getenv("REDIS_MASTER_NAME"),
    os.Getenv("REDIS_PASSWORD"),
)

cache, err := redis.NewRedisCache(config)
if err != nil {
    log.Fatal(err)
}

// Test connectivity
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := cache.Ping(ctx); err != nil {
    log.Fatal("Redis cluster not available:", err)
}
	`)

	// Example 5: Cluster Benefits
	fmt.Println("--- Why Use Redis Cluster? ---")
	fmt.Println(`
✓ High Availability: Automatic failover
✓ Horizontal Scaling: Distribute data across nodes
✓ Sharding: Automatic data partitioning
✓ Performance: Parallel operations across shards
✓ No Single Point of Failure: Master-replica architecture

Cluster Setup (Docker Compose example):
	
services:
  redis-node-1:
    image: redis:7-cluster
    command: redis-server --cluster-enabled yes
    
  redis-node-2:
    image: redis:7-cluster
    command: redis-server --cluster-enabled yes
    
  # ... more nodes ...
	`)

	fmt.Println()
	fmt.Println("✓ Example completed!")
	fmt.Println()
	fmt.Println("Note: To test with actual cluster:")
	fmt.Println("  docker-compose -f examples/redis/docker-compose-cluster.yml up")
}
