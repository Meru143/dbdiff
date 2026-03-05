package mssql

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestConnect(t *testing.T) {
	d := New()
	ctx := context.Background()
	err := d.Connect(ctx, "invalid-url", "disable", 1*time.Second)
	if err == nil {
		t.Error("Expected error connecting to invalid URL")
	}
}

func TestEnsureMigrationsTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	mock.ExpectExec(`IF NOT EXISTS`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = d.EnsureMigrationsTable(ctx)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestGetAppliedMigrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"version"}).
		AddRow("001_init.sql").
		AddRow("002_add_users.sql")

	mock.ExpectQuery(`SELECT version FROM _dbdiff_migrations`).
		WillReturnRows(rows)

	versions, err := d.GetAppliedMigrations(ctx)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestRecordMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	mock.ExpectExec(`INSERT INTO _dbdiff_migrations`).
		WithArgs("003_test.sql").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = d.RecordMigration(ctx, "003_test.sql")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestRemoveMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	mock.ExpectExec(`DELETE FROM _dbdiff_migrations`).
		WithArgs("003_test.sql").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = d.RemoveMigration(ctx, "003_test.sql")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestExec(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	mock.ExpectExec(`CREATE TABLE test_table`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = d.Exec(ctx, "CREATE TABLE test_table;")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

func TestClose(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}

	d := &Driver{db: db}
	mock.ExpectClose()
	d.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestNameAndDialect(t *testing.T) {
	d := New()
	if d.Name() != "sqlserver" {
		t.Errorf("expected name 'sqlserver', got '%s'", d.Name())
	}
	if d.Dialect() != "sqlserver" {
		t.Errorf("expected dialect 'sqlserver', got '%s'", d.Dialect())
	}
}

func TestGetServerInfo(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"DB_NAME()", "SUSER_SNAME()", "@@VERSION"}).
		AddRow("testdb", "sa", "Microsoft SQL Server 2022")

	mock.ExpectQuery(`SELECT DB_NAME`).
		WillReturnRows(rows)

	info, err := d.GetServerInfo(ctx)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if info.Database != "testdb" {
		t.Errorf("expected database 'testdb', got '%s'", info.Database)
	}
}

func TestIntrospectMocked(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()
	schema := "dbo"

	// Mock ListTables
	tableRows := sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("users")
	mock.ExpectQuery(`SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES`).
		WithArgs(schema).
		WillReturnRows(tableRows)

	// Mock getColumns
	colRows := sqlmock.NewRows([]string{
		"COLUMN_NAME", "DATA_TYPE", "COLUMN_DEFAULT", "IS_NULLABLE", "is_primary_key",
	}).AddRow("id", "int", nil, "NO", 1).
		AddRow("email", "nvarchar", nil, "YES", 0)
	mock.ExpectQuery(`SELECT`).
		WithArgs(schema, "users").
		WillReturnRows(colRows)

	// Mock getIndexes
	idxRows := sqlmock.NewRows([]string{"INDEX_NAME", "is_unique", "is_primary_key", "COLUMN_NAME"}).
		AddRow("IX_users_email", false, false, "email")
	mock.ExpectQuery(`SELECT`).
		WithArgs(schema, "users").
		WillReturnRows(idxRows)

	// Mock getConstraints
	cstRows := sqlmock.NewRows([]string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "COLUMN_NAME"}).
		AddRow("PK_users", "PRIMARY KEY", "id")
	mock.ExpectQuery(`SELECT`).
		WithArgs(schema, "users").
		WillReturnRows(cstRows)

	// Mock getForeignKeys
	fkRows := sqlmock.NewRows([]string{
		"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE",
		"REFERENCED_COLUMN", "update_action", "delete_action",
	})
	mock.ExpectQuery(`SELECT`).
		WithArgs(schema, "users").
		WillReturnRows(fkRows)

	// Mock getViews
	viewRows := sqlmock.NewRows([]string{"TABLE_NAME", "VIEW_DEFINITION"})
	mock.ExpectQuery(`SELECT TABLE_NAME, VIEW_DEFINITION`).
		WithArgs(schema).
		WillReturnRows(viewRows)

	result, err := d.Introspect(ctx, schema, nil)
	if err != nil {
		t.Fatalf("Introspect failed: %v", err)
	}

	if len(result.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(result.Tables))
	}
	if result.Tables[0].Name != "users" {
		t.Errorf("expected table 'users', got '%s'", result.Tables[0].Name)
	}
	if len(result.Tables[0].Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(result.Tables[0].Columns))
	}
}

func TestListTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"TABLE_NAME"}).
		AddRow("users").
		AddRow("sysdiagrams"). // should be ignored
		AddRow("posts")

	mock.ExpectQuery(`SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES`).
		WithArgs("dbo").
		WillReturnRows(rows)

	tables, err := d.ListTables(ctx, "dbo", DefaultIgnorePatterns)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d: %v", len(tables), tables)
	}
}
