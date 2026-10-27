package mssql

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/meru143/dbdiff/pkg/types"
)

func TestConnect(t *testing.T) {
	d := New()
	ctx := context.Background()
	err := d.Connect(ctx, "invalid-url", "disable", 1*time.Second)
	if err == nil {
		t.Error("Expected error connecting to invalid URL")
	}
}

// net/url's error for a URL it can't parse quotes the URL, and can quote part of the
// password; Connect's error holds neither.
func TestConnectParseErrorHidesPassword(t *testing.T) {
	for _, url := range []string{
		"sqlserver://sa:Sec/ret@localhost?database=app",   // url: invalid port ":Sec" after host
		"sqlserver://sa:Sec%zzret@localhost?database=app", // url: invalid URL escape "%zz"
		"sqlserver://sa:Sec ret@localhost?database=app",   // url: invalid userinfo
	} {
		err := New().Connect(context.Background(), url, "disable", time.Second)
		if err == nil || strings.Contains(err.Error(), "Sec") || strings.Contains(err.Error(), "ret@") ||
			!strings.Contains(err.Error(), "sqlserver://sa:***@localhost?database=app") {
			t.Errorf("%s: expected an error naming the URL without its password, got %v", url, err)
		}
	}
	err := New().Connect(context.Background(), "sqlserver://sa:secret@local host?database=app", "disable", time.Second)
	if err == nil || strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "invalid character") {
		t.Errorf("a host error should keep its reason, not the password, got %v", err)
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
		"COLUMN_NAME", "DATA_TYPE", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE",
		"DATETIME_PRECISION", "COLUMN_DEFAULT", "IS_NULLABLE", "is_primary_key", "identity",
	}).AddRow("id", "int", nil, 10, 0, nil, nil, "NO", 1, "IDENTITY(1,1)").
		AddRow("email", "nvarchar", 255, nil, nil, nil, nil, "YES", 0, "").
		AddRow("body", "varbinary", -1, nil, nil, nil, nil, "YES", 0, "").
		AddRow("price", "decimal", nil, 18, 2, nil, "((0))", "NO", 0, "").
		AddRow("seen_at", "datetime2", nil, nil, nil, 3, "(getdate())", "YES", 0, "")
	mock.ExpectQuery(`SELECT`).
		WithArgs(schema, "users").
		WillReturnRows(colRows)

	// Mock getIndexes
	idxRows := sqlmock.NewRows([]string{"INDEX_NAME", "is_unique", "is_unique_constraint", "type_desc",
		"filter_definition", "COLUMN_NAME", "is_descending_key", "is_included_column"}).
		AddRow("IX_users_email", false, false, "NONCLUSTERED", "([deleted_at] IS NULL)", "email", true, false).
		AddRow("IX_users_email", false, false, "NONCLUSTERED", "([deleted_at] IS NULL)", "price", false, false).
		AddRow("IX_users_email", false, false, "NONCLUSTERED", "([deleted_at] IS NULL)", "body", false, true).
		AddRow("UQ_users_email", true, true, "NONCLUSTERED", nil, "email", false, false)
	mock.ExpectQuery(`SELECT`).
		WithArgs(schema, "users").
		WillReturnRows(idxRows)

	// Mock getConstraints
	cstRows := sqlmock.NewRows([]string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "COLUMN_NAME", "definition"}).
		AddRow("PK_users", "PRIMARY KEY", "id", nil).
		AddRow("chk_price", "CHECK", nil, "([price]>=(0))")
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
	// Columns keep their length, precision and identity; constraints carry their definition.
	str := func(s string) *string { return &s }
	wantColumns := []types.Column{
		{Name: "id", DataType: "int", IsPrimaryKey: true, Extra: "IDENTITY(1,1)"},
		{Name: "email", DataType: "nvarchar(255)", IsNullable: true},
		{Name: "body", DataType: "varbinary(max)", IsNullable: true},
		{Name: "price", DataType: "decimal(18,2)", DefaultValue: str("((0))")},
		{Name: "seen_at", DataType: "datetime2(3)", DefaultValue: str("(getdate())"), IsNullable: true},
	}
	if !reflect.DeepEqual(result.Tables[0].Columns, wantColumns) {
		t.Errorf("columns:\n got %+v\nwant %+v", result.Tables[0].Columns, wantColumns)
	}
	// Key columns stay apart from INCLUDE columns, and the definition keeps both, the order
	// and the filter. The unique constraint's index is marked as the constraint's.
	wantIndexes := []types.Index{
		{Name: "IX_users_email", Columns: []string{"email", "price"},
			Definition: "CREATE INDEX IX_users_email ON dbo.users (email DESC, price) INCLUDE (body) WHERE ([deleted_at] IS NULL)"},
		{Name: "UQ_users_email", Columns: []string{"email"}, IsUnique: true, IsConstraint: true,
			Definition: "CREATE UNIQUE INDEX UQ_users_email ON dbo.users (email)"},
	}
	if !reflect.DeepEqual(result.Tables[0].Indexes, wantIndexes) {
		t.Errorf("indexes:\n got %+v\nwant %+v", result.Tables[0].Indexes, wantIndexes)
	}
	wantConstraints := []types.Constraint{
		{Name: "PK_users", Type: "PRIMARY KEY", Columns: []string{"id"}, Definition: "PRIMARY KEY (id)"},
		{Name: "chk_price", Type: "CHECK", Definition: "CHECK ([price]>=(0))"},
	}
	if !reflect.DeepEqual(result.Tables[0].Constraints, wantConstraints) {
		t.Errorf("constraints:\n got %+v\nwant %+v", result.Tables[0].Constraints, wantConstraints)
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

	tables, err := d.ListTables(ctx, "dbo", []string{"sysdiagrams"})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d: %v", len(tables), tables)
	}
}
