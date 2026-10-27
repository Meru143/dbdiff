package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/diff"
	"github.com/meru143/dbdiff/internal/output"
	"github.com/meru143/dbdiff/pkg/types"
	"github.com/stretchr/testify/require"
)

// MigrationRoundTrip generates the migration from target to source the way `dbdiff migrate`
// does by default (the target's dialect, the default schema, definitions from the source
// schema, one transaction), applies it to target the way `dbdiff apply` does, recording it
// in the migrations table, and returns it with the differences left afterwards: none when
// the migration is complete and correct and the migrations table stays out of the schema.
func MigrationRoundTrip(ctx context.Context, source, target db.Driver) (string, types.DiffList, error) {
	const schema = "public" // the CLI's default; MySQL and SQL Server map it to their own
	src, err := source.Introspect(ctx, schema, nil)
	if err != nil {
		return "", nil, fmt.Errorf("introspect source: %w", err)
	}
	tgt, err := target.Introspect(ctx, schema, nil)
	if err != nil {
		return "", nil, fmt.Errorf("introspect target: %w", err)
	}
	diffs := diff.Compare(src, tgt)
	if len(diffs) == 0 {
		return "", nil, fmt.Errorf("source and target are already the same")
	}

	formatter := output.NewFormatterWithOptions(output.FormatOptions{
		Format: "sql", Dialect: target.Dialect(), Schema: schema, SourceSchema: src,
	})
	migration, err := formatter.FormatMigration(diffs, true)
	if err != nil {
		return "", nil, err
	}
	if err := target.EnsureMigrationsTable(ctx); err != nil {
		return migration, nil, fmt.Errorf("create migrations table: %w", err)
	}
	if err := target.Exec(ctx, migration); err != nil {
		return migration, nil, fmt.Errorf("run migration: %w", err)
	}
	if err := target.RecordMigration(ctx, "migration.sql"); err != nil {
		return migration, nil, fmt.Errorf("record migration: %w", err)
	}

	after, err := target.Introspect(ctx, schema, nil)
	if err != nil {
		return migration, nil, fmt.Errorf("introspect migrated target: %w", err)
	}
	return migration, diff.Compare(src, after), nil
}

// RequireMigrationRoundTrip loads sourceDDL and targetDDL into two empty databases and
// checks that the migration dbdiff generates from target to source runs and leaves no
// differences.
func RequireMigrationRoundTrip(ctx context.Context, t *testing.T, source, target db.Driver, sourceDDL, targetDDL string) {
	t.Helper()
	require.NoError(t, source.Exec(ctx, sourceDDL), "source schema")
	require.NoError(t, target.Exec(ctx, targetDDL), "target schema")
	migration, remaining, err := MigrationRoundTrip(ctx, source, target)
	t.Logf("%s migration:\n%s", target.Dialect(), migration)
	require.NoError(t, err)
	require.Empty(t, remaining, "differences left after the migration")
}
