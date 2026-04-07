package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"pi-go/ai"
)

func TestRunLoop_SimpleTextResponse(t *testing.T) {
	cfg := &Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("Hello world")),
	}

	messages := []ai.Message{ai.NewUserMessage("Hi")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have original user message + assistant response
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	assistant, ok := result[1].(*ai.AssistantMessage)
	if !ok {
		t.Fatalf("expected AssistantMessage, got %T", result[1])
	}
	if assistant.GetText() != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", assistant.GetText())
	}
	if assistant.StopReason != ai.StopReasonStop {
		t.Errorf("expected stop reason 'stop', got %q", assistant.StopReason)
	}
}

func TestRunLoop_ToolCallCycle(t *testing.T) {
	tool := echoTool("echo")

	// First call returns a tool use, second call returns text.
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "test"})),
		textResponse("Done"),
	)

	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
	}

	messages := []ai.Message{ai.NewUserMessage("call echo")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user + assistant(tool_call) + tool_result + assistant(text)
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	// Check tool result
	toolResult, ok := result[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage at index 2, got %T", result[2])
	}
	if toolResult.ToolCallID != "tc1" {
		t.Errorf("expected tool call ID 'tc1', got %q", toolResult.ToolCallID)
	}
	if toolResult.IsError {
		t.Error("expected tool result not to be an error")
	}
	if len(toolResult.Content) == 0 || toolResult.Content[0].Text != "echo: test" {
		t.Errorf("expected tool result text 'echo: test', got %v", toolResult.Content)
	}

	// Check final response
	final, ok := result[3].(*ai.AssistantMessage)
	if !ok {
		t.Fatalf("expected AssistantMessage at index 3, got %T", result[3])
	}
	if final.GetText() != "Done" {
		t.Errorf("expected 'Done', got %q", final.GetText())
	}
}

func TestRunLoop_MultipleToolCallsSequential(t *testing.T) {
	var executionOrder []string
	var mu sync.Mutex

	makeTool := func(name string) AgentTool {
		return AgentTool{
			Tool: ai.Tool{Name: name, Description: "test tool"},
			Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
				mu.Lock()
				executionOrder = append(executionOrder, name)
				mu.Unlock()
				return TextResult("result from " + name), nil
			},
		}
	}

	tool1 := makeTool("tool1")
	tool2 := makeTool("tool2")
	tool3 := makeTool("tool3")

	// One response with 3 tool calls, followed by text.
	streamFn := mockStreamFn(
		toolCallResponse(
			ai.ToolCallBlock("tc1", "tool1", nil),
			ai.ToolCallBlock("tc2", "tool2", nil),
			ai.ToolCallBlock("tc3", "tool3", nil),
		),
		textResponse("All done"),
	)

	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool1, tool2, tool3},
		StreamFunc: streamFn,
	}

	messages := []ai.Message{ai.NewUserMessage("run all tools")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user + assistant(3 tool calls) + 3 tool results + final assistant
	if len(result) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(result))
	}

	// Verify sequential execution order
	mu.Lock()
	defer mu.Unlock()
	if len(executionOrder) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(executionOrder))
	}
	expected := []string{"tool1", "tool2", "tool3"}
	for i, name := range expected {
		if executionOrder[i] != name {
			t.Errorf("expected execution order[%d] = %q, got %q", i, name, executionOrder[i])
		}
	}
}

func TestRunLoop_UnknownToolReturnsError(t *testing.T) {
	// No tools registered, but LLM tries to call one.
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "nonexistent", nil)),
		textResponse("ok"),
	)

	cfg := &Config{
		Model:      testModel(),
		Tools:      nil,
		StreamFunc: streamFn,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user + assistant(tool_call) + tool_result(error) + final assistant
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	toolResult, ok := result[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage at index 2, got %T", result[2])
	}
	if !toolResult.IsError {
		t.Error("expected tool result to be an error")
	}
	if len(toolResult.Content) == 0 || toolResult.Content[0].Text != "Unknown tool: nonexistent" {
		t.Errorf("unexpected error text: %v", toolResult.Content)
	}
}

func TestRunLoop_ToolExecutionError(t *testing.T) {
	tool := errorTool("failing_tool")

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "failing_tool", nil)),
		textResponse("recovered"),
	)

	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user + assistant(tool_call) + tool_result(error) + final assistant
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	toolResult, ok := result[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage at index 2, got %T", result[2])
	}
	if !toolResult.IsError {
		t.Error("expected tool result to be an error")
	}
}

func TestRunLoop_SteeringMessages(t *testing.T) {
	callCount := 0
	steeringCalled := false

	tool := echoTool("echo")

	// First: tool call, second: tool call (after steering), third: text
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "a"})),
		toolCallResponse(ai.ToolCallBlock("tc2", "echo", map[string]any{"input": "b"})),
		textResponse("Done with steering"),
	)

	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		GetSteeringMessages: func() []ai.Message {
			callCount++
			if callCount == 1 {
				steeringCalled = true
				return []ai.Message{ai.NewUserMessage("please be concise")}
			}
			return nil
		},
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !steeringCalled {
		t.Error("steering messages function was not called")
	}

	// Verify steering message was injected
	foundSteering := false
	for _, msg := range result {
		if user, ok := msg.(*ai.UserMessage); ok {
			if len(user.Content) > 0 && user.Content[0].Text == "please be concise" {
				foundSteering = true
			}
		}
	}
	if !foundSteering {
		t.Error("steering message not found in result messages")
	}
}

func TestRunLoop_FollowUpMessages(t *testing.T) {
	followUpCallCount := 0

	// First turn: text response; second turn (after follow-up): text response
	streamFn := mockStreamFn(
		textResponse("First response"),
		textResponse("Second response"),
	)

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		GetFollowUpMessages: func() []ai.Message {
			followUpCallCount++
			if followUpCallCount == 1 {
				return []ai.Message{ai.NewUserMessage("follow up question")}
			}
			return nil
		},
	}

	messages := []ai.Message{ai.NewUserMessage("initial")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user + assistant + follow_up user + assistant
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	// Check the follow-up message is present
	followUpUser, ok := result[2].(*ai.UserMessage)
	if !ok {
		t.Fatalf("expected UserMessage at index 2, got %T", result[2])
	}
	if followUpUser.Content[0].Text != "follow up question" {
		t.Errorf("expected follow-up text 'follow up question', got %q", followUpUser.Content[0].Text)
	}

	// Check the second assistant response
	final, ok := result[3].(*ai.AssistantMessage)
	if !ok {
		t.Fatalf("expected AssistantMessage at index 3, got %T", result[3])
	}
	if final.GetText() != "Second response" {
		t.Errorf("expected 'Second response', got %q", final.GetText())
	}
}

func TestRunLoop_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("should not appear")),
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(ctx, cfg, messages)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRunLoop_ContextCancellationDuringToolExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	slowTool := AgentTool{
		Tool: ai.Tool{Name: "slow", Description: "slow tool"},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			cancel() // Cancel during tool execution
			return TextResult("done"), nil
		},
	}

	streamFn := mockStreamFn(
		toolCallResponse(
			ai.ToolCallBlock("tc1", "slow", nil),
			ai.ToolCallBlock("tc2", "slow", nil), // Should not execute
		),
		textResponse("should not reach"),
	)

	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{slowTool},
		StreamFunc: streamFn,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(ctx, cfg, messages)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestRunLoop_MaxTurns(t *testing.T) {
	// Each follow-up causes a new turn. Set MaxTurns = 2.
	followUpCount := 0
	streamFn := mockStreamFn(
		textResponse("turn 1"),
		textResponse("turn 2"),
		textResponse("turn 3 - should not reach"),
	)

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		MaxTurns:   2,
		GetFollowUpMessages: func() []ai.Message {
			followUpCount++
			// Always provide a follow-up to test MaxTurns
			return []ai.Message{ai.NewUserMessage(fmt.Sprintf("follow up %d", followUpCount))}
		},
	}

	messages := []ai.Message{ai.NewUserMessage("start")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we stopped at 2 turns. Each turn adds assistant response.
	// Count assistant messages.
	assistantCount := 0
	for _, msg := range result {
		if _, ok := msg.(*ai.AssistantMessage); ok {
			assistantCount++
		}
	}
	if assistantCount != 2 {
		t.Errorf("expected 2 assistant messages (max turns = 2), got %d", assistantCount)
	}
}

func TestRunLoop_ConvertToLLM(t *testing.T) {
	convertCalled := false

	streamFn := mockStreamFn(textResponse("ok"))

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		ConvertToLLM: func(messages []ai.Message) []ai.Message {
			convertCalled = true
			// Transform messages (add a marker)
			return messages
		},
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !convertCalled {
		t.Error("ConvertToLLM was not called")
	}
}

func TestRunLoop_TransformContext(t *testing.T) {
	transformCalled := false

	streamFn := mockStreamFn(textResponse("ok"))

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		TransformContext: func(ctx context.Context, messages []ai.Message) ([]ai.Message, error) {
			transformCalled = true
			return messages, nil
		},
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !transformCalled {
		t.Error("TransformContext was not called")
	}
}

func TestRunLoop_TransformContextCalledBeforeConvertToLLM(t *testing.T) {
	// TransformContext is called AFTER ConvertToLLM in the source code,
	// so let's verify the order: ConvertToLLM first, then TransformContext.
	var order []string
	var mu sync.Mutex

	streamFn := mockStreamFn(textResponse("ok"))

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		ConvertToLLM: func(messages []ai.Message) []ai.Message {
			mu.Lock()
			order = append(order, "convert")
			mu.Unlock()
			return messages
		},
		TransformContext: func(ctx context.Context, messages []ai.Message) ([]ai.Message, error) {
			mu.Lock()
			order = append(order, "transform")
			mu.Unlock()
			return messages, nil
		},
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(order))
	}
	if order[0] != "convert" || order[1] != "transform" {
		t.Errorf("expected order [convert, transform], got %v", order)
	}
}

func TestRunLoop_TransformContextError(t *testing.T) {
	streamFn := mockStreamFn(textResponse("should not reach"))

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		TransformContext: func(ctx context.Context, messages []ai.Message) ([]ai.Message, error) {
			return nil, fmt.Errorf("transform failed")
		},
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err == nil {
		t.Fatal("expected error from TransformContext")
	}
	if err.Error() != "transform context: transform failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunLoop_EventOrder(t *testing.T) {
	tool := echoTool("echo")

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "hi"})),
		textResponse("Done"),
	)

	var events []Event
	handler := collectEvents(&events)

	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		OnEvent:    handler,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Extract event types in order
	var eventTypes []EventType
	for _, e := range events {
		eventTypes = append(eventTypes, e.Type)
	}

	// Expected order for tool call cycle:
	// agent_start,
	// turn_start, message_start, message_update*(at least 1), message_end,
	// tool_execution_start, tool_execution_end,
	// turn_end,
	// turn_start, message_start, message_update*(at least 1), message_end,
	// turn_end,
	// agent_end

	expectedPrefix := []EventType{
		EventAgentStart,
		EventTurnStart,
		EventMessageStart,
	}
	for i, exp := range expectedPrefix {
		if i >= len(eventTypes) {
			t.Fatalf("event sequence too short, expected at least %d events", i+1)
		}
		if eventTypes[i] != exp {
			t.Errorf("event[%d]: expected %q, got %q", i, exp, eventTypes[i])
		}
	}

	// Verify agent_end is the last event
	if eventTypes[len(eventTypes)-1] != EventAgentEnd {
		t.Errorf("expected last event to be agent_end, got %q", eventTypes[len(eventTypes)-1])
	}

	// Verify all expected event types are present
	expectedEvents := map[EventType]bool{
		EventAgentStart:    false,
		EventAgentEnd:      false,
		EventTurnStart:     false,
		EventTurnEnd:       false,
		EventMessageStart:  false,
		EventMessageUpdate: false,
		EventMessageEnd:    false,
		EventToolExecStart: false,
		EventToolExecEnd:   false,
	}
	for _, et := range eventTypes {
		if _, ok := expectedEvents[et]; ok {
			expectedEvents[et] = true
		}
	}
	for et, found := range expectedEvents {
		if !found {
			t.Errorf("expected event type %q not found in events", et)
		}
	}

	// Verify tool_execution_start comes before tool_execution_end
	startIdx := -1
	endIdx := -1
	for i, et := range eventTypes {
		if et == EventToolExecStart && startIdx == -1 {
			startIdx = i
		}
		if et == EventToolExecEnd && endIdx == -1 {
			endIdx = i
		}
	}
	if startIdx >= endIdx {
		t.Error("tool_execution_start must come before tool_execution_end")
	}

	// Verify message_start comes before message_end
	msgStartIdx := -1
	msgEndIdx := -1
	for i, et := range eventTypes {
		if et == EventMessageStart && msgStartIdx == -1 {
			msgStartIdx = i
		}
		if et == EventMessageEnd && msgEndIdx == -1 {
			msgEndIdx = i
		}
	}
	if msgStartIdx >= msgEndIdx {
		t.Error("message_start must come before message_end")
	}
}

func TestRunLoop_EventOrderSimple(t *testing.T) {
	// Simple text-only response - no tool calls.
	streamFn := mockStreamFn(textResponse("hello"))

	var events []Event
	handler := collectEvents(&events)

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		OnEvent:    handler,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var eventTypes []EventType
	for _, e := range events {
		eventTypes = append(eventTypes, e.Type)
	}

	// For simple text response:
	// agent_start, turn_start, message_start, message_update+, message_end, turn_end, agent_end
	expected := []EventType{
		EventAgentStart,
		EventTurnStart,
		EventMessageStart,
	}
	for i, exp := range expected {
		if i >= len(eventTypes) {
			t.Fatalf("too few events")
		}
		if eventTypes[i] != exp {
			t.Errorf("event[%d]: expected %q, got %q", i, exp, eventTypes[i])
		}
	}

	// Should end with turn_end, agent_end
	n := len(eventTypes)
	if n < 2 {
		t.Fatal("too few events")
	}
	if eventTypes[n-2] != EventTurnEnd {
		t.Errorf("expected second-to-last event to be turn_end, got %q", eventTypes[n-2])
	}
	if eventTypes[n-1] != EventAgentEnd {
		t.Errorf("expected last event to be agent_end, got %q", eventTypes[n-1])
	}
}

func TestRunLoop_ToolExecStartContainsToolInfo(t *testing.T) {
	tool := echoTool("my_tool")
	args := map[string]any{"input": "hello"}

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc123", "my_tool", args)),
		textResponse("done"),
	)

	var events []Event
	handler := collectEvents(&events)

	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		OnEvent:    handler,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find tool exec start event
	var toolExecStart *Event
	for i := range events {
		if events[i].Type == EventToolExecStart {
			toolExecStart = &events[i]
			break
		}
	}
	if toolExecStart == nil {
		t.Fatal("no tool_execution_start event found")
	}
	if toolExecStart.ToolCallID != "tc123" {
		t.Errorf("expected ToolCallID 'tc123', got %q", toolExecStart.ToolCallID)
	}
	if toolExecStart.ToolName != "my_tool" {
		t.Errorf("expected ToolName 'my_tool', got %q", toolExecStart.ToolName)
	}
}

func TestRunLoop_ToolExecEndContainsResult(t *testing.T) {
	tool := echoTool("my_tool")

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "my_tool", map[string]any{"input": "test"})),
		textResponse("done"),
	)

	var events []Event
	handler := collectEvents(&events)

	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		OnEvent:    handler,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var toolExecEnd *Event
	for i := range events {
		if events[i].Type == EventToolExecEnd {
			toolExecEnd = &events[i]
			break
		}
	}
	if toolExecEnd == nil {
		t.Fatal("no tool_execution_end event found")
	}
	if toolExecEnd.ToolResult == nil {
		t.Fatal("tool_execution_end has nil ToolResult")
	}
	if toolExecEnd.IsError {
		t.Error("expected IsError to be false")
	}
}

func TestRunLoop_NilOnEvent(t *testing.T) {
	// Should not panic when OnEvent is nil.
	cfg := &Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunLoop_PartialMessageFieldsPopulated(t *testing.T) {
	model := testModel()

	streamFn := mockStreamFn(textResponse("ok"))

	var messageStartEvent *Event
	var events []Event
	handler := collectEvents(&events)

	cfg := &Config{
		Model:      model,
		StreamFunc: streamFn,
		OnEvent:    handler,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := range events {
		if events[i].Type == EventMessageStart {
			messageStartEvent = &events[i]
			break
		}
	}
	if messageStartEvent == nil {
		t.Fatal("no message_start event found")
	}

	// Check partial message has model info
	partial, ok := messageStartEvent.Message.(*ai.AssistantMessage)
	if !ok {
		t.Fatalf("expected AssistantMessage in message_start, got %T", messageStartEvent.Message)
	}
	if partial.Model != model.ID {
		t.Errorf("expected model %q, got %q", model.ID, partial.Model)
	}
	if partial.API != model.API {
		t.Errorf("expected API %q, got %q", model.API, partial.API)
	}
}

func TestRunLoop_StreamPassesSystemPromptAndTools(t *testing.T) {
	var capturedCtx *ai.Context

	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		capturedCtx = reqCtx
		msg := textResponse("ok")
		stream := ai.NewEventStream(16)
		go func() {
			stream.Push(ai.StreamEvent{Type: ai.EventStart})
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: ai.StopReasonStop})
			stream.End(msg)
		}()
		return stream
	}

	tool := echoTool("mytool")

	cfg := &Config{
		Model:        testModel(),
		SystemPrompt: "You are a helpful assistant.",
		Tools:        []AgentTool{tool},
		StreamFunc:   streamFn,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCtx == nil {
		t.Fatal("stream function was not called")
	}
	if capturedCtx.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("expected system prompt, got %q", capturedCtx.SystemPrompt)
	}
	if len(capturedCtx.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(capturedCtx.Tools))
	}
	if capturedCtx.Tools[0].Name != "mytool" {
		t.Errorf("expected tool name 'mytool', got %q", capturedCtx.Tools[0].Name)
	}
}

func TestRunLoop_SteeringBreaksRemainingTools(t *testing.T) {
	// When steering messages are returned after a tool call, remaining tool calls
	// should be skipped.
	var executed []string
	var mu sync.Mutex

	makeTool := func(name string) AgentTool {
		return AgentTool{
			Tool: ai.Tool{Name: name},
			Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
				mu.Lock()
				executed = append(executed, name)
				mu.Unlock()
				return TextResult("ok"), nil
			},
		}
	}

	steerCallCount := 0
	cfg := &Config{
		Model: testModel(),
		Tools: []AgentTool{makeTool("tool_a"), makeTool("tool_b")},
		StreamFunc: mockStreamFn(
			toolCallResponse(
				ai.ToolCallBlock("tc1", "tool_a", nil),
				ai.ToolCallBlock("tc2", "tool_b", nil),
			),
			textResponse("after steering"),
		),
		GetSteeringMessages: func() []ai.Message {
			steerCallCount++
			if steerCallCount == 1 {
				return []ai.Message{ai.NewUserMessage("steer!")}
			}
			return nil
		},
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	// Only tool_a should have executed before steering broke the loop
	if len(executed) != 1 || executed[0] != "tool_a" {
		t.Errorf("expected only tool_a executed, got %v", executed)
	}
}

func TestRunLoop_MaxTurnsZeroMeansUnlimited(t *testing.T) {
	// MaxTurns = 0 should be unlimited. We'll use follow-ups with a counter to limit ourselves.
	followUpCount := 0
	streamFn := mockStreamFn(
		textResponse("turn 1"),
		textResponse("turn 2"),
		textResponse("turn 3"),
	)

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		MaxTurns:   0, // unlimited
		GetFollowUpMessages: func() []ai.Message {
			followUpCount++
			if followUpCount <= 2 {
				return []ai.Message{ai.NewUserMessage("more")}
			}
			return nil
		},
	}

	messages := []ai.Message{ai.NewUserMessage("start")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 3 turns should have completed
	assistantCount := 0
	for _, msg := range result {
		if _, ok := msg.(*ai.AssistantMessage); ok {
			assistantCount++
		}
	}
	if assistantCount != 3 {
		t.Errorf("expected 3 assistant messages (unlimited turns), got %d", assistantCount)
	}
}

func TestRunLoop_ToolUpdateCallback(t *testing.T) {
	updateTool := AgentTool{
		Tool: ai.Tool{Name: "progress", Description: "progress tool"},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			onUpdate(ToolUpdate{Output: "50% done"})
			onUpdate(ToolUpdate{Output: "100% done"})
			return TextResult("complete"), nil
		},
	}

	var events []Event
	handler := collectEvents(&events)

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "progress", nil)),
		textResponse("done"),
	)

	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{updateTool},
		StreamFunc: streamFn,
		OnEvent:    handler,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for tool_execution_update events
	updateCount := 0
	for _, e := range events {
		if e.Type == EventToolExecUpdate {
			updateCount++
		}
	}
	if updateCount != 2 {
		t.Errorf("expected 2 tool_execution_update events, got %d", updateCount)
	}
}

func TestRunLoop_StreamNilResult(t *testing.T) {
	// If the stream returns nil result, should use partial and set error.
	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		stream := ai.NewEventStream(16)
		go func() {
			stream.Push(ai.StreamEvent{Type: ai.EventStart})
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Reason: ai.StopReasonStop})
			stream.End(nil) // nil result
		}()
		return stream
	}

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)

	// When result is nil, the loop sets StopReasonError which returns an error
	if err == nil {
		t.Fatal("expected error when stream returns nil result")
	}
}

func TestRunLoop_MultipleFollowUps(t *testing.T) {
	// Test multiple rounds of follow-ups to verify outer loop works.
	followUpRound := 0
	streamFn := mockStreamFn(
		textResponse("response 1"),
		textResponse("response 2"),
		textResponse("response 3"),
	)

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		GetFollowUpMessages: func() []ai.Message {
			followUpRound++
			if followUpRound <= 2 {
				return []ai.Message{ai.NewUserMessage(fmt.Sprintf("round %d", followUpRound))}
			}
			return nil
		},
	}

	messages := []ai.Message{ai.NewUserMessage("start")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// start_user + resp1 + followup1 + resp2 + followup2 + resp3 = 6
	if len(result) != 6 {
		t.Errorf("expected 6 messages, got %d", len(result))
	}
}

func TestRunLoop_AgentEndContainsAllMessages(t *testing.T) {
	var events []Event
	handler := collectEvents(&events)

	cfg := &Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("hello")),
		OnEvent:    handler,
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	result, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find agent_end event
	var agentEnd *Event
	for i := range events {
		if events[i].Type == EventAgentEnd {
			agentEnd = &events[i]
		}
	}
	if agentEnd == nil {
		t.Fatal("no agent_end event found")
	}
	if len(agentEnd.Messages) != len(result) {
		t.Errorf("agent_end messages count %d != result count %d", len(agentEnd.Messages), len(result))
	}
}

func TestRunLoop_StreamOptionsPassedCorrectly(t *testing.T) {
	var capturedOpts ai.StreamOptions

	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		capturedOpts = opts
		msg := textResponse("ok")
		stream := ai.NewEventStream(16)
		go func() {
			stream.Push(ai.StreamEvent{Type: ai.EventStart})
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: ai.StopReasonStop})
			stream.End(msg)
		}()
		return stream
	}

	cfg := &Config{
		Model:         testModel(),
		StreamFunc:    streamFn,
		ThinkingLevel: ai.ThinkingHigh,
		APIKey:        "test-key-123",
	}

	messages := []ai.Message{ai.NewUserMessage("test")}
	_, err := RunLoop(context.Background(), cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedOpts.Thinking != ai.ThinkingHigh {
		t.Errorf("expected ThinkingHigh, got %q", capturedOpts.Thinking)
	}
	if capturedOpts.APIKey != "test-key-123" {
		t.Errorf("expected API key 'test-key-123', got %q", capturedOpts.APIKey)
	}
}

func TestRunLoop_EmptyToolCallsNoInfiniteLoop(t *testing.T) {
	// If assistant responds with stop but no tool calls multiple times,
	// should exit cleanly.
	cfg := &Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("done")),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages := []ai.Message{ai.NewUserMessage("test")}
	result, err := RunLoop(ctx, cfg, messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestFindTool(t *testing.T) {
	tools := []AgentTool{
		echoTool("alpha"),
		echoTool("beta"),
		echoTool("gamma"),
	}

	t.Run("found", func(t *testing.T) {
		tool := findTool(tools, "beta")
		if tool == nil {
			t.Fatal("expected to find tool 'beta'")
		}
		if tool.Name != "beta" {
			t.Errorf("expected 'beta', got %q", tool.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		tool := findTool(tools, "delta")
		if tool != nil {
			t.Error("expected nil for unknown tool")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		tool := findTool(nil, "any")
		if tool != nil {
			t.Error("expected nil for empty tool list")
		}
	})
}

func TestTextResult(t *testing.T) {
	r := TextResult("hello")
	if r.IsError {
		t.Error("expected IsError to be false")
	}
	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(r.Content))
	}
	if r.Content[0].Text != "hello" {
		t.Errorf("expected 'hello', got %q", r.Content[0].Text)
	}
}

func TestErrorResult(t *testing.T) {
	r := ErrorResult("something went wrong")
	if !r.IsError {
		t.Error("expected IsError to be true")
	}
	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(r.Content))
	}
	if r.Content[0].Text != "something went wrong" {
		t.Errorf("expected 'something went wrong', got %q", r.Content[0].Text)
	}
}

// ---------------------------------------------------------------------------
// P1: BeforeToolCall / AfterToolCall hook tests
// ---------------------------------------------------------------------------

func TestBeforeToolCall_Allow(t *testing.T) {
	var called []string
	tool := echoTool("echo")
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "hi"})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		BeforeToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any) (*BeforeToolCallResult, error) {
			called = append(called, toolName)
			return nil, nil // allow
		},
	}
	result, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(called) != 1 || called[0] != "echo" {
		t.Errorf("expected BeforeToolCall to be called once with 'echo', got %v", called)
	}
	tr, ok := result[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage at index 2, got %T", result[2])
	}
	if tr.IsError {
		t.Error("expected non-error tool result when hook allows")
	}
	if len(tr.Content) == 0 || tr.Content[0].Text != "echo: hi" {
		t.Errorf("expected 'echo: hi', got %v", tr.Content)
	}
}

func TestBeforeToolCall_Deny(t *testing.T) {
	tool := echoTool("echo")
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "hi"})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		BeforeToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any) (*BeforeToolCallResult, error) {
			return &BeforeToolCallResult{
				Action:     ToolCallDeny,
				DenyResult: ErrorResult("not allowed: " + toolName),
			}, nil
		},
	}
	result, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr, ok := result[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage at index 2, got %T", result[2])
	}
	if !tr.IsError {
		t.Error("expected IsError=true when hook denies")
	}
	if len(tr.Content) == 0 || tr.Content[0].Text != "not allowed: echo" {
		t.Errorf("expected denial message, got %v", tr.Content)
	}
}

func TestBeforeToolCall_DenyWithoutResult(t *testing.T) {
	// Deny with nil DenyResult should fall back to a generic message.
	tool := echoTool("echo")
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "x"})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		BeforeToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any) (*BeforeToolCallResult, error) {
			return &BeforeToolCallResult{Action: ToolCallDeny}, nil
		},
	}
	result, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr, ok := result[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage, got %T", result[2])
	}
	if !tr.IsError {
		t.Error("expected IsError=true for deny with nil DenyResult")
	}
}

func TestBeforeToolCall_ProvideResult(t *testing.T) {
	var executed bool
	tool := AgentTool{
		Tool: ai.Tool{Name: "probe"},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			executed = true
			return TextResult("real result"), nil
		},
	}
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "probe", map[string]any{})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		BeforeToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any) (*BeforeToolCallResult, error) {
			return &BeforeToolCallResult{
				Action:         ToolCallProvideResult,
				ProvidedResult: TextResult("injected result"),
			}, nil
		},
	}
	result, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executed {
		t.Error("expected tool.Execute NOT to be called when hook provides result")
	}
	tr, ok := result[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage, got %T", result[2])
	}
	if len(tr.Content) == 0 || tr.Content[0].Text != "injected result" {
		t.Errorf("expected 'injected result', got %v", tr.Content)
	}
}

func TestBeforeToolCall_ReplaceArgs(t *testing.T) {
	var capturedArgs map[string]any
	tool := AgentTool{
		Tool: ai.Tool{Name: "capture"},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			capturedArgs = params
			return TextResult("ok"), nil
		},
	}
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "capture", map[string]any{"input": "original"})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		BeforeToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any) (*BeforeToolCallResult, error) {
			return &BeforeToolCallResult{
				Action:      ToolCallAllow,
				ReplaceArgs: map[string]any{"input": "replaced"},
			}, nil
		},
	}
	_, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedArgs["input"] != "replaced" {
		t.Errorf("expected args to be replaced, got %v", capturedArgs)
	}
}

func TestAfterToolCall_TransformResult(t *testing.T) {
	tool := echoTool("echo")
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "hello"})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		AfterToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any, result *ToolResult) (*ToolResult, error) {
			// Wrap the result
			return TextResult("WRAPPED: " + result.Content[0].Text), nil
		},
	}
	messages, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr, ok := messages[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage, got %T", messages[2])
	}
	if len(tr.Content) == 0 || tr.Content[0].Text != "WRAPPED: echo: hello" {
		t.Errorf("expected 'WRAPPED: echo: hello', got %v", tr.Content)
	}
}

func TestAfterToolCall_ReturnNilKeepsOriginal(t *testing.T) {
	tool := echoTool("echo")
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "hi"})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		AfterToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any, result *ToolResult) (*ToolResult, error) {
			return nil, nil // no transform
		},
	}
	messages, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr, ok := messages[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage, got %T", messages[2])
	}
	if len(tr.Content) == 0 || tr.Content[0].Text != "echo: hi" {
		t.Errorf("expected original 'echo: hi', got %v", tr.Content)
	}
}

func TestPerToolBeforeExecute_Deny(t *testing.T) {
	var globalCalled bool
	tool := AgentTool{
		Tool: ai.Tool{Name: "locked"},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			return TextResult("should not execute"), nil
		},
		BeforeExecute: func(ctx context.Context, toolCallID string, args map[string]any) (*BeforeToolCallResult, error) {
			return &BeforeToolCallResult{
				Action:     ToolCallDeny,
				DenyResult: ErrorResult("per-tool denied"),
			}, nil
		},
	}
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "locked", map[string]any{})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		BeforeToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any) (*BeforeToolCallResult, error) {
			globalCalled = true
			return nil, nil // global allows
		},
	}
	messages, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !globalCalled {
		t.Error("expected global BeforeToolCall to be called before per-tool hook")
	}
	tr, ok := messages[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage, got %T", messages[2])
	}
	if !tr.IsError || len(tr.Content) == 0 || tr.Content[0].Text != "per-tool denied" {
		t.Errorf("expected per-tool denial, got %v (isError=%v)", tr.Content, tr.IsError)
	}
}

func TestPerToolAfterExecute_Transform(t *testing.T) {
	var globalAfterCalled bool
	tool := AgentTool{
		Tool: ai.Tool{Name: "transform"},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			return TextResult("raw"), nil
		},
		AfterExecute: func(ctx context.Context, toolCallID string, args map[string]any, result *ToolResult) (*ToolResult, error) {
			return TextResult("per-tool: " + result.Content[0].Text), nil
		},
	}
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "transform", map[string]any{})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		AfterToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any, result *ToolResult) (*ToolResult, error) {
			globalAfterCalled = true
			return TextResult("global: " + result.Content[0].Text), nil // runs after per-tool
		},
	}
	messages, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !globalAfterCalled {
		t.Error("expected global AfterToolCall to be called after per-tool hook")
	}
	tr, ok := messages[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage, got %T", messages[2])
	}
	// Order: Execute → AfterExecute → AfterToolCall → result
	// per-tool wraps "raw" → "per-tool: raw", then global wraps that → "global: per-tool: raw"
	expected := "global: per-tool: raw"
	if len(tr.Content) == 0 || tr.Content[0].Text != expected {
		t.Errorf("expected %q, got %v", expected, tr.Content)
	}
}

func TestBeforeToolCall_Error(t *testing.T) {
	tool := echoTool("echo")
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "x"})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		BeforeToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any) (*BeforeToolCallResult, error) {
			return nil, fmt.Errorf("hook failure")
		},
	}
	messages, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected loop error: %v", err)
	}
	tr, ok := messages[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage, got %T", messages[2])
	}
	if !tr.IsError {
		t.Error("expected IsError=true when BeforeToolCall returns error")
	}
}

func TestAfterToolCall_Error(t *testing.T) {
	tool := echoTool("echo")
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "x"})),
		textResponse("done"),
	)
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		AfterToolCall: func(ctx context.Context, toolCallID, toolName string, args map[string]any, result *ToolResult) (*ToolResult, error) {
			return nil, fmt.Errorf("after hook failure")
		},
	}
	messages, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected loop error: %v", err)
	}
	tr, ok := messages[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage, got %T", messages[2])
	}
	if !tr.IsError {
		t.Error("expected IsError=true when AfterToolCall returns error")
	}
}

// ---------------------------------------------------------------------------
// P1: Streaming error / abort edge cases
// ---------------------------------------------------------------------------

func TestRunLoop_StreamEventError(t *testing.T) {
	// Stream that emits an error event should surface as an LLM error.
	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		stream := ai.NewEventStream(16)
		go func() {
			errMsg := &ai.AssistantMessage{
				StopReason:   ai.StopReasonError,
				ErrorMessage: "upstream provider error",
			}
			stream.Push(ai.StreamEvent{Type: ai.EventError, Message: errMsg})
			stream.End(errMsg)
		}()
		return stream
	}
	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
	}
	_, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("hi")})
	if err == nil {
		t.Fatal("expected error when stream emits EventError, got nil")
	}
}

func TestRunLoop_ContextCancelDuringStream(t *testing.T) {
	// Context cancelled while streaming should return ctx.Err().
	ready := make(chan struct{})
	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		stream := ai.NewEventStream(16)
		go func() {
			close(ready) // signal we started
			<-ctx.Done() // wait for cancellation
			msg := &ai.AssistantMessage{StopReason: ai.StopReasonAborted}
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: ai.StopReasonAborted})
			stream.End(msg)
		}()
		return stream
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ready
		cancel()
	}()
	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
	}
	_, err := RunLoop(ctx, cfg, []ai.Message{ai.NewUserMessage("hi")})
	if err == nil {
		t.Fatal("expected error on context cancellation during stream")
	}
}
