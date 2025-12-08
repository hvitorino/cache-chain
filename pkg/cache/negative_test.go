package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewNegativeCacheLayer(t *testing.T) {
	mock := &mockLayer{name: "test"}
	ncl := NewNegativeCacheLayer(mock, time.Second)
	defer ncl.Close()

	if ncl == nil {
		t.Fatal("NewNegativeCacheLayer returned nil")
	}

	if ncl.negativeTTL != time.Second {
		t.Errorf("Expected negativeTTL=1s, got %v", ncl.negativeTTL)
	}
}

func TestNewNegativeCacheLayer_DefaultTTL(t *testing.T) {
	mock := &mockLayer{name: "test"}
	ncl := NewNegativeCacheLayer(mock, 0) // Zero should use default
	defer ncl.Close()

	if ncl.negativeTTL != time.Minute {
		t.Errorf("Expected default negativeTTL=1m, got %v", ncl.negativeTTL)
	}
}

func TestNegativeCacheLayer_Get_CachesNotFound(t *testing.T) {
	callCount := 0
	mock := &mockLayer{
		name: "test",
		getFunc: func(ctx context.Context, key string) (interface{}, error) {
			callCount++
			return nil, ErrKeyNotFound
		},
	}

	ncl := NewNegativeCacheLayer(mock, time.Second)
	defer ncl.Close()

	ctx := context.Background()

	// First get: miss, should call underlying layer
	_, err := ncl.Get(ctx, "missing-key")
	if !IsNotFound(err) {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call to underlying layer, got %d", callCount)
	}

	// Second get: should use negative cache, NOT call underlying layer
	_, err = ncl.Get(ctx, "missing-key")
	if !IsNotFound(err) {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected still 1 call (cached), got %d", callCount)
	}

	// Third get: also from negative cache
	_, err = ncl.Get(ctx, "missing-key")
	if !IsNotFound(err) {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected still 1 call (cached), got %d", callCount)
	}
}

func TestNegativeCacheLayer_Get_Expiration(t *testing.T) {
	callCount := 0
	mock := &mockLayer{
		name: "test",
		getFunc: func(ctx context.Context, key string) (interface{}, error) {
			callCount++
			return nil, ErrKeyNotFound
		},
	}

	ncl := NewNegativeCacheLayer(mock, 50*time.Millisecond)
	defer ncl.Close()

	ctx := context.Background()

	// First get: cache miss
	_, _ = ncl.Get(ctx, "missing-key")
	if callCount != 1 {
		t.Fatalf("Expected 1 call, got %d", callCount)
	}

	// Second get: from negative cache
	_, _ = ncl.Get(ctx, "missing-key")
	if callCount != 1 {
		t.Fatalf("Expected still 1 call, got %d", callCount)
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Third get: negative cache expired, should call underlying layer again
	_, _ = ncl.Get(ctx, "missing-key")
	if callCount != 2 {
		t.Errorf("Expected 2 calls after expiration, got %d", callCount)
	}
}

func TestNegativeCacheLayer_Set_RemovesNegative(t *testing.T) {
	callCount := 0
	mock := &mockLayer{
		name: "test",
		getFunc: func(ctx context.Context, key string) (interface{}, error) {
			callCount++
			if callCount == 1 {
				return nil, ErrKeyNotFound
			}
			return "value", nil
		},
		setFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			return nil
		},
	}

	ncl := NewNegativeCacheLayer(mock, time.Minute)
	defer ncl.Close()

	ctx := context.Background()

	// Get missing key: cache negative
	_, err := ncl.Get(ctx, "key1")
	if !IsNotFound(err) {
		t.Fatalf("Expected not found, got %v", err)
	}

	// Set the key: should remove from negative cache
	err = ncl.Set(ctx, "key1", "value", time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get again: should call underlying layer (negative cache cleared)
	value, err := ncl.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get after Set failed: %v", err)
	}
	if value != "value" {
		t.Errorf("Expected 'value', got %v", value)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls to Get, got %d", callCount)
	}
}

func TestNegativeCacheLayer_Delete_CachesNegative(t *testing.T) {
	mock := &mockLayer{
		name: "test",
		deleteFunc: func(ctx context.Context, key string) error {
			return nil
		},
		getFunc: func(ctx context.Context, key string) (interface{}, error) {
			return nil, ErrKeyNotFound
		},
	}

	ncl := NewNegativeCacheLayer(mock, time.Minute)
	defer ncl.Close()

	ctx := context.Background()

	// Delete a key
	err := ncl.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Check if it's in negative cache
	if !ncl.isNegativeCached("key1") {
		t.Error("Key should be in negative cache after delete")
	}
}

func TestNegativeCacheLayer_OtherErrors(t *testing.T) {
	callCount := 0
	testErr := errors.New("some other error")
	mock := &mockLayer{
		name: "test",
		getFunc: func(ctx context.Context, key string) (interface{}, error) {
			callCount++
			return nil, testErr
		},
	}

	ncl := NewNegativeCacheLayer(mock, time.Second)
	defer ncl.Close()

	ctx := context.Background()

	// Get with non-not-found error: should NOT cache negative
	_, err := ncl.Get(ctx, "key1")
	if err != testErr {
		t.Errorf("Expected testErr, got %v", err)
	}

	// Second get: should call underlying layer again (not cached)
	_, err = ncl.Get(ctx, "key1")
	if err != testErr {
		t.Errorf("Expected testErr, got %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls (not cached), got %d", callCount)
	}
}

func TestNegativeCacheLayer_Stats(t *testing.T) {
	mock := &mockLayer{
		name: "test",
		getFunc: func(ctx context.Context, key string) (interface{}, error) {
			return nil, ErrKeyNotFound
		},
	}

	ncl := NewNegativeCacheLayer(mock, time.Minute)
	defer ncl.Close()

	ctx := context.Background()

	stats := ncl.Stats()
	if stats.NegativeCount != 0 {
		t.Errorf("Expected 0 negative entries, got %d", stats.NegativeCount)
	}

	// Add some negative entries
	ncl.Get(ctx, "key1")
	ncl.Get(ctx, "key2")
	ncl.Get(ctx, "key3")

	stats = ncl.Stats()
	if stats.NegativeCount != 3 {
		t.Errorf("Expected 3 negative entries, got %d", stats.NegativeCount)
	}

	if stats.NegativeTTL != time.Minute {
		t.Errorf("Expected 1m TTL, got %v", stats.NegativeTTL)
	}
}

func TestNegativeCacheLayer_Cleanup(t *testing.T) {
	mock := &mockLayer{
		name: "test",
		getFunc: func(ctx context.Context, key string) (interface{}, error) {
			return nil, ErrKeyNotFound
		},
	}

	ncl := NewNegativeCacheLayer(mock, 50*time.Millisecond)
	defer ncl.Close()

	ctx := context.Background()

	// Add negative entries
	ncl.Get(ctx, "key1")
	ncl.Get(ctx, "key2")

	stats := ncl.Stats()
	if stats.NegativeCount != 2 {
		t.Fatalf("Expected 2 entries, got %d", stats.NegativeCount)
	}

	// Wait for cleanup to run (cleanup runs every negativeTTL/2)
	time.Sleep(100 * time.Millisecond)

	stats = ncl.Stats()
	if stats.NegativeCount != 0 {
		t.Errorf("Expected 0 entries after cleanup, got %d", stats.NegativeCount)
	}
}

func TestNegativeCacheLayer_Name(t *testing.T) {
	mock := &mockLayer{name: "TestLayer"}
	ncl := NewNegativeCacheLayer(mock, time.Minute)
	defer ncl.Close()

	expected := "TestLayer-negative"
	if ncl.Name() != expected {
		t.Errorf("Expected name %q, got %q", expected, ncl.Name())
	}
}

// mockLayer is a simple mock for testing
type mockLayer struct {
	name       string
	getFunc    func(context.Context, string) (interface{}, error)
	setFunc    func(context.Context, string, interface{}, time.Duration) error
	deleteFunc func(context.Context, string) error
}

func (m *mockLayer) Name() string { return m.name }

func (m *mockLayer) Get(ctx context.Context, key string) (interface{}, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, key)
	}
	return nil, ErrKeyNotFound
}

func (m *mockLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.setFunc != nil {
		return m.setFunc(ctx, key, value, ttl)
	}
	return nil
}

func (m *mockLayer) Delete(ctx context.Context, key string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, key)
	}
	return nil
}

func (m *mockLayer) Close() error { return nil }
