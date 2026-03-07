package db

import (
	"fmt"
	"strings"

	"github.com/meru143/dbdiff/internal/db/mssql"
	"github.com/meru143/dbdiff/internal/db/mysql"
	"github.com/meru143/dbdiff/internal/db/postgres"
)

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
		return postgres.New(), nil

	case strings.HasPrefix(lower, "mysql://"),
		strings.HasPrefix(lower, "mysql+tcp://"):
		return mysql.New(), nil

	case strings.HasPrefix(lower, "sqlserver://"):
		return mssql.New(), nil

	default:
		return nil, fmt.Errorf("unsupported database scheme or missing prefix in connection string: %s", connStr)
	}
}

// DetectDialect returns the dialect name from a connection string
func DetectDialect(connStr string) string {
	lower := strings.ToLower(connStr)
	switch {
	case strings.HasPrefix(lower, "mysql://"), strings.HasPrefix(lower, "mysql+tcp://"):
		return "mysql"
	case strings.HasPrefix(lower, "sqlserver://"):
		return "sqlserver"
	default:
		return "postgresql"
	}
}
