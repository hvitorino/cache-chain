package cache

import (
	"errors"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrKeyNotFound", ErrKeyNotFound, true},
		{"wrapped ErrKeyNotFound", WrapError(ErrKeyNotFound, "L1", "get"), true},
		{"other error", ErrInvalidKey, false},
		{"nil error", nil, false},
		{"custom error", errors.New("custom"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFound(tt.err)
			if result != tt.expected {
				t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrTimeout", ErrTimeout, true},
		{"wrapped ErrTimeout", WrapError(ErrTimeout, "redis", "set"), true},
		{"other error", ErrKeyNotFound, false},
		{"nil error", nil, false},
		{"custom error", errors.New("network timeout"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTimeout(tt.err)
			if result != tt.expected {
				t.Errorf("IsTimeout(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsUnavailable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ErrLayerUnavailable", ErrLayerUnavailable, true},
		{"wrapped ErrLayerUnavailable", WrapError(ErrLayerUnavailable, "database", "connect"), true},
		{"other error", ErrInvalidValue, false},
		{"nil error", nil, false},
		{"custom error", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsUnavailable(tt.err)
			if result != tt.expected {
				t.Errorf("IsUnavailable(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		layer     string
		operation string
		expected  string
	}{
		{
			"wrap ErrKeyNotFound",
			ErrKeyNotFound,
			"L1",
			"get",
			"cache layer L1 get: cache: key not found",
		},
		{
			"wrap ErrTimeout",
			ErrTimeout,
			"redis",
			"set",
			"cache layer redis set: cache: operation timeout",
		},
		{
			"nil error returns nil",
			nil,
			"database",
			"query",
			"", // empty string for nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapError(tt.err, tt.layer, tt.operation)
			if tt.err == nil {
				if result != nil {
					t.Errorf("WrapError(nil) = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Error("WrapError should not return nil for non-nil error")
				return
			}

			if result.Error() != tt.expected {
				t.Errorf("WrapError() = %q, want %q", result.Error(), tt.expected)
			}

			// Verify the wrapped error can still be identified
			if !errors.Is(result, tt.err) {
				t.Errorf("WrapError should preserve original error for errors.Is()")
			}
		})
	}
}

func TestErrorVariables(t *testing.T) {
	// Test that error variables are properly defined
	if ErrKeyNotFound == nil {
		t.Error("ErrKeyNotFound should not be nil")
	}
	if ErrInvalidKey == nil {
		t.Error("ErrInvalidKey should not be nil")
	}
	if ErrInvalidValue == nil {
		t.Error("ErrInvalidValue should not be nil")
	}
	if ErrLayerUnavailable == nil {
		t.Error("ErrLayerUnavailable should not be nil")
	}
	if ErrTimeout == nil {
		t.Error("ErrTimeout should not be nil")
	}

	// Test error messages
	expectedMessages := map[error]string{
		ErrKeyNotFound:      "cache: key not found",
		ErrInvalidKey:       "cache: invalid key",
		ErrInvalidValue:     "cache: invalid value",
		ErrLayerUnavailable: "cache: layer unavailable",
		ErrTimeout:          "cache: operation timeout",
	}

	for err, expected := range expectedMessages {
		if err.Error() != expected {
			t.Errorf("Error message for %v = %q, want %q", err, err.Error(), expected)
		}
	}
}
