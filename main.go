package main

import (
	"fmt"
	"os"

	"github.com/meru143/dbdiff/cmd"
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "dbdiff",
		Short:   "PostgreSQL schema comparison and migration tool",
		Version: fmt.Sprintf("%s (commit: %s, date: %s)", version, commit, buildDate),
	}

	// Add persistent flags
	rootCmd.PersistentFlags().StringP("source", "s", "", "Source database URL")
	rootCmd.PersistentFlags().StringP("target", "t", "", "Target database URL")
	rootCmd.PersistentFlags().StringP("output", "o", "", "Output file path")
	rootCmd.PersistentFlags().String("format", "sql", "Output format: sql, table, json")
	rootCmd.PersistentFlags().Bool("dry-run", true, "Dry-run mode")
	rootCmd.PersistentFlags().StringP("config", "c", "", "Config file path")
	rootCmd.PersistentFlags().StringSlice("ignore-patterns", nil, "Columns to ignore")
	rootCmd.PersistentFlags().String("schema", "public", "Schema to compare")
	rootCmd.PersistentFlags().Duration("timeout", 30, "Query timeout")
	rootCmd.PersistentFlags().Bool("transaction", true, "Wrap in transaction")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().Bool("debug", false, "Debug mode")
	rootCmd.PersistentFlags().String("ssl-mode", "disable", "SSL mode")

	// Add subcommands
	rootCmd.AddCommand(cmd.CompareCmd)
	rootCmd.AddCommand(cmd.MigrateCmd)
	rootCmd.AddCommand(cmd.DiffCmd)
	rootCmd.AddCommand(cmd.TablesCmd)
	rootCmd.AddCommand(cmd.ValidateCmd)

	// Add completion command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			}
		},
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
