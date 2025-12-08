package cache

import (
	"context"
	"sync"
	"time"
)

// BatchCacheLayer extends CacheLayer with batch operations.
type BatchCacheLayer interface {
	CacheLayer

	// GetMulti retrieves multiple keys at once
	GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error)

	// SetMulti stores multiple key-value pairs at once
	SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error

	// DeleteMulti removes multiple keys at once
	DeleteMulti(ctx context.Context, keys []string) error
}

// BatchAdapter wraps a non-batch layer with batch operations.
type BatchAdapter struct {
	layer CacheLayer
}

// NewBatchAdapter creates a new batch adapter.
func NewBatchAdapter(layer CacheLayer) *BatchAdapter {
	return &BatchAdapter{layer: layer}
}

// Name returns the name of the underlying cache layer.
func (ba *BatchAdapter) Name() string {
	return "batch(" + ba.layer.Name() + ")"
}

// Get retrieves a single value.
func (ba *BatchAdapter) Get(ctx context.Context, key string) (interface{}, error) {
	return ba.layer.Get(ctx, key)
}

// Set stores a single value.
func (ba *BatchAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return ba.layer.Set(ctx, key, value, ttl)
}

// Delete removes a single value.
func (ba *BatchAdapter) Delete(ctx context.Context, key string) error {
	return ba.layer.Delete(ctx, key)
}

// Close closes the underlying layer.
func (ba *BatchAdapter) Close() error {
	return ba.layer.Close()
}

// GetMulti retrieves multiple keys in parallel.
func (ba *BatchAdapter) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, key := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			if value, err := ba.layer.Get(ctx, k); err == nil {
				mu.Lock()
				results[k] = value
				mu.Unlock()
			}
		}(key)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return results, nil
}

// SetMulti stores multiple key-value pairs in parallel.
func (ba *BatchAdapter) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var lastErr error

	for key, value := range items {
		wg.Add(1)
		go func(k string, v interface{}) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := ba.layer.Set(ctx, k, v, ttl); err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
			}
		}(key, value)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	return lastErr
}

// DeleteMulti removes multiple keys in parallel.
func (ba *BatchAdapter) DeleteMulti(ctx context.Context, keys []string) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var lastErr error

	for _, key := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := ba.layer.Delete(ctx, k); err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
			}
		}(key)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	return lastErr
}
