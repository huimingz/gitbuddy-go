package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditFileTool_Name(t *testing.T) {
	tool := NewEditFileTool("/tmp")
	assert.Equal(t, "edit_file", tool.Name())
}

func TestEditFileTool_Description(t *testing.T) {
	tool := NewEditFileTool("/tmp")
	desc := tool.Description()
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "edit")
	assert.Contains(t, desc, "file")
	assert.Contains(t, desc, "operation")
	assert.Contains(t, desc, "replace")
	assert.Contains(t, desc, "insert")
	assert.Contains(t, desc, "delete")
}

func TestEditFileTool_ReplaceLines(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

	// Create test file with known content
	testFile := filepath.Join(tmpDir, "test.txt")
	originalContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &EditFileParams{
		FilePath:  "test.txt",
		Operation: "replace",
		StartLine: 2,
		EndLine:   3,
		Content:   "New Line 2\nNew Line 3",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully edited")
	assert.Contains(t, result, "2 line(s) replaced")
	assert.Contains(t, result, "Backup created")

	// Verify the content was replaced correctly
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Line 1\nNew Line 2\nNew Line 3\nLine 4\nLine 5"
	assert.Equal(t, expected, string(newContent))

	// Verify backup was created
	backupFiles, err := filepath.Glob(filepath.Join(tmpDir, "test.txt.backup.*"))
	require.NoError(t, err)
	assert.Len(t, backupFiles, 1)
}

func TestEditFileTool_InsertLines(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "insert_test.txt")
	originalContent := "Line 1\nLine 2\nLine 4\nLine 5"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &EditFileParams{
		FilePath:  "insert_test.txt",
		Operation: "insert",
		StartLine: 3, // Insert after line 2
		Content:   "Line 3",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully edited")
	assert.Contains(t, result, "1 line(s) inserted")

	// Verify content was inserted correctly
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	assert.Equal(t, expected, string(newContent))
}

func TestEditFileTool_DeleteLines(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "delete_test.txt")
	originalContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &EditFileParams{
		FilePath:  "delete_test.txt",
		Operation: "delete",
		StartLine: 2,
		EndLine:   3,
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully edited")
	assert.Contains(t, result, "2 line(s) deleted")

	// Verify lines were deleted correctly
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Line 1\nLine 4\nLine 5"
	assert.Equal(t, expected, string(newContent))
}

func TestEditFileTool_InsertAtBeginning(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "beginning.txt")
	originalContent := "Line 2\nLine 3"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &EditFileParams{
		FilePath:  "beginning.txt",
		Operation: "insert",
		StartLine: 1,
		Content:   "Line 1",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully edited")

	// Verify content was inserted at beginning
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Line 1\nLine 2\nLine 3"
	assert.Equal(t, expected, string(newContent))
}

func TestEditFileTool_InsertAtEnd(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "end.txt")
	originalContent := "Line 1\nLine 2"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &EditFileParams{
		FilePath:  "end.txt",
		Operation: "insert",
		StartLine: 3, // Insert after last line
		Content:   "Line 3",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully edited")

	// Verify content was inserted at end
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Line 1\nLine 2\nLine 3"
	assert.Equal(t, expected, string(newContent))
}

func TestEditFileTool_InvalidRange(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

	// Create test file with 3 lines
	testFile := filepath.Join(tmpDir, "invalid.txt")
	originalContent := "Line 1\nLine 2\nLine 3"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name      string
		params    *EditFileParams
		errMsg    string
	}{
		{
			name: "start line greater than end line",
			params: &EditFileParams{
				FilePath:  "invalid.txt",
				Operation: "replace",
				StartLine: 3,
				EndLine:   2,
				Content:   "replacement",
			},
			errMsg: "start line must be less than or equal to end line",
		},
		{
			name: "line number too large",
			params: &EditFileParams{
				FilePath:  "invalid.txt",
				Operation: "replace",
				StartLine: 10,
				EndLine:   10,
				Content:   "replacement",
			},
			errMsg: "line range exceeds file length",
		},
		{
			name: "zero line number",
			params: &EditFileParams{
				FilePath:  "invalid.txt",
				Operation: "replace",
				StartLine: 0,
				EndLine:   1,
				Content:   "replacement",
			},
			errMsg: "line numbers must be greater than 0",
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

func TestEditFileTool_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

	params := &EditFileParams{
		FilePath:  "nonexistent.txt",
		Operation: "replace",
		StartLine: 1,
		EndLine:   1,
		Content:   "content",
	}

	result, err := tool.Execute(context.Background(), params)
	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "file not found")
}

func TestEditFileTool_InvalidParams(t *testing.T) {
	tool := NewEditFileTool("/tmp")

	tests := []struct {
		name   string
		params *EditFileParams
		errMsg string
	}{
		{
			name:   "nil params",
			params: nil,
			errMsg: "file_path is required",
		},
		{
			name:   "empty file path",
			params: &EditFileParams{FilePath: "", Operation: "replace"},
			errMsg: "file_path is required",
		},
		{
			name:   "empty operation",
			params: &EditFileParams{FilePath: "test.txt", Operation: ""},
			errMsg: "operation is required",
		},
		{
			name:   "invalid operation",
			params: &EditFileParams{FilePath: "test.txt", Operation: "invalid"},
			errMsg: "invalid operation",
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

func TestEditFileTool_MultilineReplace(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "multiline.txt")
	originalContent := "Line 1\nOld Line 2\nOld Line 3\nOld Line 4\nLine 5"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &EditFileParams{
		FilePath:  "multiline.txt",
		Operation: "replace",
		StartLine: 2,
		EndLine:   4,
		Content:   "New Line 2\nNew Line 3\nNew Line 4\nExtra New Line",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "3 line(s) replaced")

	// Verify the content
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Line 1\nNew Line 2\nNew Line 3\nNew Line 4\nExtra New Line\nLine 5"
	assert.Equal(t, expected, string(newContent))
}

func TestEditFileTool_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

	// Create empty file
	testFile := filepath.Join(tmpDir, "empty.txt")
	err := os.WriteFile(testFile, []byte(""), 0644)
	require.NoError(t, err)

	params := &EditFileParams{
		FilePath:  "empty.txt",
		Operation: "insert",
		StartLine: 1,
		Content:   "First line",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully edited")

	// Verify content was inserted
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "First line", string(newContent))
}

func TestEditFileTool_SingleLineFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

	// Create single line file
	testFile := filepath.Join(tmpDir, "single.txt")
	err := os.WriteFile(testFile, []byte("Only line"), 0644)
	require.NoError(t, err)

	params := &EditFileParams{
		FilePath:  "single.txt",
		Operation: "replace",
		StartLine: 1,
		EndLine:   1,
		Content:   "Replaced line",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "1 line(s) replaced")

	// Verify content was replaced
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "Replaced line", string(newContent))
}

func TestEditFileTool_PathSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewEditFileTool(tmpDir)

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
			params := &EditFileParams{
				FilePath:  tt.filePath,
				Operation: "replace",
				StartLine: 1,
				EndLine:   1,
				Content:   "should not work",
			}

			result, err := tool.Execute(context.Background(), params)
			assert.Error(t, err)
			assert.Empty(t, result)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}