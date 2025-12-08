package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"cache-chain/pkg/cache/memory"
	"cache-chain/pkg/cache/redis"
	"cache-chain/pkg/chain"
)

func main() {
	fmt.Println("=== Redis Cache with Pipelining Demo ===")
	fmt.Println()

	// 1. Create Redis cache layer
	redisConfig := redis.DefaultRedisCacheConfig()
	redisConfig.Name = "L2-Redis"
	redisConfig.KeyPrefix = "demo:cache:"
	redisConfig.EnablePipelining = true

	redisCache, err := redis.NewRedisCache(redisConfig)
	if err != nil {
		log.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	fmt.Println("✓ Connected to Redis")
	fmt.Println()

	// 2. Test basic operations
	ctx := context.Background()

	fmt.Println("--- Basic Operations ---")
	
	// Set
	if err := redisCache.Set(ctx, "user:123", "Alice", time.Hour); err != nil {
		log.Fatalf("Failed to set: %v", err)
	}
	fmt.Println("✓ Set user:123 = Alice")

	// Get
	val, err := redisCache.Get(ctx, "user:123")
	if err != nil {
		log.Fatalf("Failed to get: %v", err)
	}
	fmt.Printf("✓ Get user:123 = %v\n", val)

	// Exists
	exists, err := redisCache.Exists(ctx, "user:123")
	if err != nil {
		log.Fatalf("Failed to check exists: %v", err)
	}
	fmt.Printf("✓ Exists user:123 = %v\n", exists)

	// TTL
	ttl, err := redisCache.TTL(ctx, "user:123")
	if err != nil {
		log.Fatalf("Failed to get TTL: %v", err)
	}
	fmt.Printf("✓ TTL user:123 = %v\n", ttl)

	fmt.Println()

	// 3. Test batch operations with pipelining
	fmt.Println("--- Batch Operations (Pipelining) ---")

	// Batch Set
	users := map[string]interface{}{
		"user:1": "Alice",
		"user:2": "Bob",
		"user:3": "Charlie",
		"user:4": "Diana",
		"user:5": "Eve",
	}

	start := time.Now()
	if err := redisCache.BatchSet(ctx, users, time.Hour); err != nil {
		log.Fatalf("Failed to batch set: %v", err)
	}
	batchSetDuration := time.Since(start)
	fmt.Printf("✓ Batch Set 5 users in %v\n", batchSetDuration)

	// Batch Get
	keys := []string{"user:1", "user:2", "user:3", "user:4", "user:5"}
	
	start = time.Now()
	results, err := redisCache.BatchGet(ctx, keys)
	if err != nil {
		log.Fatalf("Failed to batch get: %v", err)
	}
	batchGetDuration := time.Since(start)
	fmt.Printf("✓ Batch Get 5 users in %v\n", batchGetDuration)
	fmt.Printf("  Retrieved %d values\n", len(results))

	// Batch Delete
	start = time.Now()
	if err := redisCache.BatchDelete(ctx, keys); err != nil {
		log.Fatalf("Failed to batch delete: %v", err)
	}
	batchDeleteDuration := time.Since(start)
	fmt.Printf("✓ Batch Delete 5 users in %v\n", batchDeleteDuration)

	fmt.Println()

	// 4. Test with cache chain (L1 Memory + L2 Redis)
	fmt.Println("--- Cache Chain (Memory + Redis) ---")

	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "L1-Memory",
		MaxSize: 100,
	})

	c, err := chain.New(l1, redisCache)
	if err != nil {
		log.Fatalf("Failed to create chain: %v", err)
	}
	defer c.Close()

	fmt.Println("✓ Created 2-layer chain (Memory -> Redis)")

	// Set through chain
	c.Set(ctx, "product:1", "Widget", time.Hour)
	fmt.Println("✓ Set product:1 through chain")

	// First get (should warm L1 from L2)
	start = time.Now()
	val, err = c.Get(ctx, "product:1")
	if err != nil {
		log.Fatalf("Failed to get: %v", err)
	}
	firstGetDuration := time.Since(start)
	fmt.Printf("✓ First Get product:1 = %v (from L2, warmed L1) in %v\n", val, firstGetDuration)

	// Second get (should be fast from L1)
	start = time.Now()
	val, err = c.Get(ctx, "product:1")
	if err != nil {
		log.Fatalf("Failed to get: %v", err)
	}
	secondGetDuration := time.Since(start)
	fmt.Printf("✓ Second Get product:1 = %v (from L1) in %v\n", val, secondGetDuration)

	speedup := float64(firstGetDuration) / float64(secondGetDuration)
	fmt.Printf("  L1 is %.1fx faster than L2\n", speedup)

	fmt.Println()

	// 5. Test complex data types
	fmt.Println("--- Complex Data Types ---")

	complexData := map[string]interface{}{
		"string": "Hello, Redis!",
		"number": 42,
		"float":  3.14159,
		"bool":   true,
		"map": map[string]interface{}{
			"name":  "Alice",
			"age":   30,
			"email": "alice@example.com",
		},
		"slice": []interface{}{1, 2, 3, 4, 5},
	}

	for key, value := range complexData {
		if err := redisCache.Set(ctx, key, value, time.Hour); err != nil {
			log.Fatalf("Failed to set %s: %v", key, err)
		}
	}
	fmt.Println("✓ Stored 6 complex data types")

	// Retrieve and verify
	for key := range complexData {
		val, err := redisCache.Get(ctx, key)
		if err != nil {
			log.Fatalf("Failed to get %s: %v", key, err)
		}
		fmt.Printf("  %s: %v\n", key, val)
	}

	fmt.Println()

	// 6. Test pattern matching
	fmt.Println("--- Pattern Matching ---")

	// Set some test data
	testData := map[string]interface{}{
		"session:abc123": "user_1",
		"session:def456": "user_2",
		"session:ghi789": "user_3",
		"config:timeout":  30,
		"config:retries":  3,
	}

	if err := redisCache.BatchSet(ctx, testData, time.Hour); err != nil {
		log.Fatalf("Failed to set test data: %v", err)
	}

	// Find all session keys
	sessionKeys, err := redisCache.Keys(ctx, "session:*")
	if err != nil {
		log.Fatalf("Failed to find keys: %v", err)
	}
	fmt.Printf("✓ Found %d session keys:\n", len(sessionKeys))
	for _, key := range sessionKeys {
		fmt.Printf("  - %s\n", key)
	}

	// Find all config keys
	configKeys, err := redisCache.Keys(ctx, "config:*")
	if err != nil {
		log.Fatalf("Failed to find keys: %v", err)
	}
	fmt.Printf("✓ Found %d config keys:\n", len(configKeys))
	for _, key := range configKeys {
		fmt.Printf("  - %s\n", key)
	}

	fmt.Println()

	// 7. Performance comparison: Single vs Batch
	fmt.Println("--- Performance Comparison ---")

	// Single operations
	singleStart := time.Now()
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("perf:single:%d", i)
		redisCache.Set(ctx, key, fmt.Sprintf("value%d", i), time.Minute)
	}
	singleDuration := time.Since(singleStart)
	fmt.Printf("Single Set (10 ops): %v\n", singleDuration)

	// Batch operations
	batchData := make(map[string]interface{})
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("perf:batch:%d", i)
		batchData[key] = fmt.Sprintf("value%d", i)
	}

	batchStart := time.Now()
	redisCache.BatchSet(ctx, batchData, time.Minute)
	batchDuration := time.Since(batchStart)
	fmt.Printf("Batch Set (10 ops):  %v\n", batchDuration)

	improvement := float64(singleDuration) / float64(batchDuration)
	fmt.Printf("Batch is %.1fx faster\n", improvement)

	fmt.Println()
	fmt.Println("✓ Demo completed successfully!")
}
