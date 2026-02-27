package cmd

import (
	"context"
	"fmt"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/diff"
	"github.com/meru143/dbdiff/internal/logging"
	"github.com/meru143/dbdiff/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var MigrateCmd = &cobra.Command{
	Use:   "migrate [source] [target]",
	Short: "Generate migration SQL",
	Args:  cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		logging.Init(cfg.Debug, cfg.Verbose)

		source := getConfigValue(args, 0, cfg.Source, "source")
		target := getConfigValue(args, 1, cfg.Target, "target")

		if source == "" || target == "" {
			return fmt.Errorf("source and target database URLs are required")
		}

		if cfg.Output == "" {
			cfg.Output = "migration.sql"
		}

		logging.Info(fmt.Sprintf("Generating migration: %s -> %s", source, target))

		ctx := context.Background()

		sourceDB, err := db.Connect(ctx, source, cfg.SSLMode, cfg.Timeout)
		if err != nil {
			return fmt.Errorf("failed to connect to source: %w", err)
		}
		defer sourceDB.Close()

		targetDB, err := db.Connect(ctx, target, cfg.SSLMode, cfg.Timeout)
		if err != nil {
			return fmt.Errorf("failed to connect to target: %w", err)
		}
		defer targetDB.Close()

		sourceSchema, err := db.Introspect(ctx, sourceDB, cfg.Schema, cfg.IgnorePatterns)
		if err != nil {
			return fmt.Errorf("failed to introspect source: %w", err)
		}

		targetSchema, err := db.Introspect(ctx, targetDB, cfg.Schema, cfg.IgnorePatterns)
		if err != nil {
			return fmt.Errorf("failed to introspect target: %w", err)
		}

		differences := diff.Compare(sourceSchema, targetSchema)

		if len(differences) == 0 {
			logging.Info("No differences found!")
			return nil
		}

		logging.Info(fmt.Sprintf("Found %d differences", len(differences)))

		formatter := output.NewFormatter("sql")
		migrationSQL, err := formatter.FormatMigration(differences, cfg.Transaction)
		if err != nil {
			return fmt.Errorf("failed to generate migration: %w", err)
		}

		header := fmt.Sprintf("-- DBDiff Migration\n-- Source: %s\n-- Target: %s\n-- Schema: %s\n\n", source, target, cfg.Schema)
		fullSQL := header + migrationSQL

		if cfg.Output == "stdout" {
			fmt.Println(fullSQL)
		} else {
			if err := output.WriteFile(cfg.Output, fullSQL); err != nil {
				return fmt.Errorf("failed to write migration: %w", err)
			}
			logging.Info(fmt.Sprintf("Migration written to: %s", cfg.Output))
		}

		if cfg.DryRun {
			logging.Warn("Dry-run mode: migration not applied")
		}

		return nil
	},
}
