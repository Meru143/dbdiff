package mssql_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meru143/dbdiff/internal/db"
	_ "github.com/meru143/dbdiff/internal/db/mssql"
	"github.com/meru143/dbdiff/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMSSQLIntrospectionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Setup SQL Server Testcontainer
	// We use the official SQL Server 2022 image
	req := testcontainers.ContainerRequest{
		Image:        "mcr.microsoft.com/mssql/server:2022-latest",
		ExposedPorts: []string{"1433/tcp"},
		Env: map[string]string{
			"ACCEPT_EULA":       "Y",
			"MSSQL_SA_PASSWORD": "StrongPassword123!",
		},
		WaitingFor: wait.ForLog("SQL Server is now ready for client connections.").WithStartupTimeout(120 * time.Second),
	}

	mssqlC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	defer func() {
		if err := mssqlC.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}()

	host, err := mssqlC.Host(ctx)
	require.NoError(t, err)
	port, err := mssqlC.MappedPort(ctx, "1433")
	require.NoError(t, err)

	// DSN format: sqlserver://sa:Password@host:port?database=master&encrypt=disable
	connStr := fmt.Sprintf("sqlserver://sa:StrongPassword123!@%s:%s?database=master&encrypt=disable", host, port.Port())

	// 2. Initialize dbdiff Driver
	driver, err := db.NewDriver(connStr)
	require.NoError(t, err)

	err = driver.Connect(ctx, connStr, "disable", 30*time.Second)
	require.NoError(t, err)
	defer driver.Close()

	// Create a test database and use it
	err = driver.Exec(ctx, "CREATE DATABASE testdb")
	require.NoError(t, err)

	// Reconnect to the new database
	connStrWithDB := fmt.Sprintf("sqlserver://sa:StrongPassword123!@%s:%s?database=testdb&encrypt=disable", host, port.Port())
	driverWithDB, err := db.NewDriver(connStrWithDB)
	require.NoError(t, err)
	err = driverWithDB.Connect(ctx, connStrWithDB, "disable", 15*time.Second)
	require.NoError(t, err)
	defer driverWithDB.Close()

	// 3. Setup dummy schema
	setupSQL := `
	CREATE TABLE employees (
		id INT IDENTITY(1,1) PRIMARY KEY,
		name NVARCHAR(100) NOT NULL,
		salary DECIMAL(18,2) NOT NULL,
		is_active BIT DEFAULT 1,
		hired_at DATETIME2 DEFAULT GETDATE()
	);

	CREATE INDEX idx_employees_name ON employees(name);

	EXEC('CREATE VIEW v_active_employees AS SELECT id, name FROM employees WHERE is_active = 1');
	`
	err = driverWithDB.Exec(ctx, setupSQL)
	require.NoError(t, err)

	// 4. Test Introspection
	schemaObj, err := driverWithDB.Introspect(ctx, "dbo", []string{})
	require.NoError(t, err)

	// 5. Verify parsed schema
	assert.NotNil(t, schemaObj)

	var employeesTable *types.Table
	for i := range schemaObj.Tables {
		if schemaObj.Tables[i].Name == "employees" {
			employeesTable = &schemaObj.Tables[i]
			break
		}
	}
	require.NotNil(t, employeesTable, "Expected 'employees' table to be found")
	assert.Equal(t, "employees", employeesTable.Name)

	// Verify columns
	colNames := []string{}
	for _, c := range employeesTable.Columns {
		colNames = append(colNames, c.Name)
	}
	assert.Contains(t, colNames, "id")
	assert.Contains(t, colNames, "name")
	assert.Contains(t, colNames, "salary")
	assert.Contains(t, colNames, "is_active")
	assert.Contains(t, colNames, "hired_at")

	// Verify normalization
	for _, c := range employeesTable.Columns {
		normalized := driverWithDB.NormalizeType(c.DataType)
		if c.Name == "id" {
			assert.Equal(t, "integer", normalized)
		}
		if c.Name == "name" {
			assert.Equal(t, "varchar", normalized)
		}
		if c.Name == "salary" {
			assert.Equal(t, "numeric", normalized)
		}
	}

	// Verify view
	assert.True(t, len(schemaObj.Views) > 0, "Expected at least one view")
	foundView := false
	for _, v := range schemaObj.Views {
		if v.Name == "v_active_employees" {
			foundView = true
			break
		}
	}
	assert.True(t, foundView, "Expected v_active_employees view to exist")
}
