package resilience

import (
	"testing"
	"time"
)

func TestDefaultResilientConfig(t *testing.T) {
	config := DefaultResilientConfig()

	if config.Timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", config.Timeout)
	}

	if config.CircuitBreakerConfig.MaxRequests != 1 {
		t.Errorf("Expected MaxRequests 1, got %d", config.CircuitBreakerConfig.MaxRequests)
	}

	if config.CircuitBreakerConfig.Timeout != 10*time.Second {
		t.Errorf("Expected CB timeout 10s, got %v", config.CircuitBreakerConfig.Timeout)
	}

	if config.CircuitBreakerConfig.ReadyToTrip == nil {
		t.Error("Expected ReadyToTrip function to be set")
	}

	// Test ReadyToTrip function
	if config.CircuitBreakerConfig.ReadyToTrip(Counts{ConsecutiveFailures: 4}) {
		t.Error("Should not trip with 4 failures")
	}

	if !config.CircuitBreakerConfig.ReadyToTrip(Counts{ConsecutiveFailures: 5}) {
		t.Error("Should trip with 5 failures")
	}
}

func TestResilientConfig_WithTimeout(t *testing.T) {
	config := DefaultResilientConfig()
	newConfig := config.WithTimeout(2 * time.Second)

	if newConfig.Timeout != 2*time.Second {
		t.Errorf("Expected timeout 2s, got %v", newConfig.Timeout)
	}

	// Verify original is unchanged
	if config.Timeout != 5*time.Second {
		t.Errorf("Original config changed: got %v", config.Timeout)
	}
}

func TestResilientConfig_WithCircuitBreakerTimeout(t *testing.T) {
	config := DefaultResilientConfig()
	newConfig := config.WithCircuitBreakerTimeout(20 * time.Second)

	if newConfig.CircuitBreakerConfig.Timeout != 20*time.Second {
		t.Errorf("Expected CB timeout 20s, got %v", newConfig.CircuitBreakerConfig.Timeout)
	}

	// Verify original is unchanged
	if config.CircuitBreakerConfig.Timeout != 10*time.Second {
		t.Errorf("Original config changed: got %v", config.CircuitBreakerConfig.Timeout)
	}
}

func TestCounts_Fields(t *testing.T) {
	counts := Counts{
		Requests:             100,
		TotalSuccesses:       80,
		TotalFailures:        20,
		ConsecutiveSuccesses: 5,
		ConsecutiveFailures:  0,
	}

	if counts.Requests != 100 {
		t.Errorf("Expected Requests 100, got %d", counts.Requests)
	}

	if counts.TotalSuccesses != 80 {
		t.Errorf("Expected TotalSuccesses 80, got %d", counts.TotalSuccesses)
	}

	if counts.TotalFailures != 20 {
		t.Errorf("Expected TotalFailures 20, got %d", counts.TotalFailures)
	}

	if counts.ConsecutiveSuccesses != 5 {
		t.Errorf("Expected ConsecutiveSuccesses 5, got %d", counts.ConsecutiveSuccesses)
	}

	if counts.ConsecutiveFailures != 0 {
		t.Errorf("Expected ConsecutiveFailures 0, got %d", counts.ConsecutiveFailures)
	}
}
