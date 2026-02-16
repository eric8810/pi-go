package ai

import "fmt"

// Model describes an LLM model with its capabilities and pricing.
type Model struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	API           API      `json:"api"`
	Provider      Provider `json:"provider"`
	BaseURL       string   `json:"baseUrl"`
	Reasoning     bool     `json:"reasoning"`
	InputTypes    []string `json:"input"` // "text", "image"
	Cost          Cost     `json:"cost"`
	ContextWindow int      `json:"contextWindow"`
	MaxTokens     int      `json:"maxTokens"`
	Headers       map[string]string `json:"headers,omitempty"`
}

// SupportsImages returns true if the model accepts image input.
func (m *Model) SupportsImages() bool {
	for _, t := range m.InputTypes {
		if t == "image" {
			return true
		}
	}
	return false
}

// modelRegistry stores all registered models indexed by provider then model ID.
var modelRegistry = map[Provider]map[string]*Model{}

// RegisterModel adds a model to the global registry.
func RegisterModel(m *Model) {
	if modelRegistry[m.Provider] == nil {
		modelRegistry[m.Provider] = map[string]*Model{}
	}
	modelRegistry[m.Provider][m.ID] = m
}

// GetModel retrieves a model by provider and model ID.
func GetModel(provider Provider, modelID string) (*Model, error) {
	providerModels, ok := modelRegistry[provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
	model, ok := providerModels[modelID]
	if !ok {
		return nil, fmt.Errorf("unknown model %s for provider %s", modelID, provider)
	}
	return model, nil
}

// ListModels returns all models for a given provider.
func ListModels(provider Provider) []*Model {
	var models []*Model
	for _, m := range modelRegistry[provider] {
		models = append(models, m)
	}
	return models
}

// ListProviders returns all registered providers.
func ListProviders() []Provider {
	var providers []Provider
	for p := range modelRegistry {
		providers = append(providers, p)
	}
	return providers
}

// CustomModel creates a Model for a custom/self-hosted endpoint.
func CustomModel(id, name string, api API, baseURL string) *Model {
	return &Model{
		ID:            id,
		Name:          name,
		API:           api,
		Provider:      "custom",
		BaseURL:       baseURL,
		InputTypes:    []string{"text"},
		ContextWindow: 128000,
		MaxTokens:     4096,
	}
}

func init() {
	// Anthropic models
	for _, m := range []*Model{
		{
			ID: "claude-opus-4-6", Name: "Claude Opus 4.6",
			API: APIAnthropicMessages, Provider: ProviderAnthropic,
			BaseURL: "https://api.anthropic.com", Reasoning: true,
			InputTypes: []string{"text", "image"}, ContextWindow: 200000, MaxTokens: 32000,
			Cost: Cost{Input: 15.0, Output: 75.0, CacheRead: 1.5, CacheWrite: 18.75},
		},
		{
			ID: "claude-sonnet-4-5-20250929", Name: "Claude Sonnet 4.5",
			API: APIAnthropicMessages, Provider: ProviderAnthropic,
			BaseURL: "https://api.anthropic.com", Reasoning: true,
			InputTypes: []string{"text", "image"}, ContextWindow: 200000, MaxTokens: 16384,
			Cost: Cost{Input: 3.0, Output: 15.0, CacheRead: 0.3, CacheWrite: 3.75},
		},
		{
			ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5",
			API: APIAnthropicMessages, Provider: ProviderAnthropic,
			BaseURL: "https://api.anthropic.com", Reasoning: false,
			InputTypes: []string{"text", "image"}, ContextWindow: 200000, MaxTokens: 8192,
			Cost: Cost{Input: 0.8, Output: 4.0, CacheRead: 0.08, CacheWrite: 1.0},
		},
	} {
		RegisterModel(m)
	}

	// OpenAI models
	for _, m := range []*Model{
		{
			ID: "gpt-4o", Name: "GPT-4o",
			API: APIOpenAICompletions, Provider: ProviderOpenAI,
			BaseURL: "https://api.openai.com/v1", Reasoning: false,
			InputTypes: []string{"text", "image"}, ContextWindow: 128000, MaxTokens: 16384,
			Cost: Cost{Input: 2.5, Output: 10.0},
		},
		{
			ID: "gpt-4o-mini", Name: "GPT-4o Mini",
			API: APIOpenAICompletions, Provider: ProviderOpenAI,
			BaseURL: "https://api.openai.com/v1", Reasoning: false,
			InputTypes: []string{"text", "image"}, ContextWindow: 128000, MaxTokens: 16384,
			Cost: Cost{Input: 0.15, Output: 0.6},
		},
		{
			ID: "o3-mini", Name: "o3-mini",
			API: APIOpenAICompletions, Provider: ProviderOpenAI,
			BaseURL: "https://api.openai.com/v1", Reasoning: true,
			InputTypes: []string{"text"}, ContextWindow: 200000, MaxTokens: 100000,
			Cost: Cost{Input: 1.1, Output: 4.4},
		},
	} {
		RegisterModel(m)
	}

	// Google models
	for _, m := range []*Model{
		{
			ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro",
			API: APIGoogleGenerativeAI, Provider: ProviderGoogle,
			BaseURL: "https://generativelanguage.googleapis.com", Reasoning: true,
			InputTypes: []string{"text", "image"}, ContextWindow: 1000000, MaxTokens: 65536,
			Cost: Cost{Input: 1.25, Output: 10.0},
		},
		{
			ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash",
			API: APIGoogleGenerativeAI, Provider: ProviderGoogle,
			BaseURL: "https://generativelanguage.googleapis.com", Reasoning: true,
			InputTypes: []string{"text", "image"}, ContextWindow: 1000000, MaxTokens: 65536,
			Cost: Cost{Input: 0.15, Output: 0.6},
		},
	} {
		RegisterModel(m)
	}
}
