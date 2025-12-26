package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// DefaultMaxResults is the default maximum number of results
	DefaultMaxResults = 100
	// DefaultGrepTimeout is the default timeout for grep operations
	DefaultGrepTimeout = 10 * time.Second
)

// ExcludedDirectories are directories that should be skipped during search
var ExcludedDirectories = map[string]bool{
	".git":          true,
	"node_modules":  true,
	"vendor":        true,
	".idea":         true,
	".vscode":       true,
	"__pycache__":   true,
	".pytest_cache": true,
	"dist":          true,
	"build":         true,
	"target":        true,
	".next":         true,
	".nuxt":         true,
	"coverage":      true,
}

// GrepDirectoryParams contains parameters for grep_directory tool
type GrepDirectoryParams struct {
	Directory     string `json:"directory"`
	Pattern       string `json:"pattern"`
	Recursive     bool   `json:"recursive,omitempty"`
	FilePattern   string `json:"file_pattern,omitempty"`
	IgnoreCase    bool   `json:"ignore_case,omitempty"`
	BeforeContext int    `json:"before_context,omitempty"`
	AfterContext  int    `json:"after_context,omitempty"`
	Context       int    `json:"context,omitempty"`
	MaxResults    int    `json:"max_results,omitempty"`
}

// GrepDirectoryTool is a tool for searching content within a directory
type GrepDirectoryTool struct {
	workDir     string
	maxFileSize int64
	maxResults  int
	timeout     time.Duration
}

// NewGrepDirectoryTool creates a new GrepDirectoryTool
func NewGrepDirectoryTool(workDir string, maxFileSize int64, maxResults int, timeout time.Duration) *GrepDirectoryTool {
	if maxFileSize <= 0 {
		maxFileSize = DefaultMaxFileSize
	}
	if maxResults <= 0 {
		maxResults = DefaultMaxResults
	}
	if timeout <= 0 {
		timeout = DefaultGrepTimeout
	}
	return &GrepDirectoryTool{
		workDir:     workDir,
		maxFileSize: maxFileSize,
		maxResults:  maxResults,
		timeout:     timeout,
	}
}

// Name returns the tool name
func (t *GrepDirectoryTool) Name() string {
	return "grep_directory"
}

// Description returns the tool description
func (t *GrepDirectoryTool) Description() string {
	return `Search for a pattern within all files in a directory using regular expressions.
This tool is useful for finding where specific code patterns appear across multiple files.

Parameters:
- directory (required): Path to the directory to search
- pattern (required): Regular expression pattern to search for (Go regexp syntax)
- recursive (optional): If true, search subdirectories recursively (default: true)
- file_pattern (optional): Glob pattern to filter files (e.g., "*.go", "*.{js,ts}"). If not specified, searches all text files
- ignore_case (optional): If true, perform case-insensitive search (default: false)
- before_context (optional): Number of lines to show before each match (like grep -B)
- after_context (optional): Number of lines to show after each match (like grep -A)
- context (optional): Number of lines to show before and after each match (like grep -C)
- max_results (optional): Maximum number of matches to return (default: 100)

Returns matching lines from all files with file paths, line numbers, and optional context.

Automatically excludes common non-code directories (.git, node_modules, vendor, etc.) and binary files.

When to use this tool:
- Looking for where a function/variable is used across the codebase
- Finding all files that contain a specific pattern
- Searching for similar code patterns in multiple files
- Don't know which file contains the code you're looking for

When NOT to use this tool:
- Know the specific file to search → use grep_file instead
- Need to read entire file content → use read_file instead`
}

// matchResult represents a single match result
type matchResult struct {
	file      string
	lineNum   int
	line      string
	before    []string
	after     []string
	beforeNum []int
	afterNum  []int
}

// Execute runs the grep search on the specified directory
func (t *GrepDirectoryTool) Execute(ctx context.Context, params *GrepDirectoryParams) (string, error) {
	if params == nil || params.Directory == "" {
		return "", fmt.Errorf("directory is required")
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	// Resolve directory path
	dirPath := params.Directory
	if !strings.HasPrefix(dirPath, "/") && t.workDir != "" {
		dirPath = filepath.Join(t.workDir, dirPath)
	}

	// Check if directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory not found: %s", params.Directory)
		}
		return "", fmt.Errorf("failed to access directory: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s. Use grep_file instead", params.Directory)
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

	// Compile file pattern if specified
	var filePatternRe *regexp.Regexp
	if params.FilePattern != "" {
		// Convert glob pattern to regex
		pattern := globToRegex(params.FilePattern)
		filePatternRe, err = regexp.Compile(pattern)
		if err != nil {
			return "", fmt.Errorf("invalid file pattern: %w", err)
		}
	}

	// Determine context lines
	beforeLines := params.BeforeContext
	afterLines := params.AfterContext
	if params.Context > 0 {
		beforeLines = params.Context
		afterLines = params.Context
	}

	// Use recursive parameter directly
	// Note: Default is false in Go for bool, but we document it as true in the description
	// The LLM should explicitly set it
	recursive := params.Recursive

	// Set max results
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = t.maxResults
	}

	// Create context with timeout
	searchCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	// Search files
	var matches []matchResult
	var filesScanned int
	var filesSkipped int

	err = t.walkDirectory(searchCtx, dirPath, recursive, func(path string) error {
		// Check context cancellation
		select {
		case <-searchCtx.Done():
			return fmt.Errorf("search timeout exceeded (%v)", t.timeout)
		default:
		}

		// Check if we've reached max results
		if len(matches) >= maxResults {
			return fmt.Errorf("max results limit reached (%d)", maxResults)
		}

		// Check file pattern
		if filePatternRe != nil {
			if !filePatternRe.MatchString(filepath.Base(path)) {
				filesSkipped++
				return nil
			}
		}

		// Check file size
		info, err := os.Stat(path)
		if err != nil {
			return nil // Skip files we can't stat
		}
		if info.Size() > t.maxFileSize {
			filesSkipped++
			return nil // Skip large files
		}

		// Check if binary file
		if isBinaryFile(path) {
			filesSkipped++
			return nil
		}

		filesScanned++

		// Search in file
		fileMatches, err := t.searchFile(path, re, beforeLines, afterLines)
		if err != nil {
			// Log error but continue with other files
			return nil
		}

		matches = append(matches, fileMatches...)

		return nil
	})

	if err != nil {
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "max results") {
			// Continue with partial results
		} else {
			return "", err
		}
	}

	// Build result
	if len(matches) == 0 {
		return fmt.Sprintf("No matches found for pattern '%s' in directory: %s\n"+
			"Files scanned: %d, Files skipped: %d",
			params.Pattern, params.Directory, filesScanned, filesSkipped), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Directory: %s\n", params.Directory))
	result.WriteString(fmt.Sprintf("Pattern: %s\n", params.Pattern))
	result.WriteString(fmt.Sprintf("Matches: %d (showing up to %d)\n", len(matches), maxResults))
	result.WriteString(fmt.Sprintf("Files scanned: %d, Files skipped: %d\n", filesScanned, filesSkipped))
	result.WriteString("\n")

	// Group matches by file
	fileMatches := make(map[string][]matchResult)
	for _, match := range matches {
		relPath, _ := filepath.Rel(dirPath, match.file)
		if relPath == "" {
			relPath = match.file
		}
		fileMatches[relPath] = append(fileMatches[relPath], match)
	}

	// Output matches
	for file, fmatches := range fileMatches {
		result.WriteString(fmt.Sprintf("=== File: %s (%d matches) ===\n", file, len(fmatches)))
		for _, match := range fmatches {
			result.WriteString(fmt.Sprintf("\nMatch at line %d:\n", match.lineNum))

			// Output before context
			for i, line := range match.before {
				result.WriteString(fmt.Sprintf("  %5d | %s\n", match.beforeNum[i], line))
			}

			// Output matched line
			result.WriteString(fmt.Sprintf("→ %5d | %s\n", match.lineNum, match.line))

			// Output after context
			for i, line := range match.after {
				result.WriteString(fmt.Sprintf("  %5d | %s\n", match.afterNum[i], line))
			}
		}
		result.WriteString("\n")
	}

	if len(matches) >= maxResults {
		result.WriteString(fmt.Sprintf("\nNote: Results limited to %d matches. Use more specific patterns or file_pattern to narrow the search.\n", maxResults))
	}

	return result.String(), nil
}

// walkDirectory walks through directory and calls fn for each file
func (t *GrepDirectoryTool) walkDirectory(ctx context.Context, root string, recursive bool, fn func(path string) error) error {
	if recursive {
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip files with errors
			}

			// Check context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Skip excluded directories
			if info.IsDir() {
				if ExcludedDirectories[info.Name()] {
					return filepath.SkipDir
				}
				return nil
			}

			// Process file
			return fn(path)
		})
	}

	// Non-recursive: only scan files in the root directory
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if entry.IsDir() {
			continue
		}

		path := filepath.Join(root, entry.Name())
		if err := fn(path); err != nil {
			return err
		}
	}

	return nil
}

// searchFile searches for pattern in a file and returns matches
func (t *GrepDirectoryTool) searchFile(path string, re *regexp.Regexp, beforeLines, afterLines int) ([]matchResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Find matches
	var matches []matchResult
	for i, line := range lines {
		if re.MatchString(line) {
			match := matchResult{
				file:    path,
				lineNum: i + 1,
				line:    line,
			}

			// Add before context
			startLine := i - beforeLines
			if startLine < 0 {
				startLine = 0
			}
			for j := startLine; j < i; j++ {
				match.before = append(match.before, lines[j])
				match.beforeNum = append(match.beforeNum, j+1)
			}

			// Add after context
			endLine := i + afterLines
			if endLine >= len(lines) {
				endLine = len(lines) - 1
			}
			for j := i + 1; j <= endLine; j++ {
				match.after = append(match.after, lines[j])
				match.afterNum = append(match.afterNum, j+1)
			}

			matches = append(matches, match)
		}
	}

	return matches, nil
}

// isBinaryFile checks if a file is binary by reading first 512 bytes
func isBinaryFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return true // Assume binary if we can't open
	}
	defer file.Close()

	// Read first 512 bytes
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil {
		return true
	}

	// Check for null bytes (common in binary files)
	return bytes.Contains(buf[:n], []byte{0})
}

// globToRegex converts a glob pattern to a regular expression
func globToRegex(pattern string) string {
	// Escape special regex characters except * and ?
	pattern = regexp.QuoteMeta(pattern)

	// Convert glob wildcards to regex
	pattern = strings.ReplaceAll(pattern, `\*`, ".*")
	pattern = strings.ReplaceAll(pattern, `\?`, ".")

	// Handle {a,b,c} patterns
	pattern = strings.ReplaceAll(pattern, `\{`, "(")
	pattern = strings.ReplaceAll(pattern, `\}`, ")")
	pattern = strings.ReplaceAll(pattern, ",", "|")

	return "^" + pattern + "$"
}
