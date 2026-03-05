package diff

import (
	"strings"
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

func TestCompare_View(t *testing.T) {
	source := &types.Schema{
		Views: []types.View{
			{Name: "active_users", Definition: "SELECT * FROM users WHERE active = true"},
		},
	}
	target := &types.Schema{
		Views: []types.View{
			{Name: "active_users", Definition: "SELECT id, name FROM users WHERE active = true"},
		},
	}

	result := Compare(source, target)

	if len(result) != 1 {
		t.Fatalf("Expected 1 difference, got %d", len(result))
	}
	if result[0].Object != types.ObjectView || result[0].Type != types.DiffAlter {
		t.Errorf("Expected ALTER VIEW, got %s %s", result[0].Type, result[0].Object)
	}
}

func TestCompare_MaterializedView(t *testing.T) {
	source := &types.Schema{
		MaterializedViews: []types.MaterializedView{
			{Name: "daily_stats", Definition: "SELECT date_trunc('day', created_at) FROM logs"},
		},
	}
	target := &types.Schema{} // Target has no materialized views initially

	result := Compare(source, target)

	if len(result) != 1 {
		t.Fatalf("Expected 1 difference, got %d", len(result))
	}
	if result[0].Object != types.ObjectMaterializedView || result[0].Type != types.DiffAdd {
		t.Errorf("Expected ADD MATERIALIZED_VIEW, got %s %s", result[0].Type, result[0].Object)
	}
}

func TestCompare_Function(t *testing.T) {
	source := &types.Schema{
		Functions: []types.Function{
			{Name: "get_user", Arguments: "integer", Definition: "SELECT * FROM users WHERE id = $1"},
		},
	}
	target := &types.Schema{
		Functions: []types.Function{
			{Name: "get_user", Arguments: "text", Definition: "SELECT * FROM users WHERE name = $1"}, // Overload drop case
		},
	}

	result := Compare(source, target)

	if len(result) != 2 {
		t.Fatalf("Expected 2 differences (Add get_user(integer), Drop get_user(text)), got %d", len(result))
	}
}

func TestCompare_Trigger(t *testing.T) {
	source := &types.Schema{
		Triggers: []types.Trigger{
			{Table: "users", Name: "update_timestamp", Definition: "EXECUTE FUNCTION update_ts()"},
		},
	}
	target := &types.Schema{
		Triggers: []types.Trigger{
			{Table: "users", Name: "update_timestamp", Definition: "EXECUTE PROCEDURE update_ts()"},
		},
	}

	result := Compare(source, target)

	if len(result) != 1 || result[0].Object != types.ObjectTrigger || result[0].Type != types.DiffAlter {
		t.Errorf("Expected ALTER TRIGGER, got %v", result)
	}
}

func TestCompare_Grant(t *testing.T) {
	source := &types.Schema{
		Grants: []types.Grant{
			{Table: "users", Grantee: "web_api", Privilege: "SELECT", IsGrantable: true},
		},
	}
	target := &types.Schema{}

	result := Compare(source, target)

	if len(result) != 1 || result[0].Object != types.ObjectGrant || result[0].Type != types.DiffAdd {
		t.Errorf("Expected ADD GRANT, got %v", result)
	}
}

// --- Additional edge-case tests ---

func TestCompare_DropView(t *testing.T) {
	source := &types.Schema{}
	target := &types.Schema{
		Views: []types.View{
			{Name: "old_view", Definition: "SELECT 1"},
		},
	}

	result := Compare(source, target)
	if len(result) != 1 || result[0].Type != types.DiffDrop || result[0].Object != types.ObjectView {
		t.Errorf("Expected DROP VIEW, got %v", result)
	}
}

func TestCompare_AddView(t *testing.T) {
	source := &types.Schema{
		Views: []types.View{
			{Name: "new_view", Definition: "SELECT 1"},
		},
	}
	target := &types.Schema{}

	result := Compare(source, target)
	if len(result) != 1 || result[0].Type != types.DiffAdd || result[0].Object != types.ObjectView {
		t.Errorf("Expected ADD VIEW, got %v", result)
	}
}

func TestCompare_Sequence_Add(t *testing.T) {
	source := &types.Schema{
		Sequences: []types.Sequence{
			{Name: "user_id_seq", Start: 1, Increment: 1},
		},
	}
	target := &types.Schema{}

	result := Compare(source, target)
	if len(result) != 1 || result[0].Type != types.DiffAdd || result[0].Object != types.ObjectSequence {
		t.Errorf("Expected ADD SEQUENCE, got %v", result)
	}
}

func TestCompare_Sequence_Drop(t *testing.T) {
	source := &types.Schema{}
	target := &types.Schema{
		Sequences: []types.Sequence{
			{Name: "old_seq", Start: 1, Increment: 1},
		},
	}

	result := Compare(source, target)
	if len(result) != 1 || result[0].Type != types.DiffDrop || result[0].Object != types.ObjectSequence {
		t.Errorf("Expected DROP SEQUENCE, got %v", result)
	}
}

func TestCompare_Sequence_Alter(t *testing.T) {
	source := &types.Schema{
		Sequences: []types.Sequence{
			{Name: "user_id_seq", Start: 1, Increment: 1, MinValue: 1, MaxValue: 100},
		},
	}
	target := &types.Schema{
		Sequences: []types.Sequence{
			{Name: "user_id_seq", Start: 1, Increment: 5, MinValue: 1, MaxValue: 100},
		},
	}

	result := Compare(source, target)
	found := false
	for _, d := range result {
		if d.Object == types.ObjectSequence && d.Type == types.DiffAlter {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected ALTER SEQUENCE for increment change, got %v", result)
	}
}

func TestCompare_Type_Add(t *testing.T) {
	source := &types.Schema{
		Types: []types.Type{
			{Name: "status_enum", Kind: "enum", Values: []string{"active", "inactive"}},
		},
	}
	target := &types.Schema{}

	result := Compare(source, target)
	if len(result) != 1 || result[0].Type != types.DiffAdd {
		t.Errorf("Expected ADD TYPE, got %v", result)
	}
}

func TestCompare_Type_Drop(t *testing.T) {
	source := &types.Schema{}
	target := &types.Schema{
		Types: []types.Type{
			{Name: "old_type", Kind: "enum", Values: []string{"a", "b"}},
		},
	}

	result := Compare(source, target)
	if len(result) != 1 || result[0].Type != types.DiffDrop {
		t.Errorf("Expected DROP TYPE, got %v", result)
	}
}

func TestCompare_Type_SameName_NoDiff(t *testing.T) {
	// The engine only detects add/drop for types, not value changes
	source := &types.Schema{
		Types: []types.Type{
			{Name: "status_enum", Kind: "enum", Values: []string{"active", "inactive", "pending"}},
		},
	}
	target := &types.Schema{
		Types: []types.Type{
			{Name: "status_enum", Kind: "enum", Values: []string{"active", "inactive"}},
		},
	}

	result := Compare(source, target)
	// Types with the same name are not compared for alterations in the current engine
	for _, d := range result {
		if d.Name == "status_enum" {
			t.Errorf("Did not expect diff for same-named type, got %v", d)
		}
	}
}

func TestCompare_Index_Add(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "users",
				Columns: []types.Column{{Name: "id", DataType: "integer"}, {Name: "email", DataType: "varchar"}},
				Indexes: []types.Index{
					{Name: "idx_email", Columns: []string{"email"}, IsUnique: true},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "users",
				Columns: []types.Column{{Name: "id", DataType: "integer"}, {Name: "email", DataType: "varchar"}},
			},
		},
	}

	result := Compare(source, target)
	found := false
	for _, d := range result {
		if d.Object == types.ObjectIndex && d.Type == types.DiffAdd {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected ADD INDEX, got %v", result)
	}
}

func TestCompare_Index_Drop(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "users",
				Columns: []types.Column{{Name: "id", DataType: "integer"}},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "users",
				Columns: []types.Column{{Name: "id", DataType: "integer"}},
				Indexes: []types.Index{
					{Name: "idx_old", Columns: []string{"id"}},
				},
			},
		},
	}

	result := Compare(source, target)
	found := false
	for _, d := range result {
		if d.Object == types.ObjectIndex && d.Type == types.DiffDrop {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected DROP INDEX, got %v", result)
	}
}

func TestCompare_Constraint_Add(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "orders",
				Columns: []types.Column{{Name: "id", DataType: "integer"}, {Name: "amount", DataType: "numeric"}},
				Constraints: []types.Constraint{
					{Name: "chk_amount", Type: "CHECK", Columns: []string{"amount"}},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "orders",
				Columns: []types.Column{{Name: "id", DataType: "integer"}, {Name: "amount", DataType: "numeric"}},
			},
		},
	}

	result := Compare(source, target)
	found := false
	for _, d := range result {
		if d.Object == types.ObjectConstraint && d.Type == types.DiffAdd {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected ADD CONSTRAINT, got %v", result)
	}
}

func TestCompare_ForeignKey_Add(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "orders",
				Columns: []types.Column{{Name: "id", DataType: "integer"}, {Name: "user_id", DataType: "integer"}},
				ForeignKeys: []types.ForeignKey{
					{Name: "fk_user", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}, OnDelete: "CASCADE"},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "orders",
				Columns: []types.Column{{Name: "id", DataType: "integer"}, {Name: "user_id", DataType: "integer"}},
			},
		},
	}

	result := Compare(source, target)
	found := false
	for _, d := range result {
		if d.Object == types.ObjectForeignKey && d.Type == types.DiffAdd {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected ADD FOREIGN KEY, got %v", result)
	}
}

func TestCompare_ForeignKey_Alter(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "orders",
				Columns: []types.Column{{Name: "id", DataType: "integer"}, {Name: "user_id", DataType: "integer"}},
				ForeignKeys: []types.ForeignKey{
					{Name: "fk_user", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}, OnDelete: "CASCADE"},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name:    "orders",
				Columns: []types.Column{{Name: "id", DataType: "integer"}, {Name: "user_id", DataType: "integer"}},
				ForeignKeys: []types.ForeignKey{
					{Name: "fk_user", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}, OnDelete: "SET NULL"},
				},
			},
		},
	}

	result := Compare(source, target)
	found := false
	for _, d := range result {
		if d.Object == types.ObjectForeignKey && d.Type == types.DiffAlter {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected ALTER FOREIGN KEY for OnDelete change, got %v", result)
	}
}

func TestCompare_DropMaterializedView(t *testing.T) {
	source := &types.Schema{}
	target := &types.Schema{
		MaterializedViews: []types.MaterializedView{
			{Name: "old_matview", Definition: "SELECT 1"},
		},
	}

	result := Compare(source, target)
	if len(result) != 1 || result[0].Type != types.DiffDrop || result[0].Object != types.ObjectMaterializedView {
		t.Errorf("Expected DROP MATERIALIZED_VIEW, got %v", result)
	}
}

func TestCompare_AlterMaterializedView(t *testing.T) {
	source := &types.Schema{
		MaterializedViews: []types.MaterializedView{
			{Name: "stats", Definition: "SELECT count(*) FROM users"},
		},
	}
	target := &types.Schema{
		MaterializedViews: []types.MaterializedView{
			{Name: "stats", Definition: "SELECT count(*) FROM orders"},
		},
	}

	result := Compare(source, target)
	found := false
	for _, d := range result {
		if d.Object == types.ObjectMaterializedView && d.Type == types.DiffAlter {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected ALTER MATERIALIZED_VIEW, got %v", result)
	}
}

func TestCompare_DropFunction(t *testing.T) {
	source := &types.Schema{}
	target := &types.Schema{
		Functions: []types.Function{
			{Name: "old_func", Arguments: "integer", Definition: "SELECT 1"},
		},
	}

	result := Compare(source, target)
	if len(result) != 1 || result[0].Type != types.DiffDrop || result[0].Object != types.ObjectFunction {
		t.Errorf("Expected DROP FUNCTION, got %v", result)
	}
}

func TestCompare_DropTrigger(t *testing.T) {
	source := &types.Schema{}
	target := &types.Schema{
		Triggers: []types.Trigger{
			{Table: "users", Name: "old_trigger", Definition: "EXECUTE old()"},
		},
	}

	result := Compare(source, target)
	if len(result) != 1 || result[0].Type != types.DiffDrop || result[0].Object != types.ObjectTrigger {
		t.Errorf("Expected DROP TRIGGER, got %v", result)
	}
}

func TestCompare_DropGrant(t *testing.T) {
	source := &types.Schema{}
	target := &types.Schema{
		Grants: []types.Grant{
			{Table: "users", Grantee: "admin", Privilege: "ALL"},
		},
	}

	result := Compare(source, target)
	if len(result) != 1 || result[0].Type != types.DiffDrop || result[0].Object != types.ObjectGrant {
		t.Errorf("Expected DROP GRANT, got %v", result)
	}
}

func TestCompare_ColumnNullableChange(t *testing.T) {
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "email", DataType: "varchar", IsNullable: false},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "email", DataType: "varchar", IsNullable: true},
				},
			},
		},
	}

	result := Compare(source, target)
	found := false
	for _, d := range result {
		if d.Object == types.ObjectColumn && d.Type == types.DiffAlter {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected ALTER COLUMN for nullable change, got %v", result)
	}
}

func TestCompare_ColumnDefaultChange(t *testing.T) {
	defaultVal := "'active'"
	source := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "status", DataType: "varchar", DefaultValue: &defaultVal},
				},
			},
		},
	}
	target := &types.Schema{
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "status", DataType: "varchar"},
				},
			},
		},
	}

	result := Compare(source, target)
	found := false
	for _, d := range result {
		if d.Object == types.ObjectColumn && d.Type == types.DiffAlter {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected ALTER COLUMN for default change, got %v", result)
	}
}

// --- SQL Server dialect generator tests ---

func TestSQLGenerator_SqlServer_AlterColumn(t *testing.T) {
	diffs := types.DiffList{
		{Object: types.ObjectColumn, Type: types.DiffAlter, Name: "age", TableName: "users",
			OldValue: "int", NewValue: "bigint"},
	}
	gen := NewSQLGenerator(diffs, "dbo", "sqlserver")
	sql := gen.Generate()
	if !containsStr(sql, "ALTER COLUMN age bigint") {
		t.Errorf("Expected ALTER COLUMN for sqlserver, got: %s", sql)
	}
}

func TestSQLGenerator_SqlServer_DropColumn(t *testing.T) {
	diffs := types.DiffList{
		{Object: types.ObjectColumn, Type: types.DiffDrop, Name: "old_col", TableName: "users"},
	}
	gen := NewSQLGenerator(diffs, "dbo", "sqlserver")
	sql := gen.Generate()
	if containsStr(sql, "CASCADE") || containsStr(sql, "IF EXISTS") {
		t.Errorf("SQL Server should not use CASCADE or IF EXISTS for DROP COLUMN, got: %s", sql)
	}
}

func TestSQLGenerator_SqlServer_DropConstraint(t *testing.T) {
	diffs := types.DiffList{
		{Object: types.ObjectConstraint, Type: types.DiffDrop, Name: "uq_email", TableName: "users"},
	}
	gen := NewSQLGenerator(diffs, "dbo", "sqlserver")
	sql := gen.Generate()
	if !containsStr(sql, "DROP CONSTRAINT uq_email") {
		t.Errorf("Expected DROP CONSTRAINT for sqlserver, got: %s", sql)
	}
}

func TestSQLGenerator_SqlServer_DropForeignKey(t *testing.T) {
	diffs := types.DiffList{
		{Object: types.ObjectForeignKey, Type: types.DiffDrop, Name: "fk_user", TableName: "orders"},
	}
	gen := NewSQLGenerator(diffs, "dbo", "sqlserver")
	sql := gen.Generate()
	if !containsStr(sql, "DROP CONSTRAINT fk_user") {
		t.Errorf("Expected DROP CONSTRAINT for FK in sqlserver, got: %s", sql)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && strings.Contains(s, sub)
}
