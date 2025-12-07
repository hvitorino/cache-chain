# Phase 8: Advanced Features and Optimizations

## Objective
Implement advanced caching features that enhance performance and reliability: negative caching, bloom filters for quick misses, batch operations, and TTL hierarchies.

## Scope
- Negative caching (cache "not found" results)
- Bloom filters for probabilistic miss detection
- Batch operations (GetMulti, SetMulti, DeleteMulti)
- TTL hierarchy support (different TTLs per layer)
- Cache warming strategies
- Comprehensive unit tests and benchmarks

## Requirements

### 1. Negative Caching
Create `pkg/cache/negative.go`:

```go
// NegativeEntry represents a cached "not found" result
type NegativeEntry struct {
    Key       string
    CachedAt  time.Time
    ExpiresAt time.Time
}

// NegativeCacheLayer wraps a layer with negative caching
type NegativeCacheLayer struct {
    layer       CacheLayer
    negativeMap map[string]NegativeEntry
    negativeTTL time.Duration
    mu          sync.RWMutex
}

func NewNegativeCacheLayer(layer CacheLayer, negativeTTL time.Duration) *NegativeCacheLayer

func (ncl *NegativeCacheLayer) Get(ctx context.Context, key string) (interface{}, error) {
    // Check negative cache first
    if ncl.isNegativeCached(key) {
        return nil, cache.ErrKeyNotFound
    }
    
    value, err := ncl.layer.Get(ctx, key)
    if errors.Is(err, cache.ErrKeyNotFound) {
        // Cache the negative result
        ncl.cacheNegative(key)
        return nil, err
    }
    
    return value, err
}

// Background cleanup for expired negative entries
func (ncl *NegativeCacheLayer) cleanup()
```

### 2. Bloom Filter Integration
Create `pkg/cache/bloom/bloom.go`:

```go
// BloomLayer adds probabilistic membership testing
type BloomLayer struct {
    layer  CacheLayer
    filter *bloom.BloomFilter
    mu     sync.RWMutex
}

func NewBloomLayer(layer CacheLayer, expectedItems uint, falsePositiveRate float64) *BloomLayer

func (bl *BloomLayer) Get(ctx context.Context, key string) (interface{}, error) {
    // Quick rejection if definitely not in cache
    if !bl.filter.Test([]byte(key)) {
        return nil, cache.ErrKeyNotFound
    }
    
    // Might be in cache (or false positive)
    return bl.layer.Get(ctx, key)
}

func (bl *BloomLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    // Add to bloom filter
    bl.filter.Add([]byte(key))
    return bl.layer.Set(ctx, key, value, ttl)
}

// Reset periodically to prevent saturation
func (bl *BloomLayer) Reset()
```

### 3. Batch Operations
Extend `CacheLayer` interface:

```go
// BatchCacheLayer extends CacheLayer with batch operations
type BatchCacheLayer interface {
    CacheLayer
    
    // GetMulti retrieves multiple keys at once
    GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error)
    
    // SetMulti stores multiple key-value pairs at once
    SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error
    
    // DeleteMulti removes multiple keys at once
    DeleteMulti(ctx context.Context, keys []string) error
}

// BatchAdapter wraps non-batch layer with batch operations
type BatchAdapter struct {
    layer CacheLayer
}

func NewBatchAdapter(layer CacheLayer) *BatchAdapter

// Implements batch ops by calling single ops in parallel
func (ba *BatchAdapter) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
    results := make(map[string]interface{})
    var mu sync.Mutex
    var wg sync.WaitGroup
    
    for _, key := range keys {
        wg.Add(1)
        go func(k string) {
            defer wg.Done()
            if value, err := ba.layer.Get(ctx, k); err == nil {
                mu.Lock()
                results[k] = value
                mu.Unlock()
            }
        }(key)
    }
    
    wg.Wait()
    return results, nil
}
```

### 4. TTL Hierarchy
Create `pkg/chain/ttl.go`:

```go
// TTLStrategy determines TTL for each layer
type TTLStrategy interface {
    // GetTTL returns the TTL for a specific layer index
    GetTTL(layerIndex int, baseTTL time.Duration) time.Duration
}

// DecayingTTLStrategy: upper layers have shorter TTL
type DecayingTTLStrategy struct {
    decayFactor float64  // e.g., 0.5 means each layer has half the TTL of next
}

func (s *DecayingTTLStrategy) GetTTL(layerIndex int, baseTTL time.Duration) time.Duration {
    // L1: baseTTL * (decayFactor ^ 2)
    // L2: baseTTL * decayFactor
    // L3: baseTTL
    factor := math.Pow(s.decayFactor, float64(layerIndex))
    return time.Duration(float64(baseTTL) * factor)
}

// UniformTTLStrategy: same TTL for all layers
type UniformTTLStrategy struct{}

func (s *UniformTTLStrategy) GetTTL(layerIndex int, baseTTL time.Duration) time.Duration {
    return baseTTL
}

// CustomTTLStrategy: explicit TTL per layer
type CustomTTLStrategy struct {
    ttls []time.Duration
}

func (s *CustomTTLStrategy) GetTTL(layerIndex int, baseTTL time.Duration) time.Duration {
    if layerIndex < len(s.ttls) {
        return s.ttls[layerIndex]
    }
    return baseTTL
}
```

### 5. Cache Warming Strategies
Create `pkg/chain/warming.go`:

```go
// WarmingStrategy controls how cache warming happens
type WarmingStrategy interface {
    // ShouldWarm decides if we should warm a layer
    ShouldWarm(ctx context.Context, layerIndex int, key string, value interface{}) bool
}

// AlwaysWarmStrategy: warm all upper layers (default)
type AlwaysWarmStrategy struct{}

// SelectiveWarmStrategy: warm only based on criteria
type SelectiveWarmStrategy struct {
    minValueSize int    // Only warm if value size exceeds this
    costThreshold float64  // Only warm expensive-to-fetch items
}

// PredictiveWarmStrategy: use access patterns to predict
type PredictiveWarmStrategy struct {
    accessLog map[string]int  // Track access frequency
    threshold int              // Warm if accessed >= threshold times
}

func (s *PredictiveWarmStrategy) ShouldWarm(ctx context.Context, layerIndex int, key string, value interface{}) bool {
    s.accessLog[key]++
    return s.accessLog[key] >= s.threshold
}
```

### 6. Enhanced Chain Configuration
Extend `ChainConfig`:

```go
type ChainConfig struct {
    Layers            []CacheLayer
    MetricsCollector  metrics.MetricsCollector
    EntryValidator    validator.Validator
    ValidationPolicy  validator.ValidationPolicy
    TTLStrategy       TTLStrategy        // NEW
    WarmingStrategy   WarmingStrategy    // NEW
    EnableNegativeCache bool              // NEW
    NegativeCacheTTL   time.Duration     // NEW
    EnableBloomFilter  bool               // NEW
}
```

## File Structure
```
pkg/
  cache/
    negative.go          # Negative caching
    negative_test.go     # Tests
    batch.go             # Batch operations
    batch_test.go        # Batch tests
    bloom/
      bloom.go           # Bloom filter layer
      bloom_test.go      # Tests
  chain/
    ttl.go               # TTL strategies
    ttl_test.go          # Tests
    warming.go           # Warming strategies
    warming_test.go      # Tests
examples/
  advanced/
    negative_cache.go    # Example usage
    bloom_filter.go      # Example usage
    batch_ops.go         # Example usage
    ttl_hierarchy.go     # Example usage
```

## Testing Requirements

### Test Cases

#### Negative Caching
```go
func TestNegativeCacheLayer(t *testing.T) {
    mock := &mock.MockLayer{
        GetFunc: func(ctx context.Context, key string) (interface{}, error) {
            return nil, cache.ErrKeyNotFound
        },
    }
    
    ncl := NewNegativeCacheLayer(mock, time.Second)
    defer ncl.Close()
    
    // First get: miss, cache negative
    _, err := ncl.Get(context.Background(), "missing-key")
    if !errors.Is(err, cache.ErrKeyNotFound) {
        t.Error("Should return not found")
    }
    
    if mock.GetCalls != 1 {
        t.Error("Should call underlying layer once")
    }
    
    // Second get: should use negative cache
    _, err = ncl.Get(context.Background(), "missing-key")
    if !errors.Is(err, cache.ErrKeyNotFound) {
        t.Error("Should return not found")
    }
    
    if mock.GetCalls != 1 {
        t.Error("Should NOT call underlying layer again (negative cached)")
    }
}
```

#### Bloom Filter
```go
func TestBloomLayer(t *testing.T) {
    mock := &mock.MockLayer{
        GetFunc: func(ctx context.Context, key string) (interface{}, error) {
            return nil, cache.ErrKeyNotFound
        },
    }
    
    bl := NewBloomLayer(mock, 1000, 0.01)
    
    // Get non-existent key (not in bloom filter)
    _, err := bl.Get(context.Background(), "never-set")
    if !errors.Is(err, cache.ErrKeyNotFound) {
        t.Error("Should return not found")
    }
    
    // Should NOT have called underlying layer (bloom filter rejected)
    if mock.GetCalls > 0 {
        t.Error("Bloom filter should prevent underlying call")
    }
}
```

#### Batch Operations
```go
func TestBatchAdapter_GetMulti(t *testing.T) {
    l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1"})
    
    // Set some values
    l1.Set(context.Background(), "key1", "value1", time.Minute)
    l1.Set(context.Background(), "key2", "value2", time.Minute)
    
    adapter := NewBatchAdapter(l1)
    
    results, err := adapter.GetMulti(context.Background(), []string{"key1", "key2", "key3"})
    if err != nil {
        t.Fatalf("GetMulti failed: %v", err)
    }
    
    if len(results) != 2 {
        t.Errorf("Expected 2 results, got %d", len(results))
    }
    
    if results["key1"] != "value1" || results["key2"] != "value2" {
        t.Error("Incorrect results")
    }
}
```

#### TTL Strategies
```go
func TestDecayingTTLStrategy(t *testing.T) {
    strategy := &DecayingTTLStrategy{decayFactor: 0.5}
    baseTTL := time.Hour
    
    // L0 (first layer): shortest TTL
    ttl0 := strategy.GetTTL(0, baseTTL)
    // L1: medium TTL
    ttl1 := strategy.GetTTL(1, baseTTL)
    // L2 (last layer): full TTL
    ttl2 := strategy.GetTTL(2, baseTTL)
    
    if ttl0 >= ttl1 || ttl1 >= ttl2 {
        t.Error("TTL should decay for upper layers")
    }
}
```

### Benchmarks
```go
func BenchmarkNegativeCache_Hit(b *testing.B)
func BenchmarkBloomFilter_Rejection(b *testing.B)
func BenchmarkBatchOps_GetMulti(b *testing.B)
```

## Acceptance Criteria
- [ ] Negative caching implemented and tested
- [ ] Bloom filter layer implemented
- [ ] Batch operations (GetMulti, SetMulti, DeleteMulti)
- [ ] TTL strategies (Decaying, Uniform, Custom)
- [ ] Warming strategies (Always, Selective, Predictive)
- [ ] All features integrated with Chain
- [ ] Unit tests with >85% coverage
- [ ] Benchmarks for all features
- [ ] All tests passing: `go test ./pkg/...`
- [ ] Example code for each feature
- [ ] Code follows Go instructions
- [ ] Godoc comments on all exported types

## Dependencies
Add to `go.mod`:
```
github.com/bits-and-blooms/bloom/v3 v3.6.0
```

## Notes
- Negative caching should be configurable (some use cases don't want it)
- Bloom filters trade memory for speed (document trade-offs)
- Batch operations are most effective with remote caches (Redis, Memcached)
- TTL hierarchy prevents stale data in fast layers
- Warming strategies prevent waste of warm-up bandwidth

## Performance Expectations
- Negative cache: 99% reduction in unnecessary backend calls for missing keys
- Bloom filter: 95%+ rejection rate for non-existent keys
- Batch operations: 5-10x throughput improvement for bulk access
- TTL hierarchy: Reduces inconsistency windows by 50%+

## Next Steps
After Phase 8, the library is feature-complete. Recommended follow-ups:
1. Integration guides for common backends (Redis, Memcached)
2. Production deployment checklist
3. Performance tuning guide
4. Migration guide from other caching libraries
5. Real-world case studies and benchmarks

## Example: Complete Setup
```go
// Create layers with all features
l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1", MaxSize: 1000})
l2 := redis.NewRedisCache(redisConfig)
l3 := database.NewDBCache(dbConfig)

// Wrap with bloom filters
l1Bloom := bloom.NewBloomLayer(l1, 1000, 0.01)
l2Bloom := bloom.NewBloomLayer(l2, 10000, 0.01)

// Wrap with negative caching
l1Negative := cache.NewNegativeCacheLayer(l1Bloom, time.Minute)

// Create chain with advanced config
chain, _ := chain.NewWithConfig(chain.ChainConfig{
    Layers: []cache.CacheLayer{l1Negative, l2Bloom, l3},
    MetricsCollector: promCollector,
    TTLStrategy: &chain.DecayingTTLStrategy{decayFactor: 0.5},
    WarmingStrategy: &chain.SelectiveWarmStrategy{minValueSize: 1024},
    EnableNegativeCache: true,
    NegativeCacheTTL: time.Minute,
})
defer chain.Close()

// Use batch operations
items, _ := chain.GetMulti(ctx, []string{"key1", "key2", "key3"})
```

This completes the phased development plan for the cache-chain library!
