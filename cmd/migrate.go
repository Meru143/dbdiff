package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/diff"
	"github.com/meru143/dbdiff/internal/logging"
	"github.com/meru143/dbdiff/internal/output"
	"github.com/meru143/dbdiff/internal/safety"
	"github.com/meru143/dbdiff/pkg/types"
	"github.com/spf13/cobra"
)

// confirmPrompt asks the user for confirmation
func confirmPrompt() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Do you want to proceed? (yes/no/abort): ")
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "yes", "y":
		return true
	case "no", "n":
		return false
	case "abort", "a":
		fmt.Println("Aborted.")
		os.Exit(0)
	}
	return false
}

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

		// Validate and filter protected objects
		if len(cfg.ProtectedObjects) > 0 {
			validator := safety.NewValidator(cfg.ProtectedObjects, cfg.DryRun)
			validation := validator.ValidateDiffs(differences)

			if len(validation.Errors) > 0 {
				for _, err := range validation.Errors {
					logging.Error(err)
				}
				return fmt.Errorf("migration contains protected objects")
			}

			// Filter out protected diffs
			var filteredDiffs []types.Diff
			for _, d := range differences {
				if !validator.IsProtected(d.Name) {
					filteredDiffs = append(filteredDiffs, d)
				}
			}
			differences = filteredDiffs

			if len(validation.Warnings) > 0 {
				for _, warn := range validation.Warnings {
					logging.Warn(warn)
				}
			}
		}

		formatterOpts := output.FormatOptions{
			Format:       "sql",
			IsExecutable: cfg.Executable,
		}
		formatter := output.NewFormatterWithOptions(formatterOpts)
		migrationSQL, err := formatter.FormatMigration(differences, cfg.Transaction)
		if err != nil {
			return fmt.Errorf("failed to generate migration: %w", err)
		}

		header := fmt.Sprintf("-- DBDiff Migration\n-- Source: %s\n-- Target: %s\n-- Schema: %s\n\n", source, target, cfg.Schema)

		var fullSQL string
		if cfg.Executable {
			// If executing a bash script, omit the header as shebang is inside formatExecutable natively.
			fullSQL = migrationSQL
		} else {
			fullSQL = header + migrationSQL
		}

		// Confirmation prompt (only for non-dry-run, non-force)
		if !cfg.DryRun && !cfg.Force && cfg.Output != "stdout" {
			fmt.Printf("\nMigration will write to: %s\n", cfg.Output)
			if !confirmPrompt() {
				logging.Info("Migration cancelled")
				return nil
			}
		}

		// Create backup before writing
		if cfg.BackupDir != "" && cfg.Output != "stdout" && !cfg.DryRun {
			backupMgr := safety.NewBackupManager(cfg.BackupDir, cfg.MaxBackups)

			// Check if target file exists
			if _, err := os.Stat(cfg.Output); err == nil {
				existingContent, _ := os.ReadFile(cfg.Output)
				backupPath, err := backupMgr.Backup("migration", existingContent)
				if err != nil {
					logging.Warn(fmt.Sprintf("Failed to create backup: %v", err))
				} else {
					logging.Info(fmt.Sprintf("Backup created: %s", backupPath))
				}
			}
		}

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

func init() {
	MigrateCmd.Flags().BoolP("executable", "x", false, "Generate an executable bash migration script")
}
