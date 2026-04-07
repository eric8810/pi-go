// Package agent provides the agent runtime with tool calling, state management,
// and an event-driven loop. It is the Go equivalent of @mariozechner/pi-agent-core.
package agent

import (
	"context"
	"time"

	"pi-go/ai"
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

	// BeforeExecute is called before this tool executes.
	// Runs after the global Config.BeforeToolCall hook.
	// Return nil to allow execution with original args.
	BeforeExecute func(ctx context.Context, toolCallID string, args map[string]any) (*BeforeToolCallResult, error)

	// AfterExecute is called after this tool executes and can transform the result.
	// Runs before the global Config.AfterToolCall hook.
	// Return nil to keep the result unchanged.
	AfterExecute func(ctx context.Context, toolCallID string, args map[string]any, result *ToolResult) (*ToolResult, error)
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

// FollowUpMode controls how follow-up messages are delivered after a turn completes.
type FollowUpMode string

const (
	// FollowUpOneAtATime delivers one follow-up message per turn, triggering a new turn for each.
	FollowUpOneAtATime FollowUpMode = "one-at-a-time"
	// FollowUpAll delivers all queued follow-up messages at once (default).
	FollowUpAll FollowUpMode = "all"
)

// ToolCallAction determines the outcome of a BeforeToolCall hook.
type ToolCallAction string

const (
	// ToolCallAllow allows the tool to execute normally.
	ToolCallAllow ToolCallAction = "allow"
	// ToolCallDeny blocks execution and returns DenyResult to the LLM.
	ToolCallDeny ToolCallAction = "deny"
	// ToolCallProvideResult skips execution and returns ProvidedResult to the LLM directly.
	ToolCallProvideResult ToolCallAction = "provide_result"
)

// BeforeToolCallResult is returned by BeforeToolCall hooks to control execution.
type BeforeToolCallResult struct {
	// Action determines what happens next. Zero value is treated as ToolCallAllow.
	Action ToolCallAction

	// DenyResult is returned to the LLM when Action is ToolCallDeny.
	// If nil, a generic denial message is used.
	DenyResult *ToolResult

	// ProvidedResult is returned to the LLM when Action is ToolCallProvideResult,
	// bypassing actual tool execution.
	ProvidedResult *ToolResult

	// ReplaceArgs replaces the tool arguments before execution.
	// Only applied when Action is ToolCallAllow.
	ReplaceArgs map[string]any
}

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

	// FollowUpMode controls how follow-up messages are delivered after a turn completes.
	// Defaults to FollowUpAll.
	FollowUpMode FollowUpMode

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

	// BeforeToolCall is called before any tool executes.
	// Runs before the per-tool BeforeExecute hook.
	// Return nil to allow execution with original args.
	BeforeToolCall func(ctx context.Context, toolCallID string, toolName string, args map[string]any) (*BeforeToolCallResult, error)

	// AfterToolCall is called after any tool executes and can transform the result.
	// Runs after the per-tool AfterExecute hook.
	// Return nil to keep the result unchanged.
	AfterToolCall func(ctx context.Context, toolCallID string, toolName string, args map[string]any, result *ToolResult) (*ToolResult, error)

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
