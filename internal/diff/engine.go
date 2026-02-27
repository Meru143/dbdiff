package diff

import "github.com/meru143/dbdiff/pkg/types"

// DiffEngine compares two schemas and generates differences
type DiffEngine struct {
	source *types.Schema
	target *types.Schema
}

// NewDiffEngine creates a new DiffEngine
func NewDiffEngine(source, target *types.Schema) *DiffEngine {
	return &DiffEngine{
		source: source,
		target: target,
	}
}

// Compare performs full schema comparison
func (e *DiffEngine) Compare() types.DiffList {
	var differences types.DiffList

	// Table comparison
	differences = append(differences, e.compareTables()...)

	// Sequence comparison
	differences = append(differences, e.compareSequences()...)

	// Type comparison
	differences = append(differences, e.compareTypes()...)

	return differences
}

// Legacy Compare function
func Compare(source, target *types.Schema) types.DiffList {
	engine := NewDiffEngine(source, target)
	return engine.Compare()
}

func (e *DiffEngine) compareTables() types.DiffList {
	var differences types.DiffList

	sourceTables := make(map[string]*types.Table)
	targetTables := make(map[string]*types.Table)

	for i := range e.source.Tables {
		t := &e.source.Tables[i]
		sourceTables[t.Name] = t
	}
	for i := range e.target.Tables {
		t := &e.target.Tables[i]
		targetTables[t.Name] = t
	}

	// New tables (in source but not in target)
	for name, sourceTable := range sourceTables {
		if _, exists := targetTables[name]; !exists {
			differences = append(differences, types.Diff{
				Type:   types.DiffAdd,
				Object: types.ObjectTable,
				Name:   name,
			})
		} else {
			// Table exists in both - compare columns, indexes, constraints
			targetTable := targetTables[name]
			differences = append(differences, compareColumns(sourceTable, targetTable)...)
			differences = append(differences, compareIndexes(sourceTable, targetTable)...)
			differences = append(differences, compareConstraints(sourceTable, targetTable)...)
			differences = append(differences, compareForeignKeys(sourceTable, targetTable)...)
		}
	}

	// Dropped tables (in target but not in source)
	for name := range targetTables {
		if _, exists := sourceTables[name]; !exists {
			differences = append(differences, types.Diff{
				Type:   types.DiffDrop,
				Object: types.ObjectTable,
				Name:   name,
			})
		}
	}

	return differences
}

func (e *DiffEngine) compareSequences() types.DiffList {
	var differences types.DiffList

	sourceSeqs := make(map[string]*types.Sequence)
	targetSeqs := make(map[string]*types.Sequence)

	for i := range e.source.Sequences {
		s := &e.source.Sequences[i]
		sourceSeqs[s.Name] = s
	}
	for i := range e.target.Sequences {
		s := &e.target.Sequences[i]
		targetSeqs[s.Name] = s
	}

	// New sequences
	for name, sourceSeq := range sourceSeqs {
		if targetSeq, exists := targetSeqs[name]; !exists {
			differences = append(differences, types.Diff{
				Type:   types.DiffAdd,
				Object: types.ObjectSequence,
				Name:   name,
			})
		} else {
			// Compare sequence properties
			if sourceSeq.Start != targetSeq.Start {
				differences = append(differences, types.Diff{
					Type:     types.DiffAlter,
					Object:   types.ObjectSequence,
					Name:     name,
					OldValue: string(rune(targetSeq.Start)),
					NewValue: string(rune(sourceSeq.Start)),
				})
			}
			if sourceSeq.Increment != targetSeq.Increment {
				differences = append(differences, types.Diff{
					Type:     types.DiffAlter,
					Object:   types.ObjectSequence,
					Name:     name,
					OldValue: "INCREMENT BY " + string(rune(targetSeq.Increment)),
					NewValue: "INCREMENT BY " + string(rune(sourceSeq.Increment)),
				})
			}
		}
	}

	// Dropped sequences
	for name := range targetSeqs {
		if _, exists := sourceSeqs[name]; !exists {
			differences = append(differences, types.Diff{
				Type:   types.DiffDrop,
				Object: types.ObjectSequence,
				Name:   name,
			})
		}
	}

	return differences
}

func (e *DiffEngine) compareTypes() types.DiffList {
	var differences types.DiffList

	sourceTypes := make(map[string]*types.Type)
	targetTypes := make(map[string]*types.Type)

	for i := range e.source.Types {
		t := &e.source.Types[i]
		sourceTypes[t.Name] = t
	}
	for i := range e.target.Types {
		t := &e.target.Types[i]
		targetTypes[t.Name] = t
	}

	// New types
	for name := range sourceTypes {
		if _, exists := targetTypes[name]; !exists {
			differences = append(differences, types.Diff{
				Type:   types.DiffAdd,
				Object: types.ObjectType,
				Name:   name,
			})
		}
	}

	// Dropped types
	for name := range targetTypes {
		if _, exists := sourceTypes[name]; !exists {
			differences = append(differences, types.Diff{
				Type:   types.DiffDrop,
				Object: types.ObjectType,
				Name:   name,
			})
		}
	}

	return differences
}

func compareColumns(sourceTable, targetTable *types.Table) types.DiffList {
	var differences types.DiffList
	sourceCols := make(map[string]*types.Column)
	targetCols := make(map[string]*types.Column)

	for i := range sourceTable.Columns {
		c := &sourceTable.Columns[i]
		sourceCols[c.Name] = c
	}
	for i := range targetTable.Columns {
		c := &targetTable.Columns[i]
		targetCols[c.Name] = c
	}

	for name, sourceCol := range sourceCols {
		if targetCol, exists := targetCols[name]; exists {
			// Type change
			if sourceCol.DataType != targetCol.DataType {
				differences = append(differences, types.Diff{
					Type:      types.DiffAlter,
					Object:    types.ObjectColumn,
					Name:      name,
					TableName: sourceTable.Name,
					OldValue:  targetCol.DataType,
					NewValue:  sourceCol.DataType,
				})
			}
			// Nullable change
			if sourceCol.IsNullable != targetCol.IsNullable {
				differences = append(differences, types.Diff{
					Type:      types.DiffAlter,
					Object:    types.ObjectColumn,
					Name:      name,
					TableName: sourceTable.Name,
					OldValue:  nullableToString(targetCol.IsNullable),
					NewValue:  nullableToString(sourceCol.IsNullable),
				})
			}
			// Default value change
			if (sourceCol.DefaultValue == nil) != (targetCol.DefaultValue == nil) ||
				(sourceCol.DefaultValue != nil && targetCol.DefaultValue != nil && *sourceCol.DefaultValue != *targetCol.DefaultValue) {
				oldVal := ""
				if targetCol.DefaultValue != nil {
					oldVal = *targetCol.DefaultValue
				}
				newVal := ""
				if sourceCol.DefaultValue != nil {
					newVal = *sourceCol.DefaultValue
				}
				differences = append(differences, types.Diff{
					Type:      types.DiffAlter,
					Object:    types.ObjectColumn,
					Name:      name,
					TableName: sourceTable.Name,
					OldValue:  oldVal,
					NewValue:  newVal,
				})
			}
		} else {
			// New column
			differences = append(differences, types.Diff{
				Type:      types.DiffAdd,
				Object:    types.ObjectColumn,
				Name:      name,
				TableName: sourceTable.Name,
			})
		}
	}

	// Dropped columns
	for name := range targetCols {
		if _, exists := sourceCols[name]; !exists {
			differences = append(differences, types.Diff{
				Type:      types.DiffDrop,
				Object:    types.ObjectColumn,
				Name:      name,
				TableName: sourceTable.Name,
			})
		}
	}

	return differences
}

func compareIndexes(sourceTable, targetTable *types.Table) types.DiffList {
	var differences types.DiffList
	sourceIndexes := make(map[string]*types.Index)
	targetIndexes := make(map[string]*types.Index)

	for i := range sourceTable.Indexes {
		idx := &sourceTable.Indexes[i]
		sourceIndexes[idx.Name] = idx
	}
	for i := range targetTable.Indexes {
		idx := &targetTable.Indexes[i]
		targetIndexes[idx.Name] = idx
	}

	// New indexes
	for name, idx := range sourceIndexes {
		if _, exists := targetIndexes[name]; !exists {
			differences = append(differences, types.Diff{
				Type:      types.DiffAdd,
				Object:    types.ObjectIndex,
				Name:      name,
				TableName: sourceTable.Name,
			})
		} else {
			// Index definition change
			targetIdx := targetIndexes[name]
			if idx.Definition != targetIdx.Definition {
				differences = append(differences, types.Diff{
					Type:      types.DiffAlter,
					Object:    types.ObjectIndex,
					Name:      name,
					TableName: sourceTable.Name,
					OldValue:  targetIdx.Definition,
					NewValue:  idx.Definition,
				})
			}
		}
	}

	// Dropped indexes
	for name := range targetIndexes {
		if _, exists := sourceIndexes[name]; !exists {
			differences = append(differences, types.Diff{
				Type:      types.DiffDrop,
				Object:    types.ObjectIndex,
				Name:      name,
				TableName: sourceTable.Name,
			})
		}
	}
	return differences
}

func compareConstraints(sourceTable, targetTable *types.Table) types.DiffList {
	var differences types.DiffList
	sourceCons := make(map[string]*types.Constraint)
	targetCons := make(map[string]*types.Constraint)

	for i := range sourceTable.Constraints {
		c := &sourceTable.Constraints[i]
		sourceCons[c.Name] = c
	}
	for i := range targetTable.Constraints {
		c := &targetTable.Constraints[i]
		targetCons[c.Name] = c
	}

	// New constraints
	for name := range sourceCons {
		if _, exists := targetCons[name]; !exists {
			differences = append(differences, types.Diff{
				Type:      types.DiffAdd,
				Object:    types.ObjectConstraint,
				Name:      name,
				TableName: sourceTable.Name,
			})
		}
	}

	// Dropped constraints
	for name := range targetCons {
		if _, exists := sourceCons[name]; !exists {
			differences = append(differences, types.Diff{
				Type:      types.DiffDrop,
				Object:    types.ObjectConstraint,
				Name:      name,
				TableName: sourceTable.Name,
			})
		}
	}
	return differences
}

func compareForeignKeys(sourceTable, targetTable *types.Table) types.DiffList {
	var differences types.DiffList
	sourceFKs := make(map[string]*types.ForeignKey)
	targetFKs := make(map[string]*types.ForeignKey)

	for i := range sourceTable.ForeignKeys {
		fk := &sourceTable.ForeignKeys[i]
		sourceFKs[fk.Name] = fk
	}
	for i := range targetTable.ForeignKeys {
		fk := &targetTable.ForeignKeys[i]
		targetFKs[fk.Name] = fk
	}

	// New FKs
	for name, fk := range sourceFKs {
		if _, exists := targetFKs[name]; !exists {
			differences = append(differences, types.Diff{
				Type:      types.DiffAdd,
				Object:    types.ObjectForeignKey,
				Name:      name,
				TableName: sourceTable.Name,
			})
		} else {
			// FK definition change
			targetFK := targetFKs[name]
			if fk.RefTable != targetFK.RefTable {
				differences = append(differences, types.Diff{
					Type:      types.DiffAlter,
					Object:    types.ObjectForeignKey,
					Name:      name,
					TableName: sourceTable.Name,
					OldValue:  targetFK.RefTable,
					NewValue:  fk.RefTable,
				})
			}
		}
	}

	// Dropped FKs
	for name := range targetFKs {
		if _, exists := sourceFKs[name]; !exists {
			differences = append(differences, types.Diff{
				Type:      types.DiffDrop,
				Object:    types.ObjectForeignKey,
				Name:      name,
				TableName: sourceTable.Name,
			})
		}
	}
	return differences
}

func nullableToString(b bool) string {
	if b {
		return "NULL"
	}
	return "NOT NULL"
}
