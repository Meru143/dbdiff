package testdata

import "github.com/meru143/dbdiff/pkg/types"

// UsersTable returns a test fixture for users table
func UsersTable() *types.Table {
	return &types.Table{
		Name: "users",
		Columns: []types.Column{
			{Name: "id", DataType: "integer", IsNullable: false, IsPrimaryKey: true},
			{Name: "email", DataType: "varchar", IsNullable: false},
			{Name: "password_hash", DataType: "varchar", IsNullable: false},
			{Name: "created_at", DataType: "timestamp", IsNullable: true},
			{Name: "updated_at", DataType: "timestamp", IsNullable: true},
		},
		Indexes: []types.Index{
			{Name: "users_pkey", Columns: []string{"id"}, IsPrimary: true, IsUnique: true},
			{Name: "users_email_idx", Columns: []string{"email"}, IsUnique: true},
		},
		Constraints: []types.Constraint{
			{Name: "users_pkey", Type: "PRIMARY KEY", Columns: []string{"id"}},
		},
		ForeignKeys: []types.ForeignKey{},
	}
}

// PostsTable returns a test fixture for posts table
func PostsTable() *types.Table {
	return &types.Table{
		Name: "posts",
		Columns: []types.Column{
			{Name: "id", DataType: "integer", IsNullable: false, IsPrimaryKey: true},
			{Name: "user_id", DataType: "integer", IsNullable: false},
			{Name: "title", DataType: "varchar", IsNullable: false},
			{Name: "body", DataType: "text", IsNullable: true},
			{Name: "published", DataType: "boolean", IsNullable: false, DefaultValue: strPtr("false")},
			{Name: "created_at", DataType: "timestamp", IsNullable: true},
			{Name: "updated_at", DataType: "timestamp", IsNullable: true},
		},
		Indexes: []types.Index{
			{Name: "posts_pkey", Columns: []string{"id"}, IsPrimary: true, IsUnique: true},
			{Name: "posts_user_id_idx", Columns: []string{"user_id"}, IsUnique: false},
		},
		Constraints: []types.Constraint{
			{Name: "posts_pkey", Type: "PRIMARY KEY", Columns: []string{"id"}},
		},
		ForeignKeys: []types.ForeignKey{
			{
				Name:           "posts_user_id_fkey",
				Columns:        []string{"user_id"},
				RefTable:       "users",
				RefColumns:     []string{"id"},
				OnDelete:       "CASCADE",
				OnUpdate:       "NO ACTION",
			},
		},
	}
}

// CommentsTable returns a test fixture for comments table
func CommentsTable() *types.Table {
	return &types.Table{
		Name: "comments",
		Columns: []types.Column{
			{Name: "id", DataType: "integer", IsNullable: false, IsPrimaryKey: true},
			{Name: "post_id", DataType: "integer", IsNullable: false},
			{Name: "user_id", DataType: "integer", IsNullable: false},
			{Name: "body", DataType: "text", IsNullable: false},
			{Name: "created_at", DataType: "timestamp", IsNullable: true},
		},
		Indexes: []types.Index{
			{Name: "comments_pkey", Columns: []string{"id"}, IsPrimary: true, IsUnique: true},
		},
		Constraints: []types.Constraint{
			{Name: "comments_pkey", Type: "PRIMARY KEY", Columns: []string{"id"}},
		},
		ForeignKeys: []types.ForeignKey{
			{
				Name:       "comments_post_id_fkey",
				Columns:    []string{"post_id"},
				RefTable:   "posts",
				RefColumns: []string{"id"},
				OnDelete:   "CASCADE",
			},
			{
				Name:       "comments_user_id_fkey",
				Columns:    []string{"user_id"},
				RefTable:   "users",
				RefColumns: []string{"id"},
				OnDelete:   "CASCADE",
			},
		},
	}
}

// FullTestSchema returns a complete test schema with all fixtures
func FullTestSchema() *types.Schema {
	return &types.Schema{
		Tables: []types.Table{
			*UsersTable(),
			*PostsTable(),
			*CommentsTable(),
		},
		Sequences: []types.Sequence{
			{Name: "users_id_seq", Start: 1, Increment: 1},
			{Name: "posts_id_seq", Start: 1, Increment: 1},
			{Name: "comments_id_seq", Start: 1, Increment: 1},
		},
		Types: []types.Type{
			{Name: "user_role", Kind: "enum", Values: []string{"admin", "user", "guest"}},
		},
	}
}

// IndexesFixture returns a test fixture for indexes
func IndexesFixture() []types.Index {
	return []types.Index{
		{Name: "users_pkey", Columns: []string{"id"}, IsPrimary: true, IsUnique: true},
		{Name: "users_email_idx", Columns: []string{"email"}, IsUnique: true},
		{Name: "posts_pkey", Columns: []string{"id"}, IsPrimary: true, IsUnique: true},
		{Name: "posts_user_id_idx", Columns: []string{"user_id"}, IsUnique: false},
		{Name: "posts_title_idx", Columns: []string{"title"}, IsUnique: false},
	}
}

// ForeignKeysFixture returns a test fixture for foreign keys
func ForeignKeysFixture() []types.ForeignKey {
	return []types.ForeignKey{
		{
			Name:           "posts_user_id_fkey",
			Columns:        []string{"user_id"},
			RefTable:       "users",
			RefColumns:     []string{"id"},
			OnDelete:       "CASCADE",
			OnUpdate:       "NO ACTION",
		},
		{
			Name:       "comments_post_id_fkey",
			Columns:    []string{"post_id"},
			RefTable:   "posts",
			RefColumns: []string{"id"},
			OnDelete:   "CASCADE",
		},
		{
			Name:       "comments_user_id_fkey",
			Columns:    []string{"user_id"},
			RefTable:   "users",
			RefColumns: []string{"id"},
			OnDelete:   "CASCADE",
		},
	}
}

func strPtr(s string) *string {
	return &s
}
