package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"agentsdk/ai"
	"agentsdk/coding"

	// Register providers
	_ "agentsdk/ai/providers/anthropic"
	_ "agentsdk/ai/providers/openai"
)

const version = "0.1.0"

func main() {
	// Flags
	modelFlag := flag.String("model", "", "Model ID (e.g., claude-sonnet-4-5-20250929, gpt-4o)")
	providerFlag := flag.String("provider", "", "Provider (anthropic, openai, google)")
	thinkingFlag := flag.String("thinking", "off", "Thinking level (off, minimal, low, medium, high, xhigh)")
	printFlag := flag.String("p", "", "Run in print mode with the given prompt (non-interactive)")
	printLong := flag.String("print", "", "Run in print mode with the given prompt (non-interactive)")
	apiKeyFlag := flag.String("api-key", "", "API key (overrides environment variable)")
	cwdFlag := flag.String("cwd", "", "Working directory")
	readOnlyFlag := flag.Bool("read-only", false, "Use read-only tools (no write/edit/bash)")
	maxTurnsFlag := flag.Int("max-turns", 0, "Maximum number of LLM round-trips (0 = unlimited)")
	appendPromptFlag := flag.String("append-prompt", "", "Text to append to system prompt")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	helpFlag := flag.Bool("help", false, "Print help and exit")
	listModelsFlag := flag.Bool("list-models", false, "List available models and exit")

	flag.Parse()

	if *helpFlag {
		printHelp()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Println("agentsdk version", version)
		os.Exit(0)
	}

	if *listModelsFlag {
		printModels()
		os.Exit(0)
	}

	// Resolve model
	model, err := resolveModel(*providerFlag, *modelFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Parse thinking level
	thinking := ai.ThinkingLevel(*thinkingFlag)

	// Resolve prompt for print mode
	printPrompt := *printFlag
	if printPrompt == "" {
		printPrompt = *printLong
	}
	// Also accept prompt from remaining args
	if printPrompt == "" && flag.NArg() > 0 {
		printPrompt = strings.Join(flag.Args(), " ")
	}

	// Build config
	cfg := coding.InteractiveConfig{
		Model:         model,
		ThinkingLevel: thinking,
		Cwd:           *cwdFlag,
		APIKey:        *apiKeyFlag,
		AppendPrompt:  *appendPromptFlag,
		ReadOnly:      *readOnlyFlag,
		MaxTurns:      *maxTurnsFlag,
	}

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Print mode or interactive mode
	if printPrompt != "" {
		if err := coding.RunPrintMode(ctx, cfg, printPrompt); err != nil {
			if ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
		return
	}

	// Interactive mode
	im, err := coding.NewInteractiveMode(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := im.Run(ctx); err != nil {
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// resolveModel determines which model to use based on flags and environment.
func resolveModel(providerStr, modelID string) (*ai.Model, error) {
	// If both provider and model are specified, look it up directly
	if providerStr != "" && modelID != "" {
		return ai.GetModel(ai.Provider(providerStr), modelID)
	}

	// If only model is specified, search all providers
	if modelID != "" {
		for _, provider := range ai.ListProviders() {
			if m, err := ai.GetModel(provider, modelID); err == nil {
				return m, nil
			}
		}
		return nil, fmt.Errorf("model %q not found in any provider", modelID)
	}

	// If only provider is specified, use a default model
	if providerStr != "" {
		defaults := map[ai.Provider]string{
			ai.ProviderAnthropic: "claude-sonnet-4-5-20250929",
			ai.ProviderOpenAI:    "gpt-4o",
			ai.ProviderGoogle:    "gemini-2.5-flash",
		}
		provider := ai.Provider(providerStr)
		if defaultID, ok := defaults[provider]; ok {
			return ai.GetModel(provider, defaultID)
		}
		return nil, fmt.Errorf("no default model for provider %q", providerStr)
	}

	// Auto-detect from environment
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return ai.GetModel(ai.ProviderAnthropic, "claude-sonnet-4-5-20250929")
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return ai.GetModel(ai.ProviderOpenAI, "gpt-4o")
	}
	if os.Getenv("GOOGLE_API_KEY") != "" || os.Getenv("GEMINI_API_KEY") != "" {
		return ai.GetModel(ai.ProviderGoogle, "gemini-2.5-flash")
	}

	return nil, fmt.Errorf("no API key found. Set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GOOGLE_API_KEY")
}

func printHelp() {
	fmt.Println(`agentsdk - AI coding agent

USAGE:
  agentsdk [flags] [prompt]

FLAGS:
  --model <id>          Model ID (e.g., claude-sonnet-4-5-20250929, gpt-4o)
  --provider <name>     Provider (anthropic, openai, google)
  --thinking <level>    Thinking level (off, minimal, low, medium, high, xhigh)
  -p, --print <prompt>  Non-interactive mode: send prompt and print response
  --api-key <key>       API key (overrides environment variable)
  --cwd <dir>           Working directory
  --read-only           Use read-only tools (no write/edit/bash)
  --max-turns <n>       Maximum LLM round-trips (0 = unlimited)
  --append-prompt <text> Append text to system prompt
  --list-models         List available models
  --version             Print version
  --help                Print this help

ENVIRONMENT:
  ANTHROPIC_API_KEY     Anthropic API key
  OPENAI_API_KEY        OpenAI API key
  GOOGLE_API_KEY        Google AI API key

EXAMPLES:
  agentsdk                                    # Interactive mode (auto-detects provider)
  agentsdk --model gpt-4o                     # Use GPT-4o
  agentsdk -p "explain this codebase"         # Print mode
  agentsdk --thinking high "fix the bug"      # With extended thinking`)
}

func printModels() {
	fmt.Println("Available models:")
	for _, provider := range ai.ListProviders() {
		fmt.Printf("  %s:\n", provider)
		for _, model := range ai.ListModels(provider) {
			reasoning := ""
			if model.Reasoning {
				reasoning = " (reasoning)"
			}
			fmt.Printf("    %-35s %s%s\n", model.ID, model.Name, reasoning)
		}
		fmt.Println()
	}
}
