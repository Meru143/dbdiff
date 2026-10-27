package db

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/meru143/dbdiff/internal/db/mssql"
	"github.com/meru143/dbdiff/internal/db/mysql"
	"github.com/meru143/dbdiff/internal/db/postgres"
	"github.com/meru143/dbdiff/pkg/types"
)

// MigrationsTable is the table `dbdiff apply` creates in the target to record the
// migrations it ran. It is dbdiff's history, not part of the user's schema.
const MigrationsTable = "_dbdiff_migrations"

// NewDriver creates a Driver instance based on the connection string scheme.
// Supported schemes:
//   - postgres://, postgresql:// → PostgreSQL driver
//   - mysql://, mysql+tcp://     → MySQL driver
//   - sqlserver://               → SQL Server driver
func NewDriver(connStr string) (Driver, error) {
	lower := strings.ToLower(connStr)

	switch {
	case strings.HasPrefix(lower, "postgres://"),
		strings.HasPrefix(lower, "postgresql://"):
		return withoutMigrationsTable{postgres.New()}, nil

	case strings.HasPrefix(lower, "mysql://"),
		strings.HasPrefix(lower, "mysql+tcp://"):
		return withoutMigrationsTable{mysql.New()}, nil

	case strings.HasPrefix(lower, "sqlserver://"):
		return withoutMigrationsTable{mssql.New()}, nil

	default:
		return nil, fmt.Errorf("unsupported database scheme or missing prefix in connection string: %s", connStr)
	}
}

// withoutMigrationsTable leaves MigrationsTable and its grants out of the schemas a driver
// reads. Otherwise, once apply had run on a target, every comparison with a source that
// lacks the table would report it, and the next migration would drop it with the history.
type withoutMigrationsTable struct{ Driver }

func (d withoutMigrationsTable) Introspect(ctx context.Context, schema string, ignorePatterns []string) (*types.Schema, error) {
	s, err := d.Driver.Introspect(ctx, schema, ignorePatterns)
	if err != nil {
		return nil, err
	}
	s.Tables = slices.DeleteFunc(s.Tables, func(t types.Table) bool { return t.Name == MigrationsTable })
	s.Grants = slices.DeleteFunc(s.Grants, func(g types.Grant) bool { return g.Table == MigrationsTable })
	return s, nil
}
