package cache

import (
	"strings"
	"testing"
)

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid key", "user:123", false},
		{"valid simple key", "mykey", false},
		{"valid with numbers", "cache:item:456", false},
		{"valid with underscores", "user_profile_123", false},
		{"valid with dots", "api.v1.users", false},
		{"empty key", "", true},
		{"too long", strings.Repeat("a", 300), true},
		{"control char null", "key\x00value", true},
		{"control char tab", "key\tvalue", true},
		{"control char newline", "key\nvalue", true},
		{"leading space", " key", true},
		{"trailing space", "key ", true},
		{"only spaces", "   ", true},
		{"unicode control", "key\x7fvalue", true}, // DEL character
		{"valid unicode", "café", false},
		{"valid emoji", "key🚀", false},
		{"exactly 250 chars", strings.Repeat("a", 250), false},
		{"251 chars", strings.Repeat("a", 251), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedKey string
		expectError bool
	}{
		{"trim spaces", "  key  ", "key", false},
		{"remove control chars", "key\x00value\x01", "keyvalue", false},
		{"truncate long key", strings.Repeat("a", 300), strings.Repeat("a", 250), false},
		{"empty after sanitize", "   \x00\x01   ", "", true},
		{"valid key unchanged", "user:123", "user:123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizeKey(tt.input)
			if result != tt.expectedKey {
				t.Errorf("SanitizeKey(%q) key = %q, want %q", tt.input, result, tt.expectedKey)
			}
			if (err != nil) != tt.expectError {
				t.Errorf("SanitizeKey(%q) error = %v, wantErr %v", tt.input, err, tt.expectError)
			}
		})
	}
}

func TestKeyPattern_Build(t *testing.T) {
	pattern := NewKeyPattern("user", ":")

	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{"no parts", []string{}, "user"},
		{"one part", []string{"123"}, "user:123"},
		{"two parts", []string{"123", "profile"}, "user:123:profile"},
		{"three parts", []string{"123", "posts", "456"}, "user:123:posts:456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pattern.Build(tt.parts...)
			if result != tt.expected {
				t.Errorf("Build(%v) = %q, want %q", tt.parts, result, tt.expected)
			}
		})
	}
}

func TestKeyPattern_MustBuild(t *testing.T) {
	pattern := NewKeyPattern("user", ":")

	// Valid key should work
	result := pattern.MustBuild("123")
	expected := "user:123"
	if result != expected {
		t.Errorf("MustBuild() = %q, want %q", result, expected)
	}

	// Invalid key should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustBuild should panic for invalid key")
		}
	}()

	// This should panic because the key would be too long
	longParts := make([]string, 100)
	for i := range longParts {
		longParts[i] = "verylongpart"
	}
	pattern.MustBuild(longParts...)
}

func TestNewKeyPattern(t *testing.T) {
	// Default separator
	pattern := NewKeyPattern("test", "")
	result := pattern.Build("a", "b")
	expected := "test:a:b"
	if result != expected {
		t.Errorf("NewKeyPattern with empty separator = %q, want %q", result, expected)
	}

	// Custom separator
	pattern2 := NewKeyPattern("test", "|")
	result2 := pattern2.Build("a", "b")
	expected2 := "test|a|b"
	if result2 != expected2 {
		t.Errorf("NewKeyPattern with custom separator = %q, want %q", result2, expected2)
	}
}
