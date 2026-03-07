package cmd

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/diff"
	"github.com/meru143/dbdiff/internal/logging"
	"github.com/meru143/dbdiff/internal/output"
	"github.com/meru143/dbdiff/internal/tui"
	"github.com/spf13/cobra"
)

var InteractiveCmd = &cobra.Command{
	Use:   "interactive [source] [target]",
	Short: "Interactively choose schema migrations to apply via Terminal UI",
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

		ctx := context.Background()

		// Connect to DBs
		sourceDriver, err := db.NewDriver(source)
		if err != nil {
			return fmt.Errorf("failed to initialize source driver: %w", err)
		}
		err = sourceDriver.Connect(ctx, source, cfg.SSLMode, cfg.Timeout)
		if err != nil {
			return fmt.Errorf("failed to connect to source: %w", err)
		}
		defer sourceDriver.Close()

		targetDriver, err := db.NewDriver(target)
		if err != nil {
			return fmt.Errorf("failed to initialize target driver: %w", err)
		}
		err = targetDriver.Connect(ctx, target, cfg.SSLMode, cfg.Timeout)
		if err != nil {
			return fmt.Errorf("failed to connect to target: %w", err)
		}
		defer targetDriver.Close()

		fmt.Println("Introspecting schemas... Please wait.")
		sourceSchema, err := sourceDriver.Introspect(ctx, cfg.Schema, cfg.IgnorePatterns)
		if err != nil {
			return fmt.Errorf("failed to introspect source: %w", err)
		}

		targetSchema, err := targetDriver.Introspect(ctx, cfg.Schema, cfg.IgnorePatterns)
		if err != nil {
			return fmt.Errorf("failed to introspect target: %w", err)
		}

		differences := diff.NewDiffEngine(sourceSchema, targetSchema, sourceDriver.NormalizeType).Compare()

		if len(differences) == 0 {
			logging.Info("Both schemas are perfectly equivalent! No differences found.")
			return nil
		}

		// Initialize Bubble Tea Interactive CLI TUI
		m := tui.InitialModel(differences)
		p := tea.NewProgram(&m, tea.WithAltScreen())

		_, modelErr := p.Run()
		if modelErr != nil {
			return fmt.Errorf("tui runtime error: %w", modelErr)
		}

		// Upon UI completion check if user confirmed explicitly via Enter key
		if !m.IsConfirmed {
			fmt.Println("\nMigration Cancelled.")
			return nil
		}

		finalDiffs := m.GetSelectedDiffs()
		if len(finalDiffs) == 0 {
			fmt.Println("\nNo items selected. Exiting.")
			return nil
		}

		// Proceed to format SQL output from actively filtered diffs
		formatterOpts := output.FormatOptions{
			Format:       "sql",
			IsExecutable: cfg.Executable,
		}
		formatter := output.NewFormatterWithOptions(formatterOpts)
		migrationSQL, formatErr := formatter.FormatMigration(finalDiffs, cfg.Transaction)
		if formatErr != nil {
			return fmt.Errorf("failed to generate selective migration text: %w", formatErr)
		}

		header := fmt.Sprintf("-- DBDiff Interactive Migration\n-- Source: %s\n-- Target: %s\n-- Schema: %s\n\n", source, target, cfg.Schema)

		var fullSQL string
		if cfg.Executable {
			fullSQL = migrationSQL
		} else {
			fullSQL = header + migrationSQL
		}

		if cfg.Output == "" || cfg.Output == "stdout" {
			fmt.Println("\nSelected Execution SQL Block:\n" + strings.Repeat("-", 40) + "\n\n" + fullSQL)
			return nil
		}

		if err := output.WriteFile(cfg.Output, fullSQL); err != nil {
			return fmt.Errorf("failed to save interactive migration: %w", err)
		}

		fmt.Printf("\nSelected migration output stored securely to: %s\n", cfg.Output)
		return nil
	},
}

func init() {
	InteractiveCmd.Flags().BoolP("executable", "x", false, "Generate an executable bash migration script structure for exported outputs")
}
