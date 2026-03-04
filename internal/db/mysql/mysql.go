package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/meru143/dbdiff/pkg/types"
)

// Driver implements the db.Driver interface for MySQL
type Driver struct {
	db *sql.DB
}

// New creates a new MySQL driver instance
func New() *Driver {
	return &Driver{}
}

// Connect establishes a connection to the MySQL database
func (d *Driver) Connect(ctx context.Context, connStr string, sslMode string, timeout time.Duration) error {
	// Clean up the connStr if it starts with mysql:// as go-sql-driver expects user:pass@tcp(host:port)/dbname
	// This simple conversion handles standard format but might need more logic
	// e.g. mysql://user:pass@host:port/dbname -> user:pass@tcp(host:port)/dbname
	// First, let's let the DB connect with standard sql.Open

	// Remove the "mysql://" prefix for the go-sql-driver
	dsn := connStr
	if len(dsn) > 8 && dsn[:8] == "mysql://" {
		dsn = dsn[8:]
		// Replace the first '/' with '@tcp(host)/' if no '@' or 'tcp(' exists?
		// A full parser might be better here, but let's assume valid go-sql-driver DSN for now if they strip it
	}

	// For simplicity, we assume the user provides a complete valid DSN, or we parse it.
	// We might need to ensure multiStatements=true is set for migrations

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open mysql connection: %w", err)
	}

	conn.SetConnMaxLifetime(time.Minute * 3)
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(10)

	// Ping to verify
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping mysql database: %w", err)
	}

	d.db = conn
	return nil
}

// Close closes the database connection
func (d *Driver) Close() {
	if d.db != nil {
		d.db.Close()
	}
}

// Exec executes a SQL statement
func (d *Driver) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := d.db.ExecContext(ctx, sql, args...)
	return err
}

// Name returns the driver name
func (d *Driver) Name() string {
	return "mysql"
}

// Dialect returns the SQL dialect for DDL generation
func (d *Driver) Dialect() string {
	return "mysql"
}

// Introspect performs full schema introspection for MySQL
func (d *Driver) Introspect(ctx context.Context, schema string, ignorePatterns []string) (*types.Schema, error) {
	if schema == "" {
		// Query the current database name
		var dbName string
		if err := d.db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&dbName); err != nil {
			return nil, fmt.Errorf("failed to determine current database: %w", err)
		}
		schema = dbName
	}

	result := &types.Schema{}

	tables, err := d.ListTables(ctx, schema, ignorePatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	for _, tableName := range tables {
		table := types.Table{Name: tableName}

		columns, err := getColumns(ctx, d.db, schema, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
		}
		table.Columns = filterColumns(columns, ignorePatterns)

		indexes, err := getIndexes(ctx, d.db, schema, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get indexes for table %s: %w", tableName, err)
		}
		table.Indexes = indexes

		constraints, err := getConstraints(ctx, d.db, schema, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get constraints for table %s: %w", tableName, err)
		}
		table.Constraints = constraints

		fks, err := getForeignKeys(ctx, d.db, schema, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get foreign keys for table %s: %w", tableName, err)
		}
		table.ForeignKeys = fks

		result.Tables = append(result.Tables, table)
	}

	views, err := getViews(ctx, d.db, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to get views: %w", err)
	}
	result.Views = views

	return result, nil
}
