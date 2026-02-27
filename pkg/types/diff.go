package types

// DiffType represents the type of difference
type DiffType string

const (
	DiffAdd    DiffType = "ADD"
	DiffDrop   DiffType = "DROP"
	DiffAlter  DiffType = "ALTER"
	DiffRename DiffType = "RENAME"
)

// DiffObject represents the type of database object
type DiffObject string

const (
	ObjectTable     DiffObject = "TABLE"
	ObjectColumn    DiffObject = "COLUMN"
	ObjectIndex     DiffObject = "INDEX"
	ObjectConstraint DiffObject = "CONSTRAINT"
	ObjectForeignKey DiffObject = "FOREIGN_KEY"
	ObjectSequence  DiffObject = "SEQUENCE"
)

// Diff represents a schema difference
type Diff struct {
	Type       DiffType   `json:"type"`
	Object     DiffObject `json:"object"`
	Name       string    `json:"name"`
	OldValue   string    `json:"old_value,omitempty"`
	NewValue   string    `json:"new_value,omitempty"`
	SQL        string    `json:"sql,omitempty"`
	TableName  string    `json:"table_name,omitempty"`
	Columns    []string  `json:"columns,omitempty"`
}

// DiffList is a list of differences with helper methods
type DiffList []Diff

// FilterByObject returns differences filtered by object type
func (d DiffList) FilterByObject(obj DiffObject) DiffList {
	var filtered DiffList
	for _, diff := range d {
		if diff.Object == obj {
			filtered = append(filtered, diff)
		}
	}
	return filtered
}

// FilterByType returns differences filtered by diff type
func (d DiffList) FilterByType(diffType DiffType) DiffList {
	var filtered DiffList
	for _, diff := range d {
		if diff.Type == diffType {
			filtered = append(filtered, diff)
		}
	}
	return filtered
}

// Count returns the number of differences
func (d DiffList) Count() int {
	return len(d)
}

// CountByType returns a map of diff types to their counts
func (d DiffList) CountByType() map[DiffType]int {
	counts := make(map[DiffType]int)
	for _, diff := range d {
		counts[diff.Type]++
	}
	return counts
}
