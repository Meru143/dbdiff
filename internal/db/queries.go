package db

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/meru143/dbdiff/pkg/types"
)

// Default ignore patterns for columns
var DefaultIgnorePatterns = []string{"_created_at", "_updated_at", "_modified_at", "_deleted_at"}

func getTables(ctx context.Context, db *DB, schemaName string, ignorePatterns []string) ([]string, error) {
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
		// Filter by ignore patterns at table level
		if !matchesIgnorePatterns(name, ignorePatterns) {
			tables = append(tables, name)
		}
	}
	return tables, rows.Err()
}

func getColumns(ctx context.Context, db *DB, schemaName, tableName string) ([]types.Column, error) {
	// Get columns with primary key info
	query := `
		SELECT 
			c.column_name, 
			c.data_type, 
			c.column_default, 
			c.is_nullable,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_primary_key
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT ku.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku ON tc.constraint_name = ku.constraint_name
			WHERE tc.table_schema = $1 
				AND tc.table_name = $2 
				AND tc.constraint_type = 'PRIMARY KEY'
		) pk ON c.column_name = pk.column_name
		WHERE c.table_schema = $1 AND c.table_name = $2 
		ORDER BY c.ordinal_position`
	rows, err := db.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []types.Column
	for rows.Next() {
		var col types.Column
		var nullable string
		if err := rows.Scan(&col.Name, &col.DataType, &col.DefaultValue, &nullable, &col.IsPrimaryKey); err != nil {
			return nil, err
		}
		// Convert "YES"/"NO" to boolean
		col.IsNullable = nullable == "YES"
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func getIndexes(ctx context.Context, db *DB, schemaName, tableName string) ([]types.Index, error) {
	// Get ALL indexes (not just PK/unique) with their columns
	query := `
		SELECT 
			i.relname as index_name,
			ix.indisunique,
			ix.indisprimary,
			pg_get_indexdef(ix.indexrelid) as index_def,
			COALESCE(array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)), ARRAY[]::text[]) as columns
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		LEFT JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE t.relname = $2 
			AND t.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = $1)
		GROUP BY i.relname, ix.indisunique, ix.indisprimary, pg_get_indexdef(ix.indexrelid)
		ORDER BY i.relname`
	rows, err := db.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []types.Index
	for rows.Next() {
		var idx types.Index
		if err := rows.Scan(&idx.Name, &idx.IsUnique, &idx.IsPrimary, &idx.Definition, &idx.Columns); err != nil {
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
		// Get constraint columns
		cols, err := getConstraintColumns(ctx, db, schemaName, tableName, cons.Name)
		if err != nil {
			return nil, err
		}
		cons.Columns = cols
		constraints = append(constraints, cons)
	}
	return constraints, rows.Err()
}

func getConstraintColumns(ctx context.Context, db *DB, schemaName, tableName, constraintName string) ([]string, error) {
	query := `SELECT column_name FROM information_schema.key_column_usage WHERE table_schema = $1 AND table_name = $2 AND constraint_name = $3 ORDER BY ordinal_position`
	rows, err := db.Query(ctx, query, schemaName, tableName, constraintName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func getForeignKeys(ctx context.Context, db *DB, schemaName, tableName string) ([]types.ForeignKey, error) {
	// Get FKs with multi-column support, unique_constraint_name, and ON DELETE/UPDATE
	query := `
		SELECT 
			tc.constraint_name,
			ARRAY_AGG(kcu.column_name ORDER BY kcu.ordinal_position) as fk_columns,
			ccu.table_name as reference_table,
			ARRAY_AGG(ccu.column_name ORDER BY kcu.ordinal_position) as reference_columns,
			rc.unique_constraint_name,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name 
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu 
			ON tc.constraint_name = ccu.constraint_name
		JOIN (
			SELECT 
				r.conname as constraint_name,
				r.confrelid,
				r.delete_rule,
				r.update_rule,
				r.conname as unique_constraint_name
			FROM pg_catalog.pg_constraint r
			WHERE r.contype = 'f'
		) rc ON tc.constraint_name = rc.constraint_name
		WHERE tc.table_schema = $1 
			AND tc.table_name = $2 
			AND tc.constraint_type = 'FOREIGN KEY'
		GROUP BY tc.constraint_name, ccu.table_name, rc.unique_constraint_name, rc.delete_rule, rc.update_rule`
	rows, err := db.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []types.ForeignKey
	for rows.Next() {
		var fk types.ForeignKey
		if err := rows.Scan(
			&fk.Name,
			&fk.Columns,
			&fk.RefTable,
			&fk.RefColumns,
			&fk.UniqueConstraintName,
			&fk.OnDelete,
			&fk.OnUpdate,
		); err != nil {
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

// matchesIgnorePatterns checks if a name matches any of the ignore patterns
// Supports: exact match, *suffix, prefix*, *middle*, and ? for single char
func matchesIgnorePatterns(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(name, pattern) {
			return true
		}
	}
	return false
}

// matchPattern implements glob-style matching (*, ?)
func matchPattern(name, pattern string) bool {
	// Exact match
	if name == pattern {
		return true
	}

	// Use filepath.Match for glob patterns
	matched, _ := filepath.Match(pattern, name)
	if matched {
		return true
	}

	// Handle * in pattern (filepath.Match should handle this, but just in case)
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			// prefix*suffix
			return strings.HasPrefix(name, parts[0]) && strings.HasSuffix(name, parts[1])
		}
	}

	return false
}

// filterColumns filters columns based on ignore patterns
func filterColumns(columns []types.Column, ignorePatterns []string) []types.Column {
	if len(ignorePatterns) == 0 {
		return columns
	}

	// Add default patterns if not specified
	allPatterns := append(DefaultIgnorePatterns, ignorePatterns...)

	var filtered []types.Column
	for _, col := range columns {
		if !matchesIgnorePatterns(col.Name, allPatterns) {
			filtered = append(filtered, col)
		}
	}
	return filtered
}

func getViews(ctx context.Context, db *DB, schemaName string) ([]types.View, error) {
	query := `SELECT table_name, view_definition FROM information_schema.views WHERE table_schema = $1`
	rows, err := db.Query(ctx, query, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []types.View
	for rows.Next() {
		var v types.View
		if err := rows.Scan(&v.Name, &v.Definition); err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, rows.Err()
}

func getMaterializedViews(ctx context.Context, db *DB, schemaName string) ([]types.MaterializedView, error) {
	query := `SELECT matviewname, definition FROM pg_matviews WHERE schemaname = $1`
	rows, err := db.Query(ctx, query, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matViews []types.MaterializedView
	for rows.Next() {
		var mv types.MaterializedView
		if err := rows.Scan(&mv.Name, &mv.Definition); err != nil {
			return nil, err
		}
		matViews = append(matViews, mv)
	}
	return matViews, rows.Err()
}

func getFunctions(ctx context.Context, db *DB, schemaName string) ([]types.Function, error) {
	// Query to extract functions, handling overloading correctly
	query := `
		SELECT 
			p.proname as name,
			pg_get_function_identity_arguments(p.oid) as arguments,
			pg_get_functiondef(p.oid) as definition
		FROM pg_proc p
		JOIN pg_namespace n ON p.pronamespace = n.oid
		WHERE n.nspname = $1
	`
	rows, err := db.Query(ctx, query, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var functions []types.Function
	for rows.Next() {
		var f types.Function
		if err := rows.Scan(&f.Name, &f.Arguments, &f.Definition); err != nil {
			return nil, err
		}
		functions = append(functions, f)
	}
	return functions, rows.Err()
}

func getTriggers(ctx context.Context, db *DB, schemaName string) ([]types.Trigger, error) {
	// Query to extract triggers (excluding internal/constraint triggers)
	query := `
		SELECT 
			tr.tgname as name,
			tbl.relname as table,
			pg_get_triggerdef(tr.oid) as definition
		FROM pg_trigger tr
		JOIN pg_class tbl ON tr.tgrelid = tbl.oid
		JOIN pg_namespace n ON tbl.relnamespace = n.oid
		WHERE n.nspname = $1 AND tr.tgisinternal = false
	`
	rows, err := db.Query(ctx, query, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var triggers []types.Trigger
	for rows.Next() {
		var t types.Trigger
		if err := rows.Scan(&t.Name, &t.Table, &t.Definition); err != nil {
			return nil, err
		}
		triggers = append(triggers, t)
	}
	return triggers, rows.Err()
}

func getGrants(ctx context.Context, db *DB, schemaName string) ([]types.Grant, error) {
	// Query to extract table privileges excluding root owners automatically generated grants
	query := `
		SELECT 
			table_name as table,
			grantee,
			privilege_type,
			is_grantable = 'YES' as is_grantable
		FROM information_schema.role_table_grants
		WHERE table_schema = $1 AND grantee != grantor
	`
	rows, err := db.Query(ctx, query, schemaName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []types.Grant
	for rows.Next() {
		var g types.Grant
		if err := rows.Scan(&g.Table, &g.Grantee, &g.Privilege, &g.IsGrantable); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

// EnsureMigrationsTable creates the migration tracking table if it doesn't exist
func EnsureMigrationsTable(ctx context.Context, db *DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS _dbdiff_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := db.Exec(ctx, query)
	return err
}

// GetAppliedMigrations returns a list of applied migration versions
func GetAppliedMigrations(ctx context.Context, db *DB) ([]string, error) {
	query := `SELECT version FROM _dbdiff_migrations ORDER BY applied_at ASC`
	rows, err := db.Query(ctx, query)
	if err != nil {
		// If table doesn't exist, return empty list instead of error
		if strings.Contains(err.Error(), "does not exist") {
			return []string{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

// RecordMigration records a successful migration execution
func RecordMigration(ctx context.Context, db *DB, version string) error {
	query := `INSERT INTO _dbdiff_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING`
	_, err := db.Exec(ctx, query, version)
	return err
}

// RemoveMigration deletes a recorded migration (used during rollback)
func RemoveMigration(ctx context.Context, db *DB, version string) error {
	query := `DELETE FROM _dbdiff_migrations WHERE version = $1`
	_, err := db.Exec(ctx, query, version)
	return err
}
