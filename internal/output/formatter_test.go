package output

import (
	"testing"

	dbdiff "github.com/meru143/dbdiff/internal/diff"
	"github.com/meru143/dbdiff/pkg/types"
)

func TestFormatSQL(t *testing.T) {
	formatter := NewFormatter("sql")
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
		{Type: types.DiffAdd, Object: types.ObjectColumn, Name: "email", TableName: "users"},
	}

	result := formatter.formatSQL(diffs, false)

	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}

	// Check for expected content
	expected := []string{"-- Create table: users", "-- Add column: email to users"}
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

func TestFormatExecutable(t *testing.T) {
	opts := FormatOptions{
		Format:       "sql",
		IsExecutable: true,
	}
	formatter := NewFormatterWithOptions(opts)
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
	}

	result, err := formatter.FormatMigration(diffs, false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify Bash script wrapper components
	expectedStrings := []string{
		"#!/bin/bash",
		"DB_URL=${1:-$DATABASE_URL}",
		"psql \"$DB_URL\" -c",
		"psql -v ON_ERROR_STOP=1 \"$DB_URL\" << 'EOF'",
		"BEGIN;",
		"-- Create table: users",
		"COMMIT;",
		"EOF",
	}

	for _, exp := range expectedStrings {
		if !contains(result, exp) {
			t.Errorf("Expected executable script to contain %q", exp)
		}
	}
}

func TestFormatMigration_IndexConcurrentlyOnlyOutsideTransaction(t *testing.T) {
	idx := types.Index{Name: "idx_users_email", Columns: []string{"email"},
		Definition: "CREATE INDEX idx_users_email ON public.users USING btree (email)"}
	source := &types.Schema{Tables: []types.Table{{Name: "users", Indexes: []types.Index{idx}}}}
	target := &types.Schema{Tables: []types.Table{{Name: "users"}}}
	diffs := dbdiff.Compare(source, target)

	plain := "CREATE INDEX idx_users_email ON public.users USING btree (email);"
	tests := []struct {
		name        string
		executable  bool
		transaction bool
		want        string
	}{
		{name: "wrapped in BEGIN/COMMIT", transaction: true, want: plain},
		{name: "no transaction", transaction: false,
			want: "CREATE INDEX CONCURRENTLY idx_users_email ON public.users USING btree (email);"},
		{name: "executable script always wraps", executable: true, transaction: false, want: plain},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatterWithOptions(FormatOptions{Format: "sql", IsExecutable: tt.executable})
			result, err := formatter.FormatMigration(diffs, tt.transaction)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !contains(result, tt.want) {
				t.Errorf("Expected %q in:\n%s", tt.want, result)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
