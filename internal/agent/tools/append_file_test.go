package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendFileTool_Name(t *testing.T) {
	tool := NewAppendFileTool("/tmp")
	assert.Equal(t, "append_file", tool.Name())
}

func TestAppendFileTool_Description(t *testing.T) {
	tool := NewAppendFileTool("/tmp")
	desc := tool.Description()
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "append")
	assert.Contains(t, desc, "file")
	assert.Contains(t, desc, "content")
}

func TestAppendFileTool_AppendContent(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

	// Create test file with existing content
	testFile := filepath.Join(tmpDir, "append_test.txt")
	originalContent := "Line 1\nLine 2"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &AppendFileParams{
		FilePath: "append_test.txt",
		Content:  "\nLine 3\nLine 4",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully appended")
	assert.Contains(t, result, "Backup created")

	// Verify content was appended correctly
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Line 1\nLine 2\nLine 3\nLine 4"
	assert.Equal(t, expected, string(newContent))

	// Verify backup was created
	backupFiles, err := filepath.Glob(filepath.Join(tmpDir, "append_test.txt.backup.*"))
	require.NoError(t, err)
	assert.Len(t, backupFiles, 1)
}

func TestAppendFileTool_WithSeparator(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "separator_test.txt")
	originalContent := "Existing content"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &AppendFileParams{
		FilePath:  "separator_test.txt",
		Content:   "New content",
		Separator: "\n---\n",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully appended")

	// Verify content was appended with separator
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Existing content\n---\nNew content"
	assert.Equal(t, expected, string(newContent))
}

func TestAppendFileTool_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

	params := &AppendFileParams{
		FilePath: "nonexistent.txt",
		Content:  "This should create the file",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully appended")
	assert.NotContains(t, result, "Backup created") // No backup for new files

	// Verify file was created with content
	filePath := filepath.Join(tmpDir, "nonexistent.txt")
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "This should create the file", string(content))
}

func TestAppendFileTool_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "empty_append.txt")
	originalContent := "Original content"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &AppendFileParams{
		FilePath: "empty_append.txt",
		Content:  "",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully appended")

	// Verify content remains unchanged
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(newContent))
}

func TestAppendFileTool_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

	// Create empty file
	testFile := filepath.Join(tmpDir, "empty.txt")
	err := os.WriteFile(testFile, []byte(""), 0644)
	require.NoError(t, err)

	params := &AppendFileParams{
		FilePath: "empty.txt",
		Content:  "First content",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully appended")

	// Verify content was appended to empty file
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "First content", string(newContent))
}

func TestAppendFileTool_LineEndingPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

	// Create test file with specific line ending
	testFile := filepath.Join(tmpDir, "lineending.txt")
	originalContent := "Line 1\nLine 2\n" // Ends with newline
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &AppendFileParams{
		FilePath: "lineending.txt",
		Content:  "Line 3",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully appended")

	// Verify line endings are preserved
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Line 1\nLine 2\nLine 3"
	assert.Equal(t, expected, string(newContent))
}

func TestAppendFileTool_NoTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

	// Create test file without trailing newline
	testFile := filepath.Join(tmpDir, "no_newline.txt")
	originalContent := "Line 1\nLine 2" // No trailing newline
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &AppendFileParams{
		FilePath: "no_newline.txt",
		Content:  "Line 3",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully appended")

	// Verify content is appended with automatic newline
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Line 1\nLine 2\nLine 3"
	assert.Equal(t, expected, string(newContent))
}

func TestAppendFileTool_InvalidParams(t *testing.T) {
	tool := NewAppendFileTool("/tmp")

	tests := []struct {
		name   string
		params *AppendFileParams
		errMsg string
	}{
		{
			name:   "nil params",
			params: nil,
			errMsg: "file_path is required",
		},
		{
			name:   "empty file path",
			params: &AppendFileParams{FilePath: "", Content: "test"},
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

func TestAppendFileTool_PathSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

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
			params := &AppendFileParams{
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

func TestAppendFileTool_LargeContent(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "large_append.txt")
	originalContent := "Original content"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	// Create large content to append (100KB)
	largeContent := ""
	for i := 0; i < 2500; i++ {
		largeContent += "This is line " + string(rune(48+i%10)) + " of large content.\n"
	}

	params := &AppendFileParams{
		FilePath: "large_append.txt",
		Content:  largeContent,
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully appended")

	// Verify content was appended correctly
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := originalContent + "\n" + largeContent
	assert.Equal(t, expected, string(newContent))
}

func TestAppendFileTool_MultilineSeparator(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "multiline_sep.txt")
	originalContent := "Original content"
	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	require.NoError(t, err)

	params := &AppendFileParams{
		FilePath:  "multiline_sep.txt",
		Content:   "Appended content",
		Separator: "\n\n===== NEW SECTION =====\n\n",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully appended")

	// Verify content was appended with multiline separator
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	expected := "Original content\n\n===== NEW SECTION =====\n\nAppended content"
	assert.Equal(t, expected, string(newContent))
}

func TestAppendFileTool_BinaryFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewAppendFileTool(tmpDir)

	// Create binary file (with null bytes)
	testFile := filepath.Join(tmpDir, "binary.bin")
	originalContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
	err := os.WriteFile(testFile, originalContent, 0644)
	require.NoError(t, err)

	params := &AppendFileParams{
		FilePath: "binary.bin",
		Content:  "text content",
	}

	result, err := tool.Execute(context.Background(), params)
	require.NoError(t, err)
	assert.Contains(t, result, "successfully appended")

	// Verify binary content + text content
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)

	// Check that original binary content is preserved
	assert.Equal(t, originalContent, newContent[:len(originalContent)])
	// Check that text content was appended (with automatic newline)
	assert.Equal(t, "\ntext content", string(newContent[len(originalContent):]))
}