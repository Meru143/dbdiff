package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/logging"
	"github.com/spf13/cobra"
)

var ApplyCmd = &cobra.Command{
	Use:   "apply [migration.sql] [target]",
	Short: "Apply a migration SQL file and track its execution",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		logging.Init(cfg.Debug, cfg.Verbose)

		migrationFile := args[0]
		target := args[1]

		// Check if file exists
		content, err := os.ReadFile(migrationFile)
		if err != nil {
			return fmt.Errorf("failed to read migration file: %w", err)
		}

		version := filepath.Base(migrationFile)

		ctx := context.Background()

		// Connect to DB
		driver, err := db.NewDriver(target)
		if err != nil {
			return fmt.Errorf("failed to initialize driver: %w", err)
		}

		err = driver.Connect(ctx, target, cfg.SSLMode, cfg.Timeout)
		if err != nil {
			return fmt.Errorf("failed to connect to target: %w", err)
		}
		defer driver.Close()

		// Ensure tracking table exists
		if err := driver.EnsureMigrationsTable(ctx); err != nil {
			return fmt.Errorf("failed to ensure migrations tracking table: %w", err)
		}

		// Check if it already ran
		applied, err := driver.GetAppliedMigrations(ctx)
		if err != nil {
			return fmt.Errorf("failed to retrieve applied migrations: %w", err)
		}

		for _, v := range applied {
			if v == version {
				logging.Warn(fmt.Sprintf("Migration %s has already been applied. Skipping.", version))
				return nil
			}
		}

		// Interactive confirmation if not forced
		if !cfg.Force {
			fmt.Printf("\nYou are about to apply migration '%s' to target database.\n", version)
			fmt.Print("Are you sure you want to proceed? [y/N]: ")
			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				logging.Warn("Operation aborted.")
				return nil
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				logging.Warn("Operation aborted by user.")
				return nil
			}
		}

		// Apply the sql
		logging.Info(fmt.Sprintf("Applying migration: %s", version))

		// If transaction is desired and not manually managed in SQL
		sqlCmd := string(content)
		if cfg.Transaction && !strings.Contains(strings.ToUpper(sqlCmd), "BEGIN;") {
			sqlCmd = "BEGIN;\n" + sqlCmd + "\nCOMMIT;"
		}

		if err := driver.Exec(ctx, sqlCmd); err != nil {
			return fmt.Errorf("failed to apply migration '%s': %w", version, err)
		}

		// Mark as applied
		if err := driver.RecordMigration(ctx, version); err != nil {
			return fmt.Errorf("migration applied but failed to record into tracking table: %w", err)
		}

		logging.Info(fmt.Sprintf("Migration %s successfully recorded.", version))
		return nil
	},
}
