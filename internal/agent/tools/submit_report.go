package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SubmitReportParams contains parameters for submitting a debug report
type SubmitReportParams struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// DebugReport represents a saved debug report
type DebugReport struct {
	Title    string
	Content  string
	IssueID  int
	Date     string
	FilePath string
}

// SubmitReportTool is a tool for submitting and saving debug reports
type SubmitReportTool struct {
	issuesDir string
}

// NewSubmitReportTool creates a new SubmitReportTool
func NewSubmitReportTool(issuesDir string) *SubmitReportTool {
	if issuesDir == "" {
		issuesDir = "./issues"
	}
	return &SubmitReportTool{
		issuesDir: issuesDir,
	}
}

// Name returns the tool name
func (t *SubmitReportTool) Name() string {
	return "submit_report"
}

// Description returns the tool description
func (t *SubmitReportTool) Description() string {
	return `Submit the final analysis report after completing the investigation.
Use this tool when you have finished analyzing the issue and are ready to provide conclusions and recommendations.

Parameters:
- title (required): A concise title for the report (will be used in the filename)
- content (required): The complete report content in Markdown format

The content should be a well-structured Markdown document including:
1. **Problem Description**: What issue was being investigated
2. **Analysis Process**: Steps taken and findings at each step
3. **Conclusions**: Root cause or key findings
4. **Solutions**: Recommended fixes or approaches
5. **Verification Plan**: How to verify the solution works
6. **Unresolved Items** (if applicable): What remains unclear or needs further investigation

Example structure:
# {Title}

## Problem Description
[User's original question and context]

## Analysis Process
1. [Step 1: What was done and what was found]
2. [Step 2: Follow-up investigation]
...

## Conclusions
[Root cause or key findings with supporting evidence]

## Solutions
- **Solution 1**: [Detailed approach]
- **Solution 2**: [Alternative approach]

## Verification Plan
[How to test and verify the fix]

## Unresolved Items
[If applicable: what needs more investigation]

The report will be saved to the issues directory with a unique filename.
Returns a confirmation message with the saved file path.`
}

// Execute runs the tool and saves the report
func (t *SubmitReportTool) Execute(ctx context.Context, params *SubmitReportParams) (string, error) {
	if params == nil {
		return "", fmt.Errorf("params is required")
	}

	if params.Title == "" {
		return "", fmt.Errorf("title is required")
	}

	if params.Content == "" {
		return "", fmt.Errorf("content is required")
	}

	// Create issues directory if it doesn't exist
	if err := os.MkdirAll(t.issuesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create issues directory: %w", err)
	}

	// Get next issue ID
	issueID, err := t.getNextIssueID()
	if err != nil {
		return "", fmt.Errorf("failed to get next issue ID: %w", err)
	}

	// Generate filename
	date := time.Now().Format("2006-01-02")
	slug := t.titleToSlug(params.Title)
	filename := fmt.Sprintf("issue-%03d-%s-%s.md", issueID, slug, date)
	filePath := filepath.Join(t.issuesDir, filename)

	// Write report to file
	if err := os.WriteFile(filePath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("failed to write report file: %w", err)
	}

	report := &DebugReport{
		Title:    params.Title,
		Content:  params.Content,
		IssueID:  issueID,
		Date:     date,
		FilePath: filePath,
	}

	// Return success message
	return t.formatSuccessMessage(report), nil
}

// getNextIssueID scans the issues directory and returns the next available issue ID
func (t *SubmitReportTool) getNextIssueID() (int, error) {
	entries, err := os.ReadDir(t.issuesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil // First issue
		}
		return 0, err
	}

	maxID := 0
	issuePattern := regexp.MustCompile(`^issue-(\d+)-`)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := issuePattern.FindStringSubmatch(entry.Name())
		if len(matches) > 1 {
			var id int
			fmt.Sscanf(matches[1], "%d", &id)
			if id > maxID {
				maxID = id
			}
		}
	}

	return maxID + 1, nil
}

// titleToSlug converts a title to a URL-friendly slug
func (t *SubmitReportTool) titleToSlug(title string) string {
	// Convert to lowercase
	slug := strings.ToLower(title)

	// Replace spaces and special characters with hyphens
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")

	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	// Limit length
	if len(slug) > 50 {
		slug = slug[:50]
		// Trim at last hyphen to avoid cutting words
		if lastHyphen := strings.LastIndex(slug, "-"); lastHyphen > 0 {
			slug = slug[:lastHyphen]
		}
	}

	// Fallback if slug is empty
	if slug == "" {
		slug = "report"
	}

	return slug
}

// formatSuccessMessage formats a success message for the report submission
func (t *SubmitReportTool) formatSuccessMessage(report *DebugReport) string {
	var msg strings.Builder

	msg.WriteString("✅ Debug report successfully saved!\n\n")
	msg.WriteString(fmt.Sprintf("Report ID: #%03d\n", report.IssueID))
	msg.WriteString(fmt.Sprintf("Title: %s\n", report.Title))
	msg.WriteString(fmt.Sprintf("Date: %s\n", report.Date))
	msg.WriteString(fmt.Sprintf("File: %s\n", report.FilePath))
	msg.WriteString("\nThe analysis is complete. The report has been saved for future reference.")

	return msg.String()
}
