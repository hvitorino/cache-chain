package cache

import (
	"fmt"
	"strings"
	"unicode"
)

// ValidateKey checks if a cache key is valid according to the library's rules.
// Returns nil if the key is valid, or an error describing the problem.
//
// Rules:
// - Non-empty string
// - Maximum length of 250 characters
// - No control characters (0x00-0x1F and 0x7F-0x9F)
// - No leading or trailing whitespace
func ValidateKey(key string) error {
	if key == "" {
		return ErrInvalidKey
	}

	if len(key) > 250 {
		return fmt.Errorf("%w: key too long (max 250 characters)", ErrInvalidKey)
	}

	// Check for control characters
	for _, r := range key {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: key contains control character", ErrInvalidKey)
		}
	}

	// Check for leading/trailing whitespace
	trimmed := strings.TrimSpace(key)
	if len(trimmed) != len(key) {
		return fmt.Errorf("%w: key has leading or trailing whitespace", ErrInvalidKey)
	}

	return nil
}

// SanitizeKey attempts to clean up a key to make it valid.
// This is a best-effort function and should not be relied upon for security.
// Returns the sanitized key and any validation errors that remain.
func SanitizeKey(key string) (string, error) {
	// Trim whitespace
	sanitized := strings.TrimSpace(key)

	// Remove control characters
	var result strings.Builder
	for _, r := range sanitized {
		if !unicode.IsControl(r) {
			result.WriteRune(r)
		}
	}
	sanitized = result.String()

	// Truncate if too long
	if len(sanitized) > 250 {
		sanitized = sanitized[:250]
	}

	// Final validation
	return sanitized, ValidateKey(sanitized)
}

// KeyPattern represents a pattern for generating cache keys.
// Useful for creating consistent key naming conventions.
type KeyPattern struct {
	prefix    string
	separator string
}

// NewKeyPattern creates a new key pattern with the given prefix and separator.
func NewKeyPattern(prefix, separator string) *KeyPattern {
	if separator == "" {
		separator = ":"
	}
	return &KeyPattern{
		prefix:    prefix,
		separator: separator,
	}
}

// Build creates a cache key from the pattern and provided parts.
// Example: pattern.Build("user", "123") -> "user:123"
func (kp *KeyPattern) Build(parts ...string) string {
	if len(parts) == 0 {
		return kp.prefix
	}

	result := kp.prefix
	for _, part := range parts {
		result += kp.separator + part
	}
	return result
}

// MustBuild is like Build but panics if the resulting key is invalid.
// Use with caution - prefer Build with validation.
func (kp *KeyPattern) MustBuild(parts ...string) string {
	key := kp.Build(parts...)
	if err := ValidateKey(key); err != nil {
		panic(fmt.Sprintf("invalid key generated: %v", err))
	}
	return key
}
