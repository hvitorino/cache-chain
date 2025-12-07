package cache

import "time"

// LayerConfig holds configuration for a cache layer.
// It defines the behavior and limits for individual cache implementations.
type LayerConfig struct {
	// Name is the identifier for this layer (e.g., "L1", "redis", "database")
	Name string

	// DefaultTTL is the default time-to-live for cache entries when not specified
	DefaultTTL time.Duration

	// MaxTTL is the maximum allowed TTL for cache entries
	// Values exceeding this will be capped to MaxTTL
	MaxTTL time.Duration

	// Enabled indicates whether this layer is active
	// Disabled layers are skipped during cache operations
	Enabled bool
}

// Validate checks if the configuration is valid.
// Returns an error if any required fields are missing or invalid.
func (c *LayerConfig) Validate() error {
	if c.Name == "" {
		return ErrInvalidValue
	}

	if c.DefaultTTL < 0 {
		return ErrInvalidValue
	}

	if c.MaxTTL < 0 {
		return ErrInvalidValue
	}

	if c.MaxTTL > 0 && c.DefaultTTL > c.MaxTTL {
		return ErrInvalidValue
	}

	return nil
}

// EffectiveTTL returns the effective TTL for a given duration.
// If ttl is 0, returns DefaultTTL.
// If ttl exceeds MaxTTL, returns MaxTTL.
// Otherwise returns the original ttl.
func (c *LayerConfig) EffectiveTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return c.DefaultTTL
	}

	if c.MaxTTL > 0 && ttl > c.MaxTTL {
		return c.MaxTTL
	}

	return ttl
}
