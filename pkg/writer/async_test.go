package writer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cache-chain/pkg/cache/mock"
)

func TestNewAsyncWriter(t *testing.T) {
	layer := &mock.MockLayer{}
	config := AsyncWriterConfig{
		QueueSize:   100,
		Workers:     4,
		MaxWaitTime: 5 * time.Millisecond,
	}

	writer := NewAsyncWriter(layer, config)
	defer writer.Close()

	if writer == nil {
		t.Fatal("NewAsyncWriter returned nil")
	}

	if writer.workers != 4 {
		t.Errorf("Expected 4 workers, got %d", writer.workers)
	}

	if cap(writer.queue) != 100 {
		t.Errorf("Expected queue size 100, got %d", cap(writer.queue))
	}
}

func TestNewAsyncWriter_Defaults(t *testing.T) {
	layer := &mock.MockLayer{}
	config := AsyncWriterConfig{} // Use defaults

	writer := NewAsyncWriter(layer, config)
	defer writer.Close()

	if cap(writer.queue) != 1000 {
		t.Errorf("Expected default queue size 1000, got %d", cap(writer.queue))
	}

	if writer.workers != 2 {
		t.Errorf("Expected default workers 2, got %d", writer.workers)
	}

	if writer.config.MaxWaitTime != 10*time.Millisecond {
		t.Errorf("Expected default MaxWaitTime 10ms, got %v", writer.config.MaxWaitTime)
	}
}

func TestAsyncWriter_Write(t *testing.T) {
	var mu sync.Mutex
	writes := make(map[string]interface{})

	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			mu.Lock()
			defer mu.Unlock()
			writes[key] = value
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   10,
		Workers:     1,
		MaxWaitTime: 10 * time.Millisecond,
	})
	defer writer.Close()

	// Write a value
	err := writer.Write(context.Background(), "key1", "value1", time.Minute)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if writes["key1"] != "value1" {
		t.Errorf("Expected value1, got %v", writes["key1"])
	}

	stats := writer.Stats()
	if stats.TotalWrites != 1 {
		t.Errorf("Expected 1 total write, got %d", stats.TotalWrites)
	}
}

func TestAsyncWriter_ConcurrentWrites(t *testing.T) {
	var mu sync.Mutex
	writes := make(map[string]interface{})

	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			mu.Lock()
			defer mu.Unlock()
			writes[key] = value
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   100,
		Workers:     4,
		MaxWaitTime: 10 * time.Millisecond,
	})
	defer writer.Close()

	// Write concurrently
	var wg sync.WaitGroup
	numWrites := 50

	for i := 0; i < numWrites; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			err := writer.Write(context.Background(), key, i, time.Minute)
			if err != nil {
				t.Errorf("Write %d failed: %v", i, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all writes to process
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(writes) != numWrites {
		t.Errorf("Expected %d writes, got %d", numWrites, len(writes))
	}

	stats := writer.Stats()
	if stats.TotalWrites != int64(numWrites) {
		t.Errorf("Expected %d total writes, got %d", numWrites, stats.TotalWrites)
	}
}

func TestAsyncWriter_Backpressure(t *testing.T) {
	// Create a layer that blocks on first write to prevent queue draining
	firstWrite := make(chan struct{})
	var writeCount int
	var mu sync.Mutex

	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			mu.Lock()
			writeCount++
			isFirst := writeCount == 1
			mu.Unlock()

			if isFirst {
				<-firstWrite // Block on first write
			}
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   5,
		Workers:     1,
		MaxWaitTime: 10 * time.Millisecond,
	})
	defer func() {
		close(firstWrite) // Unblock worker
		writer.Close()
	}()

	// Fill the queue quickly (6 writes for a queue of 5)
	// First write goes to worker and blocks, next 5 fill the queue
	for i := 0; i < 6; i++ {
		err := writer.Write(context.Background(), fmt.Sprintf("key%d", i), i, time.Minute)
		if err != nil {
			t.Fatalf("Write %d failed unexpectedly: %v", i, err)
		}
		time.Sleep(1 * time.Millisecond) // Small delay to ensure ordering
	}

	// Now the queue should be full (worker blocked, 5 in queue)
	// Next write should drop
	err := writer.Write(context.Background(), "key-extra", "value", time.Minute)
	if err != ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}

	stats := writer.Stats()
	if stats.DroppedWrites != 1 {
		t.Errorf("Expected 1 dropped write, got %d", stats.DroppedWrites)
	}

	if stats.TotalWrites != 6 {
		t.Errorf("Expected 6 successful writes, got %d", stats.TotalWrites)
	}
}

func TestAsyncWriter_ContextCancellation(t *testing.T) {
	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   10,
		Workers:     1,
		MaxWaitTime: 100 * time.Millisecond,
	})
	defer writer.Close()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := writer.Write(ctx, "key", "value", time.Minute)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestAsyncWriter_ErrorHandling(t *testing.T) {
	var failedWrites int64
	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			atomic.AddInt64(&failedWrites, 1)
			return fmt.Errorf("mock error")
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   10,
		Workers:     1,
		MaxWaitTime: 10 * time.Millisecond,
	})
	defer writer.Close()

	// Write should succeed (enqueue), but layer.Set will fail
	err := writer.Write(context.Background(), "key", "value", time.Minute)
	if err != nil {
		t.Fatalf("Write enqueue failed: %v", err)
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt64(&failedWrites) != 1 {
		t.Errorf("Expected 1 failed write to underlying layer, got %d", atomic.LoadInt64(&failedWrites))
	}

	stats := writer.Stats()
	if stats.FailedWrites != 1 {
		t.Errorf("Expected 1 failed write in stats, got %d", stats.FailedWrites)
	}
}

func TestAsyncWriter_Flush(t *testing.T) {
	var mu sync.Mutex
	writes := make(map[string]interface{})

	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			time.Sleep(20 * time.Millisecond) // Simulate work
			mu.Lock()
			defer mu.Unlock()
			writes[key] = value
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   10,
		Workers:     2,
		MaxWaitTime: 10 * time.Millisecond,
	})
	defer writer.Close()

	// Write multiple values
	for i := 0; i < 5; i++ {
		err := writer.Write(context.Background(), fmt.Sprintf("key%d", i), i, time.Minute)
		if err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}
	}

	// Flush with reasonable timeout (longer to ensure all writes complete)
	err := writer.Flush(1 * time.Second)
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Wait a bit more to ensure workers complete processing
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(writes) != 5 {
		t.Errorf("Expected 5 writes after flush, got %d", len(writes))
	}
}

func TestAsyncWriter_FlushTimeout(t *testing.T) {
	blocker := make(chan struct{})
	var once sync.Once

	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			// Block forever on first write
			once.Do(func() {
				<-blocker
			})
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   10,
		Workers:     1,
		MaxWaitTime: 10 * time.Millisecond,
	})
	defer func() {
		close(blocker) // Unblock
		writer.Close()
	}()

	// Write values
	for i := 0; i < 3; i++ {
		writer.Write(context.Background(), fmt.Sprintf("key%d", i), i, time.Minute)
	}

	// Flush with short timeout should fail
	err := writer.Flush(50 * time.Millisecond)
	if err != ErrFlushTimeout {
		t.Errorf("Expected ErrFlushTimeout, got %v", err)
	}
}

func TestAsyncWriter_Close(t *testing.T) {
	var mu sync.Mutex
	writes := make(map[string]interface{})

	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			defer mu.Unlock()
			writes[key] = value
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   10,
		Workers:     2,
		MaxWaitTime: 10 * time.Millisecond,
	})

	// Write some values
	for i := 0; i < 3; i++ {
		err := writer.Write(context.Background(), fmt.Sprintf("key%d", i), i, time.Minute)
		if err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}
	}

	// Close waits for workers to finish draining the queue
	err := writer.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// All writes should have been processed before Close returned
	if len(writes) != 3 {
		t.Errorf("Expected 3 writes after close, got %d", len(writes))
	}
}

func TestAsyncWriter_WriteAfterClose(t *testing.T) {
	layer := &mock.MockLayer{}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   10,
		Workers:     1,
		MaxWaitTime: 10 * time.Millisecond,
	})

	// Close the writer
	writer.Close()

	// Try to write after close
	err := writer.Write(context.Background(), "key", "value", time.Minute)
	if err != ErrWriterClosed {
		t.Errorf("Expected ErrWriterClosed, got %v", err)
	}
}

func TestAsyncWriter_Stats(t *testing.T) {
	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   10,
		Workers:     1,
		MaxWaitTime: 10 * time.Millisecond,
	})
	defer writer.Close()

	// Initial stats
	stats := writer.Stats()
	if stats.TotalWrites != 0 {
		t.Errorf("Expected 0 initial writes, got %d", stats.TotalWrites)
	}

	// Write some values
	for i := 0; i < 3; i++ {
		writer.Write(context.Background(), fmt.Sprintf("key%d", i), i, time.Minute)
	}

	stats = writer.Stats()
	if stats.TotalWrites != 3 {
		t.Errorf("Expected 3 total writes, got %d", stats.TotalWrites)
	}

	if stats.QueueDepth > 3 {
		t.Errorf("Expected queue depth <= 3, got %d", stats.QueueDepth)
	}
}

func TestAsyncWriter_Ordering(t *testing.T) {
	var mu sync.Mutex
	writes := make([]string, 0)

	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			mu.Lock()
			defer mu.Unlock()
			writes = append(writes, key)
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   20,
		Workers:     1, // Single worker ensures FIFO
		MaxWaitTime: 10 * time.Millisecond,
	})
	defer writer.Close()

	// Write in order
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	for _, key := range keys {
		writer.Write(context.Background(), key, "value", time.Minute)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// With single worker, writes should be in order
	if len(writes) != len(keys) {
		t.Fatalf("Expected %d writes, got %d", len(keys), len(writes))
	}

	for i, key := range keys {
		if writes[i] != key {
			t.Errorf("Expected write %d to be %s, got %s", i, key, writes[i])
		}
	}
}

func BenchmarkAsyncWriter_Write(b *testing.B) {
	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   10000,
		Workers:     4,
		MaxWaitTime: 10 * time.Millisecond,
	})
	defer writer.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Write(context.Background(), "key", "value", time.Minute)
	}
}

func BenchmarkAsyncWriter_ConcurrentWrites(b *testing.B) {
	layer := &mock.MockLayer{
		SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
			return nil
		},
	}

	writer := NewAsyncWriter(layer, AsyncWriterConfig{
		QueueSize:   10000,
		Workers:     4,
		MaxWaitTime: 10 * time.Millisecond,
	})
	defer writer.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			writer.Write(context.Background(), fmt.Sprintf("key%d", i), i, time.Minute)
			i++
		}
	})
}
