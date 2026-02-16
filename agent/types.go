// Package agent provides the agent runtime with tool calling, state management,
// and an event-driven loop. It is the Go equivalent of @mariozechner/pi-agent-core.
package agent

import (
	"context"
	"time"

	"agentsdk/ai"
)

// AgentTool extends ai.Tool with an execution function.
type AgentTool struct {
	ai.Tool

	// Label is a human-readable name shown in the UI during execution.
	Label string

	// Execute runs the tool with the given parameters.
	// It receives the tool call ID, parsed arguments, a context for cancellation,
	// and an optional update callback for streaming partial results.
	Execute func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error)
}

// UpdateCallback is called during tool execution to report partial progress.
type UpdateCallback func(update ToolUpdate)

// ToolUpdate represents a partial progress update from a tool.
type ToolUpdate struct {
	// Output is the partial output text so far.
	Output string
	// Metadata holds tool-specific structured update data.
	Metadata map[string]any
}

// ToolResult is the result of executing a tool.
type ToolResult struct {
	// Content is the result content blocks (typically text).
	Content []ai.ContentBlock
	// IsError indicates the tool execution failed.
	IsError bool
	// Metadata holds tool-specific structured result data.
	Metadata map[string]any
}

// TextResult creates a successful text tool result.
func TextResult(text string) *ToolResult {
	return &ToolResult{
		Content: []ai.ContentBlock{ai.TextBlock(text)},
	}
}

// ErrorResult creates an error tool result.
func ErrorResult(text string) *ToolResult {
	return &ToolResult{
		Content: []ai.ContentBlock{ai.TextBlock(text)},
		IsError: true,
	}
}

// State holds the complete state of an agent.
type State struct {
	SystemPrompt  string
	Model         *ai.Model
	ThinkingLevel ai.ThinkingLevel
	Tools         []AgentTool
	Messages      []ai.Message
	IsStreaming   bool
	Error         string
}

// EventType discriminates agent event variants.
type EventType string

const (
	EventAgentStart      EventType = "agent_start"
	EventAgentEnd        EventType = "agent_end"
	EventTurnStart       EventType = "turn_start"
	EventTurnEnd         EventType = "turn_end"
	EventMessageStart    EventType = "message_start"
	EventMessageUpdate   EventType = "message_update"
	EventMessageEnd      EventType = "message_end"
	EventToolExecStart   EventType = "tool_execution_start"
	EventToolExecUpdate  EventType = "tool_execution_update"
	EventToolExecEnd     EventType = "tool_execution_end"
)

// Event is emitted by the agent during execution for UI integration.
type Event struct {
	Type EventType

	// For agent_end
	Messages []ai.Message

	// For message_start, message_update, message_end
	Message ai.Message

	// For message_update
	StreamEvent *ai.StreamEvent

	// For turn_end
	ToolResults []*ai.ToolResultMessage

	// For tool_execution_*
	ToolCallID string
	ToolName   string
	ToolArgs   map[string]any
	ToolResult *ToolResult
	ToolUpdate *ToolUpdate
	IsError    bool
}

// EventHandler is a callback invoked for each agent event.
type EventHandler func(event Event)

// SteeringMode controls how steering messages are delivered.
type SteeringMode string

const (
	// SteeringOneAtATime delivers one steering message per tool call boundary.
	SteeringOneAtATime SteeringMode = "one-at-a-time"
	// SteeringAll delivers all queued steering messages at once.
	SteeringAll SteeringMode = "all"
)

// Config configures the agent loop.
type Config struct {
	// Model is the LLM to use.
	Model *ai.Model

	// SystemPrompt is the system prompt.
	SystemPrompt string

	// Tools available to the agent.
	Tools []AgentTool

	// ThinkingLevel controls chain-of-thought behavior.
	ThinkingLevel ai.ThinkingLevel

	// APIKey overrides the default API key.
	APIKey string

	// SteeringMode controls how steering messages are delivered.
	SteeringMode SteeringMode

	// ConvertToLLM transforms agent messages to LLM-compatible messages.
	// If nil, messages are passed through directly.
	ConvertToLLM func(messages []ai.Message) []ai.Message

	// TransformContext allows pre-processing messages before sending to the LLM.
	// Used for context compaction, pruning, etc.
	TransformContext func(ctx context.Context, messages []ai.Message) ([]ai.Message, error)

	// GetSteeringMessages returns messages to inject between tool calls.
	// Returns nil when no steering is needed.
	GetSteeringMessages func() []ai.Message

	// GetFollowUpMessages returns messages to inject after the agent completes a turn.
	// Returns nil when no follow-up is needed.
	GetFollowUpMessages func() []ai.Message

	// MaxTurns limits the number of LLM round-trips. 0 means unlimited.
	MaxTurns int

	// StreamFunc overrides the default ai.Stream function. Useful for testing.
	StreamFunc ai.StreamFunc

	// OnEvent is called for each agent event.
	OnEvent EventHandler
}

// internalMessage wraps a message with metadata for the agent loop.
type internalMessage struct {
	message   ai.Message
	timestamp time.Time
	isSteering bool
}
