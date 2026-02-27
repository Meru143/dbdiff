package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxRetries     = 3
	baseRetryDelay = 500 * time.Millisecond
)

type DB struct {
	*pgxpool.Pool
}

// Connect establishes a connection to the database with retry logic
func Connect(ctx context.Context, connStr, sslMode string, timeout time.Duration) (*DB, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := baseRetryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		db, err := connect(ctx, connStr, sslMode, timeout)
		if err == nil {
			return db, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, lastErr)
}

// connect performs a single connection attempt
func connect(ctx context.Context, connStr, sslMode string, timeout time.Duration) (*DB, error) {
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

	// Create context with query timeout
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
