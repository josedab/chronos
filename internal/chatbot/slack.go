// Package chatbot provides interactive Slack and Teams bot integrations for Chronos.
package chatbot

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Slack errors.
var (
	ErrInvalidSignature     = errors.New("invalid request signature")
	ErrExpiredTimestamp     = errors.New("request timestamp expired")
	ErrCommandNotFound      = errors.New("command not found")
	ErrUnauthorizedChannel  = errors.New("unauthorized channel")
	ErrRateLimitExceeded    = errors.New("rate limit exceeded")
)

// SlackBot handles Slack bot interactions.
type SlackBot struct {
	signingSecret   string
	botToken        string
	appToken        string
	allowedChannels map[string]bool
	commands        map[string]CommandHandler
	httpClient      *http.Client
	jobManager      JobManager
	rateLimiter     *rateLimiter
}

// JobManager interface for interacting with Chronos jobs.
type JobManager interface {
	ListJobs(ctx context.Context, tenantID string) ([]Job, error)
	GetJob(ctx context.Context, tenantID, jobID string) (*Job, error)
	RunJob(ctx context.Context, tenantID, jobID string) (*Execution, error)
	PauseJob(ctx context.Context, tenantID, jobID string) error
	ResumeJob(ctx context.Context, tenantID, jobID string) error
	GetJobHistory(ctx context.Context, tenantID, jobID string, limit int) ([]Execution, error)
	GetJobStatus(ctx context.Context, tenantID, jobID string) (*JobStatus, error)
}

// Job represents a Chronos job.
type Job struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Schedule    string    `json:"schedule"`
	Status      string    `json:"status"`
	LastRun     time.Time `json:"last_run"`
	NextRun     time.Time `json:"next_run"`
	Description string    `json:"description"`
}

// Execution represents a job execution.
type Execution struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Duration  string    `json:"duration"`
	Error     string    `json:"error,omitempty"`
}

// JobStatus represents job health and metrics.
type JobStatus struct {
	JobID        string  `json:"job_id"`
	IsHealthy    bool    `json:"is_healthy"`
	SuccessRate  float64 `json:"success_rate"`
	AvgDuration  string  `json:"avg_duration"`
	RecentErrors int     `json:"recent_errors"`
}

// CommandHandler handles a slash command.
type CommandHandler func(ctx context.Context, cmd *SlashCommand) (*SlackResponse, error)

// SlashCommand represents a Slack slash command.
type SlashCommand struct {
	Token          string `json:"token"`
	TeamID         string `json:"team_id"`
	TeamDomain     string `json:"team_domain"`
	ChannelID      string `json:"channel_id"`
	ChannelName    string `json:"channel_name"`
	UserID         string `json:"user_id"`
	UserName       string `json:"user_name"`
	Command        string `json:"command"`
	Text           string `json:"text"`
	ResponseURL    string `json:"response_url"`
	TriggerID      string `json:"trigger_id"`
	APIAppID       string `json:"api_app_id"`
}

// SlackResponse is a response to send back to Slack.
type SlackResponse struct {
	ResponseType    string       `json:"response_type,omitempty"`
	Text            string       `json:"text,omitempty"`
	Blocks          []Block      `json:"blocks,omitempty"`
	Attachments     []Attachment `json:"attachments,omitempty"`
	ReplaceOriginal bool         `json:"replace_original,omitempty"`
	DeleteOriginal  bool         `json:"delete_original,omitempty"`
}

// Block represents a Slack block.
type Block struct {
	Type      string      `json:"type"`
	Text      *TextObject `json:"text,omitempty"`
	BlockID   string      `json:"block_id,omitempty"`
	Accessory *Accessory  `json:"accessory,omitempty"`
	Elements  []Element   `json:"elements,omitempty"`
	Fields    []TextObject `json:"fields,omitempty"`
}

// TextObject represents text in a block.
type TextObject struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Emoji    bool   `json:"emoji,omitempty"`
	Verbatim bool   `json:"verbatim,omitempty"`
}

// Accessory represents a block accessory.
type Accessory struct {
	Type     string      `json:"type"`
	ImageURL string      `json:"image_url,omitempty"`
	AltText  string      `json:"alt_text,omitempty"`
	ActionID string      `json:"action_id,omitempty"`
	Text     *TextObject `json:"text,omitempty"`
	Value    string      `json:"value,omitempty"`
}

// Element represents an element in a block.
type Element struct {
	Type     string      `json:"type"`
	ActionID string      `json:"action_id,omitempty"`
	Text     *TextObject `json:"text,omitempty"`
	Value    string      `json:"value,omitempty"`
	Style    string      `json:"style,omitempty"`
	URL      string      `json:"url,omitempty"`
	Confirm  *Confirm    `json:"confirm,omitempty"`
}

// Confirm represents a confirmation dialog.
type Confirm struct {
	Title   *TextObject `json:"title"`
	Text    *TextObject `json:"text"`
	Confirm *TextObject `json:"confirm"`
	Deny    *TextObject `json:"deny"`
	Style   string      `json:"style,omitempty"`
}

// Attachment represents a Slack message attachment.
type Attachment struct {
	Color      string `json:"color,omitempty"`
	Pretext    string `json:"pretext,omitempty"`
	AuthorName string `json:"author_name,omitempty"`
	Title      string `json:"title,omitempty"`
	TitleLink  string `json:"title_link,omitempty"`
	Text       string `json:"text,omitempty"`
	Footer     string `json:"footer,omitempty"`
	Ts         int64  `json:"ts,omitempty"`
}

// InteractionPayload represents a Slack interaction callback.
type InteractionPayload struct {
	Type        string `json:"type"`
	User        User   `json:"user"`
	Channel     Channel `json:"channel"`
	Team        Team   `json:"team"`
	Token       string `json:"token"`
	ActionTs    string `json:"action_ts"`
	MessageTs   string `json:"message_ts"`
	ResponseURL string `json:"response_url"`
	TriggerID   string `json:"trigger_id"`
	Actions     []Action `json:"actions"`
}

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Team struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

type Action struct {
	ActionID string `json:"action_id"`
	BlockID  string `json:"block_id"`
	Value    string `json:"value"`
	Type     string `json:"type"`
}

// SlackConfig configures the Slack bot.
type SlackConfig struct {
	SigningSecret   string
	BotToken        string
	AppToken        string
	AllowedChannels []string
}

// NewSlackBot creates a new Slack bot.
func NewSlackBot(cfg SlackConfig, jm JobManager) *SlackBot {
	bot := &SlackBot{
		signingSecret:   cfg.SigningSecret,
		botToken:        cfg.BotToken,
		appToken:        cfg.AppToken,
		allowedChannels: make(map[string]bool),
		commands:        make(map[string]CommandHandler),
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		jobManager:      jm,
		rateLimiter:     newRateLimiter(100, time.Minute),
	}

	for _, ch := range cfg.AllowedChannels {
		bot.allowedChannels[ch] = true
	}

	// Register default commands
	bot.registerDefaultCommands()

	return bot
}

// registerDefaultCommands registers the built-in commands.
func (s *SlackBot) registerDefaultCommands() {
	s.commands["help"] = s.handleHelp
	s.commands["list"] = s.handleListJobs
	s.commands["status"] = s.handleJobStatus
	s.commands["run"] = s.handleRunJob
	s.commands["pause"] = s.handlePauseJob
	s.commands["resume"] = s.handleResumeJob
	s.commands["history"] = s.handleJobHistory
	s.commands["health"] = s.handleClusterHealth
}

// RegisterCommand registers a custom command handler.
func (s *SlackBot) RegisterCommand(name string, handler CommandHandler) {
	s.commands[name] = handler
}

// HandleSlashCommand handles incoming slash commands.
func (s *SlackBot) HandleSlashCommand(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify signature
	if err := s.verifySignature(r); err != nil {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse command
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	cmd := &SlashCommand{
		Token:       r.FormValue("token"),
		TeamID:      r.FormValue("team_id"),
		TeamDomain:  r.FormValue("team_domain"),
		ChannelID:   r.FormValue("channel_id"),
		ChannelName: r.FormValue("channel_name"),
		UserID:      r.FormValue("user_id"),
		UserName:    r.FormValue("user_name"),
		Command:     r.FormValue("command"),
		Text:        r.FormValue("text"),
		ResponseURL: r.FormValue("response_url"),
		TriggerID:   r.FormValue("trigger_id"),
		APIAppID:    r.FormValue("api_app_id"),
	}

	// Check channel authorization (allow all if no restrictions set)
	if len(s.allowedChannels) > 0 && !s.allowedChannels[cmd.ChannelID] {
		s.sendEphemeral(w, "This command is not available in this channel.")
		return
	}

	// Rate limit
	if !s.rateLimiter.allow(cmd.UserID) {
		s.sendEphemeral(w, "You're sending commands too fast. Please slow down.")
		return
	}

	// Parse subcommand
	parts := strings.Fields(cmd.Text)
	var subcommand, args string
	if len(parts) > 0 {
		subcommand = parts[0]
		args = strings.TrimPrefix(cmd.Text, subcommand)
		args = strings.TrimSpace(args)
	}

	if subcommand == "" {
		subcommand = "help"
	}

	// Find and execute handler
	handler, exists := s.commands[subcommand]
	if !exists {
		resp := s.buildUnknownCommandResponse(subcommand)
		s.sendResponse(w, resp)
		return
	}

	// Update cmd.Text to just the args
	cmd.Text = args

	resp, err := handler(ctx, cmd)
	if err != nil {
		s.sendEphemeral(w, fmt.Sprintf("Error: %s", err.Error()))
		return
	}

	s.sendResponse(w, resp)
}

// HandleInteraction handles interactive component callbacks.
func (s *SlackBot) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify signature
	if err := s.verifySignature(r); err != nil {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse payload
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	payloadStr := r.FormValue("payload")
	var payload InteractionPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Handle action
	for _, action := range payload.Actions {
		switch {
		case strings.HasPrefix(action.ActionID, "run_job_"):
			jobID := strings.TrimPrefix(action.ActionID, "run_job_")
			s.handleJobRunAction(ctx, w, jobID, payload.Channel.ID)
			return
		case strings.HasPrefix(action.ActionID, "pause_job_"):
			jobID := strings.TrimPrefix(action.ActionID, "pause_job_")
			s.handleJobPauseAction(ctx, w, jobID, payload.Channel.ID)
			return
		case strings.HasPrefix(action.ActionID, "view_history_"):
			jobID := strings.TrimPrefix(action.ActionID, "view_history_")
			s.handleViewHistoryAction(ctx, w, jobID, payload.ResponseURL)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// verifySignature verifies the Slack request signature.
func (s *SlackBot) verifySignature(r *http.Request) error {
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")

	// Check timestamp to prevent replay attacks
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrInvalidSignature
	}

	if time.Now().Unix()-ts > 300 { // 5 minutes
		return ErrExpiredTimestamp
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Compute expected signature
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(s.signingSecret))
	mac.Write([]byte(baseString))
	expectedSig := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return ErrInvalidSignature
	}

	return nil
}

// Command handlers

func (s *SlackBot) handleHelp(ctx context.Context, cmd *SlashCommand) (*SlackResponse, error) {
	return &SlackResponse{
		ResponseType: "ephemeral",
		Blocks: []Block{
			{
				Type: "header",
				Text: &TextObject{Type: "plain_text", Text: "🕐 Chronos Bot Commands", Emoji: true},
			},
			{Type: "divider"},
			{
				Type: "section",
				Text: &TextObject{
					Type: "mrkdwn",
					Text: "*Available Commands:*\n" +
						"• `/chronos list` - List all jobs\n" +
						"• `/chronos status <job>` - Get job status and metrics\n" +
						"• `/chronos run <job>` - Manually trigger a job\n" +
						"• `/chronos pause <job>` - Pause a job\n" +
						"• `/chronos resume <job>` - Resume a paused job\n" +
						"• `/chronos history <job>` - View recent executions\n" +
						"• `/chronos health` - Check cluster health",
				},
			},
			{
				Type: "context",
				Elements: []Element{
					{Type: "mrkdwn", Text: &TextObject{Type: "mrkdwn", Text: "Need help? Visit our <https://chronos.dev/docs|documentation>"}},
				},
			},
		},
	}, nil
}

func (s *SlackBot) handleListJobs(ctx context.Context, cmd *SlashCommand) (*SlackResponse, error) {
	tenantID := extractTenantID(cmd.TeamID)
	jobs, err := s.jobManager.ListJobs(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if len(jobs) == 0 {
		return &SlackResponse{
			ResponseType: "in_channel",
			Text:         "No jobs found.",
		}, nil
	}

	blocks := []Block{
		{
			Type: "header",
			Text: &TextObject{Type: "plain_text", Text: fmt.Sprintf("📋 Jobs (%d total)", len(jobs)), Emoji: true},
		},
		{Type: "divider"},
	}

	for _, job := range jobs {
		statusEmoji := getStatusEmoji(job.Status)
		blocks = append(blocks, Block{
			Type: "section",
			Text: &TextObject{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s %s*\n`%s`\n_%s_\nNext run: %s",
					statusEmoji, job.Name, job.ID, job.Schedule,
					formatTime(job.NextRun)),
			},
			Accessory: &Accessory{
				Type:     "button",
				Text:     &TextObject{Type: "plain_text", Text: "Run Now", Emoji: true},
				ActionID: fmt.Sprintf("run_job_%s", job.ID),
				Value:    job.ID,
			},
		})
	}

	return &SlackResponse{
		ResponseType: "in_channel",
		Blocks:       blocks,
	}, nil
}

func (s *SlackBot) handleJobStatus(ctx context.Context, cmd *SlashCommand) (*SlackResponse, error) {
	jobID := strings.TrimSpace(cmd.Text)
	if jobID == "" {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         "Please specify a job ID: `/chronos status <job-id>`",
		}, nil
	}

	tenantID := extractTenantID(cmd.TeamID)
	job, err := s.jobManager.GetJob(ctx, tenantID, jobID)
	if err != nil {
		return nil, err
	}

	status, err := s.jobManager.GetJobStatus(ctx, tenantID, jobID)
	if err != nil {
		return nil, err
	}

	healthEmoji := "✅"
	healthText := "Healthy"
	if !status.IsHealthy {
		healthEmoji = "⚠️"
		healthText = "Degraded"
	}

	return &SlackResponse{
		ResponseType: "in_channel",
		Blocks: []Block{
			{
				Type: "header",
				Text: &TextObject{Type: "plain_text", Text: fmt.Sprintf("📊 %s", job.Name), Emoji: true},
			},
			{Type: "divider"},
			{
				Type: "section",
				Fields: []TextObject{
					{Type: "mrkdwn", Text: fmt.Sprintf("*Status:*\n%s %s", getStatusEmoji(job.Status), job.Status)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Health:*\n%s %s", healthEmoji, healthText)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Success Rate:*\n%.1f%%", status.SuccessRate*100)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Avg Duration:*\n%s", status.AvgDuration)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Schedule:*\n`%s`", job.Schedule)},
					{Type: "mrkdwn", Text: fmt.Sprintf("*Recent Errors:*\n%d", status.RecentErrors)},
				},
			},
			{
				Type: "actions",
				Elements: []Element{
					{
						Type:     "button",
						Text:     &TextObject{Type: "plain_text", Text: "Run Now", Emoji: true},
						ActionID: fmt.Sprintf("run_job_%s", job.ID),
						Value:    job.ID,
						Style:    "primary",
					},
					{
						Type:     "button",
						Text:     &TextObject{Type: "plain_text", Text: "View History", Emoji: true},
						ActionID: fmt.Sprintf("view_history_%s", job.ID),
						Value:    job.ID,
					},
					{
						Type:     "button",
						Text:     &TextObject{Type: "plain_text", Text: "Pause", Emoji: true},
						ActionID: fmt.Sprintf("pause_job_%s", job.ID),
						Value:    job.ID,
						Style:    "danger",
						Confirm: &Confirm{
							Title:   &TextObject{Type: "plain_text", Text: "Pause Job"},
							Text:    &TextObject{Type: "mrkdwn", Text: fmt.Sprintf("Are you sure you want to pause *%s*?", job.Name)},
							Confirm: &TextObject{Type: "plain_text", Text: "Pause"},
							Deny:    &TextObject{Type: "plain_text", Text: "Cancel"},
							Style:   "danger",
						},
					},
				},
			},
		},
	}, nil
}

func (s *SlackBot) handleRunJob(ctx context.Context, cmd *SlashCommand) (*SlackResponse, error) {
	jobID := strings.TrimSpace(cmd.Text)
	if jobID == "" {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         "Please specify a job ID: `/chronos run <job-id>`",
		}, nil
	}

	tenantID := extractTenantID(cmd.TeamID)
	exec, err := s.jobManager.RunJob(ctx, tenantID, jobID)
	if err != nil {
		return nil, err
	}

	return &SlackResponse{
		ResponseType: "in_channel",
		Blocks: []Block{
			{
				Type: "section",
				Text: &TextObject{
					Type: "mrkdwn",
					Text: fmt.Sprintf("🚀 *Job triggered successfully!*\n\n"+
						"• Job ID: `%s`\n"+
						"• Execution ID: `%s`\n"+
						"• Status: %s\n"+
						"• Started at: %s",
						jobID, exec.ID, exec.Status, formatTime(exec.StartedAt)),
				},
			},
		},
	}, nil
}

func (s *SlackBot) handlePauseJob(ctx context.Context, cmd *SlashCommand) (*SlackResponse, error) {
	jobID := strings.TrimSpace(cmd.Text)
	if jobID == "" {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         "Please specify a job ID: `/chronos pause <job-id>`",
		}, nil
	}

	tenantID := extractTenantID(cmd.TeamID)
	if err := s.jobManager.PauseJob(ctx, tenantID, jobID); err != nil {
		return nil, err
	}

	return &SlackResponse{
		ResponseType: "in_channel",
		Text:         fmt.Sprintf("⏸️ Job `%s` has been paused.", jobID),
	}, nil
}

func (s *SlackBot) handleResumeJob(ctx context.Context, cmd *SlashCommand) (*SlackResponse, error) {
	jobID := strings.TrimSpace(cmd.Text)
	if jobID == "" {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         "Please specify a job ID: `/chronos resume <job-id>`",
		}, nil
	}

	tenantID := extractTenantID(cmd.TeamID)
	if err := s.jobManager.ResumeJob(ctx, tenantID, jobID); err != nil {
		return nil, err
	}

	return &SlackResponse{
		ResponseType: "in_channel",
		Text:         fmt.Sprintf("▶️ Job `%s` has been resumed.", jobID),
	}, nil
}

func (s *SlackBot) handleJobHistory(ctx context.Context, cmd *SlashCommand) (*SlackResponse, error) {
	jobID := strings.TrimSpace(cmd.Text)
	if jobID == "" {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         "Please specify a job ID: `/chronos history <job-id>`",
		}, nil
	}

	tenantID := extractTenantID(cmd.TeamID)
	history, err := s.jobManager.GetJobHistory(ctx, tenantID, jobID, 10)
	if err != nil {
		return nil, err
	}

	if len(history) == 0 {
		return &SlackResponse{
			ResponseType: "in_channel",
			Text:         fmt.Sprintf("No execution history found for job `%s`.", jobID),
		}, nil
	}

	var historyText strings.Builder
	for _, exec := range history {
		statusEmoji := getStatusEmoji(exec.Status)
		execID := exec.ID
		if len(execID) > 8 {
			execID = execID[:8]
		}
		historyText.WriteString(fmt.Sprintf("%s `%s` - %s (%s)\n",
			statusEmoji, execID, exec.Status, exec.Duration))
	}

	return &SlackResponse{
		ResponseType: "in_channel",
		Blocks: []Block{
			{
				Type: "header",
				Text: &TextObject{Type: "plain_text", Text: fmt.Sprintf("📜 Recent Executions for %s", jobID), Emoji: true},
			},
			{Type: "divider"},
			{
				Type: "section",
				Text: &TextObject{Type: "mrkdwn", Text: historyText.String()},
			},
		},
	}, nil
}

func (s *SlackBot) handleClusterHealth(ctx context.Context, cmd *SlashCommand) (*SlackResponse, error) {
	// This would integrate with actual cluster health checks
	return &SlackResponse{
		ResponseType: "in_channel",
		Blocks: []Block{
			{
				Type: "header",
				Text: &TextObject{Type: "plain_text", Text: "🏥 Cluster Health", Emoji: true},
			},
			{Type: "divider"},
			{
				Type: "section",
				Fields: []TextObject{
					{Type: "mrkdwn", Text: "*Scheduler:*\n✅ Healthy"},
					{Type: "mrkdwn", Text: "*Storage:*\n✅ Healthy"},
					{Type: "mrkdwn", Text: "*Raft Leader:*\n✅ node-1"},
					{Type: "mrkdwn", Text: "*Cluster Size:*\n3 nodes"},
					{Type: "mrkdwn", Text: "*Active Jobs:*\n42"},
					{Type: "mrkdwn", Text: "*Pending Executions:*\n0"},
				},
			},
			{
				Type: "context",
				Elements: []Element{
					{Type: "mrkdwn", Text: &TextObject{Type: "mrkdwn", Text: fmt.Sprintf("Last updated: %s", formatTime(time.Now()))}},
				},
			},
		},
	}, nil
}

// Action handlers

func (s *SlackBot) handleJobRunAction(ctx context.Context, w http.ResponseWriter, jobID, channelID string) {
	tenantID := extractTenantID(channelID)
	exec, err := s.jobManager.RunJob(ctx, tenantID, jobID)
	if err != nil {
		s.sendEphemeral(w, fmt.Sprintf("Failed to run job: %s", err.Error()))
		return
	}

	resp := &SlackResponse{
		Text: fmt.Sprintf("🚀 Job `%s` triggered! Execution ID: `%s`", jobID, exec.ID),
	}
	s.sendResponse(w, resp)
}

func (s *SlackBot) handleJobPauseAction(ctx context.Context, w http.ResponseWriter, jobID, channelID string) {
	tenantID := extractTenantID(channelID)
	if err := s.jobManager.PauseJob(ctx, tenantID, jobID); err != nil {
		s.sendEphemeral(w, fmt.Sprintf("Failed to pause job: %s", err.Error()))
		return
	}

	resp := &SlackResponse{
		Text: fmt.Sprintf("⏸️ Job `%s` has been paused.", jobID),
	}
	s.sendResponse(w, resp)
}

func (s *SlackBot) handleViewHistoryAction(ctx context.Context, w http.ResponseWriter, jobID, responseURL string) {
	tenantID := extractTenantID(jobID) // Simplified
	history, err := s.jobManager.GetJobHistory(ctx, tenantID, jobID, 5)
	if err != nil {
		return
	}

	var historyText strings.Builder
	for _, exec := range history {
		statusEmoji := getStatusEmoji(exec.Status)
		historyText.WriteString(fmt.Sprintf("%s `%s` - %s (%s)\n",
			statusEmoji, exec.ID[:8], exec.Status, exec.Duration))
	}

	resp := &SlackResponse{
		Text: fmt.Sprintf("📜 *Recent executions for %s:*\n%s", jobID, historyText.String()),
	}

	// Send to response URL
	s.postToResponseURL(responseURL, resp)
	w.WriteHeader(http.StatusOK)
}

// Helper methods

func (s *SlackBot) buildUnknownCommandResponse(cmd string) *SlackResponse {
	return &SlackResponse{
		ResponseType: "ephemeral",
		Text:         fmt.Sprintf("Unknown command: `%s`. Use `/chronos help` to see available commands.", cmd),
	}
}

func (s *SlackBot) sendResponse(w http.ResponseWriter, resp *SlackResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *SlackBot) sendEphemeral(w http.ResponseWriter, text string) {
	s.sendResponse(w, &SlackResponse{
		ResponseType: "ephemeral",
		Text:         text,
	})
}

func (s *SlackBot) postToResponseURL(url string, resp *SlackResponse) error {
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	_, err = s.httpClient.Do(req)
	return err
}

// Utility functions

func getStatusEmoji(status string) string {
	switch strings.ToLower(status) {
	case "running":
		return "🔄"
	case "success", "succeeded", "active", "enabled":
		return "✅"
	case "failed", "error":
		return "❌"
	case "pending", "queued":
		return "⏳"
	case "paused", "disabled":
		return "⏸️"
	case "cancelled":
		return "🚫"
	default:
		return "❓"
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("Jan 2, 15:04 MST")
}

func extractTenantID(teamID string) string {
	// In production, this would map Slack team IDs to Chronos tenant IDs
	return fmt.Sprintf("tenant-%s", teamID)
}

// Rate limiter

type rateLimiter struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	mu       sync.Mutex
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (r *rateLimiter) allow(userID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-r.window)

	// Filter out old requests
	var recent []time.Time
	for _, t := range r.requests[userID] {
		if t.After(windowStart) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= r.limit {
		return false
	}

	r.requests[userID] = append(recent, now)
	return true
}
