package chain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/resilience"
	"cache-chain/pkg/writer"

	"golang.org/x/sync/singleflight"
)

// Chain manages multiple cache layers with automatic fallback and warm-up.
// Layers are ordered from fastest (L1) to slowest (LN).
type Chain struct {
	layers  []cache.CacheLayer
	writers []*writer.AsyncWriter
	sf      *singleflight.Group
}

// New creates a new chain of cache layers.
// Layers should be ordered from fastest to slowest (L1 to LN).
// Returns an error if no layers are provided.
// All layers are automatically wrapped with resilience protection.
func New(layers ...cache.CacheLayer) (*Chain, error) {
	if len(layers) == 0 {
		return nil, errors.New("chain: at least one layer required")
	}

	// Wrap each layer with resilience protection
	resilientLayers := make([]cache.CacheLayer, len(layers))
	for i, layer := range layers {
		config := resilience.DefaultResilientConfig()

		// Customize timeout based on layer position
		// L1 (memory) should be fast, deeper layers can be slower
		if i == 0 {
			config = config.WithTimeout(100 * time.Millisecond)
		} else {
			config = config.WithTimeout(1 * time.Second)
		}

		resilientLayers[i] = resilience.NewResilientLayer(layer, config)
	}

	// Create async writers for each resilient layer (used for warm-up)
	writers := make([]*writer.AsyncWriter, len(resilientLayers))
	for i, layer := range resilientLayers {
		writers[i] = writer.NewAsyncWriter(layer, writer.AsyncWriterConfig{
			QueueSize:   1000,
			Workers:     2,
			MaxWaitTime: 10 * time.Millisecond,
		})
	}

	return &Chain{
		layers:  resilientLayers,
		writers: writers,
		sf:      &singleflight.Group{},
	}, nil
}

// Get retrieves a value from the chain.
// It traverses layers in order until a hit, then synchronously warms upper layers.
// Uses single-flight to prevent duplicate Gets for the same key.
func (c *Chain) Get(ctx context.Context, key string) (interface{}, error) {
	// Check context before single-flight
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Use single-flight to prevent thundering herd
	result, err, _ := c.sf.Do(key, func() (interface{}, error) {
		return c.getWithFallback(ctx, key)
	})

	return result, err
}

// getWithFallback performs the actual chain traversal and warm-up.
func (c *Chain) getWithFallback(ctx context.Context, key string) (interface{}, error) {
	var lastErr error

	for i, layer := range c.layers {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		value, err := layer.Get(ctx, key)
		if err != nil {
			// Check if it's a "not found" error - continue to next layer
			if cache.IsNotFound(err) {
				lastErr = err
				continue
			}
			// Other errors (timeout, unavailable) - skip this layer but continue
			lastErr = err
			continue
		}

		// Hit! Warm up upper layers synchronously
		if i > 0 {
			c.warmUpperLayers(ctx, key, value, i)
		}

		return value, nil
	}

	// All layers missed
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, cache.ErrKeyNotFound
}

// warmUpperLayers asynchronously warms all layers above the hit layer.
func (c *Chain) warmUpperLayers(ctx context.Context, key string, value interface{}, hitIndex int) {
	// Warm up from hit layer up to L1
	// Use a reasonable default TTL for warm-up (1 hour)
	ttl := time.Hour

	for i := hitIndex - 1; i >= 0; i-- {
		// Use async writer instead of direct Set() - non-blocking
		// Errors are tracked internally by AsyncWriter
		_ = c.writers[i].Write(ctx, key, value, ttl)
	}
}

// Set writes the value to all layers in the chain.
// If any layer fails, the error is returned but other layers are still attempted.
func (c *Chain) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var lastErr error

	for _, layer := range c.layers {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := layer.Set(ctx, key, value, ttl); err != nil {
			lastErr = err
			// Continue to set other layers even if one fails
		}
	}

	return lastErr
}

// Delete removes the key from all layers in the chain.
// If any layer fails, the error is returned but other layers are still attempted.
func (c *Chain) Delete(ctx context.Context, key string) error {
	var lastErr error

	for _, layer := range c.layers {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := layer.Delete(ctx, key); err != nil {
			lastErr = err
			// Continue to delete from other layers even if one fails
		}
	}

	return lastErr
}

// Close closes all layers in the chain.
// Returns the first error encountered, but attempts to close all layers.
func (c *Chain) Close() error {
	var lastErr error

	// Close async writers first
	for _, w := range c.writers {
		if err := w.Close(); err != nil {
			lastErr = err
		}
	}

	// Then close layers
	for _, layer := range c.layers {
		if err := layer.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Layers returns a copy of the layers slice for inspection.
func (c *Chain) Layers() []cache.CacheLayer {
	layers := make([]cache.CacheLayer, len(c.layers))
	copy(layers, c.layers)
	return layers
}

// Len returns the number of layers in the chain.
func (c *Chain) Len() int {
	return len(c.layers)
}

// String returns a string representation of the chain.
func (c *Chain) String() string {
	if len(c.layers) == 0 {
		return "chain: empty"
	}

	result := fmt.Sprintf("chain(%d layers): ", len(c.layers))
	for i, layer := range c.layers {
		if i > 0 {
			result += " → "
		}
		result += layer.Name()
	}
	return result
}
