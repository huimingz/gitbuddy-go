package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubmitReportTool_Name(t *testing.T) {
	tool := NewSubmitReportTool("./issues")
	if tool.Name() != "submit_report" {
		t.Errorf("expected name 'submit_report', got '%s'", tool.Name())
	}
}

func TestSubmitReportTool_Description(t *testing.T) {
	tool := NewSubmitReportTool("./issues")
	desc := tool.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
	if !strings.Contains(desc, "report") {
		t.Error("description should mention report")
	}
}

func TestSubmitReportTool_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		params  *SubmitReportParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid report",
			params: &SubmitReportParams{
				Title:   "Memory Leak Investigation",
				Content: "# Memory Leak Investigation\n\n## Problem\nMemory usage growing...",
			},
			wantErr: false,
		},
		{
			name: "another valid report",
			params: &SubmitReportParams{
				Title:   "Database Performance Issue",
				Content: "# Database Performance\n\n## Analysis\nSlow queries found...",
			},
			wantErr: false,
		},
		{
			name:    "nil params",
			params:  nil,
			wantErr: true,
			errMsg:  "params is required",
		},
		{
			name: "empty title",
			params: &SubmitReportParams{
				Title:   "",
				Content: "Some content",
			},
			wantErr: true,
			errMsg:  "title is required",
		},
		{
			name: "empty content",
			params: &SubmitReportParams{
				Title:   "Test Report",
				Content: "",
			},
			wantErr: true,
			errMsg:  "content is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a separate temp dir for each test
			tmpDir := t.TempDir()
			tool := NewSubmitReportTool(tmpDir)

			result, err := tool.Execute(ctx, tt.params)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got: %v", tt.errMsg, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check success message
			if !strings.Contains(result, "successfully saved") {
				t.Error("result should contain success message")
			}

			if !strings.Contains(result, tt.params.Title) {
				t.Error("result should contain the report title")
			}

			// Verify file was created
			entries, err := os.ReadDir(tmpDir)
			if err != nil {
				t.Fatalf("failed to read issues directory: %v", err)
			}

			// Find the created file
			var foundFile bool
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".md") {
					foundFile = true

					// Read and verify content
					filePath := filepath.Join(tmpDir, entry.Name())
					content, err := os.ReadFile(filePath)
					if err != nil {
						t.Fatalf("failed to read report file: %v", err)
					}

					if string(content) != tt.params.Content {
						t.Error("file content does not match expected content")
					}
					break
				}
			}

			if !foundFile {
				t.Error("report file was not created")
			}
		})
	}
}

func TestSubmitReportTool_Execute_IssueIDIncrement(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewSubmitReportTool(tmpDir)
	ctx := context.Background()

	// Create first report
	params1 := &SubmitReportParams{
		Title:   "First Report",
		Content: "Content 1",
	}
	result1, err := tool.Execute(ctx, params1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be issue #001
	if !strings.Contains(result1, "#001") {
		t.Error("first report should be #001")
	}

	// Create second report
	params2 := &SubmitReportParams{
		Title:   "Second Report",
		Content: "Content 2",
	}
	result2, err := tool.Execute(ctx, params2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be issue #002
	if !strings.Contains(result2, "#002") {
		t.Error("second report should be #002")
	}

	// Verify both files exist
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read issues directory: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 report files, got %d", len(entries))
	}
}

func TestSubmitReportTool_TitleToSlug(t *testing.T) {
	tool := NewSubmitReportTool("./issues")

	tests := []struct {
		title string
		want  string
	}{
		{
			title: "Memory Leak Investigation",
			want:  "memory-leak-investigation",
		},
		{
			title: "Database Performance Issue",
			want:  "database-performance-issue",
		},
		{
			title: "API Error: 500 Internal Server Error",
			want:  "api-error-500-internal-server-error",
		},
		{
			title: "Special!@#$%Characters&*()",
			want:  "special-characters",
		},
		{
			title: "   Leading and Trailing Spaces   ",
			want:  "leading-and-trailing-spaces",
		},
		{
			title: "",
			want:  "report",
		},
		{
			title: "A Very Long Title That Exceeds The Maximum Length Limit And Should Be Truncated At Some Point",
			want:  "a-very-long-title-that-exceeds-the-maximum-length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := tool.titleToSlug(tt.title)
			if got != tt.want {
				t.Errorf("titleToSlug(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestSubmitReportTool_GetNextIssueID(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewSubmitReportTool(tmpDir)

	// Test with empty directory
	id, err := tool.getNextIssueID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 1 {
		t.Errorf("expected first issue ID to be 1, got %d", id)
	}

	// Create some existing issue files
	os.WriteFile(filepath.Join(tmpDir, "issue-001-test-2024-01-01.md"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "issue-003-another-2024-01-02.md"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "issue-002-middle-2024-01-01.md"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "not-an-issue.md"), []byte("content"), 0644)

	// Next ID should be 4 (max is 3)
	id, err = tool.getNextIssueID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 4 {
		t.Errorf("expected next issue ID to be 4, got %d", id)
	}
}

func TestSubmitReportTool_DefaultIssuesDir(t *testing.T) {
	tool := NewSubmitReportTool("")
	if tool.issuesDir != "./issues" {
		t.Errorf("expected default issues dir to be './issues', got '%s'", tool.issuesDir)
	}
}
