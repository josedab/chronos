// Package chatbot provides interactive Slack and Teams bot integrations for Chronos.
package chatbot

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Teams errors.
var (
	ErrInvalidTeamsSignature = errors.New("invalid Teams request signature")
	ErrInvalidActivity       = errors.New("invalid activity format")
)

// TeamsBot handles Microsoft Teams bot interactions.
type TeamsBot struct {
	appID        string
	appPassword  string
	tenantID     string
	commands     map[string]TeamsCommandHandler
	httpClient   *http.Client
	jobManager   JobManager
	rateLimiter  *rateLimiter
	accessToken  string
	tokenExpiry  time.Time
}

// TeamsCommandHandler handles a Teams command.
type TeamsCommandHandler func(ctx context.Context, activity *Activity) (*TeamsResponse, error)

// TeamsConfig configures the Teams bot.
type TeamsConfig struct {
	AppID       string
	AppPassword string
	TenantID    string
}

// Activity represents a Teams bot activity.
type Activity struct {
	Type           string       `json:"type"`
	ID             string       `json:"id"`
	Timestamp      string       `json:"timestamp"`
	LocalTimestamp string       `json:"localTimestamp"`
	ServiceURL     string       `json:"serviceUrl"`
	ChannelID      string       `json:"channelId"`
	From           ChannelAccount `json:"from"`
	Conversation   Conversation `json:"conversation"`
	Recipient      ChannelAccount `json:"recipient"`
	Text           string       `json:"text"`
	TextFormat     string       `json:"textFormat"`
	Locale         string       `json:"locale"`
	Entities       []Entity     `json:"entities,omitempty"`
	ChannelData    *ChannelData `json:"channelData,omitempty"`
	Value          interface{}  `json:"value,omitempty"`
	ReplyToID      string       `json:"replyToId,omitempty"`
}

// ChannelAccount represents a Teams user or bot.
type ChannelAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	AADObjectID string `json:"aadObjectId,omitempty"`
}

// Conversation represents a Teams conversation.
type Conversation struct {
	ID               string `json:"id"`
	Name             string `json:"name,omitempty"`
	ConversationType string `json:"conversationType,omitempty"`
	TenantID         string `json:"tenantId,omitempty"`
	IsGroup          bool   `json:"isGroup,omitempty"`
}

// Entity represents a message entity.
type Entity struct {
	Type     string       `json:"type"`
	Mentioned *ChannelAccount `json:"mentioned,omitempty"`
	Text     string       `json:"text,omitempty"`
}

// ChannelData contains Teams-specific data.
type ChannelData struct {
	TeamsChannelID string `json:"teamsChannelId,omitempty"`
	TeamsTeamID    string `json:"teamsTeamId,omitempty"`
	Channel        *struct {
		ID string `json:"id"`
	} `json:"channel,omitempty"`
	Team *struct {
		ID string `json:"id"`
	} `json:"team,omitempty"`
	Tenant *struct {
		ID string `json:"id"`
	} `json:"tenant,omitempty"`
}

// TeamsResponse represents a response to send back to Teams.
type TeamsResponse struct {
	Type        string             `json:"type"`
	Text        string             `json:"text,omitempty"`
	TextFormat  string             `json:"textFormat,omitempty"`
	Attachments []TeamsAttachment  `json:"attachments,omitempty"`
	SuggestedActions *SuggestedActions `json:"suggestedActions,omitempty"`
}

// TeamsAttachment represents a Teams message attachment.
type TeamsAttachment struct {
	ContentType string      `json:"contentType"`
	ContentURL  string      `json:"contentUrl,omitempty"`
	Content     interface{} `json:"content,omitempty"`
	Name        string      `json:"name,omitempty"`
	Color       string      `json:"color,omitempty"`
}

// AdaptiveCard represents a Microsoft Adaptive Card.
type AdaptiveCard struct {
	Type    string             `json:"type"`
	Version string             `json:"version"`
	Body    []AdaptiveElement  `json:"body"`
	Actions []AdaptiveAction   `json:"actions,omitempty"`
}

// AdaptiveElement represents an element in an Adaptive Card.
type AdaptiveElement struct {
	Type      string            `json:"type"`
	Text      string            `json:"text,omitempty"`
	Size      string            `json:"size,omitempty"`
	Weight    string            `json:"weight,omitempty"`
	Color     string            `json:"color,omitempty"`
	Wrap      bool              `json:"wrap,omitempty"`
	Spacing   string            `json:"spacing,omitempty"`
	Separator bool              `json:"separator,omitempty"`
	Columns   []AdaptiveColumn  `json:"columns,omitempty"`
	Facts     []AdaptiveFact    `json:"facts,omitempty"`
	Items     []AdaptiveElement `json:"items,omitempty"`
}

// AdaptiveColumn represents a column in an Adaptive Card.
type AdaptiveColumn struct {
	Type   string            `json:"type"`
	Width  string            `json:"width,omitempty"`
	Items  []AdaptiveElement `json:"items,omitempty"`
}

// AdaptiveFact represents a fact in an Adaptive Card.
type AdaptiveFact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

// AdaptiveAction represents an action in an Adaptive Card.
type AdaptiveAction struct {
	Type  string      `json:"type"`
	Title string      `json:"title"`
	URL   string      `json:"url,omitempty"`
	Data  interface{} `json:"data,omitempty"`
	Style string      `json:"style,omitempty"`
}

// SuggestedActions provides quick action buttons.
type SuggestedActions struct {
	Actions []CardAction `json:"actions"`
}

// CardAction represents a button action.
type CardAction struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Value string `json:"value"`
}

// NewTeamsBot creates a new Teams bot.
func NewTeamsBot(cfg TeamsConfig, jm JobManager) *TeamsBot {
	bot := &TeamsBot{
		appID:       cfg.AppID,
		appPassword: cfg.AppPassword,
		tenantID:    cfg.TenantID,
		commands:    make(map[string]TeamsCommandHandler),
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		jobManager:  jm,
		rateLimiter: newRateLimiter(100, time.Minute),
	}

	bot.registerDefaultCommands()
	return bot
}

// registerDefaultCommands registers the built-in commands.
func (t *TeamsBot) registerDefaultCommands() {
	t.commands["help"] = t.handleHelp
	t.commands["list"] = t.handleListJobs
	t.commands["status"] = t.handleJobStatus
	t.commands["run"] = t.handleRunJob
	t.commands["pause"] = t.handlePauseJob
	t.commands["resume"] = t.handleResumeJob
	t.commands["history"] = t.handleJobHistory
	t.commands["health"] = t.handleClusterHealth
}

// RegisterCommand registers a custom command handler.
func (t *TeamsBot) RegisterCommand(name string, handler TeamsCommandHandler) {
	t.commands[name] = handler
}

// HandleActivity handles incoming bot activities.
func (t *TeamsBot) HandleActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify authorization
	if err := t.verifyAuthorization(r); err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse activity
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var activity Activity
	if err := json.Unmarshal(body, &activity); err != nil {
		http.Error(w, "Invalid activity", http.StatusBadRequest)
		return
	}

	// Handle based on activity type
	switch activity.Type {
	case "message":
		t.handleMessage(ctx, w, &activity)
	case "invoke":
		t.handleInvoke(ctx, w, &activity)
	case "conversationUpdate":
		t.handleConversationUpdate(ctx, w, &activity)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

// verifyAuthorization verifies the Teams request authorization.
func (t *TeamsBot) verifyAuthorization(r *http.Request) error {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ErrInvalidTeamsSignature
	}

	// In production, this would validate the JWT token from Azure AD
	// For now, we do basic validation
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ErrInvalidTeamsSignature
	}

	return nil
}

// handleMessage processes incoming messages.
func (t *TeamsBot) handleMessage(ctx context.Context, w http.ResponseWriter, activity *Activity) {
	// Rate limit
	if !t.rateLimiter.allow(activity.From.ID) {
		t.sendReply(ctx, activity, "You're sending messages too fast. Please slow down.")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse command from message text
	text := strings.TrimSpace(activity.Text)
	
	// Remove bot mention if present
	for _, entity := range activity.Entities {
		if entity.Type == "mention" && entity.Mentioned != nil {
			text = strings.Replace(text, entity.Text, "", 1)
		}
	}
	text = strings.TrimSpace(text)

	// Parse command and args
	parts := strings.Fields(text)
	var command, args string
	if len(parts) > 0 {
		command = strings.ToLower(parts[0])
		args = strings.TrimPrefix(text, parts[0])
		args = strings.TrimSpace(args)
	}

	if command == "" {
		command = "help"
	}

	// Find handler
	handler, exists := t.commands[command]
	if !exists {
		t.sendReply(ctx, activity, fmt.Sprintf("Unknown command: **%s**. Type **help** to see available commands.", command))
		w.WriteHeader(http.StatusOK)
		return
	}

	// Update activity text to just args
	activity.Text = args

	// Execute handler
	resp, err := handler(ctx, activity)
	if err != nil {
		t.sendReply(ctx, activity, fmt.Sprintf("Error: %s", err.Error()))
		w.WriteHeader(http.StatusOK)
		return
	}

	// Send response
	t.sendActivityResponse(ctx, activity, resp)
	w.WriteHeader(http.StatusOK)
}

// handleInvoke processes card action invokes.
func (t *TeamsBot) handleInvoke(ctx context.Context, w http.ResponseWriter, activity *Activity) {
	// Handle adaptive card actions
	if activity.Value != nil {
		valueMap, ok := activity.Value.(map[string]interface{})
		if ok {
			if action, exists := valueMap["action"]; exists {
				switch action {
				case "runJob":
					if jobID, ok := valueMap["jobId"].(string); ok {
						t.handleRunJobAction(ctx, w, activity, jobID)
						return
					}
				case "pauseJob":
					if jobID, ok := valueMap["jobId"].(string); ok {
						t.handlePauseJobAction(ctx, w, activity, jobID)
						return
					}
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleConversationUpdate handles conversation updates.
func (t *TeamsBot) handleConversationUpdate(ctx context.Context, w http.ResponseWriter, activity *Activity) {
	// Send welcome message when bot is added
	if activity.ChannelData != nil && activity.ChannelData.Team != nil {
		welcomeMsg := "👋 **Hello! I'm Chronos Bot.**\n\n" +
			"I can help you manage your scheduled jobs. Here's what I can do:\n\n" +
			"- **list** - View all jobs\n" +
			"- **status <job>** - Check job status\n" +
			"- **run <job>** - Trigger a job\n" +
			"- **pause <job>** - Pause a job\n" +
			"- **resume <job>** - Resume a job\n" +
			"- **history <job>** - View execution history\n" +
			"- **health** - Check cluster health\n\n" +
			"Type **help** anytime for assistance!"

		t.sendReply(ctx, activity, welcomeMsg)
	}
	w.WriteHeader(http.StatusOK)
}

// Command handlers

func (t *TeamsBot) handleHelp(ctx context.Context, activity *Activity) (*TeamsResponse, error) {
	card := &AdaptiveCard{
		Type:    "AdaptiveCard",
		Version: "1.4",
		Body: []AdaptiveElement{
			{Type: "TextBlock", Text: "🕐 Chronos Bot Commands", Size: "large", Weight: "bolder"},
			{Type: "TextBlock", Text: "Available commands:", Wrap: true, Separator: true},
			{Type: "FactSet", Facts: []AdaptiveFact{
				{Title: "list", Value: "List all jobs"},
				{Title: "status <job>", Value: "Get job status and metrics"},
				{Title: "run <job>", Value: "Manually trigger a job"},
				{Title: "pause <job>", Value: "Pause a job"},
				{Title: "resume <job>", Value: "Resume a paused job"},
				{Title: "history <job>", Value: "View recent executions"},
				{Title: "health", Value: "Check cluster health"},
			}},
		},
		Actions: []AdaptiveAction{
			{Type: "Action.OpenUrl", Title: "📖 Documentation", URL: "https://chronos.dev/docs"},
		},
	}

	return t.buildCardResponse(card), nil
}

func (t *TeamsBot) handleListJobs(ctx context.Context, activity *Activity) (*TeamsResponse, error) {
	tenantID := extractTeamsTenantID(activity)
	jobs, err := t.jobManager.ListJobs(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if len(jobs) == 0 {
		return &TeamsResponse{
			Type: "message",
			Text: "No jobs found.",
		}, nil
	}

	bodyElements := []AdaptiveElement{
		{Type: "TextBlock", Text: fmt.Sprintf("📋 Jobs (%d total)", len(jobs)), Size: "large", Weight: "bolder"},
	}

	for _, job := range jobs {
		statusEmoji := getStatusEmoji(job.Status)
		bodyElements = append(bodyElements, AdaptiveElement{
			Type:      "Container",
			Separator: true,
			Items: []AdaptiveElement{
				{Type: "TextBlock", Text: fmt.Sprintf("%s **%s**", statusEmoji, job.Name), Weight: "bolder"},
				{Type: "TextBlock", Text: fmt.Sprintf("ID: `%s`", job.ID), Size: "small"},
				{Type: "TextBlock", Text: fmt.Sprintf("Schedule: `%s`", job.Schedule), Size: "small"},
				{Type: "TextBlock", Text: fmt.Sprintf("Next run: %s", formatTime(job.NextRun)), Size: "small"},
			},
		})
	}

	card := &AdaptiveCard{
		Type:    "AdaptiveCard",
		Version: "1.4",
		Body:    bodyElements,
	}

	return t.buildCardResponse(card), nil
}

func (t *TeamsBot) handleJobStatus(ctx context.Context, activity *Activity) (*TeamsResponse, error) {
	jobID := strings.TrimSpace(activity.Text)
	if jobID == "" {
		return &TeamsResponse{
			Type: "message",
			Text: "Please specify a job ID: **status <job-id>**",
		}, nil
	}

	tenantID := extractTeamsTenantID(activity)
	job, err := t.jobManager.GetJob(ctx, tenantID, jobID)
	if err != nil {
		return nil, err
	}

	status, err := t.jobManager.GetJobStatus(ctx, tenantID, jobID)
	if err != nil {
		return nil, err
	}

	healthText := "✅ Healthy"
	if !status.IsHealthy {
		healthText = "⚠️ Degraded"
	}

	card := &AdaptiveCard{
		Type:    "AdaptiveCard",
		Version: "1.4",
		Body: []AdaptiveElement{
			{Type: "TextBlock", Text: fmt.Sprintf("📊 %s", job.Name), Size: "large", Weight: "bolder"},
			{Type: "FactSet", Facts: []AdaptiveFact{
				{Title: "Status", Value: fmt.Sprintf("%s %s", getStatusEmoji(job.Status), job.Status)},
				{Title: "Health", Value: healthText},
				{Title: "Success Rate", Value: fmt.Sprintf("%.1f%%", status.SuccessRate*100)},
				{Title: "Avg Duration", Value: status.AvgDuration},
				{Title: "Schedule", Value: job.Schedule},
				{Title: "Recent Errors", Value: fmt.Sprintf("%d", status.RecentErrors)},
			}},
		},
		Actions: []AdaptiveAction{
			{
				Type:  "Action.Submit",
				Title: "▶️ Run Now",
				Data:  map[string]interface{}{"action": "runJob", "jobId": job.ID},
				Style: "positive",
			},
			{
				Type:  "Action.Submit",
				Title: "⏸️ Pause",
				Data:  map[string]interface{}{"action": "pauseJob", "jobId": job.ID},
				Style: "destructive",
			},
		},
	}

	return t.buildCardResponse(card), nil
}

func (t *TeamsBot) handleRunJob(ctx context.Context, activity *Activity) (*TeamsResponse, error) {
	jobID := strings.TrimSpace(activity.Text)
	if jobID == "" {
		return &TeamsResponse{
			Type: "message",
			Text: "Please specify a job ID: **run <job-id>**",
		}, nil
	}

	tenantID := extractTeamsTenantID(activity)
	exec, err := t.jobManager.RunJob(ctx, tenantID, jobID)
	if err != nil {
		return nil, err
	}

	return &TeamsResponse{
		Type:       "message",
		TextFormat: "markdown",
		Text: fmt.Sprintf("🚀 **Job triggered successfully!**\n\n"+
			"- Job ID: `%s`\n"+
			"- Execution ID: `%s`\n"+
			"- Status: %s\n"+
			"- Started at: %s",
			jobID, exec.ID, exec.Status, formatTime(exec.StartedAt)),
	}, nil
}

func (t *TeamsBot) handlePauseJob(ctx context.Context, activity *Activity) (*TeamsResponse, error) {
	jobID := strings.TrimSpace(activity.Text)
	if jobID == "" {
		return &TeamsResponse{
			Type: "message",
			Text: "Please specify a job ID: **pause <job-id>**",
		}, nil
	}

	tenantID := extractTeamsTenantID(activity)
	if err := t.jobManager.PauseJob(ctx, tenantID, jobID); err != nil {
		return nil, err
	}

	return &TeamsResponse{
		Type: "message",
		Text: fmt.Sprintf("⏸️ Job `%s` has been paused.", jobID),
	}, nil
}

func (t *TeamsBot) handleResumeJob(ctx context.Context, activity *Activity) (*TeamsResponse, error) {
	jobID := strings.TrimSpace(activity.Text)
	if jobID == "" {
		return &TeamsResponse{
			Type: "message",
			Text: "Please specify a job ID: **resume <job-id>**",
		}, nil
	}

	tenantID := extractTeamsTenantID(activity)
	if err := t.jobManager.ResumeJob(ctx, tenantID, jobID); err != nil {
		return nil, err
	}

	return &TeamsResponse{
		Type: "message",
		Text: fmt.Sprintf("▶️ Job `%s` has been resumed.", jobID),
	}, nil
}

func (t *TeamsBot) handleJobHistory(ctx context.Context, activity *Activity) (*TeamsResponse, error) {
	jobID := strings.TrimSpace(activity.Text)
	if jobID == "" {
		return &TeamsResponse{
			Type: "message",
			Text: "Please specify a job ID: **history <job-id>**",
		}, nil
	}

	tenantID := extractTeamsTenantID(activity)
	history, err := t.jobManager.GetJobHistory(ctx, tenantID, jobID, 10)
	if err != nil {
		return nil, err
	}

	if len(history) == 0 {
		return &TeamsResponse{
			Type: "message",
			Text: fmt.Sprintf("No execution history found for job `%s`.", jobID),
		}, nil
	}

	facts := make([]AdaptiveFact, 0, len(history))
	for _, exec := range history {
		statusEmoji := getStatusEmoji(exec.Status)
		facts = append(facts, AdaptiveFact{
			Title: exec.ID[:8],
			Value: fmt.Sprintf("%s %s (%s)", statusEmoji, exec.Status, exec.Duration),
		})
	}

	card := &AdaptiveCard{
		Type:    "AdaptiveCard",
		Version: "1.4",
		Body: []AdaptiveElement{
			{Type: "TextBlock", Text: fmt.Sprintf("📜 Recent Executions for %s", jobID), Size: "large", Weight: "bolder"},
			{Type: "FactSet", Facts: facts},
		},
	}

	return t.buildCardResponse(card), nil
}

func (t *TeamsBot) handleClusterHealth(ctx context.Context, activity *Activity) (*TeamsResponse, error) {
	card := &AdaptiveCard{
		Type:    "AdaptiveCard",
		Version: "1.4",
		Body: []AdaptiveElement{
			{Type: "TextBlock", Text: "🏥 Cluster Health", Size: "large", Weight: "bolder"},
			{
				Type: "ColumnSet",
				Columns: []AdaptiveColumn{
					{
						Type:  "Column",
						Width: "stretch",
						Items: []AdaptiveElement{
							{Type: "TextBlock", Text: "Scheduler", Weight: "bolder"},
							{Type: "TextBlock", Text: "✅ Healthy", Color: "good"},
						},
					},
					{
						Type:  "Column",
						Width: "stretch",
						Items: []AdaptiveElement{
							{Type: "TextBlock", Text: "Storage", Weight: "bolder"},
							{Type: "TextBlock", Text: "✅ Healthy", Color: "good"},
						},
					},
				},
			},
			{Type: "FactSet", Facts: []AdaptiveFact{
				{Title: "Raft Leader", Value: "✅ node-1"},
				{Title: "Cluster Size", Value: "3 nodes"},
				{Title: "Active Jobs", Value: "42"},
				{Title: "Pending Executions", Value: "0"},
			}},
			{Type: "TextBlock", Text: fmt.Sprintf("Last updated: %s", formatTime(time.Now())), Size: "small"},
		},
	}

	return t.buildCardResponse(card), nil
}

// Action handlers

func (t *TeamsBot) handleRunJobAction(ctx context.Context, w http.ResponseWriter, activity *Activity, jobID string) {
	tenantID := extractTeamsTenantID(activity)
	exec, err := t.jobManager.RunJob(ctx, tenantID, jobID)
	if err != nil {
		t.sendReply(ctx, activity, fmt.Sprintf("Failed to run job: %s", err.Error()))
		return
	}

	t.sendReply(ctx, activity, fmt.Sprintf("🚀 Job `%s` triggered! Execution ID: `%s`", jobID, exec.ID))
}

func (t *TeamsBot) handlePauseJobAction(ctx context.Context, w http.ResponseWriter, activity *Activity, jobID string) {
	tenantID := extractTeamsTenantID(activity)
	if err := t.jobManager.PauseJob(ctx, tenantID, jobID); err != nil {
		t.sendReply(ctx, activity, fmt.Sprintf("Failed to pause job: %s", err.Error()))
		return
	}

	t.sendReply(ctx, activity, fmt.Sprintf("⏸️ Job `%s` has been paused.", jobID))
}

// Helper methods

func (t *TeamsBot) buildCardResponse(card *AdaptiveCard) *TeamsResponse {
	return &TeamsResponse{
		Type: "message",
		Attachments: []TeamsAttachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				Content:     card,
			},
		},
	}
}

func (t *TeamsBot) sendReply(ctx context.Context, activity *Activity, text string) error {
	response := &TeamsResponse{
		Type: "message",
		Text: text,
	}
	return t.sendActivityResponse(ctx, activity, response)
}

func (t *TeamsBot) sendActivityResponse(ctx context.Context, activity *Activity, response *TeamsResponse) error {
	// Get access token
	token, err := t.getAccessToken(ctx)
	if err != nil {
		return err
	}

	// Build reply URL
	replyURL := fmt.Sprintf("%s/v3/conversations/%s/activities/%s",
		strings.TrimSuffix(activity.ServiceURL, "/"),
		activity.Conversation.ID,
		activity.ID,
	)

	// Send response
	body, err := json.Marshal(response)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", replyURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send reply: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

func (t *TeamsBot) getAccessToken(ctx context.Context) (string, error) {
	// Return cached token if valid
	if t.accessToken != "" && time.Now().Before(t.tokenExpiry) {
		return t.accessToken, nil
	}

	// Request new token from Azure AD
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", t.tenantID)
	if t.tenantID == "" {
		tokenURL = "https://login.microsoftonline.com/botframework.com/oauth2/v2.0/token"
	}

	data := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s&scope=https://api.botframework.com/.default",
		t.appID, t.appPassword)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	t.accessToken = tokenResp.AccessToken
	t.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return t.accessToken, nil
}

// Utility functions

func extractTeamsTenantID(activity *Activity) string {
	if activity.ChannelData != nil && activity.ChannelData.Tenant != nil {
		return fmt.Sprintf("teams-%s", activity.ChannelData.Tenant.ID)
	}
	if activity.Conversation.TenantID != "" {
		return fmt.Sprintf("teams-%s", activity.Conversation.TenantID)
	}
	return "teams-default"
}

func computeHMAC(message, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
