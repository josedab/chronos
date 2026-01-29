package mobile

import (
	"context"
	"testing"
	"time"
)

func TestDefaultPushConfig(t *testing.T) {
	cfg := DefaultPushConfig()

	if cfg.RetryAttempts != 3 {
		t.Errorf("RetryAttempts = %d, want 3", cfg.RetryAttempts)
	}
	if cfg.RetryDelay != time.Second {
		t.Errorf("RetryDelay = %v, want 1s", cfg.RetryDelay)
	}
}

func TestNewPushService(t *testing.T) {
	cfg := DefaultPushConfig()
	svc := NewPushService(cfg)

	if svc == nil {
		t.Fatal("NewPushService returned nil")
	}
}

func TestPushService_RegisterDevice(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	deviceInfo := &DeviceInfo{
		Model:      "iPhone 14",
		OSVersion:  "iOS 17.0",
		AppVersion: "1.0.0",
		Timezone:   "America/New_York",
	}

	device, err := svc.RegisterDevice(ctx, "user-1", PlatformIOS, "push-token-123", deviceInfo)
	if err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}

	if device.ID == "" {
		t.Error("device ID should be set")
	}
	if device.UserID != "user-1" {
		t.Errorf("UserID = %s, want user-1", device.UserID)
	}
	if device.Platform != PlatformIOS {
		t.Errorf("Platform = %s, want ios", device.Platform)
	}
	if device.DeviceModel != "iPhone 14" {
		t.Errorf("DeviceModel = %s, want iPhone 14", device.DeviceModel)
	}
	if !device.Enabled {
		t.Error("device should be enabled")
	}
	if device.Preferences == nil {
		t.Error("preferences should be set")
	}
}

func TestPushService_RegisterDevice_UpdateExisting(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	// Register initial device
	svc.RegisterDevice(ctx, "user-1", PlatformIOS, "token-123", nil)

	// Re-register same token with different user
	device, err := svc.RegisterDevice(ctx, "user-2", PlatformIOS, "token-123", nil)
	if err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}

	if device.UserID != "user-2" {
		t.Errorf("UserID should be updated to user-2")
	}

	// Should only have one device
	if len(svc.devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(svc.devices))
	}
}

func TestPushService_UnregisterDevice(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	device, _ := svc.RegisterDevice(ctx, "user-1", PlatformIOS, "token-123", nil)

	err := svc.UnregisterDevice(ctx, device.ID)
	if err != nil {
		t.Fatalf("UnregisterDevice failed: %v", err)
	}

	_, err = svc.GetDevice(device.ID)
	if err != ErrDeviceNotFound {
		t.Error("device should not be found after unregistration")
	}
}

func TestPushService_UnregisterDevice_NotFound(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	err := svc.UnregisterDevice(ctx, "nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestPushService_GetDevice(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	device, _ := svc.RegisterDevice(ctx, "user-1", PlatformIOS, "token-123", nil)

	got, err := svc.GetDevice(device.ID)
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}

	if got.ID != device.ID {
		t.Errorf("device ID = %s, want %s", got.ID, device.ID)
	}
}

func TestPushService_GetDevice_NotFound(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	_, err := svc.GetDevice("nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestPushService_GetUserDevices(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	// Register multiple devices for same user
	svc.RegisterDevice(ctx, "user-1", PlatformIOS, "token-1", nil)
	svc.RegisterDevice(ctx, "user-1", PlatformAndroid, "token-2", nil)
	svc.RegisterDevice(ctx, "user-2", PlatformIOS, "token-3", nil)

	devices, err := svc.GetUserDevices("user-1")
	if err != nil {
		t.Fatalf("GetUserDevices failed: %v", err)
	}

	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

func TestPushService_GetUserDevices_NoDevices(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	devices, err := svc.GetUserDevices("user-1")
	if err != nil {
		t.Fatalf("GetUserDevices failed: %v", err)
	}

	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
}

func TestPushService_UpdateDevicePreferences(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	device, _ := svc.RegisterDevice(ctx, "user-1", PlatformIOS, "token-123", nil)

	newPrefs := &DevicePreferences{
		NotifyOnJobFailure:  false,
		NotifyOnJobSuccess:  true,
		QuietHoursEnabled:   true,
		QuietHoursStart:     "22:00",
		QuietHoursEnd:       "08:00",
	}

	err := svc.UpdateDevicePreferences(device.ID, newPrefs)
	if err != nil {
		t.Fatalf("UpdateDevicePreferences failed: %v", err)
	}

	got, _ := svc.GetDevice(device.ID)
	if got.Preferences.NotifyOnJobSuccess != true {
		t.Error("preferences should be updated")
	}
}

func TestPushService_UpdateDevicePreferences_NotFound(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	err := svc.UpdateDevicePreferences("nonexistent", &DevicePreferences{})
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestPushService_UpdatePushToken(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	device, _ := svc.RegisterDevice(ctx, "user-1", PlatformIOS, "old-token", nil)

	err := svc.UpdatePushToken(device.ID, "new-token")
	if err != nil {
		t.Fatalf("UpdatePushToken failed: %v", err)
	}

	got, _ := svc.GetDevice(device.ID)
	if got.PushToken != "new-token" {
		t.Errorf("PushToken = %s, want new-token", got.PushToken)
	}
}

func TestPushService_UpdatePushToken_NotFound(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	err := svc.UpdatePushToken("nonexistent", "new-token")
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestPushService_SendNotification_DeviceNotFound(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	notification := &PushNotification{
		DeviceID: "nonexistent",
		Title:    "Test",
		Body:     "Test body",
	}

	err := svc.SendNotification(ctx, notification)
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestPushService_SendNotification_DisabledDevice(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	device, _ := svc.RegisterDevice(ctx, "user-1", PlatformIOS, "token-123", nil)

	// Disable the device
	svc.mu.Lock()
	svc.devices[device.ID].Enabled = false
	svc.mu.Unlock()

	notification := &PushNotification{
		DeviceID: device.ID,
		Title:    "Test",
		Body:     "Test body",
	}

	err := svc.SendNotification(ctx, notification)
	if err != nil {
		t.Errorf("expected no error for disabled device, got %v", err)
	}
}

func TestPushService_RegisterProvider(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	provider := &mockPushProvider{}
	svc.RegisterProvider(PlatformIOS, provider)

	if svc.providers[PlatformIOS] != provider {
		t.Error("provider not registered")
	}
}

func TestPushService_MarkNotificationRead(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	notification := &PushNotification{
		ID:     "notif-1",
		UserID: "user-1",
		Title:  "Test",
		Body:   "Test body",
		Status: StatusSent,
	}
	svc.notifications[notification.ID] = notification

	err := svc.MarkNotificationRead("notif-1")
	if err != nil {
		t.Fatalf("MarkNotificationRead failed: %v", err)
	}

	if notification.Status != StatusRead {
		t.Errorf("status = %s, want read", notification.Status)
	}
	if notification.ReadAt == nil {
		t.Error("ReadAt should be set")
	}
}

func TestPushService_MarkNotificationRead_NotFound(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	err := svc.MarkNotificationRead("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent notification")
	}
}

func TestPushService_GetUnreadNotifications(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	// Add notifications
	now := time.Now()
	svc.notifications["1"] = &PushNotification{ID: "1", UserID: "user-1", Title: "Unread 1", ReadAt: nil}
	svc.notifications["2"] = &PushNotification{ID: "2", UserID: "user-1", Title: "Read", ReadAt: &now}
	svc.notifications["3"] = &PushNotification{ID: "3", UserID: "user-1", Title: "Unread 2", ReadAt: nil}
	svc.notifications["4"] = &PushNotification{ID: "4", UserID: "user-2", Title: "Other user", ReadAt: nil}

	unread := svc.GetUnreadNotifications("user-1", 10)

	if len(unread) != 2 {
		t.Errorf("expected 2 unread notifications, got %d", len(unread))
	}
}

func TestPushService_GetUnreadNotifications_WithLimit(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	svc.notifications["1"] = &PushNotification{ID: "1", UserID: "user-1", ReadAt: nil}
	svc.notifications["2"] = &PushNotification{ID: "2", UserID: "user-1", ReadAt: nil}
	svc.notifications["3"] = &PushNotification{ID: "3", UserID: "user-1", ReadAt: nil}

	unread := svc.GetUnreadNotifications("user-1", 2)

	if len(unread) != 2 {
		t.Errorf("expected 2 notifications with limit, got %d", len(unread))
	}
}

func TestPushService_GetNotificationHistory(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	now := time.Now()
	svc.notifications["1"] = &PushNotification{ID: "1", UserID: "user-1", CreatedAt: now}
	svc.notifications["2"] = &PushNotification{ID: "2", UserID: "user-1", CreatedAt: now.Add(-time.Hour)}
	svc.notifications["3"] = &PushNotification{ID: "3", UserID: "user-1", CreatedAt: now.Add(-48 * time.Hour)}
	svc.notifications["4"] = &PushNotification{ID: "4", UserID: "user-2", CreatedAt: now}

	history := svc.GetNotificationHistory("user-1", now.Add(-24*time.Hour), 10)

	if len(history) != 2 {
		t.Errorf("expected 2 notifications in history, got %d", len(history))
	}
}

func TestPlatformConstants(t *testing.T) {
	if string(PlatformIOS) != "ios" {
		t.Errorf("PlatformIOS = %s, want ios", PlatformIOS)
	}
	if string(PlatformAndroid) != "android" {
		t.Errorf("PlatformAndroid = %s, want android", PlatformAndroid)
	}
}

func TestNotificationCategoryConstants(t *testing.T) {
	categories := map[NotificationCategory]string{
		CategoryJobStatus:     "job_status",
		CategoryExecution:     "execution",
		CategoryClusterHealth: "cluster_health",
		CategorySystem:        "system",
		CategoryApproval:      "approval",
		CategoryAlert:         "alert",
	}

	for cat, expected := range categories {
		if string(cat) != expected {
			t.Errorf("category %s = %s, want %s", cat, string(cat), expected)
		}
	}
}

func TestNotificationPriorityConstants(t *testing.T) {
	priorities := map[NotificationPriority]string{
		PriorityLow:      "low",
		PriorityNormal:   "normal",
		PriorityHigh:     "high",
		PriorityCritical: "critical",
	}

	for p, expected := range priorities {
		if string(p) != expected {
			t.Errorf("priority %s = %s, want %s", p, string(p), expected)
		}
	}
}

func TestNotificationStatusConstants(t *testing.T) {
	statuses := map[NotificationStatus]string{
		StatusPending:   "pending",
		StatusSent:      "sent",
		StatusDelivered: "delivered",
		StatusRead:      "read",
		StatusFailed:    "failed",
	}

	for s, expected := range statuses {
		if string(s) != expected {
			t.Errorf("status %s = %s, want %s", s, string(s), expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long message", 10, "this is..."},
		{"exact", 5, "exact"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestDefaultPreferences(t *testing.T) {
	prefs := defaultPreferences()

	if !prefs.NotifyOnJobFailure {
		t.Error("NotifyOnJobFailure should be true by default")
	}
	if prefs.NotifyOnJobSuccess {
		t.Error("NotifyOnJobSuccess should be false by default")
	}
	if !prefs.NotifyOnClusterIssue {
		t.Error("NotifyOnClusterIssue should be true by default")
	}
}

func TestShouldNotify(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())

	device := &Device{
		Preferences: &DevicePreferences{
			NotifyOnJobFailure:   true,
			NotifyOnJobSuccess:   false,
			NotifyOnClusterIssue: true,
		},
	}

	tests := []struct {
		name     string
		category NotificationCategory
		priority NotificationPriority
		want     bool
	}{
		{"job failure", CategoryJobStatus, PriorityHigh, true},
		{"job success", CategoryJobStatus, PriorityNormal, false},
		{"cluster health", CategoryClusterHealth, PriorityHigh, true},
		{"approval always", CategoryApproval, PriorityNormal, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notification := &PushNotification{
				Category: tt.category,
				Priority: tt.priority,
			}
			got := svc.shouldNotify(device, notification)
			if got != tt.want {
				t.Errorf("shouldNotify = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldNotify_NilPreferences(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	device := &Device{Preferences: nil}

	notification := &PushNotification{Category: CategoryJobStatus}
	if !svc.shouldNotify(device, notification) {
		t.Error("should notify when preferences are nil")
	}
}

func TestConcurrentDeviceAccess(t *testing.T) {
	svc := NewPushService(DefaultPushConfig())
	ctx := context.Background()

	done := make(chan bool)

	// Register devices concurrently
	go func() {
		for i := 0; i < 100; i++ {
			svc.RegisterDevice(ctx, "user-1", PlatformIOS, "token-"+string(rune('0'+i)), nil)
		}
		done <- true
	}()

	// Read devices concurrently
	go func() {
		for i := 0; i < 100; i++ {
			svc.GetUserDevices("user-1")
		}
		done <- true
	}()

	<-done
	<-done
}

// Mock provider for testing
type mockPushProvider struct {
	sendCalled bool
	lastDevice *Device
	lastNotif  *PushNotification
	shouldFail bool
}

func (m *mockPushProvider) Send(ctx context.Context, device *Device, notification *PushNotification) error {
	m.sendCalled = true
	m.lastDevice = device
	m.lastNotif = notification
	if m.shouldFail {
		return ErrNotificationFailed
	}
	return nil
}

func (m *mockPushProvider) ValidateToken(platform Platform, token string) error {
	if token == "" {
		return ErrInvalidPushToken
	}
	return nil
}
