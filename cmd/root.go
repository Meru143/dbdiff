package cmd

import (
	"context"

	"github.com/meru143/dbdiff/internal/config"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "root",
	Short: "Root command",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// GetConfig returns the config from context
func GetConfig(ctx context.Context) *config.Config {
	if cfg, ok := ctx.Value("config").(*config.Config); ok {
		return cfg
	}
	return &config.Config{}
}
