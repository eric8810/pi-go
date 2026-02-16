package ai

import (
	"context"
	"testing"
	"time"
)

// mockAPIProvider is a minimal APIProvider implementation for testing.
type mockAPIProvider struct {
	api API
}

func (p *mockAPIProvider) API() API { return p.api }

func (p *mockAPIProvider) Stream(_ context.Context, _ *Model, _ *Context, _ StreamOptions) *EventStream {
	s := NewEventStream(1)
	msg := &AssistantMessage{Content: []ContentBlock{TextBlock("mock response")}}
	s.End(msg)
	return s
}

// ---------------------------------------------------------------------------
// RegisterAPIProvider / GetAPIProvider
// ---------------------------------------------------------------------------

func TestRegisterAndGetAPIProvider(t *testing.T) {
	api := API("test-api-roundtrip")
	p := &mockAPIProvider{api: api}
	RegisterAPIProvider(p)

	got := GetAPIProvider(api)
	if got == nil {
		t.Fatal("GetAPIProvider returned nil after registration")
	}
	if got.API() != api {
		t.Errorf("API() = %q, want %q", got.API(), api)
	}
}

func TestGetAPIProvider_Unregistered(t *testing.T) {
	got := GetAPIProvider(API("not-registered-at-all"))
	if got != nil {
		t.Errorf("expected nil for unregistered API, got %v", got)
	}
}

func TestRegisterAPIProvider_Overwrite(t *testing.T) {
	api := API("test-overwrite-api")
	p1 := &mockAPIProvider{api: api}
	p2 := &mockAPIProvider{api: api}

	RegisterAPIProvider(p1)
	RegisterAPIProvider(p2)

	got := GetAPIProvider(api)
	if got != p2 {
		t.Error("expected the second registered provider to replace the first")
	}
}

// ---------------------------------------------------------------------------
// MessagesToLLM
// ---------------------------------------------------------------------------

func TestMessagesToLLM_AllTypes(t *testing.T) {
	ts := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	messages := []Message{
		&UserMessage{
			Content:   []ContentBlock{TextBlock("hello")},
			Timestamp: ts,
		},
		&AssistantMessage{
			Content:      []ContentBlock{TextBlock("hi there")},
			API:          APIAnthropicMessages,
			Provider:     ProviderAnthropic,
			Model:        "claude-opus-4-6",
			Usage:        Usage{InputTokens: 10, OutputTokens: 20},
			StopReason:   StopReasonStop,
			ErrorMessage: "",
			Timestamp:    ts,
		},
		&ToolResultMessage{
			ToolCallID: "tc-1",
			ToolName:   "bash",
			Content:    []ContentBlock{TextBlock("output")},
			IsError:    false,
			Timestamp:  ts,
		},
	}

	result := MessagesToLLM(messages)
	if len(result) != 3 {
		t.Fatalf("expected 3 message JSONs, got %d", len(result))
	}

	// Check user message
	if result[0].Role != RoleUser {
		t.Errorf("result[0].Role = %q, want %q", result[0].Role, RoleUser)
	}
	if len(result[0].Content) != 1 || result[0].Content[0].Text != "hello" {
		t.Errorf("result[0] content mismatch: %+v", result[0].Content)
	}

	// Check assistant message
	if result[1].Role != RoleAssistant {
		t.Errorf("result[1].Role = %q, want %q", result[1].Role, RoleAssistant)
	}
	if result[1].Model != "claude-opus-4-6" {
		t.Errorf("result[1].Model = %q, want %q", result[1].Model, "claude-opus-4-6")
	}
	if result[1].Usage == nil {
		t.Fatal("result[1].Usage is nil")
	}
	if result[1].Usage.InputTokens != 10 {
		t.Errorf("result[1].Usage.InputTokens = %d, want 10", result[1].Usage.InputTokens)
	}
	if result[1].StopReason != StopReasonStop {
		t.Errorf("result[1].StopReason = %q, want %q", result[1].StopReason, StopReasonStop)
	}

	// Check tool result message
	if result[2].Role != RoleToolResult {
		t.Errorf("result[2].Role = %q, want %q", result[2].Role, RoleToolResult)
	}
	if result[2].ToolCallID != "tc-1" {
		t.Errorf("result[2].ToolCallID = %q, want %q", result[2].ToolCallID, "tc-1")
	}
	if result[2].ToolName != "bash" {
		t.Errorf("result[2].ToolName = %q, want %q", result[2].ToolName, "bash")
	}
	if result[2].IsError {
		t.Error("result[2].IsError should be false")
	}
}

func TestMessagesToLLM_SkipsNilMessages(t *testing.T) {
	messages := []Message{
		nil,
		&UserMessage{
			Content:   []ContentBlock{TextBlock("hello")},
			Timestamp: time.Now(),
		},
		nil,
		nil,
		&UserMessage{
			Content:   []ContentBlock{TextBlock("world")},
			Timestamp: time.Now(),
		},
		nil,
	}

	result := MessagesToLLM(messages)
	if len(result) != 2 {
		t.Fatalf("expected 2 message JSONs (nils skipped), got %d", len(result))
	}
	if result[0].Content[0].Text != "hello" {
		t.Errorf("result[0] text = %q, want %q", result[0].Content[0].Text, "hello")
	}
	if result[1].Content[0].Text != "world" {
		t.Errorf("result[1] text = %q, want %q", result[1].Content[0].Text, "world")
	}
}

func TestMessagesToLLM_EmptySlice(t *testing.T) {
	result := MessagesToLLM([]Message{})
	if len(result) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(result))
	}
}

func TestMessagesToLLM_AllNils(t *testing.T) {
	result := MessagesToLLM([]Message{nil, nil, nil})
	if len(result) != 0 {
		t.Errorf("expected 0 results for all-nil input, got %d", len(result))
	}
}

func TestMessagesToLLM_NilSlice(t *testing.T) {
	result := MessagesToLLM(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 results for nil input, got %d", len(result))
	}
}

func TestMessagesToLLM_PreservesTimestamp(t *testing.T) {
	ts := time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC)
	messages := []Message{
		&UserMessage{Content: []ContentBlock{TextBlock("test")}, Timestamp: ts},
	}
	result := MessagesToLLM(messages)
	if !result[0].Timestamp.Equal(ts) {
		t.Errorf("timestamp = %v, want %v", result[0].Timestamp, ts)
	}
}

func TestMessagesToLLM_AssistantErrorMessage(t *testing.T) {
	messages := []Message{
		&AssistantMessage{
			Content:      []ContentBlock{},
			StopReason:   StopReasonError,
			ErrorMessage: "rate limited",
			Timestamp:    time.Now(),
		},
	}
	result := MessagesToLLM(messages)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].ErrorMessage != "rate limited" {
		t.Errorf("ErrorMessage = %q, want %q", result[0].ErrorMessage, "rate limited")
	}
	if result[0].StopReason != StopReasonError {
		t.Errorf("StopReason = %q, want %q", result[0].StopReason, StopReasonError)
	}
}

func TestMessagesToLLM_ToolResultIsError(t *testing.T) {
	messages := []Message{
		&ToolResultMessage{
			ToolCallID: "tc-err",
			ToolName:   "exec",
			Content:    []ContentBlock{TextBlock("exit code 1")},
			IsError:    true,
			Timestamp:  time.Now(),
		},
	}
	result := MessagesToLLM(messages)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if !result[0].IsError {
		t.Error("expected IsError to be true")
	}
}
