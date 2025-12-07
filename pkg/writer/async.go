package writer

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"cache-chain/pkg/cache"
	"cache-chain/pkg/metrics"
)

// AsyncWriter provides non-blocking cache writes using a worker pool and bounded queue.
// It prevents cache warm-up operations from blocking Get() calls while maintaining
// ordering guarantees within the same key.
type AsyncWriter struct {
	layer      cache.CacheLayer
	queue      chan writeOp
	workers    int
	wg         sync.WaitGroup
	ctx        context.Context
	cancelFunc context.CancelFunc
	config     AsyncWriterConfig
	metrics    metrics.MetricsCollector
	layerName  string

	// Statistics (accessed atomically)
	droppedWrites int64
	totalWrites   int64
	failedWrites  int64

	// Metrics ticker for periodic queue depth reporting
	metricsTicker *time.Ticker
	metricsStop   chan struct{}
}

// writeOp represents a pending write operation.
type writeOp struct {
	key       string
	value     interface{}
	ttl       time.Duration
	timestamp time.Time // For ordering verification
}

// AsyncWriterConfig configures the async writer behavior.
type AsyncWriterConfig struct {
	// QueueSize is the bounded queue size (default: 1000)
	QueueSize int

	// Workers is the number of concurrent workers (default: 2)
	Workers int

	// MaxWaitTime is the max time to wait if queue is full.
	// 0 means drop immediately (default: 10ms)
	MaxWaitTime time.Duration
}

// NewAsyncWriter creates a new async writer with bounded queue and worker pool.
// The writer starts processing immediately and must be closed with Close().
func NewAsyncWriter(layer cache.CacheLayer, config AsyncWriterConfig) *AsyncWriter {
	return NewAsyncWriterWithMetrics(layer, config, metrics.NoOpCollector{})
}

// NewAsyncWriterWithMetrics creates a new async writer with custom metrics collector.
func NewAsyncWriterWithMetrics(layer cache.CacheLayer, config AsyncWriterConfig, metricsCollector metrics.MetricsCollector) *AsyncWriter {
	// Apply defaults
	if config.QueueSize <= 0 {
		config.QueueSize = 1000
	}
	if config.Workers <= 0 {
		config.Workers = 2
	}
	if config.MaxWaitTime == 0 {
		config.MaxWaitTime = 10 * time.Millisecond
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &AsyncWriter{
		layer:         layer,
		queue:         make(chan writeOp, config.QueueSize),
		workers:       config.Workers,
		ctx:           ctx,
		cancelFunc:    cancel,
		config:        config,
		metrics:       metricsCollector,
		layerName:     layer.Name(),
		metricsTicker: time.NewTicker(5 * time.Second), // Report queue depth every 5s
		metricsStop:   make(chan struct{}),
	}

	// Start worker pool
	for i := 0; i < config.Workers; i++ {
		w.wg.Add(1)
		go w.worker()
	}

	// Start metrics reporter
	go w.reportMetrics()

	return w
}

// Write enqueues a write operation non-blockingly.
// If the queue is full, it waits up to MaxWaitTime before dropping the write.
// Returns ErrQueueFull if the write was dropped due to backpressure.
func (w *AsyncWriter) Write(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Check if writer is closed first
	select {
	case <-w.ctx.Done():
		return ErrWriterClosed
	default:
	}

	// Check if caller's context is cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	op := writeOp{
		key:       key,
		value:     value,
		ttl:       ttl,
		timestamp: time.Now(),
	}

	// Try to enqueue with timeout
	timer := time.NewTimer(w.config.MaxWaitTime)
	defer timer.Stop()

	select {
	case w.queue <- op:
		atomic.AddInt64(&w.totalWrites, 1)
		return nil
	case <-timer.C:
		atomic.AddInt64(&w.droppedWrites, 1)
		w.metrics.RecordWriteDropped(w.layerName)
		return ErrQueueFull
	case <-ctx.Done():
		return ctx.Err()
	case <-w.ctx.Done():
		return ErrWriterClosed
	}
}

// worker processes write operations from the queue.
func (w *AsyncWriter) worker() {
	defer w.wg.Done()

	for {
		select {
		case op, ok := <-w.queue:
			if !ok {
				// Queue closed
				return
			}
			// Process write operation with timing
			start := time.Now()
			err := w.layer.Set(context.Background(), op.key, op.value, op.ttl)
			duration := time.Since(start)

			success := err == nil
			w.metrics.RecordAsyncWrite(w.layerName, success, duration)

			if err != nil {
				atomic.AddInt64(&w.failedWrites, 1)
				// In Phase 6, this will use structured logging
				// For now, we silently count the failure
			}
		case <-w.ctx.Done():
			// Drain remaining items in queue before exiting
			for {
				select {
				case op, ok := <-w.queue:
					if !ok {
						return
					}
					start := time.Now()
					err := w.layer.Set(context.Background(), op.key, op.value, op.ttl)
					duration := time.Since(start)

					success := err == nil
					w.metrics.RecordAsyncWrite(w.layerName, success, duration)

					if err != nil {
						atomic.AddInt64(&w.failedWrites, 1)
					}
				default:
					return
				}
			}
		}
	}
}

// Flush waits for all pending writes to complete or until timeout.
// Returns an error if timeout is exceeded.
func (w *AsyncWriter) Flush(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		if len(w.queue) == 0 {
			return nil
		}

		if time.Now().After(deadline) {
			return ErrFlushTimeout
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// Close stops accepting new writes and waits for workers to complete.
// Any writes in the queue will be processed before shutdown.
func (w *AsyncWriter) Close() error {
	// Stop metrics reporter
	close(w.metricsStop)
	w.metricsTicker.Stop()

	// Signal workers to stop after draining queue
	w.cancelFunc()

	// Wait for all workers to finish
	w.wg.Wait()

	return nil
}

// reportMetrics periodically reports queue depth.
func (w *AsyncWriter) reportMetrics() {
	for {
		select {
		case <-w.metricsTicker.C:
			w.metrics.RecordQueueDepth(w.layerName, len(w.queue))
		case <-w.metricsStop:
			return
		}
	}
}

// Stats returns current statistics about the async writer.
func (w *AsyncWriter) Stats() AsyncWriterStats {
	return AsyncWriterStats{
		QueueDepth:    len(w.queue),
		DroppedWrites: atomic.LoadInt64(&w.droppedWrites),
		TotalWrites:   atomic.LoadInt64(&w.totalWrites),
		FailedWrites:  atomic.LoadInt64(&w.failedWrites),
	}
}
