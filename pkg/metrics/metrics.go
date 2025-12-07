package metrics

import (
	"time"
)

// MetricsCollector defines the interface for collecting cache metrics.
// Implementations can export metrics to various backends (Prometheus, StatsD, etc.).
type MetricsCollector interface {
	// Cache operations
	RecordGet(layer string, hit bool, duration time.Duration)
	RecordSet(layer string, success bool, duration time.Duration)
	RecordDelete(layer string, success bool, duration time.Duration)

	// Circuit breaker
	RecordCircuitState(layer string, state CircuitState)

	// Async writer
	RecordQueueDepth(layer string, depth int)
	RecordWriteDropped(layer string)
	RecordAsyncWrite(layer string, success bool, duration time.Duration)

	// Chain-level
	RecordChainGet(hit bool, layerIndex int, totalDuration time.Duration)
}

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	// CircuitClosed means the circuit breaker is allowing requests through.
	CircuitClosed CircuitState = iota
	// CircuitOpen means the circuit breaker is blocking requests.
	CircuitOpen
	// CircuitHalfOpen means the circuit breaker is testing if the service has recovered.
	CircuitHalfOpen
)

// String returns the string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// NoOpCollector is a no-op implementation of MetricsCollector.
// It's used as the default collector when metrics are not needed.
type NoOpCollector struct{}

// RecordGet does nothing.
func (NoOpCollector) RecordGet(layer string, hit bool, duration time.Duration) {}

// RecordSet does nothing.
func (NoOpCollector) RecordSet(layer string, success bool, duration time.Duration) {}

// RecordDelete does nothing.
func (NoOpCollector) RecordDelete(layer string, success bool, duration time.Duration) {}

// RecordCircuitState does nothing.
func (NoOpCollector) RecordCircuitState(layer string, state CircuitState) {}

// RecordQueueDepth does nothing.
func (NoOpCollector) RecordQueueDepth(layer string, depth int) {}

// RecordWriteDropped does nothing.
func (NoOpCollector) RecordWriteDropped(layer string) {}

// RecordAsyncWrite does nothing.
func (NoOpCollector) RecordAsyncWrite(layer string, success bool, duration time.Duration) {}

// RecordChainGet does nothing.
func (NoOpCollector) RecordChainGet(hit bool, layerIndex int, totalDuration time.Duration) {}
