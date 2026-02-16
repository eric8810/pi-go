package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agentsdk/ai"
)

// RunLoop executes the agent loop: stream LLM responses, execute tools, repeat.
//
// The loop follows this pattern:
//  1. Build context from system prompt + messages + tools
//  2. Stream LLM response
//  3. If response contains tool calls, execute them sequentially
//  4. After each tool call, check for steering messages
//  5. If steering messages exist, inject them and continue
//  6. After all tools complete, check for follow-up messages
//  7. If follow-ups exist, inject and continue; otherwise, return
func RunLoop(ctx context.Context, cfg *Config, messages []ai.Message) ([]ai.Message, error) {
	streamFn := cfg.StreamFunc
	if streamFn == nil {
		streamFn = ai.Stream
	}

	emit := cfg.OnEvent
	if emit == nil {
		emit = func(Event) {}
	}

	emit(Event{Type: EventAgentStart})

	turns := 0

	// Outer loop: handles follow-up messages
	for {
		if ctx.Err() != nil {
			emit(Event{Type: EventAgentEnd, Messages: messages})
			return messages, ctx.Err()
		}

		// Check turn limit
		if cfg.MaxTurns > 0 && turns >= cfg.MaxTurns {
			emit(Event{Type: EventAgentEnd, Messages: messages})
			return messages, nil
		}
		turns++

		// Inner loop: handles tool calls and steering
		for {
			if ctx.Err() != nil {
				emit(Event{Type: EventAgentEnd, Messages: messages})
				return messages, ctx.Err()
			}

			emit(Event{Type: EventTurnStart})

			// Build LLM context
			llmMessages := messages
			if cfg.ConvertToLLM != nil {
				llmMessages = cfg.ConvertToLLM(messages)
			}
			if cfg.TransformContext != nil {
				var err error
				llmMessages, err = cfg.TransformContext(ctx, llmMessages)
				if err != nil {
					emit(Event{Type: EventAgentEnd, Messages: messages})
					return messages, fmt.Errorf("transform context: %w", err)
				}
			}

			// Build tools list
			var tools []ai.Tool
			for _, t := range cfg.Tools {
				tools = append(tools, t.Tool)
			}

			reqCtx := &ai.Context{
				SystemPrompt: cfg.SystemPrompt,
				Messages:     llmMessages,
				Tools:        tools,
			}

			opts := ai.StreamOptions{
				Thinking: cfg.ThinkingLevel,
				APIKey:   cfg.APIKey,
			}

			// Stream LLM response
			partial := &ai.AssistantMessage{
				API:       cfg.Model.API,
				Provider:  cfg.Model.Provider,
				Model:     cfg.Model.ID,
				Timestamp: time.Now(),
			}

			emit(Event{Type: EventMessageStart, Message: partial})

			stream := streamFn(ctx, cfg.Model, reqCtx, opts)

			for ev := range stream.Events() {
				// Update partial message from events
				updatePartialMessage(partial, ev)
				emit(Event{
					Type:        EventMessageUpdate,
					Message:     partial,
					StreamEvent: &ev,
				})
			}

			result := stream.Result()
			if result == nil {
				result = partial
				result.StopReason = ai.StopReasonError
				result.ErrorMessage = "stream returned nil result"
			}

			emit(Event{Type: EventMessageEnd, Message: result})

			// Add assistant message to history
			messages = append(messages, result)

			// Handle error/abort
			if result.StopReason == ai.StopReasonError || result.StopReason == ai.StopReasonAborted {
				var toolResults []*ai.ToolResultMessage
				emit(Event{Type: EventTurnEnd, Message: result, ToolResults: toolResults})
				emit(Event{Type: EventAgentEnd, Messages: messages})
				if result.StopReason == ai.StopReasonError {
					return messages, fmt.Errorf("LLM error: %s", result.ErrorMessage)
				}
				return messages, ctx.Err()
			}

			// If no tool calls, the turn is done
			toolCalls := result.GetToolCalls()
			if len(toolCalls) == 0 {
				emit(Event{Type: EventTurnEnd, Message: result})
				break // exit inner loop
			}

			// Execute tool calls sequentially
			var toolResults []*ai.ToolResultMessage
			steering := false

			for _, tc := range toolCalls {
				if ctx.Err() != nil {
					break
				}

				// Find the tool
				tool := findTool(cfg.Tools, tc.Name)
				if tool == nil {
					// Unknown tool - return error result
					tr := ai.NewToolResult(tc.ID, tc.Name,
						fmt.Sprintf("Unknown tool: %s", tc.Name), true)
					messages = append(messages, tr)
					toolResults = append(toolResults, tr)
					continue
				}

				emit(Event{
					Type:       EventToolExecStart,
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					ToolArgs:   tc.Arguments,
				})

				// Execute the tool
				toolResult, err := tool.Execute(ctx, tc.ID, tc.Arguments, func(update ToolUpdate) {
					emit(Event{
						Type:       EventToolExecUpdate,
						ToolCallID: tc.ID,
						ToolName:   tc.Name,
						ToolArgs:   tc.Arguments,
						ToolUpdate: &update,
					})
				})

				if err != nil {
					toolResult = ErrorResult(fmt.Sprintf("Tool execution error: %v", err))
				}

				emit(Event{
					Type:       EventToolExecEnd,
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					ToolResult: toolResult,
					IsError:    toolResult.IsError,
				})

				// Convert to ToolResultMessage
				tr := &ai.ToolResultMessage{
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					Content:    toolResult.Content,
					IsError:    toolResult.IsError,
					Timestamp:  time.Now(),
				}
				messages = append(messages, tr)
				toolResults = append(toolResults, tr)

				// Check for steering messages after each tool call
				if cfg.GetSteeringMessages != nil {
					steeringMsgs := cfg.GetSteeringMessages()
					if len(steeringMsgs) > 0 {
						messages = append(messages, steeringMsgs...)
						steering = true
						break // skip remaining tools
					}
				}
			}

			emit(Event{Type: EventTurnEnd, Message: result, ToolResults: toolResults})

			if steering {
				continue // continue inner loop with steering messages
			}

			// All tools executed, continue inner loop for next LLM call
			continue
		}

		// Inner loop exited (no more tool calls)
		// Check for follow-up messages
		if cfg.GetFollowUpMessages != nil {
			followUps := cfg.GetFollowUpMessages()
			if len(followUps) > 0 {
				messages = append(messages, followUps...)
				continue // continue outer loop
			}
		}

		// No follow-ups, we're done
		emit(Event{Type: EventAgentEnd, Messages: messages})
		return messages, nil
	}
}

// findTool looks up a tool by name.
func findTool(tools []AgentTool, name string) *AgentTool {
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	return nil
}

// updatePartialMessage updates the partial assistant message from a stream event.
func updatePartialMessage(msg *ai.AssistantMessage, ev ai.StreamEvent) {
	switch ev.Type {
	case ai.EventStart:
		if ev.Partial != nil {
			msg.Usage = ev.Partial.Usage
		}

	case ai.EventTextStart:
		// Ensure content slice is large enough
		ensureContentIndex(msg, ev.ContentIndex)
		msg.Content[ev.ContentIndex] = ai.TextBlock("")

	case ai.EventTextDelta:
		ensureContentIndex(msg, ev.ContentIndex)
		msg.Content[ev.ContentIndex].Text += ev.Delta

	case ai.EventThinkingStart:
		ensureContentIndex(msg, ev.ContentIndex)
		msg.Content[ev.ContentIndex] = ai.ThinkingBlock("")

	case ai.EventThinkingDelta:
		ensureContentIndex(msg, ev.ContentIndex)
		msg.Content[ev.ContentIndex].Thinking += ev.Delta

	case ai.EventToolCallStart:
		ensureContentIndex(msg, ev.ContentIndex)
		if ev.ToolCall != nil {
			msg.Content[ev.ContentIndex] = *ev.ToolCall
		}

	case ai.EventToolCallDelta:
		// Arguments are accumulated as JSON string deltas
		ensureContentIndex(msg, ev.ContentIndex)
		// Delta contains JSON fragments - we accumulate in Text temporarily
		msg.Content[ev.ContentIndex].Text += ev.Delta

	case ai.EventToolCallEnd:
		ensureContentIndex(msg, ev.ContentIndex)
		if ev.ToolCall != nil {
			msg.Content[ev.ContentIndex] = *ev.ToolCall
		} else {
			// Parse accumulated JSON arguments
			block := &msg.Content[ev.ContentIndex]
			if block.Text != "" && block.Arguments == nil {
				var args map[string]any
				if err := json.Unmarshal([]byte(block.Text), &args); err == nil {
					block.Arguments = args
				}
				block.Text = "" // clear accumulated JSON
			}
		}

	case ai.EventDone:
		if ev.Message != nil {
			msg.Usage = ev.Message.Usage
			msg.StopReason = ev.Message.StopReason
		} else {
			msg.StopReason = ev.Reason
		}

	case ai.EventError:
		if ev.Message != nil {
			msg.StopReason = ev.Message.StopReason
			msg.ErrorMessage = ev.Message.ErrorMessage
		} else {
			msg.StopReason = ai.StopReasonError
		}
	}
}

// ensureContentIndex grows the content slice to include the given index.
func ensureContentIndex(msg *ai.AssistantMessage, idx int) {
	for len(msg.Content) <= idx {
		msg.Content = append(msg.Content, ai.ContentBlock{})
	}
}
