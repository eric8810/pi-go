// Package ai provides a unified, provider-agnostic LLM API with streaming support.
// It is the Go equivalent of @mariozechner/pi-ai from the pi-mono project.
package ai

import (
	"encoding/json"
	"time"
)

// API identifies the wire protocol used to communicate with an LLM provider.
type API string

const (
	APIAnthropicMessages   API = "anthropic-messages"
	APIOpenAICompletions   API = "openai-completions"
	APIOpenAIResponses     API = "openai-responses"
	APIGoogleGenerativeAI  API = "google-generative-ai"
	APIBedrockConverse      API = "bedrock-converse-stream"
)

// Provider identifies an LLM hosting provider.
type Provider string

const (
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenAI     Provider = "openai"
	ProviderGoogle     Provider = "google"
	ProviderBedrock    Provider = "amazon-bedrock"
	ProviderGroq       Provider = "groq"
	ProviderXAI        Provider = "xai"
	ProviderOpenRouter Provider = "openrouter"
	ProviderMistral    Provider = "mistral"
)

// ThinkingLevel controls extended thinking / chain-of-thought behavior.
type ThinkingLevel string

const (
	ThinkingOff     ThinkingLevel = "off"
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
	ThinkingXHigh   ThinkingLevel = "xhigh"
)

// StopReason indicates why the LLM stopped generating.
type StopReason string

const (
	StopReasonStop    StopReason = "stop"
	StopReasonLength  StopReason = "length"
	StopReasonToolUse StopReason = "toolUse"
	StopReasonError   StopReason = "error"
	StopReasonAborted StopReason = "aborted"
)

// ContentType discriminates content block variants.
type ContentType string

const (
	ContentText     ContentType = "text"
	ContentThinking ContentType = "thinking"
	ContentImage    ContentType = "image"
	ContentToolCall ContentType = "toolCall"
)

// ContentBlock is a polymorphic content element within a message.
// Use the helper constructors (TextBlock, ThinkingBlock, etc.) to create instances.
type ContentBlock struct {
	Type ContentType `json:"type"`

	// Text content (Type == ContentText)
	Text string `json:"text,omitempty"`

	// Thinking content (Type == ContentThinking)
	Thinking string `json:"thinking,omitempty"`

	// Image content (Type == ContentImage)
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`

	// Tool call content (Type == ContentToolCall)
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// TextBlock creates a text content block.
func TextBlock(text string) ContentBlock {
	return ContentBlock{Type: ContentText, Text: text}
}

// ThinkingBlock creates a thinking content block.
func ThinkingBlock(thinking string) ContentBlock {
	return ContentBlock{Type: ContentThinking, Thinking: thinking}
}

// ImageBlock creates an image content block.
func ImageBlock(data, mimeType string) ContentBlock {
	return ContentBlock{Type: ContentImage, Data: data, MimeType: mimeType}
}

// ToolCallBlock creates a tool call content block.
func ToolCallBlock(id, name string, args map[string]any) ContentBlock {
	return ContentBlock{Type: ContentToolCall, ID: id, Name: name, Arguments: args}
}

// Role identifies the sender of a message.
type Role string

const (
	RoleUser       Role = "user"
	RoleAssistant  Role = "assistant"
	RoleToolResult Role = "toolResult"
)

// Message is the interface implemented by all message types.
type Message interface {
	GetRole() Role
	GetTimestamp() time.Time
}

// UserMessage represents a message from the user.
type UserMessage struct {
	Content   []ContentBlock `json:"content"`
	Timestamp time.Time      `json:"timestamp"`
}

func (m *UserMessage) GetRole() Role         { return RoleUser }
func (m *UserMessage) GetTimestamp() time.Time { return m.Timestamp }

// NewUserMessage creates a UserMessage from a simple text string.
func NewUserMessage(text string) *UserMessage {
	return &UserMessage{
		Content:   []ContentBlock{TextBlock(text)},
		Timestamp: time.Now(),
	}
}

// AssistantMessage represents a response from the LLM.
type AssistantMessage struct {
	Content      []ContentBlock `json:"content"`
	API          API            `json:"api"`
	Provider     Provider       `json:"provider"`
	Model        string         `json:"model"`
	Usage        Usage          `json:"usage"`
	StopReason   StopReason     `json:"stopReason"`
	ErrorMessage string         `json:"errorMessage,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
}

func (m *AssistantMessage) GetRole() Role         { return RoleAssistant }
func (m *AssistantMessage) GetTimestamp() time.Time { return m.Timestamp }

// GetText returns the concatenated text content from the assistant message.
func (m *AssistantMessage) GetText() string {
	var result string
	for _, block := range m.Content {
		if block.Type == ContentText {
			result += block.Text
		}
	}
	return result
}

// GetToolCalls returns all tool call blocks from the assistant message.
func (m *AssistantMessage) GetToolCalls() []ContentBlock {
	var calls []ContentBlock
	for _, block := range m.Content {
		if block.Type == ContentToolCall {
			calls = append(calls, block)
		}
	}
	return calls
}

// ToolResultMessage represents the result of a tool execution.
type ToolResultMessage struct {
	ToolCallID string         `json:"toolCallId"`
	ToolName   string         `json:"toolName"`
	Content    []ContentBlock `json:"content"`
	IsError    bool           `json:"isError"`
	Timestamp  time.Time      `json:"timestamp"`
}

func (m *ToolResultMessage) GetRole() Role         { return RoleToolResult }
func (m *ToolResultMessage) GetTimestamp() time.Time { return m.Timestamp }

// NewToolResult creates a ToolResultMessage with text content.
func NewToolResult(toolCallID, toolName, text string, isError bool) *ToolResultMessage {
	return &ToolResultMessage{
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Content:    []ContentBlock{TextBlock(text)},
		IsError:    isError,
		Timestamp:  time.Now(),
	}
}

// Usage tracks token consumption for a single LLM call.
type Usage struct {
	InputTokens      int `json:"inputTokens"`
	OutputTokens     int `json:"outputTokens"`
	CacheReadTokens  int `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens int `json:"cacheWriteTokens,omitempty"`
}

// Cost holds per-token pricing in dollars per million tokens.
type Cost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// ToolParameter defines a JSON Schema for tool parameters.
type ToolParameter struct {
	Type        string                   `json:"type"`
	Description string                   `json:"description,omitempty"`
	Properties  map[string]ToolParameter `json:"properties,omitempty"`
	Required    []string                 `json:"required,omitempty"`
	Enum        []string                 `json:"enum,omitempty"`
	Items       *ToolParameter           `json:"items,omitempty"`
	Default     any                      `json:"default,omitempty"`
}

// Tool defines a tool that can be called by the LLM.
type Tool struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Parameters  ToolParameter `json:"parameters"`
}

// Context is the complete payload sent to the LLM for a single call.
type Context struct {
	SystemPrompt string    `json:"systemPrompt,omitempty"`
	Messages     []Message `json:"messages"`
	Tools        []Tool    `json:"tools,omitempty"`
}

// StreamEventType discriminates streaming event variants.
type StreamEventType string

const (
	EventStart         StreamEventType = "start"
	EventTextStart     StreamEventType = "text_start"
	EventTextDelta     StreamEventType = "text_delta"
	EventTextEnd       StreamEventType = "text_end"
	EventThinkingStart StreamEventType = "thinking_start"
	EventThinkingDelta StreamEventType = "thinking_delta"
	EventThinkingEnd   StreamEventType = "thinking_end"
	EventToolCallStart StreamEventType = "toolcall_start"
	EventToolCallDelta StreamEventType = "toolcall_delta"
	EventToolCallEnd   StreamEventType = "toolcall_end"
	EventDone          StreamEventType = "done"
	EventError         StreamEventType = "error"
)

// StreamEvent is emitted during LLM response streaming.
type StreamEvent struct {
	Type         StreamEventType  `json:"type"`
	ContentIndex int              `json:"contentIndex,omitempty"`
	Delta        string           `json:"delta,omitempty"`
	ToolCall     *ContentBlock    `json:"toolCall,omitempty"`
	Message      *AssistantMessage `json:"message,omitempty"`
	Partial      *AssistantMessage `json:"partial,omitempty"`
	Reason       StopReason       `json:"reason,omitempty"`
}

// StreamOptions configures a streaming LLM call.
type StreamOptions struct {
	// Thinking controls chain-of-thought behavior.
	Thinking ThinkingLevel

	// MaxTokens overrides the model's default max output tokens.
	MaxTokens int

	// Temperature controls randomness (0.0 = deterministic).
	Temperature *float64

	// APIKey overrides the default API key for this call.
	APIKey string

	// Headers adds custom HTTP headers to the request.
	Headers map[string]string

	// Extra holds provider-specific options.
	Extra map[string]any
}

// MessageJSON is used for JSON serialization/deserialization of messages,
// since Go interfaces require a discriminator for unmarshaling.
type MessageJSON struct {
	Role      Role           `json:"role"`
	Content   []ContentBlock `json:"content,omitempty"`
	Text      string         `json:"text,omitempty"` // shorthand for user messages
	API       API            `json:"api,omitempty"`
	Provider  Provider       `json:"provider,omitempty"`
	Model     string         `json:"model,omitempty"`
	Usage     *Usage         `json:"usage,omitempty"`
	StopReason StopReason    `json:"stopReason,omitempty"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
	ToolCallID string        `json:"toolCallId,omitempty"`
	ToolName   string        `json:"toolName,omitempty"`
	IsError    bool          `json:"isError,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
}

// ToMessage converts a MessageJSON to the appropriate Message type.
func (mj *MessageJSON) ToMessage() Message {
	switch mj.Role {
	case RoleUser:
		content := mj.Content
		if len(content) == 0 && mj.Text != "" {
			content = []ContentBlock{TextBlock(mj.Text)}
		}
		return &UserMessage{Content: content, Timestamp: mj.Timestamp}
	case RoleAssistant:
		return &AssistantMessage{
			Content:      mj.Content,
			API:          mj.API,
			Provider:     mj.Provider,
			Model:        mj.Model,
			Usage:        derefUsage(mj.Usage),
			StopReason:   mj.StopReason,
			ErrorMessage: mj.ErrorMessage,
			Timestamp:    mj.Timestamp,
		}
	case RoleToolResult:
		return &ToolResultMessage{
			ToolCallID: mj.ToolCallID,
			ToolName:   mj.ToolName,
			Content:    mj.Content,
			IsError:    mj.IsError,
			Timestamp:  mj.Timestamp,
		}
	}
	return nil
}

func derefUsage(u *Usage) Usage {
	if u == nil {
		return Usage{}
	}
	return *u
}

// MarshalMessage converts a Message to JSON bytes.
func MarshalMessage(m Message) ([]byte, error) {
	switch msg := m.(type) {
	case *UserMessage:
		return json.Marshal(MessageJSON{
			Role:      RoleUser,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		})
	case *AssistantMessage:
		return json.Marshal(MessageJSON{
			Role:         RoleAssistant,
			Content:      msg.Content,
			API:          msg.API,
			Provider:     msg.Provider,
			Model:        msg.Model,
			Usage:        &msg.Usage,
			StopReason:   msg.StopReason,
			ErrorMessage: msg.ErrorMessage,
			Timestamp:    msg.Timestamp,
		})
	case *ToolResultMessage:
		return json.Marshal(MessageJSON{
			Role:       RoleToolResult,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			ToolName:   msg.ToolName,
			IsError:    msg.IsError,
			Timestamp:  msg.Timestamp,
		})
	}
	return json.Marshal(m)
}

// UnmarshalMessage deserializes a Message from JSON bytes.
func UnmarshalMessage(data []byte) (Message, error) {
	var mj MessageJSON
	if err := json.Unmarshal(data, &mj); err != nil {
		return nil, err
	}
	return mj.ToMessage(), nil
}
