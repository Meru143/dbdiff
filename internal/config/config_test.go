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
