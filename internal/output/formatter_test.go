package output

import (
	"testing"

	"github.com/meru143/dbdiff/pkg/types"
)

func TestFormatSQL(t *testing.T) {
	formatter := NewFormatter("sql")
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
		{Type: types.DiffAdd, Object: types.ObjectColumn, Name: "email", TableName: "users"},
	}

	result := formatter.formatSQL(diffs)

	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}

	// Check for expected content
	expected := []string{"-- ADD TABLE: users", "-- ADD COLUMN: email", "-- Table: users"}
	for _, exp := range expected {
		if !contains(result, exp) {
			t.Errorf("Expected to contain %q", exp)
		}
	}
}

func TestFormatTable(t *testing.T) {
	formatter := NewFormatter("table")
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
	}

	result := formatter.formatTable(diffs)

	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}

func TestFormatMigration(t *testing.T) {
	formatter := NewFormatter("sql")
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
	}

	// Without transaction
	result, err := formatter.FormatMigration(diffs, false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if contains(result, "BEGIN") {
		t.Error("Should not contain BEGIN")
	}

	// With transaction
	result, err = formatter.FormatMigration(diffs, true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !contains(result, "BEGIN") {
		t.Error("Should contain BEGIN")
	}
	if !contains(result, "COMMIT") {
		t.Error("Should contain COMMIT")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
