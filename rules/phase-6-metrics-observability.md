# Phase 6: Metrics and Observability

## Objective
Implement comprehensive metrics collection for cache operations, circuit breaker states, and async writer queues. Support pluggable exporters (Prometheus, StatsD, custom) for flexibility.

## Scope
- Define metrics interfaces and types
- Implement metrics collector with per-layer granularity
- Add Prometheus exporter
- Instrument all components (Chain, ResilientLayer, AsyncWriter)
- Create example dashboard configuration
- Unit tests for metrics collection

## Requirements

### 1. Metrics Interface
Create `pkg/metrics/metrics.go`:

```go
// MetricsCollector defines the interface for collecting cache metrics
type MetricsCollector interface {
    // Cache operations
    RecordGet(layer string, hit bool, duration time.Duration)
    RecordSet(layer string, success bool, duration time.Duration)
    RecordDelete(layer string, success bool, duration time.Duration)
    
    // Circuit breaker
    RecordCircuitState(layer string, state resilience.State)
    
    // Async writer
    RecordQueueDepth(layer string, depth int)
    RecordWriteDropped(layer string)
    RecordAsyncWrite(layer string, success bool, duration time.Duration)
    
    // Chain-level
    RecordChainGet(hit bool, layerIndex int, totalDuration time.Duration)
}

// NoOpCollector is a no-op implementation (default)
type NoOpCollector struct{}

func (NoOpCollector) RecordGet(layer string, hit bool, duration time.Duration) {}
// ... all methods no-op
```

### 2. Metrics Types
Define standard metrics:

```go
type CacheMetrics struct {
    // Counters
    Hits       int64
    Misses     int64
    Sets       int64
    Deletes    int64
    Errors     int64
    
    // Async writer
    QueueDepth      int64
    DroppedWrites   int64
    FailedWrites    int64
    
    // Circuit breaker
    CircuitState    resilience.State
    CircuitOpens    int64  // Count of transitions to Open
    
    // Latency (using histogram or summary)
    GetLatency    LatencyStats
    SetLatency    LatencyStats
}

type LatencyStats struct {
    P50   time.Duration
    P95   time.Duration
    P99   time.Duration
    Max   time.Duration
    Count int64
}
```

### 3. Prometheus Exporter
Create `pkg/metrics/prometheus/prometheus.go`:

```go
type PrometheusCollector struct {
    // Counters
    cacheHits    *prometheus.CounterVec
    cacheMisses  *prometheus.CounterVec
    cacheErrors  *prometheus.CounterVec
    
    // Histograms
    cacheLatency *prometheus.HistogramVec
    
    // Gauges
    queueDepth   *prometheus.GaugeVec
    circuitState *prometheus.GaugeVec
}

func NewPrometheusCollector(namespace string) *PrometheusCollector

// Register with prometheus.Registry
func (pc *PrometheusCollector) Register(registry *prometheus.Registry) error

// Example metrics:
// cache_chain_hits_total{layer="L1"} 1234
// cache_chain_misses_total{layer="L1"} 56
// cache_chain_operation_duration_seconds{layer="L1",operation="get",quantile="0.95"} 0.001
// cache_chain_queue_depth{layer="L1"} 42
// cache_chain_circuit_state{layer="L1"} 0  # 0=closed, 1=open, 2=half-open
```

### 4. Instrumentation

#### Chain Instrumentation
Modify `pkg/chain/chain.go`:

```go
type Chain struct {
    layers  []CacheLayer
    writers []*writer.AsyncWriter
    sf      *singleflight.Group
    metrics metrics.MetricsCollector  // Add this
}

func (c *Chain) Get(ctx context.Context, key string) (interface{}, error) {
    start := time.Now()
    defer func() {
        c.metrics.RecordChainGet(hit, layerIndex, time.Since(start))
    }()
    
    // ... existing logic
}
```

#### ResilientLayer Instrumentation
Modify `pkg/resilience/layer.go`:

```go
type ResilientLayer struct {
    layer   cache.CacheLayer
    cb      *CircuitBreaker
    timeout time.Duration
    metrics metrics.MetricsCollector
}

func (rl *ResilientLayer) Get(ctx context.Context, key string) (interface{}, error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        hit := err == nil
        rl.metrics.RecordGet(rl.layer.Name(), hit, duration)
    }()
    
    // Record circuit state changes
    if rl.cb.GetState() == StateOpen {
        rl.metrics.RecordCircuitState(rl.layer.Name(), StateOpen)
    }
    
    // ... existing logic
}
```

#### AsyncWriter Instrumentation
Modify `pkg/writer/async.go`:

```go
type AsyncWriter struct {
    // ... existing fields
    metrics metrics.MetricsCollector
}

func (w *AsyncWriter) Write(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    // Record queue depth periodically
    w.metrics.RecordQueueDepth(w.layer.Name(), len(w.queue))
    
    // Record drops
    if queueFull {
        w.metrics.RecordWriteDropped(w.layer.Name())
    }
    
    // ... existing logic
}

func (w *AsyncWriter) worker() {
    for op := range w.queue {
        start := time.Now()
        err := w.layer.Set(ctx, op.key, op.value, op.ttl)
        
        success := err == nil
        w.metrics.RecordAsyncWrite(w.layer.Name(), success, time.Since(start))
    }
}
```

### 5. Configuration
Add metrics config to chain:

```go
type ChainConfig struct {
    Layers           []LayerConfig
    MetricsCollector metrics.MetricsCollector
}

func NewWithConfig(config ChainConfig) (*Chain, error)
```

## File Structure
```
pkg/
  metrics/
    metrics.go            # Interface and NoOpCollector
    metrics_test.go       # Interface tests
    prometheus/
      prometheus.go       # Prometheus implementation
      prometheus_test.go  # Prometheus tests
    memory/
      memory.go           # In-memory collector (for testing)
      memory_test.go      # Memory collector tests
examples/
  metrics/
    prometheus.go         # Example Prometheus setup
    dashboard.json        # Grafana dashboard example
```

## Testing Requirements

### Test Cases
1. **Metrics Collection**
   - All operations recorded correctly
   - Per-layer metrics isolated
   - Concurrent operations counted accurately

2. **Prometheus Exporter**
   - Metrics registered correctly
   - Labels applied properly
   - Histograms collect latency data
   - Gauges reflect current state

3. **Memory Collector** (for testing)
   - Collects all metrics in-memory
   - Thread-safe access
   - Snapshot functionality for assertions

4. **Integration**
   - Chain operations generate metrics
   - Circuit breaker states recorded
   - Queue depth tracked
   - End-to-end metric flow

### Test Example
```go
func TestChain_MetricsCollection(t *testing.T) {
    collector := memory.NewMemoryCollector()
    
    l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1"})
    l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2"})
    
    // Set value in L2
    l2.Set(context.Background(), "key", "value", time.Minute)
    
    chain, _ := chain.NewWithConfig(chain.ChainConfig{
        Layers:           []cache.CacheLayer{l1, l2},
        MetricsCollector: collector,
    })
    defer chain.Close()
    
    // Get: L1 miss, L2 hit
    chain.Get(context.Background(), "key")
    
    // Verify metrics
    snapshot := collector.Snapshot()
    
    if snapshot.LayerMetrics["L1"].Misses != 1 {
        t.Error("L1 should have 1 miss")
    }
    if snapshot.LayerMetrics["L2"].Hits != 1 {
        t.Error("L2 should have 1 hit")
    }
    if snapshot.ChainMetrics.TotalHits != 1 {
        t.Error("Chain should have 1 total hit")
    }
}

func TestPrometheusCollector_Export(t *testing.T) {
    collector := prometheus.NewPrometheusCollector("cache_chain")
    registry := prometheus.NewRegistry()
    collector.Register(registry)
    
    // Record some metrics
    collector.RecordGet("L1", false, 10*time.Millisecond)
    collector.RecordGet("L2", true, 50*time.Millisecond)
    
    // Gather metrics
    families, err := registry.Gather()
    if err != nil {
        t.Fatal(err)
    }
    
    // Verify metric families exist
    found := make(map[string]bool)
    for _, family := range families {
        found[family.GetName()] = true
    }
    
    expected := []string{
        "cache_chain_hits_total",
        "cache_chain_misses_total",
        "cache_chain_operation_duration_seconds",
    }
    
    for _, name := range expected {
        if !found[name] {
            t.Errorf("Missing metric: %s", name)
        }
    }
}
```

## Acceptance Criteria
- [ ] MetricsCollector interface defined
- [ ] NoOpCollector (default, zero overhead)
- [ ] PrometheusCollector implementation
- [ ] MemoryCollector for testing
- [ ] All components instrumented
- [ ] Per-layer metrics isolation
- [ ] Thread-safe metric collection
- [ ] Example Prometheus setup
- [ ] Unit tests with >85% coverage
- [ ] Integration test with real Prometheus registry
- [ ] Code follows Go instructions
- [ ] Godoc comments on all exported types

## Dependencies
Add to `go.mod`:
```
github.com/prometheus/client_golang v1.17.0
```

## Example Usage
```go
// Setup with Prometheus
promCollector := prometheus.NewPrometheusCollector("myapp")
registry := prometheus.NewRegistry()
promCollector.Register(registry)

// Create chain with metrics
chain, _ := chain.NewWithConfig(chain.ChainConfig{
    Layers: []cache.CacheLayer{l1, l2, l3},
    MetricsCollector: promCollector,
})

// Expose metrics endpoint
http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
http.ListenAndServe(":9090", nil)
```

## Grafana Dashboard
Create `examples/metrics/dashboard.json` with panels for:
- Cache hit rate by layer
- Latency percentiles (P50, P95, P99)
- Circuit breaker states
- Queue depth and dropped writes
- Error rate

## Notes
- Keep metrics collection lightweight (avoid heavy allocations)
- Use labels for layer identification
- Consider cardinality - don't use high-cardinality labels (like cache keys)
- Metrics should not fail operations - always no-op on error
- Document metric names and labels in godoc

## Next Phase
Phase 7 will add validator support for data integrity checks at layer boundaries.
