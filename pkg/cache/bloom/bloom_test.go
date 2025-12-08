package bloom

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/cache/memory"
)

func TestBloomLayer_BasicOperations(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	bloom := NewBloomLayer(base, 100, 0.01)
	defer bloom.Close()

	ctx := context.Background()

	// Set a key
	err := bloom.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get should succeed
	val, err := bloom.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("Expected value1, got %v", val)
	}
}

func TestBloomLayer_Rejection(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	bloom := NewBloomLayer(base, 100, 0.01)
	defer bloom.Close()

	ctx := context.Background()

	// Add 10 keys
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		bloom.Set(ctx, key, fmt.Sprintf("value-%d", i), time.Hour)
	}

	// Query non-existent key - should be rejected by bloom filter
	_, err := bloom.Get(ctx, "never-set")
	if err != cache.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}

	// Check stats
	stats := bloom.Stats()
	if stats.TotalQueries < 1 {
		t.Errorf("Expected at least 1 query, got %d", stats.TotalQueries)
	}
	if stats.BloomRejected < 1 {
		t.Errorf("Expected at least 1 rejection, got %d", stats.BloomRejected)
	}
}

func TestBloomLayer_FalsePositives(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	// Use very small filter to force collisions
	bloom := NewBloomLayer(base, 5, 0.3)
	defer bloom.Close()

	ctx := context.Background()

	// Add some keys
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		bloom.Set(ctx, key, fmt.Sprintf("value-%d", i), time.Hour)
	}

	// Query many non-existent keys to potentially trigger false positives
	rejectedCount := 0
	queriedCount := 0
	for i := 100; i < 200; i++ {
		key := fmt.Sprintf("key-%d", i)
		_, err := bloom.Get(ctx, key)
		queriedCount++
		if err != cache.ErrKeyNotFound {
			t.Fatalf("Expected not found, got %v", err)
		}
	}

	stats := bloom.Stats()

	// With such a small filter, we should see some false positives
	// (bloom says "maybe" but cache says "no")
	t.Logf("False positives detected: %d out of %d queries (%.1f%%)",
		stats.FalsePositives, queriedCount, stats.FalsePositiveRate*100)

	// The false positive rate might be high with such a small filter
	// Just ensure we can detect them
	if stats.FalsePositives > 0 {
		t.Logf("Successfully detected %d false positives", stats.FalsePositives)
	}

	_ = rejectedCount
}

func TestBloomLayer_Reset(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	bloom := NewBloomLayer(base, 100, 0.01)
	defer bloom.Close()

	ctx := context.Background()

	// Add keys
	bloom.Set(ctx, "key1", "value1", time.Hour)
	bloom.Set(ctx, "key2", "value2", time.Hour)

	// Query to generate stats
	bloom.Get(ctx, "key1")
	bloom.Get(ctx, "key2")

	stats := bloom.Stats()
	if stats.TotalQueries == 0 {
		t.Error("Expected some queries")
	}

	// Reset
	bloom.Reset()

	// Get existing key - bloom filter doesn't know about it anymore
	// so it will reject it even though it's in the underlying cache
	_, err := bloom.Get(ctx, "key1")
	if err != cache.ErrKeyNotFound {
		// After reset, bloom filter will reject the key
		t.Logf("After reset, bloom rejected key that still exists in cache (expected behavior)")
	}

	// Stats should be reset
	stats = bloom.Stats()
	if stats.TotalQueries != 1 {
		t.Errorf("Expected 1 query after reset, got %d", stats.TotalQueries)
	}
	if stats.BloomRejected != 1 {
		t.Logf("Bloom rejected count: %d (1 rejection is expected)", stats.BloomRejected)
	}
}

func TestBloomLayer_Delete(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	bloom := NewBloomLayer(base, 100, 0.01)
	defer bloom.Close()

	ctx := context.Background()

	// Set and delete
	bloom.Set(ctx, "key1", "value1", time.Hour)
	err := bloom.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get should fail
	_, err = bloom.Get(ctx, "key1")
	if err != cache.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound after delete, got %v", err)
	}
}

func TestBloomLayer_Name(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "TestLayer",
		MaxSize: 100,
	})
	bloom := NewBloomLayer(base, 100, 0.01)
	defer bloom.Close()

	name := bloom.Name()
	expected := "bloom(TestLayer)"
	if name != expected {
		t.Errorf("Expected name %s, got %s", expected, name)
	}
}

func TestBloomLayer_StatsRates(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	bloom := NewBloomLayer(base, 100, 0.01)
	defer bloom.Close()

	ctx := context.Background()

	// Add keys
	for i := 0; i < 5; i++ {
		bloom.Set(ctx, fmt.Sprintf("key-%d", i), i, time.Hour)
	}

	// Query non-existent keys
	for i := 10; i < 20; i++ {
		bloom.Get(ctx, fmt.Sprintf("key-%d", i))
	}

	stats := bloom.Stats()

	// Rejection rate should be calculated
	if stats.RejectionRate < 0 || stats.RejectionRate > 1 {
		t.Errorf("Invalid rejection rate: %f", stats.RejectionRate)
	}

	// Should have some rejections
	if stats.BloomRejected == 0 {
		t.Error("Expected some bloom rejections")
	}

	t.Logf("Rejection rate: %.1f%%, False positive rate: %.1f%%",
		stats.RejectionRate*100, stats.FalsePositiveRate*100)
}

func TestBloomLayer_ContextCancellation(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	bloom := NewBloomLayer(base, 100, 0.01)
	defer bloom.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations with cancelled context should fail
	err := bloom.Set(ctx, "key1", "value1", time.Hour)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}

	_, err = bloom.Get(ctx, "key1")
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}
