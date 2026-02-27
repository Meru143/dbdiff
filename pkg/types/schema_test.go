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
		Name:          "fk_users_posts",
		Columns:       []string{"user_id"},
		RefTable:     "users",
		RefColumns:   []string{"id"},
		OnDelete:     "CASCADE",
		OnUpdate:     "NO ACTION",
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
