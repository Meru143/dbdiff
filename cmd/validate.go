package cmd

import (
	"context"
	"fmt"

	"github.com/meru143/dbdiff/internal/db"
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

		fmt.Printf("Validating connection to: %s\n", database)

		ctx := context.Background()
		conn, err := db.Connect(ctx, database, cfg.SSLMode, cfg.Timeout)
		if err != nil {
			fmt.Printf("❌ Connection failed: %v\n", err)
			return err
		}
		defer conn.Close()

		info, err := db.GetServerInfo(ctx, conn)
		if err != nil {
			fmt.Printf("⚠️  Connected but failed to get server info: %v\n", err)
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
