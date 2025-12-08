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
	// Create three memory cache layers
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:            "L1",
		DefaultTTL:      time.Minute,
		CleanupInterval: time.Second,
	})

	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:            "L2",
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: time.Second,
	})

	l3 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:            "L3",
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Second,
	})

	// Create a chain: L1 → L2 → L3
	cacheChain, err := chain.New(l1, l2, l3)
	if err != nil {
		log.Fatal(err)
	}
	defer cacheChain.Close()

	ctx := context.Background()

	fmt.Println("=== Cache Chain Integration Test ===")
	fmt.Printf("Chain: %s\n\n", cacheChain.String())

	// Populate L3 with some data (simulating a database)
	fmt.Println("1. Populating L3 with data...")
	err = l3.Set(ctx, "user:123", "John Doe", time.Hour)
	if err != nil {
		log.Fatal(err)
	}
	err = l3.Set(ctx, "product:456", "Laptop", time.Hour)
	if err != nil {
		log.Fatal(err)
	}

	// First Get: L1 miss → L2 miss → L3 hit → warm L2 & L1
	fmt.Println("2. First Get (should hit L3 and warm upper layers)...")
	value, err := cacheChain.Get(ctx, "user:123")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   Got: %s\n", value)

	// Second Get: should hit L1
	fmt.Println("3. Second Get (should hit L1)...")
	value, err = cacheChain.Get(ctx, "user:123")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   Got: %s\n", value)

	// Get non-existent key
	fmt.Println("4. Get non-existent key...")
	_, err = cacheChain.Get(ctx, "nonexistent")
	if err != nil {
		fmt.Printf("   Expected error: %v\n", err)
	}

	// Set through chain (writes to all layers)
	fmt.Println("5. Set through chain...")
	err = cacheChain.Set(ctx, "new:key", "new value", time.Minute)
	if err != nil {
		log.Fatal(err)
	}

	// Get the new value (should hit L1)
	value, err = cacheChain.Get(ctx, "new:key")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("   Got: %s\n", value)

	fmt.Println("\n=== Test completed successfully! ===")
}