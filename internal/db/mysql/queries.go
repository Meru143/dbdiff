package mysql

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"

	"github.com/meru143/dbdiff/pkg/types"
)

// Default ignore patterns for columns
var DefaultIgnorePatterns = []string{"_created_at", "_updated_at", "_modified_at", "_deleted_at"}

// ListTables returns table names in the given schema
func (d *Driver) ListTables(ctx context.Context, schema string, ignorePatterns []string) ([]string, error) {
	// In MySQL, the schema is often just the currently selected DB
	query := "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'"
	rows, err := d.db.QueryContext(ctx, query)
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
		if !matchesIgnorePatterns(name, ignorePatterns) {
			tables = append(tables, name)
		}
	}
	return tables, rows.Err()
}

// GetServerInfo returns database server metadata
func (d *Driver) GetServerInfo(ctx context.Context) (*types.ServerInfo, error) {
	var info types.ServerInfo
	err := d.db.QueryRowContext(ctx, "SELECT DATABASE(), CURRENT_USER(), VERSION()").Scan(
		&info.Database, &info.User, &info.Version,
	)
	return &info, err
}

// EnsureMigrationsTable creates the migration tracking table if not exists
func (d *Driver) EnsureMigrationsTable(ctx context.Context) error {
	query := `CREATE TABLE IF NOT EXISTS _dbdiff_migrations (version VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`
	_, err := d.db.ExecContext(ctx, query)
	return err
}

// GetAppliedMigrations returns applied migration versions
func (d *Driver) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	query := `SELECT version FROM _dbdiff_migrations ORDER BY applied_at ASC`
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		if strings.Contains(err.Error(), "Table") && strings.Contains(err.Error(), "doesn't exist") {
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

// RecordMigration records a successful migration
func (d *Driver) RecordMigration(ctx context.Context, version string) error {
	query := `INSERT IGNORE INTO _dbdiff_migrations (version) VALUES (?)`
	_, err := d.db.ExecContext(ctx, query, version)
	return err
}

// RemoveMigration deletes a migration record
func (d *Driver) RemoveMigration(ctx context.Context, version string) error {
	query := `DELETE FROM _dbdiff_migrations WHERE version = ?`
	_, err := d.db.ExecContext(ctx, query, version)
	return err
}

// --- Query Helpers ---

func getColumns(ctx context.Context, dbConn *sql.DB, schemaName, tableName string) ([]types.Column, error) {
	query := `
		SELECT 
			c.COLUMN_NAME, 
			c.DATA_TYPE, 
			c.COLUMN_DEFAULT, 
			c.IS_NULLABLE,
			CASE WHEN k.COLUMN_NAME IS NOT NULL THEN true ELSE false END as is_primary_key
		FROM information_schema.COLUMNS c
		LEFT JOIN information_schema.KEY_COLUMN_USAGE k 
			ON c.TABLE_SCHEMA = k.TABLE_SCHEMA 
			AND c.TABLE_NAME = k.TABLE_NAME 
			AND c.COLUMN_NAME = k.COLUMN_NAME 
			AND k.CONSTRAINT_NAME = 'PRIMARY'
		WHERE c.TABLE_SCHEMA = DATABASE() AND c.TABLE_NAME = ?
		ORDER BY c.ORDINAL_POSITION`
	rows, err := dbConn.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []types.Column
	for rows.Next() {
		var col types.Column
		var nullable string
		// Default can be nil in MySQL, so use sql.NullString
		var defaultVal sql.NullString

		if err := rows.Scan(&col.Name, &col.DataType, &defaultVal, &nullable, &col.IsPrimaryKey); err != nil {
			return nil, err
		}
		col.IsNullable = nullable == "YES"
		if defaultVal.Valid {
			dv := defaultVal.String
			col.DefaultValue = &dv
		}
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func getIndexes(ctx context.Context, dbConn *sql.DB, schemaName, tableName string) ([]types.Index, error) {
	query := `
		SELECT 
			INDEX_NAME, 
			NON_UNIQUE = 0 AS is_unique,
			INDEX_NAME = 'PRIMARY' AS is_primary,
			COLUMN_NAME
		FROM information_schema.STATISTICS 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`

	rows, err := dbConn.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	idxMap := make(map[string]*types.Index)
	var orderedNames []string

	for rows.Next() {
		var name string
		var isUnique, isPrimary bool
		var colName string

		if err := rows.Scan(&name, &isUnique, &isPrimary, &colName); err != nil {
			return nil, err
		}

		if idx, exists := idxMap[name]; exists {
			idx.Columns = append(idx.Columns, colName)
		} else {
			idxMap[name] = &types.Index{
				Name:      name,
				IsUnique:  isUnique,
				IsPrimary: isPrimary,
				Columns:   []string{colName},
			}
			orderedNames = append(orderedNames, name)
		}
	}

	var indexes []types.Index
	for _, name := range orderedNames {
		idx := idxMap[name]
		if !idx.IsPrimary { // Many tools separate PK from general indexes
			indexes = append(indexes, *idx)
		}
	}
	return indexes, rows.Err()
}

func getConstraints(ctx context.Context, dbConn *sql.DB, schemaName, tableName string) ([]types.Constraint, error) {
	query := `
		SELECT 
			tc.CONSTRAINT_NAME, 
			tc.CONSTRAINT_TYPE,
			kcu.COLUMN_NAME
		FROM information_schema.TABLE_CONSTRAINTS tc
		JOIN information_schema.KEY_COLUMN_USAGE kcu
			ON tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME AND tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA
		WHERE tc.TABLE_SCHEMA = DATABASE() AND tc.TABLE_NAME = ? AND tc.CONSTRAINT_TYPE IN ('PRIMARY KEY', 'UNIQUE', 'CHECK')
		ORDER BY tc.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`

	rows, err := dbConn.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	consMap := make(map[string]*types.Constraint)
	var orderedNames []string

	for rows.Next() {
		var name, cType, colName string

		if err := rows.Scan(&name, &cType, &colName); err != nil {
			return nil, err
		}

		if cons, exists := consMap[name]; exists {
			cons.Columns = append(cons.Columns, colName)
		} else {
			consMap[name] = &types.Constraint{
				Name:    name,
				Type:    cType,
				Columns: []string{colName},
			}
			orderedNames = append(orderedNames, name)
		}
	}

	var constraints []types.Constraint
	for _, name := range orderedNames {
		constraints = append(constraints, *consMap[name])
	}
	return constraints, rows.Err()
}

func getForeignKeys(ctx context.Context, dbConn *sql.DB, schemaName, tableName string) ([]types.ForeignKey, error) {
	query := `
		SELECT 
			CONSTRAINT_NAME,
			COLUMN_NAME,
			REFERENCED_TABLE_NAME,
			REFERENCED_COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY CONSTRAINT_NAME, ORDINAL_POSITION`

	rows, err := dbConn.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fkMap := make(map[string]*types.ForeignKey)
	var orderedNames []string

	for rows.Next() {
		var name, col, refTable, refCol string
		if err := rows.Scan(&name, &col, &refTable, &refCol); err != nil {
			return nil, err
		}

		if fk, exists := fkMap[name]; exists {
			fk.Columns = append(fk.Columns, col)
			fk.RefColumns = append(fk.RefColumns, refCol)
		} else {
			fkMap[name] = &types.ForeignKey{
				Name:       name,
				RefTable:   refTable,
				Columns:    []string{col},
				RefColumns: []string{refCol},
			}
			orderedNames = append(orderedNames, name)
		}
	}

	var fks []types.ForeignKey
	for _, name := range orderedNames {
		// fetch on update / delete
		rcQuery := `SELECT UPDATE_RULE, DELETE_RULE FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = DATABASE() AND CONSTRAINT_NAME = ?`
		var onUpd, onDel string
		if err := dbConn.QueryRowContext(ctx, rcQuery, name).Scan(&onUpd, &onDel); err == nil {
			fkMap[name].OnUpdate = onUpd
			fkMap[name].OnDelete = onDel
		}

		fks = append(fks, *fkMap[name])
	}
	return fks, rows.Err()
}

func getViews(ctx context.Context, dbConn *sql.DB, schemaName string) ([]types.View, error) {
	query := `SELECT TABLE_NAME, VIEW_DEFINITION FROM information_schema.VIEWS WHERE TABLE_SCHEMA = DATABASE()`
	rows, err := dbConn.QueryContext(ctx, query)
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

// --- Utility functions ---

func matchesIgnorePatterns(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(name, pattern) {
			return true
		}
	}
	return false
}

func matchPattern(name, pattern string) bool {
	if name == pattern {
		return true
	}
	matched, _ := filepath.Match(pattern, name)
	if matched {
		return true
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(name, parts[0]) && strings.HasSuffix(name, parts[1])
		}
	}
	return false
}

func filterColumns(columns []types.Column, ignorePatterns []string) []types.Column {
	if len(ignorePatterns) == 0 {
		return columns
	}
	allPatterns := make([]string, 0, len(DefaultIgnorePatterns)+len(ignorePatterns))
	allPatterns = append(allPatterns, DefaultIgnorePatterns...)
	allPatterns = append(allPatterns, ignorePatterns...)

	var filtered []types.Column
	for _, col := range columns {
		if !matchesIgnorePatterns(col.Name, allPatterns) {
			filtered = append(filtered, col)
		}
	}
	return filtered
}
