package chain

import (
	"math"
	"time"
)

// TTLStrategy determines TTL for each layer in the chain.
type TTLStrategy interface {
	// GetTTL returns the TTL for a specific layer index
	GetTTL(layerIndex int, baseTTL time.Duration) time.Duration
}

// UniformTTLStrategy uses the same TTL for all layers.
type UniformTTLStrategy struct{}

// GetTTL returns the base TTL for all layers.
func (s *UniformTTLStrategy) GetTTL(layerIndex int, baseTTL time.Duration) time.Duration {
	return baseTTL
}

// DecayingTTLStrategy reduces TTL for upper (faster) layers.
// This prevents stale data in fast layers while allowing longer TTL in slow layers.
type DecayingTTLStrategy struct {
	DecayFactor float64 // e.g., 0.5 means each layer has half the TTL of the next
}

// GetTTL returns decaying TTL based on layer index.
// Layer 0 (L1) has shortest TTL, last layer has full baseTTL.
func (s *DecayingTTLStrategy) GetTTL(layerIndex int, baseTTL time.Duration) time.Duration {
	if s.DecayFactor <= 0 || s.DecayFactor >= 1 {
		return baseTTL
	}

	// Calculate decay: later layers (higher index) have longer TTL
	// If we have 3 layers (0, 1, 2) and decayFactor=0.5:
	// L0: baseTTL * 0.25 (0.5^2)
	// L1: baseTTL * 0.5  (0.5^1)
	// L2: baseTTL * 1.0  (0.5^0)
	numLayers := layerIndex + 1
	exponent := float64(numLayers - layerIndex - 1)
	factor := math.Pow(s.DecayFactor, exponent)

	return time.Duration(float64(baseTTL) * factor)
}

// CustomTTLStrategy uses explicit TTL values for each layer.
type CustomTTLStrategy struct {
	TTLs []time.Duration
}

// GetTTL returns the custom TTL for a layer, or baseTTL if not specified.
func (s *CustomTTLStrategy) GetTTL(layerIndex int, baseTTL time.Duration) time.Duration {
	if layerIndex < len(s.TTLs) {
		return s.TTLs[layerIndex]
	}
	return baseTTL
}
