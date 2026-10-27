package mysql_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meru143/dbdiff/internal/db"
	_ "github.com/meru143/dbdiff/internal/db/mysql"
	testhelper "github.com/meru143/dbdiff/internal/testhelper"
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

	// Types keep their length and precision, which DDL needs; the normalizer maps names only.
	for _, c := range productsTable.Columns {
		if c.Name == "id" {
			assert.Equal(t, "integer", driver.NormalizeType(c.DataType))
			assert.Equal(t, "AUTO_INCREMENT", c.Extra)
		}
		if c.Name == "name" {
			assert.Equal(t, "varchar(100)", c.DataType)
		}
		if c.Name == "price" {
			assert.Equal(t, "decimal(10,2)", c.DataType)
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

	// Generated index DDL runs on the server and recreates the index exactly
	testhelper.RoundTripIndex(ctx, t, driver, "testdb", "products", "idx_products_name")

	// 7. The migration dbdiff generates from one database to another runs there and
	// leaves no differences. Only root may create the two databases.
	root := func(name string) db.Driver {
		url := fmt.Sprintf("mysql://root:rootpassword@tcp(%s:%s)/%s?parseTime=true", host, port.Port(), name)
		d, err := db.NewDriver(url)
		require.NoError(t, err)
		require.NoError(t, d.Connect(ctx, url, "disable", 20*time.Second))
		t.Cleanup(d.Close)
		return d
	}
	require.NoError(t, root("testdb").Exec(ctx, "CREATE DATABASE src; CREATE DATABASE tgt"))
	testhelper.RequireMigrationRoundTrip(ctx, t, root("src"), root("tgt"), mysqlSource, mysqlTarget)
}

// mysqlSource and mysqlTarget differ by:
//   - a new table with an auto-increment key, a check constraint and defaults;
//   - a new table whose foreign key points at an existing table, with a prefix-length index
//     on a TEXT column, a FULLTEXT and a functional index, and an explicit index the foreign
//     key uses (created first, so MySQL doesn't add its own);
//   - a unique key added, an index that becomes unique, and a check constraint whose
//     expression changes on a column that widens;
//   - a column that gains NOT NULL (MODIFY COLUMN must keep its varchar(100)) under an index
//     whose columns change, one widened to bigint, one added with a default, and an indexed
//     column dropped (MySQL drops the index with it, so DROP INDEX must come first);
//   - a column, its foreign key and the table it points at dropped, and a dropped table;
//   - a string default with a backslash, which a MySQL literal must double.
const mysqlSource = `
	CREATE TABLE teams (
		id int NOT NULL AUTO_INCREMENT PRIMARY KEY,
		name varchar(100) NOT NULL,
		status varchar(20) DEFAULT 'active',
		path varchar(50) DEFAULT 'C:\\temp',
		created_at timestamp DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		CONSTRAINT chk_teams_status CHECK (status <> '')
	);
	CREATE TABLE users (
		id int NOT NULL AUTO_INCREMENT PRIMARY KEY,
		email varchar(255) NOT NULL,
		name varchar(100) NOT NULL,
		handle varchar(50),
		age bigint,
		nickname varchar(50) DEFAULT 'none',
		UNIQUE KEY uq_users_email (email),
		CONSTRAINT chk_users_age CHECK (age >= 18)
	);
	CREATE INDEX idx_users_name ON users (name, email);
	CREATE UNIQUE INDEX idx_users_handle ON users (handle);
	CREATE TABLE posts (
		id int NOT NULL AUTO_INCREMENT PRIMARY KEY,
		user_id int NOT NULL,
		title varchar(200) NOT NULL,
		body text,
		KEY idx_posts_user (user_id),
		CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
	);
	CREATE INDEX idx_posts_body ON posts (body(20), title DESC);
	CREATE FULLTEXT INDEX ft_posts_body ON posts (body);
	CREATE INDEX idx_posts_lower_title ON posts ((lower(title)));`

const mysqlTarget = `
	CREATE TABLE legacy_teams (id int PRIMARY KEY);
	CREATE TABLE users (
		id int NOT NULL AUTO_INCREMENT PRIMARY KEY,
		email varchar(255) NOT NULL,
		name varchar(100),
		handle varchar(50),
		age int,
		legacy varchar(10) DEFAULT 'x',
		legacy_team_id int,
		CONSTRAINT chk_users_age CHECK (age >= 0),
		CONSTRAINT fk_users_legacy_team FOREIGN KEY (legacy_team_id) REFERENCES legacy_teams (id)
	);
	CREATE INDEX idx_users_name ON users (name);
	CREATE INDEX idx_users_handle ON users (handle);
	CREATE INDEX idx_users_legacy ON users (legacy);
	CREATE TABLE audit (id int PRIMARY KEY);`
