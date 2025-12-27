package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

// Session represents a saved agent execution session
type Session struct {
	ID             string            `json:"id"`
	AgentType      string            `json:"agent_type"` // "debug" or "review"
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Request        json.RawMessage   `json:"request"`         // Original request (DebugRequest or ReviewRequest)
	Messages       []*schema.Message `json:"messages"`        // Message history
	ExecutionPlan  json.RawMessage   `json:"execution_plan"`  // Debug Agent: ExecutionPlan
	PhaseHistory   json.RawMessage   `json:"phase_history"`   // Debug Agent: PhaseHistory
	TokenUsage     TokenUsage        `json:"token_usage"`     // Token usage statistics
	IterationCount int               `json:"iteration_count"` // Current iteration count
	MaxIterations  int               `json:"max_iterations"`  // Maximum iterations
	Metadata       map[string]string `json:"metadata"`        // Additional metadata (model, language, etc.)
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// SessionInfo represents minimal session information for listing
type SessionInfo struct {
	ID            string    `json:"id"`
	AgentType     string    `json:"agent_type"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Iterations    int       `json:"iterations"`
	MaxIterations int       `json:"max_iterations"`
	TotalTokens   int       `json:"total_tokens"`
	SizeBytes     int64     `json:"size_bytes"`
}

// Validate validates the session fields
func (s *Session) Validate() error {
	if s.ID == "" {
		return errors.New("session ID is required")
	}
	if s.AgentType == "" {
		return errors.New("agent type is required")
	}
	if s.AgentType != "debug" && s.AgentType != "review" {
		return fmt.Errorf("invalid agent type: %s (must be 'debug' or 'review')", s.AgentType)
	}
	if s.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}
	return nil
}

// GenerateSessionID generates a unique session ID
// Format: {agent-type}-{timestamp}-{short-id}
// Example: debug-2025-12-27-143045-a3f2
func GenerateSessionID(agentType string) string {
	timestamp := time.Now().Format("2006-01-02-150405")
	shortID := generateShortID()
	return fmt.Sprintf("%s-%s-%s", agentType, timestamp, shortID)
}

// generateShortID generates a random 4-character hex string
func generateShortID() string {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("%04x", time.Now().UnixNano()%0x10000)
	}
	return hex.EncodeToString(b)
}

// Manager manages session persistence
type Manager struct {
	saveDir string
}

// NewManager creates a new session manager
func NewManager(saveDir string) *Manager {
	return &Manager{
		saveDir: saveDir,
	}
}

// Save saves a session to disk
func (m *Manager) Save(session *Session) error {
	if err := session.Validate(); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(m.saveDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Update timestamp
	session.UpdatedAt = time.Now()

	// Serialize to JSON
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Check file size (max 50MB)
	const maxSize = 50 * 1024 * 1024
	if len(data) > maxSize {
		return fmt.Errorf("session size (%d bytes) exceeds maximum (%d bytes)", len(data), maxSize)
	}

	// Write to file
	filePath := filepath.Join(m.saveDir, session.ID+".json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// Load loads a session from disk
func (m *Manager) Load(sessionID string) (*Session, error) {
	filePath := filepath.Join(m.saveDir, sessionID+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session file: %w", err)
	}

	return &session, nil
}

// List lists all sessions
func (m *Manager) List() ([]*SessionInfo, error) {
	// Ensure directory exists
	if err := os.MkdirAll(m.saveDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	entries, err := os.ReadDir(m.saveDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}

	var sessions []*SessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".json")
		session, err := m.Load(sessionID)
		if err != nil {
			// Skip corrupted sessions
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		sessions = append(sessions, &SessionInfo{
			ID:            session.ID,
			AgentType:     session.AgentType,
			CreatedAt:     session.CreatedAt,
			UpdatedAt:     session.UpdatedAt,
			Iterations:    session.IterationCount,
			MaxIterations: session.MaxIterations,
			TotalTokens:   session.TokenUsage.TotalTokens,
			SizeBytes:     info.Size(),
		})
	}

	// Sort by updated time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Delete deletes a session
func (m *Manager) Delete(sessionID string) error {
	filePath := filepath.Join(m.saveDir, sessionID+".json")

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session not found: %s", sessionID)
		}
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// CleanupOld removes old sessions, keeping only the most recent maxSessions
func (m *Manager) CleanupOld(maxSessions int) error {
	sessions, err := m.List()
	if err != nil {
		return err
	}

	if len(sessions) <= maxSessions {
		return nil
	}

	// Delete oldest sessions
	toDelete := sessions[maxSessions:]
	for _, session := range toDelete {
		if err := m.Delete(session.ID); err != nil {
			// Log error but continue
			continue
		}
	}

	return nil
}

// Exists checks if a session exists
func (m *Manager) Exists(sessionID string) bool {
	filePath := filepath.Join(m.saveDir, sessionID+".json")
	_, err := os.Stat(filePath)
	return err == nil
}
