package memory

import (
	"context"
	"sync"
	"time"
	"unicode"
)

// MemoryCache is an in-memory cache implementation that satisfies the CacheLayer interface.
// It provides thread-safe operations, automatic TTL expiration, and optional LRU eviction.
type MemoryCache struct {
	// data stores the cache entries
	data map[string]*entry

	// mu protects concurrent access to data
	mu sync.RWMutex

	// config holds the cache configuration
	config MemoryCacheConfig

	// cleanupTicker controls the background cleanup interval
	cleanupTicker *time.Ticker

	// stopCleanup is used to signal cleanup goroutine to stop
	stopCleanup chan struct{}

	// wg waits for cleanup goroutine to finish
	wg sync.WaitGroup
}

// entry represents a cache entry with metadata for LRU and TTL
type entry struct {
	key        string
	value      interface{}
	expiresAt  time.Time
	accessedAt time.Time
	version    int64
}

// MemoryCacheConfig holds configuration for the memory cache
type MemoryCacheConfig struct {
	// Name is the cache layer identifier
	Name string

	// MaxSize is the maximum number of entries (0 = unlimited)
	MaxSize int

	// DefaultTTL is the default time-to-live for entries
	DefaultTTL time.Duration

	// CleanupInterval is how often to check for expired entries
	CleanupInterval time.Duration
}

// NewMemoryCache creates a new in-memory cache with the given configuration.
// It starts a background goroutine for TTL cleanup.
func NewMemoryCache(config MemoryCacheConfig) *MemoryCache {
	// Set defaults
	if config.Name == "" {
		config.Name = "memory"
	}
	if config.DefaultTTL == 0 {
		config.DefaultTTL = time.Hour
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = time.Minute
	}

	cache := &MemoryCache{
		data:          make(map[string]*entry),
		config:        config,
		stopCleanup:   make(chan struct{}),
		cleanupTicker: time.NewTicker(config.CleanupInterval),
	}

	// Start background cleanup
	cache.wg.Add(1)
	go cache.cleanup()

	return cache
}

// Get retrieves a value from the cache.
// Returns the value and nil error if found, or an error if not found or operation failed.
func (c *MemoryCache) Get(ctx context.Context, key string) (interface{}, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}

	c.mu.RLock()
	entry, exists := c.data[key]
	c.mu.RUnlock()

	if !exists {
		return nil, errKeyNotFound
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		// Remove expired entry
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		return nil, errKeyNotFound
	}

	// Update access time for LRU
	c.mu.Lock()
	entry.accessedAt = time.Now()
	c.mu.Unlock()

	return entry.value, nil
}

// Set stores a value in the cache with the specified TTL.
// If ttl is 0, uses the default TTL.
// Enforces MaxSize by evicting LRU entries if necessary.
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if err := validateKey(key); err != nil {
		return err
	}

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = c.config.DefaultTTL
	}

	expiresAt := time.Now().Add(ttl)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict for LRU
	if c.config.MaxSize > 0 && len(c.data) >= c.config.MaxSize {
		// Find the least recently used entry
		var lruKey string
		var lruTime time.Time

		for k, e := range c.data {
			if lruKey == "" || e.accessedAt.Before(lruTime) {
				lruKey = k
				lruTime = e.accessedAt
			}
		}

		// Evict LRU entry
		if lruKey != "" {
			delete(c.data, lruKey)
		}
	}

	// Store the entry
	c.data[key] = &entry{
		key:        key,
		value:      value,
		expiresAt:  expiresAt,
		accessedAt: time.Now(),
		version:    time.Now().UnixNano(), // Simple versioning
	}

	return nil
}

// Delete removes a key from the cache.
// Returns nil even if the key doesn't exist.
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()

	return nil
}

// Name returns the cache layer name.
func (c *MemoryCache) Name() string {
	return c.config.Name
}

// Close stops the background cleanup goroutine and clears all data.
func (c *MemoryCache) Close() error {
	// Stop cleanup ticker
	c.cleanupTicker.Stop()

	// Signal cleanup goroutine to stop
	close(c.stopCleanup)

	// Wait for cleanup to finish
	c.wg.Wait()

	// Clear data
	c.mu.Lock()
	c.data = nil
	c.mu.Unlock()

	return nil
}

// cleanup runs in a background goroutine to remove expired entries.
func (c *MemoryCache) cleanup() {
	defer c.wg.Done()

	for {
		select {
		case <-c.cleanupTicker.C:
			c.removeExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

// removeExpired removes all expired entries from the cache.
func (c *MemoryCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.data {
		if now.After(entry.expiresAt) {
			delete(c.data, key)
		}
	}
}

// Stats returns current cache statistics.
func (c *MemoryCache) Stats() MemoryCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := MemoryCacheStats{
		Size:     len(c.data),
		MaxSize:  c.config.MaxSize,
		Capacity: c.config.MaxSize,
	}

	if stats.Capacity == 0 {
		stats.Capacity = -1 // Unlimited
	}

	return stats
}

// MemoryCacheStats holds cache statistics.
type MemoryCacheStats struct {
	Size     int // Current number of entries
	MaxSize  int // Maximum allowed entries (0 = unlimited)
	Capacity int // Effective capacity (-1 = unlimited)
}

// validateKey checks if a cache key is valid.
// Simple validation: non-empty, no whitespace characters, reasonable length.
func validateKey(key string) error {
	if key == "" {
		return errInvalidKey
	}

	if len(key) > 250 {
		return errInvalidKey
	}

	// Check for control characters and whitespace
	for _, r := range key {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return errInvalidKey
		}
	}

	return nil
}

// Cache errors (local to this package)
var (
	errKeyNotFound = &cacheError{"key not found"}
	errInvalidKey  = &cacheError{"invalid key"}
)

// cacheError implements error
type cacheError struct {
	msg string
}

func (e *cacheError) Error() string {
	return "cache: " + e.msg
}
