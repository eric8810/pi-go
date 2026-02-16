package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"agentsdk/ai"
)

func TestNew_InitialState(t *testing.T) {
	model := testModel()
	tools := []AgentTool{echoTool("t1")}
	cfg := Config{
		Model:         model,
		SystemPrompt:  "test prompt",
		Tools:         tools,
		ThinkingLevel: ai.ThinkingHigh,
		StreamFunc:    mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)

	state := a.GetState()
	if state.SystemPrompt != "test prompt" {
		t.Errorf("expected system prompt 'test prompt', got %q", state.SystemPrompt)
	}
	if state.Model != model {
		t.Error("expected model to match")
	}
	if state.ThinkingLevel != ai.ThinkingHigh {
		t.Errorf("expected ThinkingHigh, got %q", state.ThinkingLevel)
	}
	if len(state.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(state.Tools))
	}
	if state.Tools[0].Name != "t1" {
		t.Errorf("expected tool name 't1', got %q", state.Tools[0].Name)
	}
	if len(state.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(state.Messages))
	}
	if state.IsStreaming {
		t.Error("expected IsStreaming to be false initially")
	}
	if !a.IsIdle() {
		t.Error("expected agent to be idle initially")
	}
}

func TestNew_WiresSteeringAndFollowUp(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)

	// GetSteeringMessages and GetFollowUpMessages should be wired to drainSteering/drainFollowUps
	if a.config.GetSteeringMessages == nil {
		t.Error("expected GetSteeringMessages to be wired")
	}
	if a.config.GetFollowUpMessages == nil {
		t.Error("expected GetFollowUpMessages to be wired")
	}
}

func TestNew_DoesNotOverrideCustomSteering(t *testing.T) {
	customCalled := false
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
		GetSteeringMessages: func() []ai.Message {
			customCalled = true
			return nil
		},
	}

	a := New(cfg)
	a.config.GetSteeringMessages()

	if !customCalled {
		t.Error("expected custom GetSteeringMessages to be preserved")
	}
}

func TestPrompt_SendsUserMessageAndReturns(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("Hello back")),
	}

	a := New(cfg)
	result, err := a.Prompt(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// user + assistant
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	user, ok := result[0].(*ai.UserMessage)
	if !ok {
		t.Fatalf("expected UserMessage at index 0, got %T", result[0])
	}
	if user.Content[0].Text != "Hello" {
		t.Errorf("expected 'Hello', got %q", user.Content[0].Text)
	}

	assistant, ok := result[1].(*ai.AssistantMessage)
	if !ok {
		t.Fatalf("expected AssistantMessage at index 1, got %T", result[1])
	}
	if assistant.GetText() != "Hello back" {
		t.Errorf("expected 'Hello back', got %q", assistant.GetText())
	}
}

func TestPrompt_UpdatesState(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("response")),
	}

	a := New(cfg)
	_, err := a.Prompt(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := a.GetState()
	if len(state.Messages) != 2 {
		t.Errorf("expected 2 messages in state, got %d", len(state.Messages))
	}
}

func TestPrompt_AccumulatesMessages(t *testing.T) {
	callIdx := 0
	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		callIdx++
		var msg *ai.AssistantMessage
		if callIdx == 1 {
			msg = textResponse("First reply")
		} else {
			msg = textResponse("Second reply")
		}
		stream := ai.NewEventStream(16)
		go func() {
			stream.Push(ai.StreamEvent{Type: ai.EventStart})
			stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: msg, Reason: msg.StopReason})
			stream.End(msg)
		}()
		return stream
	}

	cfg := Config{
		Model:      testModel(),
		StreamFunc: streamFn,
	}

	a := New(cfg)

	_, err := a.Prompt(context.Background(), "First")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := a.Prompt(context.Background(), "Second")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// first_user + first_assistant + second_user + second_assistant
	if len(result) != 4 {
		t.Fatalf("expected 4 messages after 2 prompts, got %d", len(result))
	}
}

func TestGetState_ReturnsSnapshot(t *testing.T) {
	cfg := Config{
		Model:        testModel(),
		SystemPrompt: "original",
		Tools:        []AgentTool{echoTool("t1")},
		StreamFunc:   mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)
	state1 := a.GetState()

	// Modify via SetSystemPrompt
	a.SetSystemPrompt("modified")

	state2 := a.GetState()

	if state1.SystemPrompt != "original" {
		t.Error("snapshot should not change after modification")
	}
	if state2.SystemPrompt != "modified" {
		t.Error("new snapshot should reflect modification")
	}
}

func TestSetModel(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)
	newModel := &ai.Model{ID: "new-model", Name: "New Model", API: ai.APIOpenAICompletions, Provider: ai.ProviderOpenAI}
	a.SetModel(newModel)

	state := a.GetState()
	if state.Model != newModel {
		t.Error("expected model to be updated")
	}
	if state.Model.ID != "new-model" {
		t.Errorf("expected 'new-model', got %q", state.Model.ID)
	}
}

func TestSetThinkingLevel(t *testing.T) {
	cfg := Config{
		Model:         testModel(),
		ThinkingLevel: ai.ThinkingOff,
		StreamFunc:    mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)
	a.SetThinkingLevel(ai.ThinkingMedium)

	state := a.GetState()
	if state.ThinkingLevel != ai.ThinkingMedium {
		t.Errorf("expected ThinkingMedium, got %q", state.ThinkingLevel)
	}
}

func TestSetTools(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		Tools:      []AgentTool{echoTool("old")},
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)
	newTools := []AgentTool{echoTool("new1"), echoTool("new2")}
	a.SetTools(newTools)

	state := a.GetState()
	if len(state.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(state.Tools))
	}
	if state.Tools[0].Name != "new1" {
		t.Errorf("expected 'new1', got %q", state.Tools[0].Name)
	}
	if state.Tools[1].Name != "new2" {
		t.Errorf("expected 'new2', got %q", state.Tools[1].Name)
	}
}

func TestSetSystemPrompt(t *testing.T) {
	cfg := Config{
		Model:        testModel(),
		SystemPrompt: "initial",
		StreamFunc:   mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)
	a.SetSystemPrompt("updated prompt")

	state := a.GetState()
	if state.SystemPrompt != "updated prompt" {
		t.Errorf("expected 'updated prompt', got %q", state.SystemPrompt)
	}
}

func TestSubscribe_ReceivesEvents(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)

	var events []Event
	var mu sync.Mutex
	a.Subscribe(func(event Event) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	_, err := a.Prompt(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(events) == 0 {
		t.Fatal("expected to receive events")
	}

	// Verify agent_start and agent_end are present
	hasStart := false
	hasEnd := false
	for _, e := range events {
		if e.Type == EventAgentStart {
			hasStart = true
		}
		if e.Type == EventAgentEnd {
			hasEnd = true
		}
	}
	if !hasStart {
		t.Error("missing agent_start event")
	}
	if !hasEnd {
		t.Error("missing agent_end event")
	}
}

func TestSubscribe_Unsubscribe(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok"), textResponse("ok")),
	}

	a := New(cfg)

	var events1 []Event
	var mu1 sync.Mutex
	unsub := a.Subscribe(func(event Event) {
		mu1.Lock()
		events1 = append(events1, event)
		mu1.Unlock()
	})

	_, err := a.Prompt(context.Background(), "first")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu1.Lock()
	count1 := len(events1)
	mu1.Unlock()

	if count1 == 0 {
		t.Fatal("expected events before unsubscribe")
	}

	// Unsubscribe
	unsub()

	_, err = a.Prompt(context.Background(), "second")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu1.Lock()
	count2 := len(events1)
	mu1.Unlock()

	// After unsubscribe, no new events should be added
	if count2 != count1 {
		t.Errorf("expected no new events after unsubscribe, got %d (was %d)", count2, count1)
	}
}

func TestSubscribe_MultipleSubscribers(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)

	var events1, events2 []EventType
	var mu1, mu2 sync.Mutex

	a.Subscribe(func(event Event) {
		mu1.Lock()
		events1 = append(events1, event.Type)
		mu1.Unlock()
	})
	a.Subscribe(func(event Event) {
		mu2.Lock()
		events2 = append(events2, event.Type)
		mu2.Unlock()
	})

	_, err := a.Prompt(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu1.Lock()
	mu2.Lock()
	defer mu1.Unlock()
	defer mu2.Unlock()

	if len(events1) == 0 {
		t.Error("subscriber 1 received no events")
	}
	if len(events2) == 0 {
		t.Error("subscriber 2 received no events")
	}
	if len(events1) != len(events2) {
		t.Errorf("subscribers received different number of events: %d vs %d", len(events1), len(events2))
	}
}

func TestSteer_QueuesSteering(t *testing.T) {
	tool := echoTool("echo")

	// Tool call triggers steering drain
	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "x"})),
		textResponse("after steer"),
	)

	cfg := Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
	}

	a := New(cfg)

	// Queue a steering message before prompting
	steerMsg := ai.NewUserMessage("steer this way")
	a.Steer(steerMsg)

	result, err := a.Prompt(context.Background(), "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify steering message was injected
	found := false
	for _, msg := range result {
		if user, ok := msg.(*ai.UserMessage); ok {
			if len(user.Content) > 0 && user.Content[0].Text == "steer this way" {
				found = true
			}
		}
	}
	if !found {
		t.Error("steering message not found in result")
	}
}

func TestSteerText(t *testing.T) {
	tool := echoTool("echo")

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "x"})),
		textResponse("done"),
	)

	cfg := Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
	}

	a := New(cfg)
	a.SteerText("be brief")

	result, err := a.Prompt(context.Background(), "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, msg := range result {
		if user, ok := msg.(*ai.UserMessage); ok {
			if len(user.Content) > 0 && user.Content[0].Text == "be brief" {
				found = true
			}
		}
	}
	if !found {
		t.Error("SteerText message not found in result")
	}
}

func TestFollowUp_QueuesFollowUp(t *testing.T) {
	streamFn := mockStreamFn(
		textResponse("response 1"),
		textResponse("response 2"),
	)

	cfg := Config{
		Model:      testModel(),
		StreamFunc: streamFn,
	}

	a := New(cfg)
	a.FollowUp(ai.NewUserMessage("follow up!"))

	result, err := a.Prompt(context.Background(), "initial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// initial_user + resp1 + follow_up_user + resp2
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	followUpUser, ok := result[2].(*ai.UserMessage)
	if !ok {
		t.Fatalf("expected UserMessage at index 2, got %T", result[2])
	}
	if followUpUser.Content[0].Text != "follow up!" {
		t.Errorf("expected 'follow up!', got %q", followUpUser.Content[0].Text)
	}
}

func TestFollowUpText(t *testing.T) {
	streamFn := mockStreamFn(
		textResponse("first"),
		textResponse("second"),
	)

	cfg := Config{
		Model:      testModel(),
		StreamFunc: streamFn,
	}

	a := New(cfg)
	a.FollowUpText("continue please")

	result, err := a.Prompt(context.Background(), "start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, msg := range result {
		if user, ok := msg.(*ai.UserMessage); ok {
			if len(user.Content) > 0 && user.Content[0].Text == "continue please" {
				found = true
			}
		}
	}
	if !found {
		t.Error("FollowUpText message not found in result")
	}
}

func TestAbort(t *testing.T) {
	// Create a tool that blocks until we abort
	blockCh := make(chan struct{})
	blockingTool := AgentTool{
		Tool: ai.Tool{Name: "block", Description: "blocks"},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			select {
			case <-ctx.Done():
				return TextResult("aborted"), ctx.Err()
			case <-blockCh:
				return TextResult("done"), nil
			}
		},
	}

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "block", nil)),
		textResponse("after"),
	)

	cfg := Config{
		Model:      testModel(),
		Tools:      []AgentTool{blockingTool},
		StreamFunc: streamFn,
	}

	a := New(cfg)

	done := make(chan struct{})
	go func() {
		a.Prompt(context.Background(), "test")
		close(done)
	}()

	// Wait a bit for prompt to start, then abort
	time.Sleep(50 * time.Millisecond)
	a.Abort()

	select {
	case <-done:
		// ok - prompt returned
	case <-time.After(5 * time.Second):
		t.Fatal("Prompt did not return after Abort")
	}
}

func TestIsIdle(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)

	if !a.IsIdle() {
		t.Error("expected idle before Prompt")
	}

	_, err := a.Prompt(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !a.IsIdle() {
		t.Error("expected idle after Prompt completes")
	}
}

func TestIsIdle_FalseDuringExecution(t *testing.T) {
	checkCh := make(chan struct{})
	var wasIdleDuringExec bool
	var mu sync.Mutex

	checkTool := AgentTool{
		Tool: ai.Tool{Name: "check", Description: "checks idle state"},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			close(checkCh)
			// Give the test goroutine time to check IsIdle
			time.Sleep(50 * time.Millisecond)
			return TextResult("ok"), nil
		},
	}

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "check", nil)),
		textResponse("done"),
	)

	cfg := Config{
		Model:      testModel(),
		Tools:      []AgentTool{checkTool},
		StreamFunc: streamFn,
	}

	a := New(cfg)

	go func() {
		<-checkCh
		mu.Lock()
		wasIdleDuringExec = a.IsIdle()
		mu.Unlock()
	}()

	_, err := a.Prompt(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	if wasIdleDuringExec {
		t.Error("expected IsIdle=false during execution")
	}
	mu.Unlock()

	// After prompt completes, should be idle
	if !a.IsIdle() {
		t.Error("expected idle after Prompt completes")
	}
}

func TestWaitForIdle_AlreadyIdle(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)

	// Should return immediately when already idle
	done := make(chan struct{})
	go func() {
		a.WaitForIdle()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(time.Second):
		t.Fatal("WaitForIdle blocked even though agent is already idle")
	}
}

func TestWaitForIdle_BlocksUntilDone(t *testing.T) {
	blockCh := make(chan struct{})
	blockingTool := AgentTool{
		Tool: ai.Tool{Name: "block", Description: "blocks"},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			<-blockCh
			return TextResult("done"), nil
		},
	}

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "block", nil)),
		textResponse("done"),
	)

	cfg := Config{
		Model:      testModel(),
		Tools:      []AgentTool{blockingTool},
		StreamFunc: streamFn,
	}

	a := New(cfg)

	// Start prompt in background
	go func() {
		a.Prompt(context.Background(), "test")
	}()

	// Wait for agent to be not-idle (give it time to start)
	time.Sleep(50 * time.Millisecond)

	// WaitForIdle should block
	waitDone := make(chan struct{})
	go func() {
		a.WaitForIdle()
		close(waitDone)
	}()

	// Should not be done yet
	select {
	case <-waitDone:
		t.Fatal("WaitForIdle returned before agent finished")
	case <-time.After(50 * time.Millisecond):
		// expected - still blocking
	}

	// Unblock the tool
	close(blockCh)

	// Now WaitForIdle should return
	select {
	case <-waitDone:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForIdle did not return after agent finished")
	}
}

func TestMessages_ReturnsCopy(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("hello")),
	}

	a := New(cfg)
	_, err := a.Prompt(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs1 := a.Messages()
	msgs2 := a.Messages()

	if len(msgs1) != len(msgs2) {
		t.Fatalf("expected same length, got %d and %d", len(msgs1), len(msgs2))
	}
	if len(msgs1) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs1))
	}

	// Modifying one should not affect the other (they are copies of slices)
	msgs1[0] = nil
	if msgs2[0] == nil {
		t.Error("modifying msgs1 should not affect msgs2")
	}
}

func TestSteeringMode_OneAtATime(t *testing.T) {
	tool := echoTool("echo")

	// The agent will receive 3 tool calls across multiple turns.
	// We queue 2 steering messages. With one-at-a-time, each tool call boundary
	// should deliver only one steering message.
	streamFn := mockStreamFn(
		// Turn 1: tool call -> steering1 injected, tool_b skipped
		toolCallResponse(
			ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "a"}),
		),
		// Turn 2 (after steering 1): tool call -> steering2 injected
		toolCallResponse(
			ai.ToolCallBlock("tc2", "echo", map[string]any{"input": "b"}),
		),
		// Turn 3 (after steering 2): no more steering, text response
		textResponse("all done"),
	)

	cfg := Config{
		Model:        testModel(),
		Tools:        []AgentTool{tool},
		StreamFunc:   streamFn,
		SteeringMode: SteeringOneAtATime,
	}

	a := New(cfg)
	a.SteerText("steer 1")
	a.SteerText("steer 2")

	result, err := a.Prompt(context.Background(), "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count steering messages delivered
	steerCount := 0
	for _, msg := range result {
		if user, ok := msg.(*ai.UserMessage); ok {
			for _, c := range user.Content {
				if c.Text == "steer 1" || c.Text == "steer 2" {
					steerCount++
				}
			}
		}
	}
	if steerCount != 2 {
		t.Errorf("expected 2 steering messages delivered, got %d", steerCount)
	}
}

func TestSteeringMode_All(t *testing.T) {
	tool := echoTool("echo")

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "a"})),
		textResponse("done"),
	)

	cfg := Config{
		Model:        testModel(),
		Tools:        []AgentTool{tool},
		StreamFunc:   streamFn,
		SteeringMode: SteeringAll,
	}

	a := New(cfg)
	a.SteerText("steer A")
	a.SteerText("steer B")

	result, err := a.Prompt(context.Background(), "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both steering messages should be delivered at once
	steerCount := 0
	for _, msg := range result {
		if user, ok := msg.(*ai.UserMessage); ok {
			for _, c := range user.Content {
				if c.Text == "steer A" || c.Text == "steer B" {
					steerCount++
				}
			}
		}
	}
	if steerCount != 2 {
		t.Errorf("expected 2 steering messages delivered, got %d", steerCount)
	}
}

func TestSteeringMode_Default_IsAll(t *testing.T) {
	// When SteeringMode is empty (default), it should behave like SteeringAll.
	tool := echoTool("echo")

	streamFn := mockStreamFn(
		toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "a"})),
		textResponse("done"),
	)

	cfg := Config{
		Model:      testModel(),
		Tools:      []AgentTool{tool},
		StreamFunc: streamFn,
		// SteeringMode not set - defaults to ""
	}

	a := New(cfg)
	a.SteerText("msg1")
	a.SteerText("msg2")

	result, err := a.Prompt(context.Background(), "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Default (SteeringAll) should deliver both at once
	steerCount := 0
	for _, msg := range result {
		if user, ok := msg.(*ai.UserMessage); ok {
			for _, c := range user.Content {
				if c.Text == "msg1" || c.Text == "msg2" {
					steerCount++
				}
			}
		}
	}
	if steerCount != 2 {
		t.Errorf("expected 2 steering messages, got %d", steerCount)
	}
}

func TestDrainSteering_EmptyQueue(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}
	a := New(cfg)
	msgs := a.drainSteering()
	if msgs != nil {
		t.Errorf("expected nil for empty steering queue, got %v", msgs)
	}
}

func TestDrainFollowUps_EmptyQueue(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}
	a := New(cfg)
	msgs := a.drainFollowUps()
	if msgs != nil {
		t.Errorf("expected nil for empty follow-up queue, got %v", msgs)
	}
}

func TestDrainSteering_OneAtATime_DrainsSingle(t *testing.T) {
	cfg := Config{
		Model:        testModel(),
		SteeringMode: SteeringOneAtATime,
		StreamFunc:   mockStreamFn(textResponse("ok")),
	}
	a := New(cfg)
	a.SteerText("a")
	a.SteerText("b")
	a.SteerText("c")

	msgs := a.drainSteering()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	msgs = a.drainSteering()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	msgs = a.drainSteering()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	msgs = a.drainSteering()
	if msgs != nil {
		t.Errorf("expected nil after draining all, got %v", msgs)
	}
}

func TestDrainSteering_All_DrainsEverything(t *testing.T) {
	cfg := Config{
		Model:        testModel(),
		SteeringMode: SteeringAll,
		StreamFunc:   mockStreamFn(textResponse("ok")),
	}
	a := New(cfg)
	a.SteerText("a")
	a.SteerText("b")
	a.SteerText("c")

	msgs := a.drainSteering()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	msgs = a.drainSteering()
	if msgs != nil {
		t.Errorf("expected nil after draining all, got %v", msgs)
	}
}

func TestDrainFollowUps_DrainsAll(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}
	a := New(cfg)
	a.FollowUpText("x")
	a.FollowUpText("y")

	msgs := a.drainFollowUps()
	if len(msgs) != 2 {
		t.Fatalf("expected 2, got %d", len(msgs))
	}

	msgs = a.drainFollowUps()
	if msgs != nil {
		t.Errorf("expected nil after drain, got %v", msgs)
	}
}

func TestEmit_SetsIsStreamingOnAgentStart(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)

	var wasStreamingDuringExec bool
	a.Subscribe(func(event Event) {
		if event.Type == EventMessageStart {
			state := a.GetState()
			wasStreamingDuringExec = state.IsStreaming
		}
	})

	_, err := a.Prompt(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wasStreamingDuringExec {
		t.Error("expected IsStreaming=true during execution")
	}

	state := a.GetState()
	if state.IsStreaming {
		t.Error("expected IsStreaming=false after completion")
	}
}

func TestPromptWithMessages(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("response")),
	}

	a := New(cfg)

	msgs := []ai.Message{
		ai.NewUserMessage("msg1"),
		ai.NewUserMessage("msg2"),
	}

	result, err := a.PromptWithMessages(context.Background(), msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 user messages + 1 assistant
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
}

func TestAbort_WhenNotRunning(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)
	// Should not panic
	a.Abort()
}

func TestGetState_MessagesAreCopied(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("hello")),
	}

	a := New(cfg)
	_, err := a.Prompt(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := a.GetState()
	originalLen := len(state.Messages)

	// Modify the returned slice
	state.Messages = append(state.Messages, ai.NewUserMessage("extra"))

	// Agent's internal state should be unchanged
	state2 := a.GetState()
	if len(state2.Messages) != originalLen {
		t.Errorf("modifying state snapshot affected agent internal state: %d vs %d", len(state2.Messages), originalLen)
	}
}

func TestGetState_ToolsAreCopied(t *testing.T) {
	cfg := Config{
		Model:      testModel(),
		Tools:      []AgentTool{echoTool("t1")},
		StreamFunc: mockStreamFn(textResponse("ok")),
	}

	a := New(cfg)
	state := a.GetState()
	originalLen := len(state.Tools)

	// Modify the returned slice
	state.Tools = append(state.Tools, echoTool("t2"))

	// Agent's internal state should be unchanged
	state2 := a.GetState()
	if len(state2.Tools) != originalLen {
		t.Errorf("modifying state snapshot affected agent internal state: %d vs %d", len(state2.Tools), originalLen)
	}
}
