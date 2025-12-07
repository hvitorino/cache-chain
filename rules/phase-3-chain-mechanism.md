# Phase 3: Chain Mechanism and Fallback

## Objective
Implement the core chain mechanism that connects N cache layers with automatic fallback and synchronous warm-up. This phase focuses on the chain traversal logic without async operations.

## Scope
- Create `Chain` type that manages multiple `CacheLayer` instances
- Implement fallback logic: L1 → L2 → ... → LN
- Implement synchronous warm-up of upper layers on cache hit
- Single-flight pattern to prevent thundering herd
- Comprehensive unit tests with mock layers

## Requirements

### 1. Chain Implementation
Create `pkg/chain/chain.go`:

```go
type Chain struct {
    layers []CacheLayer
    sf     *singleflight.Group  // Prevent concurrent gets for same key
}

// New creates a new chain of cache layers
// Layers are ordered from fastest (L1) to slowest (LN)
func New(layers ...CacheLayer) (*Chain, error)

// Get retrieves a value from the chain
// Traverses layers in order until hit, then warms upper layers
func (c *Chain) Get(ctx context.Context, key string) (interface{}, error)

// Set writes to all layers in the chain
func (c *Chain) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

// Delete removes key from all layers
func (c *Chain) Delete(ctx context.Context, key string) error

// Close closes all layers
func (c *Chain) Close() error
```

### 2. Single-Flight Pattern
Use `golang.org/x/sync/singleflight` to prevent duplicate Gets:

```go
// When multiple goroutines call Get() for same key simultaneously:
// - Only one actually executes the retrieval
// - Others wait and receive the same result
// - Prevents cache stampede
```

### 3. Fallback Logic
```
Get(key):
1. Check L1
   - Hit: return value
   - Miss: continue to L2
   
2. Check L2
   - Hit: warm L1 (sync for now), return value
   - Miss: continue to L3
   
3. Check L3
   - Hit: warm L2 and L1 (sync for now), return value
   - Miss: return ErrKeyNotFound
```

### 4. Warm-Up Strategy
For this phase, warm-up is **synchronous**:
- When L2 hits, immediately Set() to L1 before returning
- When L3 hits, immediately Set() to L2, then L1 before returning
- Warm-up failures are logged but don't fail the operation
- Use same TTL as the source layer (or configured default)

Phase 4 will convert this to async.

### 5. Error Handling
- Layer unavailable: skip to next layer (don't fail entire operation)
- Context cancellation: stop chain traversal immediately
- Timeout: respect context deadline
- All layers fail: return last error wrapped with context

## File Structure
```
pkg/
  chain/
    chain.go       # Chain implementation
    chain_test.go  # Unit tests with mock layers
    warmup.go      # Warm-up logic (sync for now)
    warmup_test.go # Warm-up tests
```

## Testing Requirements

### Test Cases
1. **Basic Chain Operations**
   - L1 hit returns immediately
   - L1 miss, L2 hit returns value and warms L1
   - All misses return ErrKeyNotFound
   - Set writes to all layers
   - Delete removes from all layers

2. **Warm-Up**
   - L2 hit warms L1 with correct value and TTL
   - L3 hit warms L2 and L1 in order
   - Warm-up failure doesn't affect Get success
   - Verify warm-up writes correct data

3. **Single-Flight**
   - Concurrent Gets for same key call underlying layer once
   - All waiters receive same result
   - Different keys execute independently

4. **Error Handling**
   - Layer returns error: skip to next layer
   - Context cancellation stops traversal
   - All layers fail: return appropriate error
   - Partial layer failures don't break chain

5. **Edge Cases**
   - Empty chain returns error
   - Single layer chain works
   - Close closes all layers
   - Get after Close returns error

### Mock Layer
Create a mock implementation for testing:

```go
// pkg/cache/mock/mock.go
type MockLayer struct {
    GetFunc    func(ctx context.Context, key string) (interface{}, error)
    SetFunc    func(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    DeleteFunc func(ctx context.Context, key string) error
    NameFunc   func() string
    CloseFunc  func() error
    
    // Call tracking
    GetCalls    int
    SetCalls    int
    DeleteCalls int
}
```

### Test Example
```go
func TestChain_FallbackAndWarmup(t *testing.T) {
    // L1: miss
    l1 := &mock.MockLayer{
        NameFunc: func() string { return "L1" },
        GetFunc: func(ctx context.Context, key string) (interface{}, error) {
            return nil, cache.ErrKeyNotFound
        },
        SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
            // Verify warm-up writes here
            if value != "data-from-l2" {
                t.Errorf("L1 warm-up: expected 'data-from-l2', got %v", value)
            }
            return nil
        },
    }
    
    // L2: hit
    l2 := &mock.MockLayer{
        NameFunc: func() string { return "L2" },
        GetFunc: func(ctx context.Context, key string) (interface{}, error) {
            return "data-from-l2", nil
        },
    }
    
    chain, err := New(l1, l2)
    if err != nil {
        t.Fatal(err)
    }
    defer chain.Close()
    
    value, err := chain.Get(context.Background(), "test-key")
    if err != nil {
        t.Fatalf("Get failed: %v", err)
    }
    
    if value != "data-from-l2" {
        t.Errorf("expected 'data-from-l2', got %v", value)
    }
    
    // Verify L1 was warmed up
    if l1.SetCalls != 1 {
        t.Errorf("L1 should be warmed up once, got %d calls", l1.SetCalls)
    }
}
```

## Acceptance Criteria
- [ ] Chain supports N layers (tested with 1, 2, 3, 5 layers)
- [ ] Fallback logic works correctly
- [ ] Synchronous warm-up implemented and tested
- [ ] Single-flight pattern prevents duplicate Gets
- [ ] Error handling for layer failures
- [ ] Context cancellation respected
- [ ] Mock layer implementation for testing
- [ ] Unit tests with >90% coverage
- [ ] All tests passing: `go test ./pkg/chain/...`
- [ ] Code follows Go instructions
- [ ] Godoc comments on all exported types/functions

## Notes
- This phase implements **synchronous** warm-up to keep it simple and testable
- Phase 4 will add async writers for non-blocking warm-up
- Use `golang.org/x/sync/singleflight` - first external dependency
- Update go.mod: `go get golang.org/x/sync/singleflight`
- Consider exposing warm-up errors through logging or metrics (Phase 6)

## Integration Test
After Phase 3, you should be able to:

```go
l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1"})
l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2"})
l3 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L3"})

chain, _ := chain.New(l1, l2, l3)

// Populate L3
l3.Set(ctx, "key", "value", time.Minute)

// Get from chain: L1 miss → L2 miss → L3 hit → warm L2 & L1
value, _ := chain.Get(ctx, "key")
```

## Next Phase
Phase 4 will add async writer queues for non-blocking warm-up operations.
