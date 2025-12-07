# Phase 1: Core Interfaces and Types

## Objective
Define the foundational interfaces and types that will serve as the contract for all cache layers and components in the library.

## Scope
- Create core `CacheLayer` interface
- Define `CacheEntry` type for value storage
- Define `CacheConfig` for layer configuration
- Define error types specific to cache operations
- Add comprehensive unit tests

## Requirements

### 1. CacheLayer Interface
Create `pkg/cache/layer.go` with the core interface:

```go
type CacheLayer interface {
    // Get retrieves a value from the cache
    Get(ctx context.Context, key string) (interface{}, error)
    
    // Set stores a value in the cache
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    
    // Delete removes a value from the cache
    Delete(ctx context.Context, key string) error
    
    // Name returns the layer identifier (e.g., "L1", "redis", "database")
    Name() string
    
    // Close releases resources
    Close() error
}
```

### 2. CacheEntry Type
Define a wrapper for cached values that includes metadata:

```go
type CacheEntry struct {
    Key       string
    Value     interface{}
    ExpiresAt time.Time
    Version   int64  // For versioning support
    CreatedAt time.Time
}
```

### 3. LayerConfig Type
Configuration for individual cache layers:

```go
type LayerConfig struct {
    Name            string
    DefaultTTL      time.Duration
    MaxTTL          time.Duration
    Enabled         bool
}
```

### 4. Error Types
Define specific error types in `pkg/cache/errors.go`:

```go
var (
    ErrKeyNotFound     = errors.New("cache: key not found")
    ErrInvalidKey      = errors.New("cache: invalid key")
    ErrInvalidValue    = errors.New("cache: invalid value")
    ErrLayerUnavailable = errors.New("cache: layer unavailable")
    ErrTimeout          = errors.New("cache: operation timeout")
)

// IsNotFound checks if error is a key not found error
func IsNotFound(err error) bool

// IsTimeout checks if error is a timeout error
func IsTimeout(err error) bool
```

### 5. Key Validation
Add basic key validation utilities in `pkg/cache/keys.go`:

```go
// ValidateKey checks if a cache key is valid
func ValidateKey(key string) error

// Rules:
// - Non-empty string
// - Max length 250 characters
// - No control characters
```

## File Structure
```
pkg/
  cache/
    layer.go       # CacheLayer interface and CacheEntry
    config.go      # LayerConfig
    errors.go      # Error types and helpers
    keys.go        # Key validation
    layer_test.go  # Interface tests (test helpers)
    errors_test.go # Error type tests
    keys_test.go   # Key validation tests
```

## Testing Requirements

### Test Coverage
- Key validation: valid/invalid keys, edge cases (empty, too long, control chars)
- Error type checking: IsNotFound, IsTimeout functions
- CacheEntry: creation, field access
- LayerConfig: validation, defaults

### Test Example
```go
func TestValidateKey(t *testing.T) {
    tests := []struct {
        name    string
        key     string
        wantErr bool
    }{
        {"valid key", "user:123", false},
        {"empty key", "", true},
        {"too long", strings.Repeat("a", 300), true},
        {"control char", "key\x00value", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateKey(tt.key)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateKey() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Acceptance Criteria
- [ ] All interfaces defined with clear documentation
- [ ] All types defined with appropriate fields
- [ ] Error types created with helper functions
- [ ] Key validation implemented
- [ ] Unit tests written with >90% coverage
- [ ] All tests passing: `go test ./pkg/cache/...`
- [ ] Code follows Go instructions in `.github/instructions/go.instructions.md`
- [ ] No external dependencies (standard library only)

## Notes
- Keep interfaces minimal - we'll extend in later phases
- Focus on clarity over premature optimization
- All exported types/functions must have godoc comments
- Use `context.Context` consistently for cancellation support

## Next Phase
Phase 2 will implement a basic in-memory cache layer using these interfaces.
