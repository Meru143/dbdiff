package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Source          string
	Target          string
	Output          string
	Format          string
	DryRun          bool
	Force           bool
	ConfigFile      string
	IgnorePatterns  []string
	Schema          string
	Timeout         time.Duration
	Transaction     bool
	Verbose         bool
	Debug           bool
	SSLMode         string
	BackupDir       string
	MaxBackups      int
	ProtectedObjects []string
}

func Load(flags map[string]interface{}) (*Config, error) {
	v := viper.New()

	v.SetDefault("output", "stdout")
	v.SetDefault("format", "sql")
	v.SetDefault("dry-run", true)
	v.SetDefault("schema", "public")
	v.SetDefault("timeout", "30s")
	v.SetDefault("transaction", true)
	v.SetDefault("verbose", false)
	v.SetDefault("debug", false)
	v.SetDefault("ssl-mode", "disable")
	v.SetDefault("backup-dir", "")
	v.SetDefault("max-backups", 5)
	v.SetDefault("protected-objects", []string{})

	for key, value := range flags {
		if value != nil {
			v.Set(key, value)
		}
	}

	if configFile, ok := flags["config"].(string); ok && configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	v.SetEnvPrefix("DBDIFF")
	v.AutomaticEnv()

	timeoutStr := v.GetString("timeout")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 30 * time.Second
	}

	return &Config{
		Source:          v.GetString("source"),
		Target:          v.GetString("target"),
		Output:          v.GetString("output"),
		Format:          v.GetString("format"),
		DryRun:          v.GetBool("dry-run"),
		Force:           v.GetBool("force"),
		ConfigFile:      v.GetString("config"),
		IgnorePatterns:  v.GetStringSlice("ignore-patterns"),
		Schema:          v.GetString("schema"),
		Timeout:         timeout,
		Transaction:     v.GetBool("transaction"),
		Verbose:         v.GetBool("verbose"),
		Debug:           v.GetBool("debug"),
		SSLMode:         v.GetString("ssl-mode"),
		BackupDir:       v.GetString("backup-dir"),
		MaxBackups:      v.GetInt("max-backups"),
		ProtectedObjects: v.GetStringSlice("protected-objects"),
	}, nil
}
