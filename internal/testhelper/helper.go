package test

import (
	"os"
	"testing"

	"github.com/meru143/dbdiff/internal/config"
)

// TestHelper provides utility functions for tests
type TestHelper struct {
	t *testing.T
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{t: t}
}

// CreateTempConfig creates a temporary config file
func (h *TestHelper) CreateTempConfig(content string) string {
	tmpFile, err := os.CreateTemp("", "dbdiff-test-*.yaml")
	if err != nil {
		h.t.Fatalf("Failed to create temp config: %v", err)
	}
	if _, err := tmpFile.WriteString(content); err != nil {
		os.Remove(tmpFile.Name())
		h.t.Fatalf("Failed to write temp config: %v", err)
	}
	tmpFile.Close()
	return tmpFile.Name()
}

// CleanupConfig removes a temp config file
func (h *TestHelper) CleanupConfig(path string) {
	os.Remove(path)
}

// RequireConfig loads config and fails test on error
func (h *TestHelper) RequireConfig(flags map[string]interface{}) *config.Config {
	cfg, err := config.Load(flags)
	if err != nil {
		h.t.Fatalf("Failed to load config: %v", err)
	}
	return cfg
}

// AssertNoError fails test if err is not nil
func (h *TestHelper) AssertNoError(err error) {
	if err != nil {
		h.t.Fatalf("Unexpected error: %v", err)
	}
}

// AssertError fails test if err is nil
func (h *TestHelper) AssertError(err error) {
	if err == nil {
		h.t.Fatalf("Expected error but got none")
	}
}

// AssertEqual fails test if a != b
func (h *TestHelper) AssertEqual(a, b interface{}) {
	if a != b {
		h.t.Fatalf("Expected %v, got %v", b, a)
	}
}

// SetupTestDB returns a test database URL
func SetupTestDB(t *testing.T) string {
	// For now, return empty - integration tests handle actual DB
	t.Skip("Skipping DB test - use integration tests")
	return ""
}
