package main

import (
	"bytes"
	"strings"
	"testing"
)

// dbdiff --help opens with what dbdiff does: all three databases, not only PostgreSQL.
func TestRootHelp(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := "Compare PostgreSQL, MySQL and SQL Server schemas and generate migration SQL\n"
	if help := out.String(); !strings.HasPrefix(help, want) {
		t.Errorf("help should start with %q, got:\n%s", want, help)
	}
}
