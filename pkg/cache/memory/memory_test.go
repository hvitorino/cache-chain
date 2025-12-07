package memory

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMemoryCache_Get(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "test",
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	ctx := context.Background()

	// Test Get non-existent key
	_, err := cache.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent key")
	}

	// Test Set and Get
	err = cache.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	value, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "value1" {
		t.Errorf("Expected 'value1', got %v", value)
	}
}

func TestMemoryCache_Set(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "test",
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	ctx := context.Background()

	// Test Set with custom TTL
	err := cache.Set(ctx, "key1", "value1", time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it's stored
	value, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "value1" {
		t.Errorf("Expected 'value1', got %v", value)
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "test",
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	ctx := context.Background()

	// Set a value
	err := cache.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Delete it
	err = cache.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err = cache.Get(ctx, "key1")
	if err == nil {
		t.Error("Expected error after delete")
	}
}

func TestMemoryCache_TTL(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "test",
		DefaultTTL:      time.Hour,
		CleanupInterval: 10 * time.Millisecond, // Fast cleanup for testing
	})
	defer cache.Close()

	ctx := context.Background()

	// Set with short TTL
	err := cache.Set(ctx, "key1", "value1", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should be available immediately
	_, err = cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed before expiration: %v", err)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, err = cache.Get(ctx, "key1")
	if err == nil {
		t.Error("Expected error after expiration")
	}
}

func TestMemoryCache_LRU(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "test",
		MaxSize:         2,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	ctx := context.Background()

	// Fill to capacity
	err := cache.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("Set key1 failed: %v", err)
	}
	err = cache.Set(ctx, "key2", "value2", 0)
	if err != nil {
		t.Fatalf("Set key2 failed: %v", err)
	}

	// Access key1 to make key2 LRU
	_, err = cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get key1 failed: %v", err)
	}

	// Add third key, should evict key2
	err = cache.Set(ctx, "key3", "value3", 0)
	if err != nil {
		t.Fatalf("Set key3 failed: %v", err)
	}

	// key1 should still be there
	_, err = cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("key1 should not be evicted: %v", err)
	}

	// key2 should be evicted
	_, err = cache.Get(ctx, "key2")
	if err == nil {
		t.Error("key2 should have been evicted")
	}

	// key3 should be there
	_, err = cache.Get(ctx, "key3")
	if err != nil {
		t.Fatalf("key3 should be present: %v", err)
	}
}

func TestMemoryCache_Concurrency(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "test",
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Start multiple goroutines doing operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			key := "key" + string(rune(id+'0'))
			value := "value" + string(rune(id+'0'))

			// Set
			err := cache.Set(ctx, key, value, 0)
			if err != nil {
				t.Errorf("Concurrent Set failed: %v", err)
			}

			// Get
			got, err := cache.Get(ctx, key)
			if err != nil {
				t.Errorf("Concurrent Get failed: %v", err)
			}
			if got != value {
				t.Errorf("Concurrent Get got %v, expected %v", got, value)
			}

			// Delete
			err = cache.Delete(ctx, key)
			if err != nil {
				t.Errorf("Concurrent Delete failed: %v", err)
			}
		}(i)
	}

	wg.Wait()
}

func TestMemoryCache_KeyValidation(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "test",
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	ctx := context.Background()

	invalidKeys := []string{
		"",                       // empty
		"key with spaces",        // spaces
		"key\twith\ttabs",        // tabs
		"key\nwith\nnewlines",    // newlines
		strings.Repeat("a", 251), // too long
	}

	for _, key := range invalidKeys {
		err := cache.Set(ctx, key, "value", 0)
		if err == nil {
			t.Errorf("Expected error for invalid key: %q", key)
		}

		_, err = cache.Get(ctx, key)
		if err == nil {
			t.Errorf("Expected error for invalid key: %q", key)
		}

		err = cache.Delete(ctx, key)
		if err == nil {
			t.Errorf("Expected error for invalid key: %q", key)
		}
	}
}

func TestMemoryCache_Name(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "my-cache",
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	if cache.Name() != "my-cache" {
		t.Errorf("Expected name 'my-cache', got %q", cache.Name())
	}
}

func TestMemoryCache_Stats(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "test",
		MaxSize:         10,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	ctx := context.Background()

	// Initially empty
	stats := cache.Stats()
	if stats.Size != 0 {
		t.Errorf("Expected size 0, got %d", stats.Size)
	}
	if stats.MaxSize != 10 {
		t.Errorf("Expected max size 10, got %d", stats.MaxSize)
	}
	if stats.Capacity != 10 {
		t.Errorf("Expected capacity 10, got %d", stats.Capacity)
	}

	// Add some entries
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	stats = cache.Stats()
	if stats.Size != 2 {
		t.Errorf("Expected size 2, got %d", stats.Size)
	}
}

func TestMemoryCache_UnlimitedSize(t *testing.T) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "test",
		MaxSize:         0, // unlimited
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	stats := cache.Stats()
	if stats.Capacity != -1 {
		t.Errorf("Expected capacity -1 for unlimited, got %d", stats.Capacity)
	}
}

func BenchmarkMemoryCache_Get(b *testing.B) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "bench",
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(ctx, "key"+string(rune(i+'0')), "value"+string(rune(i+'0')), 0)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key" + string(rune((i%1000)+'0'))
			cache.Get(ctx, key)
			i++
		}
	})
}

func BenchmarkMemoryCache_Set(b *testing.B) {
	cache := NewMemoryCache(MemoryCacheConfig{
		Name:            "bench",
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	})
	defer cache.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := "key" + string(rune((i%1000)+'0'))
			cache.Set(ctx, key, "value"+string(rune((i%1000)+'0')), 0)
			i++
		}
	})
}
