package ai

import (
	"context"
	"sync"
)

// APIProvider implements streaming for a specific LLM API wire protocol.
type APIProvider interface {
	// API returns the API identifier this provider handles.
	API() API

	// Stream initiates a streaming LLM call.
	Stream(ctx context.Context, model *Model, reqCtx *Context, opts StreamOptions) *EventStream
}

var (
	providersMu sync.RWMutex
	providers   = map[API]APIProvider{}
)

// RegisterAPIProvider registers a provider for a specific API.
func RegisterAPIProvider(p APIProvider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[p.API()] = p
}

// GetAPIProvider retrieves the provider for a given API.
func GetAPIProvider(api API) APIProvider {
	providersMu.RLock()
	defer providersMu.RUnlock()
	return providers[api]
}

// MessagesToLLM converts Message interface slices to a format suitable for serialization.
// It filters out nil messages and returns the appropriate wire representations.
func MessagesToLLM(messages []Message) []MessageJSON {
	var result []MessageJSON
	for _, m := range messages {
		if m == nil {
			continue
		}
		switch msg := m.(type) {
		case *UserMessage:
			result = append(result, MessageJSON{
				Role:      RoleUser,
				Content:   msg.Content,
				Timestamp: msg.Timestamp,
			})
		case *AssistantMessage:
			result = append(result, MessageJSON{
				Role:         RoleAssistant,
				Content:      msg.Content,
				API:          msg.API,
				Provider:     msg.Provider,
				Model:        msg.Model,
				Usage:        &msg.Usage,
				StopReason:   msg.StopReason,
				ErrorMessage: msg.ErrorMessage,
				Timestamp:    msg.Timestamp,
			})
		case *ToolResultMessage:
			result = append(result, MessageJSON{
				Role:       RoleToolResult,
				Content:    msg.Content,
				ToolCallID: msg.ToolCallID,
				ToolName:   msg.ToolName,
				IsError:    msg.IsError,
				Timestamp:  msg.Timestamp,
			})
		}
	}
	return result
}
