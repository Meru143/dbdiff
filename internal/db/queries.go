package db

import (
	"context"

	"github.com/meru143/dbdiff/pkg/types"
)

func getTables(ctx context.Context, db *DB, schemaName string) ([]string, error) {
	query := `SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name`
	rows, err := db.Query(ctx, query, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}
	return tables, nil
}

func getColumns(ctx context.Context, db *DB, schemaName, tableName string) ([]types.Column, error) {
	query := `SELECT column_name, data_type, column_default, is_nullable FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`
	rows, err := db.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []types.Column
	for rows.Next() {
		var col types.Column
		var defaultVal *string
		rows.Scan(&col.Name, &col.DataType, &defaultVal, &col.IsNullable)
		col.DefaultValue = defaultVal
		columns = append(columns, col)
	}
	return columns, nil
}

func getIndexes(ctx context.Context, db *DB, schemaName, tableName string) ([]types.Index, error) {
	query := `SELECT i.relname, ix.indisunique, ix.indisprimary, pg_get_indexdef(ix.indexrelid) FROM pg_class t JOIN pg_index ix ON t.oid = ix.indrelid JOIN pg_class i ON i.oid = ix.indexrelid WHERE t.relname = $2 AND t.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = $1)`
	rows, err := db.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []types.Index
	for rows.Next() {
		var idx types.Index
		rows.Scan(&idx.Name, &idx.IsUnique, &idx.IsPrimary, &idx.Definition)
		indexes = append(indexes, idx)
	}
	return indexes, nil
}

func getConstraints(ctx context.Context, db *DB, schemaName, tableName string) ([]types.Constraint, error) {
	query := `SELECT constraint_name, constraint_type FROM information_schema.table_constraints WHERE table_schema = $1 AND table_name = $2`
	rows, err := db.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []types.Constraint
	for rows.Next() {
		var cons types.Constraint
		rows.Scan(&cons.Name, &cons.Type)
		constraints = append(constraints, cons)
	}
	return constraints, nil
}

func getForeignKeys(ctx context.Context, db *DB, schemaName, tableName string) ([]types.ForeignKey, error) {
	query := `SELECT tc.constraint_name, kcu.column_name, ccu.table_name, ccu.column_name FROM information_schema.table_constraints tc JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name JOIN information_schema.constraint_column_usage ccu ON tc.constraint_name = ccu.constraint_name WHERE tc.table_schema = $1 AND tc.table_name = $2 AND tc.constraint_type = 'FOREIGN KEY'`
	rows, err := db.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []types.ForeignKey
	for rows.Next() {
		var fk types.ForeignKey
		rows.Scan(&fk.Name, &fk.Columns, &fk.RefTable, &fk.RefColumns)
		fks = append(fks, fk)
	}
	return fks, nil
}

func getSequences(ctx context.Context, db *DB, schemaName string) ([]types.Sequence, error) {
	query := `SELECT sequence_name FROM information_schema.sequences WHERE sequence_schema = $1`
	rows, err := db.Query(ctx, query, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sequences []types.Sequence
	for rows.Next() {
		var seq types.Sequence
		rows.Scan(&seq.Name)
		sequences = append(sequences, seq)
	}
	return sequences, nil
}

func filterColumns(columns []types.Column, ignorePatterns []string) []types.Column {
	if len(ignorePatterns) == 0 {
		return columns
	}
	var filtered []types.Column
	for _, col := range columns {
		matched := false
		for _, pattern := range ignorePatterns {
			if col.Name == pattern || (len(pattern) > 0 && pattern[0] == '*' && len(col.Name) >= len(pattern)-1 && col.Name[len(col.Name)-(len(pattern)-1):] == pattern[1:]) {
				matched = true
				break
			}
		}
		if !matched {
			filtered = append(filtered, col)
		}
	}
	return filtered
}
