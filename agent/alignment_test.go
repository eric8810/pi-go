// Package agent — behavioral alignment test suite.
//
// These tests verify that pi-go's agent loop behaves identically to the
// documented semantics of the original TypeScript pi-agent-core. Each test
// corresponds to a named behavioral contract. Tests use only mock streams so
// they run offline with no real LLM.
package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pi-go/ai"
)

// ---------------------------------------------------------------------------
// ALIGN-01: single text turn
// Agent receives a user message, LLM responds with text, loop exits.
// Expected history: [user, assistant]
// ---------------------------------------------------------------------------

func TestAlign_01_SingleTextTurn(t *testing.T) {
	cfg := &Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("Hello")),
	}
	msgs, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("Hi")})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected [user, assistant], got %d messages", len(msgs))
	}
	assertRole(t, msgs[0], ai.RoleUser, 0)
	assertRole(t, msgs[1], ai.RoleAssistant, 1)
	if msgs[1].(*ai.AssistantMessage).GetText() != "Hello" {
		t.Errorf("unexpected assistant text")
	}
}

// ---------------------------------------------------------------------------
// ALIGN-02: single tool call cycle
// LLM calls a tool, receives result, then responds with text.
// Expected history: [user, assistant(tool_call), tool_result, assistant(text)]
// ---------------------------------------------------------------------------

func TestAlign_02_SingleToolCallCycle(t *testing.T) {
	cfg := &Config{
		Model:  testModel(),
		Tools:  []AgentTool{echoTool("echo")},
		StreamFunc: mockStreamFn(
			toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "ping"})),
			textResponse("pong"),
		),
	}
	msgs, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	assertRole(t, msgs[0], ai.RoleUser, 0)
	assertRole(t, msgs[1], ai.RoleAssistant, 1)
	assertRole(t, msgs[2], ai.RoleToolResult, 2)
	assertRole(t, msgs[3], ai.RoleAssistant, 3)
}

// ---------------------------------------------------------------------------
// ALIGN-03: multi-step tool chain
// LLM calls 2 different tools in sequence (separate turns), then finishes.
// Expected history: [user, asst1, tr1, asst2, tr2, asst_final]
// ---------------------------------------------------------------------------

func TestAlign_03_MultiStepToolChain(t *testing.T) {
	cfg := &Config{
		Model:  testModel(),
		Tools:  []AgentTool{echoTool("a"), echoTool("b")},
		StreamFunc: mockStreamFn(
			toolCallResponse(ai.ToolCallBlock("tc1", "a", map[string]any{"input": "1"})),
			toolCallResponse(ai.ToolCallBlock("tc2", "b", map[string]any{"input": "2"})),
			textResponse("done"),
		),
	}
	msgs, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("chain")})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// [user, asst(tool_a), tool_result_a, asst(tool_b), tool_result_b, asst(done)]
	if len(msgs) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(msgs))
	}
}

// ---------------------------------------------------------------------------
// ALIGN-04: parallel tool calls in one turn
// LLM calls 2 tools in a single response; both are executed before next LLM call.
// Expected history: [user, asst(tool1+tool2), tr1, tr2, asst(text)]
// ---------------------------------------------------------------------------

func TestAlign_04_ParallelToolCallsInOneTurn(t *testing.T) {
	var order []string
	var mu sync.Mutex
	makeTool := func(name string) AgentTool {
		return AgentTool{
			Tool: ai.Tool{Name: name},
			Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
				mu.Lock()
				order = append(order, name)
				mu.Unlock()
				return TextResult(name + "-result"), nil
			},
		}
	}

	cfg := &Config{
		Model:  testModel(),
		Tools:  []AgentTool{makeTool("x"), makeTool("y")},
		StreamFunc: mockStreamFn(
			toolCallResponse(
				ai.ToolCallBlock("tc1", "x", map[string]any{}),
				ai.ToolCallBlock("tc2", "y", map[string]any{}),
			),
			textResponse("both done"),
		),
	}
	msgs, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("run both")})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// [user, asst, tr_x, tr_y, asst_final] = 5
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}
	if len(order) != 2 {
		t.Errorf("expected both tools to run, got %v", order)
	}
	// Go runs them sequentially (not truly parallel), but both must execute
}

// ---------------------------------------------------------------------------
// ALIGN-05: steer interrupts tool sequence
// Steering injected after first tool in a two-tool response skips the second.
// ---------------------------------------------------------------------------

func TestAlign_05_SteerInterruptsToolSequence(t *testing.T) {
	var executed []string
	makeTool := func(name string) AgentTool {
		return AgentTool{
			Tool: ai.Tool{Name: name},
			Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
				executed = append(executed, name)
				return TextResult(name), nil
			},
		}
	}

	streamFn := mockStreamFn(
		toolCallResponse(
			ai.ToolCallBlock("tc1", "first", map[string]any{}),
			ai.ToolCallBlock("tc2", "second", map[string]any{}),
		),
		textResponse("redirected"),
	)

	injected := false
	cfg := &Config{
		Model:  testModel(),
		Tools:  []AgentTool{makeTool("first"), makeTool("second")},
		StreamFunc: streamFn,
		GetSteeringMessages: func() []ai.Message {
			if !injected {
				injected = true
				return []ai.Message{ai.NewUserMessage("stop after first")}
			}
			return nil
		},
	}
	_, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("run tools")})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Only the first tool should have executed; steering skips the second
	if len(executed) != 1 || executed[0] != "first" {
		t.Errorf("expected only 'first' to execute, got %v", executed)
	}
}

// ---------------------------------------------------------------------------
// ALIGN-06: follow-up triggers additional turn
// A follow-up queued before the agent runs causes an extra LLM turn after
// the initial turn completes.
// ---------------------------------------------------------------------------

func TestAlign_06_FollowUpTriggersTurn(t *testing.T) {
	turnCount := int32(0)
	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		atomic.AddInt32(&turnCount, 1)
		return mockStreamFn(textResponse("ok"))(ctx, model, reqCtx, opts)
	}

	followUpDelivered := false
	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		GetFollowUpMessages: func() []ai.Message {
			if !followUpDelivered {
				followUpDelivered = true
				return []ai.Message{ai.NewUserMessage("follow up")}
			}
			return nil
		},
	}
	_, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("start")})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if atomic.LoadInt32(&turnCount) != 2 {
		t.Errorf("expected 2 LLM turns (initial + follow-up), got %d", turnCount)
	}
}

// ---------------------------------------------------------------------------
// ALIGN-07: max turns limit
// ---------------------------------------------------------------------------

func TestAlign_07_MaxTurnsLimit(t *testing.T) {
	calls := int32(0)
	streamFn := func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		atomic.AddInt32(&calls, 1)
		return mockStreamFn(textResponse("ok"))(ctx, model, reqCtx, opts)
	}

	// GetFollowUpMessages always injects a new turn, creating an unbounded loop.
	// MaxTurns=3 should cap it at exactly 3 LLM calls.
	cfg := &Config{
		Model:      testModel(),
		StreamFunc: streamFn,
		MaxTurns:   3,
		GetFollowUpMessages: func() []ai.Message {
			return []ai.Message{ai.NewUserMessage("keep going")}
		},
	}
	_, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("start")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("expected exactly 3 LLM calls (MaxTurns=3), got %d", calls)
	}
}

// ---------------------------------------------------------------------------
// ALIGN-08: abort terminates loop
// ---------------------------------------------------------------------------

func TestAlign_08_AbortTerminatesLoop(t *testing.T) {
	a := New(Config{
		Model: testModel(),
		Tools: []AgentTool{
			{
				Tool: ai.Tool{Name: "block"},
				Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
					<-ctx.Done()
					return ErrorResult("aborted"), nil
				},
			},
		},
		StreamFunc: mockStreamFn(
			toolCallResponse(ai.ToolCallBlock("tc1", "block", map[string]any{})),
			textResponse("unreachable"),
		),
	})

	started := make(chan struct{})
	a.Subscribe(func(event Event) {
		if event.Type == EventToolExecStart {
			close(started)
		}
	})

	done := make(chan error, 1)
	go func() {
		_, err := a.Prompt(context.Background(), "start")
		done <- err
	}()

	<-started
	a.Abort()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error after Abort, got nil")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not finish after Abort — possible deadlock")
	}
}

// ---------------------------------------------------------------------------
// ALIGN-09: events emitted in correct order
// Verifies the canonical event sequence for a single tool call turn.
// ---------------------------------------------------------------------------

func TestAlign_09_EventSequence(t *testing.T) {
	var events []EventType
	var mu sync.Mutex

	cfg := &Config{
		Model:  testModel(),
		Tools:  []AgentTool{echoTool("echo")},
		StreamFunc: mockStreamFn(
			toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "x"})),
			textResponse("done"),
		),
		OnEvent: func(e Event) {
			mu.Lock()
			events = append(events, e.Type)
			mu.Unlock()
		},
	}

	_, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Expected sequence (simplified): agent_start, turn_start, msg_start, msg_update...,
	// msg_end, tool_exec_start, tool_exec_end, turn_end, turn_start, msg_start, ...,
	// msg_end, turn_end, agent_end
	assertEventBefore(t, events, EventAgentStart, EventAgentEnd)
	assertEventBefore(t, events, EventTurnStart, EventTurnEnd)
	assertEventBefore(t, events, EventMessageStart, EventMessageEnd)
	assertEventBefore(t, events, EventToolExecStart, EventToolExecEnd)
	assertEventBefore(t, events, EventToolExecEnd, EventTurnEnd)
	// agent_end must be the last event
	if events[len(events)-1] != EventAgentEnd {
		t.Errorf("expected EventAgentEnd to be last, got %v", events[len(events)-1])
	}
}

// ---------------------------------------------------------------------------
// ALIGN-10: unknown tool returns error result (not a loop crash)
// ---------------------------------------------------------------------------

func TestAlign_10_UnknownToolErrorResult(t *testing.T) {
	cfg := &Config{
		Model:      testModel(),
		Tools:      []AgentTool{}, // no tools registered
		StreamFunc: mockStreamFn(
			toolCallResponse(ai.ToolCallBlock("tc1", "nonexistent", map[string]any{})),
			textResponse("handled"),
		),
	}
	msgs, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr, ok := msgs[2].(*ai.ToolResultMessage)
	if !ok {
		t.Fatalf("expected ToolResultMessage, got %T", msgs[2])
	}
	if !tr.IsError {
		t.Error("expected IsError=true for unknown tool")
	}
}

// ---------------------------------------------------------------------------
// ALIGN-11: context transform applied before each LLM call
// TransformContext must be called for every LLM turn.
// ---------------------------------------------------------------------------

func TestAlign_11_TransformContextCalledPerTurn(t *testing.T) {
	transformCalls := int32(0)
	cfg := &Config{
		Model:  testModel(),
		Tools:  []AgentTool{echoTool("echo")},
		StreamFunc: mockStreamFn(
			toolCallResponse(ai.ToolCallBlock("tc1", "echo", map[string]any{"input": "x"})),
			textResponse("done"),
		),
		TransformContext: func(ctx context.Context, msgs []ai.Message) ([]ai.Message, error) {
			atomic.AddInt32(&transformCalls, 1)
			return msgs, nil
		},
	}
	_, err := RunLoop(context.Background(), cfg, []ai.Message{ai.NewUserMessage("go")})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Two LLM turns: initial + after tool result
	if atomic.LoadInt32(&transformCalls) != 2 {
		t.Errorf("expected TransformContext called 2 times, got %d", transformCalls)
	}
}

// ---------------------------------------------------------------------------
// ALIGN-12: reset + re-prompt starts fresh
// After Reset(), a new Prompt should have no carry-over from previous turns.
// ---------------------------------------------------------------------------

func TestAlign_12_ResetAndReprompt(t *testing.T) {
	a := New(Config{
		Model:      testModel(),
		StreamFunc: mockStreamFn(textResponse("first"), textResponse("second")),
	})

	_, err := a.Prompt(context.Background(), "first question")
	if err != nil {
		t.Fatalf("first prompt error: %v", err)
	}
	firstLen := len(a.Messages())

	a.Reset()

	_, err = a.Prompt(context.Background(), "second question")
	if err != nil {
		t.Fatalf("second prompt error: %v", err)
	}
	secondLen := len(a.Messages())

	// After reset, history starts fresh — second prompt should not carry over first
	if secondLen >= firstLen+2 {
		t.Errorf("expected fresh history after Reset, got %d messages (first had %d)", secondLen, firstLen)
	}
	// Should be exactly 2: [user, assistant]
	if secondLen != 2 {
		t.Errorf("expected 2 messages after fresh prompt, got %d", secondLen)
	}
}

// ---------------------------------------------------------------------------
// helper assertions
// ---------------------------------------------------------------------------

func assertRole(t *testing.T, msg ai.Message, role ai.Role, idx int) {
	t.Helper()
	if msg.GetRole() != role {
		t.Errorf("messages[%d]: expected role %q, got %q", idx, role, msg.GetRole())
	}
}

func assertEventBefore(t *testing.T, events []EventType, before, after EventType) {
	t.Helper()
	bi, ai_ := -1, -1
	for i, e := range events {
		if e == before && bi == -1 {
			bi = i
		}
		if e == after {
			ai_ = i
		}
	}
	if bi == -1 {
		t.Errorf("event %q not found", before)
		return
	}
	if ai_ == -1 {
		t.Errorf("event %q not found", after)
		return
	}
	if bi >= ai_ {
		t.Errorf("expected %q (idx %d) before %q (idx %d)", before, bi, after, ai_)
	}
}
