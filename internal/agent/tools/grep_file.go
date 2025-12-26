package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	// DefaultMaxFileSize is the default maximum file size for grep (10MB)
	DefaultMaxFileSize = 10 * 1024 * 1024
)

// GrepFileParams contains parameters for grep_file tool
type GrepFileParams struct {
	FilePath      string `json:"file_path"`
	Pattern       string `json:"pattern"`
	IgnoreCase    bool   `json:"ignore_case,omitempty"`
	BeforeContext int    `json:"before_context,omitempty"`
	AfterContext  int    `json:"after_context,omitempty"`
	Context       int    `json:"context,omitempty"`
}

// GrepFileTool is a tool for searching content within a file
type GrepFileTool struct {
	workDir     string
	maxFileSize int64
}

// NewGrepFileTool creates a new GrepFileTool
func NewGrepFileTool(workDir string, maxFileSize int64) *GrepFileTool {
	if maxFileSize <= 0 {
		maxFileSize = DefaultMaxFileSize
	}
	return &GrepFileTool{
		workDir:     workDir,
		maxFileSize: maxFileSize,
	}
}

// Name returns the tool name
func (t *GrepFileTool) Name() string {
	return "grep_file"
}

// Description returns the tool description
func (t *GrepFileTool) Description() string {
	return `Search for a pattern within a specific file using regular expressions.
This tool is useful for quickly locating specific functions, variables, or code patterns without reading the entire file.

Parameters:
- file_path (required): Path to the file to search
- pattern (required): Regular expression pattern to search for (Go regexp syntax)
- ignore_case (optional): If true, perform case-insensitive search (default: false)
- before_context (optional): Number of lines to show before each match (like grep -B)
- after_context (optional): Number of lines to show after each match (like grep -A)
- context (optional): Number of lines to show before and after each match (like grep -C)

Returns matching lines with line numbers and optional context lines.

When to use this tool:
- Looking for specific function or variable definitions
- Searching for code patterns or specific keywords
- Need to find where something is used in a file
- Want to see context around matches

When NOT to use this tool:
- Need to read the entire file → use read_file instead
- Don't know which file contains the content → use grep_directory instead`
}

// Execute runs the grep search on the specified file
func (t *GrepFileTool) Execute(ctx context.Context, params *GrepFileParams) (string, error) {
	if params == nil || params.FilePath == "" {
		return "", fmt.Errorf("file_path is required")
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	// Resolve file path
	filePath := params.FilePath
	if !strings.HasPrefix(filePath, "/") && t.workDir != "" {
		filePath = t.workDir + "/" + filePath
	}

	// Check if file exists and get size
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", params.FilePath)
		}
		return "", fmt.Errorf("failed to access file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s. Use grep_directory instead", params.FilePath)
	}

	// Check file size
	if info.Size() > t.maxFileSize {
		return "", fmt.Errorf("file too large (%d bytes, max: %d bytes): %s. Consider using more specific patterns or read_file with line ranges",
			info.Size(), t.maxFileSize, params.FilePath)
	}

	// Compile regex pattern
	var re *regexp.Regexp
	if params.IgnoreCase {
		re, err = regexp.Compile("(?i)" + params.Pattern)
	} else {
		re, err = regexp.Compile(params.Pattern)
	}
	if err != nil {
		return "", fmt.Errorf("invalid regular expression pattern: %w", err)
	}

	// Determine context lines
	beforeLines := params.BeforeContext
	afterLines := params.AfterContext
	if params.Context > 0 {
		beforeLines = params.Context
		afterLines = params.Context
	}

	// Open and read file
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	// Find matches
	var matches []int
	for i, line := range lines {
		if re.MatchString(line) {
			matches = append(matches, i)
		}
	}

	// Build result
	if len(matches) == 0 {
		return fmt.Sprintf("No matches found for pattern '%s' in file: %s", params.Pattern, params.FilePath), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("File: %s\n", params.FilePath))
	result.WriteString(fmt.Sprintf("Matches: %d\n", len(matches)))
	result.WriteString(fmt.Sprintf("Total lines: %d\n", len(lines)))
	result.WriteString("\n")

	// Output matches with context
	for _, matchIdx := range matches {
		result.WriteString(fmt.Sprintf("Match at line %d:\n", matchIdx+1))

		// Calculate context range
		startLine := matchIdx - beforeLines
		if startLine < 0 {
			startLine = 0
		}
		endLine := matchIdx + afterLines
		if endLine >= len(lines) {
			endLine = len(lines) - 1
		}

		// Output context lines
		for i := startLine; i <= endLine; i++ {
			lineNum := i + 1
			prefix := "  "
			if i == matchIdx {
				prefix = "→ " // Mark the matched line
			}
			result.WriteString(fmt.Sprintf("%s%5d | %s\n", prefix, lineNum, lines[i]))
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}
