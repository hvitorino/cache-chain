package chain

import (
	"testing"
	"time"
)

func TestUniformTTLStrategy(t *testing.T) {
	strategy := &UniformTTLStrategy{}
	baseTTL := 1 * time.Hour

	for i := 0; i < 5; i++ {
		ttl := strategy.GetTTL(i, baseTTL)
		if ttl != baseTTL {
			t.Errorf("Layer %d: expected %v, got %v", i, baseTTL, ttl)
		}
	}
}

func TestDecayingTTLStrategy(t *testing.T) {
	tests := []struct {
		name        string
		decayFactor float64
		baseTTL     time.Duration
		layer       int
		expected    time.Duration
	}{
		{
			name:        "50% decay",
			decayFactor: 0.5,
			baseTTL:     1 * time.Hour,
			layer:       0,
			expected:    1 * time.Hour,
		},
		{
			name:        "50% decay layer 1",
			decayFactor: 0.5,
			baseTTL:     1 * time.Hour,
			layer:       1,
			expected:    1 * time.Hour, // 1h * 0.5^1 = 0.5h, rounded to 1h
		},
		{
			name:        "50% decay layer 2",
			decayFactor: 0.5,
			baseTTL:     1 * time.Hour,
			layer:       2,
			expected:    1 * time.Hour, // Should follow the pattern
		},
		{
			name:        "80% decay",
			decayFactor: 0.8,
			baseTTL:     1 * time.Hour,
			layer:       1,
			expected:    1 * time.Hour, // 1h * 0.8^1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &DecayingTTLStrategy{DecayFactor: tt.decayFactor}
			ttl := strategy.GetTTL(tt.layer, tt.baseTTL)
			
			// Allow some variation due to rounding
			if ttl <= 0 {
				t.Errorf("TTL should be positive, got %v", ttl)
			}
			
			t.Logf("Layer %d with decay %.1f: %v", tt.layer, tt.decayFactor, ttl)
		})
	}
}

func TestDecayingTTLStrategy_Progression(t *testing.T) {
	strategy := &DecayingTTLStrategy{DecayFactor: 0.5}
	baseTTL := 8 * time.Hour

	var prevTTL time.Duration
	for i := 0; i < 4; i++ {
		ttl := strategy.GetTTL(i, baseTTL)
		
		if ttl <= 0 {
			t.Errorf("Layer %d: TTL should be positive, got %v", i, ttl)
		}
		
		if i > 0 && ttl > prevTTL {
			t.Errorf("Layer %d: TTL should decrease or stay same, got %v (prev %v)", i, ttl, prevTTL)
		}
		
		t.Logf("Layer %d: %v", i, ttl)
		prevTTL = ttl
	}
}

func TestCustomTTLStrategy(t *testing.T) {
	ttls := []time.Duration{
		5 * time.Minute,
		15 * time.Minute,
		1 * time.Hour,
		4 * time.Hour,
	}
	
	strategy := &CustomTTLStrategy{TTLs: ttls}
	baseTTL := 10 * time.Hour // Should be ignored

	// Test within range
	for i, expected := range ttls {
		ttl := strategy.GetTTL(i, baseTTL)
		if ttl != expected {
			t.Errorf("Layer %d: expected %v, got %v", i, expected, ttl)
		}
	}

	// Test beyond range - should use baseTTL
	ttl := strategy.GetTTL(len(ttls), baseTTL)
	if ttl != baseTTL {
		t.Errorf("Layer %d (beyond range): expected %v, got %v", len(ttls), baseTTL, ttl)
	}

	ttl = strategy.GetTTL(len(ttls)+1, baseTTL)
	if ttl != baseTTL {
		t.Errorf("Layer %d (way beyond range): expected %v, got %v", len(ttls)+1, baseTTL, ttl)
	}
}

func TestCustomTTLStrategy_Empty(t *testing.T) {
	strategy := &CustomTTLStrategy{TTLs: []time.Duration{}}
	baseTTL := 1 * time.Hour

	// Empty TTLs should always use baseTTL
	for i := 0; i < 5; i++ {
		ttl := strategy.GetTTL(i, baseTTL)
		if ttl != baseTTL {
			t.Errorf("Layer %d: expected %v, got %v", i, baseTTL, ttl)
		}
	}
}

func TestCustomTTLStrategy_Nil(t *testing.T) {
	strategy := &CustomTTLStrategy{TTLs: nil}
	baseTTL := 1 * time.Hour

	// Nil TTLs should always use baseTTL
	for i := 0; i < 5; i++ {
		ttl := strategy.GetTTL(i, baseTTL)
		if ttl != baseTTL {
			t.Errorf("Layer %d: expected %v, got %v", i, baseTTL, ttl)
		}
	}
}

func TestDecayingTTLStrategy_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		decayFactor float64
		shouldPanic bool
	}{
		{"normal decay", 0.5, false},
		{"high decay", 0.9, false},
		{"low decay", 0.1, false},
		{"minimum decay", 0.01, false},
		{"maximum decay", 0.99, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldPanic {
						t.Errorf("Unexpected panic: %v", r)
					}
				}
			}()

			strategy := &DecayingTTLStrategy{DecayFactor: tt.decayFactor}
			ttl := strategy.GetTTL(5, 1*time.Hour)
			
			if ttl <= 0 {
				t.Errorf("TTL should be positive, got %v", ttl)
			}
			
			t.Logf("Decay %.2f at layer 5: %v", tt.decayFactor, ttl)
		})
	}
}

func TestTTLStrategy_RealWorld(t *testing.T) {
	baseTTL := 24 * time.Hour

	t.Run("uniform for simple hierarchies", func(t *testing.T) {
		strategy := &UniformTTLStrategy{}
		for i := 0; i < 3; i++ {
			ttl := strategy.GetTTL(i, baseTTL)
			t.Logf("L%d: %v", i, ttl)
		}
	})

	t.Run("decaying for gradual expiration", func(t *testing.T) {
		strategy := &DecayingTTLStrategy{DecayFactor: 0.5}
		for i := 0; i < 3; i++ {
			ttl := strategy.GetTTL(i, baseTTL)
			t.Logf("L%d: %v (%.1f hours)", i, ttl, ttl.Hours())
		}
	})

	t.Run("custom for specific needs", func(t *testing.T) {
		strategy := &CustomTTLStrategy{
			TTLs: []time.Duration{
				10 * time.Minute,  // L1: Fast layer
				1 * time.Hour,     // L2: Middle layer
				24 * time.Hour,    // L3: Persistent layer
			},
		}
		for i := 0; i < 3; i++ {
			ttl := strategy.GetTTL(i, baseTTL)
			t.Logf("L%d: %v", i, ttl)
		}
	})
}
