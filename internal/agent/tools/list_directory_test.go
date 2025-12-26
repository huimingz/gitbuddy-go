package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDirectoryTool_Name(t *testing.T) {
	tool := NewListDirectoryTool("/tmp")
	if tool.Name() != "list_directory" {
		t.Errorf("expected name 'list_directory', got '%s'", tool.Name())
	}
}

func TestListDirectoryTool_Description(t *testing.T) {
	tool := NewListDirectoryTool("/tmp")
	desc := tool.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
	if !strings.Contains(desc, "list") && !strings.Contains(desc, "directory") {
		t.Error("description should mention listing directories")
	}
}

func TestListDirectoryTool_Execute_NonRecursive(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create test structure
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("content2"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir1"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "subdir2"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)

	tool := NewListDirectoryTool(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name      string
		params    *ListDirectoryParams
		wantFiles []string
		wantDirs  []string
		wantErr   bool
	}{
		{
			name: "list without hidden files",
			params: &ListDirectoryParams{
				Path:       ".",
				ShowHidden: false,
				Recursive:  false,
			},
			wantFiles: []string{"file1.txt", "file2.go"},
			wantDirs:  []string{"subdir1", "subdir2"},
			wantErr:   false,
		},
		{
			name: "list with hidden files",
			params: &ListDirectoryParams{
				Path:       ".",
				ShowHidden: true,
				Recursive:  false,
			},
			wantFiles: []string{"file1.txt", "file2.go", ".hidden"},
			wantDirs:  []string{"subdir1", "subdir2"},
			wantErr:   false,
		},
		{
			name: "non-existent directory",
			params: &ListDirectoryParams{
				Path:       "nonexistent",
				ShowHidden: false,
				Recursive:  false,
			},
			wantErr: true,
		},
		{
			name: "empty path",
			params: &ListDirectoryParams{
				Path: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.params)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check that expected files are in the result
			for _, file := range tt.wantFiles {
				if !strings.Contains(result, file) {
					t.Errorf("expected file '%s' not found in result", file)
				}
			}

			// Check that expected directories are in the result
			for _, dir := range tt.wantDirs {
				if !strings.Contains(result, dir) {
					t.Errorf("expected directory '%s' not found in result", dir)
				}
			}
		})
	}
}

func TestListDirectoryTool_Execute_Recursive(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create nested structure
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "dir1", "file1.txt"), []byte("content1"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1", "dir2"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "dir1", "dir2", "file2.txt"), []byte("content2"), 0644)

	tool := NewListDirectoryTool(tmpDir)
	ctx := context.Background()

	params := &ListDirectoryParams{
		Path:       ".",
		ShowHidden: false,
		Recursive:  true,
		MaxDepth:   2,
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that all files and directories are present
	expectedItems := []string{"root.txt", "dir1", "file1.txt", "dir2"}
	for _, item := range expectedItems {
		if !strings.Contains(result, item) {
			t.Errorf("expected item '%s' not found in result", item)
		}
	}

	// Check for tree-like structure indicators
	if !strings.Contains(result, "├──") {
		t.Error("expected tree structure indicators in recursive listing")
	}
}

func TestListDirectoryTool_Execute_FileAsPath(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("content"), 0644)

	tool := NewListDirectoryTool(tmpDir)
	ctx := context.Background()

	params := &ListDirectoryParams{
		Path:       "test.txt",
		ShowHidden: false,
		Recursive:  false,
	}

	_, err := tool.Execute(ctx, params)
	if err == nil {
		t.Error("expected error when path is a file, got nil")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected 'not a directory' error, got: %v", err)
	}
}

func TestListDirectoryTool_Execute_ExcludedDirectories(t *testing.T) {
	// Create a temporary directory structure with excluded directories
	tmpDir := t.TempDir()

	os.Mkdir(filepath.Join(tmpDir, "normal"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755)
	os.Mkdir(filepath.Join(tmpDir, ".git"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644)

	tool := NewListDirectoryTool(tmpDir)
	ctx := context.Background()

	params := &ListDirectoryParams{
		Path:       ".",
		ShowHidden: false,
		Recursive:  false,
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain normal directory
	if !strings.Contains(result, "normal") {
		t.Error("expected 'normal' directory in result")
	}

	// Should NOT contain excluded directories
	if strings.Contains(result, "node_modules") {
		t.Error("should not contain 'node_modules' directory")
	}
	if strings.Contains(result, ".git") {
		t.Error("should not contain '.git' directory")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatSize(tt.size)
			if result != tt.expected {
				t.Errorf("formatSize(%d) = %s, want %s", tt.size, result, tt.expected)
			}
		})
	}
}
