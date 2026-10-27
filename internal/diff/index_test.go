package diff

import (
	"testing"

	"github.com/meru143/dbdiff/pkg/types"
)

func TestCompareIndexes(t *testing.T) {
	table := func(indexes ...types.Index) *types.Schema {
		return &types.Schema{Tables: []types.Table{{Name: "users", Indexes: indexes}}}
	}
	plain := types.Index{Name: "idx_email", Columns: []string{"email"}}
	unique := types.Index{Name: "idx_email", Columns: []string{"email"}, IsUnique: true}
	// PostgreSQL's index behind a UNIQUE constraint, and the primary key's.
	constraintIdx := types.Index{Name: "users_email_key", Columns: []string{"email"}, IsUnique: true, IsConstraint: true,
		Definition: "CREATE UNIQUE INDEX users_email_key ON public.users USING btree (email)"}
	pkeyIdx := types.Index{Name: "users_pkey", Columns: []string{"id"}, IsUnique: true, IsPrimary: true}

	tests := []struct {
		name           string
		source, target *types.Schema
		want           []types.DiffType
	}{
		// MySQL and SQL Server indexes used to be compared by name and columns only.
		{"uniqueness-only change", table(unique), table(plain), []types.DiffType{types.DiffAlter}},
		{"no change", table(unique), table(unique), nil},
		// The constraint diff creates and drops these; an index diff would collide with it.
		{"unique constraint's index added", table(constraintIdx), table(), nil},
		{"unique constraint's index dropped", table(), table(constraintIdx), nil},
		{"primary key's index", table(pkeyIdx), table(), nil},
		// A plain index replaced by a constraint of the same name: the index is dropped.
		{"plain index becomes a constraint's", table(types.Index{Name: "idx_email", Columns: []string{"email"},
			IsUnique: true, IsConstraint: true}), table(plain), []types.DiffType{types.DiffDrop}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []types.DiffType
			for _, d := range Compare(tt.source, tt.target) {
				if d.Object == types.ObjectIndex {
					got = append(got, d.Type)
				}
			}
			if len(got) != len(tt.want) || (len(got) > 0 && got[0] != tt.want[0]) {
				t.Errorf("got index diffs %v, want %v", got, tt.want)
			}
		})
	}
}

// A new table's own indexes are created after it, but not its constraints' indexes, which
// CREATE TABLE makes.
func TestCompare_NewTableIndexes(t *testing.T) {
	source := &types.Schema{Tables: []types.Table{{
		Name: "posts",
		Indexes: []types.Index{
			{Name: "posts_pkey", Columns: []string{"id"}, IsPrimary: true},
			{Name: "posts_slug_key", Columns: []string{"slug"}, IsUnique: true, IsConstraint: true},
			{Name: "idx_posts_title", Columns: []string{"title"}},
		},
	}}}
	diffs := Compare(source, &types.Schema{})
	if len(diffs) != 2 || diffs[1].Object != types.ObjectIndex || diffs[1].Name != "idx_posts_title" || diffs[1].Index == nil {
		t.Errorf("expected the table and idx_posts_title, got %+v", diffs)
	}
}

// With both definitions known, any change to a constraint shows, not only a primary key's
// columns.
func TestCompareConstraints_Definition(t *testing.T) {
	check := func(def string) *types.Schema {
		return &types.Schema{Tables: []types.Table{{Name: "users", Constraints: []types.Constraint{
			{Name: "chk_age", Type: "CHECK", Definition: def},
		}}}}
	}
	diffs := Compare(check("CHECK ((age > 18))"), check("CHECK ((age > 0))"))
	if len(diffs) != 1 || diffs[0].Type != types.DiffAlter || diffs[0].NewValue != "CHECK ((age > 18))" {
		t.Errorf("expected the CHECK change, got %+v", diffs)
	}
	if diffs := Compare(check("CHECK ((age > 0))"), check("CHECK ((age > 0))")); len(diffs) != 0 {
		t.Errorf("expected no change, got %+v", diffs)
	}
}
