package resilience

import (
	"context"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/logging"
	"cache-chain/pkg/metrics"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// ResilientLayer wraps a CacheLayer with resilience features including
// circuit breaker and timeout protection.
type ResilientLayer struct {
	layer   cache.CacheLayer
	cb      *gobreaker.CircuitBreaker
	timeout time.Duration
	metrics metrics.MetricsCollector
	logger  *logging.Logger
}

// NewResilientLayer creates a new resilient layer wrapper around the given cache layer.
// It adds circuit breaker protection and timeout enforcement to all operations.
func NewResilientLayer(layer cache.CacheLayer, config ResilientConfig) *ResilientLayer {
	return NewResilientLayerWithMetrics(layer, config, metrics.NoOpCollector{})
}

// NewResilientLayerWithMetrics creates a new resilient layer with custom metrics collector.
func NewResilientLayerWithMetrics(layer cache.CacheLayer, config ResilientConfig, metricsCollector metrics.MetricsCollector) *ResilientLayer {
	logger := logging.Global().Named("resilience").Named(layer.Name())

	rl := &ResilientLayer{
		layer:   layer,
		timeout: config.Timeout,
		metrics: metricsCollector,
		logger:  logger,
	}

	logger.Info("resilient layer initialized",
		zap.String("layer", layer.Name()),
		zap.Duration("timeout", config.Timeout),
		zap.Uint32("max_requests", config.CircuitBreakerConfig.MaxRequests),
		zap.Duration("circuit_interval", config.CircuitBreakerConfig.Interval),
		zap.Duration("circuit_timeout", config.CircuitBreakerConfig.Timeout),
	)

	// Convert our config to gobreaker settings
	settings := gobreaker.Settings{
		Name:        layer.Name(),
		MaxRequests: config.CircuitBreakerConfig.MaxRequests,
		Interval:    config.CircuitBreakerConfig.Interval,
		Timeout:     config.CircuitBreakerConfig.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if config.CircuitBreakerConfig.ReadyToTrip != nil {
				return config.CircuitBreakerConfig.ReadyToTrip(Counts{
					Requests:             counts.Requests,
					TotalSuccesses:       counts.TotalSuccesses,
					TotalFailures:        counts.TotalFailures,
					ConsecutiveSuccesses: counts.ConsecutiveSuccesses,
					ConsecutiveFailures:  counts.ConsecutiveFailures,
				})
			}
			// Default: trip after 5 consecutive failures
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			// Log state change
			logger.Warn("circuit breaker state changed",
				zap.String("layer", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)

			// Report circuit breaker state changes to metrics
			var state metrics.CircuitState
			switch to {
			case gobreaker.StateClosed:
				state = metrics.CircuitClosed
			case gobreaker.StateHalfOpen:
				state = metrics.CircuitHalfOpen
			case gobreaker.StateOpen:
				state = metrics.CircuitOpen
			}
			rl.metrics.RecordCircuitState(layer.Name(), state)
		},
	}

	rl.cb = gobreaker.NewCircuitBreaker(settings)

	return rl
}

// Name returns the name of the underlying cache layer.
func (rl *ResilientLayer) Name() string {
	return rl.layer.Name()
}

// Get retrieves a value from the cache with timeout and circuit breaker protection.
func (rl *ResilientLayer) Get(ctx context.Context, key string) (interface{}, error) {
	start := time.Now()
	layerName := rl.layer.Name()

	// Apply timeout if configured
	if rl.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rl.timeout)
		defer cancel()
	}

	// Execute through circuit breaker
	result, err := rl.cb.Execute(func() (interface{}, error) {
		return rl.layer.Get(ctx, key)
	})

	// Record metrics
	duration := time.Since(start)
	hit := err == nil
	rl.metrics.RecordGet(layerName, hit, duration)

	if err != nil {
		// Convert gobreaker.ErrOpenState to our error type
		if err == gobreaker.ErrOpenState {
			rl.logger.Warn("circuit breaker open - request rejected",
				zap.String("operation", "get"),
				zap.String("key", key),
			)
			return nil, cache.ErrCircuitOpen
		}
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			rl.logger.Warn("operation timeout",
				zap.String("operation", "get"),
				zap.String("key", key),
				zap.Duration("timeout", rl.timeout),
				zap.Duration("elapsed", duration),
			)
			return nil, cache.ErrTimeout
		}
		// Log other errors
		rl.logger.Error("get operation failed",
			zap.String("operation", "get"),
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return nil, err
	}

	return result, nil
}

// Set stores a value in the cache with timeout and circuit breaker protection.
func (rl *ResilientLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	layerName := rl.layer.Name()

	// Apply timeout if configured
	if rl.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rl.timeout)
		defer cancel()
	}

	// Execute through circuit breaker
	_, err := rl.cb.Execute(func() (interface{}, error) {
		return nil, rl.layer.Set(ctx, key, value, ttl)
	})

	// Record metrics
	duration := time.Since(start)
	success := err == nil
	rl.metrics.RecordSet(layerName, success, duration)

	if err != nil {
		// Convert gobreaker.ErrOpenState to our error type
		if err == gobreaker.ErrOpenState {
			rl.logger.Warn("circuit breaker open - request rejected",
				zap.String("operation", "set"),
			)
			return cache.ErrCircuitOpen
		}
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			rl.logger.Warn("operation timeout",
				zap.String("operation", "set"),
				zap.Duration("timeout", rl.timeout),
				zap.Duration("elapsed", duration),
			)
			return cache.ErrTimeout
		}
		// Log other errors
		rl.logger.Error("set operation failed",
			zap.String("operation", "set"),
			zap.Duration("ttl", ttl),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return err
	}

	return nil
}

// Delete removes a value from the cache with timeout and circuit breaker protection.
func (rl *ResilientLayer) Delete(ctx context.Context, key string) error {
	start := time.Now()
	layerName := rl.layer.Name()

	// Apply timeout if configured
	if rl.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rl.timeout)
		defer cancel()
	}

	// Execute through circuit breaker
	_, err := rl.cb.Execute(func() (interface{}, error) {
		return nil, rl.layer.Delete(ctx, key)
	})

	// Record metrics
	duration := time.Since(start)
	success := err == nil
	rl.metrics.RecordDelete(layerName, success, duration)

	if err != nil {
		// Convert gobreaker.ErrOpenState to our error type
		if err == gobreaker.ErrOpenState {
			rl.logger.Warn("circuit breaker open - request rejected",
				zap.String("operation", "delete"),
				zap.String("key", key),
			)
			return cache.ErrCircuitOpen
		}
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			rl.logger.Warn("operation timeout",
				zap.String("operation", "delete"),
				zap.String("key", key),
				zap.Duration("timeout", rl.timeout),
				zap.Duration("elapsed", duration),
			)
			return cache.ErrTimeout
		}
		// Log other errors
		rl.logger.Error("delete operation failed",
			zap.String("operation", "delete"),
			zap.String("key", key),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return err
	}

	return nil
}

// Clear removes all values from the cache with timeout and circuit breaker protection.
func (rl *ResilientLayer) Clear(ctx context.Context) error {
	// Apply timeout if configured
	if rl.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rl.timeout)
		defer cancel()
	}

	// Check if the underlying layer supports Clear
	type clearer interface {
		Clear(ctx context.Context) error
	}

	cl, ok := rl.layer.(clearer)
	if !ok {
		// Layer doesn't support Clear, return success
		return nil
	}

	// Execute through circuit breaker
	_, err := rl.cb.Execute(func() (interface{}, error) {
		return nil, cl.Clear(ctx)
	})

	if err != nil {
		// Convert gobreaker.ErrOpenState to our error type
		if err == gobreaker.ErrOpenState {
			return cache.ErrCircuitOpen
		}
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return cache.ErrTimeout
		}
		return err
	}

	return nil
}

// Close closes the underlying cache layer.
func (rl *ResilientLayer) Close() error {
	return rl.layer.Close()
}
