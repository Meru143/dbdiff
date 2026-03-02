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
				Name:    "users",
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

func TestCompare_DroppedColumn(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "id", DataType: "integer"},
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
					{Name: "email", DataType: "varchar"},
				},
			},
		},
	}

	result := Compare(source, target)

	// Should detect dropped column
	found := false
	for _, diff := range result {
		if diff.Object == types.ObjectColumn && diff.Type == types.DiffDrop && diff.Name == "email" {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected dropped column diff for 'email', got %v", result)
	}
}

func TestCompare_DefaultValueChange(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name: "posts",
				Columns: []types.Column{
					{Name: "id", DataType: "integer"},
					{Name: "published", DataType: "bool", DefaultValue: strPtr("true")},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name: "posts",
				Columns: []types.Column{
					{Name: "id", DataType: "integer"},
					{Name: "published", DataType: "bool"},
				},
			},
		},
	}

	result := Compare(source, target)

	found := false
	for _, diff := range result {
		if diff.Object == types.ObjectColumn && diff.Type == types.DiffAlter && diff.Name == "published" {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected default value change diff, got %v", result)
	}
}

func TestCompare_DroppedIndex(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "users",
				Indexes: []types.Index{},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Indexes: []types.Index{
					{Name: "users_email_idx", Columns: []string{"email"}},
				},
			},
		},
	}

	result := Compare(source, target)

	found := false
	for _, diff := range result {
		if diff.Object == types.ObjectIndex && diff.Type == types.DiffDrop && diff.Name == "users_email_idx" {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected dropped index diff, got %v", result)
	}
}

func TestCompare_NewForeignKey(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "posts",
				Columns: []types.Column{{Name: "id", DataType: "integer"}},
				ForeignKeys: []types.ForeignKey{
					{
						Name:       "posts_user_fk",
						Columns:    []string{"user_id"},
						RefTable:   "users",
						RefColumns: []string{"id"},
					},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "posts",
				Columns: []types.Column{{Name: "id", DataType: "integer"}},
			},
		},
	}

	result := Compare(source, target)

	found := false
	for _, diff := range result {
		if diff.Object == types.ObjectForeignKey && diff.Type == types.DiffAdd && diff.Name == "posts_user_fk" {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected new FK diff, got %v", result)
	}
}

func TestCompare_DroppedForeignKey(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "posts",
				Columns: []types.Column{{Name: "id", DataType: "integer"}},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "posts",
				Columns: []types.Column{{Name: "id", DataType: "integer"}},
				ForeignKeys: []types.ForeignKey{
					{
						Name:       "posts_user_fk",
						Columns:    []string{"user_id"},
						RefTable:   "users",
						RefColumns: []string{"id"},
					},
				},
			},
		},
	}

	result := Compare(source, target)

	found := false
	for _, diff := range result {
		if diff.Object == types.ObjectForeignKey && diff.Type == types.DiffDrop && diff.Name == "posts_user_fk" {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected dropped FK diff, got %v", result)
	}
}

func TestCompare_Sequence(t *testing.T) {
	source := &types.Schema{
		Sequences: []types.Sequence{
			{Name: "users_id_seq", Start: 100, Increment: 2},
		},
	}
	target := &types.Schema{
		Sequences: []types.Sequence{
			{Name: "users_id_seq", Start: 1, Increment: 1},
		},
	}

	result := Compare(source, target)

	if len(result) == 0 {
		t.Errorf("Expected sequence differences")
	}
}

func strPtr(s string) *string {
	return &s
}
