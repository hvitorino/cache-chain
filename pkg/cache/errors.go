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

// ClassifyError returns a string classification of the error type for metrics.
// This helps differentiate error types in observability dashboards.
func ClassifyError(err error) string {
	if err == nil {
		return "none"
	}

	switch {
	case errors.Is(err, ErrCircuitOpen):
		return "circuit_breaker_open"
	case errors.Is(err, ErrTimeout):
		return "timeout"
	case errors.Is(err, ErrKeyNotFound):
		return "key_not_found"
	case errors.Is(err, ErrLayerUnavailable):
		return "unavailable"
	case errors.Is(err, ErrInvalidKey):
		return "invalid_key"
	case errors.Is(err, ErrInvalidValue):
		return "invalid_value"
	default:
		// Check for common error patterns in the error message
		errStr := err.Error()
		switch {
		case contains(errStr, "connection", "connect", "dial"):
			return "connection"
		case contains(errStr, "serialize", "marshal", "unmarshal", "encode", "decode"):
			return "serialization"
		case contains(errStr, "redis", "memcache"):
			return "backend"
		default:
			return "other"
		}
	}
}

// contains checks if the string contains any of the given substrings (case-insensitive)
func contains(s string, substrs ...string) bool {
	lower := toLower(s)
	for _, substr := range substrs {
		if containsSubstr(lower, toLower(substr)) {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	// Simple ASCII lowercase
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func containsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && indexSubstr(s, substr) >= 0
}

func indexSubstr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
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
