package mock

import (
	"context"
	"sync/atomic"
	"time"
)

// MockLayer is a mock implementation of CacheLayer for testing.
// It allows injecting custom behavior for each method and tracks call counts.
type MockLayer struct {
	// Function hooks - set these to customize behavior
	GetFunc    func(ctx context.Context, key string) (interface{}, error)
	SetFunc    func(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	DeleteFunc func(ctx context.Context, key string) error
	NameFunc   func() string
	CloseFunc  func() error

	// Call tracking (must use atomic operations for race-free access)
	getCalls    int64
	setCalls    int64
	deleteCalls int64
	closeCalls  int64
}

// Get implements CacheLayer.Get with optional custom behavior.
func (m *MockLayer) Get(ctx context.Context, key string) (interface{}, error) {
	atomic.AddInt64(&m.getCalls, 1)
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return nil, nil
}

// Set implements CacheLayer.Set with optional custom behavior.
func (m *MockLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	atomic.AddInt64(&m.setCalls, 1)
	if m.SetFunc != nil {
		return m.SetFunc(ctx, key, value, ttl)
	}
	return nil
}

// Delete implements CacheLayer.Delete with optional custom behavior.
func (m *MockLayer) Delete(ctx context.Context, key string) error {
	atomic.AddInt64(&m.deleteCalls, 1)
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, key)
	}
	return nil
}

// Name implements CacheLayer.Name with optional custom behavior.
func (m *MockLayer) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock"
}

// GetCalls returns the number of Get calls (thread-safe).
func (m *MockLayer) GetCalls() int {
	return int(atomic.LoadInt64(&m.getCalls))
}

// SetCalls returns the number of Set calls (thread-safe).
func (m *MockLayer) SetCalls() int {
	return int(atomic.LoadInt64(&m.setCalls))
}

// DeleteCalls returns the number of Delete calls (thread-safe).
func (m *MockLayer) DeleteCalls() int {
	return int(atomic.LoadInt64(&m.deleteCalls))
}

// CloseCalls returns the number of Close calls (thread-safe).
func (m *MockLayer) CloseCalls() int {
	return int(atomic.LoadInt64(&m.closeCalls))
}

// Close implements CacheLayer.Close with optional custom behavior.
func (m *MockLayer) Close() error {
	atomic.AddInt64(&m.closeCalls, 1)
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// NewMockLayer creates a new MockLayer with default behavior.
// By default, all operations succeed and return nil.
func NewMockLayer(name string) *MockLayer {
	return &MockLayer{
		NameFunc: func() string { return name },
	}
}

// NewMockLayerWithDefaults creates a MockLayer that behaves like a real cache.
// Get returns ErrKeyNotFound by default, Set/Delete succeed.
func NewMockLayerWithDefaults(name string) *MockLayer {
	return &MockLayer{
		NameFunc: func() string { return name },
		GetFunc: func(ctx context.Context, key string) (interface{}, error) {
			return nil, ErrKeyNotFound
		},
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			return nil
		},
		DeleteFunc: func(ctx context.Context, key string) error {
			return nil
		},
		CloseFunc: func() error {
			return nil
		},
	}
}

// ErrKeyNotFound is a mock error for key not found
var ErrKeyNotFound = &mockError{"key not found"}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return "mock: " + e.msg
}
