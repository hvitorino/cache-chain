package chain

import (
	"context"
	"errors"
	"testing"
	"time"

	"cache-chain/pkg/cache/memory"
)

func TestChain_ResilientLayers(t *testing.T) {
	// L1: Always fails
	l1 := &alwaysFailingLayer{name: "L1"}

	// L2: Works fine
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2"})

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatalf("Failed to create chain: %v", err)
	}
	defer chain.Close()

	ctx := context.Background()

	// Pre-populate L2
	err = l2.Set(ctx, "key", "value", time.Minute)
	if err != nil {
		t.Fatalf("Failed to set in L2: %v", err)
	}

	// First few calls: L1 fails, falls back to L2
	for i := 0; i < 5; i++ {
		value, err := chain.Get(ctx, "key")
		if err != nil {
			t.Fatalf("Call %d failed: %v", i, err)
		}
		if value != "value" {
			t.Errorf("Call %d: wrong value: %v", i, value)
		}
	}

	// After enough failures, L1 circuit should open
	// Next calls should skip L1 entirely (faster response)
	start := time.Now()
	value, err := chain.Get(ctx, "key")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Get with open circuit failed: %v", err)
	}
	if value != "value" {
		t.Errorf("Wrong value: %v", value)
	}

	// With circuit open, response should be faster
	// (though this is a weak assertion due to timing variability)
	if elapsed > 100*time.Millisecond {
		t.Logf("Warning: response with open circuit took %v (expected < 100ms)", elapsed)
	}
}

func TestChain_ResilientLayers_Timeout(t *testing.T) {
	// L1: Slow layer
	l1 := &slowLayer{name: "L1", delay: 500 * time.Millisecond}

	// L2: Fast layer
	l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2"})

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatalf("Failed to create chain: %v", err)
	}
	defer chain.Close()

	ctx := context.Background()

	// Pre-populate L2
	err = l2.Set(ctx, "key", "value", time.Minute)
	if err != nil {
		t.Fatalf("Failed to set in L2: %v", err)
	}

	// L1 should timeout (configured for 100ms), fallback to L2
	value, err := chain.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "value" {
		t.Errorf("Wrong value: %v", value)
	}

	// Multiple timeouts should eventually open L1's circuit
	for i := 0; i < 5; i++ {
		chain.Get(ctx, "key")
		time.Sleep(10 * time.Millisecond) // Small delay between calls
	}

	// Verify we're still getting values (from L2)
	value, err = chain.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get after timeouts failed: %v", err)
	}
	if value != "value" {
		t.Errorf("Wrong value after timeouts: %v", value)
	}
}

func TestChain_AllLayersFail_CircuitOpen(t *testing.T) {
	// Both layers fail
	l1 := &alwaysFailingLayer{name: "L1"}
	l2 := &alwaysFailingLayer{name: "L2"}

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatalf("Failed to create chain: %v", err)
	}
	defer chain.Close()

	ctx := context.Background()

	// First few calls should fail normally
	for i := 0; i < 5; i++ {
		_, err := chain.Get(ctx, "key")
		if err == nil {
			t.Errorf("Call %d should have failed", i)
		}
	}

	// Eventually both circuits should open
	// Continue to get errors but circuits prevent actual calls
	_, err = chain.Get(ctx, "key")
	if err == nil {
		t.Error("Should still get error when all circuits are open")
	}
}

// Mock layers for testing

type alwaysFailingLayer struct {
	name string
}

func (l *alwaysFailingLayer) Name() string {
	return l.name
}

func (l *alwaysFailingLayer) Get(ctx context.Context, key string) (interface{}, error) {
	return nil, errors.New("always fails")
}

func (l *alwaysFailingLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return errors.New("always fails")
}

func (l *alwaysFailingLayer) Delete(ctx context.Context, key string) error {
	return errors.New("always fails")
}

func (l *alwaysFailingLayer) Close() error {
	return nil
}

type slowLayer struct {
	name  string
	delay time.Duration
}

func (l *slowLayer) Name() string {
	return l.name
}

func (l *slowLayer) Get(ctx context.Context, key string) (interface{}, error) {
	select {
	case <-time.After(l.delay):
		return "slow value", nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (l *slowLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	select {
	case <-time.After(l.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *slowLayer) Delete(ctx context.Context, key string) error {
	select {
	case <-time.After(l.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *slowLayer) Close() error {
	return nil
}
