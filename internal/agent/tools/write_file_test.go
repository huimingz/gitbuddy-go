package tools

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

func TestWriteFileTool_Name(t *testing.T) {
	tool := NewWriteFileTool("/tmp")
	assert.Equal(t, "write_file", tool.Name())
}

func TestWriteFileTool_Description(t *testing.T) {
	tool := NewWriteFileTool("/tmp")
	desc := tool.Description()
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "write")
	assert.Contains(t, desc, "file")
	assert.Contains(t, desc, "file_path")
	assert.Contains(t, desc, "content")
}

func TestWriteFileTool_CreateNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)

	params := &WriteFileParams{
		FilePath: "test.txt",
		Content:  "Hello, World!\nThis is a test file.",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully written")
	assert.Contains(t, result, "test.txt")

	// Verify file was created with correct content
	filePath := filepath.Join(tmpDir, "test.txt")
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!\nThis is a test file.", string(content))

	// Verify file info
	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
	assert.Equal(t, int64(len("Hello, World!\nThis is a test file.")), info.Size())
}

func TestWriteFileTool_OverwriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)

	// Create initial file
	filePath := filepath.Join(tmpDir, "existing.txt")
	err := os.WriteFile(filePath, []byte("Original content"), 0644)
	require.NoError(t, err)

	// Wait a moment to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	params := &WriteFileParams{
		FilePath: "existing.txt",
		Content:  "New content",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully written")
	assert.Contains(t, result, "Backup created")

	// Verify file was overwritten
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "New content", string(content))

	// Verify backup was created in .gitbuddy-backups directory
	backupFiles, err := filepath.Glob(filepath.Join(tmpDir, ".gitbuddy-backups", "existing.txt.backup.*"))
	require.NoError(t, err)
	assert.Len(t, backupFiles, 1)

	// Verify backup has original content
	backupContent, err := os.ReadFile(backupFiles[0])
	require.NoError(t, err)
	assert.Equal(t, "Original content", string(backupContent))
}

func TestWriteFileTool_CreateDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)

	params := &WriteFileParams{
		FilePath: "nested/path/to/file.txt",
		Content:  "Content in nested directory",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully written")

	// Verify directories were created
	nestedPath := filepath.Join(tmpDir, "nested", "path", "to")
	info, err := os.Stat(nestedPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify file was created
	filePath := filepath.Join(tmpDir, "nested", "path", "to", "file.txt")
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "Content in nested directory", string(content))
}

func TestWriteFileTool_PermissionDenied(t *testing.T) {
	// Skip this test on Windows as permission handling is different
	if os.Getenv("GOOS") == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	tmpDir := t.TempDir()

	// Create a read-only directory
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	err := os.Mkdir(readOnlyDir, 0555) // No write permission
	require.NoError(t, err)
	defer os.Chmod(readOnlyDir, 0755) // Restore permissions for cleanup

	tool := NewWriteFileTool(tmpDir)

	params := &WriteFileParams{
		FilePath: "readonly/file.txt",
		Content:  "Should fail",
	}

	result, err := tool.Execute(context.Background(), params)
	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "permission")
}

func TestWriteFileTool_BackupCreation(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)

	// Create initial file with specific content
	filePath := filepath.Join(tmpDir, "backup_test.txt")
	originalContent := "Original content for backup test"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	// Wait to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	params := &WriteFileParams{
		FilePath: "backup_test.txt",
		Content:  "New content replacing original",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "Backup created")

	// Find the backup file in .gitbuddy-backups directory
	backupFiles, err := filepath.Glob(filepath.Join(tmpDir, ".gitbuddy-backups", "backup_test.txt.backup.*"))
	require.NoError(t, err)
	require.Len(t, backupFiles, 1)

	// Verify backup has original content
	backupContent, err := os.ReadFile(backupFiles[0])
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(backupContent))

	// Verify main file has new content
	newContent, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "New content replacing original", string(newContent))

	// Verify backup filename format (should contain operation and timestamp)
	backupName := filepath.Base(backupFiles[0])
	assert.True(t, strings.HasPrefix(backupName, "backup_test.txt.backup.write-file."))
	assert.True(t, len(backupName) > len("backup_test.txt.backup.write-file."))
}

func TestWriteFileTool_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)

	params := &WriteFileParams{
		FilePath: "empty.txt",
		Content:  "",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully written")

	// Verify empty file was created
	filePath := filepath.Join(tmpDir, "empty.txt")
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "", string(content))
}

func TestWriteFileTool_InvalidParams(t *testing.T) {
	tool := NewWriteFileTool("/tmp")

	tests := []struct {
		name   string
		params *WriteFileParams
		errMsg string
	}{
		{
			name:   "nil params",
			params: nil,
			errMsg: "file_path is required",
		},
		{
			name:   "empty file path",
			params: &WriteFileParams{FilePath: "", Content: "test"},
			errMsg: "file_path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), tt.params)
			assert.Error(t, err)
			assert.Empty(t, result)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestWriteFileTool_PathSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)

	tests := []struct {
		name     string
		filePath string
		errMsg   string
	}{
		{
			name:     "path traversal with ../",
			filePath: "../outside.txt",
			errMsg:   "path outside working directory",
		},
		{
			name:     "absolute path outside workdir",
			filePath: "/etc/passwd",
			errMsg:   "path outside working directory",
		},
		{
			name:     "nested path traversal",
			filePath: "nested/../../outside.txt",
			errMsg:   "path outside working directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &WriteFileParams{
				FilePath: tt.filePath,
				Content:  "Should not be written",
			}

			result, err := tool.Execute(context.Background(), params)
			assert.Error(t, err)
			assert.Empty(t, result)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestWriteFileTool_LargeContent(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)

	// Create large content (1MB)
	largeContent := strings.Repeat("This is a test line with some content.\n", 25000) // ~1MB

	params := &WriteFileParams{
		FilePath: "large.txt",
		Content:  largeContent,
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully written")

	// Verify file was created with correct size
	filePath := filepath.Join(tmpDir, "large.txt")
	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, int64(len(largeContent)), info.Size())
}

func TestWriteFileTool_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)

	// Content with various special characters and encodings
	specialContent := "Special characters: áéíóú, 中文, 🚀, \n\t\r\nLine breaks and tabs\x00\xFF"

	params := &WriteFileParams{
		FilePath: "special.txt",
		Content:  specialContent,
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully written")

	// Verify content was preserved
	filePath := filepath.Join(tmpDir, "special.txt")
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, specialContent, string(content))
}

func TestWriteFileTool_RestrictedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)

	tests := []struct {
		name     string
		filePath string
		errMsg   string
	}{
		{
			name:     "git directory file",
			filePath: ".git/config",
			errMsg:   "editing Git repository files is not allowed",
		},
		{
			name:     "git directory subdirectory",
			filePath: ".git/objects/abc123",
			errMsg:   "editing Git repository files is not allowed",
		},
		{
			name:     "environment file",
			filePath: ".env",
			errMsg:   "editing .env files requires explicit confirmation",
		},
		{
			name:     "production environment file",
			filePath: ".env.production",
			errMsg:   "editing .env.production files requires explicit confirmation",
		},
		{
			name:     "dockerfile",
			filePath: "Dockerfile",
			errMsg:   "editing dockerfile files requires explicit confirmation",
		},
		{
			name:     "docker compose file",
			filePath: "docker-compose.yml",
			errMsg:   "editing docker-compose.yml files requires explicit confirmation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &WriteFileParams{
				FilePath: tt.filePath,
				Content:  "should not work",
			}

			result, err := tool.Execute(context.Background(), params)
			assert.Error(t, err)
			assert.Empty(t, result)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}