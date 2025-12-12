package memory

import (
	"sync"
	"time"

	"cache-chain/pkg/metrics"
)

// MemoryCollector implements MetricsCollector for in-memory testing.
type MemoryCollector struct {
	mu sync.RWMutex

	// Per-layer metrics
	layerMetrics map[string]*LayerMetrics

	// Chain-level metrics
	chainHits        int64
	chainMisses      int64
	chainHitsByLayer map[int]int64
}

// LayerMetrics holds metrics for a single cache layer.
type LayerMetrics struct {
	// Operation counts
	Hits    int64
	Misses  int64
	Sets    int64
	Deletes int64
	Errors  int64

	// Error types (by error_type label)
	ErrorsByType map[string]int64

	// Circuit breaker
	CircuitState metrics.CircuitState
	CircuitOpens int64

	// Async writer
	QueueDepth    int
	DroppedWrites int64
	AsyncWrites   int64
	AsyncErrors   int64

	// Latencies (simple stats)
	GetLatencies    []time.Duration
	SetLatencies    []time.Duration
	DeleteLatencies []time.Duration
	AsyncLatencies  []time.Duration
}

// NewMemoryCollector creates a new in-memory metrics collector.
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{
		layerMetrics:     make(map[string]*LayerMetrics),
		chainHitsByLayer: make(map[int]int64),
	}
}

// getOrCreateLayer returns the LayerMetrics for the given layer, creating it if needed.
func (mc *MemoryCollector) getOrCreateLayer(layer string) *LayerMetrics {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.layerMetrics[layer]; !exists {
		mc.layerMetrics[layer] = &LayerMetrics{
			ErrorsByType: make(map[string]int64),
		}
	}
	return mc.layerMetrics[layer]
}

// RecordGet records a cache get operation.
func (mc *MemoryCollector) RecordGet(layer string, hit bool, duration time.Duration) {
	lm := mc.getOrCreateLayer(layer)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	if hit {
		lm.Hits++
	} else {
		lm.Misses++
	}
	lm.GetLatencies = append(lm.GetLatencies, duration)
}

// RecordSet records a cache set operation.
func (mc *MemoryCollector) RecordSet(layer string, success bool, duration time.Duration) {
	lm := mc.getOrCreateLayer(layer)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	lm.Sets++
	if !success {
		lm.Errors++
	}
	lm.SetLatencies = append(lm.SetLatencies, duration)
}

// RecordDelete records a cache delete operation.
func (mc *MemoryCollector) RecordDelete(layer string, success bool, duration time.Duration) {
	lm := mc.getOrCreateLayer(layer)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	lm.Deletes++
	if !success {
		lm.Errors++
	}
	lm.DeleteLatencies = append(lm.DeleteLatencies, duration)
}

// RecordError records an error by type.
func (mc *MemoryCollector) RecordError(layer, operation, errorType string) {
	lm := mc.getOrCreateLayer(layer)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	lm.Errors++
	if lm.ErrorsByType == nil {
		lm.ErrorsByType = make(map[string]int64)
	}
	lm.ErrorsByType[errorType]++
}

// RecordCircuitState records the current circuit breaker state.
func (mc *MemoryCollector) RecordCircuitState(layer string, state metrics.CircuitState) {
	lm := mc.getOrCreateLayer(layer)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	oldState := lm.CircuitState
	lm.CircuitState = state

	// Count transitions to open
	if oldState != metrics.CircuitOpen && state == metrics.CircuitOpen {
		lm.CircuitOpens++
	}
}

// RecordQueueDepth records the current async writer queue depth.
func (mc *MemoryCollector) RecordQueueDepth(layer string, depth int) {
	lm := mc.getOrCreateLayer(layer)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	lm.QueueDepth = depth
}

// RecordWriteDropped records a dropped async write.
func (mc *MemoryCollector) RecordWriteDropped(layer string) {
	lm := mc.getOrCreateLayer(layer)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	lm.DroppedWrites++
}

// RecordAsyncWrite records an async write operation.
func (mc *MemoryCollector) RecordAsyncWrite(layer string, success bool, duration time.Duration) {
	lm := mc.getOrCreateLayer(layer)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	lm.AsyncWrites++
	if !success {
		lm.AsyncErrors++
	}
	lm.AsyncLatencies = append(lm.AsyncLatencies, duration)
}

// RecordChainGet records a chain-level get operation.
func (mc *MemoryCollector) RecordChainGet(hit bool, layerIndex int, totalDuration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if hit {
		mc.chainHits++
		mc.chainHitsByLayer[layerIndex]++
	} else {
		mc.chainMisses++
	}
}

// Snapshot returns a copy of the current metrics.
type Snapshot struct {
	LayerMetrics     map[string]LayerMetrics
	ChainHits        int64
	ChainMisses      int64
	ChainHitsByLayer map[int]int64
}

// Snapshot returns a copy of the current metrics state.
func (mc *MemoryCollector) Snapshot() Snapshot {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	snapshot := Snapshot{
		LayerMetrics:     make(map[string]LayerMetrics),
		ChainHits:        mc.chainHits,
		ChainMisses:      mc.chainMisses,
		ChainHitsByLayer: make(map[int]int64),
	}

	// Deep copy layer metrics
	for layer, lm := range mc.layerMetrics {
		snapshot.LayerMetrics[layer] = *lm
	}

	// Copy chain hits by layer
	for idx, hits := range mc.chainHitsByLayer {
		snapshot.ChainHitsByLayer[idx] = hits
	}

	return snapshot
}

// Reset clears all collected metrics.
func (mc *MemoryCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.layerMetrics = make(map[string]*LayerMetrics)
	mc.chainHits = 0
	mc.chainMisses = 0
	mc.chainHitsByLayer = make(map[int]int64)
}

// GetLayerMetrics returns the metrics for a specific layer.
func (mc *MemoryCollector) GetLayerMetrics(layer string) *LayerMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if lm, exists := mc.layerMetrics[layer]; exists {
		// Return a copy
		copy := *lm
		return &copy
	}
	return nil
}
