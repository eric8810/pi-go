// Package anthropic implements the Anthropic Messages API provider for the ai package.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"pi-go/ai"
)

func init() {
	ai.RegisterAPIProvider(&provider{})
}

// provider implements ai.APIProvider for the Anthropic Messages API.
type provider struct{}

func (p *provider) API() ai.API {
	return ai.APIAnthropicMessages
}

func (p *provider) Stream(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
	stream := ai.NewEventStream(128)
	go p.run(ctx, model, reqCtx, opts, stream)
	return stream
}

func (p *provider) run(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions, stream *ai.EventStream) {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		emitError(stream, "ANTHROPIC_API_KEY not set and no APIKey provided in options")
		return
	}

	body, err := buildRequestBody(model, reqCtx, opts)
	if err != nil {
		emitError(stream, fmt.Sprintf("failed to build request body: %v", err))
		return
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := strings.TrimRight(baseURL, "/") + "/v1/messages"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		emitError(stream, fmt.Sprintf("failed to create HTTP request: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	thinkingEnabled := opts.Thinking != "" && opts.Thinking != ai.ThinkingOff
	if thinkingEnabled {
		req.Header.Set("anthropic-beta", "interleaved-thinking-2025-05-14")
	}

	// Apply model-level headers.
	for k, v := range model.Headers {
		req.Header.Set(k, v)
	}

	// Apply per-request custom headers.
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			emitAborted(stream)
			return
		}
		emitError(stream, fmt.Sprintf("HTTP request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		emitError(stream, fmt.Sprintf("Anthropic API error (HTTP %d): %s", resp.StatusCode, string(respBody)))
		return
	}

	parseSSEStream(ctx, resp.Body, model, stream)
}

// thinkingBudget maps ThinkingLevel to budget_tokens.
func thinkingBudget(level ai.ThinkingLevel) int {
	switch level {
	case ai.ThinkingMinimal:
		return 1024
	case ai.ThinkingLow:
		return 4096
	case ai.ThinkingMedium:
		return 10000
	case ai.ThinkingHigh:
		return 20000
	case ai.ThinkingXHigh:
		return 40000
	default:
		return 0
	}
}

// ---------- Request body construction ----------

func buildRequestBody(model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) ([]byte, error) {
	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = model.MaxTokens
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	thinkingEnabled := opts.Thinking != "" && opts.Thinking != ai.ThinkingOff
	budget := thinkingBudget(opts.Thinking)

	// When thinking is enabled, Anthropic requires max_tokens to be at least
	// budget_tokens + 1. Adjust upward if necessary.
	if thinkingEnabled && maxTokens <= budget {
		maxTokens = budget + 1024
	}

	body := map[string]any{
		"model":      model.ID,
		"max_tokens": maxTokens,
		"stream":     true,
	}

	if reqCtx.SystemPrompt != "" {
		body["system"] = reqCtx.SystemPrompt
	}

	messages := convertMessages(reqCtx.Messages)
	body["messages"] = messages

	if len(reqCtx.Tools) > 0 {
		body["tools"] = convertTools(reqCtx.Tools)
	}

	if thinkingEnabled {
		body["thinking"] = map[string]any{
			"type":          "enabled",
			"budget_tokens": budget,
		}
	}

	if opts.Temperature != nil && !thinkingEnabled {
		body["temperature"] = *opts.Temperature
	}

	// Merge provider-specific extra options.
	for k, v := range opts.Extra {
		body[k] = v
	}

	return json.Marshal(body)
}

// ---------- Message conversion ----------

// anthropicMessage is the wire format for a single message sent to the API.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content []any  `json:"content"`
}

// convertMessages transforms the generic ai.Message slice into Anthropic's wire format.
// It merges consecutive tool results into a single user message, as required by the API.
func convertMessages(messages []ai.Message) []anthropicMessage {
	var result []anthropicMessage

	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		if msg == nil {
			continue
		}

		switch m := msg.(type) {
		case *ai.UserMessage:
			result = append(result, anthropicMessage{
				Role:    "user",
				Content: convertUserContent(m.Content),
			})

		case *ai.AssistantMessage:
			result = append(result, anthropicMessage{
				Role:    "assistant",
				Content: convertAssistantContent(m.Content),
			})

		case *ai.ToolResultMessage:
			// Collect consecutive tool results into one user message.
			var toolResults []any
			for ; i < len(messages); i++ {
				tr, ok := messages[i].(*ai.ToolResultMessage)
				if !ok {
					i-- // back up so the outer loop processes this message
					break
				}
				toolResults = append(toolResults, convertToolResult(tr))
			}
			result = append(result, anthropicMessage{
				Role:    "user",
				Content: toolResults,
			})
		}
	}

	return result
}

func convertUserContent(blocks []ai.ContentBlock) []any {
	var content []any
	for _, b := range blocks {
		switch b.Type {
		case ai.ContentText:
			content = append(content, map[string]any{
				"type": "text",
				"text": b.Text,
			})
		case ai.ContentImage:
			content = append(content, map[string]any{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": b.MimeType,
					"data":       b.Data,
				},
			})
		}
	}
	if len(content) == 0 {
		content = append(content, map[string]any{
			"type": "text",
			"text": "",
		})
	}
	return content
}

func convertAssistantContent(blocks []ai.ContentBlock) []any {
	var content []any
	for _, b := range blocks {
		switch b.Type {
		case ai.ContentText:
			content = append(content, map[string]any{
				"type": "text",
				"text": b.Text,
			})
		case ai.ContentThinking:
			content = append(content, map[string]any{
				"type":     "thinking",
				"thinking": b.Thinking,
			})
		case ai.ContentToolCall:
			input := b.Arguments
			if input == nil {
				input = map[string]any{}
			}
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    b.ID,
				"name":  b.Name,
				"input": input,
			})
		}
	}
	if len(content) == 0 {
		content = append(content, map[string]any{
			"type": "text",
			"text": "",
		})
	}
	return content
}

func convertToolResult(m *ai.ToolResultMessage) map[string]any {
	var content []map[string]any
	for _, b := range m.Content {
		if b.Type == ai.ContentText {
			content = append(content, map[string]any{
				"type": "text",
				"text": b.Text,
			})
		}
	}
	if len(content) == 0 {
		content = append(content, map[string]any{
			"type": "text",
			"text": "",
		})
	}

	result := map[string]any{
		"type":        "tool_result",
		"tool_use_id": m.ToolCallID,
		"content":     content,
	}
	if m.IsError {
		result["is_error"] = true
	}
	return result
}

func convertTools(tools []ai.Tool) []map[string]any {
	var result []map[string]any
	for _, t := range tools {
		result = append(result, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": convertToolParameter(t.Parameters),
		})
	}
	return result
}

func convertToolParameter(p ai.ToolParameter) map[string]any {
	schema := map[string]any{
		"type": p.Type,
	}
	if p.Description != "" {
		schema["description"] = p.Description
	}
	if len(p.Properties) > 0 {
		props := map[string]any{}
		for name, prop := range p.Properties {
			props[name] = convertToolParameter(prop)
		}
		schema["properties"] = props
	}
	if len(p.Required) > 0 {
		schema["required"] = p.Required
	}
	if len(p.Enum) > 0 {
		schema["enum"] = p.Enum
	}
	if p.Items != nil {
		schema["items"] = convertToolParameter(*p.Items)
	}
	if p.Default != nil {
		schema["default"] = p.Default
	}
	return schema
}

// ---------- SSE stream parsing ----------

// sseEvent represents a single Server-Sent Event.
type sseEvent struct {
	Event string
	Data  string
}

// parseSSEStream reads the SSE response body and emits ai.StreamEvent values.
func parseSSEStream(ctx context.Context, body io.Reader, model *ai.Model, stream *ai.EventStream) {
	// State accumulated across the stream.
	var (
		contentBlocks []ai.ContentBlock
		usage         ai.Usage
		stopReason    ai.StopReason = ai.StopReasonStop

		// Per-block accumulators indexed by the Anthropic content_block index.
		blockTypes   = map[int]string{}        // "text", "thinking", "tool_use"
		blockToolIDs = map[int]string{}         // tool_use id
		blockNames   = map[int]string{}         // tool_use name
		textAccum    = map[int]*strings.Builder{}
		modelID      = model.ID
	)

	scanner := bufio.NewScanner(body)
	// Increase the buffer for large SSE payloads.
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	var currentEvent string

	for scanner.Scan() {
		// Check for context cancellation between lines.
		if ctx.Err() != nil {
			emitAborted(stream)
			return
		}

		line := scanner.Text()

		// SSE blank line marks end of event.
		if line == "" {
			currentEvent = ""
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		switch currentEvent {
		case "message_start":
			var payload struct {
				Message struct {
					ID    string `json:"id"`
					Model string `json:"model"`
					Usage struct {
						InputTokens        int `json:"input_tokens"`
						OutputTokens       int `json:"output_tokens"`
						CacheReadTokens    int `json:"cache_read_input_tokens"`
						CacheCreationTokens int `json:"cache_creation_input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				continue
			}
			if payload.Message.Model != "" {
				modelID = payload.Message.Model
			}
			usage.InputTokens = payload.Message.Usage.InputTokens
			usage.OutputTokens = payload.Message.Usage.OutputTokens
			usage.CacheReadTokens = payload.Message.Usage.CacheReadTokens
			usage.CacheWriteTokens = payload.Message.Usage.CacheCreationTokens

			stream.Push(ai.StreamEvent{Type: ai.EventStart})

		case "content_block_start":
			var payload struct {
				Index        int `json:"index"`
				ContentBlock struct {
					Type string `json:"type"`
					ID   string `json:"id"`
					Name string `json:"name"`
					Text string `json:"text"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				continue
			}
			idx := payload.Index
			blockType := payload.ContentBlock.Type
			blockTypes[idx] = blockType
			textAccum[idx] = &strings.Builder{}

			switch blockType {
			case "text":
				if payload.ContentBlock.Text != "" {
					textAccum[idx].WriteString(payload.ContentBlock.Text)
				}
				stream.Push(ai.StreamEvent{
					Type:         ai.EventTextStart,
					ContentIndex: idx,
				})
			case "thinking":
				stream.Push(ai.StreamEvent{
					Type:         ai.EventThinkingStart,
					ContentIndex: idx,
				})
			case "tool_use":
				blockToolIDs[idx] = payload.ContentBlock.ID
				blockNames[idx] = payload.ContentBlock.Name
				stream.Push(ai.StreamEvent{
					Type:         ai.EventToolCallStart,
					ContentIndex: idx,
					ToolCall: &ai.ContentBlock{
						Type: ai.ContentToolCall,
						ID:   payload.ContentBlock.ID,
						Name: payload.ContentBlock.Name,
					},
				})
			}

		case "content_block_delta":
			var payload struct {
				Index int `json:"index"`
				Delta struct {
					Type     string `json:"type"`
					Text     string `json:"text"`
					Thinking string `json:"thinking"`
					JSON     string `json:"partial_json"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				continue
			}
			idx := payload.Index

			switch payload.Delta.Type {
			case "text_delta":
				if acc, ok := textAccum[idx]; ok {
					acc.WriteString(payload.Delta.Text)
				}
				stream.Push(ai.StreamEvent{
					Type:         ai.EventTextDelta,
					ContentIndex: idx,
					Delta:        payload.Delta.Text,
				})
			case "thinking_delta":
				if acc, ok := textAccum[idx]; ok {
					acc.WriteString(payload.Delta.Thinking)
				}
				stream.Push(ai.StreamEvent{
					Type:         ai.EventThinkingDelta,
					ContentIndex: idx,
					Delta:        payload.Delta.Thinking,
				})
			case "input_json_delta":
				if acc, ok := textAccum[idx]; ok {
					acc.WriteString(payload.Delta.JSON)
				}
				stream.Push(ai.StreamEvent{
					Type:         ai.EventToolCallDelta,
					ContentIndex: idx,
					Delta:        payload.Delta.JSON,
				})
			}

		case "content_block_stop":
			var payload struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				continue
			}
			idx := payload.Index
			blockType := blockTypes[idx]
			accumulated := ""
			if acc, ok := textAccum[idx]; ok {
				accumulated = acc.String()
			}

			switch blockType {
			case "text":
				contentBlocks = append(contentBlocks, ai.TextBlock(accumulated))
				stream.Push(ai.StreamEvent{
					Type:         ai.EventTextEnd,
					ContentIndex: idx,
				})
			case "thinking":
				if accumulated != "" {
					contentBlocks = append(contentBlocks, ai.ThinkingBlock(accumulated))
				}
				stream.Push(ai.StreamEvent{
					Type:         ai.EventThinkingEnd,
					ContentIndex: idx,
				})
			case "tool_use":
				var args map[string]any
				if accumulated != "" {
					if err := json.Unmarshal([]byte(accumulated), &args); err != nil {
						args = map[string]any{}
					}
				} else {
					args = map[string]any{}
				}
				toolBlock := ai.ToolCallBlock(blockToolIDs[idx], blockNames[idx], args)
				contentBlocks = append(contentBlocks, toolBlock)
				stream.Push(ai.StreamEvent{
					Type:         ai.EventToolCallEnd,
					ContentIndex: idx,
					ToolCall:     &toolBlock,
				})
			}

		case "message_delta":
			var payload struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				continue
			}
			if payload.Usage.OutputTokens > 0 {
				usage.OutputTokens = payload.Usage.OutputTokens
			}
			stopReason = mapStopReason(payload.Delta.StopReason)

		case "message_stop":
			finalMsg := &ai.AssistantMessage{
				Content:    contentBlocks,
				API:        ai.APIAnthropicMessages,
				Provider:   model.Provider,
				Model:      modelID,
				Usage:      usage,
				StopReason: stopReason,
				Timestamp:  time.Now(),
			}
			stream.Push(ai.StreamEvent{
				Type:    ai.EventDone,
				Message: finalMsg,
				Reason:  stopReason,
			})
			stream.End(finalMsg)
			return

		case "error":
			var payload struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				emitError(stream, fmt.Sprintf("SSE error event (unparseable): %s", data))
				return
			}
			emitError(stream, fmt.Sprintf("Anthropic API error (%s): %s", payload.Error.Type, payload.Error.Message))
			return

		case "ping":
			// Heartbeat; ignore.
		}
	}

	// If we reach here, the stream ended without a message_stop event.
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			emitAborted(stream)
			return
		}
		emitError(stream, fmt.Sprintf("SSE stream read error: %v", err))
		return
	}

	// Stream ended cleanly but without message_stop. Emit what we have.
	finalMsg := &ai.AssistantMessage{
		Content:    contentBlocks,
		API:        ai.APIAnthropicMessages,
		Provider:   model.Provider,
		Model:      modelID,
		Usage:      usage,
		StopReason: stopReason,
		Timestamp:  time.Now(),
	}
	stream.Push(ai.StreamEvent{
		Type:    ai.EventDone,
		Message: finalMsg,
		Reason:  stopReason,
	})
	stream.End(finalMsg)
}

// ---------- Helpers ----------

func mapStopReason(reason string) ai.StopReason {
	switch reason {
	case "end_turn", "stop":
		return ai.StopReasonStop
	case "max_tokens":
		return ai.StopReasonLength
	case "tool_use":
		return ai.StopReasonToolUse
	default:
		return ai.StopReasonStop
	}
}

// emitError sends an error event and ends the stream.
func emitError(stream *ai.EventStream, message string) {
	errMsg := &ai.AssistantMessage{
		StopReason:   ai.StopReasonError,
		ErrorMessage: message,
		Timestamp:    time.Now(),
	}
	stream.Push(ai.StreamEvent{
		Type:    ai.EventError,
		Message: errMsg,
		Reason:  ai.StopReasonError,
	})
	stream.Error(errMsg)
}

// emitAborted sends an aborted event and ends the stream.
func emitAborted(stream *ai.EventStream) {
	abortMsg := &ai.AssistantMessage{
		StopReason: ai.StopReasonAborted,
		Timestamp:  time.Now(),
	}
	stream.Push(ai.StreamEvent{
		Type:    ai.EventDone,
		Message: abortMsg,
		Reason:  ai.StopReasonAborted,
	})
	stream.End(abortMsg)
}
