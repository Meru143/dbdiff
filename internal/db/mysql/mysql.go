package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	gomysql "github.com/go-sql-driver/mysql"
	dbdsn "github.com/meru143/dbdiff/internal/db/dsn"
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

// parseDSN reads connStr, a go-sql-driver DSN after mysql:// or mysql+tcp:// in any case
// (mysql://user:pass@tcp(host:3306)/db), as go-sql-driver does.
func parseDSN(connStr string) (*gomysql.Config, error) {
	dsn := connStr
	if lower := strings.ToLower(dsn); strings.HasPrefix(lower, "mysql://") || strings.HasPrefix(lower, "mysql+tcp://") {
		dsn = dsn[strings.Index(dsn, "://")+3:]
	}
	cfg, err := gomysql.ParseDSN(dsn)
	if err != nil {
		// go-sql-driver's reason can quote part of the password: without a /database after
		// the address, it reads the database, or the network, from the password. The redacted
		// DSN fails only if the rest of it does, and its error holds no password. It is
		// redacted as a URL: Redact reads a DSN without a scheme as prose when its user name
		// holds a '=' or a space.
		redacted := dbdsn.Redact("mysql://" + dsn)
		if _, rerr := gomysql.ParseDSN(strings.TrimPrefix(redacted, "mysql://")); rerr != nil {
			return nil, fmt.Errorf("invalid mysql connection string: %w", rerr)
		}
		return nil, fmt.Errorf("invalid mysql connection string %q", redacted)
	}
	return cfg, nil
}

// Connect establishes a connection to the MySQL database
func (d *Driver) Connect(ctx context.Context, connStr string, sslMode string, timeout time.Duration) error {
	cfg, err := parseDSN(connStr)
	if err != nil {
		return err
	}
	// A migration file holds many statements, and apply runs it in one Exec.
	cfg.MultiStatements = true

	conn, err := sql.Open("mysql", cfg.FormatDSN())
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

// NormalizeType normalizes a database-specific data type into a standard format
func (d *Driver) NormalizeType(dataType string) string {
	switch dataType {
	case "int", "integer", "mediumint", "smallint", "tinyint":
		return "integer"
	case "varchar", "char", "text", "mediumtext", "longtext", "tinytext":
		return "varchar"
	case "decimal", "numeric", "double", "float":
		return "numeric"
	case "datetime", "timestamp":
		return "timestamp"
	default:
		return dataType
	}
}

// Introspect performs full schema introspection for MySQL
func (d *Driver) Introspect(ctx context.Context, schema string, ignorePatterns []string) (*types.Schema, error) {
	// Index definitions name their table the way the rest of the migration does: qualified
	// when --schema picks a database, plain when it is the connection's own.
	qualifier := ""
	if schema != "" && schema != "public" {
		qualifier = schema + "."
	}
	schema, err := d.databaseName(ctx, schema)
	if err != nil {
		return nil, err
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

		indexes, err := getIndexes(ctx, d.db, schema, tableName, qualifier)
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

// databaseName resolves the schema to read. MySQL's schemas are its databases, and the
// CLI's default schema, public, is PostgreSQL's: both it and "" mean the connection's own
// database.
func (d *Driver) databaseName(ctx context.Context, schema string) (string, error) {
	if schema != "" && schema != "public" {
		return schema, nil
	}
	var name sql.NullString
	if err := d.db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		return "", fmt.Errorf("failed to determine current database: %w", err)
	}
	if !name.Valid {
		return "", fmt.Errorf("the connection string names no database: add one or pass --schema")
	}
	return name.String, nil
}
