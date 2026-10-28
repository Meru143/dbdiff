package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

// cobra's help command prints a topic it can't find itself, past main's error print: the
// password in one stays hidden, and a topic it finds still gets its help.
func TestHelpTopicHidesPassword(t *testing.T) {
	root := &cobra.Command{Use: "dbdiff"}
	root.AddCommand(&cobra.Command{Use: "compare", Short: "Compare two database schemas", Run: func(*cobra.Command, []string) {}})
	hideHelpTopicPasswords(root)
	for args, want := range map[string]string{
		"postgres://app:s3cret@host/db": "Unknown help topic [`postgres://app:***@host/db`]",
		"compare":                       "Compare two database schemas",
	} {
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{"help", args})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if got := out.String(); strings.Contains(got, "s3cret") || !strings.Contains(got, want) {
			t.Errorf("help %s: want %q, without the password, got:\n%s", args, want, got)
		}
	}
}
