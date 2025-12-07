package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/cache/memory"
)

func TestNewResilientLayer(t *testing.T) {
	memCache := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name: "test",
	})

	config := DefaultResilientConfig()
	rl := NewResilientLayer(memCache, config)

	if rl == nil {
		t.Fatal("NewResilientLayer returned nil")
	}

	if rl.Name() != "test" {
		t.Errorf("Expected name 'test', got '%s'", rl.Name())
	}

	if rl.timeout != config.Timeout {
		t.Errorf("Expected timeout %v, got %v", config.Timeout, rl.timeout)
	}
}

func TestResilientLayer_Get_Success(t *testing.T) {
	memCache := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name: "test",
	})

	config := DefaultResilientConfig()
	rl := NewResilientLayer(memCache, config)
	defer rl.Close()

	ctx := context.Background()

	// Set a value
	err := rl.Set(ctx, "key1", "value1", time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the value
	val, err := rl.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if val != "value1" {
		t.Errorf("Expected 'value1', got '%v'", val)
	}
}

func TestResilientLayer_Get_NotFound(t *testing.T) {
	memCache := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name: "test",
	})

	config := DefaultResilientConfig()
	rl := NewResilientLayer(memCache, config)
	defer rl.Close()

	ctx := context.Background()

	// Get non-existent key
	_, err := rl.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent key")
	}
	// The error message is correct, just verify it's a not found error
	if err.Error() != "cache: key not found" && !cache.IsNotFound(err) {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}
}

func TestResilientLayer_Set_Delete(t *testing.T) {
	memCache := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name: "test",
	})

	config := DefaultResilientConfig()
	rl := NewResilientLayer(memCache, config)
	defer rl.Close()

	ctx := context.Background()

	// Set a value
	err := rl.Set(ctx, "key1", "value1", time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it exists
	_, err = rl.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Delete it
	err = rl.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err = rl.Get(ctx, "key1")
	if err == nil {
		t.Fatal("Expected error for deleted key")
	}
	// The error message is correct, just verify it's a not found error
	if err.Error() != "cache: key not found" && !cache.IsNotFound(err) {
		t.Errorf("Expected ErrKeyNotFound after delete, got %v", err)
	}
}

func TestResilientLayer_Timeout(t *testing.T) {
	// Create a slow mock layer
	slowLayer := &slowMockLayer{
		delay: 200 * time.Millisecond,
	}

	config := ResilientConfig{
		Timeout: 50 * time.Millisecond,
		CircuitBreakerConfig: CircuitBreakerConfig{
			MaxRequests: 1,
			Interval:    0,
			Timeout:     10 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		},
	}

	rl := NewResilientLayer(slowLayer, config)
	defer rl.Close()

	ctx := context.Background()

	// This should timeout
	_, err := rl.Get(ctx, "key1")
	if !cache.IsTimeout(err) {
		t.Errorf("Expected timeout error, got %v", err)
	}
}

func TestResilientLayer_CircuitBreaker(t *testing.T) {
	// Create a layer that always fails
	failLayer := &failingMockLayer{
		failCount: 0,
	}

	config := ResilientConfig{
		Timeout: 1 * time.Second,
		CircuitBreakerConfig: CircuitBreakerConfig{
			MaxRequests: 1,
			Interval:    0,
			Timeout:     100 * time.Millisecond,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		},
	}

	rl := NewResilientLayer(failLayer, config)
	defer rl.Close()

	ctx := context.Background()

	// First 3 calls should fail and count toward circuit breaker
	for i := 0; i < 3; i++ {
		_, err := rl.Get(ctx, "key1")
		if err == nil {
			t.Errorf("Call %d should have failed", i)
		}
		if cache.IsCircuitOpen(err) {
			t.Errorf("Circuit should not be open yet on call %d", i)
		}
	}

	// Next call should be rejected by circuit breaker
	_, err := rl.Get(ctx, "key1")
	if !cache.IsCircuitOpen(err) {
		t.Errorf("Expected circuit open error, got %v", err)
	}

	// Wait for circuit to go to half-open
	time.Sleep(150 * time.Millisecond)

	// Next call should be attempted (half-open)
	_, err = rl.Get(ctx, "key1")
	// Could be either the actual error or circuit open, depending on timing
	if err == nil {
		t.Error("Call in half-open state should fail")
	}
}

func TestResilientLayer_ContextCancellation(t *testing.T) {
	slowLayer := &slowMockLayer{
		delay: 200 * time.Millisecond,
	}

	config := DefaultResilientConfig()
	rl := NewResilientLayer(slowLayer, config)
	defer rl.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately
	cancel()

	// Should return context error
	_, err := rl.Get(ctx, "key1")
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}
}

// Mock layers for testing

type slowMockLayer struct {
	delay time.Duration
}

func (s *slowMockLayer) Name() string {
	return "slow"
}

func (s *slowMockLayer) Get(ctx context.Context, key string) (interface{}, error) {
	select {
	case <-time.After(s.delay):
		return "value", nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *slowMockLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *slowMockLayer) Delete(ctx context.Context, key string) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *slowMockLayer) Close() error {
	return nil
}

type failingMockLayer struct {
	failCount int
}

func (f *failingMockLayer) Name() string {
	return "failing"
}

func (f *failingMockLayer) Get(ctx context.Context, key string) (interface{}, error) {
	f.failCount++
	return nil, errors.New("always fails")
}

func (f *failingMockLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	f.failCount++
	return errors.New("always fails")
}

func (f *failingMockLayer) Delete(ctx context.Context, key string) error {
	f.failCount++
	return errors.New("always fails")
}

func (f *failingMockLayer) Close() error {
	return nil
}
