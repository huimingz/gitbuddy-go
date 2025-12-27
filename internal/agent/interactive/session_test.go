package interactive

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInteractiveSession(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	assert.NotNil(t, session)
	assert.Equal(t, workDir, session.GetWorkingDirectory())
	assert.False(t, session.IsRunning())
}

func TestInteractiveSession_Start(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	// Mock input/output
	input := strings.NewReader("exit\n")
	output := &strings.Builder{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := session.Start(ctx, input, output)
	require.NoError(t, err)

	// Verify welcome message was displayed
	outputStr := output.String()
	assert.Contains(t, outputStr, "Interactive Debug Session Started")
	assert.Contains(t, outputStr, "ask questions and discuss")
}

func TestInteractiveSession_ProcessCommand_Help(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	output := &strings.Builder{}
	err := session.ProcessCommand(context.Background(), "help", output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Interactive Debug Session Help")
	assert.Contains(t, outputStr, "help")
	assert.Contains(t, outputStr, "modify")
	assert.Contains(t, outputStr, "exit")
}

func TestInteractiveSession_ProcessCommand_Exit(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	output := &strings.Builder{}
	err := session.ProcessCommand(context.Background(), "exit", output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Goodbye!")
	assert.False(t, session.IsRunning())
}

func TestInteractiveSession_ProcessCommand_ModifyReport(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	output := &strings.Builder{}
	err := session.ProcessCommand(context.Background(), "modify add more details about the error", output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Report modification request")
	assert.Contains(t, outputStr, "add more details about the error")
}


func TestInteractiveSession_ProcessCommand_Question(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	output := &strings.Builder{}
	err := session.ProcessCommand(context.Background(), "What caused this error?", output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Question received")
	assert.Contains(t, outputStr, "What caused this error?")
}

func TestInteractiveSession_ProcessCommand_NaturalQuestion(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	output := &strings.Builder{}
	err := session.ProcessCommand(context.Background(), "invalid_command", output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Question received")
	assert.Contains(t, outputStr, "invalid_command")
}

func TestInteractiveSession_ProcessCommand_EmptyInput(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	output := &strings.Builder{}
	err := session.ProcessCommand(context.Background(), "", output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Type 'help'")
}

func TestInteractiveSession_RunLoop(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	// Simulate user input: help command, then exit
	input := strings.NewReader("help\nexit\n")
	output := &strings.Builder{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := session.Start(ctx, input, output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Interactive Debug Session Help")
	assert.Contains(t, outputStr, "Goodbye!")
	assert.False(t, session.IsRunning())
}

func TestInteractiveSession_ContextCancellation(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	// Create input that would block (no exit command)
	input := strings.NewReader("help\n")
	output := &strings.Builder{}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := session.Start(ctx, input, output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestInteractiveSession_CommandHistory(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	// Process several commands
	commands := []string{"help", "modify test", "What caused this issue?"}

	for _, cmd := range commands {
		output := &strings.Builder{}
		err := session.ProcessCommand(context.Background(), cmd, output)
		require.NoError(t, err)
	}

	history := session.GetCommandHistory()
	assert.Len(t, history, len(commands))

	for i, cmd := range commands {
		assert.Equal(t, cmd, history[i].Command)
		assert.False(t, history[i].Timestamp.IsZero())
	}
}

func TestInteractiveSession_SetReportContent(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	reportContent := "## Debug Report\nThis is a test report"
	session.SetReportContent(reportContent)

	assert.Equal(t, reportContent, session.GetReportContent())
}

func TestInteractiveSession_MultilineInput(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	// Test multiline modify command
	input := strings.NewReader("modify\nThis is a multiline\nmodification request\n.\nexit\n")
	output := &strings.Builder{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := session.Start(ctx, input, output)
	require.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Enter your modification")
	assert.Contains(t, outputStr, "multiline modification request")
}

func TestInteractiveSession_StateManagement(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	// Initially not running
	assert.False(t, session.IsRunning())

	// Should be running during command processing
	output := &strings.Builder{}
	err := session.ProcessCommand(context.Background(), "help", output)
	require.NoError(t, err)

	// After exit command, should not be running
	err = session.ProcessCommand(context.Background(), "exit", output)
	require.NoError(t, err)
	assert.False(t, session.IsRunning())
}

func TestInteractiveSession_CommandParsing(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	tests := []struct {
		name        string
		input       string
		expectType  CommandType
		expectError bool
	}{
		{"help command", "help", CommandTypeHelp, false},
		{"exit command", "exit", CommandTypeExit, false},
		{"modify command", "modify add details", CommandTypeModify, false},
		{"question command", "What is this?", CommandTypeQuestion, false},
		{"question command with edit", "edit file.go", CommandTypeQuestion, false},
		{"empty command", "", CommandTypeEmpty, false},
		{"whitespace only", "   ", CommandTypeEmpty, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdType, args, err := session.ParseCommand(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectType, cmdType)

				if tt.expectType == CommandTypeModify {
					assert.NotEmpty(t, args)
				}
			}
		})
	}
}

func TestInteractiveSession_ErrorHandling(t *testing.T) {
	workDir := t.TempDir()
	session := NewInteractiveSession(workDir)

	// Test handling of various error conditions
	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{"valid help", "help", false},
		{"valid modify", "modify test", false},
		{"empty modify", "modify", true}, // modify without args requires scanner for multiline input
		{"natural question", "edit file.go", false}, // edit is now treated as natural question
		{"another natural question", "unknown_prefix test", false}, // unknown commands are treated as questions
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &strings.Builder{}
			err := session.ProcessCommand(context.Background(), tt.command, output)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}