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

	if config.CircuitBreakerConfig.MaxRequests != 5 {
		t.Errorf("Expected MaxRequests 5, got %d", config.CircuitBreakerConfig.MaxRequests)
	}

	if config.CircuitBreakerConfig.Interval != 60*time.Second {
		t.Errorf("Expected Interval 60s, got %v", config.CircuitBreakerConfig.Interval)
	}

	if config.CircuitBreakerConfig.Timeout != 30*time.Second {
		t.Errorf("Expected CB timeout 30s, got %v", config.CircuitBreakerConfig.Timeout)
	}

	if config.CircuitBreakerConfig.ReadyToTrip == nil {
		t.Error("Expected ReadyToTrip function to be set")
	}

	// Test ReadyToTrip function with error rate threshold
	// Should not trip with < 20 requests
	if config.CircuitBreakerConfig.ReadyToTrip(Counts{Requests: 10, TotalFailures: 5}) {
		t.Error("Should not trip with < 20 requests")
	}

	// Should not trip with error rate < 15% (2 failures in 20 requests = 10%)
	if config.CircuitBreakerConfig.ReadyToTrip(Counts{Requests: 20, TotalFailures: 2}) {
		t.Error("Should not trip with 10% error rate")
	}

	// Should trip with error rate >= 15% (3 failures in 20 requests = 15%)
	if !config.CircuitBreakerConfig.ReadyToTrip(Counts{Requests: 20, TotalFailures: 3}) {
		t.Error("Should trip with 15% error rate")
	}

	// Should trip with error rate > 15% (10 failures in 50 requests = 20%)
	if !config.CircuitBreakerConfig.ReadyToTrip(Counts{Requests: 50, TotalFailures: 10}) {
		t.Error("Should trip with 20% error rate")
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
	if config.CircuitBreakerConfig.Timeout != 30*time.Second {
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
