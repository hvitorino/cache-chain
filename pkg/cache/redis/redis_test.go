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

func TestClusterCacheConfig(t *testing.T) {
	clusterAddrs := []string{"node1:6379", "node2:6379", "node3:6379"}
	config := ClusterCacheConfig("TestCluster", clusterAddrs, "secret")

	if config.Name != "TestCluster" {
		t.Errorf("Expected name 'TestCluster', got '%s'", config.Name)
	}

	if len(config.ClusterAddrs) != 3 {
		t.Errorf("Expected 3 cluster addresses, got %d", len(config.ClusterAddrs))
	}

	if config.Password != "secret" {
		t.Errorf("Expected password 'secret', got '%s'", config.Password)
	}

	if config.Addr != "" {
		t.Error("Expected Addr to be empty in cluster mode")
	}

	if config.DB != 0 {
		t.Error("Expected DB to be 0 in cluster mode")
	}
}

func TestSentinelCacheConfig(t *testing.T) {
	sentinelAddrs := []string{"sentinel1:26379", "sentinel2:26379", "sentinel3:26379"}
	config := SentinelCacheConfig("TestSentinel", sentinelAddrs, "mymaster", "secret")

	if config.Name != "TestSentinel" {
		t.Errorf("Expected name 'TestSentinel', got '%s'", config.Name)
	}

	if len(config.SentinelAddrs) != 3 {
		t.Errorf("Expected 3 sentinel addresses, got %d", len(config.SentinelAddrs))
	}

	if config.SentinelMasterSet != "mymaster" {
		t.Errorf("Expected master set 'mymaster', got '%s'", config.SentinelMasterSet)
	}

	if config.Password != "secret" {
		t.Errorf("Expected password 'secret', got '%s'", config.Password)
	}

	if config.Addr != "" {
		t.Error("Expected Addr to be empty in sentinel mode")
	}
}

func TestNewRedisCache_NoAddressError(t *testing.T) {
	config := RedisCacheConfig{
		Name: "NoAddr",
		// All address fields empty
	}

	_, err := NewRedisCache(config)
	if err == nil {
		t.Error("Expected error when no addresses configured")
	}
}
