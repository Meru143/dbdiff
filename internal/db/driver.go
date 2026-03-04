package db

import (
	"context"
	"time"

	"github.com/meru143/dbdiff/pkg/types"
)

// Driver is the interface that all database backends must implement.
// Each supported database engine (PostgreSQL, MySQL, SQL Server) provides
// its own implementation of this interface.
type Driver interface {
	// Connect establishes a connection to the database
	Connect(ctx context.Context, connStr string, sslMode string, timeout time.Duration) error

	// Close closes the database connection
	Close()

	// Introspect performs full schema introspection and returns a unified Schema
	Introspect(ctx context.Context, schema string, ignorePatterns []string) (*types.Schema, error)

	// Exec executes a SQL statement (for migrations, DDL, etc.)
	Exec(ctx context.Context, sql string, args ...any) error

	// Name returns the driver name (e.g., "postgres", "mysql")
	Name() string

	// Dialect returns the SQL dialect for DDL generation
	Dialect() string

	// ListTables returns table names in the given schema
	ListTables(ctx context.Context, schema string, ignorePatterns []string) ([]string, error)

	// GetServerInfo returns database server metadata
	GetServerInfo(ctx context.Context) (*types.ServerInfo, error)

	// Migration tracking
	EnsureMigrationsTable(ctx context.Context) error
	GetAppliedMigrations(ctx context.Context) ([]string, error)
	RecordMigration(ctx context.Context, version string) error
	RemoveMigration(ctx context.Context, version string) error
}
