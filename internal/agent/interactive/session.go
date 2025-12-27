package interactive

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/cloudwego/eino/schema"
	"github.com/huimingz/gitbuddy-go/internal/llm"
)

// CommandType represents the type of command entered by the user
type CommandType int

const (
	CommandTypeEmpty CommandType = iota
	CommandTypeHelp
	CommandTypeExit
	CommandTypeModify
	CommandTypeQuestion
)

// CommandHistory represents a command executed in the interactive session
type CommandHistory struct {
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
	Type      CommandType `json:"type"`
}

// InteractiveSession manages the post-execution interactive mode
type InteractiveSession struct {
	workingDirectory string
	isRunning        bool
	reportContent    string
	commandHistory   []CommandHistory
	llmProvider      llm.Provider // LLM provider for question answering
}

// NewInteractiveSession creates a new interactive session
func NewInteractiveSession(workingDirectory string) *InteractiveSession {
	return &InteractiveSession{
		workingDirectory: workingDirectory,
		isRunning:        false,
		commandHistory:   make([]CommandHistory, 0),
		llmProvider:      nil, // Will be set via SetLLMProvider
	}
}

// SetLLMProvider sets the LLM provider for intelligent question answering
func (s *InteractiveSession) SetLLMProvider(provider llm.Provider) {
	s.llmProvider = provider
}

// GetWorkingDirectory returns the working directory for this session
func (s *InteractiveSession) GetWorkingDirectory() string {
	return s.workingDirectory
}

// IsRunning returns whether the session is currently active
func (s *InteractiveSession) IsRunning() bool {
	return s.isRunning
}

// GetReportContent returns the current report content
func (s *InteractiveSession) GetReportContent() string {
	return s.reportContent
}

// SetReportContent sets the report content for this session
func (s *InteractiveSession) SetReportContent(content string) {
	s.reportContent = content
}

// GetCommandHistory returns the command history
func (s *InteractiveSession) GetCommandHistory() []CommandHistory {
	return s.commandHistory
}

// Start begins the interactive session loop
func (s *InteractiveSession) Start(ctx context.Context, input io.Reader, output io.Writer) error {
	s.isRunning = true
	defer func() { s.isRunning = false }()

	// Display welcome message
	s.displayWelcome(output)

	// Check if we can use readline (only for stdin/stdout)
	if input == os.Stdin && output == os.Stdout {
		return s.startWithReadline(ctx)
	}

	// Fallback to scanner for tests and non-terminal usage
	return s.startWithScanner(ctx, input, output)
}

// startWithReadline uses readline for better terminal experience (Chinese input, arrow keys, history)
func (s *InteractiveSession) startWithReadline(ctx context.Context) error {
	// Configure readline
	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "gitbuddy> ",
		HistoryFile:       ".gitbuddy_history",
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true, // Enable fuzzy search in history
	})
	if err != nil {
		return fmt.Errorf("failed to create readline: %w", err)
	}
	defer rl.Close()

	for s.isRunning {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read input with readline support
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				fmt.Println("^C")
				continue
			} else if err == io.EOF {
				fmt.Println("\nGoodbye!")
				break
			}
			return fmt.Errorf("readline error: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Process command - for multiline input, we'll create a special handler
		if err := s.ProcessCommandWithReadline(ctx, line, rl); err != nil {
			fmt.Fprintf(os.Stdout, "Error: %v\n", err)
		}

		// Add some spacing for readability
		fmt.Fprintln(os.Stdout)
	}

	return nil
}

// startWithScanner is the fallback method using bufio.Scanner for tests
func (s *InteractiveSession) startWithScanner(ctx context.Context, input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)

	for s.isRunning {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Display prompt
		fmt.Fprint(output, "gitbuddy> ")

		// Read input
		if !scanner.Scan() {
			// EOF or error
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("input scan error: %w", err)
			}
			break // EOF
		}

		line := strings.TrimSpace(scanner.Text())

		// Process command - pass scanner for potential multiline input
		if err := s.ProcessCommandWithScanner(ctx, line, output, scanner); err != nil {
			fmt.Fprintf(output, "Error: %v\n", err)
		}

		// Add some spacing for readability
		fmt.Fprintln(output)
	}

	return nil
}

// ProcessCommand processes a single command input (backward compatibility)
func (s *InteractiveSession) ProcessCommand(ctx context.Context, input string, output io.Writer) error {
	return s.ProcessCommandWithScanner(ctx, input, output, nil)
}

// ProcessCommandWithReadline processes a command using readline for better input handling
func (s *InteractiveSession) ProcessCommandWithReadline(ctx context.Context, input string, rl *readline.Instance) error {
	input = strings.TrimSpace(input)

	// Parse command
	cmdType, args, err := s.ParseCommand(input)
	if err != nil {
		return err
	}

	// Record command in history
	s.addToHistory(input, cmdType)

	// Execute command
	switch cmdType {
	case CommandTypeEmpty:
		return s.handleEmpty(os.Stdout)
	case CommandTypeHelp:
		return s.handleHelp(os.Stdout)
	case CommandTypeExit:
		return s.handleExit(os.Stdout)
	case CommandTypeModify:
		return s.handleModifyWithReadline(ctx, args, rl)
	case CommandTypeQuestion:
		return s.handleQuestion(ctx, input, os.Stdout)
	default:
		return fmt.Errorf("unknown command type: %d", cmdType)
	}
}

// ProcessCommandWithScanner processes a single command input with optional scanner for multiline input
func (s *InteractiveSession) ProcessCommandWithScanner(ctx context.Context, input string, output io.Writer, scanner *bufio.Scanner) error {
	input = strings.TrimSpace(input)

	// Parse command
	cmdType, args, err := s.ParseCommand(input)
	if err != nil {
		return err
	}

	// Record command in history
	s.addToHistory(input, cmdType)

	// Execute command
	switch cmdType {
	case CommandTypeEmpty:
		return s.handleEmpty(output)
	case CommandTypeHelp:
		return s.handleHelp(output)
	case CommandTypeExit:
		return s.handleExit(output)
	case CommandTypeModify:
		return s.handleModifyWithScanner(ctx, args, output, scanner)
	case CommandTypeQuestion:
		return s.handleQuestion(ctx, input, output)
	default:
		return fmt.Errorf("unknown command type: %d", cmdType)
	}
}

// ParseCommand parses user input into command type and arguments
func (s *InteractiveSession) ParseCommand(input string) (CommandType, string, error) {
	input = strings.TrimSpace(input)

	if input == "" {
		return CommandTypeEmpty, "", nil
	}

	// Split into command and arguments
	parts := strings.SplitN(input, " ", 2)
	command := strings.ToLower(parts[0])

	var args string
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	// Determine command type
	switch command {
	case "help", "h", "?":
		return CommandTypeHelp, args, nil
	case "exit", "quit", "q":
		return CommandTypeExit, args, nil
	case "modify", "mod", "m":
		// Allow modify without arguments for multiline input
		return CommandTypeModify, args, nil
	default:
		// Everything else is treated as a question/conversation
		return CommandTypeQuestion, input, nil
	}
}

// displayWelcome shows the welcome message
func (s *InteractiveSession) displayWelcome(output io.Writer) {
	fmt.Fprintln(output, "=== Interactive Debug Session Started ===")
	fmt.Fprintln(output, "You can now ask questions and discuss the debug results with AI!")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "What you can do:")
	fmt.Fprintln(output, "  • Ask questions: 'What caused this error?'")
	fmt.Fprintln(output, "  • Request explanations: 'Explain the memory leak issue'")
	fmt.Fprintln(output, "  • Get suggestions: 'How should I fix this?'")
	fmt.Fprintln(output, "  • Modify report: 'modify add more details about performance'")
	fmt.Fprintln(output, "  • Get help: 'help'")
	fmt.Fprintln(output, "  • Exit: 'exit'")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Just type your questions or requests naturally!")
	fmt.Fprintln(output)
}

// addToHistory adds a command to the history
func (s *InteractiveSession) addToHistory(command string, cmdType CommandType) {
	entry := CommandHistory{
		Command:   command,
		Timestamp: time.Now(),
		Type:      cmdType,
	}
	s.commandHistory = append(s.commandHistory, entry)
}

// handleEmpty handles empty input
func (s *InteractiveSession) handleEmpty(output io.Writer) error {
	fmt.Fprintln(output, "Type 'help' for available commands.")
	return nil
}

// handleHelp handles help commands
func (s *InteractiveSession) handleHelp(output io.Writer) error {
	fmt.Fprintln(output, "=== Interactive Debug Session Help ===")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "You can interact with the AI agent naturally. Here are some examples:")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Questions about the analysis:")
	fmt.Fprintln(output, "  • What caused this error?")
	fmt.Fprintln(output, "  • Why did you suggest this solution?")
	fmt.Fprintln(output, "  • Is there a performance issue?")
	fmt.Fprintln(output, "  • How critical is this bug?")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Requests for explanations:")
	fmt.Fprintln(output, "  • Explain the memory leak in detail")
	fmt.Fprintln(output, "  • Walk me through the error flow")
	fmt.Fprintln(output, "  • Show me the root cause analysis")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Modification requests:")
	fmt.Fprintln(output, "  • modify add performance recommendations")
	fmt.Fprintln(output, "  • modify include code examples for fixes")
	fmt.Fprintln(output, "  • modify focus more on security implications")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Commands:")
	fmt.Fprintln(output, "  • help - Show this help")
	fmt.Fprintln(output, "  • exit - Leave interactive mode")
	fmt.Fprintln(output)
	return nil
}

// handleExit handles exit commands
func (s *InteractiveSession) handleExit(output io.Writer) error {
	fmt.Fprintln(output, "Goodbye! Thank you for using GitBuddy interactive mode.")
	s.isRunning = false
	return nil
}

// handleModify handles report modification requests (backward compatibility)
func (s *InteractiveSession) handleModify(ctx context.Context, args string, output io.Writer) error {
	return s.handleModifyWithScanner(ctx, args, output, nil)
}

// handleModifyWithScanner handles report modification requests with optional multiline input
func (s *InteractiveSession) handleModifyWithScanner(ctx context.Context, args string, output io.Writer, scanner *bufio.Scanner) error {
	var modificationRequest string

	if args != "" {
		// Single line modification request
		modificationRequest = args
	} else if scanner != nil {
		// Multiline modification request
		fmt.Fprintln(output, "Enter your modification request (end with a line containing only '.'):")
		var lines []string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "." {
				break
			}
			lines = append(lines, line)
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading multiline input: %w", err)
		}
		modificationRequest = strings.Join(lines, "\n")
	} else {
		return fmt.Errorf("modify command requires arguments or multiline input")
	}

	// Display the request, replacing newlines with spaces for cleaner output
	displayRequest := strings.ReplaceAll(modificationRequest, "\n", " ")
	fmt.Fprintf(output, "Report modification request received: %s\n", displayRequest)

	// Check if LLM provider is available
	if s.llmProvider == nil {
		fmt.Fprintln(output, "Processing modification request...")
		fmt.Fprintln(output, "Note: LLM provider not configured. Unable to modify report with AI.")
		fmt.Fprintln(output, "Please configure an LLM provider to enable AI-powered report modifications.")
		return nil
	}

	fmt.Fprintln(output, "Processing modification request with AI...")
	fmt.Fprintln(output)

	// Create chat model
	chatModel, err := s.llmProvider.CreateChatModel(ctx)
	if err != nil {
		fmt.Fprintf(output, "Error: Failed to create chat model: %v\n", err)
		return nil
	}

	// Build system prompt
	systemPrompt := `You are a technical report modification assistant.
Your task is to improve and modify existing debug reports based on user requests.
Keep the technical accuracy and formatting of the original report.
Make the requested modifications while maintaining clarity and professionalism.
Return the complete modified report, not just the changes.`

	// Get current report content
	currentReport := s.GetReportContent()
	if currentReport == "" {
		fmt.Fprintln(output, "No existing report to modify.")
		return nil
	}

	// Build user message with context
	userMessage := fmt.Sprintf(`Current Debug Report:
%s

Modification Request: %s

Please provide the complete modified report incorporating the requested changes.`, currentReport, modificationRequest)

	// Create messages
	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userMessage},
	}

	// Stream response
	streamReader, err := chatModel.Stream(ctx, messages)
	if err != nil {
		fmt.Fprintf(output, "Error: %v\n", err)
		return nil
	}

	var modifiedReport strings.Builder
	for {
		chunk, err := streamReader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			streamReader.Close()
			fmt.Fprintf(output, "Error reading response: %v\n", err)
			return nil
		}

		if chunk.Content != "" {
			modifiedReport.WriteString(chunk.Content)
			fmt.Fprint(output, chunk.Content)
		}
	}
	streamReader.Close()

	// Update the stored report content with the modified version
	s.SetReportContent(modifiedReport.String())

	fmt.Fprintln(output)
	fmt.Fprintln(output, "✓ Report has been successfully modified and updated.")

	return nil
}

// handleModifyWithReadline handles report modification requests using readline for multiline input
func (s *InteractiveSession) handleModifyWithReadline(ctx context.Context, args string, rl *readline.Instance) error {
	var modificationRequest string

	if args != "" {
		// Single line modification request
		modificationRequest = args
	} else {
		// Multiline modification request using readline
		fmt.Println("Enter your modification request (end with a line containing only '.' and press Enter):")

		// Temporarily change the prompt
		rl.SetPrompt("  > ")
		defer rl.SetPrompt("gitbuddy> ")

		var lines []string
		for {
			line, err := rl.Readline()
			if err != nil {
				if err == readline.ErrInterrupt {
					fmt.Println("\nModification cancelled.")
					return nil
				} else if err == io.EOF {
					break
				}
				return fmt.Errorf("error reading multiline input: %w", err)
			}

			if strings.TrimSpace(line) == "." {
				break
			}
			lines = append(lines, line)
		}
		modificationRequest = strings.Join(lines, "\n")
	}

	// Display the request, replacing newlines with spaces for cleaner output
	displayRequest := strings.ReplaceAll(modificationRequest, "\n", " ")
	fmt.Printf("Report modification request received: %s\n", displayRequest)

	// Check if LLM provider is available
	if s.llmProvider == nil {
		fmt.Println("Processing modification request...")
		fmt.Println("Note: LLM provider not configured. Unable to modify report with AI.")
		fmt.Println("Please configure an LLM provider to enable AI-powered report modifications.")
		return nil
	}

	fmt.Println("Processing modification request with AI...")
	fmt.Println()

	// Create chat model
	chatModel, err := s.llmProvider.CreateChatModel(ctx)
	if err != nil {
		fmt.Printf("Error: Failed to create chat model: %v\n", err)
		return nil
	}

	// Build system prompt
	systemPrompt := `You are a technical report modification assistant.
Your task is to improve and modify existing debug reports based on user requests.
Keep the technical accuracy and formatting of the original report.
Make the requested modifications while maintaining clarity and professionalism.
Return the complete modified report, not just the changes.`

	// Get current report content
	currentReport := s.GetReportContent()
	if currentReport == "" {
		fmt.Println("No existing report to modify.")
		return nil
	}

	// Build user message with context
	userMessage := fmt.Sprintf(`Current Debug Report:
%s

Modification Request: %s

Please provide the complete modified report incorporating the requested changes.`, currentReport, modificationRequest)

	// Create messages
	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userMessage},
	}

	// Stream response
	streamReader, err := chatModel.Stream(ctx, messages)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}

	var modifiedReport strings.Builder
	for {
		chunk, err := streamReader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			streamReader.Close()
			fmt.Printf("Error reading response: %v\n", err)
			return nil
		}

		if chunk.Content != "" {
			modifiedReport.WriteString(chunk.Content)
			fmt.Print(chunk.Content)
		}
	}
	streamReader.Close()

	// Update the stored report content with the modified version
	s.SetReportContent(modifiedReport.String())

	fmt.Println()
	fmt.Println("✓ Report has been successfully modified and updated.")

	return nil
}

// handleQuestion handles user questions using LLM for intelligent responses
func (s *InteractiveSession) handleQuestion(ctx context.Context, question string, output io.Writer) error {
	fmt.Fprintf(output, "Question received: %s\n", question)

	// Check if LLM provider is available
	if s.llmProvider == nil {
		fmt.Fprintln(output, "Processing question...")
		fmt.Fprintln(output, "Note: LLM provider not configured. Unable to provide intelligent responses.")
		fmt.Fprintln(output, "Please configure an LLM provider to enable AI-powered question answering.")
		return nil
	}

	fmt.Fprintln(output, "Processing question with AI...")
	fmt.Fprintln(output)

	// Create chat model
	chatModel, err := s.llmProvider.CreateChatModel(ctx)
	if err != nil {
		fmt.Fprintf(output, "Error: Failed to create chat model: %v\n", err)
		return nil
	}

	// Build system prompt
	systemPrompt := `You are a helpful assistant answering questions about code analysis and debugging.
You have access to a detailed debug report that was previously generated.
Answer questions clearly and concisely, referring to the report when relevant.
Focus on being practical and actionable.
If the question asks for code changes or modifications, explain what should be done rather than doing it yourself.`

	// Get report context
	reportContext := s.GetReportContent()
	if reportContext == "" {
		reportContext = "(No debug report available)"
	}

	// Build user message with context
	userMessage := fmt.Sprintf(`Debug Report Context:
%s

Question: %s`, reportContext, question)

	// Create messages
	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userMessage},
	}

	// Stream response
	streamReader, err := chatModel.Stream(ctx, messages)
	if err != nil {
		fmt.Fprintf(output, "Error: %v\n", err)
		return nil
	}

	var fullResponse strings.Builder
	for {
		chunk, err := streamReader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			streamReader.Close()
			fmt.Fprintf(output, "Error reading response: %v\n", err)
			return nil
		}

		if chunk.Content != "" {
			fullResponse.WriteString(chunk.Content)
			fmt.Fprint(output, chunk.Content)
		}
	}
	streamReader.Close()
	fmt.Fprintln(output)

	return nil
}