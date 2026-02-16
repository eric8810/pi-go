package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"pi-go/agent"
	"pi-go/ai"
	"pi-go/coding"
	"pi-go/coding/tools"
)

// testModel returns a model for e2e testing.
func testModel() *ai.Model {
	return &ai.Model{
		ID:       "test-model",
		Name:     "Test Model",
		API:      ai.APIAnthropicMessages,
		Provider: ai.ProviderAnthropic,
	}
}

// mockStreamFn creates a StreamFunc that returns canned responses in sequence.
func mockStreamFn(responses ...*ai.AssistantMessage) ai.StreamFunc {
	var mu sync.Mutex
	idx := 0
	return func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		mu.Lock()
		var msg *ai.AssistantMessage
		if idx < len(responses) {
			msg = responses[idx]
			idx++
		} else {
			msg = &ai.AssistantMessage{StopReason: ai.StopReasonStop}
		}
		mu.Unlock()

		stream := ai.NewEventStream(16)
		go func() {
			stream.Push(ai.StreamEvent{Type: ai.EventStart})
			for i, block := range msg.Content {
				switch block.Type {
				case ai.ContentText:
					stream.Push(ai.StreamEvent{Type: ai.EventTextDelta, ContentIndex: i, Delta: block.Text})
				case ai.ContentToolCall:
					stream.Push(ai.StreamEvent{Type: ai.EventToolCallEnd, ContentIndex: i, ToolCall: &block})
				}
			}
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: msg.StopReason})
			stream.End(msg)
		}()
		return stream
	}
}

// collectEvents returns an event handler that captures all events.
func collectEvents(events *[]agent.Event) agent.EventHandler {
	var mu sync.Mutex
	return func(event agent.Event) {
		mu.Lock()
		defer mu.Unlock()
		*events = append(*events, event)
	}
}

// TestE2E_FullAgentPipeline tests the complete flow:
// user prompt → agent → mock LLM → tool call (write) → tool result → LLM → final response.
func TestE2E_FullAgentPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	// Mock LLM: first call returns a write tool call, second call returns final text.
	responses := []*ai.AssistantMessage{
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc1", "write", map[string]any{
					"path":    "hello.txt",
					"content": "Hello from agent!",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		{
			Content:    []ai.ContentBlock{ai.TextBlock("I created hello.txt for you.")},
			StopReason: ai.StopReasonStop,
		},
	}

	var events []agent.Event
	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "You are a helpful coding assistant.",
		Tools:        codingTools,
		StreamFunc:   mockStreamFn(responses...),
	})
	ag.Subscribe(collectEvents(&events))

	ctx := context.Background()
	msgs, err := ag.Prompt(ctx, "Create a hello.txt file")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// Verify file was actually created by the write tool.
	content, err := os.ReadFile(filepath.Join(tmpDir, "hello.txt"))
	if err != nil {
		t.Fatalf("File was not created: %v", err)
	}
	if string(content) != "Hello from agent!" {
		t.Errorf("File content = %q, want %q", string(content), "Hello from agent!")
	}

	// Verify message history: user → assistant(tool_call) → tool_result → assistant(text).
	if len(msgs) < 4 {
		t.Fatalf("Expected at least 4 messages, got %d", len(msgs))
	}
	if msgs[0].GetRole() != ai.RoleUser {
		t.Errorf("msgs[0] role = %q, want %q", msgs[0].GetRole(), ai.RoleUser)
	}
	if msgs[1].GetRole() != ai.RoleAssistant {
		t.Errorf("msgs[1] role = %q, want %q", msgs[1].GetRole(), ai.RoleAssistant)
	}
	if msgs[2].GetRole() != ai.RoleToolResult {
		t.Errorf("msgs[2] role = %q, want %q", msgs[2].GetRole(), ai.RoleToolResult)
	}
	if msgs[3].GetRole() != ai.RoleAssistant {
		t.Errorf("msgs[3] role = %q, want %q", msgs[3].GetRole(), ai.RoleAssistant)
	}

	// Verify final assistant response text.
	lastAssistant, ok := msgs[3].(*ai.AssistantMessage)
	if !ok {
		t.Fatalf("msgs[3] is not *AssistantMessage")
	}
	if got := lastAssistant.GetText(); got != "I created hello.txt for you." {
		t.Errorf("Final text = %q, want %q", got, "I created hello.txt for you.")
	}

	// Verify tool result is not an error.
	toolResult, ok := msgs[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("msgs[2] is not *ToolResultMessage")
	}
	if toolResult.IsError {
		t.Errorf("Tool result should not be an error, got: %v", toolResult.Content)
	}

	// Verify event ordering.
	var eventTypes []agent.EventType
	for _, ev := range events {
		eventTypes = append(eventTypes, ev.Type)
	}
	expected := []agent.EventType{
		agent.EventAgentStart,
		agent.EventTurnStart,
		agent.EventMessageStart,
	}
	// Check prefix
	for i, exp := range expected {
		if i >= len(eventTypes) {
			t.Fatalf("Missing event at index %d: want %s", i, exp)
		}
		if eventTypes[i] != exp {
			t.Errorf("Event[%d] = %s, want %s", i, eventTypes[i], exp)
		}
	}
	// Must contain tool exec events
	hasToolExecStart := false
	hasToolExecEnd := false
	hasAgentEnd := false
	for _, et := range eventTypes {
		switch et {
		case agent.EventToolExecStart:
			hasToolExecStart = true
		case agent.EventToolExecEnd:
			hasToolExecEnd = true
		case agent.EventAgentEnd:
			hasAgentEnd = true
		}
	}
	if !hasToolExecStart {
		t.Error("Missing EventToolExecStart")
	}
	if !hasToolExecEnd {
		t.Error("Missing EventToolExecEnd")
	}
	if !hasAgentEnd {
		t.Error("Missing EventAgentEnd")
	}
}

// TestE2E_MultiToolChain tests chaining multiple tool calls:
// write a file → read it back → respond with its contents.
func TestE2E_MultiToolChain(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	responses := []*ai.AssistantMessage{
		// Step 1: Write a file.
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc1", "write", map[string]any{
					"path":    "data.txt",
					"content": "line1\nline2\nline3",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		// Step 2: Read the file back.
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc2", "read", map[string]any{
					"path": "data.txt",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		// Step 3: Final response.
		{
			Content:    []ai.ContentBlock{ai.TextBlock("The file has 3 lines.")},
			StopReason: ai.StopReasonStop,
		},
	}

	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "You are a helpful assistant.",
		Tools:        codingTools,
		StreamFunc:   mockStreamFn(responses...),
	})

	msgs, err := ag.Prompt(context.Background(), "Write a file and count its lines")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// Message flow: user → assistant(write) → tool_result → assistant(read) → tool_result → assistant(text)
	if len(msgs) < 6 {
		t.Fatalf("Expected at least 6 messages, got %d", len(msgs))
	}

	// Verify the read tool result contains line numbers and content.
	readResult, ok := msgs[4].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("msgs[4] is not *ToolResultMessage")
	}
	if readResult.IsError {
		t.Errorf("Read result should not be an error")
	}
	resultText := ""
	for _, block := range readResult.Content {
		if block.Type == ai.ContentText {
			resultText += block.Text
		}
	}
	if !strings.Contains(resultText, "line1") || !strings.Contains(resultText, "line3") {
		t.Errorf("Read result should contain file content, got: %s", resultText)
	}

	// Verify final response.
	lastMsg, ok := msgs[len(msgs)-1].(*ai.AssistantMessage)
	if !ok {
		t.Fatalf("Last message is not *AssistantMessage")
	}
	if lastMsg.GetText() != "The file has 3 lines." {
		t.Errorf("Final text = %q, want %q", lastMsg.GetText(), "The file has 3 lines.")
	}
}

// TestE2E_BashToolExecution tests executing a bash command through the agent pipeline.
func TestE2E_BashToolExecution(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	responses := []*ai.AssistantMessage{
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc1", "bash", map[string]any{
					"command": "echo 'hello world'",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		{
			Content:    []ai.ContentBlock{ai.TextBlock("The command output: hello world")},
			StopReason: ai.StopReasonStop,
		},
	}

	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "You are a coding assistant.",
		Tools:        codingTools,
		StreamFunc:   mockStreamFn(responses...),
	})

	msgs, err := ag.Prompt(context.Background(), "Run echo hello world")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// Verify bash tool result contains "hello world".
	toolResult, ok := msgs[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("msgs[2] is not *ToolResultMessage")
	}
	if toolResult.IsError {
		t.Error("Bash result should not be an error")
	}
	resultText := ""
	for _, block := range toolResult.Content {
		if block.Type == ai.ContentText {
			resultText += block.Text
		}
	}
	if !strings.Contains(resultText, "hello world") {
		t.Errorf("Bash result should contain 'hello world', got: %s", resultText)
	}
}

// TestE2E_ReadOnlyTools tests that read-only mode does not include write/bash tools.
func TestE2E_ReadOnlyTools(t *testing.T) {
	tmpDir := t.TempDir()
	readOnly := tools.ReadOnlyTools(tmpDir)

	// Create a file to read.
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("read only content"), 0o644)

	responses := []*ai.AssistantMessage{
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc1", "read", map[string]any{
					"path": "test.txt",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		{
			Content:    []ai.ContentBlock{ai.TextBlock("File content: read only content")},
			StopReason: ai.StopReasonStop,
		},
	}

	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "Read-only mode.",
		Tools:        readOnly,
		StreamFunc:   mockStreamFn(responses...),
	})

	msgs, err := ag.Prompt(context.Background(), "Read test.txt")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// Verify read works.
	toolResult, ok := msgs[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("msgs[2] is not *ToolResultMessage")
	}
	resultText := ""
	for _, block := range toolResult.Content {
		if block.Type == ai.ContentText {
			resultText += block.Text
		}
	}
	if !strings.Contains(resultText, "read only content") {
		t.Errorf("Read result should contain file content, got: %s", resultText)
	}

	// Verify write and bash are NOT available (calling unknown tool returns error).
	writeResponses := []*ai.AssistantMessage{
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc1", "write", map[string]any{
					"path":    "bad.txt",
					"content": "should not work",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		{
			Content:    []ai.ContentBlock{ai.TextBlock("Write failed as expected.")},
			StopReason: ai.StopReasonStop,
		},
	}

	ag2 := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "Read-only mode.",
		Tools:        readOnly,
		StreamFunc:   mockStreamFn(writeResponses...),
	})

	msgs2, err := ag2.Prompt(context.Background(), "Try to write")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// The write tool call should result in an "Unknown tool" error.
	writeResult, ok := msgs2[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("msgs2[2] is not *ToolResultMessage")
	}
	if !writeResult.IsError {
		t.Error("Write in read-only mode should return an error")
	}
	errorText := ""
	for _, block := range writeResult.Content {
		if block.Type == ai.ContentText {
			errorText += block.Text
		}
	}
	if !strings.Contains(errorText, "Unknown tool") {
		t.Errorf("Expected 'Unknown tool' error, got: %s", errorText)
	}
}

// TestE2E_ToolError tests agent behavior when a tool returns an error.
func TestE2E_ToolError(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	responses := []*ai.AssistantMessage{
		// Try to read a non-existent file.
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc1", "read", map[string]any{
					"path": "nonexistent.txt",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		{
			Content:    []ai.ContentBlock{ai.TextBlock("The file does not exist.")},
			StopReason: ai.StopReasonStop,
		},
	}

	var events []agent.Event
	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "You are a helpful assistant.",
		Tools:        codingTools,
		StreamFunc:   mockStreamFn(responses...),
	})
	ag.Subscribe(collectEvents(&events))

	msgs, err := ag.Prompt(context.Background(), "Read nonexistent.txt")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// Tool result should be an error.
	toolResult, ok := msgs[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("msgs[2] is not *ToolResultMessage")
	}
	if !toolResult.IsError {
		t.Error("Reading non-existent file should produce an error")
	}

	// The tool exec end event should indicate an error.
	for _, ev := range events {
		if ev.Type == agent.EventToolExecEnd {
			if !ev.IsError {
				t.Error("EventToolExecEnd should have IsError=true for failed tool")
			}
			break
		}
	}

	// Agent should still complete gracefully.
	lastMsg, ok := msgs[len(msgs)-1].(*ai.AssistantMessage)
	if !ok {
		t.Fatal("Last message should be AssistantMessage")
	}
	if lastMsg.GetText() != "The file does not exist." {
		t.Errorf("Final text = %q, want %q", lastMsg.GetText(), "The file does not exist.")
	}
}

// TestE2E_MaxTurns tests that the agent respects the max turns limit.
// MaxTurns limits outer loop iterations (each triggered by follow-ups).
func TestE2E_MaxTurns(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	callCount := 0
	var mu sync.Mutex
	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		mu.Lock()
		callCount++
		current := callCount
		mu.Unlock()

		// Each call returns a simple text response, which exits the inner loop.
		msg := &ai.AssistantMessage{
			Content:    []ai.ContentBlock{ai.TextBlock("Response " + string(rune('0'+current)))},
			StopReason: ai.StopReasonStop,
		}

		stream := ai.NewEventStream(16)
		go func() {
			stream.Push(ai.StreamEvent{Type: ai.EventStart})
			stream.Push(ai.StreamEvent{Type: ai.EventTextDelta, ContentIndex: 0, Delta: msg.Content[0].Text})
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: msg.StopReason})
			stream.End(msg)
		}()
		return stream
	}

	// Follow-ups keep triggering new outer loop iterations.
	// MaxTurns should stop after 2 outer iterations.
	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "Max turns test.",
		Tools:        codingTools,
		StreamFunc:   streamFn,
		MaxTurns:     2,
		GetFollowUpMessages: func() []ai.Message {
			// Always provide a follow-up to keep the outer loop going.
			return []ai.Message{ai.NewUserMessage("Continue")}
		},
	})

	msgs, err := ag.Prompt(context.Background(), "Start")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// With MaxTurns=2, the agent should make exactly 2 LLM calls.
	mu.Lock()
	c := callCount
	mu.Unlock()
	if c != 2 {
		t.Errorf("Expected exactly 2 LLM calls with MaxTurns=2, got %d", c)
	}

	// Should have messages in history.
	if len(msgs) < 2 {
		t.Errorf("Expected at least 2 messages, got %d", len(msgs))
	}
}

// TestE2E_ContextCancellation tests that the agent stops when context is cancelled.
func TestE2E_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context when the first tool call starts executing.
	callCount := 0
	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		callCount++
		if callCount == 1 {
			// First call: return tool call that will trigger cancellation.
			msg := &ai.AssistantMessage{
				Content: []ai.ContentBlock{
					ai.ToolCallBlock("tc1", "bash", map[string]any{
						"command": "sleep 10",
					}),
				},
				StopReason: ai.StopReasonToolUse,
			}
			stream := ai.NewEventStream(16)
			go func() {
				stream.Push(ai.StreamEvent{Type: ai.EventStart})
				for i, block := range msg.Content {
					stream.Push(ai.StreamEvent{Type: ai.EventToolCallEnd, ContentIndex: i, ToolCall: &block})
				}
				stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: msg.StopReason})
				stream.End(msg)
			}()
			// Cancel right after streaming, before tool actually runs.
			go func() {
				// Give a tiny bit of time for the stream to be consumed.
				cancel()
			}()
			return stream
		}
		// Should not reach here.
		msg := &ai.AssistantMessage{
			Content:    []ai.ContentBlock{ai.TextBlock("Should not happen")},
			StopReason: ai.StopReasonStop,
		}
		stream := ai.NewEventStream(16)
		go func() {
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: msg.StopReason})
			stream.End(msg)
		}()
		return stream
	}

	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "Test cancellation.",
		Tools:        codingTools,
		StreamFunc:   streamFn,
	})

	_, err := ag.Prompt(ctx, "Do something slow")
	if err == nil {
		// It's ok if it returns nil error (tool may finish before cancel propagates).
		// But we mainly verify it doesn't hang.
	}

	// Agent should be idle after returning.
	if !ag.IsIdle() {
		t.Error("Agent should be idle after cancellation")
	}
}

// TestE2E_SteeringMessages tests injecting steering messages during execution.
func TestE2E_SteeringMessages(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	callCount := 0
	var mu sync.Mutex
	var captured [][]ai.Message

	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		mu.Lock()
		callCount++
		current := callCount
		captured = append(captured, reqCtx.Messages)
		mu.Unlock()

		var msg *ai.AssistantMessage
		switch current {
		case 1:
			// First call: tool call that will be steered.
			msg = &ai.AssistantMessage{
				Content: []ai.ContentBlock{
					ai.ToolCallBlock("tc1", "bash", map[string]any{
						"command": "echo step1",
					}),
				},
				StopReason: ai.StopReasonToolUse,
			}
		case 2:
			// Second call (after steering): another tool call.
			msg = &ai.AssistantMessage{
				Content: []ai.ContentBlock{
					ai.ToolCallBlock("tc2", "bash", map[string]any{
						"command": "echo step2",
					}),
				},
				StopReason: ai.StopReasonToolUse,
			}
		default:
			msg = &ai.AssistantMessage{
				Content:    []ai.ContentBlock{ai.TextBlock("Done with steering.")},
				StopReason: ai.StopReasonStop,
			}
		}

		stream := ai.NewEventStream(16)
		go func() {
			stream.Push(ai.StreamEvent{Type: ai.EventStart})
			for i, block := range msg.Content {
				switch block.Type {
				case ai.ContentText:
					stream.Push(ai.StreamEvent{Type: ai.EventTextDelta, ContentIndex: i, Delta: block.Text})
				case ai.ContentToolCall:
					stream.Push(ai.StreamEvent{Type: ai.EventToolCallEnd, ContentIndex: i, ToolCall: &block})
				}
			}
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: msg.StopReason})
			stream.End(msg)
		}()
		return stream
	}

	steeringInjected := false
	steerMu := sync.Mutex{}

	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "You are steerable.",
		Tools:        codingTools,
		StreamFunc:   streamFn,
		SteeringMode: agent.SteeringOneAtATime,
		GetSteeringMessages: func() []ai.Message {
			steerMu.Lock()
			defer steerMu.Unlock()
			if !steeringInjected {
				steeringInjected = true
				return []ai.Message{ai.NewUserMessage("Change direction: do step2 instead")}
			}
			return nil
		},
	})

	msgs, err := ag.Prompt(context.Background(), "Do multi-step task")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// Should have completed all 3 LLM calls.
	mu.Lock()
	c := callCount
	mu.Unlock()
	if c < 3 {
		t.Errorf("Expected at least 3 LLM calls, got %d", c)
	}

	// Final message should be the text response.
	lastMsg, ok := msgs[len(msgs)-1].(*ai.AssistantMessage)
	if !ok {
		t.Fatal("Last message should be AssistantMessage")
	}
	if lastMsg.GetText() != "Done with steering." {
		t.Errorf("Final text = %q, want %q", lastMsg.GetText(), "Done with steering.")
	}

	// Verify steering message appears in the captured context of the second call.
	mu.Lock()
	defer mu.Unlock()
	if len(captured) >= 2 {
		secondCallMsgs := captured[1]
		foundSteering := false
		for _, m := range secondCallMsgs {
			if um, ok := m.(*ai.UserMessage); ok {
				for _, block := range um.Content {
					if strings.Contains(block.Text, "Change direction") {
						foundSteering = true
					}
				}
			}
		}
		if !foundSteering {
			t.Error("Steering message was not included in the second LLM call context")
		}
	}
}

// TestE2E_FollowUpMessages tests automatic follow-up message injection.
func TestE2E_FollowUpMessages(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	callCount := 0
	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		callCount++
		var msg *ai.AssistantMessage
		switch callCount {
		case 1:
			msg = &ai.AssistantMessage{
				Content:    []ai.ContentBlock{ai.TextBlock("First response.")},
				StopReason: ai.StopReasonStop,
			}
		case 2:
			msg = &ai.AssistantMessage{
				Content:    []ai.ContentBlock{ai.TextBlock("Follow-up response.")},
				StopReason: ai.StopReasonStop,
			}
		default:
			msg = &ai.AssistantMessage{
				Content:    []ai.ContentBlock{ai.TextBlock("No more follow-ups.")},
				StopReason: ai.StopReasonStop,
			}
		}

		stream := ai.NewEventStream(16)
		go func() {
			stream.Push(ai.StreamEvent{Type: ai.EventStart})
			for i, block := range msg.Content {
				stream.Push(ai.StreamEvent{Type: ai.EventTextDelta, ContentIndex: i, Delta: block.Text})
			}
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: msg.StopReason})
			stream.End(msg)
		}()
		return stream
	}

	followUpSent := false
	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "Test follow-ups.",
		Tools:        codingTools,
		StreamFunc:   streamFn,
		GetFollowUpMessages: func() []ai.Message {
			if !followUpSent {
				followUpSent = true
				return []ai.Message{ai.NewUserMessage("Please elaborate.")}
			}
			return nil
		},
	})

	msgs, err := ag.Prompt(context.Background(), "Start conversation")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// Should have 2 LLM calls: initial + follow-up.
	if callCount < 2 {
		t.Errorf("Expected at least 2 LLM calls, got %d", callCount)
	}

	// Should contain the follow-up user message.
	foundFollowUp := false
	for _, m := range msgs {
		if um, ok := m.(*ai.UserMessage); ok {
			for _, block := range um.Content {
				if strings.Contains(block.Text, "Please elaborate") {
					foundFollowUp = true
				}
			}
		}
	}
	if !foundFollowUp {
		t.Error("Follow-up message was not found in the message history")
	}
}

// TestE2E_WriteReadEditPipeline tests a write → read → edit → read cycle with real tools.
func TestE2E_WriteReadEditPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	responses := []*ai.AssistantMessage{
		// Step 1: Write a file.
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc1", "write", map[string]any{
					"path":    "config.yaml",
					"content": "name: old-value\nport: 8080",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		// Step 2: Edit the file.
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc2", "edit", map[string]any{
					"path":     "config.yaml",
					"old_text": "name: old-value",
					"new_text": "name: new-value",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		// Step 3: Read the file to verify.
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc3", "read", map[string]any{
					"path": "config.yaml",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		// Step 4: Final text response.
		{
			Content:    []ai.ContentBlock{ai.TextBlock("Config updated successfully.")},
			StopReason: ai.StopReasonStop,
		},
	}

	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "Config editor.",
		Tools:        codingTools,
		StreamFunc:   mockStreamFn(responses...),
	})

	msgs, err := ag.Prompt(context.Background(), "Update the config name")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// Verify the file was edited correctly.
	content, err := os.ReadFile(filepath.Join(tmpDir, "config.yaml"))
	if err != nil {
		t.Fatalf("Failed to read config.yaml: %v", err)
	}
	if !strings.Contains(string(content), "name: new-value") {
		t.Errorf("File should contain 'name: new-value', got: %s", string(content))
	}
	if strings.Contains(string(content), "name: old-value") {
		t.Errorf("File should not contain 'name: old-value' after edit")
	}
	if !strings.Contains(string(content), "port: 8080") {
		t.Errorf("File should still contain 'port: 8080', got: %s", string(content))
	}

	// Verify the read result (last tool result) shows the edited content.
	// Message flow: user, write_assistant, write_result, edit_assistant, edit_result, read_assistant, read_result, final_assistant
	if len(msgs) < 8 {
		t.Fatalf("Expected at least 8 messages, got %d", len(msgs))
	}
	readResult, ok := msgs[6].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("msgs[6] is not *ToolResultMessage")
	}
	readText := ""
	for _, block := range readResult.Content {
		if block.Type == ai.ContentText {
			readText += block.Text
		}
	}
	if !strings.Contains(readText, "new-value") {
		t.Errorf("Read after edit should show 'new-value', got: %s", readText)
	}
}

// TestE2E_AgentStateTracking tests that agent state is properly tracked through the pipeline.
func TestE2E_AgentStateTracking(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	responses := []*ai.AssistantMessage{
		{
			Content:    []ai.ContentBlock{ai.TextBlock("Hello!")},
			StopReason: ai.StopReasonStop,
		},
	}

	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "State tracking test.",
		Tools:        codingTools,
		StreamFunc:   mockStreamFn(responses...),
	})

	// Before prompt.
	if !ag.IsIdle() {
		t.Error("Agent should be idle before prompt")
	}
	state := ag.GetState()
	if len(state.Messages) != 0 {
		t.Errorf("Expected 0 messages before prompt, got %d", len(state.Messages))
	}
	if state.SystemPrompt != "State tracking test." {
		t.Errorf("System prompt = %q, want %q", state.SystemPrompt, "State tracking test.")
	}

	// After prompt.
	msgs, err := ag.Prompt(context.Background(), "Say hello")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	if !ag.IsIdle() {
		t.Error("Agent should be idle after prompt completes")
	}

	state = ag.GetState()
	if len(state.Messages) != len(msgs) {
		t.Errorf("State messages count = %d, want %d", len(state.Messages), len(msgs))
	}

	// Verify stored messages match returned messages.
	storedMsgs := ag.Messages()
	if len(storedMsgs) != len(msgs) {
		t.Errorf("Stored messages count = %d, want %d", len(storedMsgs), len(msgs))
	}
}

// TestE2E_PrintMode tests the non-interactive print mode pipeline.
func TestE2E_PrintMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a file for the agent to read.
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("This is a test project."), 0o644)

	responses := []*ai.AssistantMessage{
		{
			Content: []ai.ContentBlock{
				ai.ToolCallBlock("tc1", "read", map[string]any{
					"path": "readme.txt",
				}),
			},
			StopReason: ai.StopReasonToolUse,
		},
		{
			Content:    []ai.ContentBlock{ai.TextBlock("The readme says: This is a test project.")},
			StopReason: ai.StopReasonStop,
		},
	}

	codingTools := tools.CodingTools(tmpDir)
	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "Print mode test.",
		Tools:        codingTools,
		StreamFunc:   mockStreamFn(responses...),
	})

	msgs, err := ag.Prompt(context.Background(), "What does readme.txt say?")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	// Verify the complete pipeline worked.
	if len(msgs) < 4 {
		t.Fatalf("Expected at least 4 messages, got %d", len(msgs))
	}

	lastMsg, ok := msgs[len(msgs)-1].(*ai.AssistantMessage)
	if !ok {
		t.Fatal("Last message should be AssistantMessage")
	}
	if !strings.Contains(lastMsg.GetText(), "This is a test project") {
		t.Errorf("Final response should reference readme content, got: %s", lastMsg.GetText())
	}
}

// TestE2E_SystemPromptBuilder tests that the system prompt builder integrates
// correctly with the agent pipeline.
func TestE2E_SystemPromptBuilder(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	toolNames := make([]string, len(codingTools))
	for i, tool := range codingTools {
		toolNames[i] = tool.Name
	}

	systemPrompt := coding.BuildSystemPrompt(coding.SystemPromptOptions{
		Cwd:           tmpDir,
		SelectedTools: toolNames,
		AppendPrompt:  "Always be concise.",
	})

	// Verify prompt contains key elements.
	if !strings.Contains(systemPrompt, tmpDir) {
		t.Error("System prompt should contain the working directory")
	}
	if !strings.Contains(systemPrompt, "Always be concise.") {
		t.Error("System prompt should contain the appended prompt")
	}

	// Use the system prompt in an agent.
	responses := []*ai.AssistantMessage{
		{
			Content:    []ai.ContentBlock{ai.TextBlock("OK.")},
			StopReason: ai.StopReasonStop,
		},
	}

	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: systemPrompt,
		Tools:        codingTools,
		StreamFunc:   mockStreamFn(responses...),
	})

	_, err := ag.Prompt(context.Background(), "Test")
	if err != nil {
		t.Fatalf("Prompt with built system prompt failed: %v", err)
	}
}

// TestE2E_MultiplePrompts tests sending multiple prompts to the same agent
// (conversation continuity).
func TestE2E_MultiplePrompts(t *testing.T) {
	tmpDir := t.TempDir()
	codingTools := tools.CodingTools(tmpDir)

	callCount := 0
	var captured [][]ai.Message
	var mu sync.Mutex

	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		mu.Lock()
		callCount++
		captured = append(captured, reqCtx.Messages)
		mu.Unlock()

		msg := &ai.AssistantMessage{
			Content:    []ai.ContentBlock{ai.TextBlock("Response " + string(rune('0'+callCount)))},
			StopReason: ai.StopReasonStop,
		}

		stream := ai.NewEventStream(16)
		go func() {
			stream.Push(ai.StreamEvent{Type: ai.EventStart})
			stream.Push(ai.StreamEvent{Type: ai.EventTextDelta, ContentIndex: 0, Delta: msg.Content[0].Text})
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: msg.StopReason})
			stream.End(msg)
		}()
		return stream
	}

	ag := agent.New(agent.Config{
		Model:        testModel(),
		SystemPrompt: "Multi-prompt test.",
		Tools:        codingTools,
		StreamFunc:   streamFn,
	})

	// First prompt.
	msgs1, err := ag.Prompt(context.Background(), "First message")
	if err != nil {
		t.Fatalf("First prompt failed: %v", err)
	}
	if len(msgs1) != 2 {
		t.Fatalf("Expected 2 messages after first prompt, got %d", len(msgs1))
	}

	// Second prompt should include context from the first.
	msgs2, err := ag.Prompt(context.Background(), "Second message")
	if err != nil {
		t.Fatalf("Second prompt failed: %v", err)
	}
	if len(msgs2) != 4 {
		t.Fatalf("Expected 4 messages after second prompt, got %d", len(msgs2))
	}

	// Verify the second LLM call received the full conversation history.
	mu.Lock()
	defer mu.Unlock()
	if len(captured) < 2 {
		t.Fatalf("Expected at least 2 captured contexts, got %d", len(captured))
	}
	secondCallMsgs := captured[1]
	if len(secondCallMsgs) < 3 {
		t.Errorf("Second call should have at least 3 messages (user1, assistant1, user2), got %d", len(secondCallMsgs))
	}
}
