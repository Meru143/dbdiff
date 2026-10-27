package diff

import (
	"strings"
	"testing"

	"github.com/meru143/dbdiff/pkg/types"
)

// sourceFixture is the desired schema the generator takes definitions from.
func sourceFixture() *types.Schema {
	active := "'active'"
	return &types.Schema{
		Tables: []types.Table{{
			Name: "users",
			Columns: []types.Column{
				{Name: "id", DataType: "int", IsPrimaryKey: true},
				{Name: "org_id", DataType: "int", IsNullable: true},
				{Name: "name", DataType: "varchar(100)"},
				{Name: "status", DataType: "varchar(20)", DefaultValue: &active, IsNullable: true},
			},
			Constraints: []types.Constraint{
				{Name: "users_pkey", Type: "PRIMARY KEY", Columns: []string{"id"}, Definition: "PRIMARY KEY (id)"},
				{Name: "uq_users_name", Type: "UNIQUE", Columns: []string{"name"}},
				{Name: "chk_status", Type: "CHECK", Definition: "CHECK (status <> '')"},
			},
			ForeignKeys: []types.ForeignKey{{
				Name: "fk_users_org", Columns: []string{"org_id"}, RefTable: "orgs", RefColumns: []string{"id"},
				OnDelete: "CASCADE", OnUpdate: "NO ACTION",
			}},
		}},
		Sequences: []types.Sequence{{Name: "users_id_seq", Start: 1, Increment: 1, MinValue: 1, MaxValue: 2147483647, Cache: 1}},
		Types:     []types.Type{{Name: "mood", Kind: "enum", Values: []string{"happy", "it's ok"}}},
	}
}

func TestSQLGenerator_DialectStatements(t *testing.T) {
	column := func(typ types.DiffType, name, old, new, change string) types.Diff {
		return types.Diff{Type: typ, Object: types.ObjectColumn, Name: name, TableName: "users",
			OldValue: old, NewValue: new, Description: change}
	}
	object := func(typ types.DiffType, obj types.DiffObject, name string) types.Diff {
		return types.Diff{Type: typ, Object: obj, Name: name, TableName: "users"}
	}
	dropDefault := func(n, column string) string {
		return "DECLARE @drop_default_" + n + " nvarchar(max) = (SELECT N'ALTER TABLE dbo.users DROP CONSTRAINT ' + QUOTENAME(name)" +
			" FROM sys.default_constraints WHERE parent_object_id = OBJECT_ID(N'dbo.users')" +
			" AND parent_column_id = COLUMNPROPERTY(OBJECT_ID(N'dbo.users'), N'" + column + "', 'ColumnId'));\n" +
			"EXEC (@drop_default_" + n + ");"
	}

	tests := []struct {
		name     string
		dialect  string
		schema   string
		noSource bool
		diff     types.Diff
		want     string
	}{
		// CREATE TABLE and ADD COLUMN spell out the source definition.
		{
			name: "postgresql/create table from the source schema", dialect: "postgresql", schema: "public",
			diff: object(types.DiffAdd, types.ObjectTable, "users"),
			want: "-- Create table: users\nCREATE TABLE users (\n" +
				"  id int NOT NULL,\n  org_id int NULL,\n  name varchar(100) NOT NULL,\n  status varchar(20) DEFAULT 'active' NULL,\n" +
				"  CONSTRAINT users_pkey PRIMARY KEY (id),\n  CONSTRAINT uq_users_name UNIQUE (name),\n  CONSTRAINT chk_status CHECK (status <> ''),\n" +
				"  CONSTRAINT fk_users_org FOREIGN KEY (org_id) REFERENCES orgs (id) ON DELETE CASCADE\n);",
		},
		{
			name: "mysql/create table leaves the primary key unnamed", dialect: "mysql",
			diff: object(types.DiffAdd, types.ObjectTable, "users"),
			want: "-- Create table: users\nCREATE TABLE users (\n" +
				"  id int NOT NULL,\n  org_id int NULL,\n  name varchar(100) NOT NULL,\n  status varchar(20) DEFAULT 'active' NULL,\n" +
				"  PRIMARY KEY (id),\n  CONSTRAINT uq_users_name UNIQUE (name),\n  CONSTRAINT chk_status CHECK (status <> ''),\n" +
				"  CONSTRAINT fk_users_org FOREIGN KEY (org_id) REFERENCES orgs (id) ON DELETE CASCADE\n);",
		},
		{
			name: "create table without the source schema is not guessed", dialect: "postgresql", noSource: true,
			diff: object(types.DiffAdd, types.ObjectTable, "users"),
			want: "-- Create table: users\n-- " + noDefinition,
		},
		{
			name: "postgresql/add column with its type, default and nullability", dialect: "postgresql",
			diff: column(types.DiffAdd, "status", "", "", ""),
			want: "-- Add column: status to users\nALTER TABLE users ADD COLUMN status varchar(20) DEFAULT 'active' NULL;",
		},
		{
			name: "sqlserver/add column has no COLUMN keyword", dialect: "sqlserver", schema: "dbo",
			diff: column(types.DiffAdd, "name", "", "", ""),
			want: "-- Add column: name to users\nALTER TABLE dbo.users ADD name varchar(100) NOT NULL;",
		},
		{
			name: "add column without a type is not guessed", dialect: "mysql", noSource: true,
			diff: column(types.DiffAdd, "status", "", "", ""),
			want: "-- Add column: status to users\n-- " + noDefinition,
		},
		// Column changes per dialect.
		{
			name: "mysql/nullability change keeps the column's type and default", dialect: "mysql",
			diff: column(types.DiffAlter, "status", "NOT NULL", "NULL", columnNullableChanged),
			want: "-- Alter column: users.status\nALTER TABLE users MODIFY COLUMN status varchar(20) DEFAULT 'active' NULL;",
		},
		{
			name: "mysql/nullability change without the source schema never guesses a type", dialect: "mysql", noSource: true,
			diff: column(types.DiffAlter, "name", "NULL", "NOT NULL", columnNullableChanged),
			want: "-- Alter column: users.name\n-- " + noDefinition,
		},
		{
			name: "mysql/default change is a full MODIFY", dialect: "mysql",
			diff: column(types.DiffAlter, "status", "'new'", "'active'", columnDefaultChanged),
			want: "-- Alter column: users.status\nALTER TABLE users MODIFY COLUMN status varchar(20) DEFAULT 'active' NULL;",
		},
		{
			name: "sqlserver/nullability change restates the type", dialect: "sqlserver", schema: "dbo",
			diff: column(types.DiffAlter, "name", "NULL", "NOT NULL", columnNullableChanged),
			want: "-- Alter column: users.name\nALTER TABLE dbo.users ALTER COLUMN name varchar(100) NOT NULL;",
		},
		{
			name: "sqlserver/type change drops and restores the default", dialect: "sqlserver", schema: "dbo",
			diff: column(types.DiffAlter, "status", "varchar(10)", "varchar(20)", columnTypeChanged),
			want: "-- Alter column: users.status\n" + dropDefault("1", "status") +
				"\nALTER TABLE dbo.users ALTER COLUMN status varchar(20) NULL;\nALTER TABLE dbo.users ADD DEFAULT 'active' FOR status;",
		},
		{
			name: "sqlserver/default change", dialect: "sqlserver", schema: "dbo",
			diff: column(types.DiffAlter, "status", "'new'", "'active'", columnDefaultChanged),
			want: "-- Alter column: users.status\n" + dropDefault("1", "status") + "\nALTER TABLE dbo.users ADD DEFAULT 'active' FOR status;",
		},
		{
			name: "sqlserver/drop column drops its default first", dialect: "sqlserver", schema: "dbo",
			diff: column(types.DiffDrop, "legacy", "", "", ""),
			want: "-- Drop column: legacy from users\n" + dropDefault("1", "legacy") + "\nALTER TABLE dbo.users DROP COLUMN legacy;",
		},
		{
			name: "postgresql/nullability change", dialect: "postgresql",
			diff: column(types.DiffAlter, "name", "NULL", "NOT NULL", columnNullableChanged),
			want: "-- Alter column: users.name\nALTER TABLE users ALTER COLUMN name SET NOT NULL;",
		},
		{
			name: "postgresql/default change", dialect: "postgresql",
			diff: column(types.DiffAlter, "status", "'new'::character varying", "'active'::character varying", columnDefaultChanged),
			want: "-- Alter column: users.status\nALTER TABLE users ALTER COLUMN status SET DEFAULT 'active'::character varying;",
		},
		{
			name: "postgresql/default removed", dialect: "postgresql",
			diff: column(types.DiffAlter, "status", "'new'::character varying", "", columnDefaultChanged),
			want: "-- Alter column: users.status\nALTER TABLE users ALTER COLUMN status DROP DEFAULT;",
		},
		{
			name: "postgresql/type change", dialect: "postgresql",
			diff: column(types.DiffAlter, "org_id", "integer", "bigint", columnTypeChanged),
			want: "-- Alter column: users.org_id\nALTER TABLE users ALTER COLUMN org_id TYPE bigint USING (org_id::bigint);",
		},
		{
			name: "column order is reported, not rewritten", dialect: "postgresql",
			diff: column(types.DiffAlter, "__column_order__", "a, b", "b, a", columnOrderChanged),
			want: "-- Column order of users differs; this migration does not reorder columns",
		},
		// Constraints and foreign keys come with their definitions.
		{
			name: "postgresql/add check constraint", dialect: "postgresql",
			diff: object(types.DiffAdd, types.ObjectConstraint, "chk_status"),
			want: "-- Add constraint: chk_status\nALTER TABLE users ADD CONSTRAINT chk_status CHECK (status <> '');",
		},
		{
			name: "sqlserver/add unique constraint from its columns", dialect: "sqlserver", schema: "dbo",
			diff: object(types.DiffAdd, types.ObjectConstraint, "uq_users_name"),
			want: "-- Add constraint: uq_users_name\nALTER TABLE dbo.users ADD CONSTRAINT uq_users_name UNIQUE (name);",
		},
		{
			name: "mysql/add primary key", dialect: "mysql",
			diff: object(types.DiffAdd, types.ObjectConstraint, "users_pkey"),
			want: "-- Add constraint: users_pkey\nALTER TABLE users ADD PRIMARY KEY (id);",
		},
		{
			name: "mysql/drop primary key", dialect: "mysql",
			diff: object(types.DiffDrop, types.ObjectConstraint, "PRIMARY"),
			want: "-- Drop constraint: PRIMARY\nALTER TABLE users DROP PRIMARY KEY;",
		},
		{
			name: "add constraint without a definition is not guessed", dialect: "postgresql", noSource: true,
			diff: object(types.DiffAdd, types.ObjectConstraint, "chk_status"),
			want: "-- Add constraint: chk_status\n-- " + noDefinition,
		},
		{
			name: "alter constraint without a definition never drops it", dialect: "sqlserver", noSource: true,
			diff: object(types.DiffAlter, types.ObjectConstraint, "chk_status"),
			want: "-- Alter constraint: chk_status\n-- " + noDefinition,
		},
		{
			name: "sqlserver/add foreign key", dialect: "sqlserver", schema: "dbo",
			diff: object(types.DiffAdd, types.ObjectForeignKey, "fk_users_org"),
			want: "-- Add foreign key: fk_users_org\n" +
				"ALTER TABLE dbo.users ADD CONSTRAINT fk_users_org FOREIGN KEY (org_id) REFERENCES dbo.orgs (id) ON DELETE CASCADE;",
		},
		{
			name: "mysql/alter foreign key drops and re-adds it", dialect: "mysql",
			diff: object(types.DiffAlter, types.ObjectForeignKey, "fk_users_org"),
			want: "-- Alter foreign key: fk_users_org\nALTER TABLE users DROP FOREIGN KEY fk_users_org;\n" +
				"ALTER TABLE users ADD CONSTRAINT fk_users_org FOREIGN KEY (org_id) REFERENCES orgs (id) ON DELETE CASCADE;",
		},
		{
			name: "alter foreign key without a definition never drops it", dialect: "mysql", noSource: true,
			diff: object(types.DiffAlter, types.ObjectForeignKey, "fk_users_org"),
			want: "-- Alter foreign key: fk_users_org\n-- " + noDefinition,
		},
		// Tables, sequences and types.
		{
			name: "postgresql/drop table cascades", dialect: "postgresql",
			diff: object(types.DiffDrop, types.ObjectTable, "audit"),
			want: "-- Drop table: audit\nDROP TABLE IF EXISTS audit CASCADE;",
		},
		{
			name: "sqlserver/drop table has no CASCADE", dialect: "sqlserver", schema: "dbo",
			diff: object(types.DiffDrop, types.ObjectTable, "audit"),
			want: "-- Drop table: audit\nDROP TABLE IF EXISTS dbo.audit;",
		},
		{
			name: "mysql/drop table has no CASCADE", dialect: "mysql",
			diff: object(types.DiffDrop, types.ObjectTable, "audit"),
			want: "-- Drop table: audit\nDROP TABLE IF EXISTS audit;",
		},
		{
			name: "postgresql/create sequence with the source's bounds", dialect: "postgresql",
			diff: object(types.DiffAdd, types.ObjectSequence, "users_id_seq"),
			want: "-- Create sequence: users_id_seq\n" +
				"CREATE SEQUENCE users_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START WITH 1 CACHE 1;",
		},
		{
			name: "postgresql/create enum type with its labels", dialect: "postgresql",
			diff: object(types.DiffAdd, types.ObjectType, "mood"),
			want: "-- Create type: mood\nCREATE TYPE mood AS ENUM ('happy', 'it''s ok');",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := sourceFixture()
			if tt.noSource {
				source = nil
			}
			gen := NewSQLGeneratorWithSchema(types.DiffList{tt.diff}, tt.schema, source, tt.dialect)
			if got := gen.generateStatement(&tt.diff); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

// Each dialect gets its own lock timeout and transaction statements: SET lock_timeout and
// DO blocks are PostgreSQL-only, and T-SQL rejects BEGIN; and COMMIT; without BEGIN TRANSACTION.
func TestSQLGenerator_DialectPreambleAndTransaction(t *testing.T) {
	diffs := types.DiffList{{Type: types.DiffDrop, Object: types.ObjectTable, Name: "audit"}}
	tests := []struct {
		dialect     string
		lockTimeout string
		begin, end  string
	}{
		{"postgresql", "SET lock_timeout = '10s';", "DO $$\nBEGIN\n", "\n  RAISE;\nEND $$;"},
		{"mysql", "SET SESSION lock_wait_timeout = 10;", "START TRANSACTION;\n\n", "\n\nCOMMIT;"},
		{"sqlserver", "SET LOCK_TIMEOUT 10000;", "SET XACT_ABORT ON;\nBEGIN TRANSACTION;\n\n", "\n\nCOMMIT TRANSACTION;"},
	}
	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			gen := NewSQLGenerator(diffs, "", tt.dialect)
			gen.SetTransaction(false)
			plain := gen.Generate()
			if !strings.HasPrefix(plain, tt.lockTimeout+"\n") {
				t.Errorf("expected the %s lock timeout first, got:\n%s", tt.dialect, plain)
			}
			for _, other := range tests {
				if other.dialect != tt.dialect && strings.Contains(plain, other.lockTimeout) {
					t.Errorf("%s output has the %s lock timeout:\n%s", tt.dialect, other.dialect, plain)
				}
			}

			wrapped := NewSQLGenerator(diffs, "", tt.dialect).Generate()
			if !strings.Contains(wrapped, tt.begin+tt.lockTimeout) || !strings.HasSuffix(wrapped, tt.end) {
				t.Errorf("expected the %s transaction wrapper, got:\n%s", tt.dialect, wrapped)
			}
		})
	}

	if got := NewSQLGenerator(nil, "", "sqlserver").Generate(); got != "" {
		t.Errorf("expected no SQL for no diffs, got %q", got)
	}
}

func TestWithDetails(t *testing.T) {
	alter := types.Diff{Type: types.DiffAlter, Object: types.ObjectForeignKey, Name: "fk", TableName: "users",
		OldValue: "ON DELETE CASCADE", NewValue: "ON DELETE SET NULL", Description: "ON DELETE action changed"}
	got := withDetails(&alter, "-- Alter foreign key: fk\nALTER TABLE users DROP CONSTRAINT fk;")
	want := "-- Alter foreign key: fk\n-- ON DELETE action changed\n-- Old: ON DELETE CASCADE\n-- New: ON DELETE SET NULL\n" +
		"ALTER TABLE users DROP CONSTRAINT fk;"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	// Multi-line values stay comments: a definition must never leak into the SQL.
	index := types.Diff{Type: types.DiffAlter, Object: types.ObjectIndex, Name: "idx", OldValue: "a\nb", NewValue: "c"}
	got = withDetails(&index, "-- Recreate index: idx")
	if want := "-- Recreate index: idx\n-- Old: a\n-- b\n-- New: c"; got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	// A view's definitions are its statement; they are not repeated as comments.
	view := types.Diff{Type: types.DiffAlter, Object: types.ObjectView, Name: "v", OldValue: "SELECT 1", NewValue: "SELECT 2"}
	if got := withDetails(&view, "-- Alter view: v"); got != "-- Alter view: v" {
		t.Errorf("got:\n%s", got)
	}
}
