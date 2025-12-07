package chain

import (
	"context"
	"testing"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/cache/memory"
	metricsMemory "cache-chain/pkg/metrics/memory"
	"cache-chain/pkg/resilience"
)

// TestChain_WithMetrics tests that metrics are collected during cache operations.
func TestChain_WithMetrics(t *testing.T) {
	// Create memory collector
	mc := metricsMemory.NewMemoryCollector()

	// Create layers
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2", MaxSize: 200})

	// Create chain with metrics
	chain, err := NewWithConfig(ChainConfig{
		Metrics: mc,
	}, l1, l2)
	if err != nil {
		t.Fatalf("NewWithConfig failed: %v", err)
	}
	defer chain.Close()

	ctx := context.Background()

	// Test 1: Get miss on all layers (should record 2 layer misses + 1 chain miss)
	_, err = chain.Get(ctx, "key1")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !cache.IsNotFound(err) {
		t.Fatalf("Expected key not found error, got: %v (IsNotFound=%v)", err, cache.IsNotFound(err))
	}

	// Small delay for metrics to propagate
	time.Sleep(50 * time.Millisecond)

	snapshot := mc.Snapshot()

	// Check chain metrics - should have 1 miss
	if snapshot.ChainHits != 0 {
		t.Errorf("Expected 0 chain hits, got %d", snapshot.ChainHits)
	}
	if snapshot.ChainMisses != 1 {
		t.Errorf("Expected 1 chain miss, got %d", snapshot.ChainMisses)
	}

	// Test 2: Set a value in all layers
	err = chain.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Test 3: Get hit on L1 (should record 1 layer hit + 1 chain hit)
	value, err := chain.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value.(string) != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	time.Sleep(50 * time.Millisecond)

	snapshot = mc.Snapshot()

	// Check chain metrics - should have 1 hit now
	if snapshot.ChainHits != 1 {
		t.Errorf("Expected 1 chain hit, got %d", snapshot.ChainHits)
	}

	// Test 4: Delete from all layers
	err = chain.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
}

// TestChain_MetricsWithWarmUp tests metrics collection during layer warm-up.
func TestChain_MetricsWithWarmUp(t *testing.T) {
	// Create memory collector
	mc := metricsMemory.NewMemoryCollector()

	// Create layers
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2", MaxSize: 200})

	// Create chain with metrics
	chain, err := NewWithConfig(ChainConfig{
		Metrics: mc,
	}, l1, l2)
	if err != nil {
		t.Fatalf("NewWithConfig failed: %v", err)
	}
	defer chain.Close()

	ctx := context.Background()

	// Set value only in L2
	err = l2.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Fatalf("L2 Set failed: %v", err)
	}

	// Get should hit L2 and warm up L1
	value, err := chain.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value.(string) != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	// Wait for warm-up to complete
	time.Sleep(100 * time.Millisecond)

	snapshot := mc.Snapshot()

	// Should have recorded:
	// - Chain get (1 hit at layer 1 = L2)
	// - Async write to L1
	if snapshot.ChainHits != 1 {
		t.Errorf("Expected 1 chain hit, got %d", snapshot.ChainHits)
	}

	// Check that L1 was warmed up
	value, err = l1.Get(ctx, "key1")
	if err != nil {
		t.Errorf("L1 should have been warmed up, got error: %v", err)
	}
	if value.(string) != "value1" {
		t.Errorf("L1 should have value1, got %v", value)
	}
}

// TestChain_NoOpMetrics tests that NoOpCollector doesn't cause issues.
func TestChain_NoOpMetrics(t *testing.T) {
	// Create chain with default (NoOp) metrics
	l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 100})

	chain, err := New(l1)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer chain.Close()

	ctx := context.Background()

	// All operations should work normally
	err = chain.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	_, err = chain.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	err = chain.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

// TestResilientLayer_CircuitBreakerMetrics tests that circuit breaker state changes are recorded.
func TestResilientLayer_CircuitBreakerMetrics(t *testing.T) {
	// Create memory collector
	mc := metricsMemory.NewMemoryCollector()

	// Create a flaky layer that fails
	flaky := &flakyLayer{
		failCount: 10, // Fail 10 times to trip circuit
	}

	// Create chain with custom resilient config
	chain, err := NewWithConfig(ChainConfig{
		Metrics: mc,
		ResilientConfigs: []resilience.ResilientConfig{
			resilience.DefaultResilientConfig().WithTimeout(50 * time.Millisecond),
		},
	}, flaky)
	if err != nil {
		t.Fatalf("NewWithConfig failed: %v", err)
	}
	defer chain.Close()

	ctx := context.Background()

	// Make multiple failing requests to trip circuit breaker
	for i := 0; i < 10; i++ {
		_, _ = chain.Get(ctx, "key1")
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for circuit to open
	time.Sleep(100 * time.Millisecond)

	snapshot := mc.Snapshot()

	// Should have recorded layer metrics
	if len(snapshot.LayerMetrics) == 0 {
		t.Error("Expected layer metrics to be recorded")
	}

	// Circuit should now be open, next request should fail fast
	_, err = chain.Get(ctx, "key1")
	if !cache.IsCircuitOpen(err) {
		t.Errorf("Expected circuit open error, got: %v", err)
	}
}

// flakyLayer is a test layer that fails a specified number of times.
type flakyLayer struct {
	failCount int
	attempts  int
}

func (f *flakyLayer) Name() string {
	return "flaky"
}

func (f *flakyLayer) Get(ctx context.Context, key string) (interface{}, error) {
	f.attempts++
	if f.attempts <= f.failCount {
		return nil, cache.ErrLayerUnavailable
	}
	return "value", nil
}

func (f *flakyLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}

func (f *flakyLayer) Delete(ctx context.Context, key string) error {
	return nil
}

func (f *flakyLayer) Close() error {
	return nil
}
