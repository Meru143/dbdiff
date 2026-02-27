package diff

import "github.com/meru143/dbdiff/pkg/types"

func Compare(source, target *types.Schema) types.DiffList {
	var differences types.DiffList

	sourceTables := make(map[string]*types.Table)
	targetTables := make(map[string]*types.Table)

	for i := range source.Tables {
		t := &source.Tables[i]
		sourceTables[t.Name] = t
	}
	for i := range target.Tables {
		t := &target.Tables[i]
		targetTables[t.Name] = t
	}

	for name, sourceTable := range sourceTables {
		if targetTable, exists := targetTables[name]; exists {
			differences = append(differences, compareColumns(sourceTable, targetTable)...)
			differences = append(differences, compareIndexes(sourceTable, targetTable)...)
			differences = append(differences, compareConstraints(sourceTable, targetTable)...)
			differences = append(differences, compareForeignKeys(sourceTable, targetTable)...)
		} else {
			differences = append(differences, types.Diff{
				Type: types.DiffAdd, Object: types.ObjectTable, Name: name,
			})
		}
	}

	for name := range targetTables {
		if _, exists := sourceTables[name]; !exists {
			differences = append(differences, types.Diff{
				Type: types.DiffDrop, Object: types.ObjectTable, Name: name,
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
			if sourceCol.DataType != targetCol.DataType {
				differences = append(differences, types.Diff{
					Type: types.DiffAlter, Object: types.ObjectColumn, Name: name, TableName: sourceTable.Name,
					OldValue: targetCol.DataType, NewValue: sourceCol.DataType,
				})
			}
			if sourceCol.IsNullable != targetCol.IsNullable {
				differences = append(differences, types.Diff{
					Type: types.DiffAlter, Object: types.ObjectColumn, Name: name, TableName: sourceTable.Name,
					OldValue: boolToString(targetCol.IsNullable), NewValue: boolToString(sourceCol.IsNullable),
				})
			}
		} else {
			differences = append(differences, types.Diff{
				Type: types.DiffAdd, Object: types.ObjectColumn, Name: name, TableName: sourceTable.Name,
			})
		}
	}

	for name := range targetCols {
		if _, exists := sourceCols[name]; !exists {
			differences = append(differences, types.Diff{
				Type: types.DiffDrop, Object: types.ObjectColumn, Name: name, TableName: sourceTable.Name,
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

	for name := range sourceIndexes {
		if _, exists := targetIndexes[name]; !exists {
			differences = append(differences, types.Diff{
				Type: types.DiffAdd, Object: types.ObjectIndex, Name: name, TableName: sourceTable.Name,
			})
		}
	}
	for name := range targetIndexes {
		if _, exists := sourceIndexes[name]; !exists {
			differences = append(differences, types.Diff{
				Type: types.DiffDrop, Object: types.ObjectIndex, Name: name, TableName: sourceTable.Name,
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

	for name := range sourceCons {
		if _, exists := targetCons[name]; !exists {
			differences = append(differences, types.Diff{
				Type: types.DiffAdd, Object: types.ObjectConstraint, Name: name, TableName: sourceTable.Name,
			})
		}
	}
	for name := range targetCons {
		if _, exists := sourceCons[name]; !exists {
			differences = append(differences, types.Diff{
				Type: types.DiffDrop, Object: types.ObjectConstraint, Name: name, TableName: sourceTable.Name,
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

	for name := range sourceFKs {
		if _, exists := targetFKs[name]; !exists {
			differences = append(differences, types.Diff{
				Type: types.DiffAdd, Object: types.ObjectForeignKey, Name: name, TableName: sourceTable.Name,
			})
		}
	}
	for name := range targetFKs {
		if _, exists := sourceFKs[name]; !exists {
			differences = append(differences, types.Diff{
				Type: types.DiffDrop, Object: types.ObjectForeignKey, Name: name, TableName: sourceTable.Name,
			})
		}
	}
	return differences
}

func boolToString(b bool) string {
	if b {
		return "NULL"
	}
	return "NOT NULL"
}
