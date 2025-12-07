package writer

import "errors"

// AsyncWriterStats provides statistics about async writer operations.
type AsyncWriterStats struct {
	// QueueDepth is the current number of pending writes in the queue
	QueueDepth int

	// DroppedWrites is the total number of writes dropped due to backpressure
	DroppedWrites int64

	// TotalWrites is the total number of writes attempted
	TotalWrites int64

	// FailedWrites is the total number of writes that failed
	FailedWrites int64
}

// Errors returned by async writer operations.
var (
	// ErrQueueFull is returned when the write queue is full and MaxWaitTime exceeded
	ErrQueueFull = errors.New("writer: queue full, write dropped")

	// ErrWriterClosed is returned when attempting to write to a closed writer
	ErrWriterClosed = errors.New("writer: writer is closed")

	// ErrFlushTimeout is returned when Flush() times out waiting for queue to drain
	ErrFlushTimeout = errors.New("writer: flush timeout exceeded")
)
