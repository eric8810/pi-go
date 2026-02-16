package agent

import (
	"context"
	"sync"

	"pi-go/ai"
)

// testModel returns a Model suitable for testing.
func testModel() *ai.Model {
	return &ai.Model{
		ID:       "test-model",
		Name:     "Test Model",
		API:      ai.APIAnthropicMessages,
		Provider: ai.ProviderAnthropic,
	}
}

// textResponse creates an AssistantMessage with a single text block and stop reason.
func textResponse(text string) *ai.AssistantMessage {
	return &ai.AssistantMessage{
		Content:    []ai.ContentBlock{ai.TextBlock(text)},
		StopReason: ai.StopReasonStop,
	}
}

// toolCallResponse creates an AssistantMessage with one or more tool call blocks.
func toolCallResponse(calls ...ai.ContentBlock) *ai.AssistantMessage {
	return &ai.AssistantMessage{
		Content:    calls,
		StopReason: ai.StopReasonToolUse,
	}
}

// mockStreamFn creates a StreamFunc that returns responses from a list.
// Each call to the returned function pops the next response.
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

// mockStreamFnWithCapture creates a StreamFunc that captures the contexts passed to it.
func mockStreamFnWithCapture(responses []*ai.AssistantMessage, captured *[][]ai.Message) ai.StreamFunc {
	var mu sync.Mutex
	idx := 0
	return func(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
		mu.Lock()
		*captured = append(*captured, reqCtx.Messages)
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

// echoTool creates an AgentTool that returns whatever was passed as "input".
func echoTool(name string) AgentTool {
	return AgentTool{
		Tool: ai.Tool{
			Name:        name,
			Description: "Echo tool for testing",
			Parameters: ai.ToolParameter{
				Type: "object",
				Properties: map[string]ai.ToolParameter{
					"input": {Type: "string"},
				},
			},
		},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			input, _ := params["input"].(string)
			return TextResult("echo: " + input), nil
		},
	}
}

// errorTool creates an AgentTool that always returns an error.
func errorTool(name string) AgentTool {
	return AgentTool{
		Tool: ai.Tool{
			Name:        name,
			Description: "Error tool for testing",
		},
		Execute: func(ctx context.Context, toolCallID string, params map[string]any, onUpdate UpdateCallback) (*ToolResult, error) {
			return nil, context.DeadlineExceeded
		},
	}
}

// collectEvents returns an EventHandler that appends events to a slice.
func collectEvents(events *[]Event) EventHandler {
	var mu sync.Mutex
	return func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		*events = append(*events, event)
	}
}
