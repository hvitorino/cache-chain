package cache

import (
	"errors"
	"fmt"
)

// Common cache operation errors.
// These are the standard errors that cache implementations should return.
var (
	// ErrKeyNotFound is returned when a requested key does not exist in the cache
	ErrKeyNotFound = errors.New("cache: key not found")

	// ErrCacheMiss is an alias for ErrKeyNotFound, commonly used in cache implementations
	ErrCacheMiss = ErrKeyNotFound

	// ErrInvalidKey is returned when a cache key is invalid (empty, too long, contains invalid characters)
	ErrInvalidKey = errors.New("cache: invalid key")

	// ErrInvalidValue is returned when a cache value is invalid or cannot be stored
	ErrInvalidValue = errors.New("cache: invalid value")

	// ErrLayerUnavailable is returned when a cache layer is temporarily unavailable
	ErrLayerUnavailable = errors.New("cache: layer unavailable")

	// ErrTimeout is returned when a cache operation times out
	ErrTimeout = errors.New("cache: operation timeout")

	// ErrCircuitOpen is returned when the circuit breaker is in open state
	ErrCircuitOpen = errors.New("cache: circuit breaker open")
)

// IsNotFound checks if the given error indicates that a key was not found.
// This is a convenience function for checking cache miss errors.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrKeyNotFound)
}

// IsTimeout checks if the given error indicates a timeout occurred.
// This is a convenience function for checking timeout errors.
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsUnavailable checks if the given error indicates a layer is unavailable.
// This is a convenience function for checking layer availability.
func IsUnavailable(err error) bool {
	return errors.Is(err, ErrLayerUnavailable)
}

// IsCircuitOpen checks if the given error indicates the circuit breaker is open.
// This is a convenience function for checking circuit breaker state.
func IsCircuitOpen(err error) bool {
	return errors.Is(err, ErrCircuitOpen)
}

// WrapError wraps an error with additional context about the cache operation.
// This is useful for adding layer-specific information to errors.
func WrapError(err error, layer string, operation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cache layer %s %s: %w", layer, operation, err)
}
