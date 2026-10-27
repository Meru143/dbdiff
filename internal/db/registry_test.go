package db

import (
	"context"
	"strings"
	"testing"

	"github.com/meru143/dbdiff/pkg/types"
)

type fixedSchema struct {
	Driver
	schema *types.Schema
}

func (f fixedSchema) Introspect(context.Context, string, []string) (*types.Schema, error) {
	return f.schema, nil
}

func TestIntrospectLeavesOutMigrationsTable(t *testing.T) {
	d := withoutMigrationsTable{fixedSchema{schema: &types.Schema{
		Tables:   []types.Table{{Name: "users"}, {Name: "_dbdiff_migrations"}, {Name: "_DBDIFF_MIGRATIONS"}},
		Grants:   []types.Grant{{Table: "_dbdiff_migrations", Grantee: "reader"}, {Table: "users", Grantee: "reader"}},
		Triggers: []types.Trigger{{Name: "audit", Table: "_dbdiff_migrations"}, {Name: "touch", Table: "users"}},
	}}}
	s, err := d.Introspect(context.Background(), "public", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Tables) != 1 || s.Tables[0].Name != "users" ||
		len(s.Grants) != 1 || s.Grants[0].Table != "users" ||
		len(s.Triggers) != 1 || s.Triggers[0].Name != "touch" {
		t.Errorf("expected only users and its grant and trigger, got %+v", s)
	}
}

// A URL NewDriver refuses stays out of its error: it may hold a password.
func TestNewDriverErrorHidesURL(t *testing.T) {
	for _, url := range []string{"postgress://app:s3cret@host/db", "root:s3cret@tcp(host:3306)/db"} {
		_, err := NewDriver(url)
		if err == nil || strings.Contains(err.Error(), "s3cret") {
			t.Errorf("%s: expected an error without the password, got %v", url, err)
		}
	}
}
