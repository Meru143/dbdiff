package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/meru143/dbdiff/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// customUsageTemplate adds custom usage template
var customUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}

{{if .HasAvailableSubCommands}}
For more information, visit: https://github.com/meru143/dbdiff{{end}}
`

// customHelpTemplate adds custom help template
var customHelpTemplate = `{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}

Report bugs at: https://github.com/meru143/dbdiff/issues
`

func init() {
	// Set up Viper configuration
	viper.SetConfigName("dbdiff")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.dbdiff")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("DBDIFF")
	viper.AutomaticEnv()

	// Bind flags to viper with error checking
	_ = viper.BindPFlag("source", rootCmd.PersistentFlags().Lookup("source"))
	_ = viper.BindPFlag("target", rootCmd.PersistentFlags().Lookup("target"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	_ = viper.BindPFlag("format", rootCmd.PersistentFlags().Lookup("format"))
	_ = viper.BindPFlag("dry-run", rootCmd.PersistentFlags().Lookup("dry-run"))
	_ = viper.BindPFlag("force", rootCmd.PersistentFlags().Lookup("force"))
	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("ignore-patterns", rootCmd.PersistentFlags().Lookup("ignore-patterns"))
	_ = viper.BindPFlag("schema", rootCmd.PersistentFlags().Lookup("schema"))
	_ = viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	_ = viper.BindPFlag("transaction", rootCmd.PersistentFlags().Lookup("transaction"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("log-json", rootCmd.PersistentFlags().Lookup("log-json"))
	_ = viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("ssl-mode", rootCmd.PersistentFlags().Lookup("ssl-mode"))
	_ = viper.BindPFlag("backup-dir", rootCmd.PersistentFlags().Lookup("backup-dir"))
	_ = viper.BindPFlag("max-backups", rootCmd.PersistentFlags().Lookup("max-backups"))
	_ = viper.BindPFlag("protected-objects", rootCmd.PersistentFlags().Lookup("protected-objects"))

	// Set custom templates
	rootCmd.SetUsageTemplate(customUsageTemplate)
	rootCmd.SetHelpTemplate(customHelpTemplate)

	// Add PersistentPreRun for setup
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Load config file if specified
		if cfg := viper.GetString("config"); cfg != "" {
			viper.SetConfigFile(cfg)
			if err := viper.ReadInConfig(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to read config file: %v\n", err)
			}
		}

		// Set up colored output if terminal supports it
		if isTerminal() {
			cmd.SetOut(os.Stdout)
		}
	}

	// Add PersistentPostRun for cleanup
	rootCmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {
		// Any cleanup needed
	}

	// Update version with build date if set
	if buildDate != "unknown" {
		rootCmd.Version = fmt.Sprintf("%s (commit: %s, date: %s)", version, commit, buildDate)
	}
}

// isTerminal checks if output is a terminal
func isTerminal() bool {
	return isTerminalFunc(os.Stdout)
}

// isTerminalFunc is a variable for testing
var isTerminalFunc func(*os.File) bool = func(f *os.File) bool {
	return isatty(f.Fd())
}

// isatty checks if file descriptor is a TTY
func isatty(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

var rootCmd = &cobra.Command{
	Use:     "dbdiff",
	Short:   "PostgreSQL schema comparison and migration tool",
	Version: fmt.Sprintf("%s (commit: %s, date: %s)", version, commit, buildDate),
}

func main() {
	// Add persistent flags
	rootCmd.PersistentFlags().StringP("source", "s", "", "Source database URL")
	rootCmd.PersistentFlags().StringP("target", "t", "", "Target database URL")
	rootCmd.PersistentFlags().StringP("output", "o", "", "Output file path")
	rootCmd.PersistentFlags().String("format", "sql", "Output format: sql, table, json")
	rootCmd.PersistentFlags().Bool("dry-run", true, "Dry-run mode")
	rootCmd.PersistentFlags().BoolP("force", "f", false, "Skip confirmation prompt")
	rootCmd.PersistentFlags().StringP("config", "c", "", "Config file path")
	rootCmd.PersistentFlags().StringSlice("ignore-patterns", nil, "Columns to ignore")
	rootCmd.PersistentFlags().String("schema", "public", "Schema to compare")
	rootCmd.PersistentFlags().Duration("timeout", 30, "Query timeout")
	rootCmd.PersistentFlags().Bool("transaction", true, "Wrap in transaction")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().Bool("debug", false, "Debug mode")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level: debug, info, warn, error")
	rootCmd.PersistentFlags().Bool("log-json", false, "Output logs in JSON format")
	rootCmd.PersistentFlags().String("log-file", "", "Log to file (default: stdout)")
	rootCmd.PersistentFlags().String("ssl-mode", "disable", "SSL mode")
	rootCmd.PersistentFlags().String("backup-dir", "", "Backup directory for migrations")
	rootCmd.PersistentFlags().Int("max-backups", 5, "Maximum number of backups to keep")
	rootCmd.PersistentFlags().StringSlice("protected-objects", nil, "Protected objects (skip in migration)")

	// Add subcommands
	rootCmd.AddCommand(cmd.CompareCmd)
	rootCmd.AddCommand(cmd.MigrateCmd)
	rootCmd.AddCommand(cmd.DiffCmd)
	rootCmd.AddCommand(cmd.TablesCmd)
	rootCmd.AddCommand(cmd.ValidateCmd)
	rootCmd.AddCommand(cmd.ApplyCmd)
	rootCmd.AddCommand(cmd.HistoryCmd)
	rootCmd.AddCommand(cmd.RollbackCmd)
	rootCmd.AddCommand(cmd.InteractiveCmd)

	// Add completion command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			switch args[0] {
			case "bash":
				err = rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				err = rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				err = rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				err = rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating completion: %v\n", err)
				os.Exit(1)
			}
		},
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
