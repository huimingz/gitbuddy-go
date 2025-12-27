package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/huimingz/gitbuddy-go/internal/agent/session"
	"github.com/huimingz/gitbuddy-go/internal/config"
	"github.com/huimingz/gitbuddy-go/internal/ui"
)

// SessionInterruptHandler handles graceful shutdown with session saving on interrupt signals
type SessionInterruptHandler struct {
	sessionMgr     *session.Manager
	sessionConfig  *config.SessionConfig
	currentSession *string // pointer to the current session ID
	agentType      string  // "debug" or "review"
	cancel         context.CancelFunc
	sigChan        chan os.Signal
	printer        *ui.StreamPrinter
	interrupted    bool
}

// NewSessionInterruptHandler creates a new session interrupt handler
func NewSessionInterruptHandler(
	sessionMgr *session.Manager,
	sessionConfig *config.SessionConfig,
	currentSession *string,
	agentType string,
	cancel context.CancelFunc,
	printer *ui.StreamPrinter,
) *SessionInterruptHandler {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	return &SessionInterruptHandler{
		sessionMgr:     sessionMgr,
		sessionConfig:  sessionConfig,
		currentSession: currentSession,
		agentType:      agentType,
		cancel:         cancel,
		sigChan:        sigChan,
		printer:        printer,
		interrupted:    false,
	}
}

// Start starts the interrupt handler in a goroutine
func (h *SessionInterruptHandler) Start() {
	go h.handleSignals()
}

// handleSignals handles interrupt signals
func (h *SessionInterruptHandler) handleSignals() {
	<-h.sigChan
	h.interrupted = true

	fmt.Println("\n\n⚠️  Received interrupt signal.")

	// **IMMEDIATELY** cancel the context to stop agent output
	fmt.Println("Stopping agent...")
	h.cancel()

	// Give agent more time to save current state and stop gracefully
	// The agent needs time to:
	// 1. Detect context cancellation
	// 2. Prepare session data
	// 3. Save session to disk
	time.Sleep(3 * time.Second)

	// Setup a goroutine to listen for second interrupt signal during confirmation
	go func() {
		select {
		case <-h.sigChan:
			// Second Ctrl+C received during confirmation - force exit immediately
			fmt.Println("\n\n🛑  Force exit requested.")
			os.Exit(130)
		case <-time.After(30 * time.Second):
			// Timeout after 30 seconds
			fmt.Println("\n⏰  Confirmation timeout. Exiting.")
			os.Exit(130)
		}
	}()

	// Check if session was actually saved
	sessionExists := false
	if h.currentSession != nil && *h.currentSession != "" && h.sessionMgr != nil {
		sessionExists = h.sessionMgr.Exists(*h.currentSession)
	}

	// Ask user if they want to keep the session
	var confirmed bool
	var err error
	if sessionExists {
		confirmed, err = ui.ConfirmWithDefault("Agent has been stopped and session saved. Do you want to keep the saved session? (Ctrl+C again to force exit)", true, os.Stdin, os.Stdout)
	} else {
		fmt.Println("No session was saved (session was empty or saving failed).")
		confirmed = false
		err = nil
	}

	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		// Default to keeping on error if session exists
		confirmed = sessionExists
	}

	if confirmed && sessionExists {
		fmt.Printf("✓ Session kept: %s\n", *h.currentSession)
		fmt.Printf("  Resume with: gitbuddy %s --resume %s\n", h.agentType, *h.currentSession)
	} else if sessionExists {
		fmt.Println("Session discarded.")
		// Delete the saved session file
		if err := h.sessionMgr.Delete(*h.currentSession); err != nil {
			// Don't show error to user, but we can log it if needed
		}
	}

	os.Exit(130) // Standard exit code for SIGINT
}

// IsInterrupted returns whether the handler has been interrupted
func (h *SessionInterruptHandler) IsInterrupted() bool {
	return h.interrupted
}

// Stop stops the signal handling
func (h *SessionInterruptHandler) Stop() {
	signal.Stop(h.sigChan)
	close(h.sigChan)
}
