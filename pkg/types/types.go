package types

import (
	"encoding/json"
)

type Schema struct {
	Tables            []Table            `json:"tables"`
	Sequences         []Sequence         `json:"sequences,omitempty"`
	Types             []Type             `json:"types,omitempty"`
	Views             []View             `json:"views,omitempty"`
	MaterializedViews []MaterializedView `json:"materialized_views,omitempty"`
	Functions         []Function         `json:"functions,omitempty"`
	Triggers          []Trigger          `json:"triggers,omitempty"`
	Grants            []Grant            `json:"grants,omitempty"`
}

type Table struct {
	Name        string       `json:"name"`
	Columns     []Column     `json:"columns"`
	Indexes     []Index      `json:"indexes,omitempty"`
	Constraints []Constraint `json:"constraints,omitempty"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
}

type Column struct {
	Name         string  `json:"name"`
	DataType     string  `json:"data_type"`
	DefaultValue *string `json:"default_value,omitempty"`
	IsNullable   bool    `json:"is_nullable"`
	IsPrimaryKey bool    `json:"is_primary_key,omitempty"`
}

type Index struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns,omitempty"`
	IsUnique   bool     `json:"is_unique"`
	IsPrimary  bool     `json:"is_primary"`
	Definition string   `json:"definition"`
}

type Constraint struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"` // PRIMARY KEY, UNIQUE, CHECK, FOREIGN KEY
	Columns []string `json:"columns,omitempty"`
}

type ForeignKey struct {
	Name                 string   `json:"name"`
	Columns              []string `json:"columns"`
	RefTable             string   `json:"reference_table"`
	RefColumns           []string `json:"reference_columns"`
	UniqueConstraintName string   `json:"unique_constraint_name,omitempty"`
	OnDelete             string   `json:"on_delete,omitempty"`
	OnUpdate             string   `json:"on_update,omitempty"`
}

type Sequence struct {
	Name      string `json:"name"`
	Start     int64  `json:"start,omitempty"`
	MinValue  int64  `json:"min_value,omitempty"`
	MaxValue  int64  `json:"max_value,omitempty"`
	Increment int64  `json:"increment,omitempty"`
	Cache     int64  `json:"cache,omitempty"`
	Cycle     bool   `json:"cycle,omitempty"`
}

// Type represents a custom PostgreSQL type
type Type struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`             // enum, composite, domain
	Values []string `json:"values,omitempty"` // for enum
}

// View represents a database view
type View struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

// MaterializedView represents a PostgreSQL materialized view
type MaterializedView struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

// Function represents a PostgreSQL stored procedure or function
type Function struct {
	Name       string `json:"name"`
	Arguments  string `json:"arguments"`
	Definition string `json:"definition"`
}

// Trigger represents a PostgreSQL trigger
type Trigger struct {
	Name       string `json:"name"`
	Table      string `json:"table"`
	Definition string `json:"definition"`
}

// Grant represents a user privilege on a database object like a table
type Grant struct {
	Table       string `json:"table"`
	Grantee     string `json:"grantee"`
	Privilege   string `json:"privilege_type"`
	IsGrantable bool   `json:"is_grantable"`
}

// ServerInfo holds database server metadata
type ServerInfo struct {
	Version  string
	Database string
	User     string
}

type DiffType string

const (
	DiffAdd    DiffType = "ADD"
	DiffDrop   DiffType = "DROP"
	DiffAlter  DiffType = "ALTER"
	DiffRename DiffType = "RENAME"
)

type DiffObject string

const (
	ObjectTable            DiffObject = "TABLE"
	ObjectColumn           DiffObject = "COLUMN"
	ObjectIndex            DiffObject = "INDEX"
	ObjectConstraint       DiffObject = "CONSTRAINT"
	ObjectForeignKey       DiffObject = "FOREIGN_KEY"
	ObjectSequence         DiffObject = "SEQUENCE"
	ObjectType             DiffObject = "TYPE"
	ObjectView             DiffObject = "VIEW"
	ObjectMaterializedView DiffObject = "MATERIALIZED_VIEW"
	ObjectFunction         DiffObject = "FUNCTION"
	ObjectTrigger          DiffObject = "TRIGGER"
	ObjectGrant            DiffObject = "GRANT"
)

type Diff struct {
	Type        DiffType   `json:"type"`
	Object      DiffObject `json:"object"`
	Name        string     `json:"name"`
	TableName   string     `json:"table_name,omitempty"`
	OldValue    string     `json:"old_value,omitempty"`
	NewValue    string     `json:"new_value,omitempty"`
	SQL         string     `json:"sql,omitempty"`
	Description string     `json:"description,omitempty"`
}

type DiffList []Diff

// ToJSON converts DiffList to JSON
func (dl DiffList) ToJSON() ([]byte, error) {
	return json.Marshal(dl)
}
