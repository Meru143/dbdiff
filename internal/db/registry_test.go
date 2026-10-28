package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

// A URL NewDriver refuses stays out of its error: it may hold a password. It refuses those
// whose driver would read part of the password as the host or the database, which the
// driver's errors quote: failed to connect to `user=me database=s3cret@host:5432/db`.
func TestNewDriverErrorHidesURL(t *testing.T) {
	for _, url := range []string{"postgress://app:s3cret@host/db", "root:s3cret@tcp(host:3306)/db",
		"postgres://app:/s3cret@host:5432/db", "postgres://app:4321/s3cret@host:5432/db",
		"mysql://app:x/s3cret%zz@tcp(host:3306)", "mysql://app:s3cret/x@tcp(host:3306)",
		"sqlserver://sa:4321/s3cret@host:1433?database=db"} {
		_, err := NewDriver(url)
		if err == nil || strings.Contains(err.Error(), "s3cret") {
			t.Errorf("%s: expected an error without the password, got %v", url, err)
		}
	}
}

type failingConnect struct {
	Driver
	err error
}

func (f failingConnect) Connect(context.Context, string, string, time.Duration) error { return f.err }

// Every driver NewDriver makes connects through dsn.HideParams: an error that quotes what
// follows a password parameter's '=' is left out, and the others pass through.
func TestConnectHidesParamsAfterPassword(t *testing.T) {
	quoting := errors.New(`server error: FATAL: unrecognized configuration parameter "k3y" (SQLSTATE 42704)`)
	err := withoutMigrationsTable{failingConnect{err: quoting}}.Connect(context.Background(), "postgres://app@host/db?password=s3cret&k3y=x", "disable", time.Second)
	if err == nil || strings.Contains(err.Error(), "k3y") || strings.Contains(err.Error(), "s3cret") {
		t.Errorf("expected an error without the parameter, got %v", err)
	}
	other := errors.New("connection refused")
	if err := (withoutMigrationsTable{failingConnect{err: other}}).Connect(context.Background(), "postgres://app@host/db?password=s3cret&k3y=x", "disable", time.Second); err != other {
		t.Errorf("expected the driver's error, got %v", err)
	}
	// go-mssqldb names a duplicated key without quotes; it is hidden too when it follows the
	// password parameter.
	dupKey := errors.New("key k3y provided more than once")
	if err := (withoutMigrationsTable{failingConnect{err: dupKey}}).Connect(context.Background(), "sqlserver://host?database=db&password=s3cret&k3y&k3y", "disable", time.Second); err == nil || strings.Contains(err.Error(), "k3y") || strings.Contains(err.Error(), "s3cret") {
		t.Errorf("expected the duplicate key hidden, got %v", err)
	}
}
