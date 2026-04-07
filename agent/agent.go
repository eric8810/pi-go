package agent

import (
	"context"
	"sync"

	"pi-go/ai"
)

// Agent is a stateful wrapper around the agent loop with pub/sub events,
// steering/follow-up queues, and state management.
type Agent struct {
	mu sync.Mutex

	config   Config
	state    State
	handlers []EventHandler

	// Steering and follow-up queues
	steeringMu    sync.Mutex
	steeringQueue []ai.Message
	followUpMu    sync.Mutex
	followUpQueue []ai.Message

	// Lifecycle
	cancel context.CancelFunc
	doneCh chan struct{}
	idle   bool
}

// New creates a new Agent with the given configuration.
func New(cfg Config) *Agent {
	a := &Agent{
		config: cfg,
		state: State{
			SystemPrompt:  cfg.SystemPrompt,
			Model:         cfg.Model,
			ThinkingLevel: cfg.ThinkingLevel,
			Tools:         cfg.Tools,
		},
		doneCh: make(chan struct{}),
		idle:   true,
	}

	// Wire up steering and follow-up message providers
	if cfg.GetSteeringMessages == nil {
		a.config.GetSteeringMessages = a.drainSteering
	}
	if cfg.GetFollowUpMessages == nil {
		a.config.GetFollowUpMessages = a.drainFollowUps
	}

	// Wire up event handler to broadcast to all subscribers
	a.config.OnEvent = a.emit

	return a
}

// Subscribe registers an event handler. Returns an unsubscribe function.
func (a *Agent) Subscribe(handler EventHandler) func() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.handlers = append(a.handlers, handler)
	idx := len(a.handlers) - 1
	return func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		// nil out to avoid slice reallocation
		if idx < len(a.handlers) {
			a.handlers[idx] = nil
		}
	}
}

// emit broadcasts an event to all subscribers.
func (a *Agent) emit(event Event) {
	a.mu.Lock()
	handlers := make([]EventHandler, len(a.handlers))
	copy(handlers, a.handlers)
	a.mu.Unlock()

	// Update internal state
	switch event.Type {
	case EventAgentStart:
		a.mu.Lock()
		a.state.IsStreaming = true
		a.idle = false
		a.mu.Unlock()
	case EventAgentEnd:
		a.mu.Lock()
		a.state.IsStreaming = false
		a.state.Messages = event.Messages
		a.idle = true
		a.mu.Unlock()
		// Signal done
		select {
		case a.doneCh <- struct{}{}:
		default:
		}
	}

	for _, h := range handlers {
		if h != nil {
			h(event)
		}
	}
}

// Prompt sends a user message and starts the agent loop.
// It blocks until the agent finishes processing (all tool calls complete).
func (a *Agent) Prompt(ctx context.Context, text string) ([]ai.Message, error) {
	a.mu.Lock()
	a.state.Messages = append(a.state.Messages, ai.NewUserMessage(text))
	messages := make([]ai.Message, len(a.state.Messages))
	copy(messages, a.state.Messages)
	cfg := a.config
	a.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()
	defer cancel()

	result, err := RunLoop(ctx, &cfg, messages)

	a.mu.Lock()
	a.state.Messages = result
	a.mu.Unlock()

	return result, err
}

// PromptWithMessages sends pre-built messages and starts the agent loop.
func (a *Agent) PromptWithMessages(ctx context.Context, msgs []ai.Message) ([]ai.Message, error) {
	a.mu.Lock()
	a.state.Messages = append(a.state.Messages, msgs...)
	messages := make([]ai.Message, len(a.state.Messages))
	copy(messages, a.state.Messages)
	cfg := a.config
	a.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()
	defer cancel()

	result, err := RunLoop(ctx, &cfg, messages)

	a.mu.Lock()
	a.state.Messages = result
	a.mu.Unlock()

	return result, err
}

// Steer injects a steering message to be delivered at the next tool call boundary.
func (a *Agent) Steer(msg ai.Message) {
	a.steeringMu.Lock()
	defer a.steeringMu.Unlock()
	a.steeringQueue = append(a.steeringQueue, msg)
}

// SteerText injects a text steering message.
func (a *Agent) SteerText(text string) {
	a.Steer(ai.NewUserMessage(text))
}

// FollowUp queues a message to be delivered after the current agent turn completes.
func (a *Agent) FollowUp(msg ai.Message) {
	a.followUpMu.Lock()
	defer a.followUpMu.Unlock()
	a.followUpQueue = append(a.followUpQueue, msg)
}

// FollowUpText queues a text follow-up message.
func (a *Agent) FollowUpText(text string) {
	a.FollowUp(ai.NewUserMessage(text))
}

// Abort cancels the current agent execution.
func (a *Agent) Abort() {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// WaitForIdle blocks until the agent finishes its current execution.
func (a *Agent) WaitForIdle() {
	a.mu.Lock()
	if a.idle {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	<-a.doneCh
}

// GetState returns a snapshot of the current agent state.
func (a *Agent) GetState() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.state
	s.Messages = make([]ai.Message, len(a.state.Messages))
	copy(s.Messages, a.state.Messages)
	s.Tools = make([]AgentTool, len(a.state.Tools))
	copy(s.Tools, a.state.Tools)
	return s
}

// SetModel changes the model used by the agent.
func (a *Agent) SetModel(model *ai.Model) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Model = model
	a.config.Model = model
}

// SetThinkingLevel changes the thinking level.
func (a *Agent) SetThinkingLevel(level ai.ThinkingLevel) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.ThinkingLevel = level
	a.config.ThinkingLevel = level
}

// SetTools replaces the agent's tool set.
func (a *Agent) SetTools(tools []AgentTool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Tools = tools
	a.config.Tools = tools
}

// SetSystemPrompt changes the system prompt.
func (a *Agent) SetSystemPrompt(prompt string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.SystemPrompt = prompt
	a.config.SystemPrompt = prompt
}

// Messages returns the current message history.
func (a *Agent) Messages() []ai.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	msgs := make([]ai.Message, len(a.state.Messages))
	copy(msgs, a.state.Messages)
	return msgs
}

// IsIdle returns true if the agent is not currently processing.
func (a *Agent) IsIdle() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.idle
}

// ClearSteeringQueue discards all pending steering messages.
func (a *Agent) ClearSteeringQueue() {
	a.steeringMu.Lock()
	defer a.steeringMu.Unlock()
	a.steeringQueue = nil
}

// ClearFollowUpQueue discards all pending follow-up messages.
func (a *Agent) ClearFollowUpQueue() {
	a.followUpMu.Lock()
	defer a.followUpMu.Unlock()
	a.followUpQueue = nil
}

// ClearAllQueues discards all pending steering and follow-up messages.
func (a *Agent) ClearAllQueues() {
	a.ClearSteeringQueue()
	a.ClearFollowUpQueue()
}

// Reset clears the message history. Configuration (model, tools, system prompt) is preserved.
func (a *Agent) Reset() {
	a.mu.Lock()
	a.state.Messages = nil
	a.mu.Unlock()
	a.ClearAllQueues()
}

// ReplaceMessages replaces the entire message history with the provided messages.
func (a *Agent) ReplaceMessages(msgs []ai.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Messages = make([]ai.Message, len(msgs))
	copy(a.state.Messages, msgs)
}

// AppendMessage appends a single message to the history without triggering the agent loop.
func (a *Agent) AppendMessage(msg ai.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Messages = append(a.state.Messages, msg)
}

// drainSteering returns and clears all queued steering messages.
func (a *Agent) drainSteering() []ai.Message {
	a.steeringMu.Lock()
	defer a.steeringMu.Unlock()

	if len(a.steeringQueue) == 0 {
		return nil
	}

	switch a.config.SteeringMode {
	case SteeringOneAtATime:
		msg := a.steeringQueue[0]
		a.steeringQueue = a.steeringQueue[1:]
		return []ai.Message{msg}
	default: // SteeringAll
		msgs := a.steeringQueue
		a.steeringQueue = nil
		return msgs
	}
}

// drainFollowUps returns and clears queued follow-up messages according to FollowUpMode.
func (a *Agent) drainFollowUps() []ai.Message {
	a.followUpMu.Lock()
	defer a.followUpMu.Unlock()

	if len(a.followUpQueue) == 0 {
		return nil
	}

	switch a.config.FollowUpMode {
	case FollowUpOneAtATime:
		msg := a.followUpQueue[0]
		a.followUpQueue = a.followUpQueue[1:]
		return []ai.Message{msg}
	default: // FollowUpAll
		msgs := a.followUpQueue
		a.followUpQueue = nil
		return msgs
	}
}
