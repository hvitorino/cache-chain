package writer

import (
	"testing"
)

func TestAsyncWriterStats(t *testing.T) {
	stats := AsyncWriterStats{
		QueueDepth:    10,
		DroppedWrites: 5,
		TotalWrites:   100,
		FailedWrites:  2,
	}

	if stats.QueueDepth != 10 {
		t.Errorf("Expected QueueDepth 10, got %d", stats.QueueDepth)
	}

	if stats.DroppedWrites != 5 {
		t.Errorf("Expected DroppedWrites 5, got %d", stats.DroppedWrites)
	}

	if stats.TotalWrites != 100 {
		t.Errorf("Expected TotalWrites 100, got %d", stats.TotalWrites)
	}

	if stats.FailedWrites != 2 {
		t.Errorf("Expected FailedWrites 2, got %d", stats.FailedWrites)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrQueueFull",
			err:      ErrQueueFull,
			expected: "writer: queue full, write dropped",
		},
		{
			name:     "ErrWriterClosed",
			err:      ErrWriterClosed,
			expected: "writer: writer is closed",
		},
		{
			name:     "ErrFlushTimeout",
			err:      ErrFlushTimeout,
			expected: "writer: flush timeout exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Expected error message %q, got %q", tt.expected, tt.err.Error())
			}
		})
	}
}
