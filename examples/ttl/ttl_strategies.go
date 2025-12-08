package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"cache-chain/pkg/cache/memory"
	"cache-chain/pkg/chain"
)

func main() {
	fmt.Println("=== TTL Strategies Demo ===\n")

	// 1. Uniform TTL Strategy
	demonstrateUniformStrategy()

	// 2. Decaying TTL Strategy
	demonstrateDecayingStrategy()

	// 3. Custom TTL Strategy
	demonstrateCustomStrategy()

	// 4. TTL Strategy with Warmup
	demonstrateTTLWithWarmup()
}

func demonstrateUniformStrategy() {
	fmt.Println("1. Uniform TTL Strategy")
	fmt.Println("-----------------------")

	// Create layers
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1-Memory", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2-Redis", MaxSize: 1000})
	l3 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L3-Memcached", MaxSize: 10000})

	// Create chain with uniform strategy (default)
	c, err := chain.New(l1, l2, l3)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx := context.Background()

	// Set with uniform TTL - all layers get the same TTL
	err = c.Set(ctx, "user:123", "Alice", 1*time.Hour)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("  ✓ Set 'user:123' with 1h TTL")
	fmt.Println("  → All layers: 1h TTL (uniform)")
	fmt.Println()
}

func demonstrateDecayingStrategy() {
	fmt.Println("2. Decaying TTL Strategy")
	fmt.Println("------------------------")

	// Create layers
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1-Memory", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2-Redis", MaxSize: 1000})
	l3 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L3-DB", MaxSize: 10000})

	// Create chain with decaying strategy (50% decay per layer)
	strategy := &chain.DecayingTTLStrategy{DecayFactor: 0.5}
	config := chain.ChainConfig{
		TTLStrategy: strategy,
	}

	c, err := chain.NewWithConfig(config, l1, l2, l3)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx := context.Background()
	baseTTL := 8 * time.Hour

	// Set with decaying TTL
	err = c.Set(ctx, "session:xyz", "token-abc-123", baseTTL)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("  ✓ Set 'session:xyz' with %v base TTL\n", baseTTL)
	fmt.Printf("  → L1: %v\n", strategy.GetTTL(0, baseTTL))
	fmt.Printf("  → L2: %v\n", strategy.GetTTL(1, baseTTL))
	fmt.Printf("  → L3: %v\n", strategy.GetTTL(2, baseTTL))
	fmt.Println("  (Exponential decay: L1 expires first, L3 lasts longest)")
	fmt.Println()
}

func demonstrateCustomStrategy() {
	fmt.Println("3. Custom TTL Strategy")
	fmt.Println("----------------------")

	// Create layers
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1-Fast", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2-Medium", MaxSize: 1000})
	l3 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L3-Persistent", MaxSize: 10000})

	// Create chain with custom TTL per layer
	strategy := &chain.CustomTTLStrategy{
		TTLs: []time.Duration{
			5 * time.Minute,  // L1: Hot data, short TTL
			30 * time.Minute, // L2: Warm data, medium TTL
			4 * time.Hour,    // L3: Cold data, long TTL
		},
	}
	config := chain.ChainConfig{
		TTLStrategy: strategy,
	}

	c, err := chain.NewWithConfig(config, l1, l2, l3)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx := context.Background()

	// Set with custom TTL per layer
	err = c.Set(ctx, "product:456", "Product Details", 24*time.Hour)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("  ✓ Set 'product:456' with custom TTL per layer")
	fmt.Println("  → L1 (Fast):       5m  - Hot cache, expires quickly")
	fmt.Println("  → L2 (Medium):     30m - Warm cache, medium lifetime")
	fmt.Println("  → L3 (Persistent): 4h  - Cold cache, long lifetime")
	fmt.Println("  (Base TTL of 24h is overridden by custom values)")
	fmt.Println()
}

func demonstrateTTLWithWarmup() {
	fmt.Println("4. TTL Strategy with Automatic Warmup")
	fmt.Println("--------------------------------------")

	// Create layers
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2", MaxSize: 1000})
	l3 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L3", MaxSize: 10000})

	// Custom strategy with increasing TTL per layer
	strategy := &chain.CustomTTLStrategy{
		TTLs: []time.Duration{
			2 * time.Minute,
			10 * time.Minute,
			1 * time.Hour,
		},
	}
	config := chain.ChainConfig{
		TTLStrategy: strategy,
	}

	c, err := chain.NewWithConfig(config, l1, l2, l3)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx := context.Background()

	// Set only in L3 (simulating database)
	err = l3.Set(ctx, "article:789", "Article Content", 2*time.Hour)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("  ✓ Set 'article:789' only in L3")
	fmt.Println("  ⏱  Getting from chain...")

	// Get from chain - will hit L3 and warm up L1 and L2
	start := time.Now()
	val, err := c.Get(ctx, "article:789")
	duration := time.Since(start)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("  ✓ Got '%v' from L3 in %v\n", val, duration)
	fmt.Println("  → Automatically warming L1 and L2 with appropriate TTLs:")
	fmt.Println("    • L1 will get 2m TTL")
	fmt.Println("    • L2 will get 10m TTL")

	// Wait for async warmup
	time.Sleep(50 * time.Millisecond)

	// Second get should hit L1 (much faster)
	start = time.Now()
	val, err = c.Get(ctx, "article:789")
	duration = time.Since(start)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("  ⚡ Got '%v' from L1 in %v (much faster!)\n", val, duration)
	fmt.Println()
}

func init() {
	log.SetFlags(0)
}
