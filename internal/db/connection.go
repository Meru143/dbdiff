package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	*pgxpool.Pool
}

// Connect establishes a connection to the database
func Connect(ctx context.Context, connStr, sslMode string, timeout time.Duration) (*DB, error) {
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Apply SSL configuration
	if err := applySSLConfig(config, sslMode); err != nil {
		return nil, err
	}

	// Apply timeout settings
	if timeout > 0 {
		config.ConnConfig.ConnectTimeout = timeout
	}

	// Configure pool settings
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// applySSLConfig applies SSL configuration based on mode
func applySSLConfig(config *pgxpool.Config, sslMode string) error {
	switch sslMode {
	case "disable":
		config.ConnConfig.TLSConfig = nil
	case "require":
		// Default behavior - TLS required but no cert verification
		config.ConnConfig.TLSConfig.ServerName = config.ConnConfig.Host
	case "verify-ca":
		config.ConnConfig.TLSConfig.ServerName = config.ConnConfig.Host
	case "verify-full":
		config.ConnConfig.TLSConfig.ServerName = config.ConnConfig.Host
	default:
		return fmt.Errorf("unsupported SSL mode: %s", sslMode)
	}
	return nil
}

// GetPoolStats returns current pool statistics
func (db *DB) GetPoolStats() map[string]int {
	stats := make(map[string]int)
	// Note: pgxpool doesn't expose stats directly in v5
	// Would need to implement custom metrics collection
	return stats
}

// Close closes the connection pool
func (db *DB) Close() {
	db.Pool.Close()
}
