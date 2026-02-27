package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/meru143/dbdiff/internal/config"
	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/diff"
	"github.com/meru143/dbdiff/internal/logging"
	"github.com/meru143/dbdiff/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var CompareCmd = &cobra.Command{
	Use:   "compare [source] [target]",
	Short: "Compare two database schemas",
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

		logging.Info(fmt.Sprintf("Comparing schemas: %s -> %s", source, target))

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

		formatter := output.NewFormatter(cfg.Format)
		outputStr, err := formatter.Format(differences)
		if err != nil {
			return fmt.Errorf("failed to format output: %w", err)
		}

		if cfg.Output == "" || cfg.Output == "stdout" {
			fmt.Println(outputStr)
		} else {
			if err := output.WriteFile(cfg.Output, outputStr); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
			logging.Info(fmt.Sprintf("Output written to: %s", cfg.Output))
		}

		return nil
	},
}

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	flags := make(map[string]interface{})
	cmd.Flags().VisitAll(func(f *cobra.Flag) {
		if f.Changed {
			flags[f.Name] = f.Value.String()
		}
	})
	return config.Load(flags)
}

func getConfigValue(args []string, index int, flagVal string, flagName string) string {
	if len(args) > index {
		return args[index]
	}
	if flagVal != "" {
		return flagVal
	}
	return viper.GetString(flagName)
}
