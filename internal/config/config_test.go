package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Output != "stdout" {
		t.Errorf("Expected output 'stdout', got %s", cfg.Output)
	}
	if cfg.Format != "sql" {
		t.Errorf("Expected format 'sql', got %s", cfg.Format)
	}
	if cfg.DryRun != true {
		t.Errorf("Expected dry-run true, got %v", cfg.DryRun)
	}
	if cfg.Schema != "public" {
		t.Errorf("Expected schema 'public', got %s", cfg.Schema)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", cfg.Timeout)
	}
	if cfg.Transaction != true {
		t.Errorf("Expected transaction true, got %v", cfg.Transaction)
	}
	if cfg.Verbose != false {
		t.Errorf("Expected verbose false, got %v", cfg.Verbose)
	}
	if cfg.Debug != false {
		t.Errorf("Expected debug false, got %v", cfg.Debug)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("Expected ssl-mode 'disable', got %s", cfg.SSLMode)
	}
}

func TestLoad_FlagBinding(t *testing.T) {
	flags := map[string]interface{}{
		"source":   "postgres://user:pass@localhost/db",
		"target":   "postgres://user:pass@localhost/db2",
		"output":   "migration.sql",
		"format":   "json",
		"dry-run":  "false",
		"schema":   "myschema",
		"timeout":  "60s",
		"transaction": "false",
		"verbose":  "true",
		"debug":    "true",
		"ssl-mode": "require",
	}

	cfg, err := Load(flags)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Source != "postgres://user:pass@localhost/db" {
		t.Errorf("Expected source, got %s", cfg.Source)
	}
	if cfg.Target != "postgres://user:pass@localhost/db2" {
		t.Errorf("Expected target, got %s", cfg.Target)
	}
	if cfg.Output != "migration.sql" {
		t.Errorf("Expected output, got %s", cfg.Output)
	}
	if cfg.Format != "json" {
		t.Errorf("Expected format, got %s", cfg.Format)
	}
	if cfg.DryRun != false {
		t.Errorf("Expected dry-run false, got %v", cfg.DryRun)
	}
	if cfg.Schema != "myschema" {
		t.Errorf("Expected schema, got %s", cfg.Schema)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", cfg.Timeout)
	}
	if cfg.Transaction != false {
		t.Errorf("Expected transaction false, got %v", cfg.Transaction)
	}
	if cfg.Verbose != true {
		t.Errorf("Expected verbose true, got %v", cfg.Verbose)
	}
	if cfg.Debug != true {
		t.Errorf("Expected debug true, got %v", cfg.Debug)
	}
	if cfg.SSLMode != "require" {
		t.Errorf("Expected ssl-mode require, got %s", cfg.SSLMode)
	}
}

func TestLoad_ConfigFile(t *testing.T) {
	// Create temp config file
	content := []byte(`
source: postgres://user:pass@localhost/db
target: postgres://user:pass@localhost/db2
schema: testschema
`)
	tmpFile, err := os.CreateTemp("", "dbdiff-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	flags := map[string]interface{}{
		"config": tmpFile.Name(),
	}

	cfg, err := Load(flags)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Schema != "testschema" {
		t.Errorf("Expected schema from config file, got %s", cfg.Schema)
	}
}

func TestLoad_IgnorePatterns(t *testing.T) {
	flags := map[string]interface{}{
		"ignore-patterns": []string{"*_created_at", "*_updated_at"},
	}

	cfg, err := Load(flags)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.IgnorePatterns) != 2 {
		t.Errorf("Expected 2 ignore patterns, got %d", len(cfg.IgnorePatterns))
	}
	if cfg.IgnorePatterns[0] != "*_created_at" {
		t.Errorf("Expected *_created_at, got %s", cfg.IgnorePatterns[0])
	}
}

func TestLoad_BackupConfig(t *testing.T) {
	flags := map[string]interface{}{
		"backup-dir":       "/tmp/backups",
		"max-backups":      "10",
		"protected-objects": []string{"users", "accounts"},
	}

	cfg, err := Load(flags)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.BackupDir != "/tmp/backups" {
		t.Errorf("Expected backup-dir, got %s", cfg.BackupDir)
	}
	if cfg.MaxBackups != 10 {
		t.Errorf("Expected max-backups 10, got %d", cfg.MaxBackups)
	}
	if len(cfg.ProtectedObjects) != 2 {
		t.Errorf("Expected 2 protected objects, got %d", len(cfg.ProtectedObjects))
	}
}

func TestLoad_FromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("DBDIFF_SOURCE", "postgres://envuser:envpass@localhost/envdb")
	os.Setenv("DBDIFF_SCHEMA", "envschema")
	os.Setenv("DBDIFF_TIMEOUT", "60s")
	defer os.Unsetenv("DBDIFF_SOURCE")
	defer os.Unsetenv("DBDIFF_SCHEMA")
	defer os.Unsetenv("DBDIFF_TIMEOUT")

	cfg, err := Load(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Source != "postgres://envuser:envpass@localhost/envdb" {
		t.Errorf("Expected source from env, got %s", cfg.Source)
	}
	if cfg.Schema != "envschema" {
		t.Errorf("Expected schema from env, got %s", cfg.Schema)
	}
}

func TestLoad_Validation(t *testing.T) {
	// Test that empty source/target is allowed (commands handle validation)
	cfg, err := Load(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Default values should be set
	if cfg.Timeout == 0 {
		t.Errorf("Expected timeout to be set")
	}
	if cfg.MaxBackups == 0 {
		t.Errorf("Expected max-backups to have default")
	}
}

func TestLoad_AllFlags(t *testing.T) {
	flags := map[string]interface{}{
		"source":            "postgres://a:b@localhost:5432/db",
		"target":            "postgres://c:d@localhost:5432/db2",
		"output":           "/tmp/out.sql",
		"format":           "table",
		"dry-run":          "false",
		"force":            "true",
		"schema":           "custom",
		"timeout":          "120s",
		"transaction":      "false",
		"verbose":          "true",
		"debug":            "true",
		"ssl-mode":         "require",
		"backup-dir":       "/backups",
		"max-backups":      "20",
		"protected-objects": []string{"critical_table"},
	}

	cfg, err := Load(flags)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify all values
	tests := []struct {
		got, want interface{}
		name     string
	}{
		{cfg.Source, "postgres://a:b@localhost:5432/db", "source"},
		{cfg.Target, "postgres://c:d@localhost:5432/db2", "target"},
		{cfg.Output, "/tmp/out.sql", "output"},
		{cfg.Format, "table", "format"},
		{cfg.DryRun, false, "dry-run"},
		{cfg.Force, true, "force"},
		{cfg.Schema, "custom", "schema"},
		{cfg.Timeout.Seconds(), 120.0, "timeout"},
		{cfg.Transaction, false, "transaction"},
		{cfg.Verbose, true, "verbose"},
		{cfg.Debug, true, "debug"},
		{cfg.SSLMode, "require", "ssl-mode"},
		{cfg.BackupDir, "/backups", "backup-dir"},
		{cfg.MaxBackups, 20, "max-backups"},
		{len(cfg.ProtectedObjects), 1, "protected-objects"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("Expected %s = %v, got %v", tt.name, tt.want, tt.got)
		}
	}
}
