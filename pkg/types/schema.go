package types

// Schema represents a database schema
type Schema struct {
	Tables    []Table    `json:"tables"`
	Sequences []Sequence `json:"sequences,omitempty"`
}

// Table represents a database table
type Table struct {
	Name        string       `json:"name"`
	Columns     []Column    `json:"columns"`
	Indexes     []Index     `json:"indexes,omitempty"`
	Constraints []Constraint `json:"constraints,omitempty"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
}

// Column represents a table column
type Column struct {
	Name         string  `json:"name"`
	DataType     string  `json:"data_type"`
	DefaultValue *string `json:"default_value,omitempty"`
	IsNullable   bool    `json:"is_nullable"`
	IsPrimaryKey bool    `json:"is_primary_key"`
}

// Index represents a database index
type Index struct {
	Name       string `json:"name"`
	IsUnique   bool   `json:"is_unique"`
	IsPrimary  bool   `json:"is_primary"`
	Definition string `json:"definition"`
}

// Constraint represents a table constraint
type Constraint struct {
	Name    string   `json:"name"`
	Type   string   `json:"type"` // PRIMARY KEY, UNIQUE, CHECK
	Columns []string `json:"columns,omitempty"`
}

// ForeignKey represents a foreign key constraint
type ForeignKey struct {
	Name     string   `json:"name"`
	Columns  []string `json:"columns"`
	RefTable string   `json:"reference_table"`
	RefColumns []string `json:"reference_columns"`
}

// Sequence represents a database sequence
type Sequence struct {
	Name       string `json:"name"`
	StartValue *int64 `json:"start_value,omitempty"`
	MinValue   *int64 `json:"min_value,omitempty"`
	MaxValue   *int64 `json:"max_value,omitempty"`
	Increment  *int64 `json:"increment,omitempty"`
}
