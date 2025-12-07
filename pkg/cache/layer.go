package cache

import (
	"context"
	"time"
)

// CacheLayer defines the interface that all cache layer implementations must satisfy.
// It provides basic cache operations with context support for cancellation and timeouts.
type CacheLayer interface {
	// Get retrieves a value from the cache by key.
	// Returns the value and nil error if found, or an error if not found or operation failed.
	Get(ctx context.Context, key string) (interface{}, error)

	// Set stores a value in the cache with the specified key and time-to-live.
	// The TTL determines how long the value should be cached before expiration.
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a value from the cache by key.
	// Returns nil if the key was deleted or didn't exist, error if operation failed.
	Delete(ctx context.Context, key string) error

	// Name returns the identifier for this cache layer (e.g., "L1", "redis", "database").
	// Used for logging, metrics, and debugging.
	Name() string

	// Close releases any resources held by the cache layer.
	// Should be called when the layer is no longer needed.
	Close() error
}

// CacheEntry represents a cached value with metadata.
// It includes the key, value, expiration time, version for optimistic concurrency,
// and creation timestamp for debugging and metrics.
type CacheEntry struct {
	// Key is the cache key
	Key string

	// Value is the cached value (can be any type)
	Value interface{}

	// ExpiresAt is when this entry should be considered expired
	ExpiresAt time.Time

	// Version is used for optimistic concurrency control and cache invalidation
	Version int64

	// CreatedAt is when this entry was first created
	CreatedAt time.Time
}

// IsExpired checks if the cache entry has expired based on the current time.
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// TimeToLive returns the remaining time-to-live for this entry.
// Returns 0 if already expired.
func (e *CacheEntry) TimeToLive() time.Duration {
	if e.IsExpired() {
		return 0
	}
	return time.Until(e.ExpiresAt)
}
