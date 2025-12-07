package resilience

import (
	"context"
	"time"

	"cache-chain/pkg/cache"

	"github.com/sony/gobreaker"
)

// ResilientLayer wraps a CacheLayer with resilience features including
// circuit breaker and timeout protection.
type ResilientLayer struct {
	layer   cache.CacheLayer
	cb      *gobreaker.CircuitBreaker
	timeout time.Duration
}

// NewResilientLayer creates a new resilient layer wrapper around the given cache layer.
// It adds circuit breaker protection and timeout enforcement to all operations.
func NewResilientLayer(layer cache.CacheLayer, config ResilientConfig) *ResilientLayer {
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
			// Optional: log state changes
			// For now, just silent state transitions
		},
	}

	return &ResilientLayer{
		layer:   layer,
		cb:      gobreaker.NewCircuitBreaker(settings),
		timeout: config.Timeout,
	}
}

// Name returns the name of the underlying cache layer.
func (rl *ResilientLayer) Name() string {
	return rl.layer.Name()
}

// Get retrieves a value from the cache with timeout and circuit breaker protection.
func (rl *ResilientLayer) Get(ctx context.Context, key string) (interface{}, error) {
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

	if err != nil {
		// Convert gobreaker.ErrOpenState to our error type
		if err == gobreaker.ErrOpenState {
			return nil, cache.ErrCircuitOpen
		}
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, cache.ErrTimeout
		}
		return nil, err
	}

	return result, nil
}

// Set stores a value in the cache with timeout and circuit breaker protection.
func (rl *ResilientLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
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

// Delete removes a value from the cache with timeout and circuit breaker protection.
func (rl *ResilientLayer) Delete(ctx context.Context, key string) error {
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
