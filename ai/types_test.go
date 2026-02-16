package ai

import (
	"encoding/json"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ContentBlock constructors
// ---------------------------------------------------------------------------

func TestTextBlock(t *testing.T) {
	b := TextBlock("hello")
	if b.Type != ContentText {
		t.Fatalf("expected type %q, got %q", ContentText, b.Type)
	}
	if b.Text != "hello" {
		t.Fatalf("expected text %q, got %q", "hello", b.Text)
	}
}

func TestThinkingBlock(t *testing.T) {
	b := ThinkingBlock("let me think")
	if b.Type != ContentThinking {
		t.Fatalf("expected type %q, got %q", ContentThinking, b.Type)
	}
	if b.Thinking != "let me think" {
		t.Fatalf("expected thinking %q, got %q", "let me think", b.Thinking)
	}
}

func TestImageBlock(t *testing.T) {
	b := ImageBlock("base64data", "image/png")
	if b.Type != ContentImage {
		t.Fatalf("expected type %q, got %q", ContentImage, b.Type)
	}
	if b.Data != "base64data" {
		t.Fatalf("expected data %q, got %q", "base64data", b.Data)
	}
	if b.MimeType != "image/png" {
		t.Fatalf("expected mimeType %q, got %q", "image/png", b.MimeType)
	}
}

func TestToolCallBlock(t *testing.T) {
	args := map[string]any{"path": "/tmp"}
	b := ToolCallBlock("call-1", "readFile", args)
	if b.Type != ContentToolCall {
		t.Fatalf("expected type %q, got %q", ContentToolCall, b.Type)
	}
	if b.ID != "call-1" {
		t.Fatalf("expected id %q, got %q", "call-1", b.ID)
	}
	if b.Name != "readFile" {
		t.Fatalf("expected name %q, got %q", "readFile", b.Name)
	}
	if b.Arguments["path"] != "/tmp" {
		t.Fatalf("expected argument path=%q, got %v", "/tmp", b.Arguments["path"])
	}
}

// ---------------------------------------------------------------------------
// NewUserMessage
// ---------------------------------------------------------------------------

func TestNewUserMessage(t *testing.T) {
	before := time.Now()
	m := NewUserMessage("hi there")
	after := time.Now()

	if m.GetRole() != RoleUser {
		t.Fatalf("expected role %q, got %q", RoleUser, m.GetRole())
	}
	if len(m.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(m.Content))
	}
	if m.Content[0].Type != ContentText || m.Content[0].Text != "hi there" {
		t.Fatalf("unexpected content block: %+v", m.Content[0])
	}
	if m.Timestamp.Before(before) || m.Timestamp.After(after) {
		t.Fatalf("timestamp %v not between %v and %v", m.Timestamp, before, after)
	}
}

// ---------------------------------------------------------------------------
// NewToolResult
// ---------------------------------------------------------------------------

func TestNewToolResult(t *testing.T) {
	before := time.Now()
	r := NewToolResult("call-1", "bash", "output text", true)
	after := time.Now()

	if r.GetRole() != RoleToolResult {
		t.Fatalf("expected role %q, got %q", RoleToolResult, r.GetRole())
	}
	if r.ToolCallID != "call-1" {
		t.Fatalf("expected toolCallID %q, got %q", "call-1", r.ToolCallID)
	}
	if r.ToolName != "bash" {
		t.Fatalf("expected toolName %q, got %q", "bash", r.ToolName)
	}
	if !r.IsError {
		t.Fatal("expected IsError to be true")
	}
	if len(r.Content) != 1 || r.Content[0].Text != "output text" {
		t.Fatalf("unexpected content: %+v", r.Content)
	}
	if r.Timestamp.Before(before) || r.Timestamp.After(after) {
		t.Fatalf("timestamp %v not between %v and %v", r.Timestamp, before, after)
	}
}

func TestNewToolResult_NotError(t *testing.T) {
	r := NewToolResult("call-2", "read", "data", false)
	if r.IsError {
		t.Fatal("expected IsError to be false")
	}
}

// ---------------------------------------------------------------------------
// AssistantMessage.GetText
// ---------------------------------------------------------------------------

func TestAssistantMessage_GetText(t *testing.T) {
	tests := []struct {
		name    string
		content []ContentBlock
		want    string
	}{
		{
			name:    "single text block",
			content: []ContentBlock{TextBlock("hello")},
			want:    "hello",
		},
		{
			name: "multiple text blocks",
			content: []ContentBlock{
				TextBlock("hello "),
				ThinkingBlock("hmm"),
				TextBlock("world"),
			},
			want: "hello world",
		},
		{
			name:    "no text blocks",
			content: []ContentBlock{ThinkingBlock("thinking only")},
			want:    "",
		},
		{
			name:    "empty content",
			content: nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &AssistantMessage{Content: tt.content}
			got := m.GetText()
			if got != tt.want {
				t.Errorf("GetText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AssistantMessage.GetToolCalls
// ---------------------------------------------------------------------------

func TestAssistantMessage_GetToolCalls(t *testing.T) {
	tc1 := ToolCallBlock("c1", "bash", map[string]any{"cmd": "ls"})
	tc2 := ToolCallBlock("c2", "read", map[string]any{"path": "/"})

	tests := []struct {
		name    string
		content []ContentBlock
		want    int
	}{
		{
			name:    "two tool calls mixed with text",
			content: []ContentBlock{TextBlock("hi"), tc1, ThinkingBlock("ok"), tc2},
			want:    2,
		},
		{
			name:    "no tool calls",
			content: []ContentBlock{TextBlock("just text")},
			want:    0,
		},
		{
			name:    "empty content",
			content: nil,
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &AssistantMessage{Content: tt.content}
			got := m.GetToolCalls()
			if len(got) != tt.want {
				t.Errorf("GetToolCalls() returned %d calls, want %d", len(got), tt.want)
			}
			// Verify each returned block is actually a tool call.
			for _, b := range got {
				if b.Type != ContentToolCall {
					t.Errorf("returned block has type %q, want %q", b.Type, ContentToolCall)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MarshalMessage / UnmarshalMessage roundtrip
// ---------------------------------------------------------------------------

func TestMarshalUnmarshal_UserMessage(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	orig := &UserMessage{
		Content:   []ContentBlock{TextBlock("hello"), ImageBlock("data", "image/jpeg")},
		Timestamp: ts,
	}

	data, err := MarshalMessage(orig)
	if err != nil {
		t.Fatalf("MarshalMessage: %v", err)
	}

	got, err := UnmarshalMessage(data)
	if err != nil {
		t.Fatalf("UnmarshalMessage: %v", err)
	}

	um, ok := got.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", got)
	}
	if um.GetRole() != RoleUser {
		t.Errorf("role = %q, want %q", um.GetRole(), RoleUser)
	}
	if len(um.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(um.Content))
	}
	if um.Content[0].Text != "hello" {
		t.Errorf("content[0].Text = %q, want %q", um.Content[0].Text, "hello")
	}
	if um.Content[1].Data != "data" || um.Content[1].MimeType != "image/jpeg" {
		t.Errorf("content[1] image block mismatch: %+v", um.Content[1])
	}
	if !um.Timestamp.Equal(ts) {
		t.Errorf("timestamp = %v, want %v", um.Timestamp, ts)
	}
}

func TestMarshalUnmarshal_AssistantMessage(t *testing.T) {
	ts := time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC)
	orig := &AssistantMessage{
		Content:      []ContentBlock{TextBlock("response"), ToolCallBlock("c1", "bash", map[string]any{"cmd": "ls"})},
		API:          APIAnthropicMessages,
		Provider:     ProviderAnthropic,
		Model:        "claude-opus-4-6",
		Usage:        Usage{InputTokens: 100, OutputTokens: 50, CacheReadTokens: 10},
		StopReason:   StopReasonStop,
		ErrorMessage: "",
		Timestamp:    ts,
	}

	data, err := MarshalMessage(orig)
	if err != nil {
		t.Fatalf("MarshalMessage: %v", err)
	}

	got, err := UnmarshalMessage(data)
	if err != nil {
		t.Fatalf("UnmarshalMessage: %v", err)
	}

	am, ok := got.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", got)
	}
	if am.GetRole() != RoleAssistant {
		t.Errorf("role = %q, want %q", am.GetRole(), RoleAssistant)
	}
	if am.API != APIAnthropicMessages {
		t.Errorf("API = %q, want %q", am.API, APIAnthropicMessages)
	}
	if am.Provider != ProviderAnthropic {
		t.Errorf("Provider = %q, want %q", am.Provider, ProviderAnthropic)
	}
	if am.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", am.Model, "claude-opus-4-6")
	}
	if am.Usage.InputTokens != 100 || am.Usage.OutputTokens != 50 || am.Usage.CacheReadTokens != 10 {
		t.Errorf("Usage mismatch: %+v", am.Usage)
	}
	if am.StopReason != StopReasonStop {
		t.Errorf("StopReason = %q, want %q", am.StopReason, StopReasonStop)
	}
	if len(am.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(am.Content))
	}
	if !am.Timestamp.Equal(ts) {
		t.Errorf("timestamp = %v, want %v", am.Timestamp, ts)
	}
}

func TestMarshalUnmarshal_ToolResultMessage(t *testing.T) {
	ts := time.Date(2025, 6, 15, 11, 30, 0, 0, time.UTC)
	orig := &ToolResultMessage{
		ToolCallID: "call-42",
		ToolName:   "grep",
		Content:    []ContentBlock{TextBlock("found 3 matches")},
		IsError:    false,
		Timestamp:  ts,
	}

	data, err := MarshalMessage(orig)
	if err != nil {
		t.Fatalf("MarshalMessage: %v", err)
	}

	got, err := UnmarshalMessage(data)
	if err != nil {
		t.Fatalf("UnmarshalMessage: %v", err)
	}

	tr, ok := got.(*ToolResultMessage)
	if !ok {
		t.Fatalf("expected *ToolResultMessage, got %T", got)
	}
	if tr.GetRole() != RoleToolResult {
		t.Errorf("role = %q, want %q", tr.GetRole(), RoleToolResult)
	}
	if tr.ToolCallID != "call-42" {
		t.Errorf("ToolCallID = %q, want %q", tr.ToolCallID, "call-42")
	}
	if tr.ToolName != "grep" {
		t.Errorf("ToolName = %q, want %q", tr.ToolName, "grep")
	}
	if tr.IsError {
		t.Error("expected IsError to be false")
	}
	if len(tr.Content) != 1 || tr.Content[0].Text != "found 3 matches" {
		t.Errorf("content mismatch: %+v", tr.Content)
	}
	if !tr.Timestamp.Equal(ts) {
		t.Errorf("timestamp = %v, want %v", tr.Timestamp, ts)
	}
}

// ---------------------------------------------------------------------------
// MessageJSON.ToMessage
// ---------------------------------------------------------------------------

func TestMessageJSON_ToMessage_User(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// With Content blocks
	mj := &MessageJSON{
		Role:      RoleUser,
		Content:   []ContentBlock{TextBlock("hello")},
		Timestamp: ts,
	}
	msg := mj.ToMessage()
	um, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}
	if len(um.Content) != 1 || um.Content[0].Text != "hello" {
		t.Errorf("unexpected content: %+v", um.Content)
	}
}

func TestMessageJSON_ToMessage_User_TextShorthand(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// With Text shorthand (no Content blocks)
	mj := &MessageJSON{
		Role:      RoleUser,
		Text:      "shorthand",
		Timestamp: ts,
	}
	msg := mj.ToMessage()
	um, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}
	if len(um.Content) != 1 || um.Content[0].Text != "shorthand" {
		t.Errorf("expected Text shorthand to produce text block, got: %+v", um.Content)
	}
}

func TestMessageJSON_ToMessage_Assistant(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	usage := &Usage{InputTokens: 5, OutputTokens: 10}
	mj := &MessageJSON{
		Role:       RoleAssistant,
		Content:    []ContentBlock{TextBlock("answer")},
		API:        APIOpenAICompletions,
		Provider:   ProviderOpenAI,
		Model:      "gpt-4o",
		Usage:      usage,
		StopReason: StopReasonStop,
		Timestamp:  ts,
	}
	msg := mj.ToMessage()
	am, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}
	if am.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", am.Model, "gpt-4o")
	}
	if am.Usage.InputTokens != 5 || am.Usage.OutputTokens != 10 {
		t.Errorf("Usage mismatch: %+v", am.Usage)
	}
}

func TestMessageJSON_ToMessage_ToolResult(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mj := &MessageJSON{
		Role:       RoleToolResult,
		Content:    []ContentBlock{TextBlock("result")},
		ToolCallID: "tc-1",
		ToolName:   "bash",
		IsError:    true,
		Timestamp:  ts,
	}
	msg := mj.ToMessage()
	tr, ok := msg.(*ToolResultMessage)
	if !ok {
		t.Fatalf("expected *ToolResultMessage, got %T", msg)
	}
	if tr.ToolCallID != "tc-1" {
		t.Errorf("ToolCallID = %q, want %q", tr.ToolCallID, "tc-1")
	}
	if !tr.IsError {
		t.Error("expected IsError to be true")
	}
}

func TestMessageJSON_ToMessage_UnknownRole(t *testing.T) {
	mj := &MessageJSON{Role: "unknown"}
	msg := mj.ToMessage()
	if msg != nil {
		t.Errorf("expected nil for unknown role, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestAssistantMessage_EmptyContent(t *testing.T) {
	m := &AssistantMessage{Content: []ContentBlock{}}
	if m.GetText() != "" {
		t.Errorf("expected empty text, got %q", m.GetText())
	}
	if len(m.GetToolCalls()) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(m.GetToolCalls()))
	}
}

func TestMessageJSON_ToMessage_NilUsage(t *testing.T) {
	mj := &MessageJSON{
		Role:      RoleAssistant,
		Content:   []ContentBlock{TextBlock("hi")},
		Usage:     nil,
		Timestamp: time.Now(),
	}
	msg := mj.ToMessage()
	am, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}
	// derefUsage(nil) should yield zero Usage
	if am.Usage.InputTokens != 0 || am.Usage.OutputTokens != 0 {
		t.Errorf("expected zero Usage, got %+v", am.Usage)
	}
}

func TestUnmarshalMessage_InvalidJSON(t *testing.T) {
	_, err := UnmarshalMessage([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMarshalMessage_ContentBlockJSON(t *testing.T) {
	// Verify that ContentBlock fields serialize correctly
	b := ToolCallBlock("id-1", "fn", map[string]any{"x": float64(42)})
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded ContentBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if decoded.Type != ContentToolCall {
		t.Errorf("type = %q, want %q", decoded.Type, ContentToolCall)
	}
	if decoded.ID != "id-1" {
		t.Errorf("id = %q, want %q", decoded.ID, "id-1")
	}
	if decoded.Arguments["x"] != float64(42) {
		t.Errorf("arguments[x] = %v, want 42", decoded.Arguments["x"])
	}
}

func TestUserMessage_GetTimestamp(t *testing.T) {
	ts := time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC)
	m := &UserMessage{Timestamp: ts}
	if !m.GetTimestamp().Equal(ts) {
		t.Errorf("GetTimestamp() = %v, want %v", m.GetTimestamp(), ts)
	}
}

func TestToolResultMessage_GetTimestamp(t *testing.T) {
	ts := time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC)
	m := &ToolResultMessage{Timestamp: ts}
	if !m.GetTimestamp().Equal(ts) {
		t.Errorf("GetTimestamp() = %v, want %v", m.GetTimestamp(), ts)
	}
}

func TestAssistantMessage_GetTimestamp(t *testing.T) {
	ts := time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC)
	m := &AssistantMessage{Timestamp: ts}
	if !m.GetTimestamp().Equal(ts) {
		t.Errorf("GetTimestamp() = %v, want %v", m.GetTimestamp(), ts)
	}
}
