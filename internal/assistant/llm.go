// Package assistant provides AI-powered job creation assistance for Chronos.
package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LLMProvider represents supported LLM providers.
type LLMProvider string

const (
	ProviderOpenAI    LLMProvider = "openai"
	ProviderAnthropic LLMProvider = "anthropic"
	ProviderAzure     LLMProvider = "azure"
	ProviderLocal     LLMProvider = "local" // Ollama or similar
)

// LLMConfig configures the LLM assistant.
type LLMConfig struct {
	Provider    LLMProvider `json:"provider"`
	APIKey      string      `json:"api_key"`
	Model       string      `json:"model"`
	BaseURL     string      `json:"base_url,omitempty"`
	MaxTokens   int         `json:"max_tokens"`
	Temperature float64     `json:"temperature"`
	Timeout     time.Duration `json:"timeout"`
	
	// Rate limiting
	RequestsPerMinute int `json:"requests_per_minute"`
	
	// Caching
	CacheEnabled bool          `json:"cache_enabled"`
	CacheTTL     time.Duration `json:"cache_ttl"`
}

// DefaultLLMConfig returns sensible defaults.
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		Provider:          ProviderOpenAI,
		Model:             "gpt-4o-mini",
		MaxTokens:         1024,
		Temperature:       0.3,
		Timeout:           30 * time.Second,
		RequestsPerMinute: 60,
		CacheEnabled:      true,
		CacheTTL:          1 * time.Hour,
	}
}

// LLMAssistant provides LLM-powered job creation and troubleshooting.
type LLMAssistant struct {
	config     LLMConfig
	httpClient *http.Client
	cache      *responseCache
	rateLimiter *rateLimiter
	fallback   *Assistant // Regex-based fallback
	mu         sync.RWMutex
}

// NewLLMAssistant creates a new LLM-powered assistant.
func NewLLMAssistant(config LLMConfig) *LLMAssistant {
	if config.BaseURL == "" {
		switch config.Provider {
		case ProviderOpenAI:
			config.BaseURL = "https://api.openai.com/v1"
		case ProviderAnthropic:
			config.BaseURL = "https://api.anthropic.com/v1"
		case ProviderAzure:
			// Azure requires custom endpoint
		case ProviderLocal:
			config.BaseURL = "http://localhost:11434/api" // Ollama local development default
		}
	}

	return &LLMAssistant{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		cache:       newResponseCache(config.CacheTTL),
		rateLimiter: newRateLimiter(config.RequestsPerMinute),
		fallback:    New("UTC"),
	}
}

// ConversationContext maintains multi-turn conversation state.
type ConversationContext struct {
	ID        string          `json:"id"`
	Messages  []Message       `json:"messages"`
	JobDraft  *JobDraft       `json:"job_draft,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Message represents a conversation message.
type Message struct {
	Role      string    `json:"role"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// JobDraft represents a job being built through conversation.
type JobDraft struct {
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Schedule    string            `json:"schedule,omitempty"`
	Timezone    string            `json:"timezone,omitempty"`
	WebhookURL  string            `json:"webhook_url,omitempty"`
	Method      string            `json:"method,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	RetryPolicy *RetryPolicyDraft `json:"retry_policy,omitempty"`
	Confidence  float64           `json:"confidence"`
	Missing     []string          `json:"missing,omitempty"`
}

// RetryPolicyDraft represents retry configuration.
type RetryPolicyDraft struct {
	MaxAttempts     int    `json:"max_attempts"`
	InitialInterval string `json:"initial_interval"`
	Multiplier      float64 `json:"multiplier"`
}

// ParseWithLLM uses LLM to parse natural language into job configuration.
func (a *LLMAssistant) ParseWithLLM(ctx context.Context, input string) (*JobDraft, error) {
	// Check cache first
	if a.config.CacheEnabled {
		if cached, ok := a.cache.get(input); ok {
			return cached.(*JobDraft), nil
		}
	}

	// Rate limiting
	if err := a.rateLimiter.wait(ctx); err != nil {
		// Fall back to regex parser on rate limit
		result, err := a.fallback.Parse(input)
		if err != nil {
			return nil, err
		}
		return &JobDraft{
			Schedule:   result.Schedule,
			Timezone:   result.Timezone,
			Confidence: result.Confidence,
		}, nil
	}

	prompt := a.buildParsePrompt(input)
	response, err := a.callLLM(ctx, prompt)
	if err != nil {
		// Fall back to regex parser
		result, fallbackErr := a.fallback.Parse(input)
		if fallbackErr != nil {
			return nil, fmt.Errorf("LLM failed: %w, fallback failed: %v", err, fallbackErr)
		}
		return &JobDraft{
			Schedule:   result.Schedule,
			Timezone:   result.Timezone,
			Confidence: result.Confidence * 0.8, // Lower confidence for fallback
		}, nil
	}

	draft, err := a.parseJobDraftResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Cache successful response
	if a.config.CacheEnabled {
		a.cache.set(input, draft)
	}

	return draft, nil
}

// Chat handles multi-turn conversation for job creation.
func (a *LLMAssistant) Chat(ctx context.Context, conv *ConversationContext, userMessage string) (*ConversationContext, string, error) {
	if conv == nil {
		conv = &ConversationContext{
			ID:        generateID(),
			Messages:  []Message{},
			CreatedAt: time.Now(),
		}
	}

	// Add user message
	conv.Messages = append(conv.Messages, Message{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})

	// Build conversation prompt
	prompt := a.buildChatPrompt(conv)

	// Rate limiting
	if err := a.rateLimiter.wait(ctx); err != nil {
		return conv, "I'm currently rate limited. Please try again in a moment.", nil
	}

	response, err := a.callLLM(ctx, prompt)
	if err != nil {
		return conv, "I encountered an error processing your request. Could you try rephrasing?", err
	}

	// Parse structured response
	assistantReply, updatedDraft := a.parseChatResponse(response, conv.JobDraft)

	// Update conversation
	conv.Messages = append(conv.Messages, Message{
		Role:      "assistant",
		Content:   assistantReply,
		Timestamp: time.Now(),
	})
	conv.JobDraft = updatedDraft
	conv.UpdatedAt = time.Now()

	return conv, assistantReply, nil
}

// ExplainSchedule provides a detailed explanation of a cron expression.
func (a *LLMAssistant) ExplainSchedule(ctx context.Context, cronExpr string) (string, error) {
	// Try simple explanation first
	simple := ExplainCron(cronExpr)
	
	// For complex expressions, use LLM
	if strings.Contains(cronExpr, ",") || strings.Contains(cronExpr, "-") || strings.Contains(cronExpr, "L") || strings.Contains(cronExpr, "W") {
		prompt := fmt.Sprintf(`Explain this cron expression in plain English: "%s"

Provide:
1. When it runs (be specific with times and days)
2. Frequency (how often)
3. Next 3 example run times
4. Common use cases for this schedule

Be concise but thorough.`, cronExpr)

		response, err := a.callLLM(ctx, prompt)
		if err != nil {
			return simple, nil // Fall back to simple explanation
		}
		return response, nil
	}

	return simple, nil
}

// Troubleshoot analyzes job failures and suggests fixes.
func (a *LLMAssistant) Troubleshoot(ctx context.Context, req *TroubleshootRequest) (*TroubleshootResponse, error) {
	prompt := a.buildTroubleshootPrompt(req)

	response, err := a.callLLM(ctx, prompt)
	if err != nil {
		return &TroubleshootResponse{
			Analysis: "Unable to analyze at this time.",
			Suggestions: []TroubleshootSuggestion{
				{
					Title:       "Check logs",
					Description: "Review the job execution logs for error details.",
					Priority:    "high",
				},
			},
		}, nil
	}

	return a.parseTroubleshootResponse(response), nil
}

// TroubleshootRequest contains information about a failed job.
type TroubleshootRequest struct {
	JobID          string                 `json:"job_id"`
	JobName        string                 `json:"job_name"`
	Schedule       string                 `json:"schedule"`
	WebhookURL     string                 `json:"webhook_url"`
	LastError      string                 `json:"last_error"`
	LastStatusCode int                    `json:"last_status_code"`
	FailureCount   int                    `json:"failure_count"`
	AvgDuration    time.Duration          `json:"avg_duration"`
	LastDuration   time.Duration          `json:"last_duration"`
	RecentHistory  []ExecutionSummary     `json:"recent_history"`
	JobConfig      map[string]interface{} `json:"job_config"`
}

// ExecutionSummary summarizes a single execution.
type ExecutionSummary struct {
	Status     string        `json:"status"`
	StatusCode int           `json:"status_code"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
}

// TroubleshootResponse contains analysis and suggestions.
type TroubleshootResponse struct {
	Analysis       string                   `json:"analysis"`
	RootCause      string                   `json:"root_cause,omitempty"`
	Suggestions    []TroubleshootSuggestion `json:"suggestions"`
	AutoFixable    bool                     `json:"auto_fixable"`
	SuggestedFixes []SuggestedFix           `json:"suggested_fixes,omitempty"`
}

// TroubleshootSuggestion is a troubleshooting suggestion.
type TroubleshootSuggestion struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // "high", "medium", "low"
	Category    string `json:"category"` // "network", "auth", "timeout", "config"
}

// SuggestedFix is an automatic fix that can be applied.
type SuggestedFix struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	Changes     map[string]interface{} `json:"changes"`
	Risk        string                 `json:"risk"` // "low", "medium", "high"
}

// GenerateWorkflowFromDescription creates a workflow from natural language.
func (a *LLMAssistant) GenerateWorkflowFromDescription(ctx context.Context, description string) (*WorkflowSuggestion, error) {
	prompt := fmt.Sprintf(`Create a Chronos workflow based on this description:

"%s"

Generate a JSON workflow with:
1. Workflow name and description
2. List of jobs/steps with:
   - name
   - description
   - suggested schedule (cron expression)
   - dependencies (which jobs must complete first)
   - estimated duration
3. Overall workflow metadata

Return valid JSON matching this structure:
{
  "name": "workflow-name",
  "description": "what it does",
  "steps": [
    {
      "name": "step-name",
      "description": "what this step does",
      "schedule": "cron expression or 'triggered'",
      "dependencies": ["previous-step-name"],
      "estimated_duration": "5m"
    }
  ],
  "triggers": ["schedule", "webhook", "manual"],
  "estimated_total_duration": "30m"
}`, description)

	response, err := a.callLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate workflow: %w", err)
	}

	return a.parseWorkflowResponse(response)
}

// WorkflowSuggestion represents a suggested workflow.
type WorkflowSuggestion struct {
	Name                   string         `json:"name"`
	Description            string         `json:"description"`
	Steps                  []WorkflowStep `json:"steps"`
	Triggers               []string       `json:"triggers"`
	EstimatedTotalDuration string         `json:"estimated_total_duration"`
	Confidence             float64        `json:"confidence"`
}

// WorkflowStep is a step in a suggested workflow.
type WorkflowStep struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Schedule          string   `json:"schedule"`
	Dependencies      []string `json:"dependencies"`
	EstimatedDuration string   `json:"estimated_duration"`
}

// Internal methods

func (a *LLMAssistant) buildParsePrompt(input string) string {
	return fmt.Sprintf(`You are a cron schedule parsing assistant for Chronos, a distributed job scheduler.

Parse the following natural language description into a job configuration:

"%s"

Extract:
1. schedule: A valid cron expression (5 fields: minute hour day month weekday)
2. timezone: IANA timezone (e.g., "America/New_York", "UTC")
3. name: A suggested job name (lowercase, hyphens, no spaces)
4. description: A brief description of what the job does
5. confidence: Your confidence in the parsing (0.0 to 1.0)
6. missing: List of any information that would help (e.g., "specific time", "target URL")

Common cron patterns:
- Every minute: * * * * *
- Every 5 minutes: */5 * * * *
- Every hour: 0 * * * *
- Daily at midnight: 0 0 * * *
- Daily at 9 AM: 0 9 * * *
- Every Monday at 9 AM: 0 9 * * 1
- First of month: 0 0 1 * *

Return ONLY valid JSON:
{
  "schedule": "cron expression",
  "timezone": "timezone",
  "name": "suggested-name",
  "description": "description",
  "confidence": 0.95,
  "missing": []
}`, input)
}

func (a *LLMAssistant) buildChatPrompt(conv *ConversationContext) string {
	var sb strings.Builder

	sb.WriteString(`You are a helpful assistant for Chronos, a distributed job scheduler.
Help users create jobs through conversation. Extract job details incrementally.

Current job draft:
`)
	if conv.JobDraft != nil {
		draftJSON, _ := json.MarshalIndent(conv.JobDraft, "", "  ")
		sb.WriteString(string(draftJSON))
	} else {
		sb.WriteString("{}")
	}

	sb.WriteString("\n\nConversation history:\n")
	for _, msg := range conv.Messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	sb.WriteString(`
Respond with:
1. A natural language reply to the user
2. Updated job draft JSON (if any new information was provided)

Format your response as:
REPLY: [your conversational response]
DRAFT: [updated JSON or "unchanged"]`)

	return sb.String()
}

func (a *LLMAssistant) buildTroubleshootPrompt(req *TroubleshootRequest) string {
	historyJSON, _ := json.Marshal(req.RecentHistory)

	return fmt.Sprintf(`Analyze this failing Chronos job and suggest fixes:

Job: %s (%s)
Schedule: %s
Webhook: %s
Last Error: %s
Last Status Code: %d
Failure Count: %d
Average Duration: %s
Last Duration: %s

Recent execution history:
%s

Analyze the failure pattern and provide:
1. Root cause analysis
2. Specific suggestions ranked by priority
3. Any automatic fixes that could be applied

Return JSON:
{
  "analysis": "detailed analysis",
  "root_cause": "most likely cause",
  "suggestions": [
    {"title": "...", "description": "...", "priority": "high/medium/low", "category": "network/auth/timeout/config"}
  ],
  "auto_fixable": true/false,
  "suggested_fixes": [
    {"id": "fix-1", "description": "...", "changes": {...}, "risk": "low/medium/high"}
  ]
}`,
		req.JobName, req.JobID,
		req.Schedule,
		req.WebhookURL,
		req.LastError,
		req.LastStatusCode,
		req.FailureCount,
		req.AvgDuration,
		req.LastDuration,
		string(historyJSON))
}

func (a *LLMAssistant) callLLM(ctx context.Context, prompt string) (string, error) {
	switch a.config.Provider {
	case ProviderOpenAI:
		return a.callOpenAI(ctx, prompt)
	case ProviderAnthropic:
		return a.callAnthropic(ctx, prompt)
	case ProviderAzure:
		return a.callAzureOpenAI(ctx, prompt)
	case ProviderLocal:
		return a.callOllama(ctx, prompt)
	default:
		return "", fmt.Errorf("unsupported provider: %s", a.config.Provider)
	}
}

func (a *LLMAssistant) callOpenAI(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": a.config.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  a.config.MaxTokens,
		"temperature": a.config.Temperature,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.config.APIKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", errors.New("no response from OpenAI")
	}

	return result.Choices[0].Message.Content, nil
}

func (a *LLMAssistant) callAnthropic(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":      a.config.Model,
		"max_tokens": a.config.MaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Anthropic API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Content) == 0 {
		return "", errors.New("no response from Anthropic")
	}

	return result.Content[0].Text, nil
}

func (a *LLMAssistant) callAzureOpenAI(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  a.config.MaxTokens,
		"temperature": a.config.Temperature,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=2024-02-15-preview",
		a.config.BaseURL, a.config.Model)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", a.config.APIKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Azure OpenAI API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", errors.New("no response from Azure OpenAI")
	}

	return result.Choices[0].Message.Content, nil
}

func (a *LLMAssistant) callOllama(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":  a.config.Model,
		"prompt": prompt,
		"stream": false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL+"/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Response, nil
}

func (a *LLMAssistant) parseJobDraftResponse(response string) (*JobDraft, error) {
	// Extract JSON from response (might have markdown code blocks)
	jsonStr := extractJSON(response)

	var draft JobDraft
	if err := json.Unmarshal([]byte(jsonStr), &draft); err != nil {
		return nil, fmt.Errorf("invalid JSON in response: %w", err)
	}

	// Validate the schedule
	if draft.Schedule != "" {
		if err := ValidateCron(draft.Schedule); err != nil {
			draft.Confidence *= 0.5
			draft.Missing = append(draft.Missing, "schedule may be invalid")
		}
	}

	return &draft, nil
}

func (a *LLMAssistant) parseChatResponse(response string, currentDraft *JobDraft) (string, *JobDraft) {
	// Parse REPLY and DRAFT sections
	reply := response
	draft := currentDraft

	if idx := strings.Index(response, "REPLY:"); idx >= 0 {
		endIdx := strings.Index(response, "DRAFT:")
		if endIdx > idx {
			reply = strings.TrimSpace(response[idx+6 : endIdx])
		} else {
			reply = strings.TrimSpace(response[idx+6:])
		}
	}

	if idx := strings.Index(response, "DRAFT:"); idx >= 0 {
		draftStr := strings.TrimSpace(response[idx+6:])
		if draftStr != "unchanged" && draftStr != "" {
			jsonStr := extractJSON(draftStr)
			var newDraft JobDraft
			if err := json.Unmarshal([]byte(jsonStr), &newDraft); err == nil {
				draft = &newDraft
			}
		}
	}

	return reply, draft
}

func (a *LLMAssistant) parseTroubleshootResponse(response string) *TroubleshootResponse {
	jsonStr := extractJSON(response)

	var result TroubleshootResponse
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return &TroubleshootResponse{
			Analysis: response,
			Suggestions: []TroubleshootSuggestion{
				{
					Title:       "Review error details",
					Description: "Check the execution logs for more information.",
					Priority:    "high",
				},
			},
		}
	}

	return &result
}

func (a *LLMAssistant) parseWorkflowResponse(response string) (*WorkflowSuggestion, error) {
	jsonStr := extractJSON(response)

	var workflow WorkflowSuggestion
	if err := json.Unmarshal([]byte(jsonStr), &workflow); err != nil {
		return nil, fmt.Errorf("invalid workflow JSON: %w", err)
	}

	workflow.Confidence = 0.85
	return &workflow, nil
}

// Helper functions

func extractJSON(s string) string {
	// Remove markdown code blocks
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// Find JSON object
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}

	return s
}

func generateID() string {
	return fmt.Sprintf("conv-%d", time.Now().UnixNano())
}

// Response cache

type responseCache struct {
	items map[string]cacheItem
	ttl   time.Duration
	mu    sync.RWMutex
}

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

func newResponseCache(ttl time.Duration) *responseCache {
	return &responseCache{
		items: make(map[string]cacheItem),
		ttl:   ttl,
	}
}

func (c *responseCache) get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

func (c *responseCache) set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Rate limiter

type rateLimiter struct {
	requestsPerMinute int
	tokens            chan struct{}
	mu                sync.Mutex
}

func newRateLimiter(rpm int) *rateLimiter {
	if rpm <= 0 {
		rpm = 60 // Default to 60 requests per minute
	}
	rl := &rateLimiter{
		requestsPerMinute: rpm,
		tokens:            make(chan struct{}, rpm),
	}

	// Fill initial tokens
	for i := 0; i < rpm; i++ {
		rl.tokens <- struct{}{}
	}

	// Refill tokens
	go func() {
		ticker := time.NewTicker(time.Minute / time.Duration(rpm))
		defer ticker.Stop()
		for range ticker.C {
			select {
			case rl.tokens <- struct{}{}:
			default:
			}
		}
	}()

	return rl
}

func (rl *rateLimiter) wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return errors.New("rate limit exceeded")
	}
}
