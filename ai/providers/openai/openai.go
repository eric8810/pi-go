// Package openai implements the OpenAI Chat Completions streaming API provider.
package openai

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

	"agentsdk/ai"
)

func init() {
	ai.RegisterAPIProvider(&provider{})
}

// provider implements ai.APIProvider for the OpenAI Chat Completions API.
type provider struct{}

func (p *provider) API() ai.API {
	return ai.APIOpenAICompletions
}

func (p *provider) Stream(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) *ai.EventStream {
	stream := ai.NewEventStream(64)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errMsg := &ai.AssistantMessage{
					Model:        model.ID,
					Provider:     model.Provider,
					API:          model.API,
					StopReason:   ai.StopReasonError,
					ErrorMessage: fmt.Sprintf("panic in openai provider: %v", r),
					Timestamp:    time.Now(),
				}
				stream.Push(ai.StreamEvent{Type: ai.EventError, Message: errMsg, Reason: ai.StopReasonError})
				stream.Error(errMsg)
			}
		}()
		p.run(ctx, model, reqCtx, opts, stream)
	}()

	return stream
}

func (p *provider) run(ctx context.Context, model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions, stream *ai.EventStream) {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		p.emitError(stream, model, "OpenAI API key not set: provide via StreamOptions.APIKey or OPENAI_API_KEY env var")
		return
	}

	body, err := p.buildRequestBody(model, reqCtx, opts)
	if err != nil {
		p.emitError(stream, model, fmt.Sprintf("failed to build request body: %v", err))
		return
	}

	baseURL := model.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := strings.TrimRight(baseURL, "/") + "/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		p.emitError(stream, model, fmt.Sprintf("failed to create HTTP request: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	// Apply model-level headers.
	for k, v := range model.Headers {
		req.Header.Set(k, v)
	}
	// Apply per-request headers (override model headers if overlapping).
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			p.emitAborted(stream, model)
			return
		}
		p.emitError(stream, model, fmt.Sprintf("HTTP request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		p.emitError(stream, model, fmt.Sprintf("OpenAI API error (HTTP %d): %s", resp.StatusCode, string(respBody)))
		return
	}

	p.processSSEStream(ctx, resp.Body, model, stream)
}

// ---------- Request building ----------

func (p *provider) buildRequestBody(model *ai.Model, reqCtx *ai.Context, opts ai.StreamOptions) ([]byte, error) {
	req := requestBody{
		Model:         model.ID,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
	}

	// Messages: start with the system prompt, then convert conversation messages.
	if reqCtx.SystemPrompt != "" {
		req.Messages = append(req.Messages, chatMessage{
			Role:    "system",
			Content: rawOrString(reqCtx.SystemPrompt),
		})
	}

	for _, msg := range reqCtx.Messages {
		converted := p.convertMessage(msg)
		req.Messages = append(req.Messages, converted...)
	}

	// Tools.
	for _, tool := range reqCtx.Tools {
		schema, err := toolParameterToJSON(tool.Parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool %q parameters: %w", tool.Name, err)
		}
		req.Tools = append(req.Tools, chatTool{
			Type: "function",
			Function: chatFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  schema,
			},
		})
	}

	// Max tokens.
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = model.MaxTokens
	}
	if model.Reasoning {
		req.MaxCompletionTokens = maxTokens
	} else {
		req.MaxTokens = maxTokens
	}

	// Temperature (not applicable for reasoning models).
	if opts.Temperature != nil && !model.Reasoning {
		req.Temperature = opts.Temperature
	}

	// Reasoning effort for reasoning models.
	if model.Reasoning && opts.Thinking != "" && opts.Thinking != ai.ThinkingOff {
		req.ReasoningEffort = mapThinkingLevel(opts.Thinking)
	}

	return json.Marshal(req)
}

func (p *provider) convertMessage(msg ai.Message) []chatMessage {
	switch m := msg.(type) {
	case *ai.UserMessage:
		return []chatMessage{p.convertUserMessage(m)}
	case *ai.AssistantMessage:
		return []chatMessage{p.convertAssistantMessage(m)}
	case *ai.ToolResultMessage:
		return []chatMessage{p.convertToolResultMessage(m)}
	}
	return nil
}

func (p *provider) convertUserMessage(m *ai.UserMessage) chatMessage {
	// If the message is a single text block, send as a plain string.
	if len(m.Content) == 1 && m.Content[0].Type == ai.ContentText {
		return chatMessage{
			Role:    "user",
			Content: rawOrString(m.Content[0].Text),
		}
	}

	// Otherwise, send as a content array.
	var parts []contentPart
	for _, block := range m.Content {
		switch block.Type {
		case ai.ContentText:
			parts = append(parts, contentPart{
				Type: "text",
				Text: block.Text,
			})
		case ai.ContentImage:
			dataURL := "data:" + block.MimeType + ";base64," + block.Data
			parts = append(parts, contentPart{
				Type: "image_url",
				ImageURL: &imageURL{
					URL: dataURL,
				},
			})
		}
	}

	encoded, _ := json.Marshal(parts)
	return chatMessage{
		Role:    "user",
		Content: json.RawMessage(encoded),
	}
}

func (p *provider) convertAssistantMessage(m *ai.AssistantMessage) chatMessage {
	cm := chatMessage{
		Role: "assistant",
	}

	// Collect text content.
	var textParts []string
	for _, block := range m.Content {
		if block.Type == ai.ContentText {
			textParts = append(textParts, block.Text)
		}
	}
	if len(textParts) > 0 {
		cm.Content = rawOrString(strings.Join(textParts, ""))
	}

	// Collect tool calls.
	for _, block := range m.Content {
		if block.Type == ai.ContentToolCall {
			argsJSON, _ := json.Marshal(block.Arguments)
			cm.ToolCalls = append(cm.ToolCalls, toolCall{
				ID:   block.ID,
				Type: "function",
				Function: functionCall{
					Name:      block.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	return cm
}

func (p *provider) convertToolResultMessage(m *ai.ToolResultMessage) chatMessage {
	var text string
	for _, block := range m.Content {
		if block.Type == ai.ContentText {
			text += block.Text
		}
	}
	return chatMessage{
		Role:       "tool",
		Content:    rawOrString(text),
		ToolCallID: m.ToolCallID,
	}
}

// ---------- SSE stream processing ----------

func (p *provider) processSSEStream(ctx context.Context, body io.Reader, model *ai.Model, stream *ai.EventStream) {
	scanner := bufio.NewScanner(body)
	// Allow up to 1MB lines for large JSON payloads.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Accumulator state for building the final AssistantMessage.
	var (
		contentBlocks []ai.ContentBlock
		usage         ai.Usage
		stopReason    ai.StopReason
		started       bool
		// Track tool calls being incrementally built by index.
		toolCalls       = map[int]*toolCallAccumulator{}
		textAccumulated string
		textStarted     bool
		contentIndex    int
	)

	finalize := func() {
		// Flush any open text block.
		if textStarted {
			contentBlocks = append(contentBlocks, ai.TextBlock(textAccumulated))
			stream.Push(ai.StreamEvent{Type: ai.EventTextEnd, ContentIndex: contentIndex})
			contentIndex++
		}

		// Flush tool calls in order.
		indices := sortedToolCallIndices(toolCalls)
		for _, idx := range indices {
			tc := toolCalls[idx]
			var args map[string]any
			if tc.argumentsJSON != "" {
				_ = json.Unmarshal([]byte(tc.argumentsJSON), &args)
			}
			block := ai.ToolCallBlock(tc.id, tc.name, args)
			contentBlocks = append(contentBlocks, block)

			stream.Push(ai.StreamEvent{
				Type:         ai.EventToolCallEnd,
				ContentIndex: contentIndex,
				ToolCall:     &block,
			})
			contentIndex++
		}

		if stopReason == "" {
			stopReason = ai.StopReasonStop
		}

		finalMsg := &ai.AssistantMessage{
			Content:    contentBlocks,
			API:        model.API,
			Provider:   model.Provider,
			Model:      model.ID,
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

	for scanner.Scan() {
		// Check for context cancellation.
		if ctx.Err() != nil {
			p.emitAborted(stream, model)
			return
		}

		line := scanner.Text()

		// SSE lines that don't start with "data: " are either empty or comments; skip them.
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Terminal event.
		if data == "[DONE]" {
			finalize()
			return
		}

		var chunk sseChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Skip malformed chunks.
			continue
		}

		// Emit start event on the first chunk.
		if !started {
			started = true
			stream.Push(ai.StreamEvent{Type: ai.EventStart})
		}

		// Process usage from the chunk (typically in the final chunk).
		if chunk.Usage != nil {
			usage = ai.Usage{
				InputTokens:     chunk.Usage.PromptTokens,
				OutputTokens:    chunk.Usage.CompletionTokens,
				CacheReadTokens: chunk.Usage.PromptTokensDetails.CachedTokens,
			}
		}

		// Process each choice.
		for _, choice := range chunk.Choices {
			delta := choice.Delta

			// Text content delta.
			if delta.Content != "" {
				if !textStarted {
					textStarted = true
					stream.Push(ai.StreamEvent{Type: ai.EventTextStart, ContentIndex: contentIndex})
				}
				textAccumulated += delta.Content
				stream.Push(ai.StreamEvent{
					Type:         ai.EventTextDelta,
					ContentIndex: contentIndex,
					Delta:        delta.Content,
				})
			}

			// Tool call deltas.
			for _, tcDelta := range delta.ToolCalls {
				tc, exists := toolCalls[tcDelta.Index]
				if !exists {
					// New tool call starting. Close text block if open.
					if textStarted {
						contentBlocks = append(contentBlocks, ai.TextBlock(textAccumulated))
						stream.Push(ai.StreamEvent{Type: ai.EventTextEnd, ContentIndex: contentIndex})
						contentIndex++
						textAccumulated = ""
						textStarted = false
					}

					tc = &toolCallAccumulator{
						contentIndex: contentIndex,
					}
					toolCalls[tcDelta.Index] = tc

					stream.Push(ai.StreamEvent{
						Type:         ai.EventToolCallStart,
						ContentIndex: contentIndex,
					})
				}

				if tcDelta.ID != "" {
					tc.id = tcDelta.ID
				}
				if tcDelta.Function.Name != "" {
					tc.name = tcDelta.Function.Name
				}
				if tcDelta.Function.Arguments != "" {
					tc.argumentsJSON += tcDelta.Function.Arguments
					stream.Push(ai.StreamEvent{
						Type:         ai.EventToolCallDelta,
						ContentIndex: tc.contentIndex,
						Delta:        tcDelta.Function.Arguments,
					})
				}
			}

			// Map finish_reason.
			if choice.FinishReason != "" {
				switch choice.FinishReason {
				case "stop":
					stopReason = ai.StopReasonStop
				case "length":
					stopReason = ai.StopReasonLength
				case "tool_calls":
					stopReason = ai.StopReasonToolUse
				default:
					stopReason = ai.StopReasonStop
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			p.emitAborted(stream, model)
			return
		}
		p.emitError(stream, model, fmt.Sprintf("error reading SSE stream: %v", err))
		return
	}

	// If we got here without [DONE], finalize with what we have.
	if started {
		finalize()
	} else {
		p.emitError(stream, model, "empty response from OpenAI API")
	}
}

// ---------- Error/abort helpers ----------

func (p *provider) emitError(stream *ai.EventStream, model *ai.Model, message string) {
	errMsg := &ai.AssistantMessage{
		Model:        model.ID,
		Provider:     model.Provider,
		API:          model.API,
		StopReason:   ai.StopReasonError,
		ErrorMessage: message,
		Timestamp:    time.Now(),
	}
	stream.Push(ai.StreamEvent{Type: ai.EventError, Message: errMsg, Reason: ai.StopReasonError})
	stream.Error(errMsg)
}

func (p *provider) emitAborted(stream *ai.EventStream, model *ai.Model) {
	abortMsg := &ai.AssistantMessage{
		Model:      model.ID,
		Provider:   model.Provider,
		API:        model.API,
		StopReason: ai.StopReasonAborted,
		Timestamp:  time.Now(),
	}
	stream.Push(ai.StreamEvent{Type: ai.EventDone, Message: abortMsg, Reason: ai.StopReasonAborted})
	stream.End(abortMsg)
}

// ---------- Wire format types ----------

type requestBody struct {
	Model               string         `json:"model"`
	Messages            []chatMessage  `json:"messages"`
	Tools               []chatTool     `json:"tools,omitempty"`
	Stream              bool           `json:"stream"`
	StreamOptions       *streamOptions `json:"stream_options,omitempty"`
	MaxTokens           int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens int            `json:"max_completion_tokens,omitempty"`
	Temperature         *float64       `json:"temperature,omitempty"`
	ReasoningEffort     string         `json:"reasoning_effort,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content,omitempty"`
	ToolCalls  []toolCall      `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// SSE chunk types (response).

type sseChunk struct {
	Choices []sseChoice `json:"choices"`
	Usage   *sseUsage   `json:"usage,omitempty"`
}

type sseChoice struct {
	Delta        sseDelta `json:"delta"`
	FinishReason string   `json:"finish_reason,omitempty"`
}

type sseDelta struct {
	Role      string          `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []sseToolCall   `json:"tool_calls,omitempty"`
}

type sseToolCall struct {
	Index    int             `json:"index"`
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type,omitempty"`
	Function sseFunctionCall `json:"function"`
}

type sseFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type sseUsage struct {
	PromptTokens        int                  `json:"prompt_tokens"`
	CompletionTokens    int                  `json:"completion_tokens"`
	TotalTokens         int                  `json:"total_tokens"`
	PromptTokensDetails promptTokensDetails  `json:"prompt_tokens_details"`
}

type promptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// toolCallAccumulator incrementally builds a tool call from streaming deltas.
type toolCallAccumulator struct {
	id            string
	name          string
	argumentsJSON string
	contentIndex  int
}

// ---------- Utility functions ----------

// rawOrString converts a Go string to a json.RawMessage containing a JSON string.
func rawOrString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// toolParameterToJSON converts an ai.ToolParameter to a json.RawMessage (JSON Schema).
func toolParameterToJSON(tp ai.ToolParameter) (json.RawMessage, error) {
	b, err := json.Marshal(tp)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// mapThinkingLevel converts an ai.ThinkingLevel to an OpenAI reasoning_effort string.
func mapThinkingLevel(level ai.ThinkingLevel) string {
	switch level {
	case ai.ThinkingMinimal, ai.ThinkingLow:
		return "low"
	case ai.ThinkingMedium:
		return "medium"
	case ai.ThinkingHigh, ai.ThinkingXHigh:
		return "high"
	default:
		return "medium"
	}
}

// sortedToolCallIndices returns the tool call indices sorted in ascending order.
func sortedToolCallIndices(m map[int]*toolCallAccumulator) []int {
	if len(m) == 0 {
		return nil
	}
	// Find max index to avoid importing sort.
	max := 0
	for k := range m {
		if k > max {
			max = k
		}
	}
	var result []int
	for i := 0; i <= max; i++ {
		if _, ok := m[i]; ok {
			result = append(result, i)
		}
	}
	return result
}
