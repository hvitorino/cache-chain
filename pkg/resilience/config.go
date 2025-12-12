package resilience

import (
	"time"
)

// ResilientConfig configures resilience features for a cache layer.
type ResilientConfig struct {
	// Timeout for cache operations
	Timeout time.Duration

	// CircuitBreakerConfig configures the circuit breaker behavior
	CircuitBreakerConfig CircuitBreakerConfig
}

// CircuitBreakerConfig configures circuit breaker behavior.
type CircuitBreakerConfig struct {
	// MaxRequests is the maximum number of requests allowed to pass through
	// when the CircuitBreaker is half-open. Default: 1
	MaxRequests uint32

	// Interval is the cyclic period of the closed state for the CircuitBreaker
	// to clear the internal counts. If Interval is 0, it never clears. Default: 0
	Interval time.Duration

	// Timeout is the period of the open state after which the state becomes half-open.
	// Default: 60s
	Timeout time.Duration

	// ReadyToTrip is called with a copy of Counts whenever a request fails.
	// If ReadyToTrip returns true, the CircuitBreaker will be placed into the open state.
	// If nil, default threshold is used (5 consecutive failures).
	ReadyToTrip func(counts Counts) bool
}

// Counts holds the numbers of requests and their successes/failures.
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// DefaultResilientConfig returns sensible defaults for resilience configuration.
func DefaultResilientConfig() ResilientConfig {
	return ResilientConfig{
		Timeout: 5 * time.Second,
		CircuitBreakerConfig: CircuitBreakerConfig{
			MaxRequests: 5,
			Interval:    60 * time.Second,
			Timeout:     30 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				// Require at least 20 requests before considering error rate
				if counts.Requests < 20 {
					return false
				}
				// Trip if error rate >= 15%
				failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
				return failureRate >= 0.15
			},
		},
	}
}

// WithTimeout returns a copy of the config with the specified timeout.
func (c ResilientConfig) WithTimeout(timeout time.Duration) ResilientConfig {
	c.Timeout = timeout
	return c
}

// WithCircuitBreakerTimeout returns a copy of the config with the specified circuit breaker timeout.
func (c ResilientConfig) WithCircuitBreakerTimeout(timeout time.Duration) ResilientConfig {
	c.CircuitBreakerConfig.Timeout = timeout
	return c
}
