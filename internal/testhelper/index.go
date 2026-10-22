package test

import (
	"context"
	"strings"
	"testing"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/diff"
	"github.com/meru143/dbdiff/pkg/types"
	"github.com/stretchr/testify/require"
)

// RoundTripIndex drops an index on a live database and recreates it using only the index
// DDL dbdiff generates, then checks the server reports exactly the original indexes again.
// It runs once with the plain DDL a transaction-wrapped migration gets, and once without
// a transaction, where PostgreSQL uses CONCURRENTLY.
func RoundTripIndex(ctx context.Context, t *testing.T, driver db.Driver, schema, table, index string) {
	t.Helper()
	want := introspectTable(ctx, t, driver, schema, table)
	without := want
	without.Indexes = nil
	for _, idx := range want.Indexes {
		if idx.Name != index {
			without.Indexes = append(without.Indexes, idx)
		}
	}
	require.Len(t, without.Indexes, len(want.Indexes)-1, "index %s not found on %s", index, table)

	for _, concurrently := range []bool{false, true} {
		runIndexDDL(ctx, t, driver, without, want, concurrently)
		got := introspectTable(ctx, t, driver, schema, table)
		require.Equal(t, without.Indexes, got.Indexes, "after DROP INDEX (concurrently=%v)", concurrently)

		runIndexDDL(ctx, t, driver, want, got, concurrently)
		got = introspectTable(ctx, t, driver, schema, table)
		require.Equal(t, want.Indexes, got.Indexes, "after CREATE INDEX (concurrently=%v)", concurrently)
	}
}

// runIndexDDL generates the migration from target to source, which must be a single index
// change, and runs its statements one at a time: PostgreSQL rejects CONCURRENTLY inside a
// multi-statement batch, and the session preamble is not under test here.
func runIndexDDL(ctx context.Context, t *testing.T, driver db.Driver, source, target types.Table, concurrently bool) {
	t.Helper()
	diffs := diff.Compare(&types.Schema{Tables: []types.Table{source}}, &types.Schema{Tables: []types.Table{target}})
	require.Len(t, diffs, 1)
	gen := diff.NewSQLGenerator(diffs, "", driver.Dialect())
	gen.SetTransaction(false)
	gen.SetConcurrently(concurrently)
	ran := 0
	for _, stmt := range strings.Split(gen.Generate(), "\n") {
		if strings.HasPrefix(stmt, "CREATE ") || strings.HasPrefix(stmt, "DROP ") {
			require.NoError(t, driver.Exec(ctx, stmt), stmt)
			ran++
		}
	}
	require.Equal(t, 1, ran, "expected one index statement for %v", diffs[0])
}

func introspectTable(ctx context.Context, t *testing.T, driver db.Driver, schema, table string) types.Table {
	t.Helper()
	s, err := driver.Introspect(ctx, schema, nil)
	require.NoError(t, err)
	for _, tb := range s.Tables {
		if tb.Name == table {
			return tb
		}
	}
	t.Fatalf("table %s not found in schema %s", table, schema)
	return types.Table{}
}
