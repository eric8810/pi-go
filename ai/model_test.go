package ai

import (
	"testing"
)

// ---------------------------------------------------------------------------
// GetModel - known models
// ---------------------------------------------------------------------------

func TestGetModel_Anthropic(t *testing.T) {
	tests := []struct {
		id   string
		name string
	}{
		{"claude-opus-4-6", "Claude Opus 4.6"},
		{"claude-sonnet-4-5-20250929", "Claude Sonnet 4.5"},
		{"claude-haiku-4-5-20251001", "Claude Haiku 4.5"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			m, err := GetModel(ProviderAnthropic, tt.id)
			if err != nil {
				t.Fatalf("GetModel(%q, %q): %v", ProviderAnthropic, tt.id, err)
			}
			if m.Name != tt.name {
				t.Errorf("Name = %q, want %q", m.Name, tt.name)
			}
			if m.Provider != ProviderAnthropic {
				t.Errorf("Provider = %q, want %q", m.Provider, ProviderAnthropic)
			}
			if m.API != APIAnthropicMessages {
				t.Errorf("API = %q, want %q", m.API, APIAnthropicMessages)
			}
		})
	}
}

func TestGetModel_OpenAI(t *testing.T) {
	tests := []struct {
		id   string
		name string
	}{
		{"gpt-4o", "GPT-4o"},
		{"gpt-4o-mini", "GPT-4o Mini"},
		{"o3-mini", "o3-mini"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			m, err := GetModel(ProviderOpenAI, tt.id)
			if err != nil {
				t.Fatalf("GetModel(%q, %q): %v", ProviderOpenAI, tt.id, err)
			}
			if m.Name != tt.name {
				t.Errorf("Name = %q, want %q", m.Name, tt.name)
			}
			if m.Provider != ProviderOpenAI {
				t.Errorf("Provider = %q, want %q", m.Provider, ProviderOpenAI)
			}
		})
	}
}

func TestGetModel_Google(t *testing.T) {
	tests := []struct {
		id   string
		name string
	}{
		{"gemini-2.5-pro", "Gemini 2.5 Pro"},
		{"gemini-2.5-flash", "Gemini 2.5 Flash"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			m, err := GetModel(ProviderGoogle, tt.id)
			if err != nil {
				t.Fatalf("GetModel(%q, %q): %v", ProviderGoogle, tt.id, err)
			}
			if m.Name != tt.name {
				t.Errorf("Name = %q, want %q", m.Name, tt.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetModel - error cases
// ---------------------------------------------------------------------------

func TestGetModel_UnknownProvider(t *testing.T) {
	_, err := GetModel(Provider("nonexistent"), "whatever")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestGetModel_UnknownModelID(t *testing.T) {
	_, err := GetModel(ProviderAnthropic, "not-a-real-model")
	if err == nil {
		t.Fatal("expected error for unknown model ID")
	}
}

// ---------------------------------------------------------------------------
// ListModels
// ---------------------------------------------------------------------------

func TestListModels(t *testing.T) {
	models := ListModels(ProviderAnthropic)
	if len(models) < 3 {
		t.Errorf("expected at least 3 Anthropic models, got %d", len(models))
	}
	for _, m := range models {
		if m.Provider != ProviderAnthropic {
			t.Errorf("model %q has provider %q, want %q", m.ID, m.Provider, ProviderAnthropic)
		}
	}
}

func TestListModels_EmptyProvider(t *testing.T) {
	models := ListModels(Provider("never-registered"))
	if len(models) != 0 {
		t.Errorf("expected 0 models for unregistered provider, got %d", len(models))
	}
}

// ---------------------------------------------------------------------------
// ListProviders
// ---------------------------------------------------------------------------

func TestListProviders(t *testing.T) {
	providers := ListProviders()
	if len(providers) < 3 {
		t.Errorf("expected at least 3 providers, got %d", len(providers))
	}
	// Check that the known providers from init() are present.
	found := map[Provider]bool{}
	for _, p := range providers {
		found[p] = true
	}
	for _, want := range []Provider{ProviderAnthropic, ProviderOpenAI, ProviderGoogle} {
		if !found[want] {
			t.Errorf("provider %q not found in ListProviders()", want)
		}
	}
}

// ---------------------------------------------------------------------------
// RegisterModel
// ---------------------------------------------------------------------------

func TestRegisterModel(t *testing.T) {
	m := &Model{
		ID:            "test-model-register",
		Name:          "Test Model",
		API:           APIOpenAICompletions,
		Provider:      Provider("test-provider-register"),
		BaseURL:       "http://localhost:8080",
		InputTypes:    []string{"text"},
		ContextWindow: 4096,
		MaxTokens:     512,
	}
	RegisterModel(m)

	got, err := GetModel(Provider("test-provider-register"), "test-model-register")
	if err != nil {
		t.Fatalf("GetModel after register: %v", err)
	}
	if got.Name != "Test Model" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Model")
	}
	if got.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, "http://localhost:8080")
	}
}

func TestRegisterModel_OverwritesExisting(t *testing.T) {
	provider := Provider("test-overwrite-provider")
	m1 := &Model{
		ID:       "overwrite-model",
		Name:     "Version 1",
		Provider: provider,
	}
	RegisterModel(m1)

	m2 := &Model{
		ID:       "overwrite-model",
		Name:     "Version 2",
		Provider: provider,
	}
	RegisterModel(m2)

	got, err := GetModel(provider, "overwrite-model")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if got.Name != "Version 2" {
		t.Errorf("Name = %q, want %q (should have been overwritten)", got.Name, "Version 2")
	}
}

// ---------------------------------------------------------------------------
// CustomModel
// ---------------------------------------------------------------------------

func TestCustomModel(t *testing.T) {
	m := CustomModel("local-llama", "Local Llama", APIOpenAICompletions, "http://localhost:11434/v1")
	if m.ID != "local-llama" {
		t.Errorf("ID = %q, want %q", m.ID, "local-llama")
	}
	if m.Name != "Local Llama" {
		t.Errorf("Name = %q, want %q", m.Name, "Local Llama")
	}
	if m.API != APIOpenAICompletions {
		t.Errorf("API = %q, want %q", m.API, APIOpenAICompletions)
	}
	if m.Provider != "custom" {
		t.Errorf("Provider = %q, want %q", m.Provider, "custom")
	}
	if m.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("BaseURL = %q, want %q", m.BaseURL, "http://localhost:11434/v1")
	}
	if m.ContextWindow != 128000 {
		t.Errorf("ContextWindow = %d, want %d", m.ContextWindow, 128000)
	}
	if m.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want %d", m.MaxTokens, 4096)
	}
	if len(m.InputTypes) != 1 || m.InputTypes[0] != "text" {
		t.Errorf("InputTypes = %v, want [\"text\"]", m.InputTypes)
	}
}

// ---------------------------------------------------------------------------
// Model.SupportsImages
// ---------------------------------------------------------------------------

func TestModel_SupportsImages(t *testing.T) {
	tests := []struct {
		name       string
		inputTypes []string
		want       bool
	}{
		{"text and image", []string{"text", "image"}, true},
		{"text only", []string{"text"}, false},
		{"image only", []string{"image"}, true},
		{"empty", []string{}, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{InputTypes: tt.inputTypes}
			if got := m.SupportsImages(); got != tt.want {
				t.Errorf("SupportsImages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModel_SupportsImages_KnownModels(t *testing.T) {
	// claude-opus-4-6 supports images
	m, err := GetModel(ProviderAnthropic, "claude-opus-4-6")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if !m.SupportsImages() {
		t.Error("claude-opus-4-6 should support images")
	}

	// o3-mini does NOT support images
	m, err = GetModel(ProviderOpenAI, "o3-mini")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if m.SupportsImages() {
		t.Error("o3-mini should NOT support images")
	}
}

func TestModel_Properties(t *testing.T) {
	m, err := GetModel(ProviderAnthropic, "claude-opus-4-6")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if !m.Reasoning {
		t.Error("claude-opus-4-6 should have Reasoning = true")
	}
	if m.ContextWindow != 200000 {
		t.Errorf("ContextWindow = %d, want 200000", m.ContextWindow)
	}
	if m.MaxTokens != 32000 {
		t.Errorf("MaxTokens = %d, want 32000", m.MaxTokens)
	}
	if m.Cost.Input != 15.0 {
		t.Errorf("Cost.Input = %f, want 15.0", m.Cost.Input)
	}
}
