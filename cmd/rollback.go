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

var RollbackCmd = &cobra.Command{
	Use:   "rollback [down.sql] [target]",
	Short: "Rollback a migration and remove it from history",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		logging.Init(cfg.Debug, cfg.Verbose)

		migrationFile := args[0]
		target := args[1]

		content, err := os.ReadFile(migrationFile)
		if err != nil {
			return fmt.Errorf("failed to read rollback file: %w", err)
		}

		// The version is assumed to be the original filename, e.g., if down.sql is 001_down.sql,
		// you could pass 001.sql as a flag? Simplest approach: let user specify the version string or infer it.
		// For this simple version, let's use the filename, but prompt the user.
		version := filepath.Base(migrationFile)

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

		// Check if it's actually applied
		isApplied := false
		for _, v := range applied {
			// Basic heuristic matching: if rollback script is "001_down.sql", look for "001*.sql"
			if strings.HasPrefix(v, strings.Split(version, "_")[0]) || v == version {
				version = v // Adopt the database's version identifier natively
				isApplied = true
				break
			}
		}

		if !isApplied {
			logging.Warn(fmt.Sprintf("Migration matching %s does not appear in history. Skipping rollback.", version))
			return nil
		}

		logging.Info(fmt.Sprintf("Reverting migration: %s", version))

		sqlCmd := string(content)
		if cfg.Transaction && !strings.Contains(strings.ToUpper(sqlCmd), "BEGIN;") {
			sqlCmd = "BEGIN;\n" + sqlCmd + "\nCOMMIT;"
		}

		if _, err := targetDB.Exec(ctx, sqlCmd); err != nil {
			return fmt.Errorf("failed to execute rollback script: %w", err)
		}

		if err := db.RemoveMigration(ctx, targetDB, version); err != nil {
			return fmt.Errorf("rollback script succeeded but failed to purge history record: %w", err)
		}

		logging.Info(fmt.Sprintf("Migration %s successfully rolled back.", version))
		return nil
	},
}
