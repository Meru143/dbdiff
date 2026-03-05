package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	"github.com/meru143/dbdiff/pkg/types"
	_ "github.com/microsoft/go-mssqldb"
)

// Driver implements the db.Driver interface for SQL Server
type Driver struct {
	db *sql.DB
}

// New creates a new SQL Server driver instance
func New() *Driver {
	return &Driver{}
}

// Connect establishes a connection to the SQL Server database
func (d *Driver) Connect(ctx context.Context, connStr string, sslMode string, timeout time.Duration) error {
	// go-mssqldb accepts sqlserver:// URLs directly
	dsn := connStr

	// Parse and ensure connection timeout
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("failed to parse sqlserver connection string: %w", err)
	}

	q := u.Query()
	if q.Get("connection timeout") == "" {
		q.Set("connection timeout", fmt.Sprintf("%d", int(timeout.Seconds())))
	}
	if sslMode == "disable" {
		q.Set("encrypt", "disable")
	}
	u.RawQuery = q.Encode()
	dsn = u.String()

	conn, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return fmt.Errorf("failed to open sqlserver connection: %w", err)
	}

	conn.SetConnMaxLifetime(time.Minute * 3)
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(10)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping sqlserver database: %w", err)
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
	return "sqlserver"
}

// Dialect returns the SQL dialect for DDL generation
func (d *Driver) Dialect() string {
	return "sqlserver"
}

// Introspect performs full schema introspection for SQL Server
func (d *Driver) Introspect(ctx context.Context, schema string, ignorePatterns []string) (*types.Schema, error) {
	if schema == "" {
		schema = "dbo"
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
