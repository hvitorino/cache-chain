# Phase 2: In-Memory Cache Layer Implementation

## Objective
Implement a production-ready in-memory cache layer using the interfaces defined in Phase 1, with proper concurrency control and TTL expiration.

## Scope
- Implement `MemoryCache` that satisfies `CacheLayer` interface
- Thread-safe operations using sync.RWMutex
- Automatic TTL expiration with background cleanup
- LRU eviction when max size is reached
- Comprehensive unit tests including concurrent access

## Requirements

### 1. MemoryCache Implementation
Create `pkg/cache/memory/memory.go`:

```go
type MemoryCache struct {
    // Thread-safe map for storage
    // Background goroutine for cleanup
    // Configuration (max size, cleanup interval)
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(config MemoryCacheConfig) *MemoryCache

type MemoryCacheConfig struct {
    Name            string
    MaxSize         int           // Maximum number of entries (0 = unlimited)
    DefaultTTL      time.Duration
    CleanupInterval time.Duration // How often to check for expired entries
}
```

### 2. Core Operations
Implement all `CacheLayer` interface methods:

- **Get**: Thread-safe read, check expiration, update access time (for LRU)
- **Set**: Thread-safe write, enforce max size, set expiration
- **Delete**: Thread-safe removal
- **Name**: Return configured name
- **Close**: Stop cleanup goroutine, clear data

### 3. TTL Management
- Store entry expiration time
- Background goroutine runs cleanup at configured interval
- Lazy expiration: check on Get() before returning
- Cleanup should not block operations

### 4. Eviction Policy (LRU)
When MaxSize is reached:
- Track last access time for each entry
- Evict least recently used entry
- Update access time on Get()

### 5. Internal Structure
```go
type entry struct {
    value      interface{}
    expiresAt  time.Time
    accessedAt time.Time
    version    int64
}
```

## File Structure
```
pkg/
  cache/
    memory/
      memory.go       # MemoryCache implementation
      memory_test.go  # Unit tests
      lru.go          # LRU eviction logic
      lru_test.go     # LRU tests
```

## Testing Requirements

### Test Cases
1. **Basic Operations**
   - Set and Get value
   - Get non-existent key returns ErrKeyNotFound
   - Delete removes key
   - Close stops background cleanup

2. **TTL Expiration**
   - Expired entries not returned by Get
   - Background cleanup removes expired entries
   - Set with 0 TTL uses default TTL

3. **Concurrency**
   - Concurrent Gets are safe
   - Concurrent Sets are safe
   - Concurrent Get/Set/Delete combinations are safe
   - Use `go test -race` to verify

4. **Eviction**
   - MaxSize enforced (evict LRU when full)
   - Access updates LRU ordering
   - Most recently used entries retained

5. **Edge Cases**
   - Set nil value
   - Set with negative TTL
   - Get after Close returns error
   - Cleanup goroutine stops after Close

### Test Example
```go
func TestMemoryCache_ConcurrentAccess(t *testing.T) {
    cache := NewMemoryCache(MemoryCacheConfig{
        Name:       "test",
        MaxSize:    100,
        DefaultTTL: time.Minute,
    })
    defer cache.Close()
    
    var wg sync.WaitGroup
    ctx := context.Background()
    
    // 100 goroutines writing
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            key := fmt.Sprintf("key-%d", id)
            cache.Set(ctx, key, id, time.Minute)
        }(i)
    }
    
    // 100 goroutines reading
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            key := fmt.Sprintf("key-%d", id)
            cache.Get(ctx, key)
        }(i)
    }
    
    wg.Wait()
}
```

### Benchmarks
Include basic benchmarks:
```go
func BenchmarkMemoryCache_Get(b *testing.B)
func BenchmarkMemoryCache_Set(b *testing.B)
func BenchmarkMemoryCache_ConcurrentGet(b *testing.B)
```

## Acceptance Criteria
- [ ] MemoryCache implements CacheLayer interface
- [ ] All operations are thread-safe (verified with -race flag)
- [ ] TTL expiration works (both lazy and background cleanup)
- [ ] LRU eviction works when MaxSize reached
- [ ] Background cleanup goroutine starts and stops properly
- [ ] Unit tests with >90% coverage
- [ ] Concurrent access tests pass
- [ ] Benchmarks included
- [ ] All tests passing: `go test -race ./pkg/cache/memory/...`
- [ ] Code follows Go instructions
- [ ] Godoc comments on all exported types/functions

## Notes
- Use `sync.RWMutex` for read/write locking
- Background cleanup should use `time.Ticker`
- Use `context.Context` for cancellation in cleanup goroutine
- Consider using `sync.Map` if contention becomes an issue (document decision)
- MaxSize=0 means unlimited capacity

## Next Phase
Phase 3 will implement the chain mechanism to connect multiple cache layers.
