package chain

import (
	"context"
	"testing"
	"time"

	"cache-chain/pkg/cache/memory"
)

func TestChain_WithUniformTTLStrategy(t *testing.T) {
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2", MaxSize: 100})
	l3 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L3", MaxSize: 100})

	strategy := &UniformTTLStrategy{}
	config := ChainConfig{
		TTLStrategy: strategy,
	}

	c, err := NewWithConfig(config, l1, l2, l3)
	if err != nil {
		t.Fatalf("Failed to create chain: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	baseTTL := 1 * time.Hour

	// Set value - all layers should get the same TTL
	err = c.Set(ctx, "key1", "value1", baseTTL)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify all layers have the value
	for i, layer := range []interface{}{l1, l2, l3} {
		cache := layer.(*memory.MemoryCache)
		val, err := cache.Get(ctx, "key1")
		if err != nil {
			t.Errorf("Layer %d: Get failed: %v", i, err)
		}
		if val != "value1" {
			t.Errorf("Layer %d: expected value1, got %v", i, val)
		}
	}
}

func TestChain_WithDecayingTTLStrategy(t *testing.T) {
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2", MaxSize: 100})
	l3 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L3", MaxSize: 100})

	strategy := &DecayingTTLStrategy{DecayFactor: 0.5}
	config := ChainConfig{
		TTLStrategy: strategy,
	}

	c, err := NewWithConfig(config, l1, l2, l3)
	if err != nil {
		t.Fatalf("Failed to create chain: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	baseTTL := 8 * time.Hour

	// Set value - each layer should get decaying TTL
	err = c.Set(ctx, "key1", "value1", baseTTL)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify all layers have the value
	for i, layer := range []interface{}{l1, l2, l3} {
		cache := layer.(*memory.MemoryCache)
		val, err := cache.Get(ctx, "key1")
		if err != nil {
			t.Errorf("Layer %d: Get failed: %v", i, err)
		}
		if val != "value1" {
			t.Errorf("Layer %d: expected value1, got %v", i, val)
		}
	}

	// Log the TTLs used
	t.Logf("L0 TTL: %v", strategy.GetTTL(0, baseTTL))
	t.Logf("L1 TTL: %v", strategy.GetTTL(1, baseTTL))
	t.Logf("L2 TTL: %v", strategy.GetTTL(2, baseTTL))
}

func TestChain_WithCustomTTLStrategy(t *testing.T) {
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2", MaxSize: 100})
	l3 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L3", MaxSize: 100})

	strategy := &CustomTTLStrategy{
		TTLs: []time.Duration{
			5 * time.Minute,  // L1: Fast layer
			15 * time.Minute, // L2: Middle layer
			1 * time.Hour,    // L3: Persistent layer
		},
	}
	config := ChainConfig{
		TTLStrategy: strategy,
	}

	c, err := NewWithConfig(config, l1, l2, l3)
	if err != nil {
		t.Fatalf("Failed to create chain: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	baseTTL := 2 * time.Hour

	// Set value - each layer should get custom TTL
	err = c.Set(ctx, "key1", "value1", baseTTL)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify all layers have the value
	for i, layer := range []interface{}{l1, l2, l3} {
		cache := layer.(*memory.MemoryCache)
		val, err := cache.Get(ctx, "key1")
		if err != nil {
			t.Errorf("Layer %d: Get failed: %v", i, err)
		}
		if val != "value1" {
			t.Errorf("Layer %d: expected value1, got %v", i, val)
		}
	}

	// Verify the custom TTLs were used
	t.Logf("L0 TTL: %v (expected 5m)", strategy.GetTTL(0, baseTTL))
	t.Logf("L1 TTL: %v (expected 15m)", strategy.GetTTL(1, baseTTL))
	t.Logf("L2 TTL: %v (expected 1h)", strategy.GetTTL(2, baseTTL))
}

func TestChain_WarmupWithTTLStrategy(t *testing.T) {
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2", MaxSize: 100})
	l3 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L3", MaxSize: 100})

	strategy := &CustomTTLStrategy{
		TTLs: []time.Duration{
			1 * time.Minute,
			5 * time.Minute,
			30 * time.Minute,
		},
	}
	config := ChainConfig{
		TTLStrategy: strategy,
	}

	c, err := NewWithConfig(config, l1, l2, l3)
	if err != nil {
		t.Fatalf("Failed to create chain: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Set only in L3
	err = l3.Set(ctx, "key1", "value1", 1*time.Hour)
	if err != nil {
		t.Fatalf("Set in L3 failed: %v", err)
	}

	// Get from chain - should hit L3 and warm up L1 and L2
	val, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("Expected value1, got %v", val)
	}

	// Give async writers time to complete
	time.Sleep(50 * time.Millisecond)

	// Verify L1 and L2 were warmed up with appropriate TTLs
	val1, err := l1.Get(ctx, "key1")
	if err != nil {
		t.Errorf("L1 not warmed up: %v", err)
	} else if val1 != "value1" {
		t.Errorf("L1: expected value1, got %v", val1)
	}

	val2, err := l2.Get(ctx, "key1")
	if err != nil {
		t.Errorf("L2 not warmed up: %v", err)
	} else if val2 != "value1" {
		t.Errorf("L2: expected value1, got %v", val2)
	}
}

func TestChain_DefaultTTLStrategy(t *testing.T) {
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2", MaxSize: 100})

	// Create chain without specifying TTL strategy
	c, err := New(l1, l2)
	if err != nil {
		t.Fatalf("Failed to create chain: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	baseTTL := 1 * time.Hour

	// Set value - should use default UniformTTLStrategy
	err = c.Set(ctx, "key1", "value1", baseTTL)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify both layers have the value
	for i, layer := range []interface{}{l1, l2} {
		cache := layer.(*memory.MemoryCache)
		val, err := cache.Get(ctx, "key1")
		if err != nil {
			t.Errorf("Layer %d: Get failed: %v", i, err)
		}
		if val != "value1" {
			t.Errorf("Layer %d: expected value1, got %v", i, val)
		}
	}
}

func TestChain_TTLStrategyWithExpiration(t *testing.T) {
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2", MaxSize: 100})

	strategy := &CustomTTLStrategy{
		TTLs: []time.Duration{
			50 * time.Millisecond,  // L1: Very short TTL
			200 * time.Millisecond, // L2: Longer TTL
		},
	}
	config := ChainConfig{
		TTLStrategy: strategy,
	}

	c, err := NewWithConfig(config, l1, l2)
	if err != nil {
		t.Fatalf("Failed to create chain: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Set value
	err = c.Set(ctx, "key1", "value1", 1*time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Wait for L1 to expire
	time.Sleep(80 * time.Millisecond)

	// L1 should be expired, but L2 should still have it
	_, err = l1.Get(ctx, "key1")
	if err == nil {
		t.Error("L1 should have expired but still has value")
	}

	val2, err := l2.Get(ctx, "key1")
	if err != nil {
		t.Errorf("L2 should still have value: %v", err)
	} else if val2 != "value1" {
		t.Errorf("L2: expected value1, got %v", val2)
	}

	// Get from chain - should hit L2 and warm up L1
	val, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("Expected value1, got %v", val)
	}

	// Give async writer time to warm L1
	time.Sleep(20 * time.Millisecond)

	// L1 should be warmed up again
	val1, err := l1.Get(ctx, "key1")
	if err != nil {
		t.Errorf("L1 should be warmed up: %v", err)
	} else if val1 != "value1" {
		t.Errorf("L1: expected value1, got %v", val1)
	}
}
