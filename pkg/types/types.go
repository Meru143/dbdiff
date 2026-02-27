package types

type Schema struct {
	Tables    []Table    `json:"tables"`
	Sequences []Sequence `json:"sequences,omitempty"`
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
}

type Index struct {
	Name       string `json:"name"`
	IsUnique   bool   `json:"is_unique"`
	IsPrimary  bool   `json:"is_primary"`
	Definition string `json:"definition"`
}

type Constraint struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ForeignKey struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	RefTable   string   `json:"reference_table"`
	RefColumns []string `json:"reference_columns"`
}

type Sequence struct {
	Name string `json:"name"`
}

type DiffType string

const (
	DiffAdd   DiffType = "ADD"
	DiffDrop  DiffType = "DROP"
	DiffAlter DiffType = "ALTER"
)

type DiffObject string

const (
	ObjectTable      DiffObject = "TABLE"
	ObjectColumn     DiffObject = "COLUMN"
	ObjectIndex      DiffObject = "INDEX"
	ObjectConstraint DiffObject = "CONSTRAINT"
	ObjectForeignKey DiffObject = "FOREIGN_KEY"
	ObjectSequence   DiffObject = "SEQUENCE"
)

type Diff struct {
	Type      DiffType   `json:"type"`
	Object    DiffObject `json:"object"`
	Name      string     `json:"name"`
	TableName string     `json:"table_name,omitempty"`
	OldValue  string     `json:"old_value,omitempty"`
	NewValue  string     `json:"new_value,omitempty"`
	SQL       string     `json:"sql,omitempty"`
}

type DiffList []Diff
