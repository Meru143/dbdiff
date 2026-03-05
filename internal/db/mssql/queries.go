package mssql

import (
	"context"
	"database/sql"
	"path/filepath"

	"github.com/meru143/dbdiff/pkg/types"
)

// DefaultIgnorePatterns filters out internal SQL Server objects
var DefaultIgnorePatterns = []string{
	"sysdiagrams",
	"__*",
	"_dbdiff_*",
}

// ListTables returns table names in the given schema
func (d *Driver) ListTables(ctx context.Context, schema string, ignorePatterns []string) ([]string, error) {
	query := `SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = @p1 AND TABLE_TYPE = 'BASE TABLE' 
		ORDER BY TABLE_NAME`
	rows, err := d.db.QueryContext(ctx, query, schema)
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

// GetServerInfo returns SQL Server metadata
func (d *Driver) GetServerInfo(ctx context.Context) (*types.ServerInfo, error) {
	info := &types.ServerInfo{}
	err := d.db.QueryRowContext(ctx,
		"SELECT DB_NAME(), SUSER_SNAME(), @@VERSION").Scan(
		&info.Database, &info.User, &info.Version,
	)
	return info, err
}

// EnsureMigrationsTable creates the migrations tracking table
func (d *Driver) EnsureMigrationsTable(ctx context.Context) error {
	query := `IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = '_dbdiff_migrations')
		CREATE TABLE _dbdiff_migrations (
			version NVARCHAR(255) NOT NULL PRIMARY KEY,
			applied_at DATETIME2 DEFAULT GETDATE()
		)`
	_, err := d.db.ExecContext(ctx, query)
	return err
}

// GetAppliedMigrations returns list of applied migration versions
func (d *Driver) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT version FROM _dbdiff_migrations ORDER BY applied_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// RecordMigration records a migration version
func (d *Driver) RecordMigration(ctx context.Context, version string) error {
	query := `IF NOT EXISTS (SELECT 1 FROM _dbdiff_migrations WHERE version = @p1)
		INSERT INTO _dbdiff_migrations (version) VALUES (@p1)`
	_, err := d.db.ExecContext(ctx, query, version)
	return err
}

// RemoveMigration removes a migration record
func (d *Driver) RemoveMigration(ctx context.Context, version string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM _dbdiff_migrations WHERE version = @p1", version)
	return err
}

// getColumns returns columns for a table
func getColumns(ctx context.Context, dbConn *sql.DB, schema, tableName string) ([]types.Column, error) {
	query := `SELECT 
			c.COLUMN_NAME,
			c.DATA_TYPE,
			c.COLUMN_DEFAULT,
			c.IS_NULLABLE,
			CASE WHEN pk.COLUMN_NAME IS NOT NULL THEN 1 ELSE 0 END as is_primary_key
		FROM INFORMATION_SCHEMA.COLUMNS c
		LEFT JOIN (
			SELECT ku.COLUMN_NAME
			FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
			JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku
				ON tc.CONSTRAINT_NAME = ku.CONSTRAINT_NAME
			WHERE tc.TABLE_SCHEMA = @p1 AND tc.TABLE_NAME = @p2 AND tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
		) pk ON c.COLUMN_NAME = pk.COLUMN_NAME
		WHERE c.TABLE_SCHEMA = @p1 AND c.TABLE_NAME = @p2
		ORDER BY c.ORDINAL_POSITION`

	rows, err := dbConn.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []types.Column
	for rows.Next() {
		var col types.Column
		var nullable string
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

// getIndexes returns indexes for a table
func getIndexes(ctx context.Context, dbConn *sql.DB, schema, tableName string) ([]types.Index, error) {
	query := `SELECT 
			i.name AS INDEX_NAME,
			i.is_unique,
			i.is_primary_key,
			c.name AS COLUMN_NAME
		FROM sys.indexes i
		JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		JOIN sys.tables t ON i.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE s.name = @p1 AND t.name = @p2 AND i.name IS NOT NULL
		ORDER BY i.name, ic.key_ordinal`

	rows, err := dbConn.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	idxMap := make(map[string]*types.Index)
	var orderedNames []string

	for rows.Next() {
		var name, colName string
		var isUnique, isPrimary bool

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
		if !idx.IsPrimary {
			indexes = append(indexes, *idx)
		}
	}
	return indexes, rows.Err()
}

// getConstraints returns constraints for a table
func getConstraints(ctx context.Context, dbConn *sql.DB, schema, tableName string) ([]types.Constraint, error) {
	query := `SELECT 
			tc.CONSTRAINT_NAME,
			tc.CONSTRAINT_TYPE,
			kcu.COLUMN_NAME
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
		JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu
			ON tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME AND tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA
		WHERE tc.TABLE_SCHEMA = @p1 AND tc.TABLE_NAME = @p2
			AND tc.CONSTRAINT_TYPE IN ('PRIMARY KEY', 'UNIQUE', 'CHECK')
		ORDER BY tc.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`

	rows, err := dbConn.QueryContext(ctx, query, schema, tableName)
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

// getForeignKeys returns foreign keys for a table
func getForeignKeys(ctx context.Context, dbConn *sql.DB, schema, tableName string) ([]types.ForeignKey, error) {
	query := `SELECT 
			fk.name AS CONSTRAINT_NAME,
			c.name AS COLUMN_NAME,
			OBJECT_NAME(fk.referenced_object_id) AS REFERENCED_TABLE,
			rc.name AS REFERENCED_COLUMN,
			fk.update_referential_action_desc,
			fk.delete_referential_action_desc
		FROM sys.foreign_keys fk
		JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		JOIN sys.columns c ON fkc.parent_object_id = c.object_id AND fkc.parent_column_id = c.column_id
		JOIN sys.columns rc ON fkc.referenced_object_id = rc.object_id AND fkc.referenced_column_id = rc.column_id
		JOIN sys.tables t ON fk.parent_object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE s.name = @p1 AND t.name = @p2
		ORDER BY fk.name, fkc.constraint_column_id`

	rows, err := dbConn.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fkMap := make(map[string]*types.ForeignKey)
	var orderedNames []string

	for rows.Next() {
		var name, col, refTable, refCol, onUpdate, onDelete string
		if err := rows.Scan(&name, &col, &refTable, &refCol, &onUpdate, &onDelete); err != nil {
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
				OnUpdate:   onUpdate,
				OnDelete:   onDelete,
			}
			orderedNames = append(orderedNames, name)
		}
	}

	var fks []types.ForeignKey
	for _, name := range orderedNames {
		fks = append(fks, *fkMap[name])
	}
	return fks, rows.Err()
}

// getViews returns views in the schema
func getViews(ctx context.Context, dbConn *sql.DB, schema string) ([]types.View, error) {
	query := `SELECT TABLE_NAME, VIEW_DEFINITION 
		FROM INFORMATION_SCHEMA.VIEWS 
		WHERE TABLE_SCHEMA = @p1`
	rows, err := dbConn.QueryContext(ctx, query, schema)
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

// matchesIgnorePatterns checks if name matches any pattern
func matchesIgnorePatterns(name string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// filterColumns removes columns matching ignore patterns
func filterColumns(columns []types.Column, patterns []string) []types.Column {
	if len(patterns) == 0 {
		return columns
	}
	var filtered []types.Column
	for _, col := range columns {
		if !matchesIgnorePatterns(col.Name, patterns) {
			filtered = append(filtered, col)
		}
	}
	return filtered
}
