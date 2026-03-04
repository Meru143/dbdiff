package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/meru143/dbdiff/pkg/types"
)

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

func TestFilterColumns(t *testing.T) {
	columns := []types.Column{
		{Name: "id"},
		{Name: "_created_at"},
		{Name: "valid_name"},
		{Name: "ignore_me"},
	}

	expected := []types.Column{
		{Name: "id"},
		{Name: "valid_name"},
	}

	ignorePatterns := []string{"ignore_*"}

	filtered := filterColumns(columns, ignorePatterns)

	if len(filtered) != len(expected) {
		t.Fatalf("Expected %d columns, got %d", len(expected), len(filtered))
	}

	for i, col := range filtered {
		if col.Name != expected[i].Name {
			t.Errorf("Expected column %s, got %s", expected[i].Name, col.Name)
		}
	}
}

func TestListTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	// Mock valid tables and ignored tables
	rows := sqlmock.NewRows([]string{"table_name"}).
		AddRow("users").
		AddRow("posts").
		AddRow("_created_at") // This should be ignored by DefaultIgnorePatterns

	// The query uses DATABASE() and 'BASE TABLE'
	mock.ExpectQuery(`SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE\(\) AND table_type = 'BASE TABLE'`).
		WillReturnRows(rows)

	tables, err := d.ListTables(ctx, "testdb", DefaultIgnorePatterns) // schema arg is ignored in query string anyway
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(tables))
	}

	if tables[0] != "users" || tables[1] != "posts" {
		t.Errorf("unexpected tables returned: %v", tables)
	}
}

func TestGetServerInfo(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"DATABASE()", "CURRENT_USER()", "VERSION()"}).
		AddRow("testdb", "testuser@localhost", "8.0.35")

	mock.ExpectQuery(`SELECT DATABASE\(\), CURRENT_USER\(\), VERSION\(\)`).
		WillReturnRows(rows)

	info, err := d.GetServerInfo(ctx)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if info.Database != "testdb" || info.User != "testuser@localhost" || info.Version != "8.0.35" {
		t.Errorf("unexpected info returned: %+v", info)
	}
}
