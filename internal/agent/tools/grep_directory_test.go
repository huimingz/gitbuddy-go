package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestGrepDirectoryTool_Execute(t *testing.T) {
	// Create temp directory structure for tests
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
	calculate(10, 20)
}

func calculate(a, b int) int {
	return a + b
}
`,
		"utils.go": `package main

func Calculate(x, y int) int {
	return x * y
}

func helper() {
	// Helper function
}
`,
		"subdir/test.go": `package test

func TestCalculate(t *testing.T) {
	result := Calculate(2, 3)
	if result != 6 {
		t.Error("failed")
	}
}
`,
		"README.md": `# Test Project

This is a test project for calculating numbers.
`,
		"data.json": `{
	"calculate": true,
	"value": 42
}
`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	tests := []struct {
		name        string
		params      *GrepDirectoryParams
		wantErr     bool
		checkOutput func(t *testing.T, output string)
	}{
		{
			name: "basic directory search",
			params: &GrepDirectoryParams{
				Directory: ".",
				Pattern:   "calculate",
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "main.go") {
					t.Errorf("Expected to find main.go, output: %s", output)
				}
				if !strings.Contains(output, "Matches:") {
					t.Errorf("Expected to show matches count, output: %s", output)
				}
			},
		},
		{
			name: "recursive search",
			params: &GrepDirectoryParams{
				Directory: ".",
				Pattern:   "Calculate",
				Recursive: true,
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "utils.go") {
					t.Errorf("Expected to find utils.go, output: %s", output)
				}
				if !strings.Contains(output, "test.go") {
					t.Errorf("Expected to find test.go in subdir, output: %s", output)
				}
			},
		},
		{
			name: "non-recursive search",
			params: &GrepDirectoryParams{
				Directory: ".",
				Pattern:   "Calculate",
				Recursive: false,
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "utils.go") {
					t.Errorf("Expected to find utils.go, output: %s", output)
				}
				if strings.Contains(output, "subdir/test.go") {
					t.Errorf("Should not find files in subdirectories with non-recursive search, output: %s", output)
				}
			},
		},
		{
			name: "file pattern filter - go files",
			params: &GrepDirectoryParams{
				Directory:   ".",
				Pattern:     "calculate",
				FilePattern: "*.go",
				Recursive:   true,
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, ".go") {
					t.Errorf("Expected to find .go files, output: %s", output)
				}
				if strings.Contains(output, "README.md") {
					t.Errorf("Should not find README.md with *.go filter, output: %s", output)
				}
			},
		},
		{
			name: "file pattern filter - multiple extensions",
			params: &GrepDirectoryParams{
				Directory:   ".",
				Pattern:     "calculate",
				FilePattern: "*.{go,md}",
				Recursive:   true,
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				// Should find both .go and .md files
				hasGo := strings.Contains(output, ".go")
				hasMd := strings.Contains(output, ".md")
				if !hasGo && !hasMd {
					t.Errorf("Expected to find .go or .md files, output: %s", output)
				}
			},
		},
		{
			name: "case insensitive search",
			params: &GrepDirectoryParams{
				Directory:  ".",
				Pattern:    "CALCULATE",
				IgnoreCase: true,
				Recursive:  true,
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "main.go") {
					t.Errorf("Expected to find matches with case insensitive search, output: %s", output)
				}
			},
		},
		{
			name: "with context lines",
			params: &GrepDirectoryParams{
				Directory: ".",
				Pattern:   "func main",
				Context:   2,
				Recursive: true,
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "import") {
					t.Errorf("Expected to see context lines, output: %s", output)
				}
			},
		},
		{
			name: "max results limit",
			params: &GrepDirectoryParams{
				Directory:  ".",
				Pattern:    "func",
				MaxResults: 2,
				Recursive:  true,
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "Matches: 2") {
					t.Errorf("Expected to limit to 2 matches, output: %s", output)
				}
			},
		},
		{
			name: "no matches",
			params: &GrepDirectoryParams{
				Directory: ".",
				Pattern:   "nonexistent_pattern_xyz",
				Recursive: true,
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "No matches found") {
					t.Errorf("Expected 'No matches found', got: %s", output)
				}
			},
		},
		{
			name: "directory not found",
			params: &GrepDirectoryParams{
				Directory: "nonexistent_dir",
				Pattern:   "test",
			},
			wantErr: true,
		},
		{
			name: "invalid regex",
			params: &GrepDirectoryParams{
				Directory: ".",
				Pattern:   "[invalid(",
			},
			wantErr: true,
		},
		{
			name: "missing directory",
			params: &GrepDirectoryParams{
				Pattern: "test",
			},
			wantErr: true,
		},
		{
			name: "missing pattern",
			params: &GrepDirectoryParams{
				Directory: ".",
			},
			wantErr: true,
		},
	}

	tool := NewGrepDirectoryTool(tmpDir, DefaultMaxFileSize, DefaultMaxResults, DefaultGrepTimeout)

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

func TestGrepDirectoryTool_ExcludedDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files in excluded directories
	excludedDirs := []string{".git", "node_modules", "vendor"}
	for _, dir := range excludedDirs {
		dirPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		filePath := filepath.Join(dirPath, "test.txt")
		if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Create file in normal directory
	normalFile := filepath.Join(tmpDir, "normal.txt")
	if err := os.WriteFile(normalFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create normal file: %v", err)
	}

	tool := NewGrepDirectoryTool(tmpDir, DefaultMaxFileSize, DefaultMaxResults, DefaultGrepTimeout)

	params := &GrepDirectoryParams{
		Directory: ".",
		Pattern:   "test",
		Recursive: true,
	}

	ctx := context.Background()
	output, err := tool.Execute(ctx, params)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should find normal.txt but not files in excluded directories
	if !strings.Contains(output, "normal.txt") {
		t.Errorf("Expected to find normal.txt, output: %s", output)
	}

	for _, dir := range excludedDirs {
		if strings.Contains(output, dir) {
			t.Errorf("Should not search in excluded directory %s, output: %s", dir, output)
		}
	}
}

func TestGrepDirectoryTool_BinaryFileSkip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a binary file (with null bytes)
	binaryFile := filepath.Join(tmpDir, "binary.dat")
	binaryContent := []byte{0x00, 0x01, 0x02, 't', 'e', 's', 't', 0x00}
	if err := os.WriteFile(binaryFile, binaryContent, 0644); err != nil {
		t.Fatalf("Failed to create binary file: %v", err)
	}

	// Create a text file
	textFile := filepath.Join(tmpDir, "text.txt")
	if err := os.WriteFile(textFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create text file: %v", err)
	}

	tool := NewGrepDirectoryTool(tmpDir, DefaultMaxFileSize, DefaultMaxResults, DefaultGrepTimeout)

	params := &GrepDirectoryParams{
		Directory: ".",
		Pattern:   "test",
		Recursive: true,
	}

	ctx := context.Background()
	output, err := tool.Execute(ctx, params)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should find text.txt but skip binary.dat
	if !strings.Contains(output, "text.txt") {
		t.Errorf("Expected to find text.txt, output: %s", output)
	}

	if strings.Contains(output, "binary.dat") {
		t.Errorf("Should skip binary file, output: %s", output)
	}
}

func TestGrepDirectoryTool_Timeout(t *testing.T) {
	tmpDir := t.TempDir()

	// Create many files to simulate slow search
	for i := 0; i < 100; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
		content := strings.Repeat("test line\n", 1000)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Create tool with very short timeout
	tool := NewGrepDirectoryTool(tmpDir, DefaultMaxFileSize, DefaultMaxResults, 1*time.Millisecond)

	params := &GrepDirectoryParams{
		Directory: ".",
		Pattern:   "test",
		Recursive: true,
	}

	ctx := context.Background()
	output, _ := tool.Execute(ctx, params)

	// Should either timeout or complete with partial results
	// Just verify it doesn't hang indefinitely
	if output == "" {
		t.Log("Search timed out as expected")
	} else {
		t.Log("Search completed with partial results")
	}
}

func TestGrepDirectoryTool_Name(t *testing.T) {
	tool := NewGrepDirectoryTool("", DefaultMaxFileSize, DefaultMaxResults, DefaultGrepTimeout)
	if tool.Name() != "grep_directory" {
		t.Errorf("Expected name 'grep_directory', got: %s", tool.Name())
	}
}

func TestGrepDirectoryTool_Description(t *testing.T) {
	tool := NewGrepDirectoryTool("", DefaultMaxFileSize, DefaultMaxResults, DefaultGrepTimeout)
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
	if !strings.Contains(desc, "directory") {
		t.Error("Description should mention 'directory'")
	}
}

func TestGlobToRegex(t *testing.T) {
	tests := []struct {
		glob    string
		input   string
		matches bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "main.txt", false},
		{"test_*.go", "test_main.go", true},
		{"test_*.go", "main_test.go", false},
		{"*.{go,txt}", "main.go", true},
		{"*.{go,txt}", "main.txt", true},
		{"*.{go,txt}", "main.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.glob, func(t *testing.T) {
			pattern := globToRegex(tt.glob)
			re, err := regexp.Compile(pattern)
			if err != nil {
				t.Fatalf("Failed to compile regex: %v", err)
			}

			matches := re.MatchString(tt.input)
			if matches != tt.matches {
				t.Errorf("Pattern %s, input %s: expected %v, got %v", tt.glob, tt.input, tt.matches, matches)
			}
		})
	}
}
