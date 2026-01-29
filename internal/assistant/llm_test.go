package assistant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewLLMAssistant(t *testing.T) {
	config := LLMConfig{
		Provider:    ProviderOpenAI,
		APIKey:      "test-key",
		Model:       "gpt-4",
		MaxTokens:   1000,
		Temperature: 0.7,
	}

	assistant := NewLLMAssistant(config)
	if assistant == nil {
		t.Fatal("expected non-nil assistant")
	}
	if assistant.config.Provider != ProviderOpenAI {
		t.Errorf("expected provider %s, got %s", ProviderOpenAI, assistant.config.Provider)
	}
}

func TestDefaultLLMConfig(t *testing.T) {
	config := DefaultLLMConfig()
	if config.MaxTokens != 1024 {
		t.Errorf("expected default max tokens 1024, got %d", config.MaxTokens)
	}
	if config.Temperature != 0.3 {
		t.Errorf("expected default temperature 0.3, got %f", config.Temperature)
	}
	if config.Provider != ProviderOpenAI {
		t.Errorf("expected default provider OpenAI, got %s", config.Provider)
	}
}

func TestParseWithLLM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"name\": \"daily-backup\", \"schedule\": \"0 2 * * *\", \"description\": \"Daily backup job\"}"
				}
			}]
		}`))
	}))
	defer server.Close()

	config := LLMConfig{
		Provider: ProviderOpenAI,
		APIKey:   "test-key",
		Model:    "gpt-4",
		BaseURL:  server.URL,
	}

	assistant := NewLLMAssistant(config)
	ctx := context.Background()

	result, err := assistant.ParseWithLLM(ctx, "Create a daily backup job at 2am")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "daily-backup" {
		t.Errorf("expected name 'daily-backup', got '%s'", result.Name)
	}
	if result.Schedule != "0 2 * * *" {
		t.Errorf("expected schedule '0 2 * * *', got '%s'", result.Schedule)
	}
}

func TestChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "To run a backup every day at 2am, use the cron expression: 0 2 * * *"
				}
			}]
		}`))
	}))
	defer server.Close()

	config := LLMConfig{
		Provider: ProviderOpenAI,
		APIKey:   "test-key",
		BaseURL:  server.URL,
	}

	assistant := NewLLMAssistant(config)
	ctx := context.Background()

	conv := &ConversationContext{ID: "test"}
	_, response, err := assistant.Chat(ctx, conv, "How do I schedule a daily backup?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(response, "0 2 * * *") {
		t.Errorf("expected response to contain cron expression, got: %s", response)
	}
}

func TestLLMProviders(t *testing.T) {
	providers := []LLMProvider{
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderAzure,
		ProviderLocal,
	}

	for _, p := range providers {
		if p == "" {
			t.Error("empty provider")
		}
	}
}

func TestTroubleshoot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "1. Increase timeout\n2. Check network\n3. Review logs"
				}
			}]
		}`))
	}))
	defer server.Close()

	config := LLMConfig{
		Provider: ProviderOpenAI,
		APIKey:   "test-key",
		BaseURL:  server.URL,
	}

	assistant := NewLLMAssistant(config)
	ctx := context.Background()

	req := &TroubleshootRequest{
		JobID:     "job-123",
		LastError: "timeout error",
	}
	response, err := assistant.Troubleshoot(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response == nil {
		t.Error("expected response")
	}
}
