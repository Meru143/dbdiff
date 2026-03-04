package mysql

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestConnect(t *testing.T) {
	// Since Connect relies heavily on sql.Open natively, we'll verify the DSN parsing
	// by checking if it flags obviously malformed URLs.
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
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS _dbdiff_migrations")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = d.EnsureMigrationsTable(ctx)
	if err != nil {
		t.Errorf("error was not expected while ensuring migrations table: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetAppliedMigrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"version"}).
		AddRow("001_init.sql").
		AddRow("002_add_users.sql")

	mock.ExpectQuery(regexp.QuoteMeta("SELECT version FROM _dbdiff_migrations")).
		WillReturnRows(rows)

	versions, err := d.GetAppliedMigrations(ctx)
	if err != nil {
		t.Errorf("error was not expected while getting applied migrations: %s", err)
	}

	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
	if versions[0] != "001_init.sql" {
		t.Errorf("expected first version to be 001_init.sql, got %s", versions[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestRecordMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("INSERT IGNORE INTO _dbdiff_migrations")).
		WithArgs("003_test.sql").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = d.RecordMigration(ctx, "003_test.sql")
	if err != nil {
		t.Errorf("error was not expected while recording migration: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestRemoveMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM _dbdiff_migrations")).
		WithArgs("003_test.sql").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = d.RemoveMigration(ctx, "003_test.sql")
	if err != nil {
		t.Errorf("error was not expected while removing migration: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestExec(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE test_table")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = d.Exec(ctx, "CREATE TABLE test_table;")
	if err != nil {
		t.Errorf("error was not expected while executing query: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestClose(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	d := &Driver{db: db}
	mock.ExpectClose()

	d.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestIntrospectMocked(t *testing.T) {
	// This performs a full introspection mocking every query.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %s", err)
	}
	defer db.Close()

	d := &Driver{db: db}
	ctx := context.Background()
	schemaName := "test_schema"

	// Mock getTables (ListTables now takes schema as param)
	tableRows := sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("users")
	mock.ExpectQuery(`SELECT table_name FROM information_schema\.tables`).
		WithArgs(schemaName).
		WillReturnRows(tableRows)

	// Mock getColumns
	colRows := sqlmock.NewRows([]string{
		"COLUMN_NAME", "DATA_TYPE", "COLUMN_DEFAULT", "IS_NULLABLE", "is_primary_key",
	}).AddRow("id", "int", nil, "NO", 1).
		AddRow("email", "varchar", nil, "YES", 0)

	mock.ExpectQuery(`SELECT c\.COLUMN_NAME, c\.DATA_TYPE`).
		WithArgs(schemaName, "users").
		WillReturnRows(colRows)

	// Mock getIndexes
	idxRows := sqlmock.NewRows([]string{"INDEX_NAME", "is_unique", "is_primary", "COLUMN_NAME"}).
		AddRow("idx_email", false, false, "email")
	mock.ExpectQuery(`SELECT INDEX_NAME, NON_UNIQUE \= 0 AS is_unique, INDEX_NAME \= 'PRIMARY' AS is_primary, COLUMN_NAME`).
		WithArgs(schemaName, "users").
		WillReturnRows(idxRows)

	// Mock getConstraints
	cstRows := sqlmock.NewRows([]string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "COLUMN_NAME"}).
		AddRow("chk_age", "CHECK", "age")
	mock.ExpectQuery(`SELECT tc\.CONSTRAINT_NAME, tc\.CONSTRAINT_TYPE, kcu\.COLUMN_NAME`).
		WithArgs(schemaName, "users").
		WillReturnRows(cstRows)

	// Mock getForeignKeys
	fkRows := sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME"}).
		AddRow("fk_user", "user_id", "users", "id")
	mock.ExpectQuery(`SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME`).
		WithArgs(schemaName, "users").
		WillReturnRows(fkRows)

	mock.ExpectQuery(`SELECT UPDATE_RULE, DELETE_RULE`).
		WithArgs(schemaName, "fk_user").
		WillReturnRows(sqlmock.NewRows([]string{"UPDATE_RULE", "DELETE_RULE"}).AddRow("RESTRICT", "CASCADE"))

	// Mock getViews
	viewRows := sqlmock.NewRows([]string{"TABLE_NAME", "VIEW_DEFINITION"}).AddRow("v_users", "SELECT * FROM users")
	mock.ExpectQuery(`SELECT TABLE_NAME, VIEW_DEFINITION FROM information_schema.VIEWS`).
		WithArgs(schemaName).
		WillReturnRows(viewRows)

	schema, err := d.Introspect(ctx, schemaName, nil)
	if err != nil {
		t.Fatalf("Introspect failed: %v", err)
	}

	if len(schema.Tables) != 1 {
		t.Fatalf("Expected 1 table, got %d", len(schema.Tables))
	}

	if schema.Tables[0].Name != "users" {
		t.Errorf("Expected table name 'users', got '%s'", schema.Tables[0].Name)
	}

	if len(schema.Tables[0].Columns) != 2 {
		t.Fatalf("Expected 2 columns, got %d", len(schema.Tables[0].Columns))
	}

	if schema.Tables[0].Columns[0].Name != "id" {
		t.Errorf("Expected first column to be 'id', got '%s'", schema.Tables[0].Columns[0].Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
