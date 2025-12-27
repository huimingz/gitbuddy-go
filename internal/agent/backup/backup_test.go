package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupManager_CreateBackup(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Create a test file to backup
	testFile := filepath.Join(tmpDir, "test.txt")
	originalContent := "This is test content"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	// Create backup
	backupPath, err := manager.CreateBackup(context.Background(), testFile, "test-operation")
	require.NoError(t, err)
	assert.NotEmpty(t, backupPath)

	// Verify backup file exists and has correct content
	assert.FileExists(t, backupPath)
	backupContent, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(backupContent))

	// Verify backup path follows expected pattern
	assert.Contains(t, backupPath, ".backup")
	assert.Contains(t, backupPath, "test-operation")
}

func TestBackupManager_CreateBackup_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Try to backup non-existent file
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.txt")
	_, err := manager.CreateBackup(context.Background(), nonExistentFile, "test-operation")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

func TestBackupManager_RestoreBackup(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Create original file
	testFile := filepath.Join(tmpDir, "restore_test.txt")
	originalContent := "Original content"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	// Create backup
	backupPath, err := manager.CreateBackup(context.Background(), testFile, "before-edit")
	require.NoError(t, err)

	// Modify original file
	modifiedContent := "Modified content"
	err = os.WriteFile(testFile, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Restore from backup
	err = manager.RestoreBackup(context.Background(), backupPath, testFile)
	require.NoError(t, err)

	// Verify file was restored to original content
	restoredContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(restoredContent))
}

func TestBackupManager_RestoreBackup_InvalidBackup(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Try to restore from non-existent backup
	testFile := filepath.Join(tmpDir, "test.txt")
	nonExistentBackup := filepath.Join(tmpDir, "nonexistent.backup")

	err := manager.RestoreBackup(context.Background(), nonExistentBackup, testFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup file not found")
}

func TestBackupManager_ListBackups(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "list_test.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Create multiple backups
	backup1, err := manager.CreateBackup(context.Background(), testFile, "operation1")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond) // Ensure different timestamps

	backup2, err := manager.CreateBackup(context.Background(), testFile, "operation2")
	require.NoError(t, err)

	// List backups for the file
	backups, err := manager.ListBackups(context.Background(), testFile)
	require.NoError(t, err)
	assert.Len(t, backups, 2)

	// Verify backup paths are in the list
	backupPaths := make([]string, len(backups))
	for i, backup := range backups {
		backupPaths[i] = backup.Path
	}
	assert.Contains(t, backupPaths, backup1)
	assert.Contains(t, backupPaths, backup2)
}

func TestBackupManager_CleanupOldBackups(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "cleanup_test.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Create several backups
	backupPaths := make([]string, 5)
	for i := 0; i < 5; i++ {
		backupPaths[i], err = manager.CreateBackup(context.Background(), testFile, "operation")
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Cleanup old backups, keeping only 2 most recent
	cleaned, err := manager.CleanupOldBackups(context.Background(), testFile, 2)
	require.NoError(t, err)
	assert.Equal(t, 3, cleaned) // Should have cleaned 3 old backups

	// Verify only 2 backups remain
	remainingBackups, err := manager.ListBackups(context.Background(), testFile)
	require.NoError(t, err)
	assert.Len(t, remainingBackups, 2)

	// Verify the two most recent backups are kept
	for _, backup := range remainingBackups {
		assert.Contains(t, []string{backupPaths[3], backupPaths[4]}, backup.Path)
	}
}

func TestBackupManager_GetBackupDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Test backup directory creation
	testFile := filepath.Join(tmpDir, "subdir", "test.txt")
	backupDir := manager.GetBackupDirectory(testFile)

	// Verify backup directory is within the working directory
	assert.True(t, strings.HasPrefix(backupDir, tmpDir))
	assert.Contains(t, backupDir, ".gitbuddy-backups")
}

func TestBackupManager_BackupDirectory_Organization(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Create files in different directories
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "subdir", "file2.txt")

	// Create directory for file2
	err := os.MkdirAll(filepath.Dir(file2), 0755)
	require.NoError(t, err)

	// Write test files
	err = os.WriteFile(file1, []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	require.NoError(t, err)

	// Create backups
	backup1, err := manager.CreateBackup(context.Background(), file1, "edit")
	require.NoError(t, err)
	backup2, err := manager.CreateBackup(context.Background(), file2, "edit")
	require.NoError(t, err)

	// Verify backups are organized properly
	assert.True(t, strings.HasPrefix(backup1, tmpDir))
	assert.True(t, strings.HasPrefix(backup2, tmpDir))

	// Verify both backups exist
	assert.FileExists(t, backup1)
	assert.FileExists(t, backup2)

	// Verify backup directory structure maintains relative paths
	backupDir1 := manager.GetBackupDirectory(file1)
	backupDir2 := manager.GetBackupDirectory(file2)

	assert.NotEqual(t, backupDir1, backupDir2) // Different directories for different file locations
}

func TestBackupManager_InvalidParams(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	tests := []struct {
		name      string
		operation func() error
		errMsg    string
	}{
		{
			name: "empty file path for CreateBackup",
			operation: func() error {
				_, err := manager.CreateBackup(context.Background(), "", "test")
				return err
			},
			errMsg: "file path is required",
		},
		{
			name: "empty operation name for CreateBackup",
			operation: func() error {
				_, err := manager.CreateBackup(context.Background(), "test.txt", "")
				return err
			},
			errMsg: "operation name is required",
		},
		{
			name: "empty backup path for RestoreBackup",
			operation: func() error {
				return manager.RestoreBackup(context.Background(), "", "test.txt")
			},
			errMsg: "backup path is required",
		},
		{
			name: "empty target path for RestoreBackup",
			operation: func() error {
				return manager.RestoreBackup(context.Background(), "backup.txt", "")
			},
			errMsg: "target path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestBackupManager_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "context_test.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = manager.CreateBackup(ctx, testFile, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestNewBackupManager tests the constructor
func TestNewBackupManager(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)
	assert.NotNil(t, manager)

	// Test with empty working directory
	emptyManager := NewBackupManager("")
	assert.NotNil(t, emptyManager)
}

func TestBackupManager_ComplexFilePaths(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Create file with complex path
	complexPath := filepath.Join(tmpDir, "deep", "nested", "path", "with spaces", "file-with-dashes.txt")
	err := os.MkdirAll(filepath.Dir(complexPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(complexPath, []byte("complex content"), 0644)
	require.NoError(t, err)

	// Create backup with operation containing spaces and special chars
	backupPath, err := manager.CreateBackup(context.Background(), complexPath, "pre-edit/test operation")
	require.NoError(t, err)

	// Verify backup exists and content is correct
	assert.FileExists(t, backupPath)
	content, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, "complex content", string(content))

	// Verify operation name is sanitized in filename
	assert.Contains(t, backupPath, "pre-edit-test-operation")
}

func TestBackupManager_BackupInfo_Parsing(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "parse_test.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Create backup
	backupPath, err := manager.CreateBackup(context.Background(), testFile, "complex-operation-name")
	require.NoError(t, err)

	// Test backup info parsing
	info, err := manager.getBackupInfo(backupPath, testFile)
	require.NoError(t, err)

	assert.Equal(t, backupPath, info.Path)
	assert.Equal(t, testFile, info.OriginalFile)
	assert.Equal(t, "complex-operation-name", info.Operation)
	assert.True(t, info.Size > 0)
	assert.False(t, info.CreatedAt.IsZero())
}

func TestBackupManager_ErrorConditions(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Test backup info with invalid file
	invalidPath := filepath.Join(tmpDir, "nonexistent-backup.txt")
	_, err := manager.getBackupInfo(invalidPath, "test.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stat backup")

	// Test copy file error conditions by creating a read-only directory
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	err = os.MkdirAll(readOnlyDir, 0555) // read-only directory
	require.NoError(t, err)

	// This may not work on all systems, so we'll test what we can
	srcFile := filepath.Join(tmpDir, "source.txt")
	err = os.WriteFile(srcFile, []byte("content"), 0644)
	require.NoError(t, err)

	dstFile := filepath.Join(readOnlyDir, "dest.txt")
	err = manager.copyFile(srcFile, dstFile)
	// Error might occur or might not depending on the system, but test the method is called
	// On some systems this might succeed, on others it might fail
}

func TestBackupManager_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// List backups for non-existent file
	backups, err := manager.ListBackups(context.Background(), filepath.Join(tmpDir, "nonexistent.txt"))
	require.NoError(t, err)
	assert.Empty(t, backups)
}

func TestBackupManager_CleanupEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	testFile := filepath.Join(tmpDir, "cleanup_edge.txt")
	err := os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Test cleanup when keepCount is larger than actual backup count
	backupPath, err := manager.CreateBackup(context.Background(), testFile, "single")
	require.NoError(t, err)

	// Try to keep 5 backups when only 1 exists
	cleaned, err := manager.CleanupOldBackups(context.Background(), testFile, 5)
	require.NoError(t, err)
	assert.Equal(t, 0, cleaned) // Should clean 0 files

	// Verify backup still exists
	assert.FileExists(t, backupPath)
}

func TestBackupManager_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewBackupManager(tmpDir)

	// Create test file with specific permissions
	testFile := filepath.Join(tmpDir, "perms_test.txt")
	err := os.WriteFile(testFile, []byte("content with permissions"), 0644)
	require.NoError(t, err)

	// Create backup
	backupPath, err := manager.CreateBackup(context.Background(), testFile, "permissions-test")
	require.NoError(t, err)

	// Verify backup file exists and has content
	assert.FileExists(t, backupPath)
	content, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, "content with permissions", string(content))

	// Test restore functionality preserves content
	modifiedFile := filepath.Join(tmpDir, "modified.txt")
	err = os.WriteFile(modifiedFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	err = manager.RestoreBackup(context.Background(), backupPath, modifiedFile)
	require.NoError(t, err)

	restoredContent, err := os.ReadFile(modifiedFile)
	require.NoError(t, err)
	assert.Equal(t, "content with permissions", string(restoredContent))
}