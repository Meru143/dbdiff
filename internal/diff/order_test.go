package diff

import (
	"strings"
	"testing"

	"github.com/meru143/dbdiff/pkg/types"
)

// statements lists the SQL statements of a migration, without comments or the lock timeout,
// cut to their first line.
func statements(sql string) []string {
	var out []string
	for _, line := range strings.Split(sql, "\n") {
		for _, verb := range []string{"CREATE ", "ALTER ", "DROP "} {
			if strings.HasPrefix(line, verb) {
				out = append(out, strings.TrimSuffix(line, " ("))
			}
		}
	}
	return out
}

// Statements run after what they depend on, whatever order the engine found the diffs in.
func TestSQLGenerator_StatementOrder(t *testing.T) {
	source := sourceFixture()
	source.Tables = append(source.Tables, types.Table{
		Name:        "posts",
		Columns:     []types.Column{{Name: "id", DataType: "int"}, {Name: "user_id", DataType: "int"}},
		ForeignKeys: []types.ForeignKey{{Name: "fk_posts_user", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}}},
		Indexes:     []types.Index{{Name: "idx_posts_user", Columns: []string{"user_id"}}},
	})
	userName := source.Tables[0].Columns[2]
	idxName := &types.Index{Name: "idx_users_name", Columns: []string{"name"}, IsUnique: true}

	tests := []struct {
		name    string
		dialect string
		diffs   types.DiffList
		want    []string
	}{
		{
			name: "an indexed column is dropped after its index", dialect: "sqlserver",
			diffs: types.DiffList{
				{Type: types.DiffDrop, Object: types.ObjectColumn, Name: "legacy", TableName: "users"},
				{Type: types.DiffDrop, Object: types.ObjectIndex, Name: "idx_users_legacy", TableName: "users"},
			},
			want: []string{
				"DROP INDEX idx_users_legacy ON users;",
				"ALTER TABLE users DROP COLUMN legacy;",
			},
		},
		{
			name: "new tables exist before the foreign keys between them", dialect: "postgresql",
			diffs: types.DiffList{
				{Type: types.DiffAdd, Object: types.ObjectForeignKey, Name: "fk_posts_user", TableName: "posts"},
				{Type: types.DiffAdd, Object: types.ObjectTable, Name: "posts"},
				{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
			},
			want: []string{
				"CREATE TABLE posts",
				"CREATE TABLE users",
				"ALTER TABLE posts ADD CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users (id);",
			},
		},
		{
			name: "foreign keys go before the tables they point at", dialect: "mysql",
			diffs: types.DiffList{
				{Type: types.DiffDrop, Object: types.ObjectTable, Name: "teams"},
				{Type: types.DiffDrop, Object: types.ObjectForeignKey, Name: "fk_users_team", TableName: "users"},
			},
			want: []string{
				"ALTER TABLE users DROP FOREIGN KEY fk_users_team;",
				"DROP TABLE IF EXISTS teams;",
			},
		},
		{
			name: "a recreated index is dropped before its column changes and added after", dialect: "sqlserver",
			diffs: types.DiffList{
				{Type: types.DiffAlter, Object: types.ObjectIndex, Name: idxName.Name, TableName: "users", Index: idxName},
				{Type: types.DiffAlter, Object: types.ObjectColumn, Name: "name", TableName: "users",
					OldValue: "NULL", NewValue: "NOT NULL", Description: columnNullableChanged},
			},
			want: []string{
				"DROP INDEX idx_users_name ON users;",
				"ALTER TABLE users ALTER COLUMN name varchar(100) NOT NULL;",
				"CREATE UNIQUE INDEX idx_users_name ON users (name);",
			},
		},
		{
			name: "a foreign key changed twice is recreated once", dialect: "postgresql",
			diffs: types.DiffList{
				{Type: types.DiffAlter, Object: types.ObjectForeignKey, Name: "fk_users_org", TableName: "users",
					OldValue: "ON DELETE NO ACTION", NewValue: "ON DELETE CASCADE", Description: "ON DELETE action changed"},
				{Type: types.DiffAlter, Object: types.ObjectForeignKey, Name: "fk_users_org", TableName: "users",
					OldValue: "teams", NewValue: "orgs", Description: "Referenced table changed"},
			},
			want: []string{
				"ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_org;",
				"ALTER TABLE users ADD CONSTRAINT fk_users_org FOREIGN KEY (org_id) REFERENCES orgs (id) ON DELETE CASCADE;",
			},
		},
		{
			name: "a MySQL column changed twice is modified once", dialect: "mysql",
			diffs: types.DiffList{
				{Type: types.DiffAlter, Object: types.ObjectColumn, Name: "name", TableName: "users",
					OldValue: "varchar(50)", NewValue: userName.DataType, Description: columnTypeChanged},
				{Type: types.DiffAlter, Object: types.ObjectColumn, Name: "name", TableName: "users",
					OldValue: "NULL", NewValue: "NOT NULL", Description: columnNullableChanged},
			},
			want: []string{"ALTER TABLE users MODIFY COLUMN name varchar(100) NOT NULL;"},
		},
		{
			name: "sequences exist before the tables that use them and outlive the dropped ones", dialect: "postgresql",
			diffs: types.DiffList{
				{Type: types.DiffDrop, Object: types.ObjectSequence, Name: "audit_id_seq"},
				{Type: types.DiffAdd, Object: types.ObjectTable, Name: "users"},
				{Type: types.DiffDrop, Object: types.ObjectTable, Name: "audit"},
				{Type: types.DiffAdd, Object: types.ObjectSequence, Name: "users_id_seq"},
			},
			want: []string{
				"CREATE SEQUENCE users_id_seq INCREMENT BY 1 MINVALUE 1 MAXVALUE 2147483647 START WITH 1 CACHE 1;",
				"CREATE TABLE users",
				"DROP TABLE IF EXISTS audit CASCADE;",
				"DROP SEQUENCE IF EXISTS audit_id_seq;",
			},
		},
		{
			name: "functions are dropped before the types their signatures use", dialect: "postgresql",
			diffs: types.DiffList{
				{Type: types.DiffDrop, Object: types.ObjectType, Name: "mood"},
				{Type: types.DiffDrop, Object: types.ObjectFunction, Name: "cheer(mood)"},
			},
			want: []string{
				"DROP FUNCTION IF EXISTS cheer(mood) CASCADE;",
				"DROP TYPE IF EXISTS mood;",
			},
		},
		{
			name: "new columns keep the source's order", dialect: "postgresql",
			diffs: types.DiffList{
				{Type: types.DiffAdd, Object: types.ObjectColumn, Name: "status", TableName: "users"},
				{Type: types.DiffAdd, Object: types.ObjectColumn, Name: "name", TableName: "users"},
				{Type: types.DiffAdd, Object: types.ObjectColumn, Name: "org_id", TableName: "users"},
			},
			want: []string{
				"ALTER TABLE users ADD COLUMN org_id int NULL;",
				"ALTER TABLE users ADD COLUMN name varchar(100) NOT NULL;",
				"ALTER TABLE users ADD COLUMN status varchar(20) DEFAULT 'active' NULL;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewSQLGeneratorWithSchema(tt.diffs, "", source, tt.dialect)
			gen.SetTransaction(false)
			sql := gen.Generate()
			got := statements(sql)
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("got:\n%s\nwant:\n%s\nfull SQL:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"), sql)
			}
		})
	}
}

// An ALTER that can't be written whole stays one comment: nothing is dropped that the
// migration can't add back.
func TestSQLGenerator_UnrecreatableAlterIsNotSplit(t *testing.T) {
	diffs := types.DiffList{
		{Type: types.DiffAlter, Object: types.ObjectConstraint, Name: "chk_missing", TableName: "users"},
		{Type: types.DiffAlter, Object: types.ObjectIndex, Name: "idx_missing", TableName: "users"},
	}
	gen := NewSQLGeneratorWithSchema(diffs, "", sourceFixture(), "mysql")
	gen.SetTransaction(false)
	if sql := gen.Generate(); len(statements(sql)) != 0 {
		t.Errorf("expected no statements, got:\n%s", sql)
	}
}

// The engine lists a new table's foreign keys, which CREATE TABLE leaves out.
func TestCompare_NewTableForeignKeys(t *testing.T) {
	source := &types.Schema{Tables: []types.Table{{
		Name:        "posts",
		Columns:     []types.Column{{Name: "user_id", DataType: "int"}},
		ForeignKeys: []types.ForeignKey{{Name: "fk_posts_user", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}}},
	}}}
	diffs := Compare(source, &types.Schema{})
	if len(diffs) != 2 || diffs[1].Object != types.ObjectForeignKey || diffs[1].Name != "fk_posts_user" || diffs[1].TableName != "posts" {
		t.Errorf("expected the table and its foreign key, got %+v", diffs)
	}
}
