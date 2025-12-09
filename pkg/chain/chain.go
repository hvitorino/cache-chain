package chain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/logging"
	"cache-chain/pkg/metrics"
	"cache-chain/pkg/resilience"
	"cache-chain/pkg/writer"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// Chain manages multiple cache layers with automatic fallback and warm-up.
// Layers are ordered from fastest (L1) to slowest (LN).
type Chain struct {
	layers      []cache.CacheLayer
	writers     []*writer.AsyncWriter
	sf          *singleflight.Group
	metrics     metrics.MetricsCollector
	ttlStrategy TTLStrategy
	logger      *logging.Logger
}

// ChainConfig holds configuration for Chain creation.
type ChainConfig struct {
	// Metrics collector for observability (optional, defaults to NoOpCollector)
	Metrics metrics.MetricsCollector

	// ResilientConfig for each layer (optional, uses defaults if nil)
	ResilientConfigs []resilience.ResilientConfig

	// AsyncWriterConfig for each layer (optional, uses defaults if nil)
	WriterConfigs []writer.AsyncWriterConfig

	// TTLStrategy for hierarchical TTL management (optional, defaults to UniformTTLStrategy)
	TTLStrategy TTLStrategy

	// Logger for structured logging (optional, uses global if nil)
	Logger *logging.Logger
}

// New creates a new chain of cache layers with default configuration.
// Layers should be ordered from fastest to slowest (L1 to LN).
// Returns an error if no layers are provided.
// All layers are automatically wrapped with resilience protection.
func New(layers ...cache.CacheLayer) (*Chain, error) {
	return NewWithConfig(ChainConfig{}, layers...)
}

// NewWithConfig creates a new chain with custom configuration.
func NewWithConfig(config ChainConfig, layers ...cache.CacheLayer) (*Chain, error) {
	if len(layers) == 0 {
		return nil, errors.New("chain: at least one layer required")
	}

	// Default to NoOpCollector if not provided
	if config.Metrics == nil {
		config.Metrics = metrics.NoOpCollector{}
	}

	// Default to UniformTTLStrategy if not provided
	if config.TTLStrategy == nil {
		config.TTLStrategy = &UniformTTLStrategy{}
	}

	// Set logger
	logger := config.Logger
	if logger == nil {
		logger = logging.Global()
	}

	logger.Info("initializing cache chain",
		zap.Int("num_layers", len(layers)),
		zap.Strings("layer_names", func() []string {
			names := make([]string, len(layers))
			for i, l := range layers {
				names[i] = l.Name()
			}
			return names
		}()),
	)

	// Wrap each layer with resilience protection
	resilientLayers := make([]cache.CacheLayer, len(layers))
	for i, layer := range layers {
		var resConfig resilience.ResilientConfig

		// Use provided config or default
		if config.ResilientConfigs != nil && i < len(config.ResilientConfigs) {
			resConfig = config.ResilientConfigs[i]
		} else {
			resConfig = resilience.DefaultResilientConfig()

			// Customize timeout based on layer position
			// L1 (memory) should be fast, deeper layers can be slower
			if i == 0 {
				resConfig = resConfig.WithTimeout(100 * time.Millisecond)
			} else {
				resConfig = resConfig.WithTimeout(1 * time.Second)
			}
		}

		// Pass metrics to resilient layer
		resilientLayers[i] = resilience.NewResilientLayerWithMetrics(layer, resConfig, config.Metrics)
	}

	// Create async writers for each resilient layer (used for warm-up)
	writers := make([]*writer.AsyncWriter, len(resilientLayers))
	for i, layer := range resilientLayers {
		var writerConfig writer.AsyncWriterConfig

		// Use provided config or default
		if config.WriterConfigs != nil && i < len(config.WriterConfigs) {
			writerConfig = config.WriterConfigs[i]
		} else {
			writerConfig = writer.AsyncWriterConfig{
				QueueSize:   1000,
				Workers:     2,
				MaxWaitTime: 10 * time.Millisecond,
			}
		}

		// Pass metrics to async writer
		writers[i] = writer.NewAsyncWriterWithMetrics(layer, writerConfig, config.Metrics)
	}

	logger.Info("cache chain initialized successfully",
		zap.Int("num_layers", len(resilientLayers)),
	)

	return &Chain{
		layers:      resilientLayers,
		writers:     writers,
		sf:          &singleflight.Group{},
		metrics:     config.Metrics,
		ttlStrategy: config.TTLStrategy,
		logger:      logger,
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
	start := time.Now()
	var lastErr error
	hitLayer := -1

	c.logger.Debug("chain get started",
		zap.String("key", key),
	)

	// Track metrics at the end
	defer func() {
		hit := hitLayer >= 0
		duration := time.Since(start)
		c.metrics.RecordChainGet(hit, hitLayer, duration)

		if hit {
			c.logger.Debug("chain get completed",
				zap.String("key", key),
				zap.Int("hit_layer", hitLayer),
				zap.String("layer_name", c.layers[hitLayer].Name()),
				zap.Duration("duration", duration),
			)
		} else {
			c.logger.Warn("chain get miss - all layers",
				zap.String("key", key),
				zap.Duration("duration", duration),
				zap.Error(lastErr),
			)
		}
	}()

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
		hitLayer = i
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
	baseTTL := time.Hour

	c.logger.Debug("warming upper layers",
		zap.String("key", key),
		zap.Int("hit_layer", hitIndex),
		zap.Int("layers_to_warm", hitIndex),
	)

	for i := hitIndex - 1; i >= 0; i-- {
		// Calculate TTL for this layer using strategy
		ttl := c.ttlStrategy.GetTTL(i, baseTTL)

		// Use async writer instead of direct Set() - non-blocking
		// Errors are tracked internally by AsyncWriter
		_ = c.writers[i].Write(ctx, key, value, ttl)
	}
}

// Set writes the value to all layers in the chain.
// If any layer fails, the error is returned but other layers are still attempted.
// The TTL is adjusted per layer using the configured TTLStrategy.
func (c *Chain) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var lastErr error

	for i, layer := range c.layers {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Calculate TTL for this layer using strategy
		layerTTL := c.ttlStrategy.GetTTL(i, ttl)

		if err := layer.Set(ctx, key, value, layerTTL); err != nil {
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
