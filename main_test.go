package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

// cobra's __complete command, which shells run for completions, prints the arguments it
// can't place itself, on stderr, past main's error print: the password in one stays hidden,
// and completions still come.
func TestCompletionHidesPassword(t *testing.T) {
	root := &cobra.Command{Use: "dbdiff"}
	root.AddCommand(&cobra.Command{Use: "compare", Run: func(*cobra.Command, []string) {}})
	hideCompletionPasswords(root)
	stderr := os.Stderr
	t.Cleanup(func() { os.Stderr = stderr })
	for _, args := range [][]string{
		{cobra.ShellCompRequestCmd, "postgres://app:s3cret@host/db", ""},                         // unable to find a command for arguments: [...]
		{cobra.ShellCompNoDescRequestCmd, "--bogus", "mysql://app:s3cret@tcp(host:3306)/db", ""}, // Error while parsing flags from args [...]
		{cobra.ShellCompRequestCmd, "comp"},
	} {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stderr = w
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs(args)
		err = root.Execute()
		os.Stderr = stderr
		w.Close()
		printed, _ := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		got := out.String() + string(printed)
		if strings.Contains(got, "s3cret") {
			t.Errorf("%q printed the password:\n%s", args, got)
		}
		if args[1] == "comp" && !strings.Contains(out.String(), "compare") {
			t.Errorf("%q should still complete compare, got:\n%s", args, got)
		} else if args[1] != "comp" && !strings.Contains(got, "app:***@") {
			t.Errorf("%q should show the URL with its password hidden, got:\n%s", args, got)
		}
	}
}

// root's PersistentPreRun doesn't give viper a URL given for --config, or in DBDIFF_CONFIG,
// to read as a file. (config.Load, which reads --config again, printed part of the password
// viper took for the file's type: Unsupported Config Type "QZYb@localhost:5432".)
func TestConfigURLIsNotRead(t *testing.T) {
	viper.Set("config", "postgres://app:QZXa.QZYb@localhost:5432")
	stderr := os.Stderr
	t.Cleanup(func() {
		os.Stderr = stderr
		viper.Set("config", "")
	})
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	rootCmd.PersistentPreRun(rootCmd, nil)
	os.Stderr = stderr
	w.Close()
	printed, _ := io.ReadAll(r)
	r.Close()
	if got := string(printed); strings.Contains(got, "QZ") || !strings.Contains(got, "not a URL: postgres://app:***@localhost:5432\n") {
		t.Errorf("expected a warning naming the URL without its password, got %q", got)
	}
}
