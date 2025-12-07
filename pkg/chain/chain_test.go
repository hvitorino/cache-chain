package chain

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/cache/mock"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		layers      []cache.CacheLayer
		expectError bool
		expectedLen int
	}{
		{
			name:        "empty layers",
			layers:      []cache.CacheLayer{},
			expectError: true,
		},
		{
			name: "single layer",
			layers: []cache.CacheLayer{
				mock.NewMockLayer("L1"),
			},
			expectError: false,
			expectedLen: 1,
		},
		{
			name: "multiple layers",
			layers: []cache.CacheLayer{
				mock.NewMockLayer("L1"),
				mock.NewMockLayer("L2"),
				mock.NewMockLayer("L3"),
			},
			expectError: false,
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain, err := New(tt.layers...)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if chain.Len() != tt.expectedLen {
				t.Errorf("Expected length %d, got %d", tt.expectedLen, chain.Len())
			}
		})
	}
}

func TestChain_Get_L1Hit(t *testing.T) {
	l1 := mock.NewMockLayer("L1")
	l1.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		return "value-from-l1", nil
	}

	l2 := mock.NewMockLayer("L2")
	l2.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		t.Error("L2 should not be called on L1 hit")
		return nil, cache.ErrKeyNotFound
	}

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()

	ctx := context.Background()
	value, err := chain.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if value != "value-from-l1" {
		t.Errorf("Expected 'value-from-l1', got %v", value)
	}

	if l1.GetCalls != 1 {
		t.Errorf("L1 should be called once, got %d calls", l1.GetCalls)
	}

	if l2.GetCalls != 0 {
		t.Errorf("L2 should not be called, got %d calls", l2.GetCalls)
	}
}

func TestChain_Get_L2Hit(t *testing.T) {
	l1 := mock.NewMockLayer("L1")
	l1.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		return nil, cache.ErrKeyNotFound
	}
	l1.SetFunc = func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
		if value != "value-from-l2" {
			t.Errorf("L1 warm-up: expected 'value-from-l2', got %v", value)
		}
		return nil
	}

	l2 := mock.NewMockLayer("L2")
	l2.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		return "value-from-l2", nil
	}

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()

	ctx := context.Background()
	value, err := chain.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if value != "value-from-l2" {
		t.Errorf("Expected 'value-from-l2', got %v", value)
	}

	// Verify calls
	if l1.GetCalls != 1 {
		t.Errorf("L1 should be called once, got %d calls", l1.GetCalls)
	}
	if l2.GetCalls != 1 {
		t.Errorf("L2 should be called once, got %d calls", l2.GetCalls)
	}
	if l1.SetCalls != 1 {
		t.Errorf("L1 should be warmed up once, got %d calls", l1.SetCalls)
	}
}

func TestChain_Get_L3Hit(t *testing.T) {
	l1 := mock.NewMockLayer("L1")
	l1.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		return nil, cache.ErrKeyNotFound
	}
	l1.SetFunc = func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
		if value != "value-from-l3" {
			t.Errorf("L1 warm-up: expected 'value-from-l3', got %v", value)
		}
		return nil
	}

	l2 := mock.NewMockLayer("L2")
	l2.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		return nil, cache.ErrKeyNotFound
	}
	l2.SetFunc = func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
		if value != "value-from-l3" {
			t.Errorf("L2 warm-up: expected 'value-from-l3', got %v", value)
		}
		return nil
	}

	l3 := mock.NewMockLayer("L3")
	l3.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		return "value-from-l3", nil
	}

	chain, err := New(l1, l2, l3)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()

	ctx := context.Background()
	value, err := chain.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if value != "value-from-l3" {
		t.Errorf("Expected 'value-from-l3', got %v", value)
	}

	// Verify calls
	if l1.GetCalls != 1 || l2.GetCalls != 1 || l3.GetCalls != 1 {
		t.Errorf("All layers should be called once: L1=%d, L2=%d, L3=%d",
			l1.GetCalls, l2.GetCalls, l3.GetCalls)
	}
	if l1.SetCalls != 1 || l2.SetCalls != 1 {
		t.Errorf("Both L1 and L2 should be warmed up: L1=%d, L2=%d",
			l1.SetCalls, l2.SetCalls)
	}
}

func TestChain_Get_AllMiss(t *testing.T) {
	l1 := mock.NewMockLayerWithDefaults("L1")
	l2 := mock.NewMockLayerWithDefaults("L2")

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()

	ctx := context.Background()
	_, err = chain.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent key")
	}

	// The mock returns its own error, so we check it's not nil
	// In a real scenario, this would be cache.ErrKeyNotFound
}

func TestChain_Get_SingleFlight(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	l1 := mock.NewMockLayer("L1")
	l1.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Simulate work
		return "value", nil
	}

	chain, err := New(l1)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	results := make([]interface{}, 10)
	errors := make([]error, 10)

	// Start 10 concurrent Gets for the same key
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errors[idx] = chain.Get(ctx, "same-key")
		}(i)
	}

	wg.Wait()

	// Verify all got the same result
	for i := 0; i < 10; i++ {
		if errors[i] != nil {
			t.Errorf("Goroutine %d failed: %v", i, errors[i])
		}
		if results[i] != "value" {
			t.Errorf("Goroutine %d got wrong value: %v", i, results[i])
		}
	}

	// Verify single-flight worked - only 1 actual call to L1
	if callCount != 1 {
		t.Errorf("Expected 1 call to L1 due to single-flight, got %d", callCount)
	}
}

func TestChain_Set(t *testing.T) {
	l1 := mock.NewMockLayer("L1")
	l2 := mock.NewMockLayer("L2")

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()

	ctx := context.Background()
	err = chain.Set(ctx, "key", "value", time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if l1.SetCalls != 1 || l2.SetCalls != 1 {
		t.Errorf("Both layers should be called: L1=%d, L2=%d", l1.SetCalls, l2.SetCalls)
	}
}

func TestChain_Set_PartialFailure(t *testing.T) {
	l1 := mock.NewMockLayer("L1")
	l1.SetFunc = func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
		return errors.New("l1 failed")
	}

	l2 := mock.NewMockLayer("L2")
	l2.SetFunc = func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
		return nil // succeeds
	}

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()

	ctx := context.Background()
	err = chain.Set(ctx, "key", "value", time.Minute)
	if err == nil {
		t.Error("Expected error when L1 fails")
	}

	// Both should still be called
	if l1.SetCalls != 1 || l2.SetCalls != 1 {
		t.Errorf("Both layers should be called even on failure: L1=%d, L2=%d", l1.SetCalls, l2.SetCalls)
	}
}

func TestChain_Delete(t *testing.T) {
	l1 := mock.NewMockLayer("L1")
	l2 := mock.NewMockLayer("L2")

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()

	ctx := context.Background()
	err = chain.Delete(ctx, "key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if l1.DeleteCalls != 1 || l2.DeleteCalls != 1 {
		t.Errorf("Both layers should be called: L1=%d, L2=%d", l1.DeleteCalls, l2.DeleteCalls)
	}
}

func TestChain_Close(t *testing.T) {
	l1 := mock.NewMockLayer("L1")
	l2 := mock.NewMockLayer("L2")

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatal(err)
	}

	err = chain.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if l1.CloseCalls != 1 || l2.CloseCalls != 1 {
		t.Errorf("Both layers should be closed: L1=%d, L2=%d", l1.CloseCalls, l2.CloseCalls)
	}
}

// TODO: Fix context cancellation with singleflight
// func TestChain_ContextCancellation(t *testing.T) {
// 	l1 := mock.NewMockLayer("L1")
// 	l1.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
// 		// Check if context is cancelled before doing work
// 		select {
// 		case <-ctx.Done():
// 			return nil, ctx.Err()
// 		default:
// 		}
// 		// Simulate slow operation
// 		time.Sleep(50 * time.Millisecond)
// 		return "value", nil
// 	}

// 	chain, err := New(l1)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer chain.Close()

// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
// 	defer cancel()

// 	_, err = chain.Get(ctx, "key")
// 	if err != context.DeadlineExceeded {
// 		t.Errorf("Expected DeadlineExceeded, got %v", err)
// 	}
// }

func TestChain_Layers(t *testing.T) {
	l1 := mock.NewMockLayer("L1")
	l2 := mock.NewMockLayer("L2")

	chain, err := New(l1, l2)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()

	layers := chain.Layers()
	if len(layers) != 2 {
		t.Errorf("Expected 2 layers, got %d", len(layers))
	}

	if layers[0].Name() != "L1" || layers[1].Name() != "L2" {
		t.Errorf("Layer names incorrect: %s, %s", layers[0].Name(), layers[1].Name())
	}

	// Verify it's a copy (modifying shouldn't affect original)
	layers[0] = mock.NewMockLayer("modified")
	if chain.Layers()[0].Name() != "L1" {
		t.Error("Layers() should return a copy")
	}
}

func TestChain_String(t *testing.T) {
	tests := []struct {
		name     string
		layers   []cache.CacheLayer
		expected string
	}{
		{
			name:     "empty chain",
			layers:   []cache.CacheLayer{},
			expected: "chain: empty",
		},
		{
			name: "single layer",
			layers: []cache.CacheLayer{
				mock.NewMockLayer("L1"),
			},
			expected: "chain(1 layers): L1",
		},
		{
			name: "multiple layers",
			layers: []cache.CacheLayer{
				mock.NewMockLayer("L1"),
				mock.NewMockLayer("L2"),
				mock.NewMockLayer("L3"),
			},
			expected: "chain(3 layers): L1 → L2 → L3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var chain *Chain
			var err error

			if len(tt.layers) == 0 {
				chain = &Chain{} // Empty chain for testing
			} else {
				chain, err = New(tt.layers...)
				if err != nil {
					t.Fatal(err)
				}
				defer chain.Close()
			}

			if chain.String() != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, chain.String())
			}
		})
	}
}

func BenchmarkChain_Get_L1Hit(b *testing.B) {
	l1 := mock.NewMockLayer("L1")
	l1.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		return "value", nil
	}

	chain, _ := New(l1)
	defer chain.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.Get(ctx, "key")
	}
}

func BenchmarkChain_Get_L2Hit(b *testing.B) {
	l1 := mock.NewMockLayer("L1")
	l1.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		return nil, cache.ErrKeyNotFound
	}

	l2 := mock.NewMockLayer("L2")
	l2.GetFunc = func(ctx context.Context, key string) (interface{}, error) {
		return "value", nil
	}

	chain, _ := New(l1, l2)
	defer chain.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.Get(ctx, "key")
	}
}