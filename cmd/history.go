package cmd

import (
	"context"
	"fmt"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/logging"
	"github.com/spf13/cobra"
)

var HistoryCmd = &cobra.Command{
	Use:   "history [target]",
	Short: "Show schema migration history",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		logging.Init(cfg.Debug, cfg.Verbose)

		target := args[0]

		ctx := context.Background()

		targetDB, err := db.Connect(ctx, target, cfg.SSLMode, cfg.Timeout)
		if err != nil {
			return fmt.Errorf("failed to connect to target: %w", err)
		}
		defer targetDB.Close()

		if err := db.EnsureMigrationsTable(ctx, targetDB); err != nil {
			return fmt.Errorf("failed to ensure migrations tracking table: %w", err)
		}

		applied, err := db.GetAppliedMigrations(ctx, targetDB)
		if err != nil {
			return fmt.Errorf("failed to retrieve applied migrations: %w", err)
		}

		if len(applied) == 0 {
			fmt.Println("No migrations have been applied to this database yet.")
			return nil
		}

		fmt.Println("┌──────────────────────────────────────────────┐")
		fmt.Println("│ APPLIED MIGRATIONS                           │")
		fmt.Println("├──────────────────────────────────────────────┤")
		for _, version := range applied {
			fmt.Printf("│ %-44s │\n", version)
		}
		fmt.Println("└──────────────────────────────────────────────┘")

		return nil
	},
}
