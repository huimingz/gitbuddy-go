package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/huimingz/gitbuddy-go/internal/agent/backup"
)

// WriteFileParams contains parameters for writing a file
type WriteFileParams struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// WriteFileTool is a tool for writing file contents
type WriteFileTool struct {
	workDir       string
	backupManager *backup.BackupManager
}

// NewWriteFileTool creates a new WriteFileTool
func NewWriteFileTool(workDir string) *WriteFileTool {
	return &WriteFileTool{
		workDir:       workDir,
		backupManager: backup.NewBackupManager(workDir),
	}
}

// Name returns the tool name
func (t *WriteFileTool) Name() string {
	return "write_file"
}

// Description returns the tool description
func (t *WriteFileTool) Description() string {
	return `Write content to a file. Creates new files or overwrites existing files.
Parameters:
- file_path (required): Path to the file to write
- content (required): Content to write to the file
Returns confirmation of successful write operation.
Note: For existing files, a backup is automatically created before overwriting.
Safety: File operations are restricted to the working directory and subdirectories.`
}

// Execute runs the tool and writes content to the specified file
func (t *WriteFileTool) Execute(ctx context.Context, params *WriteFileParams) (string, error) {
	if params == nil || params.FilePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	// Validate and resolve file path
	resolvedPath, err := t.validateAndResolvePath(params.FilePath)
	if err != nil {
		return "", err
	}

	// Check if this is a restricted file
	if restricted, reason := t.isRestrictedPath(params.FilePath); restricted {
		return "", fmt.Errorf("file access restricted: %s", reason)
	}

	// Check if file exists for backup creation
	var backupPath string
	if _, err := os.Stat(resolvedPath); err == nil {
		// File exists, create backup using BackupManager
		backupPath, err = t.backupManager.CreateBackup(ctx, resolvedPath, "write-file")
		if err != nil {
			return "", fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	// Write the file
	if err := os.WriteFile(resolvedPath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Build response message
	var result strings.Builder
	result.WriteString(fmt.Sprintf("File '%s' successfully written", params.FilePath))

	// Add file stats
	if info, err := os.Stat(resolvedPath); err == nil {
		result.WriteString(fmt.Sprintf(" (%d bytes)", info.Size()))
	}

	if backupPath != "" {
		result.WriteString(fmt.Sprintf("\nBackup created at: %s", filepath.Base(backupPath)))
	}

	result.WriteString(".")

	return result.String(), nil
}

// validateAndResolvePath validates the file path and resolves it relative to workDir
func (t *WriteFileTool) validateAndResolvePath(filePath string) (string, error) {
	// Clean the path to handle .. and other path traversals
	cleanPath := filepath.Clean(filePath)

	// Handle absolute paths
	if filepath.IsAbs(cleanPath) {
		// For absolute paths, check if they're within the working directory
		absWorkDir, err := filepath.Abs(t.workDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve working directory: %w", err)
		}

		// Check if the absolute path is within the working directory
		rel, err := filepath.Rel(absWorkDir, cleanPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("path outside working directory not allowed: %s", filePath)
		}
		return cleanPath, nil
	}

	// For relative paths, resolve against working directory
	resolvedPath := filepath.Join(t.workDir, cleanPath)

	// Verify the resolved path is still within the working directory
	absWorkDir, err := filepath.Abs(t.workDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve working directory: %w", err)
	}

	absResolvedPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}

	rel, err := filepath.Rel(absWorkDir, absResolvedPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path outside working directory not allowed: %s", filePath)
	}

	return resolvedPath, nil
}

// isRestrictedPath checks if the file path should be restricted from editing
func (t *WriteFileTool) isRestrictedPath(filePath string) (bool, string) {
	cleanPath := strings.ToLower(filepath.Clean(filePath))

	// Check for Git directory files
	if strings.Contains(cleanPath, ".git/") || strings.HasPrefix(cleanPath, ".git/") {
		return true, "editing Git repository files is not allowed"
	}

	// Check for sensitive configuration files
	restrictedFiles := []string{
		".gitignore",
		".env",
		".env.local",
		".env.production",
		"dockerfile",
		"docker-compose.yml",
		"docker-compose.yaml",
	}

	fileName := filepath.Base(cleanPath)
	for _, restricted := range restrictedFiles {
		if fileName == restricted {
			return true, fmt.Sprintf("editing %s files requires explicit confirmation", restricted)
		}
	}

	return false, ""
}

