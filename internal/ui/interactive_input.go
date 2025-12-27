package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
)

var (
	// ErrEmptyInput is returned when the user provides no input
	ErrEmptyInput = errors.New("empty input")

	// ErrInterrupted is returned when the user interrupts input with Ctrl+C
	ErrInterrupted = errors.New("input interrupted")

	// ErrTimeout is returned when input times out
	ErrTimeout = errors.New("input timeout")
)

// MultilinePrompt represents a multi-line input prompt with hints and examples
type MultilinePrompt struct {
	Prompt   string   // The main prompt message
	Hint     string   // Hint text shown to help users
	Examples []string // Example inputs to show users
}

// Show displays the prompt and collects multi-line input from the user
// Input is terminated by Ctrl+D (EOF) or Ctrl+C (interrupt)
func (p *MultilinePrompt) Show(input io.Reader, output io.Writer) (string, error) {
	return p.ShowWithContext(context.Background(), input, output)
}

// ShowWithContext displays the prompt with context support for cancellation and timeout
func (p *MultilinePrompt) ShowWithContext(ctx context.Context, input io.Reader, output io.Writer) (string, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		if ctx.Err() == context.Canceled {
			return "", ErrInterrupted
		}
		return "", ErrTimeout
	default:
	}

	// Display the prompt
	if err := p.displayPrompt(output); err != nil {
		return "", err
	}

	// Use readline for better terminal experience if using stdin/stdout
	if input == os.Stdin && output == os.Stdout {
		return p.readWithReadline(ctx)
	}

	// Set up signal handling for Ctrl+C if using stdin
	if input == os.Stdin {
		return p.readWithSignalHandling(ctx, input, output)
	}

	return p.readInput(ctx, input)
}

// displayPrompt shows the prompt, hint, and examples
func (p *MultilinePrompt) displayPrompt(output io.Writer) error {
	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgHiBlack)
	green := color.New(color.FgGreen)

	// Show main prompt
	_, err := bold.Fprintln(output, fmt.Sprintf("\n🤔 %s", p.Prompt))
	if err != nil {
		return err
	}

	// Show hint if provided
	if p.Hint != "" {
		_, err = dim.Fprintln(output, fmt.Sprintf("   %s", p.Hint))
		if err != nil {
			return err
		}
	}

	// Show examples if provided
	if len(p.Examples) > 0 {
		_, err = cyan.Fprintln(output, "\n   Examples:")
		if err != nil {
			return err
		}

		for _, example := range p.Examples {
			_, err = green.Fprintln(output, fmt.Sprintf("   • %s", example))
			if err != nil {
				return err
			}
		}
	}

	// Show input cursor
	_, err = fmt.Fprint(output, "\n> ")
	return err
}

// readWithSignalHandling reads input with support for Ctrl+C interruption
func (p *MultilinePrompt) readWithSignalHandling(ctx context.Context, input io.Reader, output io.Writer) (string, error) {
	// Set up signal channel for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Channel for input result
	resultChan := make(chan inputResult, 1)

	// Start reading in a goroutine
	go func() {
		result, err := p.readInput(context.Background(), input)
		resultChan <- inputResult{result: result, err: err}
	}()

	// Wait for either input, signal, or context cancellation
	select {
	case <-ctx.Done():
		return "", ErrTimeout
	case <-sigChan:
		// User pressed Ctrl+C
		_, _ = fmt.Fprintln(output, "\n\nInput cancelled.")
		return "", ErrInterrupted
	case result := <-resultChan:
		return result.result, result.err
	}
}

// inputResult holds the result from reading input
type inputResult struct {
	result string
	err    error
}

// readInput reads multi-line input until EOF (Ctrl+D)
func (p *MultilinePrompt) readInput(ctx context.Context, input io.Reader) (string, error) {
	scanner := bufio.NewScanner(input)
	var lines []string
	var ctrlDDetected bool

	for {
		// Quick check for context cancellation
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return "", ErrInterrupted
			}
			return "", ErrTimeout
		default:
		}

		// Read next line
		if !scanner.Scan() {
			// Check if we got EOF or an error
			if err := scanner.Err(); err != nil {
				return "", err
			}
			// EOF reached
			break
		}

		line := scanner.Text()

		// Check for Ctrl+D character in the line (some terminals include it)
		if strings.Contains(line, "\x04") {
			ctrlDDetected = true
			// Remove Ctrl+D and add the part before it (if any)
			parts := strings.Split(line, "\x04")
			if parts[0] != "" {
				lines = append(lines, parts[0])
			}
			break
		}

		lines = append(lines, line)
	}

	// Join all lines
	result := strings.Join(lines, "\n")

	// Check if input is empty (no content)
	if strings.TrimSpace(result) == "" {
		// If Ctrl+D was detected or we had some interaction, it's empty input
		if ctrlDDetected || len(lines) > 0 {
			return "", ErrEmptyInput
		}
		// If no lines were read and no Ctrl+D detected, it's immediate EOF
		return "", io.EOF
	}

	return result, nil
}

// readWithReadline uses readline for better terminal input experience (Chinese, arrows, history)
func (p *MultilinePrompt) readWithReadline(ctx context.Context) (string, error) {
	// Configure readline for multiline input
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		InterruptPrompt: "^C",
		EOFPrompt:       "^D",
	})
	if err != nil {
		// Fallback to regular input if readline fails
		return p.readInput(ctx, os.Stdin)
	}
	defer rl.Close()

	var lines []string

	// Read lines until Ctrl+D (EOF)
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return "", ErrInterrupted
			}
			return "", ErrTimeout
		default:
		}

		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				fmt.Println("\nInput cancelled.")
				return "", ErrInterrupted
			} else if err == io.EOF {
				// EOF (Ctrl+D) - finish input
				break
			}
			return "", err
		}

		// Check for Ctrl+D character in the line
		if strings.Contains(line, "\x04") {
			// Remove Ctrl+D and add the part before it (if any)
			parts := strings.Split(line, "\x04")
			if parts[0] != "" {
				lines = append(lines, parts[0])
			}
			break
		}

		lines = append(lines, line)
	}

	// Join all lines
	result := strings.Join(lines, "\n")

	// Check if input is empty
	if strings.TrimSpace(result) == "" {
		if len(lines) > 0 {
			return "", ErrEmptyInput
		}
		return "", io.EOF
	}

	return result, nil
}