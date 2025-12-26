package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListFilesTool_Name(t *testing.T) {
	tool := NewListFilesTool("/tmp", 100)
	if tool.Name() != "list_files" {
		t.Errorf("expected name 'list_files', got '%s'", tool.Name())
	}
}

func TestListFilesTool_Description(t *testing.T) {
	tool := NewListFilesTool("/tmp", 100)
	desc := tool.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
	if !strings.Contains(desc, "glob") && !strings.Contains(desc, "pattern") {
		t.Error("description should mention glob patterns")
	}
}

func TestListFilesTool_Execute_SimplePattern(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("content2"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file3.txt"), []byte("content3"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test_file.go"), []byte("test"), 0644)

	tool := NewListFilesTool(tmpDir, 100)
	ctx := context.Background()

	tests := []struct {
		name      string
		pattern   string
		wantFiles []string
		wantCount int
	}{
		{
			name:      "match all .go files",
			pattern:   "*.go",
			wantFiles: []string{"file1.go", "file2.go", "test_file.go"},
			wantCount: 3,
		},
		{
			name:      "match all .txt files",
			pattern:   "*.txt",
			wantFiles: []string{"file3.txt"},
			wantCount: 1,
		},
		{
			name:      "match test files",
			pattern:   "test_*.go",
			wantFiles: []string{"test_file.go"},
			wantCount: 1,
		},
		{
			name:      "no matches",
			pattern:   "*.py",
			wantFiles: []string{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &ListFilesParams{
				Pattern: tt.pattern,
				Path:    ".",
			}

			result, err := tool.Execute(ctx, params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check that expected files are in the result
			for _, file := range tt.wantFiles {
				if !strings.Contains(result, file) {
					t.Errorf("expected file '%s' not found in result", file)
				}
			}

			// Check match count
			if tt.wantCount == 0 {
				if !strings.Contains(result, "No files found") {
					t.Error("expected 'No files found' message")
				}
			}
		})
	}
}

func TestListFilesTool_Execute_RecursivePattern(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create nested structure
	os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte("root"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "dir1", "file1.go"), []byte("content1"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1", "dir2"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "dir1", "dir2", "file2.go"), []byte("content2"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "dir1", "dir2", "file3.txt"), []byte("content3"), 0644)

	tool := NewListFilesTool(tmpDir, 100)
	ctx := context.Background()

	params := &ListFilesParams{
		Pattern: "**/*.go",
		Path:    ".",
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find all .go files recursively
	expectedFiles := []string{"root.go", "file1.go", "file2.go"}
	for _, file := range expectedFiles {
		if !strings.Contains(result, file) {
			t.Errorf("expected file '%s' not found in result", file)
		}
	}

	// Should NOT find .txt files
	if strings.Contains(result, "file3.txt") {
		t.Error("should not find .txt files with *.go pattern")
	}
}

func TestListFilesTool_Execute_ExcludeDirs(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create structure with excluded directories
	os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte("root"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "node_modules", "lib.go"), []byte("lib"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("main"), 0644)

	tool := NewListFilesTool(tmpDir, 100)
	ctx := context.Background()

	params := &ListFilesParams{
		Pattern: "*.go",
		Path:    ".",
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find files in root and src
	if !strings.Contains(result, "root.go") {
		t.Error("expected to find root.go")
	}
	if !strings.Contains(result, "main.go") {
		t.Error("expected to find src/main.go")
	}

	// Should NOT find files in node_modules (auto-excluded)
	if strings.Contains(result, "lib.go") {
		t.Error("should not find files in node_modules")
	}
}

func TestListFilesTool_Execute_MaxResults(t *testing.T) {
	// Create a temporary directory with many files
	tmpDir := t.TempDir()

	// Create 10 files
	for i := 0; i < 10; i++ {
		filename := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		os.WriteFile(filename, []byte("content"), 0644)
	}

	tool := NewListFilesTool(tmpDir, 5) // Max 5 results
	ctx := context.Background()

	params := &ListFilesParams{
		Pattern: "*.txt",
		Path:    ".",
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should mention that results are limited
	if !strings.Contains(result, "limited") {
		t.Error("expected message about limited results")
	}
}

func TestListFilesTool_Execute_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewListFilesTool(tmpDir, 100)
	ctx := context.Background()

	tests := []struct {
		name    string
		params  *ListFilesParams
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil params",
			params:  nil,
			wantErr: true,
			errMsg:  "pattern is required",
		},
		{
			name: "empty pattern",
			params: &ListFilesParams{
				Pattern: "",
				Path:    ".",
			},
			wantErr: true,
			errMsg:  "pattern is required",
		},
		{
			name: "empty path",
			params: &ListFilesParams{
				Pattern: "*.go",
				Path:    "",
			},
			wantErr: true,
			errMsg:  "path is required",
		},
		{
			name: "non-existent path",
			params: &ListFilesParams{
				Pattern: "*.go",
				Path:    "nonexistent",
			},
			wantErr: true,
			errMsg:  "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, tt.params)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestListFilesTool_Execute_FileAsPath(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(filePath, []byte("content"), 0644)

	tool := NewListFilesTool(tmpDir, 100)
	ctx := context.Background()

	params := &ListFilesParams{
		Pattern: "*.txt",
		Path:    "test.txt",
	}

	_, err := tool.Execute(ctx, params)
	if err == nil {
		t.Error("expected error when path is a file, got nil")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected 'not a directory' error, got: %v", err)
	}
}

func TestMatchGlobPattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/*.go", "main.go", true},
		{"**/*.go", "src/main.go", true},
		{"**/*.go", "src/pkg/main.go", true},
		{"**/*.go", "main.txt", false},
		{"src/**/*.go", "src/main.go", true},
		{"src/**/*.go", "src/pkg/main.go", true},
		{"src/**/*.go", "main.go", false},
		{"**/test_*.go", "test_main.go", true},
		{"**/test_*.go", "src/test_util.go", true},
		{"**/test_*.go", "src/main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			got := matchGlobPattern(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("matchGlobPattern(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}
