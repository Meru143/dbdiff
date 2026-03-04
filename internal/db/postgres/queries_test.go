package postgres

import (
	"testing"
)

// TestIgnorePatterns tests the ignore pattern matching
func TestIgnorePatterns(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		input    string
		expected bool
	}{
		{"exact match", "id", "id", true},
		{"no match", "id", "name", false},
		{"prefix star", "*_at", "created_at", true},
		{"prefix star no match", "*_at", "name", false},
		{"suffix star", "created_*", "created_at", true},
		{"question mark", "id?", "id1", true},
		{"question mark no match", "id?", "id", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesIgnorePatterns(tt.input, []string{tt.pattern})
			if result != tt.expected {
				t.Errorf("matchesIgnorePatterns(%q, %q) = %v, want %v", tt.input, tt.pattern, result, tt.expected)
			}
		})
	}
}
