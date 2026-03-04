package db

import (
	"fmt"
	"strings"

	"github.com/meru143/dbdiff/internal/db/postgres"
)

// NewDriver creates a Driver instance based on the connection string scheme.
// Supported schemes:
//   - postgres://, postgresql:// → PostgreSQL driver
//   - mysql://, mysql+tcp://     → MySQL driver (planned)
//   - sqlserver://               → SQL Server driver (planned)
func NewDriver(connStr string) (Driver, error) {
	lower := strings.ToLower(connStr)

	switch {
	case strings.HasPrefix(lower, "postgres://"),
		strings.HasPrefix(lower, "postgresql://"):
		return postgres.New(), nil

	case strings.HasPrefix(lower, "mysql://"),
		strings.HasPrefix(lower, "mysql+tcp://"):
		return nil, fmt.Errorf("mysql support is not yet implemented — coming soon")

	case strings.HasPrefix(lower, "sqlserver://"):
		return nil, fmt.Errorf("sql server support is not yet implemented — coming soon")

	default:
		// Default to PostgreSQL for backwards compatibility with raw host:port strings
		return postgres.New(), nil
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
