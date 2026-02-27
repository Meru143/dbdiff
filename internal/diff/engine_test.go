package diff

import (
	"testing"

	"github.com/meru143/dbdiff/pkg/types"
)

func TestCompare_EmptySchemas(t *testing.T) {
	source := &types.Schema{Tables: []types.Table{}}
	target := &types.Schema{Tables: []types.Table{}}

	result := Compare(source, target)

	if len(result) != 0 {
		t.Errorf("Expected 0 differences, got %d", len(result))
	}
}

func TestCompare_NewTable(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{Name: "users", Columns: []types.Column{{Name: "id", DataType: "integer"}}},
		},
	}
	target := &types.Schema{Tables: []types.Table{}}

	result := Compare(source, target)

	if len(result) != 1 {
		t.Fatalf("Expected 1 difference, got %d", len(result))
	}

	if result[0].Type != types.DiffAdd {
		t.Errorf("Expected ADD, got %s", result[0].Type)
	}

	if result[0].Object != types.ObjectTable {
		t.Errorf("Expected TABLE, got %s", result[0].Object)
	}

	if result[0].Name != "users" {
		t.Errorf("Expected users, got %s", result[0].Name)
	}
}

func TestCompare_DroppedTable(t *testing.T) {
	source := &types.Schema{Tables: []types.Table{}}
	target := &types.Schema{
		Tables: []types.Table{
			{Name: "users"},
		},
	}

	result := Compare(source, target)

	if len(result) != 1 {
		t.Fatalf("Expected 1 difference, got %d", len(result))
	}

	if result[0].Type != types.DiffDrop {
		t.Errorf("Expected DROP, got %s", result[0].Type)
	}
}

func TestCompare_NewColumn(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "id", DataType: "integer"},
					{Name: "email", DataType: "varchar"},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "id", DataType: "integer"},
				},
			},
		},
	}

	result := Compare(source, target)

	if len(result) != 1 {
		t.Fatalf("Expected 1 difference, got %d", len(result))
	}

	if result[0].Type != types.DiffAdd {
		t.Errorf("Expected ADD, got %s", result[0].Type)
	}

	if result[0].Object != types.ObjectColumn {
		t.Errorf("Expected COLUMN, got %s", result[0].Object)
	}

	if result[0].Name != "email" {
		t.Errorf("Expected email, got %s", result[0].Name)
	}
}

func TestCompare_TypeChange(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "id", DataType: "bigint"},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "id", DataType: "integer"},
				},
			},
		},
	}

	result := Compare(source, target)

	if len(result) != 1 {
		t.Fatalf("Expected 1 difference, got %d", len(result))
	}

	if result[0].Type != types.DiffAlter {
		t.Errorf("Expected ALTER, got %s", result[0].Type)
	}

	if result[0].OldValue != "integer" {
		t.Errorf("Expected old value integer, got %s", result[0].OldValue)
	}

	if result[0].NewValue != "bigint" {
		t.Errorf("Expected new value bigint, got %s", result[0].NewValue)
	}
}

func TestCompare_IndexDiff(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Indexes: []types.Index{
					{Name: "users_email_idx", Definition: "CREATE INDEX users_email_idx ON users(email)"},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Indexes: []types.Index{},
			},
		},
	}

	result := Compare(source, target)

	// Should have 1 diff for the new index
	found := false
	for _, diff := range result {
		if diff.Object == types.ObjectIndex && diff.Name == "users_email_idx" {
			found = true
			if diff.Type != types.DiffAdd {
				t.Errorf("Expected ADD, got %s", diff.Type)
			}
		}
	}
	if !found {
		t.Errorf("Expected index diff, got %v", result)
	}
}
