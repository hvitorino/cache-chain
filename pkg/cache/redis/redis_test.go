package redis

import (
	"context"
	"testing"
	"time"

	"cache-chain/pkg/cache"
)

func skipIfNoRedis(t *testing.T, r *RedisCache) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := r.Ping(ctx); err != nil {
		t.Skipf("Redis not available: %v", err)
	}
}

func setupTestRedis(t *testing.T) *RedisCache {
	config := DefaultRedisCacheConfig()
	config.Name = "TestRedis"
	config.KeyPrefix = "test:cache:"

	r, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Failed to create Redis client: %v", err)
	}

	skipIfNoRedis(t, r)

	ctx := context.Background()
	r.FlushDB(ctx)

	return r
}

func TestNewRedisCache(t *testing.T) {
	config := DefaultRedisCacheConfig()
	config.Name = "TestRedis"

	r, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer r.Close()

	if r.Name() != "TestRedis" {
		t.Errorf("Expected name 'TestRedis', got '%s'", r.Name())
	}

	skipIfNoRedis(t, r)
}

func TestRedisCache_SetGet(t *testing.T) {
	r := setupTestRedis(t)
	defer r.Close()

	ctx := context.Background()

	err := r.Set(ctx, "key1", "value1", time.Minute)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}

	val, err := r.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	if val != "value1" {
		t.Errorf("Expected 'value1', got '%v'", val)
	}
}

func TestRedisCache_GetMiss(t *testing.T) {
	r := setupTestRedis(t)
	defer r.Close()

	ctx := context.Background()

	_, err := r.Get(ctx, "nonexistent")
	if err != cache.ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss, got %v", err)
	}
}

func TestRedisCache_Delete(t *testing.T) {
	r := setupTestRedis(t)
	defer r.Close()

	ctx := context.Background()

	r.Set(ctx, "key1", "value1", time.Minute)

	err := r.Delete(ctx, "key1")
	if err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	_, err = r.Get(ctx, "key1")
	if err != cache.ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestDefaultRedisCacheConfig(t *testing.T) {
	config := DefaultRedisCacheConfig()

	if config.Name != "Redis" {
		t.Errorf("Expected default name 'Redis', got '%s'", config.Name)
	}

	if config.Addr != "localhost:6379" {
		t.Errorf("Expected default addr 'localhost:6379', got '%s'", config.Addr)
	}

	if config.KeyPrefix != "cache:" {
		t.Errorf("Expected default prefix 'cache:', got '%s'", config.KeyPrefix)
	}

	if !config.EnablePipelining {
		t.Error("Expected pipelining to be enabled by default")
	}
}
