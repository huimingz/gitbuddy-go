package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/huimingz/gitbuddy-go/internal/agent"
	"github.com/huimingz/gitbuddy-go/internal/agent/session"
	"github.com/huimingz/gitbuddy-go/internal/config"
	"github.com/huimingz/gitbuddy-go/internal/git"
	"github.com/huimingz/gitbuddy-go/internal/llm"
	"github.com/huimingz/gitbuddy-go/internal/ui"
	"github.com/spf13/cobra"
)

var (
	chatLanguage      string
	chatModel         string
	chatMaxIterations int
	chatResume        string
)

var chatCmd = &cobra.Command{
	Use:   "chat [query]",
	Short: "Interactive chat with AI assistant",
	Long: `Chat with GitBuddy AI assistant in interactive or single-query mode.

In interactive mode (no query provided):
  gitbuddy chat
  > Your question here
  > Another question

In single-query mode:
  gitbuddy chat "Your question here"

The assistant has access to:
- File system tools (read, write, search files)
- Git tools (status, log, diff, etc.)
- And more...

Examples:
  gitbuddy chat                              # Start interactive mode
  gitbuddy chat "What's in main.go?"        # Single query
  gitbuddy chat --resume <session-id>       # Resume a session
  gitbuddy chat --language zh               # Use Chinese
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleChat(cmd.Context(), args)
	},
}

func init() {
	chatCmd.Flags().StringVar(&chatLanguage, "language", "en", "Output language (en, zh)")
	chatCmd.Flags().StringVar(&chatModel, "model", "", "LLM model to use (optional)")
	chatCmd.Flags().IntVar(&chatMaxIterations, "max-iterations", 10, "Maximum agent iterations")
	chatCmd.Flags().StringVar(&chatResume, "resume", "", "Resume a previous session by ID")
	rootCmd.AddCommand(chatCmd)
}

func handleChat(ctx context.Context, args []string) error {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create UI printer
	printer := ui.NewStreamPrinter(os.Stdout)

	// Create session manager
	sessionsDir := "./sessions"
	sessionManager := session.NewManager(sessionsDir)

	// Get or resume session
	var sess *session.Session
	var sessionID string
	if chatResume != "" {
		var err error
		sess, err = sessionManager.Load(chatResume)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}
		sessionID = chatResume
	} else {
		sessionID = session.GenerateSessionID("chat")
	}

	// Create Git executor
	gitExec := git.NewExecutor(workDir)

	// Get the model configuration
	var modelCfg config.ModelConfig
	if chatModel != "" {
		// Use specified model
		modelCfgPtr, err := cfg.GetModel(chatModel)
		if err != nil {
			return fmt.Errorf("model not found: %w", err)
		}
		modelCfg = *modelCfgPtr
	} else {
		// Use default model
		if cfg.DefaultModel == "" {
			return fmt.Errorf("no default model configured")
		}
		modelCfgPtr, err := cfg.GetModel(cfg.DefaultModel)
		if err != nil {
			return fmt.Errorf("failed to get default model: %w", err)
		}
		modelCfg = *modelCfgPtr
	}

	// Create LLM provider
	factory := llm.NewProviderFactory()
	provider, err := factory.Create(modelCfg)
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Get retry configuration
	var retryConfig llm.RetryConfig
	if cfg.Retry != nil {
		retryConfig = llm.RetryConfig{
			Enabled:     cfg.Retry.Enabled,
			MaxAttempts: cfg.Retry.MaxAttempts,
			BackoffBase: cfg.Retry.BackoffBase,
			BackoffMax:  cfg.Retry.BackoffMax,
		}
	} else {
		retryConfig = llm.DefaultRetryConfig()
	}

	// Create ChatAgent
	chatAgent := agent.NewChatAgent(agent.ChatAgentOptions{
		Language:        chatLanguage,
		GitExecutor:     gitExec,
		LLMProvider:     provider,
		Printer:         printer,
		Output:          os.Stdout,
		Input:           os.Stdin,
		WorkDir:         workDir,
		MaxLinesPerRead: 1000,
		RetryConfig:     retryConfig,
		SessionManager:  sessionManager,
	})

	// Print welcome message
	fmt.Println(agent.GetChatWelcomeMessage(chatLanguage))
	fmt.Println()

	// Determine if we're in interactive or single-query mode
	if len(args) > 0 {
		// Single query mode
		query := strings.Join(args, " ")
		return handleSingleQuery(ctx, chatAgent, query, sessionID, sess)
	}

	// Interactive mode
	return handleInteractiveChat(ctx, chatAgent, sessionID, sess)
}

func handleSingleQuery(ctx context.Context, chatAgent *agent.ChatAgent, query string, sessionID string, sess *session.Session) error {
	req := &agent.ChatRequest{
		Query:                 query,
		Language:              chatLanguage,
		WorkDir:               "",
		MaxIterations:         chatMaxIterations,
		EnableCompression:     false,
		CompressionThreshold:  20,
		CompressionKeepRecent: 10,
		Session:               sess,
		PreGeneratedSessionID: sessionID,
		OnStreamChunk: func(chunk string) {
			fmt.Print(chunk)
		},
	}

	// Create a cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Setup signal handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	fmt.Println()
	fmt.Println("🤖 Assistant:")

	_, err := chatAgent.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("chat failed: %w", err)
	}

	fmt.Println()
	fmt.Println()

	return nil
}

func handleInteractiveChat(ctx context.Context, chatAgent *agent.ChatAgent, sessionID string, sess *session.Session) error {
	// Setup signal handler for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")

	for scanner.Scan() {
		select {
		case <-sigChan:
			fmt.Println()
			fmt.Println(agent.GetChatExitMessage(chatLanguage))
			return nil
		default:
		}

		input := strings.TrimSpace(scanner.Text())

		// Handle special commands
		if input == "" {
			fmt.Print("> ")
			continue
		}

		if strings.ToLower(input) == "exit" || strings.ToLower(input) == "quit" {
			fmt.Println(agent.GetChatExitMessage(chatLanguage))
			return nil
		}

		if strings.ToLower(input) == "help" {
			printChatHelp(chatLanguage)
			fmt.Print("> ")
			continue
		}

		// Create a cancellable context for this query
		queryCtx, cancel := context.WithCancel(ctx)

		// Run chat with timeout
		req := &agent.ChatRequest{
			Query:                 input,
			Language:              chatLanguage,
			WorkDir:               "",
			MaxIterations:         chatMaxIterations,
			EnableCompression:     true,
			CompressionThreshold:  20,
			CompressionKeepRecent: 10,
			Session:               sess,
			PreGeneratedSessionID: sessionID,
			OnStreamChunk: func(chunk string) {
				fmt.Print(chunk)
			},
		}

		// Create a goroutine to handle signal during chat
		done := make(chan error, 1)
		go func() {
			fmt.Println()
			fmt.Println("🤖 Assistant:")
			_, err := chatAgent.Chat(queryCtx, req)
			if err != nil {
				done <- err
				return
			}
			fmt.Println()
			fmt.Println()
			done <- nil
		}()

		// Wait for either completion or signal
		select {
		case err := <-done:
			cancel()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		case <-sigChan:
			cancel()
			fmt.Println()
			fmt.Println(agent.GetChatExitMessage(chatLanguage))
			return nil
		}

		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func printChatHelp(language string) {
	if language == "zh" || language == "zh-cn" || language == "chinese" {
		fmt.Print(`
帮助命令:

特殊命令:
  help  - 显示此帮助信息
  exit  - 退出聊天
  quit  - 退出聊天

提示:
- 在任何时刻按 Ctrl+C 退出
- 输入你的问题,AI 会尽力回答
- 可以进行多轮对话
`)
	} else {
		fmt.Print(`
Help:

Special commands:
  help  - Show this help message
  exit  - Exit chat
  quit  - Exit chat

Tips:
- Press Ctrl+C at any time to exit
- Type your question and I'll help
- You can have multi-turn conversations
`)
	}
}
