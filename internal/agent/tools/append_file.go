package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AppendFileParams contains parameters for appending to a file
type AppendFileParams struct {
	FilePath  string `json:"file_path"`
	Content   string `json:"content"`
	Separator string `json:"separator,omitempty"`
}

// AppendFileTool is a tool for appending content to existing files
type AppendFileTool struct {
	workDir string
}

// NewAppendFileTool creates a new AppendFileTool
func NewAppendFileTool(workDir string) *AppendFileTool {
	return &AppendFileTool{
		workDir: workDir,
	}
}

// Name returns the tool name
func (t *AppendFileTool) Name() string {
	return "append_file"
}

// Description returns the tool description
func (t *AppendFileTool) Description() string {
	return `Append content to the end of a file. Creates the file if it doesn't exist.
Parameters:
- file_path (required): Path to the file to append to
- content (required): Content to append to the file
- separator (optional): Separator to insert before the new content
Returns confirmation of successful append operation.
Note: For existing files, a backup is automatically created before appending.
The tool automatically handles line endings and preserves existing file format.
Safety: File operations are restricted to the working directory and subdirectories.`
}

// Execute runs the tool and appends content to the specified file
func (t *AppendFileTool) Execute(ctx context.Context, params *AppendFileParams) (string, error) {
	if params == nil || params.FilePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	// Validate and resolve file path
	resolvedPath, err := t.validateAndResolvePath(params.FilePath)
	if err != nil {
		return "", err
	}

	// Check if file exists
	fileExists := true
	var originalContent []byte
	if content, err := os.ReadFile(resolvedPath); os.IsNotExist(err) {
		fileExists = false
		originalContent = []byte{}
	} else if err != nil {
		return "", fmt.Errorf("failed to read existing file: %w", err)
	} else {
		originalContent = content
	}

	var backupPath string
	if fileExists && len(originalContent) > 0 {
		// Create backup for existing non-empty files
		backupPath, err = t.createBackup(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	// Build new content
	newContent, err := t.buildNewContent(originalContent, params)
	if err != nil {
		return "", fmt.Errorf("failed to build new content: %w", err)
	}

	// Write the combined content
	if err := os.WriteFile(resolvedPath, newContent, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Build response message
	var result strings.Builder
	result.WriteString(fmt.Sprintf("File '%s' successfully appended", params.FilePath))

	// Add size information
	if info, err := os.Stat(resolvedPath); err == nil {
		contentSize := len(params.Content)
		result.WriteString(fmt.Sprintf(" (%d bytes added, %d bytes total)", contentSize, info.Size()))
	}

	if backupPath != "" {
		result.WriteString(fmt.Sprintf("\nBackup created at: %s", filepath.Base(backupPath)))
	}

	result.WriteString(".")

	return result.String(), nil
}

// buildNewContent constructs the new file content by combining existing content with new content
func (t *AppendFileTool) buildNewContent(originalContent []byte, params *AppendFileParams) ([]byte, error) {
	var result []byte
	result = append(result, originalContent...)

	// If there's content to append
	if params.Content != "" {
		// Handle line ending logic
		if len(originalContent) > 0 {
			// Add separator if specified
			if params.Separator != "" {
				result = append(result, []byte(params.Separator)...)
			} else {
				// Check if original content ends with a newline and content to append starts with one
				originalEndsWithNewline := strings.HasSuffix(string(originalContent), "\n")
				contentStartsWithNewline := strings.HasPrefix(params.Content, "\n")

				// Add a newline only if original doesn't end with one AND content doesn't start with one
				if !originalEndsWithNewline && !contentStartsWithNewline {
					result = append(result, '\n')
				}
			}
		} else {
			// For empty files, add separator if specified (but no leading newline)
			if params.Separator != "" {
				result = append(result, []byte(params.Separator)...)
			}
		}

		// Add the new content
		result = append(result, []byte(params.Content)...)
	}

	return result, nil
}

// createBackup creates a timestamped backup of the existing file
func (t *AppendFileTool) createBackup(filePath string) (string, error) {
	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.backup.%s", filePath, timestamp)

	// Read original file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read original file: %w", err)
	}

	// Write backup file
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup file: %w", err)
	}

	return backupPath, nil
}

// validateAndResolvePath validates the file path and resolves it relative to workDir
func (t *AppendFileTool) validateAndResolvePath(filePath string) (string, error) {
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