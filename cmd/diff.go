package cmd

import (
	"context"
	"fmt"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/diff"
	"github.com/meru143/dbdiff/internal/logging"
	"github.com/meru143/dbdiff/internal/output"
	"github.com/spf13/cobra"
)

var DiffCmd = &cobra.Command{
	Use:   "diff [source] [target]",
	Short: "Show schema differences in table format",
	Args:  cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cmd)
		if err != nil {
			return err
		}

		logging.Init(cfg.Debug, cfg.Verbose)
		cfg.Format = "table"

		source := getConfigValue(args, 0, cfg.Source, "source")
		target := getConfigValue(args, 1, cfg.Target, "target")

		if source == "" || target == "" {
			return fmt.Errorf("source and target database URLs are required")
		}

		logging.Info(fmt.Sprintf("Comparing: %s -> %s", source, target))

		ctx := context.Background()

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
			logging.Info("No differences found!")
			return nil
		}

		logging.Info(fmt.Sprintf("Found %d differences", len(differences)))

		formatter := output.NewFormatter("table")
		outputStr, err := formatter.Format(differences)
		if err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}

		fmt.Println(outputStr)
		return nil
	},
}
