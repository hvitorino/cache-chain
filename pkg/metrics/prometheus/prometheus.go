package prometheus

import (
	"time"

	"cache-chain/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusCollector implements MetricsCollector for Prometheus.
type PrometheusCollector struct {
	namespace string

	// Counters
	cacheHits    *prometheus.CounterVec
	cacheMisses  *prometheus.CounterVec
	cacheSets    *prometheus.CounterVec
	cacheDeletes *prometheus.CounterVec
	cacheErrors  *prometheus.CounterVec

	// Circuit breaker
	circuitOpens *prometheus.CounterVec
	circuitState *prometheus.GaugeVec

	// Async writer
	queueDepth    *prometheus.GaugeVec
	droppedWrites *prometheus.CounterVec
	asyncWrites   *prometheus.CounterVec
	asyncErrors   *prometheus.CounterVec

	// Histograms
	getLatency    *prometheus.HistogramVec
	setLatency    *prometheus.HistogramVec
	deleteLatency *prometheus.HistogramVec
	asyncLatency  *prometheus.HistogramVec

	// Chain-level
	chainHits    *prometheus.CounterVec
	chainMisses  *prometheus.CounterVec
	chainLatency *prometheus.HistogramVec
}

// NewPrometheusCollector creates a new Prometheus metrics collector.
func NewPrometheusCollector(namespace string) *PrometheusCollector {
	pc := &PrometheusCollector{
		namespace: namespace,
		cacheHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_hits_total",
				Help:      "Total number of cache hits per layer",
			},
			[]string{"layer"},
		),
		cacheMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_misses_total",
				Help:      "Total number of cache misses per layer",
			},
			[]string{"layer"},
		),
		cacheSets: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_sets_total",
				Help:      "Total number of cache set operations per layer",
			},
			[]string{"layer"},
		),
		cacheDeletes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_deletes_total",
				Help:      "Total number of cache delete operations per layer",
			},
			[]string{"layer"},
		),
		cacheErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_errors_total",
				Help:      "Total number of cache errors per layer and operation",
			},
			[]string{"layer", "operation"},
		),
		circuitOpens: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "circuit_opens_total",
				Help:      "Total number of circuit breaker opens per layer",
			},
			[]string{"layer"},
		),
		circuitState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "circuit_state",
				Help:      "Current circuit breaker state per layer (0=closed, 1=open, 2=half-open)",
			},
			[]string{"layer"},
		),
		queueDepth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "queue_depth",
				Help:      "Current async writer queue depth per layer",
			},
			[]string{"layer"},
		),
		droppedWrites: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "dropped_writes_total",
				Help:      "Total number of dropped async writes per layer",
			},
			[]string{"layer"},
		),
		asyncWrites: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "async_writes_total",
				Help:      "Total number of async writes per layer",
			},
			[]string{"layer", "status"},
		),
		asyncErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "async_errors_total",
				Help:      "Total number of async write errors per layer",
			},
			[]string{"layer"},
		),
		getLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "get_duration_seconds",
				Help:      "Cache get operation latency",
				Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 15), // 0.1ms to ~3s
			},
			[]string{"layer"},
		),
		setLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "set_duration_seconds",
				Help:      "Cache set operation latency",
				Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 15),
			},
			[]string{"layer"},
		),
		deleteLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "delete_duration_seconds",
				Help:      "Cache delete operation latency",
				Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 15),
			},
			[]string{"layer"},
		),
		asyncLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "async_write_duration_seconds",
				Help:      "Async write operation latency",
				Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 15),
			},
			[]string{"layer"},
		),
		chainLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "chain_get_duration_seconds",
				Help:      "Chain get operation total latency",
				Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 15),
			},
			[]string{"hit"},
		),
		chainHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "chain_hits_total",
				Help:      "Total number of chain-level cache hits",
			},
			[]string{"layer_index"},
		),
		chainMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "chain_misses_total",
				Help:      "Total number of chain-level cache misses",
			},
			[]string{},
		),
	}

	return pc
}

// Register registers all metrics with the given Prometheus registry.
func (pc *PrometheusCollector) Register(registry *prometheus.Registry) error {
	collectors := []prometheus.Collector{
		pc.cacheHits,
		pc.cacheMisses,
		pc.cacheSets,
		pc.cacheDeletes,
		pc.cacheErrors,
		pc.circuitOpens,
		pc.circuitState,
		pc.queueDepth,
		pc.droppedWrites,
		pc.asyncWrites,
		pc.asyncErrors,
		pc.getLatency,
		pc.setLatency,
		pc.deleteLatency,
		pc.asyncLatency,
		pc.chainLatency,
		pc.chainHits,
		pc.chainMisses,
	}

	for _, collector := range collectors {
		if err := registry.Register(collector); err != nil {
			return err
		}
	}

	return nil
}

// RecordGet records a cache get operation.
func (pc *PrometheusCollector) RecordGet(layer string, hit bool, duration time.Duration) {
	if hit {
		pc.cacheHits.WithLabelValues(layer).Inc()
	} else {
		pc.cacheMisses.WithLabelValues(layer).Inc()
	}
	pc.getLatency.WithLabelValues(layer).Observe(duration.Seconds())
}

// RecordSet records a cache set operation.
func (pc *PrometheusCollector) RecordSet(layer string, success bool, duration time.Duration) {
	pc.cacheSets.WithLabelValues(layer).Inc()
	if !success {
		pc.cacheErrors.WithLabelValues(layer, "set").Inc()
	}
	pc.setLatency.WithLabelValues(layer).Observe(duration.Seconds())
}

// RecordDelete records a cache delete operation.
func (pc *PrometheusCollector) RecordDelete(layer string, success bool, duration time.Duration) {
	pc.cacheDeletes.WithLabelValues(layer).Inc()
	if !success {
		pc.cacheErrors.WithLabelValues(layer, "delete").Inc()
	}
	pc.deleteLatency.WithLabelValues(layer).Observe(duration.Seconds())
}

// RecordCircuitState records the current circuit breaker state.
func (pc *PrometheusCollector) RecordCircuitState(layer string, state metrics.CircuitState) {
	pc.circuitState.WithLabelValues(layer).Set(float64(state))
	if state == metrics.CircuitOpen {
		pc.circuitOpens.WithLabelValues(layer).Inc()
	}
}

// RecordQueueDepth records the current async writer queue depth.
func (pc *PrometheusCollector) RecordQueueDepth(layer string, depth int) {
	pc.queueDepth.WithLabelValues(layer).Set(float64(depth))
}

// RecordWriteDropped records a dropped async write.
func (pc *PrometheusCollector) RecordWriteDropped(layer string) {
	pc.droppedWrites.WithLabelValues(layer).Inc()
}

// RecordAsyncWrite records an async write operation.
func (pc *PrometheusCollector) RecordAsyncWrite(layer string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
		pc.asyncErrors.WithLabelValues(layer).Inc()
	}
	pc.asyncWrites.WithLabelValues(layer, status).Inc()
	pc.asyncLatency.WithLabelValues(layer).Observe(duration.Seconds())
}

// RecordChainGet records a chain-level get operation.
func (pc *PrometheusCollector) RecordChainGet(hit bool, layerIndex int, totalDuration time.Duration) {
	hitLabel := "false"
	if hit {
		pc.chainHits.WithLabelValues(string(rune('0' + layerIndex))).Inc()
		hitLabel = "true"
	} else {
		pc.chainMisses.WithLabelValues().Inc()
	}
	pc.chainLatency.WithLabelValues(hitLabel).Observe(totalDuration.Seconds())
}
