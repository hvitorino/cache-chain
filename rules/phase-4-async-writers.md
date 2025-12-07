# Phase 4: Async Writers for Non-Blocking Warm-Up

## Objective
Replace synchronous warm-up with asynchronous writers using buffered channels and worker pools. This prevents warm-up operations from blocking Get() calls while maintaining ordering and preventing memory leaks.

## Scope
- Implement `AsyncWriter` component with bounded queues
- Integrate async writers into Chain warm-up logic
- Add write operation batching support
- Implement graceful shutdown
- Comprehensive unit tests including backpressure scenarios

## Requirements

### 1. AsyncWriter Implementation
Create `pkg/writer/async.go`:

```go
type AsyncWriter struct {
    layer      CacheLayer
    queue      chan writeOp
    workers    int
    wg         sync.WaitGroup
    ctx        context.Context
    cancelFunc context.CancelFunc
}

type writeOp struct {
    key       string
    value     interface{}
    ttl       time.Duration
    timestamp time.Time  // For ordering
}

// NewAsyncWriter creates a writer with bounded queue and N workers
func NewAsyncWriter(layer CacheLayer, config AsyncWriterConfig) *AsyncWriter

type AsyncWriterConfig struct {
    QueueSize   int           // Bounded queue size
    Workers     int           // Number of concurrent workers
    MaxWaitTime time.Duration // Max time to wait if queue full (0 = drop immediately)
}
```

### 2. Core Operations
```go
// Write enqueues a write operation (non-blocking with timeout)
func (w *AsyncWriter) Write(ctx context.Context, key string, value interface{}, ttl time.Duration) error

// Flush waits for all pending writes to complete (for testing/shutdown)
func (w *AsyncWriter) Flush(timeout time.Duration) error

// Close stops workers and drains queue
func (w *AsyncWriter) Close() error

// Stats returns queue metrics
func (w *AsyncWriter) Stats() AsyncWriterStats

type AsyncWriterStats struct {
    QueueDepth    int
    DroppedWrites int64
    TotalWrites   int64
    FailedWrites  int64
}
```

### 3. Worker Pool Pattern
```go
// Start N workers that consume from queue
for i := 0; i < workers; i++ {
    w.wg.Add(1)
    go w.worker()
}

func (w *AsyncWriter) worker() {
    defer w.wg.Done()
    for {
        select {
        case op := <-w.queue:
            // Write to underlying layer
            // Handle errors (log, increment counter)
        case <-w.ctx.Done():
            return
        }
    }
}
```

### 4. Backpressure Handling
When queue is full:
1. **Wait**: Try to enqueue for `MaxWaitTime`
2. **Drop**: If still full, drop write and increment counter
3. **Never Block Forever**: Protect caller from indefinite blocking

```go
select {
case w.queue <- op:
    // Successfully enqueued
case <-time.After(config.MaxWaitTime):
    atomic.AddInt64(&w.droppedWrites, 1)
    return ErrQueueFull
case <-ctx.Done():
    return ctx.Err()
}
```

### 5. Ordering Considerations
- Within same key: maintain order using timestamp
- Across different keys: no ordering guarantee needed
- Worker processes ops in FIFO order from queue

### 6. Integration with Chain
Modify `pkg/chain/chain.go`:

```go
type Chain struct {
    layers  []CacheLayer
    writers []*writer.AsyncWriter  // One per layer
    sf      *singleflight.Group
}

// Warm-up becomes async:
func (c *Chain) warmUp(layerIndex int, key string, value interface{}, ttl time.Duration) {
    if layerIndex < 0 {
        return
    }
    
    // Use async writer instead of direct Set()
    c.writers[layerIndex].Write(context.Background(), key, value, ttl)
    
    // Continue warming upper layers
    c.warmUp(layerIndex-1, key, value, ttl)
}
```

## File Structure
```
pkg/
  writer/
    async.go       # AsyncWriter implementation
    async_test.go  # Unit tests
    stats.go       # Statistics tracking
    stats_test.go  # Stats tests
```

## Testing Requirements

### Test Cases
1. **Basic Async Operations**
   - Write enqueues successfully
   - Workers process writes
   - Flush waits for completion
   - Close stops workers

2. **Concurrency**
   - Multiple concurrent writes
   - Workers process in parallel
   - No race conditions (test with -race)

3. **Backpressure**
   - Queue full: wait then drop
   - Dropped writes counted correctly
   - MaxWaitTime respected
   - Never blocks indefinitely

4. **Ordering**
   - Same key writes maintain order
   - Timestamp-based ordering works
   - Different keys can process out of order

5. **Error Handling**
   - Underlying layer errors logged and counted
   - Failed writes don't crash workers
   - Context cancellation stops writes

6. **Graceful Shutdown**
   - Close() stops accepting new writes
   - Pending writes in queue complete
   - Workers exit cleanly
   - Flush() works correctly

### Test Example
```go
func TestAsyncWriter_Backpressure(t *testing.T) {
    mock := &mock.MockLayer{
        SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
            time.Sleep(100 * time.Millisecond) // Slow writes
            return nil
        },
    }
    
    writer := NewAsyncWriter(mock, AsyncWriterConfig{
        QueueSize:   5,
        Workers:     1,
        MaxWaitTime: 10 * time.Millisecond,
    })
    defer writer.Close()
    
    // Fill queue
    for i := 0; i < 5; i++ {
        err := writer.Write(context.Background(), fmt.Sprintf("key%d", i), i, time.Minute)
        if err != nil {
            t.Fatalf("Write %d failed: %v", i, err)
        }
    }
    
    // Next write should drop (queue full + timeout)
    err := writer.Write(context.Background(), "key-extra", "value", time.Minute)
    if err != writer.ErrQueueFull {
        t.Errorf("Expected ErrQueueFull, got %v", err)
    }
    
    stats := writer.Stats()
    if stats.DroppedWrites != 1 {
        t.Errorf("Expected 1 dropped write, got %d", stats.DroppedWrites)
    }
}
```

### Integration Test
```go
func TestChain_AsyncWarmup(t *testing.T) {
    l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1"})
    l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2"})
    
    chain, _ := chain.New(l1, l2)
    defer chain.Close()
    
    // Populate L2
    l2.Set(context.Background(), "key", "value", time.Minute)
    
    // Get from chain (L1 miss, L2 hit)
    value, _ := chain.Get(context.Background(), "key")
    
    // Wait for async warm-up
    time.Sleep(100 * time.Millisecond)
    
    // Verify L1 was warmed
    cached, err := l1.Get(context.Background(), "key")
    if err != nil {
        t.Errorf("L1 should be warmed up: %v", err)
    }
    if cached != value {
        t.Errorf("L1 cached value incorrect")
    }
}
```

### Benchmarks
```go
func BenchmarkAsyncWriter_Write(b *testing.B)
func BenchmarkAsyncWriter_ConcurrentWrites(b *testing.B)
```

## Acceptance Criteria
- [ ] AsyncWriter implemented with worker pool
- [ ] Bounded queue with backpressure handling
- [ ] No indefinite blocking (MaxWaitTime respected)
- [ ] Graceful shutdown (Flush and Close)
- [ ] Statistics tracking (queue depth, drops, failures)
- [ ] Chain integration complete (async warm-up)
- [ ] Unit tests with >90% coverage
- [ ] Race detector clean: `go test -race ./pkg/writer/...`
- [ ] Integration test with Chain passes
- [ ] Code follows Go instructions
- [ ] Godoc comments on all exported types/functions

## Notes
- Use `atomic` package for statistics counters
- Consider making QueueSize and Workers configurable per layer
- Log failed writes at DEBUG level (Phase 6 will add structured logging)
- Dropped writes are acceptable - cache warm-up is best-effort
- Workers should continue on write errors (don't crash)

## Performance Considerations
- QueueSize: balance memory vs throughput (default: 1000)
- Workers: typically 1-4 per layer (more doesn't always help)
- MaxWaitTime: typically 1-10ms (prevent long blocks)

## Next Phase
Phase 5 will add circuit breakers and timeouts for layer resilience.
