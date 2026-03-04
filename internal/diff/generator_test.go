package diff

import (
	"strings"
	"testing"

	"github.com/meru143/dbdiff/pkg/types"
)

func TestSQLGenerator_CreateTable(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "CREATE TABLE") {
		t.Errorf("Expected CREATE TABLE, got: %s", sql)
	}
}

func TestSQLGenerator_DropTable(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffDrop, Object: types.ObjectTable, Name: "users"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "DROP TABLE") {
		t.Errorf("Expected DROP TABLE, got: %s", sql)
	}
}

func TestSQLGenerator_AddColumn(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectColumn, Name: "email", TableName: "users", NewValue: "varchar"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "ADD COLUMN") {
		t.Errorf("Expected ADD COLUMN, got: %s", sql)
	}
}

func TestSQLGenerator_DropColumn(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffDrop, Object: types.ObjectColumn, Name: "email", TableName: "users"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "DROP COLUMN") {
		t.Errorf("Expected DROP COLUMN, got: %s", sql)
	}
}

func TestSQLGenerator_AlterColumn(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAlter, Object: types.ObjectColumn, Name: "email", TableName: "users", OldValue: "varchar", NewValue: "text"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "ALTER COLUMN") {
		t.Errorf("Expected ALTER COLUMN, got: %s", sql)
	}
}

func TestSQLGenerator_CreateIndex(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectIndex, Name: "users_email_idx", TableName: "users"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "CREATE INDEX") {
		t.Errorf("Expected CREATE INDEX, got: %s", sql)
	}
}

func TestSQLGenerator_DropIndex(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffDrop, Object: types.ObjectIndex, Name: "users_email_idx"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "DROP INDEX") {
		t.Errorf("Expected DROP INDEX, got: %s", sql)
	}
}

func TestSQLGenerator_AddConstraint(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectConstraint, Name: "users_pkey", TableName: "users"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "ADD CONSTRAINT") {
		t.Errorf("Expected ADD CONSTRAINT, got: %s", sql)
	}
}

func TestSQLGenerator_DropConstraint(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffDrop, Object: types.ObjectConstraint, Name: "users_pkey", TableName: "users"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "DROP CONSTRAINT") {
		t.Errorf("Expected DROP CONSTRAINT, got: %s", sql)
	}
}

func TestSQLGenerator_CreateSequence(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectSequence, Name: "users_id_seq"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "CREATE SEQUENCE") {
		t.Errorf("Expected CREATE SEQUENCE, got: %s", sql)
	}
}

func TestSQLGenerator_TransactionWrapper(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(true)
	sql := gen.Generate()

	if !strings.Contains(sql, "DO $$") {
		t.Errorf("Expected transaction wrapper, got: %s", sql)
	}
}

func TestSQLGenerator_EmptyDiffs(t *testing.T) {
	diffs := types.DiffList{}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if strings.Contains(sql, "CREATE") || strings.Contains(sql, "DROP") {
		t.Errorf("Expected empty output for empty diffs, got: %s", sql)
	}
}

func TestSQLGenerator_MultipleDiffs(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
		{Type: types.DiffAdd, Object: types.ObjectTable, Name: "posts"},
		{Type: types.DiffDrop, Object: types.ObjectTable, Name: "old_table"},
	}
	gen := NewSQLGenerator(diffs, "public", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "users") || !strings.Contains(sql, "posts") || !strings.Contains(sql, "old_table") {
		t.Errorf("Expected all table names in output, got: %s", sql)
	}
}

func TestSQLGenerator_View(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectView, Name: "active_users", NewValue: "SELECT * FROM users;"},
	}
	gen := NewSQLGenerator(diffs, "", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "CREATE OR REPLACE VIEW active_users") {
		t.Errorf("Invalid view format: %s", sql)
	}
}

func TestSQLGenerator_MaterializedView(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffDrop, Object: types.ObjectMaterializedView, Name: "daily_stats"},
	}
	gen := NewSQLGenerator(diffs, "", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "DROP MATERIALIZED VIEW IF EXISTS daily_stats CASCADE") {
		t.Errorf("Invalid materialized view format: %s", sql)
	}
}

func TestSQLGenerator_Function(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectFunction, Name: "get_user(integer)", NewValue: "CREATE OR REPLACE FUNCTION get_user(integer) RETURNS void;"},
	}
	gen := NewSQLGenerator(diffs, "", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "CREATE OR REPLACE FUNCTION get_user(integer) RETURNS void;") {
		t.Errorf("Invalid function format: %s", sql)
	}
}

func TestSQLGenerator_Trigger(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAlter, Object: types.ObjectTrigger, Name: "update_timestamp", TableName: "users", NewValue: "CREATE TRIGGER update_timestamp BEFORE UPDATE..."},
	}
	gen := NewSQLGenerator(diffs, "", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "DROP TRIGGER IF EXISTS update_timestamp ON users CASCADE") {
		t.Errorf("Expected alter trigger to invoke drop: %s", sql)
	}
	if !strings.Contains(sql, "CREATE TRIGGER update_timestamp") {
		t.Errorf("Expected alter trigger to invoke create after drop: %s", sql)
	}
}

func TestSQLGenerator_Grant(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAdd, Object: types.ObjectGrant, Name: "SELECT", TableName: "users", NewValue: "api_role", OldValue: "true"},
	}
	gen := NewSQLGenerator(diffs, "", "postgresql")
	gen.SetTransaction(false)
	sql := gen.Generate()

	if !strings.Contains(sql, "GRANT SELECT ON TABLE users TO api_role WITH GRANT OPTION") {
		t.Errorf("Invalid grant format: %s", sql)
	}
}
