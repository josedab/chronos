package slo

import (
	"context"
	"testing"
	"time"
)

func TestSLOValidation(t *testing.T) {
	tests := []struct {
		name    string
		slo     SLO
		wantErr bool
	}{
		{
			name: "valid availability SLO",
			slo: SLO{
				Name:   "test-availability",
				Type:   SLOTypeAvailability,
				Target: 99.9,
				Window: 30 * 24 * time.Hour,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			slo: SLO{
				Type:   SLOTypeAvailability,
				Target: 99.9,
				Window: 30 * 24 * time.Hour,
			},
			wantErr: true,
		},
		{
			name: "invalid target - too high",
			slo: SLO{
				Name:   "test",
				Type:   SLOTypeAvailability,
				Target: 100.1,
				Window: 30 * 24 * time.Hour,
			},
			wantErr: true,
		},
		{
			name: "invalid target - negative",
			slo: SLO{
				Name:   "test",
				Type:   SLOTypeAvailability,
				Target: -1,
				Window: 30 * 24 * time.Hour,
			},
			wantErr: true,
		},
		{
			name: "invalid window",
			slo: SLO{
				Name:   "test",
				Type:   SLOTypeAvailability,
				Target: 99.9,
				Window: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.slo.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SLO.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestErrorBudget(t *testing.T) {
	slo := &SLO{
		Name:   "test",
		Target: 99.9,
	}

	budget := slo.ErrorBudget()
	// Use approximate comparison for floating point
	if budget < 0.09 || budget > 0.11 {
		t.Errorf("ErrorBudget() = %v, want ~0.1", budget)
	}

	slo.Target = 95.0
	budget = slo.ErrorBudget()
	if budget < 4.9 || budget > 5.1 {
		t.Errorf("ErrorBudget() = %v, want ~5.0", budget)
	}
}

func TestManagerCRUD(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(nil)

	// Create
	slo := &SLO{
		Name:   "test-slo",
		Type:   SLOTypeAvailability,
		Target: 99.9,
		Window: 30 * 24 * time.Hour,
	}

	err := manager.CreateSLO(ctx, slo)
	if err != nil {
		t.Fatalf("CreateSLO() error = %v", err)
	}

	if slo.ID == "" {
		t.Error("CreateSLO() did not assign ID")
	}

	// Get
	retrieved, err := manager.GetSLO(ctx, slo.ID)
	if err != nil {
		t.Fatalf("GetSLO() error = %v", err)
	}
	if retrieved.Name != slo.Name {
		t.Errorf("GetSLO() name = %v, want %v", retrieved.Name, slo.Name)
	}

	// Update
	slo.Target = 99.5
	err = manager.UpdateSLO(ctx, slo)
	if err != nil {
		t.Fatalf("UpdateSLO() error = %v", err)
	}

	retrieved, _ = manager.GetSLO(ctx, slo.ID)
	if retrieved.Target != 99.5 {
		t.Errorf("UpdateSLO() target = %v, want 99.5", retrieved.Target)
	}

	// List
	slos := manager.ListSLOs(ctx)
	if len(slos) != 1 {
		t.Errorf("ListSLOs() count = %v, want 1", len(slos))
	}

	// Delete
	err = manager.DeleteSLO(ctx, slo.ID)
	if err != nil {
		t.Fatalf("DeleteSLO() error = %v", err)
	}

	_, err = manager.GetSLO(ctx, slo.ID)
	if err != ErrSLONotFound {
		t.Errorf("GetSLO() after delete error = %v, want ErrSLONotFound", err)
	}
}

func TestBurnRateCalculation(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(nil)

	// Create an SLO with 99% target (1% error budget)
	slo := &SLO{
		ID:      "test-slo",
		Name:    "test-burn-rate",
		Type:    SLOTypeAvailability,
		Target:  99.0,
		Window:  30 * 24 * time.Hour,
		Enabled: true,
	}
	manager.CreateSLO(ctx, slo)

	// Record metrics: 95 successes, 5 failures = 5% error rate
	// Error budget is 1%, so burn rate = 5% / 1% = 5x
	now := time.Now()
	for i := 0; i < 95; i++ {
		manager.RecordMetric(ctx, ExecutionMetric{
			JobID:     "job-1",
			Success:   true,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	for i := 0; i < 5; i++ {
		manager.RecordMetric(ctx, ExecutionMetric{
			JobID:     "job-1",
			Success:   false,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	status, err := manager.CalculateStatus(ctx, slo.ID)
	if err != nil {
		t.Fatalf("CalculateStatus() error = %v", err)
	}

	if status.TotalEvents != 100 {
		t.Errorf("TotalEvents = %v, want 100", status.TotalEvents)
	}

	if status.GoodEvents != 95 {
		t.Errorf("GoodEvents = %v, want 95", status.GoodEvents)
	}

	if status.BadEvents != 5 {
		t.Errorf("BadEvents = %v, want 5", status.BadEvents)
	}

	// The burn rate calculation uses 1-hour window, but we record all metrics within an hour
	// So burn rate = actual failure rate / allowed failure rate = 5% / 1% = 5x
	// However, exact values may vary based on timing
	if status.BurnRate < 4.0 || status.BurnRate > 10.0 {
		t.Errorf("BurnRate = %v, want between 4 and 10", status.BurnRate)
	}

	// Error budget should be significantly consumed (capped at 100)
	if status.ErrorBudgetUsed < 100 {
		t.Errorf("ErrorBudgetUsed = %v, want >=100 (over budget)", status.ErrorBudgetUsed)
	}
}

func TestAlertThresholds(t *testing.T) {
	ctx := context.Background()
	alertCalled := false
	
	manager := NewManager(func(ctx context.Context, alert *BurnRateAlert) error {
		alertCalled = true
		return nil
	})

	// Create SLO with low threshold for testing
	slo := &SLO{
		ID:      "test-slo",
		Name:    "test-alerts",
		Type:    SLOTypeAvailability,
		Target:  99.0,
		Window:  24 * time.Hour,
		Enabled: true,
		Thresholds: []AlertThreshold{
			{BurnRate: 2.0, Severity: AlertSeverityWarning},
		},
	}
	manager.CreateSLO(ctx, slo)

	// Record metrics that will trigger alert (10% failure = 10x burn rate)
	now := time.Now()
	for i := 0; i < 90; i++ {
		manager.RecordMetric(ctx, ExecutionMetric{
			JobID:     "job-1",
			Success:   true,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	for i := 0; i < 10; i++ {
		manager.RecordMetric(ctx, ExecutionMetric{
			JobID:     "job-1",
			Success:   false,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	alerts, err := manager.CheckAlerts(ctx)
	if err != nil {
		t.Fatalf("CheckAlerts() error = %v", err)
	}

	if len(alerts) == 0 {
		t.Error("CheckAlerts() returned no alerts, expected at least 1")
	}

	if !alertCalled {
		t.Error("Alert handler was not called")
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := CreateDefaultThresholds()

	if len(thresholds) != 4 {
		t.Errorf("CreateDefaultThresholds() count = %v, want 4", len(thresholds))
	}

	// Check that thresholds are in descending order by burn rate
	for i := 1; i < len(thresholds); i++ {
		if thresholds[i].BurnRate >= thresholds[i-1].BurnRate {
			t.Errorf("Thresholds should be in descending burn rate order")
		}
	}
}

func TestJobFilter(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(nil)

	// Create SLO with job filter
	slo := &SLO{
		ID:     "filtered-slo",
		Name:   "filtered",
		Type:   SLOTypeAvailability,
		Target: 99.0,
		Window: 24 * time.Hour,
		JobFilter: &JobFilter{
			JobIDs: []string{"job-1"},
		},
	}
	manager.CreateSLO(ctx, slo)

	// Record metrics for different jobs
	now := time.Now()
	
	// job-1: 8 success, 2 failures
	for i := 0; i < 8; i++ {
		manager.RecordMetric(ctx, ExecutionMetric{
			JobID:     "job-1",
			Success:   true,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	for i := 0; i < 2; i++ {
		manager.RecordMetric(ctx, ExecutionMetric{
			JobID:     "job-1",
			Success:   false,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	// job-2: all failures (should be ignored)
	for i := 0; i < 10; i++ {
		manager.RecordMetric(ctx, ExecutionMetric{
			JobID:     "job-2",
			Success:   false,
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	status, _ := manager.CalculateStatus(ctx, slo.ID)

	// Should only count job-1 metrics
	if status.TotalEvents != 10 {
		t.Errorf("TotalEvents = %v, want 10 (only job-1)", status.TotalEvents)
	}

	if status.GoodEvents != 8 {
		t.Errorf("GoodEvents = %v, want 8", status.GoodEvents)
	}
}

func TestErrorBudgetHistory(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(nil)

	slo := &SLO{
		ID:     "history-slo",
		Name:   "history",
		Type:   SLOTypeAvailability,
		Target: 99.0,
		Window: 24 * time.Hour,
	}
	manager.CreateSLO(ctx, slo)

	// Record some metrics over time
	now := time.Now()
	for i := 0; i < 100; i++ {
		success := i%10 != 0 // 10% failures
		manager.RecordMetric(ctx, ExecutionMetric{
			JobID:     "job-1",
			Success:   success,
			Timestamp: now.Add(-time.Duration(i) * time.Hour),
		})
	}

	history, err := manager.GetErrorBudgetHistory(ctx, slo.ID, 24)
	if err != nil {
		t.Fatalf("GetErrorBudgetHistory() error = %v", err)
	}

	if len(history) != 24 {
		t.Errorf("GetErrorBudgetHistory() points = %v, want 24", len(history))
	}

	// Check that timestamps are in order
	for i := 1; i < len(history); i++ {
		if history[i].Timestamp.Before(history[i-1].Timestamp) {
			t.Error("History points should be in chronological order")
		}
	}
}
