package types

import (
	"encoding/json"
	"testing"
)

func TestSchema_JSON(t *testing.T) {
	schema := Schema{
		Tables: []Table{
			{Name: "users"},
		},
		Sequences: []Sequence{
			{Name: "users_id_seq"},
		},
		Types: []Type{
			{Name: "user_status", Kind: "enum", Values: []string{"active", "inactive"}},
		},
		Views: []View{
			{Name: "active_users", Definition: "SELECT * FROM users"},
		},
		MaterializedViews: []MaterializedView{
			{Name: "daily_stats"},
		},
		Functions: []Function{
			{Name: "get_user"},
		},
		Triggers: []Trigger{
			{Name: "update_log"},
		},
		Grants: []Grant{
			{Privilege: "SELECT"},
		},
	}

	// Marshal
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	var decoded Schema
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(decoded.Tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(decoded.Tables))
	}
	if decoded.Tables[0].Name != "users" {
		t.Errorf("Expected table name 'users', got %s", decoded.Tables[0].Name)
	}
	if len(decoded.Sequences) != 1 {
		t.Errorf("Expected 1 sequence, got %d", len(decoded.Sequences))
	}
	if len(decoded.Types) != 1 {
		t.Errorf("Expected 1 type, got %d", len(decoded.Types))
	}
	if len(decoded.Views) != 1 {
		t.Errorf("Expected 1 view, got %d", len(decoded.Views))
	}
	if len(decoded.MaterializedViews) != 1 {
		t.Errorf("Expected 1 materialized view, got %d", len(decoded.MaterializedViews))
	}
	if len(decoded.Functions) != 1 {
		t.Errorf("Expected 1 function, got %d", len(decoded.Functions))
	}
	if len(decoded.Triggers) != 1 {
		t.Errorf("Expected 1 trigger, got %d", len(decoded.Triggers))
	}
	if len(decoded.Grants) != 1 {
		t.Errorf("Expected 1 grant, got %d", len(decoded.Grants))
	}
}

func TestTable_JSON(t *testing.T) {
	table := Table{
		Name: "users",
		Columns: []Column{
			{Name: "id", DataType: "integer", IsNullable: false, IsPrimaryKey: true},
			{Name: "email", DataType: "varchar", IsNullable: false},
		},
		Indexes: []Index{
			{Name: "users_pkey", IsPrimary: true, IsUnique: true},
		},
		Constraints: []Constraint{
			{Name: "users_pkey", Type: "PRIMARY KEY", Columns: []string{"id"}},
		},
	}

	data, err := json.Marshal(table)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded Table
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Name != "users" {
		t.Errorf("Expected 'users', got %s", decoded.Name)
	}
	if len(decoded.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(decoded.Columns))
	}
	if decoded.Columns[0].IsPrimaryKey != true {
		t.Errorf("Expected first column to be primary key")
	}
	if decoded.Columns[1].IsNullable != false {
		t.Errorf("Expected email to not be nullable")
	}
}

func TestColumn_JSON(t *testing.T) {
	col := Column{
		Name:         "id",
		DataType:     "integer",
		DefaultValue: strPtr("nextval('users_id_seq'::regclass)"),
		IsNullable:   false,
		IsPrimaryKey: true,
	}

	data, err := json.Marshal(col)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded Column
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Name != "id" {
		t.Errorf("Expected 'id', got %s", decoded.Name)
	}
	if decoded.DataType != "integer" {
		t.Errorf("Expected 'integer', got %s", decoded.DataType)
	}
	if decoded.IsNullable != false {
		t.Errorf("Expected not nullable")
	}
	if decoded.IsPrimaryKey != true {
		t.Errorf("Expected primary key")
	}
}

func TestForeignKey_JSON(t *testing.T) {
	fk := ForeignKey{
		Name:       "fk_users_posts",
		Columns:    []string{"user_id"},
		RefTable:   "users",
		RefColumns: []string{"id"},
		OnDelete:   "CASCADE",
		OnUpdate:   "NO ACTION",
	}

	data, err := json.Marshal(fk)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded ForeignKey
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Name != "fk_users_posts" {
		t.Errorf("Expected 'fk_users_posts', got %s", decoded.Name)
	}
	if decoded.RefTable != "users" {
		t.Errorf("Expected ref table 'users', got %s", decoded.RefTable)
	}
	if decoded.OnDelete != "CASCADE" {
		t.Errorf("Expected on delete 'CASCADE', got %s", decoded.OnDelete)
	}
}

func TestSequence_JSON(t *testing.T) {
	seq := Sequence{
		Name:      "users_id_seq",
		Start:     1,
		MinValue:  1,
		MaxValue:  9223372036854775807,
		Increment: 1,
		Cache:     1,
		Cycle:     false,
	}

	data, err := json.Marshal(seq)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded Sequence
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Name != "users_id_seq" {
		t.Errorf("Expected 'users_id_seq', got %s", decoded.Name)
	}
	if decoded.Start != 1 {
		t.Errorf("Expected start 1, got %d", decoded.Start)
	}
}

func strPtr(s string) *string {
	return &s
}

func TestIndex_JSON(t *testing.T) {
	idx := Index{
		Name:       "users_email_idx",
		Columns:    []string{"email"},
		IsUnique:   true,
		IsPrimary:  false,
		Definition: "CREATE UNIQUE INDEX users_email_idx ON users(email)",
	}

	data, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded Index
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Name != "users_email_idx" {
		t.Errorf("Expected 'users_email_idx', got %s", decoded.Name)
	}
	if len(decoded.Columns) != 1 || decoded.Columns[0] != "email" {
		t.Errorf("Expected columns [email], got %v", decoded.Columns)
	}
	if !decoded.IsUnique {
		t.Errorf("Expected unique index")
	}
}

func TestConstraint_JSON(t *testing.T) {
	cons := Constraint{
		Name:    "users_email_key",
		Type:    "UNIQUE",
		Columns: []string{"email"},
	}

	data, err := json.Marshal(cons)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded Constraint
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Name != "users_email_key" {
		t.Errorf("Expected 'users_email_key', got %s", decoded.Name)
	}
	if decoded.Type != "UNIQUE" {
		t.Errorf("Expected type UNIQUE, got %s", decoded.Type)
	}
}

func TestView_JSON(t *testing.T) {
	view := View{Name: "active_users", Definition: "SELECT * FROM users"}
	data, _ := json.Marshal(view)
	var decoded View
	json.Unmarshal(data, &decoded)
	if decoded.Name != "active_users" {
		t.Errorf("Expected 'active_users', got %s", decoded.Name)
	}
}

func TestMaterializedView_JSON(t *testing.T) {
	mview := MaterializedView{Name: "daily_stats", Definition: "SELECT * FROM logs"}
	data, _ := json.Marshal(mview)
	var decoded MaterializedView
	json.Unmarshal(data, &decoded)
	if decoded.Name != "daily_stats" {
		t.Errorf("Expected 'daily_stats', got %s", decoded.Name)
	}
}

func TestFunction_JSON(t *testing.T) {
	fn := Function{Name: "get_user", Arguments: "integer", Definition: "SELECT 1"}
	data, _ := json.Marshal(fn)
	var decoded Function
	json.Unmarshal(data, &decoded)
	if decoded.Name != "get_user" {
		t.Errorf("Expected 'get_user', got %s", decoded.Name)
	}
}

func TestTrigger_JSON(t *testing.T) {
	trig := Trigger{Name: "update_log", Table: "users", Definition: "EXECUTE fn()"}
	data, _ := json.Marshal(trig)
	var decoded Trigger
	json.Unmarshal(data, &decoded)
	if decoded.Name != "update_log" {
		t.Errorf("Expected 'update_log', got %s", decoded.Name)
	}
}

func TestGrant_JSON(t *testing.T) {
	grant := Grant{Table: "users", Grantee: "web", Privilege: "SELECT", IsGrantable: true}
	data, _ := json.Marshal(grant)
	var decoded Grant
	json.Unmarshal(data, &decoded)
	if decoded.Privilege != "SELECT" {
		t.Errorf("Expected 'SELECT', got %s", decoded.Privilege)
	}
}
