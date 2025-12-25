package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

const (
	// DefaultLinesNoRange is the number of lines to read when no range is specified
	DefaultLinesNoRange = 200
	// DefaultMaxLinesPerRead is the default maximum lines per read
	DefaultMaxLinesPerRead = 1000
)

// ReadFileParams contains parameters for reading a file
type ReadFileParams struct {
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

// ReadFileTool is a tool for reading file contents
type ReadFileTool struct {
	workDir         string
	maxLinesPerRead int
}

// NewReadFileTool creates a new ReadFileTool
func NewReadFileTool(workDir string, maxLinesPerRead int) *ReadFileTool {
	if maxLinesPerRead <= 0 {
		maxLinesPerRead = DefaultMaxLinesPerRead
	}
	return &ReadFileTool{
		workDir:         workDir,
		maxLinesPerRead: maxLinesPerRead,
	}
}

// Name returns the tool name
func (t *ReadFileTool) Name() string {
	return "read_file"
}

// Description returns the tool description
func (t *ReadFileTool) Description() string {
	return `Read the contents of a file. Use this tool to examine source code for deeper analysis.
Parameters:
- file_path (required): Path to the file to read
- start_line (optional): Starting line number (1-indexed). If not specified, reads from the beginning.
- end_line (optional): Ending line number (1-indexed, inclusive). If not specified, reads up to 200 lines by default.
Returns the file contents with line numbers prefixed to each line.
Note: There is a maximum line limit per read. If the requested range exceeds this limit, it will be truncated.`
}

// Execute runs the tool and returns the file contents
func (t *ReadFileTool) Execute(ctx context.Context, params *ReadFileParams) (string, error) {
	if params == nil || params.FilePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	// Resolve file path relative to work directory
	filePath := params.FilePath
	if !strings.HasPrefix(filePath, "/") && t.workDir != "" {
		filePath = t.workDir + "/" + filePath
	}

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", params.FilePath)
		}
		return "", fmt.Errorf("failed to access file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", params.FilePath)
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Determine line range
	startLine := params.StartLine
	endLine := params.EndLine

	// If no range specified, read first DefaultLinesNoRange lines
	noRangeSpecified := startLine <= 0 && endLine <= 0
	if noRangeSpecified {
		startLine = 1
		endLine = DefaultLinesNoRange
	}

	// Validate and adjust line numbers
	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= 0 || endLine < startLine {
		endLine = startLine + t.maxLinesPerRead - 1
	}

	// Enforce maximum lines limit
	requestedLines := endLine - startLine + 1
	truncated := false
	if requestedLines > t.maxLinesPerRead {
		endLine = startLine + t.maxLinesPerRead - 1
		truncated = true
	}

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	var result strings.Builder
	currentLine := 0
	linesRead := 0
	totalLines := 0

	for scanner.Scan() {
		currentLine++
		totalLines = currentLine

		if currentLine < startLine {
			continue
		}
		if currentLine > endLine {
			// Continue counting total lines
			continue
		}

		linesRead++
		result.WriteString(fmt.Sprintf("%6d | %s\n", currentLine, scanner.Text()))
	}

	// Count remaining lines
	for scanner.Scan() {
		totalLines++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	// Build response with metadata
	var response strings.Builder
	response.WriteString(fmt.Sprintf("File: %s\n", params.FilePath))
	response.WriteString(fmt.Sprintf("Lines: %d-%d (total lines in file: %d)\n", startLine, startLine+linesRead-1, totalLines))

	if truncated {
		response.WriteString(fmt.Sprintf("Note: Output truncated to %d lines (max_lines_per_read limit)\n", t.maxLinesPerRead))
	}

	hasMore := totalLines > endLine
	if hasMore {
		response.WriteString(fmt.Sprintf("Note: File has more content after line %d\n", endLine))
	}

	if noRangeSpecified && hasMore {
		response.WriteString(fmt.Sprintf("Tip: Use start_line and end_line parameters to read specific sections\n"))
	}

	response.WriteString("---\n")
	response.WriteString(result.String())

	return response.String(), nil
}
