package cmd

import (
	"strings"
	"testing"

	"github.com/meru143/dbdiff/internal/config"
	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/pkg/types"
)

func driver(t *testing.T, url string) db.Driver {
	t.Helper()
	d, err := db.NewDriver(url)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// migrate, interactive and compare hand the formatter the target's dialect, the schema to
// qualify names with and the source schema; before, they passed none of them and every
// database got PostgreSQL SQL.
func TestSQLOptions(t *testing.T) {
	source := &types.Schema{Tables: []types.Table{{Name: "users"}}}
	tests := []struct {
		url, dialect string
	}{
		{"postgres://u:p@localhost/db", "postgresql"},
		{"mysql://u:p@tcp(localhost:3306)/db", "mysql"},
		{"sqlserver://u:p@localhost?database=db", "sqlserver"},
	}
	for _, tt := range tests {
		opts := sqlOptions(&config.Config{Schema: "app", Executable: true}, driver(t, tt.url), source)
		if opts.Dialect != tt.dialect || opts.Schema != "app" || opts.SourceSchema != source ||
			opts.Format != "sql" || !opts.IsExecutable {
			t.Errorf("%s: got %+v", tt.url, opts)
		}
	}
}

func TestSameEngine(t *testing.T) {
	pg, my := driver(t, "postgres://localhost/a"), driver(t, "mysql://u:p@tcp(localhost:3306)/b")
	if err := sameEngine(pg, driver(t, "postgresql://localhost/b")); err != nil {
		t.Errorf("same engine: %v", err)
	}
	if err := sameEngine(pg, my); err == nil {
		t.Error("expected an error for a PostgreSQL to MySQL migration")
	}
}

func TestInTransaction(t *testing.T) {
	tests := []struct {
		dialect, sql, begin string
	}{
		{"postgresql", "DROP TABLE a;", "BEGIN;\n"},
		{"mysql", "DROP TABLE a;", "START TRANSACTION;\n"},
		{"sqlserver", "DROP TABLE a;", "SET XACT_ABORT ON;\nBEGIN TRANSACTION;\n"},
		// A file that manages its own transaction is left alone.
		{"sqlserver", "SET XACT_ABORT ON;\nBEGIN TRANSACTION;\nDROP TABLE a;\nCOMMIT TRANSACTION;", ""},
		{"mysql", "START TRANSACTION;\nDROP TABLE a;\nCOMMIT;", ""},
		{"postgresql", "BEGIN;\nDROP TABLE a;\nCOMMIT;", ""},
	}
	for _, tt := range tests {
		got := inTransaction(tt.dialect, tt.sql)
		if tt.begin == "" {
			if got != tt.sql {
				t.Errorf("%s: rewrapped %q as %q", tt.dialect, tt.sql, got)
			}
			continue
		}
		if !strings.Contains(got, tt.begin) || strings.Count(got, "DROP TABLE a;") != 1 {
			t.Errorf("%s: expected %q in %q", tt.dialect, tt.begin, got)
		}
	}
}

func TestTypeNormalizer(t *testing.T) {
	ms := driver(t, "sqlserver://u:p@localhost?database=a")
	// SQL Server's normalizer folds int and bigint together: within one engine that would
	// hide a type change, so types compare exactly.
	if typeNormalizer(ms, driver(t, "sqlserver://u:p@localhost?database=b")) != nil {
		t.Error("expected exact type comparison within one engine")
	}
	if typeNormalizer(ms, driver(t, "postgres://localhost/b")) == nil {
		t.Error("expected the source's normalizer across engines")
	}
}
