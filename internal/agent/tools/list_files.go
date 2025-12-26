package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// DefaultMaxFiles is the default maximum number of files to return
	DefaultMaxFiles = 100
)

// ListFilesParams contains parameters for finding files by pattern
type ListFilesParams struct {
	Pattern     string   `json:"pattern"`
	Path        string   `json:"path"`
	ExcludeDirs []string `json:"exclude_dirs,omitempty"`
	MaxResults  int      `json:"max_results,omitempty"`
}

// ListFilesTool is a tool for finding files matching a glob pattern
type ListFilesTool struct {
	workDir    string
	maxResults int
}

// NewListFilesTool creates a new ListFilesTool
func NewListFilesTool(workDir string, maxResults int) *ListFilesTool {
	if maxResults <= 0 {
		maxResults = DefaultMaxFiles
	}
	return &ListFilesTool{
		workDir:    workDir,
		maxResults: maxResults,
	}
}

// Name returns the tool name
func (t *ListFilesTool) Name() string {
	return "list_files"
}

// Description returns the tool description
func (t *ListFilesTool) Description() string {
	return `Find files matching a glob pattern. Use this tool to locate specific files in the codebase.

Parameters:
- pattern (required): Glob pattern to match files (e.g., "*.go", "**/*.js", "test_*.py")
  - * matches any characters within a single directory
  - ** matches any characters across directories (recursive)
  - ? matches a single character
  - {a,b,c} matches any of the alternatives
- path (required): Root directory to start searching from
- exclude_dirs (optional): List of directory names to exclude from search (e.g., ["node_modules", "vendor"])
- max_results (optional): Maximum number of files to return (default: 100)

Returns a list of file paths matching the pattern, relative to the search path.

Automatically excludes common non-code directories (.git, node_modules, vendor, etc.) unless explicitly included.

When to use this tool:
- Finding all files of a specific type (e.g., all .go files)
- Locating test files (e.g., *_test.go)
- Finding files by naming pattern
- Getting a list of files before reading them

When NOT to use this tool:
- Exploring directory structure → use list_directory instead
- Searching for content within files → use grep_directory instead
- Reading file contents → use read_file instead`
}

// Execute runs the tool and returns matching file paths
func (t *ListFilesTool) Execute(ctx context.Context, params *ListFilesParams) (string, error) {
	if params == nil || params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Resolve path relative to work directory
	searchPath := params.Path
	if !strings.HasPrefix(searchPath, "/") && t.workDir != "" {
		searchPath = filepath.Join(t.workDir, searchPath)
	}

	// Check if path exists
	info, err := os.Stat(searchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path not found: %s", params.Path)
		}
		return "", fmt.Errorf("failed to access path: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", params.Path)
	}

	// Build exclude map
	excludeDirs := make(map[string]bool)
	// Add default excluded directories
	for dir := range ExcludedDirectories {
		excludeDirs[dir] = true
	}
	// Add user-specified excluded directories
	for _, dir := range params.ExcludeDirs {
		excludeDirs[dir] = true
	}

	// Set max results
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = t.maxResults
	}

	// Find matching files
	var matches []string
	var filesScanned int
	var dirsSkipped int

	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if we've reached max results
		if len(matches) >= maxResults {
			return filepath.SkipAll
		}

		// Skip excluded directories
		if info.IsDir() {
			if excludeDirs[info.Name()] {
				dirsSkipped++
				return filepath.SkipDir
			}
			return nil
		}

		filesScanned++

		// Get relative path
		relPath, err := filepath.Rel(searchPath, path)
		if err != nil {
			relPath = path
		}

		// Check if file matches pattern
		matched, err := filepath.Match(params.Pattern, filepath.Base(path))
		if err != nil {
			return nil // Skip invalid patterns
		}

		// Also try matching the full relative path for ** patterns
		if !matched {
			matched, _ = filepath.Match(params.Pattern, relPath)
		}

		// For ** patterns, we need custom matching
		if !matched && strings.Contains(params.Pattern, "**") {
			matched = matchGlobPattern(params.Pattern, relPath)
		}

		if matched {
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", fmt.Errorf("error walking directory: %w", err)
	}

	// Sort matches
	sort.Strings(matches)

	// Build result
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Pattern: %s\n", params.Pattern))
	result.WriteString(fmt.Sprintf("Search path: %s\n", params.Path))
	result.WriteString(fmt.Sprintf("Matches: %d", len(matches)))
	if len(matches) >= maxResults {
		result.WriteString(fmt.Sprintf(" (limited to %d)", maxResults))
	}
	result.WriteString("\n")
	result.WriteString(fmt.Sprintf("Files scanned: %d, Directories skipped: %d\n", filesScanned, dirsSkipped))
	result.WriteString("\n")

	if len(matches) == 0 {
		result.WriteString("No files found matching the pattern.\n")
		return result.String(), nil
	}

	result.WriteString("Matching files:\n")
	for _, match := range matches {
		result.WriteString(fmt.Sprintf("  %s\n", match))
	}

	if len(matches) >= maxResults {
		result.WriteString(fmt.Sprintf("\nNote: Results limited to %d files. Use more specific patterns or reduce the search scope.\n", maxResults))
	}

	return result.String(), nil
}

// matchGlobPattern matches a glob pattern with ** support
func matchGlobPattern(pattern, path string) bool {
	// Split pattern and path by /
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	return matchParts(patternParts, pathParts)
}

// matchParts recursively matches pattern parts against path parts
func matchParts(patternParts, pathParts []string) bool {
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}

	if len(pathParts) == 0 {
		// Check if remaining pattern parts are all **
		for _, p := range patternParts {
			if p != "**" {
				return false
			}
		}
		return true
	}

	pattern := patternParts[0]
	path := pathParts[0]

	if pattern == "**" {
		// ** can match zero or more path segments
		// Try matching with current segment consumed
		if matchParts(patternParts, pathParts[1:]) {
			return true
		}
		// Try matching with ** still active
		if matchParts(patternParts[1:], pathParts) {
			return true
		}
		// Try matching ** with one path segment
		return matchParts(patternParts[1:], pathParts[1:])
	}

	// Regular pattern matching
	matched, err := filepath.Match(pattern, path)
	if err != nil || !matched {
		return false
	}

	return matchParts(patternParts[1:], pathParts[1:])
}
