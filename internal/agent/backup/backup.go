package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupInfo contains metadata about a backup
type BackupInfo struct {
	Path         string    `json:"path"`
	OriginalFile string    `json:"original_file"`
	Operation    string    `json:"operation"`
	CreatedAt    time.Time `json:"created_at"`
	Size         int64     `json:"size"`
}

// BackupManager manages file backups within a working directory
type BackupManager struct {
	workDir string
}

// NewBackupManager creates a new BackupManager
func NewBackupManager(workDir string) *BackupManager {
	return &BackupManager{
		workDir: workDir,
	}
}

// CreateBackup creates a timestamped backup of the specified file
func (m *BackupManager) CreateBackup(ctx context.Context, filePath, operation string) (string, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("backup creation cancelled: %w", ctx.Err())
	default:
	}

	if filePath == "" {
		return "", fmt.Errorf("file path is required")
	}
	if operation == "" {
		return "", fmt.Errorf("operation name is required")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", filePath)
	} else if err != nil {
		return "", fmt.Errorf("failed to check file: %w", err)
	}

	// Generate backup path
	backupPath, err := m.generateBackupPath(filePath, operation)
	if err != nil {
		return "", fmt.Errorf("failed to generate backup path: %w", err)
	}

	// Ensure backup directory exists
	backupDir := filepath.Dir(backupPath)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy file to backup location
	if err := m.copyFile(filePath, backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupPath, nil
}

// RestoreBackup restores a file from a backup
func (m *BackupManager) RestoreBackup(ctx context.Context, backupPath, targetPath string) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("backup restoration cancelled: %w", ctx.Err())
	default:
	}

	if backupPath == "" {
		return fmt.Errorf("backup path is required")
	}
	if targetPath == "" {
		return fmt.Errorf("target path is required")
	}

	// Check if backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", backupPath)
	} else if err != nil {
		return fmt.Errorf("failed to check backup file: %w", err)
	}

	// Ensure target directory exists
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Copy backup to target location
	if err := m.copyFile(backupPath, targetPath); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	return nil
}

// ListBackups returns a list of backups for a specific file, sorted by creation time (newest first)
func (m *BackupManager) ListBackups(ctx context.Context, filePath string) ([]BackupInfo, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("backup listing cancelled: %w", ctx.Err())
	default:
	}

	backupDir := m.GetBackupDirectory(filePath)

	// If backup directory doesn't exist, return empty list
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return []BackupInfo{}, nil
	}

	// Find all backup files for this original file
	pattern := m.getBackupPattern(filePath)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find backups: %w", err)
	}

	var backups []BackupInfo
	for _, backupPath := range matches {
		info, err := m.getBackupInfo(backupPath, filePath)
		if err != nil {
			// Skip invalid backups but continue processing
			continue
		}
		backups = append(backups, info)
	}

	// Sort by creation time, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// CleanupOldBackups removes old backups, keeping only the specified number of most recent backups
func (m *BackupManager) CleanupOldBackups(ctx context.Context, filePath string, keepCount int) (int, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return 0, fmt.Errorf("backup cleanup cancelled: %w", ctx.Err())
	default:
	}

	backups, err := m.ListBackups(ctx, filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) <= keepCount {
		return 0, nil // Nothing to clean up
	}

	// Remove old backups (keep the first keepCount items since they're sorted newest first)
	toRemove := backups[keepCount:]
	removed := 0

	for _, backup := range toRemove {
		if err := os.Remove(backup.Path); err != nil {
			// Log error but continue with other backups
			continue
		}
		removed++
	}

	return removed, nil
}

// GetBackupDirectory returns the backup directory for a given file
func (m *BackupManager) GetBackupDirectory(filePath string) string {
	// Convert file path to relative path within working directory
	relPath, err := filepath.Rel(m.workDir, filePath)
	if err != nil {
		// If relative path calculation fails, use the base name
		relPath = filepath.Base(filePath)
	}

	// Create backup directory structure that mirrors the original file structure
	backupDir := filepath.Join(m.workDir, ".gitbuddy-backups", filepath.Dir(relPath))
	return backupDir
}

// generateBackupPath creates a unique backup path for a file
func (m *BackupManager) generateBackupPath(filePath, operation string) (string, error) {
	backupDir := m.GetBackupDirectory(filePath)
	fileName := filepath.Base(filePath)

	// Create timestamp for uniqueness
	timestamp := time.Now().Format("20060102-150405.000")

	// Clean operation name for filename
	cleanOperation := strings.ReplaceAll(operation, " ", "-")
	cleanOperation = strings.ReplaceAll(cleanOperation, "/", "-")

	// Generate backup filename: originalname.backup.operation.timestamp
	backupFileName := fmt.Sprintf("%s.backup.%s.%s", fileName, cleanOperation, timestamp)

	return filepath.Join(backupDir, backupFileName), nil
}

// getBackupPattern returns a glob pattern to find all backups for a file
func (m *BackupManager) getBackupPattern(filePath string) string {
	backupDir := m.GetBackupDirectory(filePath)
	fileName := filepath.Base(filePath)

	// Pattern: originalname.backup.*
	pattern := filepath.Join(backupDir, fmt.Sprintf("%s.backup.*", fileName))
	return pattern
}

// getBackupInfo extracts backup metadata from a backup file
func (m *BackupManager) getBackupInfo(backupPath, originalFile string) (BackupInfo, error) {
	stat, err := os.Stat(backupPath)
	if err != nil {
		return BackupInfo{}, fmt.Errorf("failed to stat backup: %w", err)
	}

	// Extract operation from filename
	// Expected format: filename.backup.operation.timestamp
	// where timestamp is YYYYMMDD-HHMMSS.000 (contains a dot)
	baseName := filepath.Base(backupPath)
	parts := strings.Split(baseName, ".")

	operation := "unknown"

	// Find the "backup" part in the split filename
	backupIndex := -1
	for i, part := range parts {
		if part == "backup" {
			backupIndex = i
			break
		}
	}

	// If we found "backup" and there are parts after it
	if backupIndex >= 0 && len(parts) > backupIndex+1 {
		// The operation is everything between "backup" and the last 2 parts (timestamp.000)
		if len(parts) >= backupIndex+4 { // backup + operation + timestamp + 000
			operation = strings.Join(parts[backupIndex+1:len(parts)-2], ".")
		} else if len(parts) == backupIndex+2 {
			// Simple case: just the operation part
			operation = parts[backupIndex+1]
		}
	}

	return BackupInfo{
		Path:         backupPath,
		OriginalFile: originalFile,
		Operation:    operation,
		CreatedAt:    stat.ModTime(),
		Size:         stat.Size(),
	}, nil
}

// copyFile copies a file from source to destination
func (m *BackupManager) copyFile(src, dst string) error {
	sourceContent, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(dst, sourceContent, 0644); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}