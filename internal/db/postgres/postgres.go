package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meru143/dbdiff/internal/db/dsn"
	"github.com/meru143/dbdiff/pkg/types"
)

const (
	maxRetries     = 3
	baseRetryDelay = 500 * time.Millisecond
)

// safeReasons are pgx's parse reasons that name no value, so connectOnce shows them for a URL
// whose password parameter may run on past an '&'. Any other reason may repeat part of what
// follows the password, escaped or trimmed (unknown target_session_attrs value: x), and gets
// the generic hint instead.
var safeReasons = []string{
	"invalid port",
	"failed to configure TLS (sslmode is invalid)",
	"failed to configure TLS (unable to add CA to cert pool)",
	"failed to configure TLS (failed to decode sslkey)",
	"failed to configure TLS (unable to find sslpassword)",
	"invalid connect_timeout (negative timeout)",
	"min_protocol_version cannot be greater than max_protocol_version",
	"pool_max_conns too small",
}

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

// NormalizeType normalizes a database-specific data type into a standard format
func (d *Driver) NormalizeType(dataType string) string {
	switch dataType {
	case "character varying", "varchar":
		return "varchar"
	case "integer", "int4":
		return "integer"
	case "bigint", "int8":
		return "bigint"
	case "timestamp without time zone":
		return "timestamp"
	case "timestamp with time zone":
		return "timestamptz"
	case "boolean", "bool":
		return "boolean"
	case "numeric", "decimal":
		return "numeric"
	default:
		return dataType
	}
}

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
		// pgx hides the password in the URL it quotes, but when the query holds a password
		// parameter, its reason and that URL can hold part of what follows the parameter's
		// '=': a raw '&' pgx read as its own parameter, or a value it repeats. Re-parse a copy
		// whose password is masked but whose other parameters stay (RedactToSeparator), so pgx
		// still reports a genuinely bad one. With no password parameter, show its error,
		// redacted. With one, show only a reason that names no value (safeReasons), after the
		// URL redacted, never pgx's own text; otherwise give the hint, which a bad parameter
		// after the password, or the password value itself, falls to.
		rs := dsn.RedactToSeparator(connStr)
		if _, rerr := pgxpool.ParseConfig(rs); rerr != nil {
			if !dsn.HasPasswordParam(connStr) {
				return nil, fmt.Errorf("failed to parse connection string: %s", dsn.Redact(rerr.Error()))
			}
			// pgx's own prefix for rs (cannot parse `…`: ), so the whole error is compared and
			// a value that ends in "`: invalid port" can't pass for that reason.
			prefix := pgconn.NewParseConfigError(rs, "", nil).Error()
			for _, r := range safeReasons {
				if rerr.Error() == prefix+r {
					return nil, fmt.Errorf("failed to parse connection string %q: %s", dsn.Redact(connStr), r)
				}
			}
		}
		return nil, fmt.Errorf("failed to parse connection string %q: percent-encode the special characters in its user name and password, or correct a parameter after the password", dsn.Redact(connStr))
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
