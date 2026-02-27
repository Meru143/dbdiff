package safety

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BackupManager handles creating and managing backups
type BackupManager struct {
	backupDir string
	maxBackups int
}

// NewBackupManager creates a new backup manager
func NewBackupManager(backupDir string, maxBackups int) *BackupManager {
	if maxBackups == 0 {
		maxBackups = 5
	}
	return &BackupManager{
		backupDir:  backupDir,
		maxBackups: maxBackups,
	}
}

// Backup creates a backup of the given content
func (bm *BackupManager) Backup(name string, content []byte) (string, error) {
	if bm.backupDir == "" {
		return "", nil
	}

	// Create backup directory if needed
	if err := os.MkdirAll(bm.backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s.sql", name, timestamp)
	backupPath := filepath.Join(bm.backupDir, backupName)

	// Write backup
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	// Prune old backups
	if err := bm.Prune(name); err != nil {
		return "", fmt.Errorf("failed to prune old backups: %w", err)
	}

	return backupPath, nil
}

// Prune removes old backups, keeping only maxBackups
func (bm *BackupManager) Prune(name string) error {
	if bm.backupDir == "" || bm.maxBackups <= 0 {
		return nil
	}

	// List matching backups
	pattern := fmt.Sprintf("%s_*.sql", name)
	matches, err := filepath.Glob(filepath.Join(bm.backupDir, pattern))
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	// Sort by modification time (oldest first)
	if len(matches) <= bm.maxBackups {
		return nil
	}

	// Remove oldest backups
	toRemove := matches[:len(matches)-bm.maxBackups]
	for _, path := range toRemove {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove old backup %s: %w", path, err)
		}
	}

	return nil
}

// ListBackups lists all backups for a given name
func (bm *BackupManager) ListBackups(name string) ([]string, error) {
	if bm.backupDir == "" {
		return nil, nil
	}

	pattern := fmt.Sprintf("%s_*.sql", name)
	matches, err := filepath.Glob(filepath.Join(bm.backupDir, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	return matches, nil
}

// RestorePath returns the path to restore from a backup
func (bm *BackupManager) RestorePath(backupPath string) ([]byte, error) {
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("backup not found: %s", backupPath)
	}

	return os.ReadFile(backupPath)
}
