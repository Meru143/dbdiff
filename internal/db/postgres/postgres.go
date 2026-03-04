package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meru143/dbdiff/pkg/types"
)

const (
	maxRetries     = 3
	baseRetryDelay = 500 * time.Millisecond
)

// Driver implements db.Driver for PostgreSQL using pgx
type Driver struct {
	pool *pgxpool.Pool
}

// New creates a new unconnected PostgreSQL driver
func New() *Driver {
	return &Driver{}
}

func (d *Driver) Name() string    { return "postgres" }
func (d *Driver) Dialect() string { return "postgresql" }

// Connect establishes a connection to PostgreSQL with retry logic
func (d *Driver) Connect(ctx context.Context, connStr, sslMode string, timeout time.Duration) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseRetryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		pool, err := connectOnce(ctx, connStr, sslMode, timeout)
		if err == nil {
			d.pool = pool
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, lastErr)
}

func connectOnce(ctx context.Context, connStr, sslMode string, timeout time.Duration) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	if sslMode == "disable" {
		config.ConnConfig.TLSConfig = nil
	}

	if timeout > 0 {
		config.ConnConfig.ConnectTimeout = timeout
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	queryCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		queryCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	pool, err := pgxpool.NewWithConfig(queryCtx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	if err := pool.Ping(queryCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping: %w", err)
	}

	return pool, nil
}

// Close closes the connection pool
func (d *Driver) Close() {
	if d.pool != nil {
		d.pool.Close()
	}
}

// Exec executes a SQL statement
func (d *Driver) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := d.pool.Exec(ctx, sql, args...)
	return err
}

// Introspect performs full schema introspection
func (d *Driver) Introspect(ctx context.Context, schemaName string, ignorePatterns []string) (*types.Schema, error) {
	schema := &types.Schema{
		Tables:            make([]types.Table, 0),
		Sequences:         make([]types.Sequence, 0),
		Types:             make([]types.Type, 0),
		Views:             make([]types.View, 0),
		MaterializedViews: make([]types.MaterializedView, 0),
		Functions:         make([]types.Function, 0),
		Triggers:          make([]types.Trigger, 0),
		Grants:            make([]types.Grant, 0),
	}

	tables, err := getTables(ctx, d.pool, schemaName, ignorePatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	for _, tableName := range tables {
		columns, err := getColumns(ctx, d.pool, schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for %s: %w", tableName, err)
		}

		filteredColumns := filterColumns(columns, ignorePatterns)
		if len(filteredColumns) == 0 {
			continue
		}

		indexes, err := getIndexes(ctx, d.pool, schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get indexes for %s: %w", tableName, err)
		}

		constraints, err := getConstraints(ctx, d.pool, schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get constraints for %s: %w", tableName, err)
		}

		fks, err := getForeignKeys(ctx, d.pool, schemaName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get foreign keys for %s: %w", tableName, err)
		}

		table := types.Table{
			Name:        tableName,
			Columns:     filteredColumns,
			Indexes:     indexes,
			Constraints: constraints,
			ForeignKeys: fks,
		}
		schema.Tables = append(schema.Tables, table)
	}

	sequences, err := getSequences(ctx, d.pool, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get sequences: %w", err)
	}
	schema.Sequences = sequences

	customTypes, err := getCustomTypes(ctx, d.pool, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get custom types: %w", err)
	}
	schema.Types = customTypes

	views, err := getViews(ctx, d.pool, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get views: %w", err)
	}
	schema.Views = views

	matViews, err := getMaterializedViews(ctx, d.pool, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get materialized views: %w", err)
	}
	schema.MaterializedViews = matViews

	functions, err := getFunctions(ctx, d.pool, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get functions: %w", err)
	}
	schema.Functions = functions

	triggers, err := getTriggers(ctx, d.pool, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get triggers: %w", err)
	}
	schema.Triggers = triggers

	grants, err := getGrants(ctx, d.pool, schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get grants: %w", err)
	}
	schema.Grants = grants

	return schema, nil
}

// ListTables returns table names in the given schema
func (d *Driver) ListTables(ctx context.Context, schema string, ignorePatterns []string) ([]string, error) {
	return getTables(ctx, d.pool, schema, ignorePatterns)
}

// GetServerInfo returns database server metadata
func (d *Driver) GetServerInfo(ctx context.Context) (*types.ServerInfo, error) {
	var info types.ServerInfo
	err := d.pool.QueryRow(ctx, "SELECT current_database(), current_user, version()").Scan(
		&info.Database, &info.User, &info.Version,
	)
	return &info, err
}

// EnsureMigrationsTable creates the migration tracking table if not exists
func (d *Driver) EnsureMigrationsTable(ctx context.Context) error {
	return EnsureMigrationsTable(ctx, d.pool)
}

// GetAppliedMigrations returns applied migration versions
func (d *Driver) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	return GetAppliedMigrations(ctx, d.pool)
}

// RecordMigration records a successful migration
func (d *Driver) RecordMigration(ctx context.Context, version string) error {
	return RecordMigration(ctx, d.pool, version)
}

// RemoveMigration deletes a migration record
func (d *Driver) RemoveMigration(ctx context.Context, version string) error {
	return RemoveMigration(ctx, d.pool, version)
}
