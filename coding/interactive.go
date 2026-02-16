package coding

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"agentsdk/agent"
	"agentsdk/ai"
	"agentsdk/coding/tools"
	"agentsdk/tui"
)

// InteractiveConfig configures the interactive coding agent.
type InteractiveConfig struct {
	// Model is the LLM to use.
	Model *ai.Model
	// ThinkingLevel controls chain-of-thought.
	ThinkingLevel ai.ThinkingLevel
	// Cwd is the working directory.
	Cwd string
	// APIKey overrides the default API key.
	APIKey string
	// SystemPrompt overrides the default system prompt.
	SystemPrompt string
	// AppendPrompt is appended to the system prompt.
	AppendPrompt string
	// Tools overrides the default tool set. If nil, coding tools are used.
	Tools []agent.AgentTool
	// SessionDir overrides the session directory.
	SessionDir string
	// MaxTurns limits the number of LLM round-trips. 0 = unlimited.
	MaxTurns int
	// ReadOnly uses read-only tools instead of coding tools.
	ReadOnly bool
}

// Predefined text styles.
var (
	styleDim  = tui.TextStyle{Dim: true}
	styleBold = tui.TextStyle{Bold: true}
)

// InteractiveMode runs the interactive coding agent with a terminal UI.
type InteractiveMode struct {
	config  InteractiveConfig
	agent   *agent.Agent
	session *SessionManager
	app     *tui.TUI

	// TUI components
	messagesView *tui.Container
	inputEditor  *tui.Editor
	statusBar    *tui.TruncatedText
	loader       *tui.CancellableLoader
}

// NewInteractiveMode creates a new interactive mode.
func NewInteractiveMode(cfg InteractiveConfig) (*InteractiveMode, error) {
	if cfg.Cwd == "" {
		var err error
		cfg.Cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	// Set up tools
	agentTools := cfg.Tools
	if agentTools == nil {
		if cfg.ReadOnly {
			agentTools = tools.ReadOnlyTools(cfg.Cwd)
		} else {
			agentTools = tools.CodingTools(cfg.Cwd)
		}
	}

	// Set up system prompt
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		toolNames := make([]string, len(agentTools))
		for i, t := range agentTools {
			toolNames[i] = t.Name
		}
		systemPrompt = BuildSystemPrompt(SystemPromptOptions{
			Cwd:           cfg.Cwd,
			SelectedTools: toolNames,
			AppendPrompt:  cfg.AppendPrompt,
		})
	}

	// Set up session manager
	sessionDir := cfg.SessionDir
	if sessionDir == "" {
		sessionDir = DefaultSessionDir()
	}
	sessionMgr, err := NewSessionManager(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("create session manager: %w", err)
	}
	if err := sessionMgr.NewSession(); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Create agent
	ag := agent.New(agent.Config{
		Model:         cfg.Model,
		SystemPrompt:  systemPrompt,
		Tools:         agentTools,
		ThinkingLevel: cfg.ThinkingLevel,
		APIKey:        cfg.APIKey,
		SteeringMode:  agent.SteeringOneAtATime,
		MaxTurns:      cfg.MaxTurns,
	})

	// Create TUI components
	messagesView := tui.NewContainer()
	inputEditor := tui.NewEditor()

	statusText := fmt.Sprintf(" %s | %s | %s", cfg.Model.Name, cfg.Cwd, sessionMgr.SessionID())
	statusBar := tui.NewTruncatedText(statusText)
	statusBar.SetStyle(styleDim)

	loader := tui.NewCancellableLoader("Thinking...", "Ctrl+C")

	im := &InteractiveMode{
		config:       cfg,
		agent:        ag,
		session:      sessionMgr,
		messagesView: messagesView,
		inputEditor:  inputEditor,
		statusBar:    statusBar,
		loader:       loader,
	}

	return im, nil
}

// Run starts the interactive mode. Blocks until the user exits.
func (im *InteractiveMode) Run(ctx context.Context) error {
	defer im.session.Close()

	// Build the UI layout
	root := tui.NewContainer()

	// Header
	header := tui.NewTruncatedText(fmt.Sprintf(" agentsdk - %s", im.config.Model.Name))
	header.SetStyle(styleBold)
	root.AddChild(header)
	root.AddChild(tui.NewSpacer(1))

	// Messages area
	root.AddChild(im.messagesView)
	root.AddChild(tui.NewSpacer(1))

	// Input area
	root.AddChild(im.inputEditor)
	root.AddChild(im.statusBar)

	// Create TUI
	im.app = tui.New(root)
	im.app.SetFocus(im.inputEditor)

	// Subscribe to agent events
	im.agent.Subscribe(im.handleAgentEvent)

	// Set up input handling
	im.inputEditor.SetOnSubmit(func(text string) {
		im.handleSubmit(ctx, text)
	})

	// Add welcome message
	welcome := tui.NewText(fmt.Sprintf("Welcome! Using %s. Type your message below.", im.config.Model.Name))
	welcome.SetStyle(styleDim)
	im.messagesView.AddChild(welcome)
	im.messagesView.AddChild(tui.NewSpacer(1))

	// Run the TUI
	return im.app.Run(ctx)
}

// handleSubmit processes user input.
func (im *InteractiveMode) handleSubmit(ctx context.Context, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	// Handle commands
	if strings.HasPrefix(text, "/") {
		im.handleCommand(ctx, text)
		return
	}

	// Add user message to UI
	userMsg := tui.NewText(fmt.Sprintf("> %s", text))
	userMsg.SetStyle(styleBold)
	im.messagesView.AddChild(userMsg)
	im.messagesView.AddChild(tui.NewSpacer(1))

	// Show loader
	im.loader.Loader().SetLabel("Thinking...")
	im.messagesView.AddChild(im.loader)

	// Persist user message
	im.session.AppendMessage(ai.NewUserMessage(text), "")

	// Run agent in background
	go func() {
		_, err := im.agent.Prompt(ctx, text)
		if err != nil && ctx.Err() == nil {
			errText := tui.NewText(fmt.Sprintf("Error: %v", err))
			errText.SetStyle(styleBold)
			im.messagesView.AddChild(errText)
		}
		im.messagesView.RemoveChild(im.loader)
		if im.app != nil {
			im.app.Render()
		}
	}()

	if im.app != nil {
		im.app.Render()
	}
}

// handleCommand processes slash commands.
func (im *InteractiveMode) handleCommand(_ context.Context, text string) {
	parts := strings.Fields(text)
	cmd := parts[0]

	switch cmd {
	case "/help":
		help := tui.NewText(strings.Join([]string{
			"Available commands:",
			"  /help    - Show this help message",
			"  /model   - Show or change the current model",
			"  /clear   - Clear the message history",
			"  /session - Show session info",
			"  /exit    - Exit the agent",
		}, "\n"))
		help.SetStyle(styleDim)
		im.messagesView.AddChild(help)
		im.messagesView.AddChild(tui.NewSpacer(1))

	case "/model":
		if len(parts) > 1 {
			msg := tui.NewText(fmt.Sprintf("Model switching not yet implemented. Current: %s", im.config.Model.Name))
			msg.SetStyle(styleDim)
			im.messagesView.AddChild(msg)
		} else {
			msg := tui.NewText(fmt.Sprintf("Current model: %s (%s)", im.config.Model.Name, im.config.Model.ID))
			msg.SetStyle(styleDim)
			im.messagesView.AddChild(msg)
		}
		im.messagesView.AddChild(tui.NewSpacer(1))

	case "/clear":
		im.messagesView.SetChildren(nil)
		msg := tui.NewText("Messages cleared.")
		msg.SetStyle(styleDim)
		im.messagesView.AddChild(msg)
		im.messagesView.AddChild(tui.NewSpacer(1))

	case "/session":
		msg := tui.NewText(fmt.Sprintf("Session: %s\nDirectory: %s",
			im.session.SessionID(), im.config.Cwd))
		msg.SetStyle(styleDim)
		im.messagesView.AddChild(msg)
		im.messagesView.AddChild(tui.NewSpacer(1))

	case "/exit":
		if im.app != nil {
			im.app.Quit()
		}

	default:
		msg := tui.NewText(fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd))
		msg.SetStyle(styleDim)
		im.messagesView.AddChild(msg)
		im.messagesView.AddChild(tui.NewSpacer(1))
	}

	if im.app != nil {
		im.app.Render()
	}
}

// handleAgentEvent processes agent events for UI rendering.
func (im *InteractiveMode) handleAgentEvent(event agent.Event) {
	switch event.Type {
	case agent.EventMessageStart:
		im.loader.Loader().SetLabel("Generating...")

	case agent.EventMessageUpdate:
		if event.StreamEvent != nil && event.StreamEvent.Type == ai.EventTextDelta {
			im.loader.Loader().SetLabel("Writing...")
		}

	case agent.EventMessageEnd:
		if msg, ok := event.Message.(*ai.AssistantMessage); ok {
			text := msg.GetText()
			if text != "" {
				// Render as markdown
				md := tui.NewMarkdown(text)
				im.messagesView.AddChild(md)
				im.messagesView.AddChild(tui.NewSpacer(1))
			}

			// Persist
			im.session.AppendMessage(msg, "")
		}

	case agent.EventToolExecStart:
		label := fmt.Sprintf("Running %s...", event.ToolName)
		im.loader.Loader().SetLabel(label)

	case agent.EventToolExecEnd:
		if event.IsError {
			errText := tui.NewText(fmt.Sprintf("[%s] Error: %s",
				event.ToolName, getContentText(event.ToolResult)))
			errText.SetStyle(styleDim)
			im.messagesView.AddChild(errText)
		} else {
			result := getContentText(event.ToolResult)
			if len(result) > 500 {
				result = result[:500] + "..."
			}
			resultText := tui.NewText(fmt.Sprintf("[%s] %s", event.ToolName, result))
			resultText.SetStyle(styleDim)
			im.messagesView.AddChild(resultText)
		}

	case agent.EventAgentEnd:
		im.loader.Loader().SetLabel("")
		// Update status bar with usage info
		if len(event.Messages) > 0 {
			last := event.Messages[len(event.Messages)-1]
			if msg, ok := last.(*ai.AssistantMessage); ok {
				im.statusBar.SetContent(fmt.Sprintf(" %s | tokens: %d in / %d out | %s",
					im.config.Model.Name, msg.Usage.InputTokens, msg.Usage.OutputTokens,
					time.Now().Format("15:04:05")))
			}
		}
	}

	if im.app != nil {
		im.app.Render()
	}
}

// getContentText extracts text from a tool result.
func getContentText(result *agent.ToolResult) string {
	if result == nil {
		return ""
	}
	var parts []string
	for _, block := range result.Content {
		if block.Type == ai.ContentText {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// RunPrintMode runs the agent in non-interactive (print) mode.
// It sends a single prompt and prints the response to stdout.
func RunPrintMode(ctx context.Context, cfg InteractiveConfig, prompt string) error {
	if cfg.Cwd == "" {
		var err error
		cfg.Cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}

	agentTools := cfg.Tools
	if agentTools == nil {
		if cfg.ReadOnly {
			agentTools = tools.ReadOnlyTools(cfg.Cwd)
		} else {
			agentTools = tools.CodingTools(cfg.Cwd)
		}
	}

	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		toolNames := make([]string, len(agentTools))
		for i, t := range agentTools {
			toolNames[i] = t.Name
		}
		systemPrompt = BuildSystemPrompt(SystemPromptOptions{
			Cwd:           cfg.Cwd,
			SelectedTools: toolNames,
			AppendPrompt:  cfg.AppendPrompt,
		})
	}

	ag := agent.New(agent.Config{
		Model:         cfg.Model,
		SystemPrompt:  systemPrompt,
		Tools:         agentTools,
		ThinkingLevel: cfg.ThinkingLevel,
		APIKey:        cfg.APIKey,
		MaxTurns:      cfg.MaxTurns,
		OnEvent: func(event agent.Event) {
			switch event.Type {
			case agent.EventMessageUpdate:
				if event.StreamEvent != nil && event.StreamEvent.Type == ai.EventTextDelta {
					fmt.Print(event.StreamEvent.Delta)
				}
			case agent.EventToolExecStart:
				fmt.Fprintf(os.Stderr, "\n[%s] executing...\n", event.ToolName)
			case agent.EventToolExecEnd:
				if event.IsError {
					fmt.Fprintf(os.Stderr, "[%s] error\n", event.ToolName)
				} else {
					fmt.Fprintf(os.Stderr, "[%s] done\n", event.ToolName)
				}
			case agent.EventAgentEnd:
				fmt.Println()
			}
		},
	})

	_, err := ag.Prompt(ctx, prompt)
	return err
}
