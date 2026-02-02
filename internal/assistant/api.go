// Package assistant provides NLP API handlers.
package assistant

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// APIHandler handles NLP assistant API requests.
type APIHandler struct {
	llm           *LLMAssistant
	fallback      *Assistant
	conversations sync.Map // map[string]*ConversationContext
}

// NewAPIHandler creates a new NLP API handler.
func NewAPIHandler(llm *LLMAssistant) *APIHandler {
	return &APIHandler{
		llm:      llm,
		fallback: New("UTC"),
	}
}

// APIResponse is the standard API response format.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Routes returns the chi router with all NLP routes.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Parse natural language
	r.Post("/parse", h.Parse)
	r.Post("/parse-simple", h.ParseSimple)

	// Chat-based job creation
	r.Post("/conversations", h.StartConversation)
	r.Get("/conversations/{conversationID}", h.GetConversation)
	r.Post("/conversations/{conversationID}/messages", h.SendMessage)
	r.Delete("/conversations/{conversationID}", h.EndConversation)
	r.Post("/conversations/{conversationID}/finalize", h.FinalizeJob)

	// Schedule helpers
	r.Post("/explain-schedule", h.ExplainSchedule)
	r.Post("/suggest-schedule", h.SuggestSchedule)
	r.Post("/validate-schedule", h.ValidateSchedule)

	// Troubleshooting
	r.Post("/troubleshoot", h.Troubleshoot)
	r.Post("/suggest-fix", h.SuggestFix)

	// Templates and suggestions
	r.Get("/templates", h.GetTemplates)
	r.Post("/from-template", h.CreateFromTemplate)

	return r
}

// ParseRequest is a request to parse natural language.
type ParseRequest struct {
	Input    string `json:"input"`
	Timezone string `json:"timezone,omitempty"`
}

// Parse parses natural language using LLM.
func (h *APIHandler) Parse(w http.ResponseWriter, r *http.Request) {
	var req ParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	if req.Input == "" {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "input is required"})
		return
	}

	if h.llm != nil {
		draft, err := h.llm.ParseWithLLM(r.Context(), req.Input)
		if err == nil && draft != nil {
			h.writeJSON(w, http.StatusOK, APIResponse{
				Success: true,
				Data: map[string]interface{}{
					"draft":    draft,
					"provider": "llm",
				},
			})
			return
		}
	}

	// Fall back to regex parser
	result, err := h.fallback.Parse(req.Input)
	if err != nil {
		h.writeJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"message":  "Could not parse the input. Please try rephrasing.",
				"provider": "fallback",
			},
		})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"schedule":   result.Schedule,
			"timezone":   result.Timezone,
			"confidence": result.Confidence,
			"provider":   "regex",
		},
	})
}

// ParseSimple uses regex-based parsing only.
func (h *APIHandler) ParseSimple(w http.ResponseWriter, r *http.Request) {
	var req ParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	result, err := h.fallback.Parse(req.Input)
	if err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// StartConversation starts a new chat conversation.
func (h *APIHandler) StartConversation(w http.ResponseWriter, r *http.Request) {
	conv := &ConversationContext{
		ID:        uuid.New().String(),
		Messages:  []Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Add system message
	conv.Messages = append(conv.Messages, Message{
		Role:      "system",
		Content:   "Hi! I'm your job creation assistant. Tell me what you want to schedule and I'll help you set it up. You can say things like 'Run my backup job every night at midnight' or 'Send a report every Monday at 9am'.",
		Timestamp: time.Now(),
	})

	h.conversations.Store(conv.ID, conv)

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    conv,
	})
}

// GetConversation retrieves a conversation.
func (h *APIHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "conversationID")
	
	conv, ok := h.conversations.Load(convID)
	if !ok {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: "conversation not found"})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: conv})
}

// MessageRequest is a request to send a message.
type MessageRequest struct {
	Message string `json:"message"`
}

// SendMessage sends a message in a conversation.
func (h *APIHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "conversationID")

	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	convAny, ok := h.conversations.Load(convID)
	if !ok {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: "conversation not found"})
		return
	}

	conv := convAny.(*ConversationContext)
	var reply string
	var err error

	if h.llm != nil {
		conv, reply, err = h.llm.Chat(r.Context(), conv, req.Message)
		if err != nil {
			reply = "I had trouble understanding that. Could you try rephrasing?"
		}
	} else {
		// Simple fallback without LLM
		result, parseErr := h.fallback.Parse(req.Message)
		if parseErr == nil {
			if conv.JobDraft == nil {
				conv.JobDraft = &JobDraft{}
			}
			conv.JobDraft.Schedule = result.Schedule
			conv.JobDraft.Timezone = result.Timezone
			conv.JobDraft.Confidence = result.Confidence
			reply = "I understood that as: " + ExplainCron(result.Schedule) + "\n\nIs this correct? If so, what webhook URL should I call?"
		} else {
			reply = "I couldn't parse a schedule from that. Try something like 'every day at 9am' or 'every Monday at 3pm'."
		}

		conv.Messages = append(conv.Messages, Message{
			Role:      "user",
			Content:   req.Message,
			Timestamp: time.Now(),
		})
		conv.Messages = append(conv.Messages, Message{
			Role:      "assistant",
			Content:   reply,
			Timestamp: time.Now(),
		})
		conv.UpdatedAt = time.Now()
	}

	h.conversations.Store(convID, conv)

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"reply":     reply,
			"job_draft": conv.JobDraft,
		},
	})
}

// EndConversation ends and removes a conversation.
func (h *APIHandler) EndConversation(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "conversationID")
	h.conversations.Delete(convID)
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true})
}

// FinalizeJob finalizes the job draft from a conversation.
func (h *APIHandler) FinalizeJob(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "conversationID")

	convAny, ok := h.conversations.Load(convID)
	if !ok {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: "conversation not found"})
		return
	}

	conv := convAny.(*ConversationContext)
	if conv.JobDraft == nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "no job draft in conversation"})
		return
	}

	// Validate required fields
	missing := []string{}
	if conv.JobDraft.Schedule == "" {
		missing = append(missing, "schedule")
	}
	if conv.JobDraft.WebhookURL == "" {
		missing = append(missing, "webhook_url")
	}

	if len(missing) > 0 {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{
			Error: "missing required fields",
			Data: map[string]interface{}{
				"missing": missing,
			},
		})
		return
	}

	// Return finalized job config
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"job":    conv.JobDraft,
			"status": "ready_to_create",
		},
	})
}

// ExplainScheduleRequest is a request to explain a schedule.
type ExplainScheduleRequest struct {
	Schedule string `json:"schedule"`
}

// ExplainSchedule explains a cron schedule.
func (h *APIHandler) ExplainSchedule(w http.ResponseWriter, r *http.Request) {
	var req ExplainScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	var explanation string
	if h.llm != nil {
		var err error
		explanation, err = h.llm.ExplainSchedule(r.Context(), req.Schedule)
		if err != nil {
			explanation = ExplainCron(req.Schedule)
		}
	} else {
		explanation = ExplainCron(req.Schedule)
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"schedule":    req.Schedule,
			"explanation": explanation,
		},
	})
}

// SuggestScheduleRequest is a request to suggest a schedule.
type SuggestScheduleRequest struct {
	Description string `json:"description"`
}

// SuggestSchedule suggests schedules based on description.
func (h *APIHandler) SuggestSchedule(w http.ResponseWriter, r *http.Request) {
	var req SuggestScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	suggestions := []map[string]string{}

	// Use LLM if available
	if h.llm != nil {
		draft, err := h.llm.ParseWithLLM(r.Context(), req.Description)
		if err == nil && draft != nil && draft.Schedule != "" {
			suggestions = append(suggestions, map[string]string{
				"schedule":    draft.Schedule,
				"explanation": ExplainCron(draft.Schedule),
				"source":      "llm",
			})
		}
	}

	// Also try regex parser
	result, err := h.fallback.Parse(req.Description)
	if err == nil {
		// Avoid duplicates
		found := false
		for _, s := range suggestions {
			if s["schedule"] == result.Schedule {
				found = true
				break
			}
		}
		if !found {
			suggestions = append(suggestions, map[string]string{
				"schedule":    result.Schedule,
				"explanation": ExplainCron(result.Schedule),
				"source":      "regex",
			})
		}
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"suggestions": suggestions,
		},
	})
}

// ValidateScheduleRequest is a request to validate a schedule.
type ValidateScheduleRequest struct {
	Schedule string `json:"schedule"`
}

// ValidateSchedule validates a cron schedule.
func (h *APIHandler) ValidateSchedule(w http.ResponseWriter, r *http.Request) {
	var req ValidateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	err := ValidateCron(req.Schedule)
	valid := err == nil
	explanation := ""
	if valid {
		explanation = ExplainCron(req.Schedule)
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"schedule":    req.Schedule,
			"valid":       valid,
			"explanation": explanation,
		},
	})
}

// TroubleshootAPIRequest is a request to troubleshoot an error.
type TroubleshootAPIRequest struct {
	JobID       string `json:"job_id,omitempty"`
	ErrorType   string `json:"error_type"`
	ErrorMessage string `json:"error_message"`
	StatusCode  int    `json:"status_code,omitempty"`
}

// Troubleshoot provides troubleshooting suggestions.
func (h *APIHandler) Troubleshoot(w http.ResponseWriter, r *http.Request) {
	var req TroubleshootAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	suggestions := []string{}

	// Basic troubleshooting based on error type
	switch {
	case req.StatusCode >= 500:
		suggestions = append(suggestions, 
			"The webhook endpoint is returning server errors",
			"Check if the target server is running and healthy",
			"Review server logs for the root cause",
			"Consider increasing timeout if the endpoint is slow")
	case req.StatusCode == 404:
		suggestions = append(suggestions,
			"The webhook URL may be incorrect",
			"Verify the endpoint path is correct",
			"Check if the endpoint has been moved or removed")
	case req.StatusCode == 401 || req.StatusCode == 403:
		suggestions = append(suggestions,
			"Authentication is required or failing",
			"Check your API key or credentials",
			"Verify the authorization header is correct")
	case req.StatusCode == 429:
		suggestions = append(suggestions,
			"Rate limit exceeded on the target endpoint",
			"Add backoff/retry logic",
			"Reduce execution frequency")
	case req.ErrorType == "timeout":
		suggestions = append(suggestions,
			"The webhook took too long to respond",
			"Increase the job timeout",
			"Check if the endpoint is overloaded")
	case req.ErrorType == "connection":
		suggestions = append(suggestions,
			"Could not connect to the webhook",
			"Verify the URL is accessible",
			"Check firewall rules and network connectivity")
	default:
		suggestions = append(suggestions,
			"Check the webhook URL is correct",
			"Verify the endpoint is accessible",
			"Review the job configuration")
	}

	// Use LLM for detailed troubleshooting if available
	if h.llm != nil {
		llmReq := &TroubleshootRequest{
			JobID:          req.JobID,
			LastError:      req.ErrorMessage,
			LastStatusCode: req.StatusCode,
		}
		llmResponse, err := h.llm.Troubleshoot(r.Context(), llmReq)
		if err == nil && llmResponse != nil && len(llmResponse.Suggestions) > 0 {
			for _, s := range llmResponse.Suggestions {
				suggestions = append([]string{s.Title + ": " + s.Description}, suggestions...)
			}
		}
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"suggestions": suggestions,
		},
	})
}

// SuggestFixAPIRequest is a request to suggest a fix.
type SuggestFixAPIRequest struct {
	Problem string `json:"problem"`
	Context string `json:"context,omitempty"`
}

// SuggestFix suggests fixes for a problem.
func (h *APIHandler) SuggestFix(w http.ResponseWriter, r *http.Request) {
	var req SuggestFixAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	// Generic suggestions
	suggestions := []string{
		"Review the job configuration",
		"Check webhook endpoint health",
		"Verify credentials and permissions",
	}

	if h.llm != nil {
		llmReq := &TroubleshootRequest{
			LastError: req.Problem,
		}
		llmResponse, err := h.llm.Troubleshoot(r.Context(), llmReq)
		if err == nil && llmResponse != nil && len(llmResponse.Suggestions) > 0 {
			suggestions = make([]string, 0)
			for _, s := range llmResponse.Suggestions {
				suggestions = append(suggestions, s.Title+": "+s.Description)
			}
		}
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"suggestions": suggestions,
		},
	})
}

// JobTemplate represents a pre-built job template.
type JobTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Schedule    string            `json:"schedule"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Tags        []string          `json:"tags"`
}

// GetTemplates returns available job templates.
func (h *APIHandler) GetTemplates(w http.ResponseWriter, r *http.Request) {
	templates := []JobTemplate{
		{
			ID:          "health-check",
			Name:        "Health Check",
			Description: "Periodic health check endpoint ping",
			Category:    "monitoring",
			Schedule:    "*/5 * * * *",
			Method:      "GET",
			Tags:        []string{"monitoring", "health"},
		},
		{
			ID:          "daily-backup",
			Name:        "Daily Backup",
			Description: "Trigger daily backup job at midnight",
			Category:    "maintenance",
			Schedule:    "0 0 * * *",
			Method:      "POST",
			Tags:        []string{"backup", "daily"},
		},
		{
			ID:          "weekly-report",
			Name:        "Weekly Report",
			Description: "Generate weekly report every Monday morning",
			Category:    "reporting",
			Schedule:    "0 9 * * 1",
			Method:      "POST",
			Tags:        []string{"report", "weekly"},
		},
		{
			ID:          "hourly-sync",
			Name:        "Hourly Data Sync",
			Description: "Synchronize data every hour",
			Category:    "sync",
			Schedule:    "0 * * * *",
			Method:      "POST",
			Tags:        []string{"sync", "hourly"},
		},
		{
			ID:          "cache-warmup",
			Name:        "Cache Warmup",
			Description: "Warm up caches before peak hours",
			Category:    "performance",
			Schedule:    "0 7 * * 1-5",
			Method:      "POST",
			Tags:        []string{"cache", "performance"},
		},
		{
			ID:          "db-cleanup",
			Name:        "Database Cleanup",
			Description: "Clean up old records weekly",
			Category:    "maintenance",
			Schedule:    "0 3 * * 0",
			Method:      "POST",
			Tags:        []string{"database", "cleanup"},
		},
		{
			ID:          "cert-check",
			Name:        "Certificate Check",
			Description: "Check SSL certificate expiration daily",
			Category:    "security",
			Schedule:    "0 6 * * *",
			Method:      "GET",
			Tags:        []string{"ssl", "security"},
		},
		{
			ID:          "invoice-generation",
			Name:        "Monthly Invoice",
			Description: "Generate invoices on the 1st of each month",
			Category:    "billing",
			Schedule:    "0 0 1 * *",
			Method:      "POST",
			Tags:        []string{"invoice", "billing"},
		},
	}

	// Filter by category if specified
	category := r.URL.Query().Get("category")
	if category != "" {
		filtered := []JobTemplate{}
		for _, t := range templates {
			if t.Category == category {
				filtered = append(filtered, t)
			}
		}
		templates = filtered
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: templates})
}

// CreateFromTemplateRequest is a request to create a job from template.
type CreateFromTemplateRequest struct {
	TemplateID string `json:"template_id"`
	WebhookURL string `json:"webhook_url"`
	Name       string `json:"name,omitempty"`
	Timezone   string `json:"timezone,omitempty"`
}

// CreateFromTemplate creates a job draft from a template.
func (h *APIHandler) CreateFromTemplate(w http.ResponseWriter, r *http.Request) {
	var req CreateFromTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	// Find template
	templates := map[string]JobTemplate{
		"health-check":       {Schedule: "*/5 * * * *", Method: "GET", Name: "Health Check"},
		"daily-backup":       {Schedule: "0 0 * * *", Method: "POST", Name: "Daily Backup"},
		"weekly-report":      {Schedule: "0 9 * * 1", Method: "POST", Name: "Weekly Report"},
		"hourly-sync":        {Schedule: "0 * * * *", Method: "POST", Name: "Hourly Data Sync"},
		"cache-warmup":       {Schedule: "0 7 * * 1-5", Method: "POST", Name: "Cache Warmup"},
		"db-cleanup":         {Schedule: "0 3 * * 0", Method: "POST", Name: "Database Cleanup"},
		"cert-check":         {Schedule: "0 6 * * *", Method: "GET", Name: "Certificate Check"},
		"invoice-generation": {Schedule: "0 0 1 * *", Method: "POST", Name: "Monthly Invoice"},
	}

	template, ok := templates[req.TemplateID]
	if !ok {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: "template not found"})
		return
	}

	draft := &JobDraft{
		Name:       req.Name,
		Schedule:   template.Schedule,
		WebhookURL: req.WebhookURL,
		Method:     template.Method,
		Timezone:   req.Timezone,
		Confidence: 1.0,
	}

	if draft.Name == "" {
		draft.Name = template.Name
	}
	if draft.Timezone == "" {
		draft.Timezone = "UTC"
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    draft,
	})
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
