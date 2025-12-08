package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/cache/bloom"
	"cache-chain/pkg/cache/memory"
	"cache-chain/pkg/chain"
)

func main() {
	fmt.Println("=== Phase 8: Advanced Features Demo ===")
	fmt.Println()

	// 1. Negative Caching
	demonstrateNegativeCaching()

	// 2. Bloom Filters
	demonstrateBloomFilter()

	// 3. Batch Operations
	demonstrateBatchOperations()

	// 4. TTL Strategies
	demonstrateTTLStrategies()
}

func demonstrateNegativeCaching() {
	fmt.Println("1. Negative Caching")
	fmt.Println("-------------------")

	// Create a base layer
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "L1",
		MaxSize: 100,
	})

	// Wrap with negative caching
	negative := cache.NewNegativeCacheLayer(l1, 5*time.Second)
	defer negative.Close()

	ctx := context.Background()

	// First get of missing key - will cache negative result
	start := time.Now()
	_, err := negative.Get(ctx, "missing-key")
	duration1 := time.Since(start)
	fmt.Printf("  First Get (miss): %v in %v\n", err, duration1)

	// Second get - served from negative cache (faster)
	start = time.Now()
	_, err = negative.Get(ctx, "missing-key")
	duration2 := time.Since(start)
	fmt.Printf("  Second Get (negative cached): %v in %v\n", err, duration2)

	stats := negative.Stats()
	fmt.Printf("  Negative cache stats: %d entries\n", stats.NegativeCount)
	fmt.Println()
}

func demonstrateBloomFilter() {
	fmt.Println("2. Bloom Filter")
	fmt.Println("---------------")

	// Create base layer
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "L1",
		MaxSize: 1000,
	})

	// Wrap with bloom filter
	bloomLayer := bloom.NewBloomLayer(l1, 1000, 0.01)
	defer bloomLayer.Close()

	ctx := context.Background()

	// Add some keys
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		bloomLayer.Set(ctx, key, fmt.Sprintf("value-%d", i), time.Hour)
	}

	// Query existing key (will pass bloom filter)
	_, err := bloomLayer.Get(ctx, "key-5")
	fmt.Printf("  Get existing key: %v\n", err)

	// Query non-existent key (bloom filter rejects immediately)
	_, err = bloomLayer.Get(ctx, "never-set-key")
	fmt.Printf("  Get non-existent key: %v\n", err)

	stats := bloomLayer.Stats()
	fmt.Printf("  Bloom stats: %.1f%% rejection rate, %.1f%% false positive rate\n",
		stats.RejectionRate*100, stats.FalsePositiveRate*100)
	fmt.Println()
}

func demonstrateBatchOperations() {
	fmt.Println("3. Batch Operations")
	fmt.Println("-------------------")

	// Create base layer
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "L1",
		MaxSize: 1000,
	})

	// Wrap with batch adapter
	batchLayer := cache.NewBatchAdapter(l1)
	defer batchLayer.Close()

	ctx := context.Background()

	// Set multiple items at once
	items := map[string]interface{}{
		"user:1": "Alice",
		"user:2": "Bob",
		"user:3": "Charlie",
		"user:4": "David",
		"user:5": "Eve",
	}

	start := time.Now()
	err := batchLayer.SetMulti(ctx, items, time.Hour)
	duration := time.Since(start)
	fmt.Printf("  SetMulti (5 items): %v in %v\n", err, duration)

	// Get multiple items at once
	keys := []string{"user:1", "user:3", "user:5", "user:99"}
	start = time.Now()
	results, err := batchLayer.GetMulti(ctx, keys)
	duration = time.Since(start)
	fmt.Printf("  GetMulti (4 keys): %v results in %v\n", len(results), duration)

	for key, value := range results {
		fmt.Printf("    %s = %v\n", key, value)
	}

	// Delete multiple items
	deleteKeys := []string{"user:1", "user:2"}
	err = batchLayer.DeleteMulti(ctx, deleteKeys)
	fmt.Printf("  DeleteMulti (2 keys): %v\n", err)
	fmt.Println()
}

func demonstrateTTLStrategies() {
	fmt.Println("4. TTL Strategies")
	fmt.Println("-----------------")

	baseTTL := 1 * time.Hour

	// Uniform strategy
	uniform := &chain.UniformTTLStrategy{}
	fmt.Println("  Uniform Strategy:")
	for i := 0; i < 3; i++ {
		ttl := uniform.GetTTL(i, baseTTL)
		fmt.Printf("    Layer %d: %v\n", i, ttl)
	}

	// Decaying strategy
	decaying := &chain.DecayingTTLStrategy{DecayFactor: 0.5}
	fmt.Println("  Decaying Strategy (0.5 factor):")
	for i := 0; i < 3; i++ {
		ttl := decaying.GetTTL(i, baseTTL)
		fmt.Printf("    Layer %d: %v\n", i, ttl)
	}

	// Custom strategy
	custom := &chain.CustomTTLStrategy{
		TTLs: []time.Duration{5 * time.Minute, 15 * time.Minute, 1 * time.Hour},
	}
	fmt.Println("  Custom Strategy:")
	for i := 0; i < 3; i++ {
		ttl := custom.GetTTL(i, baseTTL)
		fmt.Printf("    Layer %d: %v\n", i, ttl)
	}
	fmt.Println()
}

func init() {
	log.SetFlags(0)
}
