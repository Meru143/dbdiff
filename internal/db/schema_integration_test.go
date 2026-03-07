package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSchemaIntrospectionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Setup Postgres Testcontainer
	pgContainer, err := postgres.Run(ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	// Clean up the container after the test
	defer func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}()

	// Grab connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// 2. Initialize dbdiff Driver
	driver, err := db.NewDriver(connStr)
	require.NoError(t, err)

	err = driver.Connect(ctx, connStr, "disable", 5*time.Second)
	require.NoError(t, err)
	defer driver.Close()

	// 3. Setup dummy schema
	setupSQL := `
	CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX idx_users_username ON users(username);
	`
	err = driver.Exec(ctx, setupSQL)
	require.NoError(t, err)

	// 4. Test Introspection
	schemaObj, err := driver.Introspect(ctx, "public", []string{})
	require.NoError(t, err)

	// 5. Verify parsed schema
	assert.NotNil(t, schemaObj)

	// Verify "users" table
	var usersTable *types.Table
	for i := range schemaObj.Tables {
		if schemaObj.Tables[i].Name == "users" {
			usersTable = &schemaObj.Tables[i]
			break
		}
	}
	require.NotNil(t, usersTable, "Expected 'users' table to be found in introspected schema.")
	assert.Equal(t, "users", usersTable.Name)

	// Verify "users" columns
	columnNames := []string{}
	for _, c := range usersTable.Columns {
		columnNames = append(columnNames, c.Name)
	}
	assert.Contains(t, columnNames, "id")
	assert.Contains(t, columnNames, "username")
	assert.Contains(t, columnNames, "email")
	assert.Contains(t, columnNames, "created_at")

	// Verify "users" indices
	assert.True(t, len(usersTable.Indexes) > 0, "Expected generated indexes on 'users' table")

	hasUsernameIdx := false
	for _, idx := range usersTable.Indexes {
		if idx.Name == "idx_users_username" {
			hasUsernameIdx = true
		}
	}
	assert.True(t, hasUsernameIdx, "Expected idx_users_username to exist")
}
