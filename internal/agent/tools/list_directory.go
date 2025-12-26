package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListDirectoryParams contains parameters for listing directory contents
type ListDirectoryParams struct {
	Path       string `json:"path"`
	ShowHidden bool   `json:"show_hidden,omitempty"`
	Recursive  bool   `json:"recursive,omitempty"`
	MaxDepth   int    `json:"max_depth,omitempty"`
}

// DirectoryEntry represents a single directory entry
type DirectoryEntry struct {
	Name  string
	IsDir bool
	Size  int64
}

// ListDirectoryTool is a tool for listing directory contents
type ListDirectoryTool struct {
	workDir string
}

// NewListDirectoryTool creates a new ListDirectoryTool
func NewListDirectoryTool(workDir string) *ListDirectoryTool {
	return &ListDirectoryTool{
		workDir: workDir,
	}
}

// Name returns the tool name
func (t *ListDirectoryTool) Name() string {
	return "list_directory"
}

// Description returns the tool description
func (t *ListDirectoryTool) Description() string {
	return `List the contents of a directory. Use this tool to explore the directory structure of the codebase.

Parameters:
- path (required): Path to the directory to list
- show_hidden (optional): If true, show hidden files and directories (default: false)
- recursive (optional): If true, list subdirectories recursively (default: false)
- max_depth (optional): Maximum depth for recursive listing. Only applies when recursive=true (default: 3)

Returns a structured list of files and directories with their types and sizes.

When to use this tool:
- Exploring the structure of a project or directory
- Finding what files exist in a specific location
- Understanding the organization of code

When NOT to use this tool:
- Searching for files by pattern → use list_files instead
- Reading file contents → use read_file instead
- Searching for content in files → use grep_file or grep_directory instead`
}

// Execute runs the tool and returns the directory listing
func (t *ListDirectoryTool) Execute(ctx context.Context, params *ListDirectoryParams) (string, error) {
	if params == nil || params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Resolve path relative to work directory
	dirPath := params.Path
	if !strings.HasPrefix(dirPath, "/") && t.workDir != "" {
		dirPath = filepath.Join(t.workDir, dirPath)
	}

	// Check if directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory not found: %s", params.Path)
		}
		return "", fmt.Errorf("failed to access directory: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", params.Path)
	}

	// Set default max depth
	maxDepth := params.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Directory: %s\n", params.Path))
	result.WriteString(fmt.Sprintf("Recursive: %v", params.Recursive))
	if params.Recursive {
		result.WriteString(fmt.Sprintf(" (max depth: %d)", maxDepth))
	}
	result.WriteString("\n\n")

	if params.Recursive {
		// Recursive listing
		err = t.listRecursive(dirPath, "", params.ShowHidden, maxDepth, 0, &result)
	} else {
		// Non-recursive listing
		err = t.listSingle(dirPath, params.ShowHidden, &result)
	}

	if err != nil {
		return "", err
	}

	return result.String(), nil
}

// listSingle lists a single directory (non-recursive)
func (t *ListDirectoryTool) listSingle(dirPath string, showHidden bool, result *strings.Builder) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Separate directories and files
	var dirs []DirectoryEntry
	var files []DirectoryEntry

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files if not requested
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		// Skip excluded directories
		if entry.IsDir() && ExcludedDirectories[name] {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		de := DirectoryEntry{
			Name:  name,
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		}

		if entry.IsDir() {
			dirs = append(dirs, de)
		} else {
			files = append(files, de)
		}
	}

	// Sort entries
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	// Output directories first
	if len(dirs) > 0 {
		result.WriteString("Directories:\n")
		for _, dir := range dirs {
			result.WriteString(fmt.Sprintf("  %s/\n", dir.Name))
		}
		result.WriteString("\n")
	}

	// Output files
	if len(files) > 0 {
		result.WriteString("Files:\n")
		for _, file := range files {
			sizeStr := formatSize(file.Size)
			result.WriteString(fmt.Sprintf("  %s (%s)\n", file.Name, sizeStr))
		}
	}

	if len(dirs) == 0 && len(files) == 0 {
		result.WriteString("(empty directory)\n")
	}

	return nil
}

// listRecursive lists directory recursively
func (t *ListDirectoryTool) listRecursive(dirPath, prefix string, showHidden bool, maxDepth, currentDepth int, result *strings.Builder) error {
	if currentDepth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Separate directories and files
	var dirs []DirectoryEntry
	var files []DirectoryEntry

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files if not requested
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		// Skip excluded directories
		if entry.IsDir() && ExcludedDirectories[name] {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		de := DirectoryEntry{
			Name:  name,
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		}

		if entry.IsDir() {
			dirs = append(dirs, de)
		} else {
			files = append(files, de)
		}
	}

	// Sort entries
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	// Output directories first
	for _, dir := range dirs {
		result.WriteString(fmt.Sprintf("%s├── %s/\n", prefix, dir.Name))

		// Recurse into subdirectory
		subPath := filepath.Join(dirPath, dir.Name)
		newPrefix := prefix + "│   "
		if err := t.listRecursive(subPath, newPrefix, showHidden, maxDepth, currentDepth+1, result); err != nil {
			// Continue with other directories even if one fails
			continue
		}
	}

	// Output files
	for _, file := range files {
		sizeStr := formatSize(file.Size)
		result.WriteString(fmt.Sprintf("%s├── %s (%s)\n", prefix, file.Name, sizeStr))
	}

	return nil
}

// formatSize formats file size in human-readable format
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
