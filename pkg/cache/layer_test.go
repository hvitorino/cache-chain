package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCacheEntry_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{"not expired", now.Add(time.Hour), false},
		{"expired", now.Add(-time.Hour), true},
		{"exactly now", now, true}, // time.Now() > now is true
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := CacheEntry{ExpiresAt: tt.expiresAt}
			result := entry.IsExpired()
			if result != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCacheEntry_TimeToLive(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		expiresAt time.Time
		expected  time.Duration
	}{
		{"not expired", now.Add(time.Hour), time.Hour - time.Minute}, // approximate
		{"expired", now.Add(-time.Hour), 0},
		{"exactly now", now, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := CacheEntry{ExpiresAt: tt.expiresAt}
			result := entry.TimeToLive()

			// For non-expired entries, check approximate range
			if tt.expected > 0 {
				if result <= 0 {
					t.Errorf("TimeToLive() = %v, want > 0", result)
				}
				// Don't check exact value due to timing
			} else {
				if result != 0 {
					t.Errorf("TimeToLive() = %v, want 0", result)
				}
			}
		})
	}
}

func TestCacheEntry_Fields(t *testing.T) {
	now := time.Now()
	entry := CacheEntry{
		Key:       "test-key",
		Value:     "test-value",
		ExpiresAt: now.Add(time.Hour),
		Version:   42,
		CreatedAt: now,
	}

	if entry.Key != "test-key" {
		t.Errorf("Key = %q, want %q", entry.Key, "test-key")
	}
	if entry.Value != "test-value" {
		t.Errorf("Value = %q, want %q", entry.Value, "test-value")
	}
	if entry.Version != 42 {
		t.Errorf("Version = %d, want %d", entry.Version, 42)
	}
	if entry.CreatedAt != now {
		t.Errorf("CreatedAt = %v, want %v", entry.CreatedAt, now)
	}
}

// MockCacheLayer is a test implementation of CacheLayer for testing
type MockCacheLayer struct {
	GetFunc    func(ctx context.Context, key string) (interface{}, error)
	SetFunc    func(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	DeleteFunc func(ctx context.Context, key string) error
	NameFunc   func() string
	CloseFunc  func() error

	// Call counters for testing
	GetCalls    int
	SetCalls    int
	DeleteCalls int
	CloseCalls  int
}

func (m *MockCacheLayer) Get(ctx context.Context, key string) (interface{}, error) {
	m.GetCalls++
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return nil, ErrKeyNotFound
}

func (m *MockCacheLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.SetCalls++
	if m.SetFunc != nil {
		return m.SetFunc(ctx, key, value, ttl)
	}
	return nil
}

func (m *MockCacheLayer) Delete(ctx context.Context, key string) error {
	m.DeleteCalls++
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, key)
	}
	return nil
}

func (m *MockCacheLayer) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock"
}

func (m *MockCacheLayer) Close() error {
	m.CloseCalls++
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestMockCacheLayer(t *testing.T) {
	mock := &MockCacheLayer{}

	// Test default behavior
	_, err := mock.Get(context.Background(), "key")
	if !IsNotFound(err) {
		t.Errorf("Get should return ErrKeyNotFound by default")
	}
	if mock.GetCalls != 1 {
		t.Errorf("GetCalls = %d, want 1", mock.GetCalls)
	}

	err = mock.Set(context.Background(), "key", "value", time.Minute)
	if err != nil {
		t.Errorf("Set should return nil by default")
	}
	if mock.SetCalls != 1 {
		t.Errorf("SetCalls = %d, want 1", mock.SetCalls)
	}

	err = mock.Delete(context.Background(), "key")
	if err != nil {
		t.Errorf("Delete should return nil by default")
	}
	if mock.DeleteCalls != 1 {
		t.Errorf("DeleteCalls = %d, want 1", mock.DeleteCalls)
	}

	if mock.Name() != "mock" {
		t.Errorf("Name() = %q, want %q", mock.Name(), "mock")
	}

	err = mock.Close()
	if err != nil {
		t.Errorf("Close should return nil by default")
	}
	if mock.CloseCalls != 1 {
		t.Errorf("CloseCalls = %d, want 1", mock.CloseCalls)
	}
}

func TestMockCacheLayer_CustomFuncs(t *testing.T) {
	mock := &MockCacheLayer{
		GetFunc: func(ctx context.Context, key string) (interface{}, error) {
			return "custom-value", nil
		},
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			return ErrInvalidValue
		},
		NameFunc: func() string {
			return "custom-mock"
		},
	}

	// Test custom GetFunc
	value, err := mock.Get(context.Background(), "key")
	if err != nil {
		t.Errorf("Get should not return error")
	}
	if value != "custom-value" {
		t.Errorf("Get returned %v, want %v", value, "custom-value")
	}

	// Test custom SetFunc
	err = mock.Set(context.Background(), "key", "value", time.Minute)
	if !errors.Is(err, ErrInvalidValue) {
		t.Errorf("Set should return ErrInvalidValue")
	}

	// Test custom NameFunc
	if mock.Name() != "custom-mock" {
		t.Errorf("Name() = %q, want %q", mock.Name(), "custom-mock")
	}
}
