package mysql_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meru143/dbdiff/internal/db"
	_ "github.com/meru143/dbdiff/internal/db/mysql"
	"github.com/meru143/dbdiff/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMySQLIntrospectionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Setup MySQL Testcontainer
	// We use the official MySQL 8.0 image
	req := testcontainers.ContainerRequest{
		Image:        "mysql:8.0",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "rootpassword",
			"MYSQL_DATABASE":      "testdb",
			"MYSQL_USER":          "testuser",
			"MYSQL_PASSWORD":      "testpassword",
		},
		WaitingFor: wait.ForLog("port: 3306  MySQL Community Server").WithStartupTimeout(90 * time.Second),
	}

	mysqlC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	defer func() {
		if err := mysqlC.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}()

	host, err := mysqlC.Host(ctx)
	require.NoError(t, err)
	port, err := mysqlC.MappedPort(ctx, "3306")
	require.NoError(t, err)

	// DSN format: user:password@tcp(host:port)/dbname?parseTime=true
	dsn := fmt.Sprintf("testuser:testpassword@tcp(%s:%s)/testdb?parseTime=true&multiStatements=true", host, port.Port())
	fullUrl := "mysql://" + dsn

	// 2. Initialize dbdiff Driver
	driver, err := db.NewDriver(fullUrl)
	require.NoError(t, err)

	err = driver.Connect(ctx, fullUrl, "disable", 20*time.Second)
	require.NoError(t, err)
	defer driver.Close()

	// 3. Setup dummy schema
	setupSQL := `
	CREATE TABLE products (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		price DECIMAL(10,2) NOT NULL,
		available BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX idx_products_name ON products(name);

	CREATE VIEW v_active_products AS
	SELECT id, name, price FROM products WHERE available = TRUE;
	`
	err = driver.Exec(ctx, setupSQL)
	require.NoError(t, err)

	// 4. Test Introspection
	// MySQL Introspect uses current DB if schema is empty
	schemaObj, err := driver.Introspect(ctx, "testdb", []string{})
	require.NoError(t, err)

	// 5. Verify parsed schema
	assert.NotNil(t, schemaObj)

	var productsTable *types.Table
	for i := range schemaObj.Tables {
		if schemaObj.Tables[i].Name == "products" {
			productsTable = &schemaObj.Tables[i]
			break
		}
	}
	require.NotNil(t, productsTable, "Expected 'products' table to be found")
	assert.Equal(t, "products", productsTable.Name)

	// Verify columns
	colNames := []string{}
	for _, c := range productsTable.Columns {
		colNames = append(colNames, c.Name)
	}
	assert.Contains(t, colNames, "id")
	assert.Contains(t, colNames, "name")
	assert.Contains(t, colNames, "price")
	assert.Contains(t, colNames, "available")
	assert.Contains(t, colNames, "created_at")

	// Verify normalization via DiffEngine logic (or just direct NormalizeType)
	for _, c := range productsTable.Columns {
		if c.Name == "id" {
			assert.Equal(t, "integer", driver.NormalizeType(c.DataType))
		}
		if c.Name == "name" {
			assert.Equal(t, "varchar", driver.NormalizeType(c.DataType))
		}
		if c.Name == "price" {
			assert.Equal(t, "numeric", driver.NormalizeType(c.DataType))
		}
	}

	// Verify view
	assert.True(t, len(schemaObj.Views) > 0, "Expected at least one view")
	foundView := false
	for _, v := range schemaObj.Views {
		if v.Name == "v_active_products" {
			foundView = true
			break
		}
	}
	assert.True(t, foundView, "Expected v_active_products view to exist")
}
