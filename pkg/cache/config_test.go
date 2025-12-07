package cache

import (
	"testing"
	"time"
)

func TestLayerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  LayerConfig
		wantErr bool
	}{
		{
			"valid config",
			LayerConfig{
				Name:       "L1",
				DefaultTTL: time.Minute,
				MaxTTL:     time.Hour,
				Enabled:    true,
			},
			false,
		},
		{
			"empty name",
			LayerConfig{
				Name:       "",
				DefaultTTL: time.Minute,
				MaxTTL:     time.Hour,
				Enabled:    true,
			},
			true,
		},
		{
			"negative default TTL",
			LayerConfig{
				Name:       "L1",
				DefaultTTL: -time.Minute,
				MaxTTL:     time.Hour,
				Enabled:    true,
			},
			true,
		},
		{
			"negative max TTL",
			LayerConfig{
				Name:       "L1",
				DefaultTTL: time.Minute,
				MaxTTL:     -time.Hour,
				Enabled:    true,
			},
			true,
		},
		{
			"default TTL > max TTL",
			LayerConfig{
				Name:       "L1",
				DefaultTTL: time.Hour,
				MaxTTL:     time.Minute,
				Enabled:    true,
			},
			true,
		},
		{
			"zero values ok",
			LayerConfig{
				Name:       "L1",
				DefaultTTL: 0,
				MaxTTL:     0,
				Enabled:    false,
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLayerConfig_EffectiveTTL(t *testing.T) {
	config := LayerConfig{
		Name:       "L1",
		DefaultTTL: time.Minute,
		MaxTTL:     time.Hour,
		Enabled:    true,
	}

	tests := []struct {
		name     string
		ttl      time.Duration
		expected time.Duration
	}{
		{"zero TTL uses default", 0, time.Minute},
		{"normal TTL unchanged", time.Minute * 30, time.Minute * 30},
		{"TTL exceeding max is capped", time.Hour * 2, time.Hour},
		{"negative TTL uses default", -time.Minute, time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.EffectiveTTL(tt.ttl)
			if result != tt.expected {
				t.Errorf("EffectiveTTL(%v) = %v, want %v", tt.ttl, result, tt.expected)
			}
		})
	}
}

func TestLayerConfig_EffectiveTTL_NoMax(t *testing.T) {
	config := LayerConfig{
		Name:       "L1",
		DefaultTTL: time.Minute,
		MaxTTL:     0, // No max
		Enabled:    true,
	}

	tests := []struct {
		name     string
		ttl      time.Duration
		expected time.Duration
	}{
		{"zero TTL uses default", 0, time.Minute},
		{"large TTL allowed", time.Hour * 24, time.Hour * 24},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.EffectiveTTL(tt.ttl)
			if result != tt.expected {
				t.Errorf("EffectiveTTL(%v) = %v, want %v", tt.ttl, result, tt.expected)
			}
		})
	}
}

func TestLayerConfig_Fields(t *testing.T) {
	config := LayerConfig{
		Name:       "test-layer",
		DefaultTTL: time.Minute * 5,
		MaxTTL:     time.Hour,
		Enabled:    true,
	}

	if config.Name != "test-layer" {
		t.Errorf("Name = %q, want %q", config.Name, "test-layer")
	}
	if config.DefaultTTL != time.Minute*5 {
		t.Errorf("DefaultTTL = %v, want %v", config.DefaultTTL, time.Minute*5)
	}
	if config.MaxTTL != time.Hour {
		t.Errorf("MaxTTL = %v, want %v", config.MaxTTL, time.Hour)
	}
	if !config.Enabled {
		t.Error("Enabled should be true")
	}
}
