package cmd

import (
	"context"
	"fmt"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/logging"
	"github.com/spf13/cobra"
)

var TablesCmd = &cobra.Command{
	Use:   "tables [database]",
	Short: "List tables in a database",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		logging.Init(cfg.Debug, cfg.Verbose)

		database := getConfigValue(args, 0, cfg.Source, "source")
		if database == "" {
			return fmt.Errorf("database URL is required")
		}

		logging.Info(fmt.Sprintf("Connecting to: %s", database))

		ctx := context.Background()
		conn, err := db.Connect(ctx, database, cfg.SSLMode, cfg.Timeout)
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer conn.Close()

		tables, err := db.ListTables(ctx, conn, cfg.Schema, cfg.IgnorePatterns)
		if err != nil {
			return fmt.Errorf("failed to list tables: %w", err)
		}

		if len(tables) == 0 {
			fmt.Printf("No tables found in schema: %s\n", cfg.Schema)
			return nil
		}

		fmt.Printf("\nTables in schema '%s':\n\n", cfg.Schema)
		for i, table := range tables {
			fmt.Printf("  %d. %s\n", i+1, table)
		}
		fmt.Printf("\nTotal: %d tables\n", len(tables))

		return nil
	},
}
