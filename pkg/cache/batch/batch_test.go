package batch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/cache/memory"
)

func TestBatchAdapter_SetMulti(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	batch := cache.NewBatchAdapter(base)
	defer batch.Close()

	ctx := context.Background()

	items := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": 42,
		"key5": true,
	}

	err := batch.SetMulti(ctx, items, time.Hour)
	if err != nil {
		t.Fatalf("SetMulti failed: %v", err)
	}

	// Verify all items were set
	for key, expected := range items {
		val, err := batch.Get(ctx, key)
		if err != nil {
			t.Errorf("Get %s failed: %v", key, err)
		}
		if val != expected {
			t.Errorf("Key %s: expected %v, got %v", key, expected, val)
		}
	}
}

func TestBatchAdapter_GetMulti(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	batch := cache.NewBatchAdapter(base)
	defer batch.Close()

	ctx := context.Background()

	// Set some keys
	batch.Set(ctx, "key1", "value1", time.Hour)
	batch.Set(ctx, "key2", "value2", time.Hour)
	batch.Set(ctx, "key3", "value3", time.Hour)

	keys := []string{"key1", "key2", "key3", "key-missing"}
	results, err := batch.GetMulti(ctx, keys)
	if err != nil {
		t.Fatalf("GetMulti failed: %v", err)
	}

	// Should get 3 results (missing key excluded)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Check values
	expected := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, val := range expected {
		if results[key] != val {
			t.Errorf("Key %s: expected %v, got %v", key, val, results[key])
		}
	}

	// Missing key should not be in results
	if _, ok := results["key-missing"]; ok {
		t.Error("Missing key should not be in results")
	}
}

func TestBatchAdapter_DeleteMulti(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	batch := cache.NewBatchAdapter(base)
	defer batch.Close()

	ctx := context.Background()

	// Set some keys
	batch.Set(ctx, "key1", "value1", time.Hour)
	batch.Set(ctx, "key2", "value2", time.Hour)
	batch.Set(ctx, "key3", "value3", time.Hour)

	keys := []string{"key1", "key2"}
	err := batch.DeleteMulti(ctx, keys)
	if err != nil {
		t.Fatalf("DeleteMulti failed: %v", err)
	}

	// Deleted keys should not exist
	_, err = batch.Get(ctx, "key1")
	if err != cache.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound for key1, got %v", err)
	}

	_, err = batch.Get(ctx, "key2")
	if err != cache.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound for key2, got %v", err)
	}

	// key3 should still exist
	val, err := batch.Get(ctx, "key3")
	if err != nil {
		t.Errorf("Get key3 failed: %v", err)
	}
	if val != "value3" {
		t.Errorf("Expected value3, got %v", val)
	}
}

func TestBatchAdapter_ContextCancellation(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	batch := cache.NewBatchAdapter(base)
	defer batch.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	items := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	err := batch.SetMulti(ctx, items, time.Hour)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}

	keys := []string{"key1", "key2"}
	_, err = batch.GetMulti(ctx, keys)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}

	err = batch.DeleteMulti(ctx, keys)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestBatchAdapter_EmptyOperations(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	batch := cache.NewBatchAdapter(base)
	defer batch.Close()

	ctx := context.Background()

	// Empty SetMulti
	err := batch.SetMulti(ctx, map[string]interface{}{}, time.Hour)
	if err != nil {
		t.Errorf("Empty SetMulti failed: %v", err)
	}

	// Empty GetMulti
	results, err := batch.GetMulti(ctx, []string{})
	if err != nil {
		t.Errorf("Empty GetMulti failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}

	// Empty DeleteMulti
	err = batch.DeleteMulti(ctx, []string{})
	if err != nil {
		t.Errorf("Empty DeleteMulti failed: %v", err)
	}
}

func TestBatchAdapter_Name(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "TestLayer",
		MaxSize: 100,
	})
	batch := cache.NewBatchAdapter(base)
	defer batch.Close()

	name := batch.Name()
	expected := "batch(TestLayer)"
	if name != expected {
		t.Errorf("Expected name %s, got %s", expected, name)
	}
}

func TestBatchAdapter_LargeScale(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 10000,
	})
	batch := cache.NewBatchAdapter(base)
	defer batch.Close()

	ctx := context.Background()

	// Set 1000 items
	items := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		items[fmt.Sprintf("key-%d", i)] = fmt.Sprintf("value-%d", i)
	}

	err := batch.SetMulti(ctx, items, time.Hour)
	if err != nil {
		t.Fatalf("SetMulti failed: %v", err)
	}

	// Get 1000 items
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = fmt.Sprintf("key-%d", i)
	}

	results, err := batch.GetMulti(ctx, keys)
	if err != nil {
		t.Fatalf("GetMulti failed: %v", err)
	}

	if len(results) != 1000 {
		t.Errorf("Expected 1000 results, got %d", len(results))
	}

	// Delete 1000 items
	err = batch.DeleteMulti(ctx, keys)
	if err != nil {
		t.Fatalf("DeleteMulti failed: %v", err)
	}

	// Verify all deleted
	results, err = batch.GetMulti(ctx, keys)
	if err != nil {
		t.Fatalf("GetMulti after delete failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results after delete, got %d", len(results))
	}
}

func TestBatchAdapter_PartialFailures(t *testing.T) {
	base := memory.NewMemoryCache(memory.MemoryCacheConfig{
		Name:    "test",
		MaxSize: 100,
	})
	batch := cache.NewBatchAdapter(base)
	defer batch.Close()

	ctx := context.Background()

	// Set some keys
	batch.Set(ctx, "key1", "value1", time.Hour)
	batch.Set(ctx, "key3", "value3", time.Hour)

	// GetMulti with mix of existing and missing keys
	keys := []string{"key1", "key2", "key3", "key4"}
	results, err := batch.GetMulti(ctx, keys)
	if err != nil {
		t.Fatalf("GetMulti failed: %v", err)
	}

	// Should get 2 results
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Check we got the right ones
	if results["key1"] != "value1" {
		t.Error("Missing key1")
	}
	if results["key3"] != "value3" {
		t.Error("Missing key3")
	}
}
