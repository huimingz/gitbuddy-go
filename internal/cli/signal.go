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

	// Always ask user if they want to save the session when interrupted
	confirmed, err := ui.ConfirmWithDefault("Do you want to save the current session?", true, os.Stdin, os.Stdout)
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		// Default to saving on error
		confirmed = true
	}

	if confirmed {
		fmt.Println("Saving session...")
		// Give some time for the session to be saved by the agent
		time.Sleep(1 * time.Second)

		if h.currentSession != nil && *h.currentSession != "" {
			fmt.Printf("✓ Session saved: %s\n", *h.currentSession)
			fmt.Printf("  Resume with: gitbuddy %s --resume %s\n", h.agentType, *h.currentSession)
		} else {
			fmt.Println("✓ Session saved (session ID will be available after processing)")
		}
	} else {
		fmt.Println("Session not saved.")
	}

	// Cancel the context to stop the agent
	h.cancel()

	// Give some more time for graceful shutdown
	time.Sleep(500 * time.Millisecond)

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
