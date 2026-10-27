package output

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// The CLI path: the formatter writes the target's dialect, qualifies names with the schema
// (never with the connection URL), and takes definitions from the source schema.
func TestFormatMigration_TargetDialect(t *testing.T) {
	name := "name"
	source := &types.Schema{Tables: []types.Table{{Name: "users", Columns: []types.Column{
		{Name: "id", DataType: "int"},
		{Name: name, DataType: "varchar(100)"},
	}}}}
	target := &types.Schema{Tables: []types.Table{{Name: "users", Columns: []types.Column{
		{Name: "id", DataType: "int"},
		{Name: name, DataType: "varchar(100)", IsNullable: true},
	}}}}
	diffs := dbdiff.Compare(source, target)

	tests := []struct {
		dialect, schema string
		want            []string
		notWant         []string
	}{
		{dialect: "postgresql", schema: "app",
			want:    []string{"BEGIN;", "SET lock_timeout = '10s';", "ALTER TABLE app.users ALTER COLUMN name SET NOT NULL;", "COMMIT;"},
			notWant: []string{"postgres://"}},
		{dialect: "mysql", schema: "public",
			want:    []string{"START TRANSACTION;", "SET SESSION lock_wait_timeout = 10;", "ALTER TABLE users MODIFY COLUMN name varchar(100) NOT NULL;", "COMMIT;"},
			notWant: []string{"BEGIN;", "VARCHAR(255)", "lock_timeout =", "postgres://"}},
		{dialect: "sqlserver", schema: "dbo",
			want:    []string{"SET XACT_ABORT ON;\nBEGIN TRANSACTION;", "SET LOCK_TIMEOUT 10000;", "ALTER TABLE dbo.users ALTER COLUMN name varchar(100) NOT NULL;", "COMMIT TRANSACTION;"},
			notWant: []string{"BEGIN;", "COMMIT;", "DO $$", "lock_timeout =", "postgres://"}},
	}
	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			formatter := NewFormatterWithOptions(FormatOptions{Format: "sql", Dialect: tt.dialect, Schema: tt.schema,
				SourceSchema: source, SourceDB: "postgres://user:pass@localhost:5432/db"})
			result, err := formatter.FormatMigration(diffs, true)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			for _, w := range tt.want {
				if !contains(result, w) {
					t.Errorf("Expected %q in:\n%s", w, result)
				}
			}
			for _, w := range tt.notWant {
				if contains(result, w) {
					t.Errorf("Unexpected %q in:\n%s", w, result)
				}
			}
		})
	}
}

func TestFormatMigration_ExecutableIsPostgresOnly(t *testing.T) {
	diffs := types.DiffList{{Type: types.DiffDrop, Object: types.ObjectTable, Name: "audit"}}
	for _, dialect := range []string{"mysql", "sqlserver"} {
		formatter := NewFormatterWithOptions(FormatOptions{Format: "sql", IsExecutable: true, Dialect: dialect})
		if _, err := formatter.FormatMigration(diffs, true); err == nil {
			t.Errorf("%s: expected an error for a psql script", dialect)
		}
	}
	formatter := NewFormatterWithOptions(FormatOptions{Format: "sql", IsExecutable: true, Dialect: "postgresql"})
	if script, err := formatter.FormatMigration(diffs, true); err != nil || !contains(script, "psql") {
		t.Errorf("expected a psql script, got %v:\n%s", err, script)
	}
}

// compare --format json prints a valid result when nothing differs: an empty list, not null.
func TestFormatJSON_NoDiffs(t *testing.T) {
	out, err := NewFormatterWithOptions(FormatOptions{Format: "json", IncludeMeta: true}).Format(nil)
	if err != nil {
		t.Fatal(err)
	}
	var result struct{ Diffs []types.Diff }
	if err := json.Unmarshal([]byte(out), &result); err != nil || result.Diffs == nil || len(result.Diffs) != 0 {
		t.Errorf("expected JSON with an empty diffs list, got %v:\n%s", err, out)
	}
}

// A migration file ends with a newline after its COMMIT;.
func TestWriteFileEndsWithNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migration.sql")
	for content, want := range map[string]string{"BEGIN;\n\nCOMMIT;": "BEGIN;\n\nCOMMIT;\n", "SELECT 1;\n": "SELECT 1;\n"} {
		if err := WriteFile(path, content); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(path); string(got) != want {
			t.Errorf("WriteFile(%q) wrote %q, want %q", content, got, want)
		}
	}
}
