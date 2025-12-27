package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

// TestSession_MarshalJSON tests Session JSON serialization
func TestSession_MarshalJSON(t *testing.T) {
	session := &Session{
		ID:        "debug-2025-12-27-143045-a3f2",
		AgentType: "debug",
		CreatedAt: time.Date(2025, 12, 27, 14, 30, 45, 0, time.UTC),
		UpdatedAt: time.Date(2025, 12, 27, 14, 35, 45, 0, time.UTC),
		Request:   json.RawMessage(`{"issue":"test issue"}`),
		Messages: []*schema.Message{
			{
				Role:    schema.User,
				Content: "test message",
			},
		},
		TokenUsage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		IterationCount: 5,
		MaxIterations:  50,
		Metadata: map[string]string{
			"model":    "gpt-4",
			"language": "en",
		},
	}

	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded Session
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.ID != session.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, session.ID)
	}
	if decoded.AgentType != session.AgentType {
		t.Errorf("AgentType = %v, want %v", decoded.AgentType, session.AgentType)
	}
	if decoded.IterationCount != session.IterationCount {
		t.Errorf("IterationCount = %v, want %v", decoded.IterationCount, session.IterationCount)
	}
}

// TestSession_UnmarshalJSON tests Session JSON deserialization
func TestSession_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"id": "debug-2025-12-27-143045-a3f2",
		"agent_type": "debug",
		"created_at": "2025-12-27T14:30:45Z",
		"updated_at": "2025-12-27T14:35:45Z",
		"request": {"issue":"test issue"},
		"messages": [{"role":"user","content":"test"}],
		"token_usage": {"prompt_tokens":100,"completion_tokens":50,"total_tokens":150},
		"iteration_count": 5,
		"max_iterations": 50,
		"metadata": {"model":"gpt-4"}
	}`

	var session Session
	if err := json.Unmarshal([]byte(jsonData), &session); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if session.ID != "debug-2025-12-27-143045-a3f2" {
		t.Errorf("ID = %v, want debug-2025-12-27-143045-a3f2", session.ID)
	}
	if session.AgentType != "debug" {
		t.Errorf("AgentType = %v, want debug", session.AgentType)
	}
	if session.TokenUsage.TotalTokens != 150 {
		t.Errorf("TotalTokens = %v, want 150", session.TokenUsage.TotalTokens)
	}
}

// TestSession_Validate tests Session validation
func TestSession_Validate(t *testing.T) {
	tests := []struct {
		name    string
		session *Session
		wantErr bool
	}{
		{
			name: "valid session",
			session: &Session{
				ID:        "debug-2025-12-27-143045-a3f2",
				AgentType: "debug",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Messages:  []*schema.Message{},
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			session: &Session{
				AgentType: "debug",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "empty agent type",
			session: &Session{
				ID:        "debug-2025-12-27-143045-a3f2",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid agent type",
			session: &Session{
				ID:        "invalid-2025-12-27-143045-a3f2",
				AgentType: "invalid",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "zero created time",
			session: &Session{
				ID:        "debug-2025-12-27-143045-a3f2",
				AgentType: "debug",
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGenerateSessionID tests session ID generation
func TestGenerateSessionID(t *testing.T) {
	tests := []struct {
		name      string
		agentType string
	}{
		{"debug agent", "debug"},
		{"review agent", "review"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateSessionID(tt.agentType)
			if id == "" {
				t.Error("GenerateSessionID() returned empty string")
			}
			// Check format: {type}-{timestamp}-{short-id}
			if len(id) < len(tt.agentType)+1+15+1+4 { // type + - + timestamp + - + shortid
				t.Errorf("GenerateSessionID() = %v, format seems incorrect", id)
			}
		})
	}
}

// TestSave_NewSession tests saving a new session
func TestSave_NewSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	session := &Session{
		ID:        GenerateSessionID("debug"),
		AgentType: "debug",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []*schema.Message{},
	}

	if err := mgr.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tmpDir, session.ID+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Session file not created at %s", filePath)
	}
}

// TestSave_OverwriteSession tests overwriting an existing session
func TestSave_OverwriteSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	session := &Session{
		ID:             GenerateSessionID("debug"),
		AgentType:      "debug",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Messages:       []*schema.Message{},
		IterationCount: 1,
	}

	// Save first time
	if err := mgr.Save(session); err != nil {
		t.Fatalf("Save() first time error = %v", err)
	}

	// Update and save again
	session.IterationCount = 2
	session.UpdatedAt = time.Now()
	if err := mgr.Save(session); err != nil {
		t.Fatalf("Save() second time error = %v", err)
	}

	// Load and verify
	loaded, err := mgr.Load(session.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.IterationCount != 2 {
		t.Errorf("IterationCount = %v, want 2", loaded.IterationCount)
	}
}

// TestSave_CreateDirectory tests automatic directory creation
func TestSave_CreateDirectory(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "nested", "sessions")
	mgr := NewManager(tmpDir)

	session := &Session{
		ID:        GenerateSessionID("debug"),
		AgentType: "debug",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []*schema.Message{},
	}

	if err := mgr.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Errorf("Directory not created at %s", tmpDir)
	}
}

// TestSave_InvalidSession tests saving an invalid session
func TestSave_InvalidSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	session := &Session{
		// Missing required fields
		AgentType: "debug",
	}

	err := mgr.Save(session)
	if err == nil {
		t.Error("Save() should return error for invalid session")
	}
}

// TestLoad_ExistingSession tests loading an existing session
func TestLoad_ExistingSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	original := &Session{
		ID:        GenerateSessionID("debug"),
		AgentType: "debug",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []*schema.Message{},
		Metadata: map[string]string{
			"test": "value",
		},
	}

	if err := mgr.Save(original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := mgr.Load(original.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.ID != original.ID {
		t.Errorf("ID = %v, want %v", loaded.ID, original.ID)
	}
	if loaded.AgentType != original.AgentType {
		t.Errorf("AgentType = %v, want %v", loaded.AgentType, original.AgentType)
	}
	if loaded.Metadata["test"] != "value" {
		t.Errorf("Metadata[test] = %v, want value", loaded.Metadata["test"])
	}
}

// TestLoad_NonExistentSession tests loading a non-existent session
func TestLoad_NonExistentSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	_, err := mgr.Load("nonexistent-session")
	if err == nil {
		t.Error("Load() should return error for non-existent session")
	}
}

// TestLoad_CorruptedFile tests loading a corrupted session file
func TestLoad_CorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create a corrupted file
	sessionID := "debug-2025-12-27-143045-a3f2"
	filePath := filepath.Join(tmpDir, sessionID+".json")
	if err := os.WriteFile(filePath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := mgr.Load(sessionID)
	if err == nil {
		t.Error("Load() should return error for corrupted file")
	}
}

// TestList_EmptyDirectory tests listing sessions in empty directory
func TestList_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("List() returned %d sessions, want 0", len(sessions))
	}
}

// TestList_MultipleSessions tests listing multiple sessions
func TestList_MultipleSessions(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		session := &Session{
			ID:        GenerateSessionID("debug"),
			AgentType: "debug",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now().Add(time.Duration(i) * time.Second),
			Messages:  []*schema.Message{},
		}
		if err := mgr.Save(session); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("List() returned %d sessions, want 3", len(sessions))
	}

	// Verify sorted by updated time (newest first)
	for i := 0; i < len(sessions)-1; i++ {
		if sessions[i].UpdatedAt.Before(sessions[i+1].UpdatedAt) {
			t.Error("List() sessions not sorted by updated time")
		}
	}
}

// TestList_SkipsCorruptedFiles tests that List skips corrupted files
func TestList_SkipsCorruptedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create a valid session
	session := &Session{
		ID:        GenerateSessionID("debug"),
		AgentType: "debug",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []*schema.Message{},
	}
	if err := mgr.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Create a corrupted file
	corruptedPath := filepath.Join(tmpDir, "corrupted.json")
	if err := os.WriteFile(corruptedPath, []byte("invalid"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("List() returned %d sessions, want 1 (should skip corrupted)", len(sessions))
	}
}

// TestDelete_ExistingSession tests deleting an existing session
func TestDelete_ExistingSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	session := &Session{
		ID:        GenerateSessionID("debug"),
		AgentType: "debug",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []*schema.Message{},
	}

	if err := mgr.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := mgr.Delete(session.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify file is deleted
	filePath := filepath.Join(tmpDir, session.ID+".json")
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("Delete() did not remove session file")
	}
}

// TestDelete_NonExistentSession tests deleting a non-existent session
func TestDelete_NonExistentSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	err := mgr.Delete("nonexistent-session")
	if err == nil {
		t.Error("Delete() should return error for non-existent session")
	}
}

// TestCleanupOld_NoCleanupNeeded tests cleanup when under limit
func TestCleanupOld_NoCleanupNeeded(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create 3 sessions
	for i := 0; i < 3; i++ {
		session := &Session{
			ID:        GenerateSessionID("debug"),
			AgentType: "debug",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Messages:  []*schema.Message{},
		}
		if err := mgr.Save(session); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cleanup with limit of 5 (no cleanup needed)
	if err := mgr.CleanupOld(5); err != nil {
		t.Fatalf("CleanupOld() error = %v", err)
	}

	sessions, _ := mgr.List()
	if len(sessions) != 3 {
		t.Errorf("CleanupOld() deleted sessions when it shouldn't, got %d sessions", len(sessions))
	}
}

// TestCleanupOld_RemovesOldSessions tests cleanup removes old sessions
func TestCleanupOld_RemovesOldSessions(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create 5 sessions
	for i := 0; i < 5; i++ {
		session := &Session{
			ID:        GenerateSessionID("debug"),
			AgentType: "debug",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Messages:  []*schema.Message{},
		}
		if err := mgr.Save(session); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cleanup, keep only 2 most recent
	if err := mgr.CleanupOld(2); err != nil {
		t.Fatalf("CleanupOld() error = %v", err)
	}

	sessions, _ := mgr.List()
	if len(sessions) != 2 {
		t.Errorf("CleanupOld() kept %d sessions, want 2", len(sessions))
	}
}

// TestExists tests session existence check
func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	session := &Session{
		ID:        GenerateSessionID("debug"),
		AgentType: "debug",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  []*schema.Message{},
	}

	// Should not exist before saving
	if mgr.Exists(session.ID) {
		t.Error("Exists() returned true for non-existent session")
	}

	// Save session
	if err := mgr.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Should exist after saving
	if !mgr.Exists(session.ID) {
		t.Error("Exists() returned false for existing session")
	}

	// Delete session
	if err := mgr.Delete(session.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Should not exist after deletion
	if mgr.Exists(session.ID) {
		t.Error("Exists() returned true after deletion")
	}
}

// TestSave_FileSizeLimit tests file size limit enforcement
func TestSave_FileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	// Create a session with huge message content
	hugeContent := make([]byte, 51*1024*1024) // 51MB
	for i := range hugeContent {
		hugeContent[i] = 'a'
	}

	session := &Session{
		ID:        GenerateSessionID("debug"),
		AgentType: "debug",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []*schema.Message{
			{
				Role:    schema.User,
				Content: string(hugeContent),
			},
		},
	}

	err := mgr.Save(session)
	if err == nil {
		t.Error("Save() should return error for session exceeding size limit")
	}
}
