package interactive

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/c-bata/go-prompt"
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
	ctx              context.Context // Context for cancellation
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
	s.ctx = ctx
	defer func() { s.isRunning = false }()

	// Display welcome message
	s.displayWelcome(output)

	// Check if we can use go-prompt (only for stdin/stdout in terminal)
	if input == os.Stdin && output == os.Stdout {
		// Try go-prompt first, but provide Bubbletea as fallback for better Unicode support
		// Users can set GITBUDDY_USE_BUBBLETEA=1 to force Bubbletea usage
		if os.Getenv("GITBUDDY_USE_BUBBLETEA") == "1" {
			return s.startWithBubbletea(ctx)
		}
		return s.startWithGoPrompt(ctx)
	}

	// Fallback to scanner for tests and non-terminal usage
	return s.startWithScanner(ctx, input, output)
}

// startWithGoPrompt uses go-prompt for excellent terminal experience with proper Chinese character handling
func (s *InteractiveSession) startWithGoPrompt(ctx context.Context) error {
	// Create prompt with auto-completion and history, with enhanced Unicode/Chinese character support
	p := prompt.New(
		s.executor,     // Function to handle user input
		s.completer,    // Function for auto-completion
		prompt.OptionTitle("GitBuddy Interactive Debug Session"),
		prompt.OptionPrefix("gitbuddy> "),
		prompt.OptionHistory(s.getHistoryStrings()), // Load command history
		prompt.OptionLivePrefix(s.livePrefix),       // Dynamic prefix
		prompt.OptionInputTextColor(prompt.DefaultColor),
		prompt.OptionPrefixTextColor(prompt.Blue),
		prompt.OptionPreviewSuggestionTextColor(prompt.Green),
		prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
		prompt.OptionMaxSuggestion(10),
		// Enhanced options for better Unicode/Chinese character handling
		prompt.OptionShowCompletionAtStart(),     // Show completion immediately
		prompt.OptionCompletionOnDown(),          // Better completion navigation
		// Try different key binding modes - some handle Unicode better
		prompt.OptionSwitchKeyBindMode(prompt.CommonKeyBind), // CommonKeyBind might handle wide chars better than Emacs
	)

	// Start the prompt - this blocks until exit
	p.Run()

	return nil
}

// executor handles user input from go-prompt
func (s *InteractiveSession) executor(input string) {
	input = strings.TrimSpace(input)

	// Check for exit commands first
	if input == "exit" || input == "quit" || input == "q" {
		s.isRunning = false
		fmt.Println("Goodbye! Thank you for using GitBuddy interactive mode.")
		return
	}

	// Handle empty input
	if input == "" {
		return
	}

	// Process the command
	if err := s.processCommandFromPrompt(input); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	fmt.Println() // Add spacing for readability
}

// completer provides auto-completion suggestions
func (s *InteractiveSession) completer(d prompt.Document) []prompt.Suggest {
	suggestions := []prompt.Suggest{
		{Text: "help", Description: "Show help information"},
		{Text: "exit", Description: "Exit interactive mode"},
		{Text: "quit", Description: "Exit interactive mode"},
		{Text: "modify", Description: "Modify the debug report"},
		{Text: "What caused this error?", Description: "Ask about the error cause"},
		{Text: "How can I fix this?", Description: "Ask for solution suggestions"},
		{Text: "Explain the issue in detail", Description: "Request detailed explanation"},
		{Text: "Is this a performance issue?", Description: "Ask about performance"},
		{Text: "How critical is this bug?", Description: "Ask about severity"},
		{Text: "modify add performance analysis", Description: "Add performance analysis to report"},
		{Text: "modify include code examples", Description: "Add code examples to report"},
		{Text: "modify focus on security", Description: "Focus on security aspects"},
	}

	// Return filtered suggestions based on user input
	return prompt.FilterHasPrefix(suggestions, d.GetWordBeforeCursor(), true)
}

// getHistoryStrings converts command history to string slice for go-prompt
func (s *InteractiveSession) getHistoryStrings() []string {
	var history []string
	for _, cmd := range s.commandHistory {
		if cmd.Command != "" {
			history = append(history, cmd.Command)
		}
	}
	return history
}

// livePrefix provides dynamic prefix based on session state
func (s *InteractiveSession) livePrefix() (string, bool) {
	if !s.isRunning {
		return "gitbuddy> ", false
	}

	// Could add context-aware prefixes here
	// For example, different colors based on LLM provider status
	if s.llmProvider != nil {
		return "gitbuddy🤖> ", true // AI mode indicator
	}

	return "gitbuddy> ", true
}

// processCommandFromPrompt processes commands from go-prompt
func (s *InteractiveSession) processCommandFromPrompt(input string) error {
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
		s.isRunning = false
		fmt.Println("Goodbye! Thank you for using GitBuddy interactive mode.")
		return nil
	case CommandTypeModify:
		return s.handleModifyFromPrompt(args)
	case CommandTypeQuestion:
		return s.handleQuestion(s.ctx, input, os.Stdout)
	default:
		return fmt.Errorf("unknown command type: %d", cmdType)
	}
}

// handleModifyFromPrompt handles modify commands from go-prompt
func (s *InteractiveSession) handleModifyFromPrompt(args string) error {
	var modificationRequest string

	if args != "" {
		// Single line modification request
		modificationRequest = args
	} else {
		// For multiline input, we'll use a simple input prompt
		fmt.Println("Enter your modification request (press Enter when finished):")
		fmt.Print("  > ")

		// Read a single line for now - could be enhanced for true multiline
		var input string
		fmt.Scanln(&input)
		modificationRequest = input
	}

	// Process the modification request (reuse existing logic)
	return s.handleModifyRequest(s.ctx, modificationRequest, os.Stdout)
}

// handleModifyRequest handles the core modification logic (shared by different input methods)
func (s *InteractiveSession) handleModifyRequest(ctx context.Context, modificationRequest string, output io.Writer) error {
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

	// Use the shared modification logic
	return s.handleModifyRequest(ctx, modificationRequest, output)
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