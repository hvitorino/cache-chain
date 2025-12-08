package cache

import (
	"context"
	"sync"
	"time"
)

// NegativeEntry represents a cached "not found" result.
type NegativeEntry struct {
	Key       string
	CachedAt  time.Time
	ExpiresAt time.Time
}

// NegativeCacheLayer wraps a cache layer with negative caching capability.
// It caches "not found" results to avoid repeated lookups for missing keys.
type NegativeCacheLayer struct {
	layer       CacheLayer
	negativeMap map[string]NegativeEntry
	negativeTTL time.Duration
	mu          sync.RWMutex
	stopCleanup chan struct{}
	cleanupDone chan struct{}
}

// NewNegativeCacheLayer creates a new negative cache layer wrapper.
// negativeTTL determines how long to cache "not found" results.
func NewNegativeCacheLayer(layer CacheLayer, negativeTTL time.Duration) *NegativeCacheLayer {
	if negativeTTL <= 0 {
		negativeTTL = 1 * time.Minute // Default: 1 minute
	}

	ncl := &NegativeCacheLayer{
		layer:       layer,
		negativeMap: make(map[string]NegativeEntry),
		negativeTTL: negativeTTL,
		stopCleanup: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}

	// Start background cleanup
	go ncl.cleanup()

	return ncl
}

// Name returns the name of the underlying cache layer.
func (ncl *NegativeCacheLayer) Name() string {
	return ncl.layer.Name() + "-negative"
}

// Get retrieves a value from the cache, checking negative cache first.
func (ncl *NegativeCacheLayer) Get(ctx context.Context, key string) (interface{}, error) {
	// Check negative cache first (fast path)
	if ncl.isNegativeCached(key) {
		return nil, ErrKeyNotFound
	}

	// Try to get from underlying layer
	value, err := ncl.layer.Get(ctx, key)
	if err != nil {
		// If key not found, cache the negative result
		if IsNotFound(err) {
			ncl.cacheNegative(key)
		}
		return nil, err
	}

	// Found: remove from negative cache if present
	ncl.removeNegative(key)
	return value, nil
}

// Set stores a value in the cache and removes it from negative cache.
func (ncl *NegativeCacheLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Remove from negative cache since we're setting a value
	ncl.removeNegative(key)
	return ncl.layer.Set(ctx, key, value, ttl)
}

// Delete removes a value from the cache and adds it to negative cache.
func (ncl *NegativeCacheLayer) Delete(ctx context.Context, key string) error {
	err := ncl.layer.Delete(ctx, key)
	if err == nil {
		// Successfully deleted: cache as negative
		ncl.cacheNegative(key)
	}
	return err
}

// Close stops the cleanup goroutine and closes the underlying layer.
func (ncl *NegativeCacheLayer) Close() error {
	close(ncl.stopCleanup)
	<-ncl.cleanupDone
	return ncl.layer.Close()
}

// isNegativeCached checks if a key is in the negative cache and still valid.
func (ncl *NegativeCacheLayer) isNegativeCached(key string) bool {
	ncl.mu.RLock()
	defer ncl.mu.RUnlock()

	entry, exists := ncl.negativeMap[key]
	if !exists {
		return false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		return false
	}

	return true
}

// cacheNegative adds a key to the negative cache.
func (ncl *NegativeCacheLayer) cacheNegative(key string) {
	ncl.mu.Lock()
	defer ncl.mu.Unlock()

	now := time.Now()
	ncl.negativeMap[key] = NegativeEntry{
		Key:       key,
		CachedAt:  now,
		ExpiresAt: now.Add(ncl.negativeTTL),
	}
}

// removeNegative removes a key from the negative cache.
func (ncl *NegativeCacheLayer) removeNegative(key string) {
	ncl.mu.Lock()
	defer ncl.mu.Unlock()

	delete(ncl.negativeMap, key)
}

// cleanup periodically removes expired negative cache entries.
func (ncl *NegativeCacheLayer) cleanup() {
	defer close(ncl.cleanupDone)

	ticker := time.NewTicker(ncl.negativeTTL / 2) // Cleanup twice per TTL period
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ncl.cleanupExpired()
		case <-ncl.stopCleanup:
			return
		}
	}
}

// cleanupExpired removes expired entries from negative cache.
func (ncl *NegativeCacheLayer) cleanupExpired() {
	ncl.mu.Lock()
	defer ncl.mu.Unlock()

	now := time.Now()
	for key, entry := range ncl.negativeMap {
		if now.After(entry.ExpiresAt) {
			delete(ncl.negativeMap, key)
		}
	}
}

// Stats returns statistics about the negative cache.
func (ncl *NegativeCacheLayer) Stats() NegativeCacheStats {
	ncl.mu.RLock()
	defer ncl.mu.RUnlock()

	return NegativeCacheStats{
		NegativeCount: len(ncl.negativeMap),
		NegativeTTL:   ncl.negativeTTL,
	}
}

// NegativeCacheStats holds statistics about negative caching.
type NegativeCacheStats struct {
	NegativeCount int
	NegativeTTL   time.Duration
}
