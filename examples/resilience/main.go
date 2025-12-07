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
	// Create a 2-layer cache system with automatic resilience
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:     "L1-Memory",
		MaxSize:  100,
		DefaultTTL: 1 * time.Minute,
	})

	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:     "L2-Slower",
		MaxSize:  1000,
		DefaultTTL: 10 * time.Minute,
	})

	// Create chain - layers are automatically wrapped with resilience protection
	// L1 gets 100ms timeout, L2 gets 1s timeout
	cacheChain, err := chain.New(l1, l2)
	if err != nil {
		log.Fatalf("Failed to create chain: %v", err)
	}
	defer cacheChain.Close()

	ctx := context.Background()

	// Populate L2 with some data
	fmt.Println("Setting up test data...")
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		if err := l2.Set(ctx, key, value, 10*time.Minute); err != nil {
			log.Printf("Failed to set %s: %v", key, err)
		}
	}

	// Demonstrate resilience features
	fmt.Println("\n=== Cache Chain with Resilience ===")

	// Normal operation - cache miss in L1, hit in L2
	fmt.Println("\n1. Normal operation (L1 miss, L2 hit):")
	val, err := cacheChain.Get(ctx, "key1")
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("   Got value: %v\n", val)
	}

	// Check if L1 was warmed up
	time.Sleep(100 * time.Millisecond)
	val, err = cacheChain.Get(ctx, "key1")
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("   Second get (from L1): %v\n", val)
	}

	// Demonstrate timeout protection
	fmt.Println("\n2. Timeout protection:")
	fmt.Println("   Layers are protected with timeouts:")
	fmt.Println("   - L1 (memory): 100ms timeout")
	fmt.Println("   - L2 (slower): 1s timeout")

	// Demonstrate circuit breaker
	fmt.Println("\n3. Circuit breaker protection:")
	fmt.Println("   After 5 consecutive failures, circuit opens")
	fmt.Println("   Circuit remains open for 10s, then enters half-open")
	fmt.Println("   In half-open, 1 success closes the circuit")

	// Test with various keys
	fmt.Println("\n4. Testing multiple keys:")
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("key%d", i)
		val, err := cacheChain.Get(ctx, key)
		if err != nil {
			log.Printf("   %s: error - %v", key, err)
		} else {
			fmt.Printf("   %s: %v\n", key, val)
		}
	}

	// Demonstrate graceful degradation
	fmt.Println("\n5. Graceful degradation:")
	fmt.Println("   If a layer fails, the chain continues with remaining layers")
	fmt.Println("   Circuit breakers prevent cascade failures")

	// Show layer information
	fmt.Println("\n6. Chain configuration:")
	fmt.Printf("   Layers: %s\n", cacheChain.String())
	fmt.Println("   Each layer is wrapped with:")
	fmt.Println("   - Circuit breaker (gobreaker)")
	fmt.Println("   - Timeout protection")
	fmt.Println("   - Automatic error handling")

	fmt.Println("\n✅ Resilience features demonstrated successfully!")
}
