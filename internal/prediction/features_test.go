package prediction

import (
	"context"
	"testing"
	"time"
)

func TestFeatureExtractor_ExtractFeatures(t *testing.T) {
	extractor := NewFeatureExtractor()

	// Create test records
	records := generateTestRecords(50, 0.2) // 20% failure rate

	features, err := extractor.ExtractFeatures(records, time.Now())
	if err != nil {
		t.Fatalf("ExtractFeatures failed: %v", err)
	}

	// Verify time-based features
	if features.HourOfDay < 0 || features.HourOfDay > 23 {
		t.Errorf("HourOfDay out of range: %d", features.HourOfDay)
	}

	if features.DayOfWeek < 0 || features.DayOfWeek > 6 {
		t.Errorf("DayOfWeek out of range: %d", features.DayOfWeek)
	}

	// Verify historical failure rate is reasonable
	if features.HistoricalFailureRate < 0 || features.HistoricalFailureRate > 1 {
		t.Errorf("HistoricalFailureRate out of range: %f", features.HistoricalFailureRate)
	}

	// Verify total executions
	if features.TotalExecutions != 50 {
		t.Errorf("TotalExecutions = %d, want 50", features.TotalExecutions)
	}

	// Verify stability score is calculated
	if features.StabilityScore < 0 || features.StabilityScore > 1 {
		t.Errorf("StabilityScore out of range: %f", features.StabilityScore)
	}

	// Verify health score is calculated
	if features.HealthScore < 0 || features.HealthScore > 1 {
		t.Errorf("HealthScore out of range: %f", features.HealthScore)
	}
}

func TestFeatureExtractor_ConsecutiveFailures(t *testing.T) {
	extractor := NewFeatureExtractor()

	// Create records with consecutive failures at the end
	records := make([]ExecutionRecord, 20)
	baseTime := time.Now().Add(-20 * time.Minute)
	for i := 0; i < 20; i++ {
		records[i] = ExecutionRecord{
			JobID:       "test-job",
			ExecutionID: "exec-" + string(rune('a'+i)),
			Timestamp:   baseTime.Add(time.Duration(i) * time.Minute),
			Duration:    100 * time.Millisecond,
			Success:     i < 15, // Last 5 are failures
		}
	}

	features, err := extractor.ExtractFeatures(records, time.Now())
	if err != nil {
		t.Fatalf("ExtractFeatures failed: %v", err)
	}

	if features.ConsecutiveFailures != 5 {
		t.Errorf("ConsecutiveFailures = %d, want 5", features.ConsecutiveFailures)
	}

	if features.ConsecutiveSuccesses != 0 {
		t.Errorf("ConsecutiveSuccesses = %d, want 0", features.ConsecutiveSuccesses)
	}
}

func TestGradientBoostingPredictor_Train(t *testing.T) {
	predictor := NewGradientBoostingPredictor(10, 0.1)
	extractor := NewFeatureExtractor()

	// Generate training data
	var features []*FeatureSet
	var labels []bool

	// Generate features for successful executions
	for i := 0; i < 50; i++ {
		records := generateTestRecords(20, 0.1) // Low failure rate
		fs, err := extractor.ExtractFeatures(records, time.Now())
		if err != nil {
			t.Fatalf("ExtractFeatures failed: %v", err)
		}
		features = append(features, fs)
		labels = append(labels, false) // Success
	}

	// Generate features for failing executions
	for i := 0; i < 50; i++ {
		records := generateTestRecords(20, 0.7) // High failure rate
		fs, err := extractor.ExtractFeatures(records, time.Now())
		if err != nil {
			t.Fatalf("ExtractFeatures failed: %v", err)
		}
		features = append(features, fs)
		labels = append(labels, true) // Failure
	}

	err := predictor.Train(features, labels)
	if err != nil {
		t.Fatalf("Train failed: %v", err)
	}

	// Test prediction
	testRecords := generateTestRecords(20, 0.5)
	testFeatures, _ := extractor.ExtractFeatures(testRecords, time.Now())
	
	prob, err := predictor.Predict(testFeatures)
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if prob < 0 || prob > 1 {
		t.Errorf("Probability out of range: %f", prob)
	}
}

func TestGradientBoostingPredictor_Evaluate(t *testing.T) {
	predictor := NewGradientBoostingPredictor(10, 0.1)
	extractor := NewFeatureExtractor()

	// Generate training data
	var features []*FeatureSet
	var labels []bool

	for i := 0; i < 50; i++ {
		records := generateTestRecords(20, 0.1)
		fs, _ := extractor.ExtractFeatures(records, time.Now())
		features = append(features, fs)
		labels = append(labels, false)
	}

	for i := 0; i < 50; i++ {
		records := generateTestRecords(20, 0.7)
		fs, _ := extractor.ExtractFeatures(records, time.Now())
		features = append(features, fs)
		labels = append(labels, true)
	}

	if err := predictor.Train(features, labels); err != nil {
		t.Fatalf("Train failed: %v", err)
	}

	metrics, err := predictor.Evaluate(features, labels)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if metrics.Accuracy < 0 || metrics.Accuracy > 1 {
		t.Errorf("Accuracy out of range: %f", metrics.Accuracy)
	}

	if metrics.SampleCount != 100 {
		t.Errorf("SampleCount = %d, want 100", metrics.SampleCount)
	}
}

func TestGradientBoostingPredictor_Serialize(t *testing.T) {
	predictor := NewGradientBoostingPredictor(5, 0.1)
	extractor := NewFeatureExtractor()

	var features []*FeatureSet
	var labels []bool

	for i := 0; i < 20; i++ {
		records := generateTestRecords(20, 0.2)
		fs, _ := extractor.ExtractFeatures(records, time.Now())
		features = append(features, fs)
		labels = append(labels, i%2 == 0)
	}

	predictor.Train(features, labels)

	// Serialize
	data, err := predictor.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Create new predictor and deserialize
	newPredictor := NewGradientBoostingPredictor(5, 0.1)
	if err := newPredictor.Deserialize(data); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	// Test prediction on both
	testRecords := generateTestRecords(20, 0.3)
	testFeatures, _ := extractor.ExtractFeatures(testRecords, time.Now())

	prob1, _ := predictor.Predict(testFeatures)
	prob2, _ := newPredictor.Predict(testFeatures)

	if prob1 != prob2 {
		t.Errorf("Predictions differ after deserialization: %f vs %f", prob1, prob2)
	}
}

func TestNotificationManager_Send(t *testing.T) {
	manager := NewNotificationManager()

	// Create a test webhook channel (won't actually send)
	testChannel := &testNotificationChannel{name: "test"}
	if err := manager.RegisterChannel(testChannel); err != nil {
		t.Fatalf("RegisterChannel failed: %v", err)
	}

	// Add a rule
	manager.AddRule(NotificationRule{
		ID:          "rule1",
		Name:        "Test Rule",
		Enabled:     true,
		Channels:    []string{"test"},
		MinSeverity: SeverityWarning,
	})

	// Send a notification
	notification := &Notification{
		ID:       "notif1",
		Type:     NotificationPredictedFailure,
		Severity: SeverityError,
		Title:    "Test Alert",
		Message:  "This is a test",
		JobID:    "job-123",
	}

	err := manager.Send(context.Background(), notification)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if testChannel.sendCount != 1 {
		t.Errorf("Expected 1 send, got %d", testChannel.sendCount)
	}
}

func TestNotificationManager_RateLimiting(t *testing.T) {
	manager := NewNotificationManager()
	manager.rateLimiter.maxBurst = 2
	manager.rateLimiter.window = time.Minute

	testChannel := &testNotificationChannel{name: "test"}
	manager.RegisterChannel(testChannel)

	manager.AddRule(NotificationRule{
		ID:          "rule1",
		Name:        "Test Rule",
		Enabled:     true,
		Channels:    []string{"test"},
		MinSeverity: SeverityInfo,
	})

	// Send multiple notifications quickly
	for i := 0; i < 5; i++ {
		notification := &Notification{
			ID:       "notif" + string(rune('0'+i)),
			Type:     NotificationType("type" + string(rune('0'+i))),
			Severity: SeverityError,
			JobID:    "job" + string(rune('0'+i)),
		}
		manager.deduplicator.ttl = 0 // Disable deduplication for this test
		manager.Send(context.Background(), notification)
	}

	// Should be rate limited to 2
	if testChannel.sendCount > 2 {
		t.Errorf("Rate limiting failed: sent %d, expected <= 2", testChannel.sendCount)
	}
}

func TestNotificationManager_Deduplication(t *testing.T) {
	manager := NewNotificationManager()

	testChannel := &testNotificationChannel{name: "test"}
	manager.RegisterChannel(testChannel)

	manager.AddRule(NotificationRule{
		ID:          "rule1",
		Name:        "Test Rule",
		Enabled:     true,
		Channels:    []string{"test"},
		MinSeverity: SeverityInfo,
	})

	// Send same notification twice
	notification := &Notification{
		ID:       "notif1",
		Type:     NotificationPredictedFailure,
		Severity: SeverityError,
		JobID:    "job-123",
	}

	manager.Send(context.Background(), notification)
	manager.Send(context.Background(), notification)

	// Should be deduplicated to 1
	if testChannel.sendCount != 1 {
		t.Errorf("Deduplication failed: sent %d, expected 1", testChannel.sendCount)
	}
}

func TestSlackChannel_Name(t *testing.T) {
	channel := NewSlackChannel(SlackConfig{
		WebhookURL: "https://hooks.slack.com/test",
	})

	if channel.Name() != "slack" {
		t.Errorf("Name = %q, want slack", channel.Name())
	}
}

func TestPagerDutyChannel_Name(t *testing.T) {
	channel := NewPagerDutyChannel(PagerDutyConfig{
		RoutingKey: "test-key",
	})

	if channel.Name() != "pagerduty" {
		t.Errorf("Name = %q, want pagerduty", channel.Name())
	}
}

func TestEmailChannel_Name(t *testing.T) {
	channel := NewEmailChannel(EmailConfig{
		SMTPHost:   "smtp.example.com",
		From:       "test@example.com",
		Recipients: []string{"admin@example.com"},
	})

	if channel.Name() != "email" {
		t.Errorf("Name = %q, want email", channel.Name())
	}
}

func TestWebhookChannel_CustomName(t *testing.T) {
	channel := NewWebhookChannel(WebhookConfig{
		Name: "custom-webhook",
		URL:  "https://example.com/webhook",
	})

	if channel.Name() != "custom-webhook" {
		t.Errorf("Name = %q, want custom-webhook", channel.Name())
	}
}

// Helper to generate test execution records.
func generateTestRecords(count int, failureRate float64) []ExecutionRecord {
	records := make([]ExecutionRecord, count)
	baseTime := time.Now().Add(-time.Duration(count) * time.Minute)

	for i := 0; i < count; i++ {
		success := float64(i)/float64(count) > failureRate
		errorType := ""
		if !success {
			errorType = "timeout"
		}

		records[i] = ExecutionRecord{
			JobID:       "test-job",
			ExecutionID: "exec-" + string(rune('a'+i)),
			Timestamp:   baseTime.Add(time.Duration(i) * time.Minute),
			Duration:    time.Duration(100+i*10) * time.Millisecond,
			Success:     success,
			ErrorType:   errorType,
			RetryCount:  i % 3,
		}
		records[i].DayOfWeek = int(records[i].Timestamp.Weekday())
		records[i].HourOfDay = records[i].Timestamp.Hour()
	}

	return records
}

// Test notification channel for unit tests.
type testNotificationChannel struct {
	name      string
	sendCount int
}

func (t *testNotificationChannel) Name() string {
	return t.name
}

func (t *testNotificationChannel) Send(ctx context.Context, n *Notification) error {
	t.sendCount++
	return nil
}

func (t *testNotificationChannel) Test(ctx context.Context) error {
	return nil
}
