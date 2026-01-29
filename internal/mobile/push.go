// Package mobile provides mobile companion app support for Chronos.
package mobile

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrDeviceNotFound       = errors.New("device not found")
	ErrDeviceAlreadyExists  = errors.New("device already registered")
	ErrInvalidPushToken     = errors.New("invalid push token")
	ErrNotificationFailed   = errors.New("notification delivery failed")
	ErrUserNotFound         = errors.New("user not found")
)

// Platform represents a mobile platform.
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

// Device represents a registered mobile device.
type Device struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	Platform     Platform          `json:"platform"`
	PushToken    string            `json:"push_token"`
	DeviceModel  string            `json:"device_model,omitempty"`
	OSVersion    string            `json:"os_version,omitempty"`
	AppVersion   string            `json:"app_version,omitempty"`
	Timezone     string            `json:"timezone,omitempty"`
	Enabled      bool              `json:"enabled"`
	Preferences  *DevicePreferences `json:"preferences,omitempty"`
	LastActiveAt time.Time         `json:"last_active_at"`
	RegisteredAt time.Time         `json:"registered_at"`
}

// DevicePreferences holds notification preferences for a device.
type DevicePreferences struct {
	// Job notifications
	NotifyOnJobFailure  bool `json:"notify_on_job_failure"`
	NotifyOnJobSuccess  bool `json:"notify_on_job_success"`
	NotifyOnJobStart    bool `json:"notify_on_job_start"`
	
	// Execution notifications
	NotifyOnExecFailure bool `json:"notify_on_exec_failure"`
	NotifyOnExecRetry   bool `json:"notify_on_exec_retry"`
	
	// System notifications
	NotifyOnClusterIssue bool `json:"notify_on_cluster_issue"`
	NotifyOnQuotaWarning bool `json:"notify_on_quota_warning"`
	
	// Schedule
	QuietHoursEnabled   bool   `json:"quiet_hours_enabled"`
	QuietHoursStart     string `json:"quiet_hours_start,omitempty"` // HH:MM
	QuietHoursEnd       string `json:"quiet_hours_end,omitempty"`   // HH:MM
	
	// Filtering
	WatchedJobIDs       []string `json:"watched_job_ids,omitempty"`
	WatchedTags         []string `json:"watched_tags,omitempty"`
	MinimumSeverity     string   `json:"minimum_severity,omitempty"` // info, warning, error, critical
}

// PushNotification represents a push notification to send.
type PushNotification struct {
	ID          string                 `json:"id"`
	DeviceID    string                 `json:"device_id"`
	UserID      string                 `json:"user_id"`
	Title       string                 `json:"title"`
	Body        string                 `json:"body"`
	Category    NotificationCategory   `json:"category"`
	Priority    NotificationPriority   `json:"priority"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Badge       *int                   `json:"badge,omitempty"`
	Sound       string                 `json:"sound,omitempty"`
	ActionURL   string                 `json:"action_url,omitempty"`
	ImageURL    string                 `json:"image_url,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	SentAt      *time.Time             `json:"sent_at,omitempty"`
	DeliveredAt *time.Time             `json:"delivered_at,omitempty"`
	ReadAt      *time.Time             `json:"read_at,omitempty"`
	Status      NotificationStatus     `json:"status"`
	Error       string                 `json:"error,omitempty"`
}

// NotificationCategory represents the category of notification.
type NotificationCategory string

const (
	CategoryJobStatus      NotificationCategory = "job_status"
	CategoryExecution      NotificationCategory = "execution"
	CategoryClusterHealth  NotificationCategory = "cluster_health"
	CategorySystem         NotificationCategory = "system"
	CategoryApproval       NotificationCategory = "approval"
	CategoryAlert          NotificationCategory = "alert"
)

// NotificationPriority represents notification priority.
type NotificationPriority string

const (
	PriorityLow      NotificationPriority = "low"
	PriorityNormal   NotificationPriority = "normal"
	PriorityHigh     NotificationPriority = "high"
	PriorityCritical NotificationPriority = "critical"
)

// NotificationStatus represents the delivery status.
type NotificationStatus string

const (
	StatusPending   NotificationStatus = "pending"
	StatusSent      NotificationStatus = "sent"
	StatusDelivered NotificationStatus = "delivered"
	StatusRead      NotificationStatus = "read"
	StatusFailed    NotificationStatus = "failed"
)

// PushProvider interface for push notification providers.
type PushProvider interface {
	Send(ctx context.Context, device *Device, notification *PushNotification) error
	ValidateToken(platform Platform, token string) error
}

// PushService manages push notifications.
type PushService struct {
	devices       map[string]*Device       // deviceID -> Device
	userDevices   map[string][]string      // userID -> []deviceID
	notifications map[string]*PushNotification
	providers     map[Platform]PushProvider
	mu            sync.RWMutex
	config        PushConfig
}

// PushConfig configures the push service.
type PushConfig struct {
	APNSKeyID      string `json:"apns_key_id,omitempty"`
	APNSTeamID     string `json:"apns_team_id,omitempty"`
	APNSBundleID   string `json:"apns_bundle_id,omitempty"`
	APNSProduction bool   `json:"apns_production"`
	FCMProjectID   string `json:"fcm_project_id,omitempty"`
	FCMAPIKey      string `json:"fcm_api_key,omitempty"`
	RetryAttempts  int    `json:"retry_attempts"`
	RetryDelay     time.Duration `json:"retry_delay"`
}

// DefaultPushConfig returns default push configuration.
func DefaultPushConfig() PushConfig {
	return PushConfig{
		RetryAttempts: 3,
		RetryDelay:    time.Second,
	}
}

// NewPushService creates a new push notification service.
func NewPushService(cfg PushConfig) *PushService {
	return &PushService{
		devices:       make(map[string]*Device),
		userDevices:   make(map[string][]string),
		notifications: make(map[string]*PushNotification),
		providers:     make(map[Platform]PushProvider),
		config:        cfg,
	}
}

// RegisterDevice registers a mobile device for push notifications.
func (s *PushService) RegisterDevice(ctx context.Context, userID string, platform Platform, pushToken string, deviceInfo *DeviceInfo) (*Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if device with same token exists
	for _, d := range s.devices {
		if d.PushToken == pushToken {
			// Update existing device
			d.UserID = userID
			d.LastActiveAt = time.Now()
			d.Enabled = true
			if deviceInfo != nil {
				d.DeviceModel = deviceInfo.Model
				d.OSVersion = deviceInfo.OSVersion
				d.AppVersion = deviceInfo.AppVersion
				d.Timezone = deviceInfo.Timezone
			}
			return d, nil
		}
	}

	device := &Device{
		ID:           uuid.New().String(),
		UserID:       userID,
		Platform:     platform,
		PushToken:    pushToken,
		Enabled:      true,
		Preferences:  defaultPreferences(),
		LastActiveAt: time.Now(),
		RegisteredAt: time.Now(),
	}

	if deviceInfo != nil {
		device.DeviceModel = deviceInfo.Model
		device.OSVersion = deviceInfo.OSVersion
		device.AppVersion = deviceInfo.AppVersion
		device.Timezone = deviceInfo.Timezone
	}

	s.devices[device.ID] = device
	s.userDevices[userID] = append(s.userDevices[userID], device.ID)

	return device, nil
}

// DeviceInfo contains device metadata.
type DeviceInfo struct {
	Model      string `json:"model"`
	OSVersion  string `json:"os_version"`
	AppVersion string `json:"app_version"`
	Timezone   string `json:"timezone"`
}

// UnregisterDevice removes a device registration.
func (s *PushService) UnregisterDevice(ctx context.Context, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, exists := s.devices[deviceID]
	if !exists {
		return ErrDeviceNotFound
	}

	// Remove from user's device list
	if devices, ok := s.userDevices[device.UserID]; ok {
		newDevices := make([]string, 0, len(devices)-1)
		for _, id := range devices {
			if id != deviceID {
				newDevices = append(newDevices, id)
			}
		}
		s.userDevices[device.UserID] = newDevices
	}

	delete(s.devices, deviceID)
	return nil
}

// GetDevice retrieves a device by ID.
func (s *PushService) GetDevice(deviceID string) (*Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	device, exists := s.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return device, nil
}

// GetUserDevices returns all devices for a user.
func (s *PushService) GetUserDevices(userID string) ([]*Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	deviceIDs, exists := s.userDevices[userID]
	if !exists {
		return []*Device{}, nil
	}

	devices := make([]*Device, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		if d, ok := s.devices[id]; ok {
			devices = append(devices, d)
		}
	}

	return devices, nil
}

// UpdateDevicePreferences updates notification preferences for a device.
func (s *PushService) UpdateDevicePreferences(deviceID string, prefs *DevicePreferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, exists := s.devices[deviceID]
	if !exists {
		return ErrDeviceNotFound
	}

	device.Preferences = prefs
	return nil
}

// UpdatePushToken updates the push token for a device.
func (s *PushService) UpdatePushToken(deviceID, newToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, exists := s.devices[deviceID]
	if !exists {
		return ErrDeviceNotFound
	}

	device.PushToken = newToken
	device.LastActiveAt = time.Now()
	return nil
}

// SendNotification sends a push notification to a specific device.
func (s *PushService) SendNotification(ctx context.Context, notification *PushNotification) error {
	s.mu.Lock()
	device, exists := s.devices[notification.DeviceID]
	s.mu.Unlock()

	if !exists {
		return ErrDeviceNotFound
	}

	if !device.Enabled {
		return nil // Device disabled, skip silently
	}

	// Check quiet hours
	if s.isInQuietHours(device) {
		// Queue for later or skip based on priority
		if notification.Priority != PriorityCritical {
			return nil
		}
	}

	// Check preferences
	if !s.shouldNotify(device, notification) {
		return nil
	}

	notification.ID = uuid.New().String()
	notification.CreatedAt = time.Now()
	notification.Status = StatusPending

	// Get provider and send
	provider, exists := s.providers[device.Platform]
	if !exists {
		notification.Status = StatusFailed
		notification.Error = "no provider for platform"
		s.storeNotification(notification)
		return fmt.Errorf("no provider for platform: %s", device.Platform)
	}

	var lastErr error
	for attempt := 0; attempt < s.config.RetryAttempts; attempt++ {
		if err := provider.Send(ctx, device, notification); err != nil {
			lastErr = err
			time.Sleep(s.config.RetryDelay * time.Duration(attempt+1))
			continue
		}

		now := time.Now()
		notification.SentAt = &now
		notification.Status = StatusSent
		s.storeNotification(notification)
		return nil
	}

	notification.Status = StatusFailed
	notification.Error = lastErr.Error()
	s.storeNotification(notification)
	return ErrNotificationFailed
}

// SendToUser sends a notification to all of a user's devices.
func (s *PushService) SendToUser(ctx context.Context, userID string, title, body string, category NotificationCategory, priority NotificationPriority, data map[string]interface{}) error {
	devices, err := s.GetUserDevices(userID)
	if err != nil {
		return err
	}

	var lastErr error
	for _, device := range devices {
		notification := &PushNotification{
			DeviceID: device.ID,
			UserID:   userID,
			Title:    title,
			Body:     body,
			Category: category,
			Priority: priority,
			Data:     data,
		}

		if err := s.SendNotification(ctx, notification); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// NotifyJobFailure sends a job failure notification.
func (s *PushService) NotifyJobFailure(ctx context.Context, userID, jobID, jobName, errorMsg string) error {
	return s.SendToUser(ctx, userID,
		"Job Failed: "+jobName,
		"Error: "+truncate(errorMsg, 100),
		CategoryJobStatus,
		PriorityHigh,
		map[string]interface{}{
			"job_id":   jobID,
			"job_name": jobName,
			"error":    errorMsg,
			"action":   "view_job",
		},
	)
}

// NotifyExecutionComplete sends an execution completion notification.
func (s *PushService) NotifyExecutionComplete(ctx context.Context, userID, jobID, executionID string, success bool, duration time.Duration) error {
	title := "Execution Completed"
	body := fmt.Sprintf("Job completed in %s", duration.Round(time.Second))
	priority := PriorityNormal

	if !success {
		title = "Execution Failed"
		body = "Job execution failed after retries"
		priority = PriorityHigh
	}

	return s.SendToUser(ctx, userID, title, body, CategoryExecution, priority,
		map[string]interface{}{
			"job_id":       jobID,
			"execution_id": executionID,
			"success":      success,
			"duration_ms":  duration.Milliseconds(),
			"action":       "view_execution",
		},
	)
}

// NotifyApprovalRequired sends an approval request notification.
func (s *PushService) NotifyApprovalRequired(ctx context.Context, userID, approvalID, jobName, message string) error {
	return s.SendToUser(ctx, userID,
		"Approval Required",
		fmt.Sprintf("%s: %s", jobName, truncate(message, 80)),
		CategoryApproval,
		PriorityHigh,
		map[string]interface{}{
			"approval_id": approvalID,
			"job_name":    jobName,
			"action":      "approve",
		},
	)
}

// NotifyClusterIssue sends a cluster health issue notification.
func (s *PushService) NotifyClusterIssue(ctx context.Context, userID, clusterID, issue string) error {
	return s.SendToUser(ctx, userID,
		"Cluster Health Alert",
		issue,
		CategoryClusterHealth,
		PriorityCritical,
		map[string]interface{}{
			"cluster_id": clusterID,
			"action":     "view_cluster",
		},
	)
}

// MarkNotificationRead marks a notification as read.
func (s *PushService) MarkNotificationRead(notificationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	notification, exists := s.notifications[notificationID]
	if !exists {
		return errors.New("notification not found")
	}

	now := time.Now()
	notification.ReadAt = &now
	notification.Status = StatusRead
	return nil
}

// GetUnreadNotifications returns unread notifications for a user.
func (s *PushService) GetUnreadNotifications(userID string, limit int) []*PushNotification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var unread []*PushNotification
	for _, n := range s.notifications {
		if n.UserID == userID && n.ReadAt == nil {
			unread = append(unread, n)
			if limit > 0 && len(unread) >= limit {
				break
			}
		}
	}

	return unread
}

// GetNotificationHistory returns notification history for a user.
func (s *PushService) GetNotificationHistory(userID string, since time.Time, limit int) []*PushNotification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var history []*PushNotification
	for _, n := range s.notifications {
		if n.UserID == userID && n.CreatedAt.After(since) {
			history = append(history, n)
			if limit > 0 && len(history) >= limit {
				break
			}
		}
	}

	return history
}

// RegisterProvider registers a push notification provider.
func (s *PushService) RegisterProvider(platform Platform, provider PushProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[platform] = provider
}

func (s *PushService) storeNotification(n *PushNotification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifications[n.ID] = n
}

func (s *PushService) isInQuietHours(device *Device) bool {
	if device.Preferences == nil || !device.Preferences.QuietHoursEnabled {
		return false
	}

	// Parse quiet hours and check current time
	// Simplified implementation
	return false
}

func (s *PushService) shouldNotify(device *Device, n *PushNotification) bool {
	if device.Preferences == nil {
		return true
	}

	prefs := device.Preferences

	switch n.Category {
	case CategoryJobStatus:
		if n.Priority == PriorityHigh {
			return prefs.NotifyOnJobFailure
		}
		return prefs.NotifyOnJobSuccess

	case CategoryExecution:
		return prefs.NotifyOnExecFailure || prefs.NotifyOnExecRetry

	case CategoryClusterHealth:
		return prefs.NotifyOnClusterIssue

	case CategoryApproval:
		return true // Always notify for approvals

	default:
		return true
	}
}

func defaultPreferences() *DevicePreferences {
	return &DevicePreferences{
		NotifyOnJobFailure:   true,
		NotifyOnJobSuccess:   false,
		NotifyOnJobStart:     false,
		NotifyOnExecFailure:  true,
		NotifyOnExecRetry:    false,
		NotifyOnClusterIssue: true,
		NotifyOnQuotaWarning: true,
		QuietHoursEnabled:    false,
		MinimumSeverity:      "warning",
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// MobileAPIResponse is a standard API response for mobile clients.
type MobileAPIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *APIMeta    `json:"meta,omitempty"`
}

// APIError represents an API error.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// APIMeta contains response metadata.
type APIMeta struct {
	RequestID string `json:"request_id"`
	Timestamp int64  `json:"timestamp"`
	Version   string `json:"version"`
}

// JobSummary is a mobile-optimized job summary.
type JobSummary struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Status          string    `json:"status"`
	Enabled         bool      `json:"enabled"`
	Schedule        string    `json:"schedule"`
	NextRun         *time.Time `json:"next_run,omitempty"`
	LastRunStatus   string    `json:"last_run_status,omitempty"`
	LastRunTime     *time.Time `json:"last_run_time,omitempty"`
	FailureCount24h int       `json:"failure_count_24h"`
}

// ExecutionSummary is a mobile-optimized execution summary.
type ExecutionSummary struct {
	ID          string        `json:"id"`
	JobID       string        `json:"job_id"`
	JobName     string        `json:"job_name"`
	Status      string        `json:"status"`
	StartedAt   time.Time     `json:"started_at"`
	Duration    time.Duration `json:"duration,omitempty"`
	Attempt     int           `json:"attempt"`
	Error       string        `json:"error,omitempty"`
}

// DashboardData is mobile-optimized dashboard data.
type DashboardData struct {
	TotalJobs        int                 `json:"total_jobs"`
	EnabledJobs      int                 `json:"enabled_jobs"`
	FailingJobs      int                 `json:"failing_jobs"`
	Executions24h    int                 `json:"executions_24h"`
	SuccessRate24h   float64             `json:"success_rate_24h"`
	RecentExecutions []*ExecutionSummary `json:"recent_executions"`
	UpcomingJobs     []*JobSummary       `json:"upcoming_jobs"`
	Alerts           []*Alert            `json:"alerts,omitempty"`
}

// Alert represents a system alert for mobile display.
type Alert struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"`
	Severity string    `json:"severity"`
	Title    string    `json:"title"`
	Message  string    `json:"message"`
	JobID    string    `json:"job_id,omitempty"`
	Time     time.Time `json:"time"`
}

// QuickAction represents a quick action available to mobile users.
type QuickAction struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // trigger, enable, disable, approve, reject
	Label       string `json:"label"`
	Icon        string `json:"icon"`
	JobID       string `json:"job_id,omitempty"`
	ApprovalID  string `json:"approval_id,omitempty"`
	Destructive bool   `json:"destructive"`
}

// TriggerJobRequest is the request to manually trigger a job.
type TriggerJobRequest struct {
	JobID    string            `json:"job_id"`
	Variables map[string]string `json:"variables,omitempty"`
}

// ApprovalAction is the request to approve or reject.
type ApprovalAction struct {
	ApprovalID string `json:"approval_id"`
	Action     string `json:"action"` // approve, reject
	Comment    string `json:"comment,omitempty"`
}
