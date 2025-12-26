package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepFileTool_Execute(t *testing.T) {
	// Create temp directory for test files
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
	result := calculate(10, 20)
	fmt.Printf("Result: %d\n", result)
}

func calculate(a, b int) int {
	return a + b
}

func Calculate(x, y int) int {
	return x * y
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		params      *GrepFileParams
		wantErr     bool
		wantMatches int
		checkOutput func(t *testing.T, output string)
	}{
		{
			name: "basic search - find function",
			params: &GrepFileParams{
				FilePath: "test.go",
				Pattern:  "func main",
			},
			wantErr:     false,
			wantMatches: 1,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "Matches: 1") {
					t.Errorf("Expected 1 match, output: %s", output)
				}
				if !strings.Contains(output, "func main()") {
					t.Errorf("Expected to find 'func main()', output: %s", output)
				}
			},
		},
		{
			name: "regex pattern - find all functions",
			params: &GrepFileParams{
				FilePath: "test.go",
				Pattern:  `func \w+`,
			},
			wantErr:     false,
			wantMatches: 3,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "Matches: 3") {
					t.Errorf("Expected 3 matches, output: %s", output)
				}
			},
		},
		{
			name: "case insensitive search",
			params: &GrepFileParams{
				FilePath:   "test.go",
				Pattern:    "CALCULATE",
				IgnoreCase: true,
			},
			wantErr:     false,
			wantMatches: 3,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "Matches: 3") {
					t.Errorf("Expected 3 matches with case insensitive, output: %s", output)
				}
			},
		},
		{
			name: "case sensitive search",
			params: &GrepFileParams{
				FilePath:   "test.go",
				Pattern:    "Calculate",
				IgnoreCase: false,
			},
			wantErr:     false,
			wantMatches: 1,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "Matches: 1") {
					t.Errorf("Expected 1 match with case sensitive, output: %s", output)
				}
			},
		},
		{
			name: "with before context",
			params: &GrepFileParams{
				FilePath:      "test.go",
				Pattern:       "result := calculate",
				BeforeContext: 2,
			},
			wantErr:     false,
			wantMatches: 1,
			checkOutput: func(t *testing.T, output string) {
				// Should show 2 lines before the match (lines 5 and 6)
				if !strings.Contains(output, "func main()") {
					t.Errorf("Expected to see context before match, output: %s", output)
				}
				if !strings.Contains(output, "fmt.Println") {
					t.Errorf("Expected to see context before match, output: %s", output)
				}
			},
		},
		{
			name: "with after context",
			params: &GrepFileParams{
				FilePath:     "test.go",
				Pattern:      "func main",
				AfterContext: 2,
			},
			wantErr:     false,
			wantMatches: 1,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "fmt.Println") {
					t.Errorf("Expected to see context after match, output: %s", output)
				}
			},
		},
		{
			name: "with context (both before and after)",
			params: &GrepFileParams{
				FilePath: "test.go",
				Pattern:  "func calculate",
				Context:  1,
			},
			wantErr:     false,
			wantMatches: 1,
			checkOutput: func(t *testing.T, output string) {
				lines := strings.Split(output, "\n")
				contextLines := 0
				for _, line := range lines {
					if strings.Contains(line, "|") {
						contextLines++
					}
				}
				// Should have at least 3 lines: 1 before + 1 match + 1 after
				if contextLines < 3 {
					t.Errorf("Expected at least 3 context lines, got %d", contextLines)
				}
			},
		},
		{
			name: "no matches",
			params: &GrepFileParams{
				FilePath: "test.go",
				Pattern:  "nonexistent_pattern_xyz",
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "No matches found") {
					t.Errorf("Expected 'No matches found', got: %s", output)
				}
			},
		},
		{
			name: "file not found",
			params: &GrepFileParams{
				FilePath: "nonexistent.go",
				Pattern:  "test",
			},
			wantErr: true,
		},
		{
			name: "invalid regex",
			params: &GrepFileParams{
				FilePath: "test.go",
				Pattern:  "[invalid(",
			},
			wantErr: true,
		},
		{
			name: "missing file_path",
			params: &GrepFileParams{
				Pattern: "test",
			},
			wantErr: true,
		},
		{
			name: "missing pattern",
			params: &GrepFileParams{
				FilePath: "test.go",
			},
			wantErr: true,
		},
	}

	tool := NewGrepFileTool(tmpDir, DefaultMaxFileSize)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			output, err := tool.Execute(ctx, tt.params)

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkOutput != nil {
				tt.checkOutput(t, output)
			}
		})
	}
}

func TestGrepFileTool_FileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file that exceeds the size limit
	testFile := filepath.Join(tmpDir, "large.txt")
	content := strings.Repeat("This is a test line\n", 10000) // ~200KB
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create tool with small size limit (1KB)
	tool := NewGrepFileTool(tmpDir, 1024)

	params := &GrepFileParams{
		FilePath: "large.txt",
		Pattern:  "test",
	}

	ctx := context.Background()
	_, err := tool.Execute(ctx, params)

	if err == nil {
		t.Error("Expected error for file exceeding size limit")
	}
	if !strings.Contains(err.Error(), "file too large") {
		t.Errorf("Expected 'file too large' error, got: %v", err)
	}
}

func TestGrepFileTool_DirectoryError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	tool := NewGrepFileTool(tmpDir, DefaultMaxFileSize)

	params := &GrepFileParams{
		FilePath: "subdir",
		Pattern:  "test",
	}

	ctx := context.Background()
	_, err := tool.Execute(ctx, params)

	if err == nil {
		t.Error("Expected error when trying to grep a directory")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("Expected directory error, got: %v", err)
	}
}

func TestGrepFileTool_Name(t *testing.T) {
	tool := NewGrepFileTool("", DefaultMaxFileSize)
	if tool.Name() != "grep_file" {
		t.Errorf("Expected name 'grep_file', got: %s", tool.Name())
	}
}

func TestGrepFileTool_Description(t *testing.T) {
	tool := NewGrepFileTool("", DefaultMaxFileSize)
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
	if !strings.Contains(desc, "pattern") {
		t.Error("Description should mention 'pattern'")
	}
}
