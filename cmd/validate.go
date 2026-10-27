package cmd

import (
	"context"
	"fmt"

	"github.com/meru143/dbdiff/internal/db"
	"github.com/meru143/dbdiff/internal/db/dsn"
	"github.com/meru143/dbdiff/internal/logging"
	"github.com/spf13/cobra"
)

var ValidateCmd = &cobra.Command{
	Use:   "validate [database]",
	Short: "Validate database connection",
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

		ctx := context.Background()
		driver, err := db.NewDriver(database)
		if err != nil {
			return fmt.Errorf("failed to initialize driver: %w", err)
		}
		// Printed once NewDriver has taken it for a URL: Redact finds a password by the
		// URL's scheme.
		fmt.Printf("Validating connection to: %s\n", dsn.Redact(database))

		err = driver.Connect(ctx, database, cfg.SSLMode, cfg.Timeout)
		if err != nil {
			fmt.Printf("❌ Connection failed: %v\n", dsn.Redact(err.Error()))
			return err
		}
		defer driver.Close()

		info, err := driver.GetServerInfo(ctx)
		if err != nil {
			fmt.Printf("⚠️  Connected but failed to get server info: %v\n", dsn.Redact(err.Error()))
		} else {
			fmt.Printf("✅ Connection successful!\n\n")
			fmt.Printf("Server Information:\n")
			fmt.Printf("  Version: %s\n", info.Version)
			fmt.Printf("  Database: %s\n", info.Database)
			fmt.Printf("  User: %s\n", info.User)
		}

		logging.Info("Connection validated successfully")
		return nil
	},
}
