package safety

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupManager_NewBackupManager(t *testing.T) {
	tmpDir := t.TempDir()
	bm := NewBackupManager(tmpDir, 3)

	if bm == nil {
		t.Fatal("Expected backup manager, got nil")
	}
}

func TestBackupManager_Backup(t *testing.T) {
	tmpDir := t.TempDir()
	bm := NewBackupManager(tmpDir, 3)

	content := []byte("test migration content")
	backupPath, err := bm.Backup("test", content)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("Backup file not created")
	}

	// Check content
	readContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup: %v", err)
	}
	if string(readContent) != string(content) {
		t.Error("Backup content mismatch")
	}
}

func TestBackupManager_RestorePath(t *testing.T) {
	tmpDir := t.TempDir()
	bm := NewBackupManager(tmpDir, 3)

	backupPath, _ := bm.Backup("test", []byte("original content"))
	restored, err := bm.RestorePath(backupPath)
	if err != nil {
		t.Fatalf("RestorePath failed: %v", err)
	}
	if string(restored) != "original content" {
		t.Error("Restored content mismatch")
	}
}

func TestBackupManager_CustomPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	bm := NewBackupManager(tmpDir, 3)

	backupPath, _ := bm.Backup("custom", []byte("data"))
	if !strings.HasPrefix(filepath.Base(backupPath), "custom") {
		t.Error("Backup path should have custom prefix")
	}
}
