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
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
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
		if err := rows.Scan(&col.Name, &col.DataType, &defaultVal, &col.IsNullable); err != nil {
			return nil, err
		}
		col.DefaultValue = defaultVal
		columns = append(columns, col)
	}
	return columns, rows.Err()
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
		if err := rows.Scan(&idx.Name, &idx.IsUnique, &idx.IsPrimary, &idx.Definition); err != nil {
			return nil, err
		}
		indexes = append(indexes, idx)
	}
	return indexes, rows.Err()
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
		if err := rows.Scan(&cons.Name, &cons.Type); err != nil {
			return nil, err
		}
		constraints = append(constraints, cons)
	}
	return constraints, rows.Err()
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
		if err := rows.Scan(&fk.Name, &fk.Columns, &fk.RefTable, &fk.RefColumns); err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func getSequences(ctx context.Context, db *DB, schemaName string) ([]types.Sequence, error) {
	query := `SELECT sequence_name, start_value, minimum_value, maximum_value, increment, cache_size, cycle FROM information_schema.sequences WHERE sequence_schema = $1`
	rows, err := db.Query(ctx, query, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sequences []types.Sequence
	for rows.Next() {
		var seq types.Sequence
		if err := rows.Scan(&seq.Name, &seq.Start, &seq.MinValue, &seq.MaxValue, &seq.Increment, &seq.Cache, &seq.Cycle); err != nil {
			return nil, err
		}
		sequences = append(sequences, seq)
	}
	return sequences, rows.Err()
}

// getCustomTypes returns user-defined types in the schema
func getCustomTypes(ctx context.Context, db *DB, schemaName string) ([]types.Type, error) {
	// Get enum types
	enumQuery := `SELECT t.typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE n.nspname = $1 AND t.typtype = 'e'`
	rows, err := db.Query(ctx, enumQuery, schemaName)
	if err != nil {
		return nil, err
	}

	var customTypes []types.Type
	for rows.Next() {
		var typeName string
		if err := rows.Scan(&typeName); err != nil {
			rows.Close()
			return nil, err
		}
		// Get enum values
		vals, err := getEnumValues(ctx, db, typeName)
		if err != nil {
			rows.Close()
			return nil, err
		}
		customTypes = append(customTypes, types.Type{
			Name:   typeName,
			Kind:   "enum",
			Values: vals,
		})
	}
	rows.Close()

	// Get composite types
	compQuery := `SELECT t.typname FROM pg_type t JOIN pg_namespace n ON t.typnamespace = n.oid WHERE n.nspname = $1 AND t.typtype = 'c' AND t.typname NOT LIKE '_%'`
	rows, err = db.Query(ctx, compQuery, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var typeName string
		if err := rows.Scan(&typeName); err != nil {
			return nil, err
		}
		customTypes = append(customTypes, types.Type{
			Name: typeName,
			Kind: "composite",
		})
	}

	return customTypes, rows.Err()
}

func getEnumValues(ctx context.Context, db *DB, enumName string) ([]string, error) {
	query := `SELECT enumlabel FROM pg_enum WHERE enumtypid = (SELECT oid FROM pg_type WHERE typname = $1) ORDER BY enumsortorder`
	rows, err := db.Query(ctx, query, enumName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			return nil, err
		}
		values = append(values, val)
	}
	return values, rows.Err()
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
