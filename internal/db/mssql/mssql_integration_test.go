package mssql_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/meru143/dbdiff/internal/db"
	_ "github.com/meru143/dbdiff/internal/db/mssql"
	testhelper "github.com/meru143/dbdiff/internal/testhelper"
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
		WaitingFor: wait.ForSQL("1433/tcp", "sqlserver", func(host string, port nat.Port) string {
			return fmt.Sprintf("sqlserver://sa:StrongPassword123!@%s:%s?database=master&encrypt=disable", host, port.Port())
		}).WithStartupTimeout(120 * time.Second),
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

	// Types keep their length and precision, which DDL needs; the normalizer maps names only.
	for _, c := range employeesTable.Columns {
		if c.Name == "id" {
			assert.Equal(t, "integer", driverWithDB.NormalizeType(c.DataType))
			assert.Equal(t, "IDENTITY(1,1)", c.Extra)
		}
		if c.Name == "name" {
			assert.Equal(t, "nvarchar(100)", c.DataType)
		}
		if c.Name == "salary" {
			assert.Equal(t, "decimal(18,2)", c.DataType)
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

	// Generated index DDL runs on the server and recreates the index exactly
	testhelper.RoundTripIndex(ctx, t, driverWithDB, "dbo", "employees", "idx_employees_name")

	// 7. The migration dbdiff generates from one database to another runs there and
	// leaves no differences.
	require.NoError(t, driver.Exec(ctx, "CREATE DATABASE src; CREATE DATABASE tgt"))
	connect := func(name string) db.Driver {
		url := fmt.Sprintf("sqlserver://sa:StrongPassword123!@%s:%s?database=%s&encrypt=disable", host, port.Port(), name)
		d, err := db.NewDriver(url)
		require.NoError(t, err)
		require.NoError(t, d.Connect(ctx, url, "disable", 15*time.Second))
		t.Cleanup(d.Close)
		return d
	}
	testhelper.RequireMigrationRoundTrip(ctx, t, connect("src"), connect("tgt"), mssqlSource, mssqlTarget)
}

// mssqlSource and mssqlTarget differ by a new table with an identity key, a check constraint
// and defaults; another whose foreign key points at an existing table; a column that gains
// NOT NULL under an index whose columns change (SQL Server refuses to alter an indexed
// column), one that widens to bigint, one added with a default; a column dropped while it
// still has an index and a DEFAULT constraint; a column, its foreign key and the table it
// points at dropped; and a dropped table.
// ponytail: keys are named; SQL Server names unnamed ones PK__users__<random>, which differs
// between databases.
const mssqlSource = `
	CREATE TABLE teams (
		id int IDENTITY(1,1) CONSTRAINT pk_teams PRIMARY KEY,
		name nvarchar(100) NOT NULL,
		status nvarchar(20) DEFAULT 'active',
		created_at datetime2(3) DEFAULT SYSUTCDATETIME(),
		CONSTRAINT chk_teams_status CHECK (status <> '')
	);
	CREATE TABLE users (
		id int IDENTITY(1,1) CONSTRAINT pk_users PRIMARY KEY,
		email nvarchar(255) NOT NULL,
		name nvarchar(100) NOT NULL,
		age bigint,
		nickname nvarchar(50) DEFAULT 'none'
	);
	CREATE INDEX idx_users_name ON users (name, email);
	CREATE TABLE posts (
		id int IDENTITY(1,1) CONSTRAINT pk_posts PRIMARY KEY,
		user_id int NOT NULL,
		title nvarchar(200) NOT NULL,
		CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
	);`

const mssqlTarget = `
	CREATE TABLE legacy_teams (id int CONSTRAINT pk_legacy_teams PRIMARY KEY);
	CREATE TABLE users (
		id int IDENTITY(1,1) CONSTRAINT pk_users PRIMARY KEY,
		email nvarchar(255) NOT NULL,
		name nvarchar(100),
		age int,
		legacy nvarchar(10) DEFAULT 'x',
		legacy_team_id int,
		CONSTRAINT fk_users_legacy_team FOREIGN KEY (legacy_team_id) REFERENCES legacy_teams (id)
	);
	CREATE INDEX idx_users_name ON users (name);
	CREATE INDEX idx_users_legacy ON users (legacy);
	CREATE TABLE audit (id int CONSTRAINT pk_audit PRIMARY KEY);`
