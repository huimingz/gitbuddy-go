package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EditFileParams contains parameters for editing a file
type EditFileParams struct {
	FilePath  string `json:"file_path"`
	Operation string `json:"operation"` // "replace", "insert", "delete"
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line,omitempty"`
	Content   string `json:"content,omitempty"`
}

// EditFileTool is a tool for making precise edits to existing files
type EditFileTool struct {
	workDir string
}

// NewEditFileTool creates a new EditFileTool
func NewEditFileTool(workDir string) *EditFileTool {
	return &EditFileTool{
		workDir: workDir,
	}
}

// Name returns the tool name
func (t *EditFileTool) Name() string {
	return "edit_file"
}

// Description returns the tool description
func (t *EditFileTool) Description() string {
	return `Edit existing files with precise line-based operations.
Parameters:
- file_path (required): Path to the file to edit
- operation (required): Type of operation: "replace", "insert", or "delete"
- start_line (required): Starting line number (1-indexed)
- end_line (optional): Ending line number (1-indexed, inclusive). Required for replace and delete operations.
- content (optional): New content for replace and insert operations
Operations:
- replace: Replace lines from start_line to end_line with new content
- insert: Insert new content at start_line (existing lines shift down)
- delete: Delete lines from start_line to end_line
Returns confirmation with details of the edit operation and creates automatic backups.
Safety: Operations are restricted to the working directory and subdirectories.`
}

// Execute runs the tool and performs the specified edit operation
func (t *EditFileTool) Execute(ctx context.Context, params *EditFileParams) (string, error) {
	if err := t.validateParams(params); err != nil {
		return "", err
	}

	// Validate and resolve file path (reuse logic from WriteFileTool)
	writeFileTool := NewWriteFileTool(t.workDir)
	resolvedPath, err := writeFileTool.validateAndResolvePath(params.FilePath)
	if err != nil {
		return "", err
	}

	// Check if file exists
	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", params.FilePath)
	}

	// Read the current file content
	lines, err := t.readFileLines(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Validate line ranges against file content
	if err := t.validateLineRange(params, len(lines)); err != nil {
		return "", err
	}

	// Create backup before editing
	backupPath, err := t.createBackup(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	// Perform the edit operation
	newLines, operationDesc, err := t.performOperation(params, lines)
	if err != nil {
		return "", fmt.Errorf("failed to perform operation: %w", err)
	}

	// Write the modified content back to the file
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(resolvedPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write modified file: %w", err)
	}

	// Build response message
	result := fmt.Sprintf("File '%s' successfully edited. %s", params.FilePath, operationDesc)
	if backupPath != "" {
		result += fmt.Sprintf("\nBackup created at: %s", filepath.Base(backupPath))
	}

	return result + ".", nil
}

// validateParams validates the input parameters
func (t *EditFileTool) validateParams(params *EditFileParams) error {
	if params == nil || params.FilePath == "" {
		return fmt.Errorf("file_path is required")
	}

	if params.Operation == "" {
		return fmt.Errorf("operation is required")
	}

	// Validate operation type
	switch params.Operation {
	case "replace", "insert", "delete":
		// Valid operations
	default:
		return fmt.Errorf("invalid operation: %s. Valid operations are: replace, insert, delete", params.Operation)
	}

	// Validate line numbers
	if params.StartLine <= 0 {
		return fmt.Errorf("line numbers must be greater than 0")
	}

	// For replace and delete operations, end_line is required
	if (params.Operation == "replace" || params.Operation == "delete") && params.EndLine <= 0 {
		params.EndLine = params.StartLine // Default to single line if not specified
	}

	// Validate line range
	if params.EndLine > 0 && params.StartLine > params.EndLine {
		return fmt.Errorf("start line must be less than or equal to end line")
	}

	return nil
}

// validateLineRange validates that the line range is within the file bounds
func (t *EditFileTool) validateLineRange(params *EditFileParams, fileLineCount int) error {
	switch params.Operation {
	case "replace", "delete":
		if params.StartLine > fileLineCount {
			return fmt.Errorf("line range exceeds file length (file has %d lines)", fileLineCount)
		}
		if params.EndLine > fileLineCount {
			return fmt.Errorf("line range exceeds file length (file has %d lines)", fileLineCount)
		}
	case "insert":
		// For insert, we can insert up to one line past the end of file
		if params.StartLine > fileLineCount+1 {
			return fmt.Errorf("insert position exceeds file length (file has %d lines)", fileLineCount)
		}
	}
	return nil
}

// readFileLines reads a file and returns its content as a slice of lines
func (t *EditFileTool) readFileLines(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// performOperation performs the specified edit operation on the lines
func (t *EditFileTool) performOperation(params *EditFileParams, lines []string) ([]string, string, error) {
	switch params.Operation {
	case "replace":
		return t.replaceLines(params, lines)
	case "insert":
		return t.insertLines(params, lines)
	case "delete":
		return t.deleteLines(params, lines)
	default:
		return nil, "", fmt.Errorf("unsupported operation: %s", params.Operation)
	}
}

// replaceLines replaces the specified line range with new content
func (t *EditFileTool) replaceLines(params *EditFileParams, lines []string) ([]string, string, error) {
	startIdx := params.StartLine - 1 // Convert to 0-based index
	endIdx := params.EndLine - 1     // Convert to 0-based index

	// Split new content into lines
	newLines := strings.Split(params.Content, "\n")

	// Build new file content
	var result []string
	result = append(result, lines[:startIdx]...)        // Lines before replacement
	result = append(result, newLines...)                // New content
	result = append(result, lines[endIdx+1:]...)        // Lines after replacement

	linesReplaced := endIdx - startIdx + 1
	operationDesc := fmt.Sprintf("%d line(s) replaced", linesReplaced)

	return result, operationDesc, nil
}

// insertLines inserts new content at the specified line position
func (t *EditFileTool) insertLines(params *EditFileParams, lines []string) ([]string, string, error) {
	insertIdx := params.StartLine - 1 // Convert to 0-based index

	// Handle insertion at the end of file
	if insertIdx > len(lines) {
		insertIdx = len(lines)
	}

	// Split new content into lines
	newLines := strings.Split(params.Content, "\n")

	// Build new file content
	var result []string
	result = append(result, lines[:insertIdx]...)    // Lines before insertion
	result = append(result, newLines...)             // New content
	result = append(result, lines[insertIdx:]...)    // Lines after insertion point

	linesInserted := len(newLines)
	operationDesc := fmt.Sprintf("%d line(s) inserted", linesInserted)

	return result, operationDesc, nil
}

// deleteLines deletes the specified line range
func (t *EditFileTool) deleteLines(params *EditFileParams, lines []string) ([]string, string, error) {
	startIdx := params.StartLine - 1 // Convert to 0-based index
	endIdx := params.EndLine - 1     // Convert to 0-based index

	// Build new file content by skipping the deleted range
	var result []string
	result = append(result, lines[:startIdx]...)     // Lines before deletion
	result = append(result, lines[endIdx+1:]...)     // Lines after deletion

	linesDeleted := endIdx - startIdx + 1
	operationDesc := fmt.Sprintf("%d line(s) deleted", linesDeleted)

	return result, operationDesc, nil
}

// createBackup creates a timestamped backup of the existing file
func (t *EditFileTool) createBackup(filePath string) (string, error) {
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