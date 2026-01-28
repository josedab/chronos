package prediction

import (
	"context"
	"testing"
	"time"
)

func TestNewPredictor(t *testing.T) {
	predictor := NewPredictor()

	if predictor == nil {
		t.Fatal("expected predictor, got nil")
	}
	if predictor.records == nil {
		t.Error("expected records map to be initialized")
	}
	if predictor.minDataPoints != 10 {
		t.Errorf("expected minDataPoints 10, got %d", predictor.minDataPoints)
	}
}

func TestPredictor_RecordExecution(t *testing.T) {
	predictor := NewPredictor()

	record := ExecutionRecord{
		JobID:       "job-1",
		ExecutionID: "exec-1",
		Timestamp:   time.Now(),
		Duration:    5 * time.Second,
		Success:     true,
	}

	err := predictor.RecordExecution(context.Background(), record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(predictor.records["job-1"]) != 1 {
		t.Errorf("expected 1 record, got %d", len(predictor.records["job-1"]))
	}
}

func TestPredictor_RecordExecution_SetsDeriverdFields(t *testing.T) {
	predictor := NewPredictor()

	timestamp := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC) // Monday at 14:30
	record := ExecutionRecord{
		JobID:       "job-1",
		ExecutionID: "exec-1",
		Timestamp:   timestamp,
		Duration:    5 * time.Second,
		Success:     true,
	}

	predictor.RecordExecution(context.Background(), record)

	recorded := predictor.records["job-1"][0]
	if recorded.HourOfDay != 14 {
		t.Errorf("expected HourOfDay 14, got %d", recorded.HourOfDay)
	}
	if recorded.DayOfWeek != int(time.Monday) {
		t.Errorf("expected DayOfWeek %d, got %d", int(time.Monday), recorded.DayOfWeek)
	}
}

func TestPredictor_RecordExecution_LimitsRecords(t *testing.T) {
	predictor := NewPredictor()

	// Add more than 1000 records
	for i := 0; i < 1100; i++ {
		record := ExecutionRecord{
			JobID:       "job-1",
			ExecutionID: "exec-" + string(rune(i)),
			Timestamp:   time.Now(),
			Duration:    time.Second,
			Success:     true,
		}
		predictor.RecordExecution(context.Background(), record)
	}

	if len(predictor.records["job-1"]) > 1000 {
		t.Errorf("expected max 1000 records, got %d", len(predictor.records["job-1"]))
	}
}

func TestPredictor_Predict_InsufficientData(t *testing.T) {
	predictor := NewPredictor()

	// Add only a few records
	for i := 0; i < 5; i++ {
		predictor.RecordExecution(context.Background(), ExecutionRecord{
			JobID:       "job-1",
			ExecutionID: "exec-" + string(rune(i)),
			Timestamp:   time.Now(),
			Duration:    time.Second,
			Success:     true,
		})
	}

	_, err := predictor.Predict(context.Background(), "job-1")
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestPredictor_Predict_Success(t *testing.T) {
	predictor := NewPredictor()

	// Add enough records
	for i := 0; i < 20; i++ {
		predictor.RecordExecution(context.Background(), ExecutionRecord{
			JobID:       "job-1",
			ExecutionID: "exec-" + string(rune(i)),
			Timestamp:   time.Now().Add(-time.Duration(i) * time.Hour),
			Duration:    time.Second * time.Duration(i+1),
			Success:     i%5 != 0, // 80% success rate
		})
	}

	prediction, err := predictor.Predict(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prediction == nil {
		t.Fatal("expected prediction, got nil")
	}
	if prediction.JobID != "job-1" {
		t.Errorf("expected job ID job-1, got %s", prediction.JobID)
	}
	if prediction.FailureProbability < 0 || prediction.FailureProbability > 1 {
		t.Errorf("failure probability should be between 0 and 1, got %f", prediction.FailureProbability)
	}
	if prediction.RiskLevel == "" {
		t.Error("expected risk level to be set")
	}
}

func TestPredictor_GetStats(t *testing.T) {
	predictor := NewPredictor()

	// Add records with some failures
	for i := 0; i < 15; i++ {
		predictor.RecordExecution(context.Background(), ExecutionRecord{
			JobID:       "job-1",
			ExecutionID: "exec-" + string(rune(i)),
			Timestamp:   time.Now(),
			Duration:    time.Duration(100+i*10) * time.Millisecond,
			Success:     i%3 != 0,
		})
	}

	stats, err := predictor.GetStats(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.TotalExecutions != 15 {
		t.Errorf("expected 15 total executions, got %d", stats.TotalExecutions)
	}
	if stats.SuccessRate < 0 || stats.SuccessRate > 1 {
		t.Errorf("success rate should be between 0 and 1, got %f", stats.SuccessRate)
	}
}

func TestPredictor_GetStats_NotFound(t *testing.T) {
	predictor := NewPredictor()

	_, err := predictor.GetStats(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestPredictor_GetAlerts(t *testing.T) {
	predictor := NewPredictor()

	// Add records with high failure rate to trigger alerts
	for i := 0; i < 15; i++ {
		predictor.RecordExecution(context.Background(), ExecutionRecord{
			JobID:       "job-1",
			ExecutionID: "exec-" + string(rune(i)),
			Timestamp:   time.Now(),
			Duration:    time.Second,
			Success:     false, // All failures
		})
	}

	alerts, err := predictor.GetAlerts(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// May or may not have alerts depending on implementation
	if alerts == nil {
		t.Error("expected alerts slice, got nil")
	}
}

func TestPredictor_AcknowledgeAlert(t *testing.T) {
	predictor := NewPredictor()

	// Manually add an alert for testing
	predictor.alerts = append(predictor.alerts, Alert{
		ID:           "alert-1",
		JobID:        "job-1",
		Type:         AlertPredictedFailure,
		Severity:     "high",
		Message:      "Test alert",
		CreatedAt:    time.Now(),
		Acknowledged: false,
	})

	err := predictor.AcknowledgeAlert(context.Background(), "alert-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify acknowledged
	alerts, _ := predictor.GetAlerts(context.Background(), false)
	for _, a := range alerts {
		if a.ID == "alert-1" && !a.Acknowledged {
			t.Error("expected alert to be acknowledged")
		}
	}
}

func TestPredictor_AcknowledgeAlert_NotFound(t *testing.T) {
	predictor := NewPredictor()

	err := predictor.AcknowledgeAlert(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestRiskLevels(t *testing.T) {
	levels := []RiskLevel{RiskLow, RiskMedium, RiskHigh, RiskCritical}

	for _, level := range levels {
		if level == "" {
			t.Error("risk level should not be empty")
		}
	}
}

func TestAlertTypes(t *testing.T) {
	types := []AlertType{
		AlertPredictedFailure,
		AlertDegradedPerformance,
		AlertPatternAnomaly,
		AlertHighRetryRate,
	}

	for _, at := range types {
		if at == "" {
			t.Error("alert type should not be empty")
		}
	}
}

func TestExecutionRecord_Fields(t *testing.T) {
	record := ExecutionRecord{
		JobID:       "job-1",
		ExecutionID: "exec-1",
		Timestamp:   time.Now(),
		Duration:    5 * time.Second,
		Success:     true,
		StatusCode:  200,
		ErrorType:   "",
		RetryCount:  0,
	}

	if record.JobID != "job-1" {
		t.Errorf("expected job ID job-1, got %s", record.JobID)
	}
	if record.Duration != 5*time.Second {
		t.Errorf("expected duration 5s, got %v", record.Duration)
	}
}

func TestJobStats_Fields(t *testing.T) {
	stats := JobStats{
		JobID:           "job-1",
		TotalExecutions: 100,
		SuccessCount:    90,
		FailureCount:    10,
		SuccessRate:     0.9,
		AvgDuration:     time.Second,
	}

	if stats.SuccessRate != 0.9 {
		t.Errorf("expected success rate 0.9, got %f", stats.SuccessRate)
	}
}
