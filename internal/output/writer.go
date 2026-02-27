package output

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/meru143/dbdiff/internal/config"
)

// Writer handles writing diff output to various destinations
type Writer struct {
	config *config.Config
}

// NewWriter creates a new Writer instance
func NewWriter(cfg *config.Config) *Writer {
	return &Writer{config: cfg}
}

// Write writes content to the configured output destination
func (w *Writer) Write(content string) error {
	if w.config.Output == "" || w.config.Output == "stdout" {
		fmt.Print(content)
		return nil
	}

	// Ensure parent directory exists
	dir := filepath.Dir(w.config.Output)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to file
	return os.WriteFile(w.config.Output, []byte(content), 0644)
}

// WriteDiff writes a diff to the configured output
func (w *Writer) WriteDiff(diff string) error {
	return w.Write(diff)
}

// Backup creates a backup of the target file before overwriting
func (w *Writer) Backup(path string) (string, error) {
	if path == "" || path == "stdout" {
		return "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read file for backup: %w", err)
	}

	// Create backup with timestamp
	backupPath := fmt.Sprintf("%s.backup", path)
	err = os.WriteFile(backupPath, data, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupPath, nil
}

// EnsureStdout ensures we're writing to stdout if no output file specified
func EnsureStdout(w io.Writer, outputPath string) io.Writer {
	if outputPath == "" || outputPath == "stdout" {
		return os.Stdout
	}
	return w
}
