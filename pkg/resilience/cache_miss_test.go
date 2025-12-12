package resilience

import (
	"context"
	"testing"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/cache/memory"
)

// TestResilientLayer_CacheMissDoesNotTripCircuit verifies that cache misses
// (ErrKeyNotFound) do NOT count as failures for the circuit breaker.
// This is critical because cache misses are normal operations, not errors.
func TestResilientLayer_CacheMissDoesNotTripCircuit(t *testing.T) {
	// Create a memory cache
	memCache := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test-cache-miss",
		MaxSize: 100,
	})

	// Create resilient layer with aggressive circuit breaker settings
	config := ResilientConfig{
		Timeout: 1 * time.Second,
		CircuitBreakerConfig: CircuitBreakerConfig{
			MaxRequests: 1,
			Interval:    60 * time.Second,
			Timeout:     10 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				// Trip immediately after just 3 failures
				// This would have been triggered by 3 cache misses before the fix
				return counts.TotalFailures >= 3
			},
		},
	}

	resilientLayer := NewResilientLayer(memCache, config)
	ctx := context.Background()

	// Perform 100 cache misses - these should NOT trip the circuit breaker
	for i := 0; i < 100; i++ {
		_, err := resilientLayer.Get(ctx, "nonexistent-key")

		// Should get cache miss error
		if !cache.IsNotFound(err) {
			t.Errorf("Expected ErrKeyNotFound, got: %v", err)
		}

		// Should NOT get circuit breaker open error
		if cache.IsCircuitOpen(err) {
			t.Fatalf("Circuit breaker opened after %d cache misses - cache misses should not count as failures!", i+1)
		}
	}

	t.Log("✓ Circuit breaker did NOT open after 100 cache misses")

	// Verify we can still perform successful operations
	err := resilientLayer.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Fatalf("Set failed after cache misses: %v", err)
	}

	value, err := resilientLayer.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed after cache misses: %v", err)
	}
	if value.(string) != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	t.Log("✓ Cache operations still work after many cache misses")
}

// TestResilientLayer_RealErrorsStillTripCircuit verifies that real errors
// (like layer unavailable) DO count as failures and trip the circuit breaker.
func TestResilientLayer_RealErrorsStillTripCircuit(t *testing.T) {
	// Create a failing layer
	failingLayer := &alwaysFailingLayer{name: "always-failing"}

	// Create resilient layer with aggressive circuit breaker
	config := ResilientConfig{
		Timeout: 100 * time.Millisecond,
		CircuitBreakerConfig: CircuitBreakerConfig{
			MaxRequests: 1,
			Interval:    60 * time.Second,
			Timeout:     10 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				// Trip after 5 real failures
				return counts.TotalFailures >= 5
			},
		},
	}

	resilientLayer := NewResilientLayer(failingLayer, config)
	ctx := context.Background()

	// Perform operations that cause real errors
	for i := 0; i < 10; i++ {
		_, err := resilientLayer.Get(ctx, "key1")

		if i < 5 {
			// First 5 should fail with layer unavailable
			if !cache.IsUnavailable(err) {
				t.Errorf("Expected ErrLayerUnavailable, got: %v", err)
			}
		} else {
			// After 5 failures, circuit should be open
			if !cache.IsCircuitOpen(err) {
				t.Errorf("Expected circuit to be open after %d failures, got: %v", i+1, err)
			}
		}
	}

	t.Log("✓ Circuit breaker opened after real errors (as expected)")
}

// alwaysFailingLayer is a test layer that always returns errors.
type alwaysFailingLayer struct {
	name string
}

func (l *alwaysFailingLayer) Name() string {
	return l.name
}

func (l *alwaysFailingLayer) Get(ctx context.Context, key string) (interface{}, error) {
	return nil, cache.ErrLayerUnavailable
}

func (l *alwaysFailingLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return cache.ErrLayerUnavailable
}

func (l *alwaysFailingLayer) Delete(ctx context.Context, key string) error {
	return cache.ErrLayerUnavailable
}

func (l *alwaysFailingLayer) Close() error {
	return nil
}
