package bloom

import (
	"context"
	"sync"
	"time"

	"cache-chain/pkg/cache"

	"github.com/bits-and-blooms/bloom/v3"
)

// BloomLayer adds probabilistic membership testing to a cache layer.
type BloomLayer struct {
	layer  cache.CacheLayer
	filter *bloom.BloomFilter
	mu     sync.RWMutex

	totalQueries   uint64
	bloomRejected  uint64
	falsePositives uint64
}

// NewBloomLayer creates a new bloom filter layer wrapper.
func NewBloomLayer(layer cache.CacheLayer, expectedItems uint, falsePositiveRate float64) *BloomLayer {
	if expectedItems == 0 {
		expectedItems = 10000
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = 0.01
	}

	filter := bloom.NewWithEstimates(expectedItems, falsePositiveRate)

	return &BloomLayer{
		layer:  layer,
		filter: filter,
	}
}

// Name returns the name of the underlying cache layer.
func (bl *BloomLayer) Name() string {
	return "bloom(" + bl.layer.Name() + ")"
}

// Get retrieves a value from the cache.
func (bl *BloomLayer) Get(ctx context.Context, key string) (interface{}, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	bl.mu.Lock()
	bl.totalQueries++
	mayExist := bl.filter.Test([]byte(key))
	if !mayExist {
		bl.bloomRejected++
		bl.mu.Unlock()
		return nil, cache.ErrKeyNotFound
	}
	bl.mu.Unlock()

	value, err := bl.layer.Get(ctx, key)

	if cache.IsNotFound(err) {
		bl.mu.Lock()
		bl.falsePositives++
		bl.mu.Unlock()
	}

	return value, err
}

// Set stores a value in the cache.
func (bl *BloomLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	bl.mu.Lock()
	bl.filter.Add([]byte(key))
	bl.mu.Unlock()

	return bl.layer.Set(ctx, key, value, ttl)
}

// Delete removes a value from the cache.
func (bl *BloomLayer) Delete(ctx context.Context, key string) error {
	return bl.layer.Delete(ctx, key)
}

// Close closes the underlying cache layer.
func (bl *BloomLayer) Close() error {
	return bl.layer.Close()
}

// Reset clears the bloom filter.
func (bl *BloomLayer) Reset() {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	expectedItems := uint(bl.filter.Cap())
	bl.filter = bloom.NewWithEstimates(expectedItems, 0.01)
	bl.totalQueries = 0
	bl.bloomRejected = 0
	bl.falsePositives = 0
}

// Stats returns statistics about the bloom filter.
func (bl *BloomLayer) Stats() BloomStats {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	rejectionRate := 0.0
	falsePositiveRate := 0.0

	if bl.totalQueries > 0 {
		rejectionRate = float64(bl.bloomRejected) / float64(bl.totalQueries)
		queried := bl.totalQueries - bl.bloomRejected
		if queried > 0 {
			falsePositiveRate = float64(bl.falsePositives) / float64(queried)
		}
	}

	return BloomStats{
		TotalQueries:      bl.totalQueries,
		BloomRejected:     bl.bloomRejected,
		FalsePositives:    bl.falsePositives,
		RejectionRate:     rejectionRate,
		FalsePositiveRate: falsePositiveRate,
		FilterCapacity:    uint(bl.filter.Cap()),
	}
}

// BloomStats holds statistics about bloom filter performance.
type BloomStats struct {
	TotalQueries      uint64
	BloomRejected     uint64
	FalsePositives    uint64
	RejectionRate     float64
	FalsePositiveRate float64
	FilterCapacity    uint
}
