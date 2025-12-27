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

	// **IMMEDIATELY** cancel the context to stop the agent
	fmt.Println("Stopping agent...")
	h.cancel()

	// Give a short time for the agent to stop gracefully
	time.Sleep(200 * time.Millisecond)

	// Setup a channel to listen for second interrupt signal during confirmation
	forceExit := make(chan bool, 1)
	go func() {
		select {
		case <-h.sigChan:
			// Second Ctrl+C received during confirmation
			fmt.Println("\n\n🛑  Force exit requested.")
			forceExit <- true
		case <-time.After(30 * time.Second):
			// Timeout after 30 seconds
			fmt.Println("\n⏰  Confirmation timeout. Exiting without saving.")
			forceExit <- false
		}
	}()

	// Ask user if they want to save the session with timeout
	confirmationChan := make(chan bool, 1)
	errorChan := make(chan error, 1)
	go func() {
		confirmed, err := ui.ConfirmWithDefault("Do you want to save the current session? (Ctrl+C again to force exit)", true, os.Stdin, os.Stdout)
		if err != nil {
			errorChan <- err
		} else {
			confirmationChan <- confirmed
		}
	}()

	var confirmed bool
	select {
	case forceExit := <-forceExit:
		if forceExit {
			fmt.Println("Forcing immediate exit...")
			os.Exit(130)
		} else {
			// Timeout - exit without saving
			confirmed = false
		}
	case confirmed = <-confirmationChan:
		// User provided input
	case err := <-errorChan:
		fmt.Printf("Error reading input: %v\n", err)
		confirmed = true // Default to saving on error
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
