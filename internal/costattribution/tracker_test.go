package costattribution

import (
	"testing"
	"time"
)

func TestTracker_RecordExecution(t *testing.T) {
	tracker := NewTracker(DefaultCostConfig())

	record := tracker.RecordExecution("e1", "j1", "daily-backup", "prod",
		5*time.Second, 1, map[string]string{"team": "platform"})

	if record.ExecutionID != "e1" {
		t.Errorf("expected e1, got %s", record.ExecutionID)
	}
	if record.Cost <= 0 {
		t.Error("expected positive cost")
	}
	// Cost = 5s * 0.001 + 1 * 0.01 = 0.005 + 0.01 = 0.015
	expectedCost := 5*0.001 + 1*0.01
	if record.Cost != expectedCost {
		t.Errorf("expected cost %.4f, got %.4f", expectedCost, record.Cost)
	}
}

func TestTracker_GenerateReport(t *testing.T) {
	tracker := NewTracker(DefaultCostConfig())

	tracker.RecordExecution("e1", "j1", "backup", "prod", 10*time.Second, 1, nil)
	tracker.RecordExecution("e2", "j1", "backup", "prod", 5*time.Second, 2, nil)
	tracker.RecordExecution("e3", "j2", "cleanup", "dev", 3*time.Second, 1, nil)

	report := tracker.GenerateReport(time.Now().Add(-1 * time.Hour))

	if report.Executions != 3 {
		t.Errorf("expected 3 executions, got %d", report.Executions)
	}
	if report.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
	if len(report.ByNamespace) != 2 {
		t.Errorf("expected 2 namespaces, got %d", len(report.ByNamespace))
	}
	if _, ok := report.ByNamespace["prod"]; !ok {
		t.Error("expected prod namespace in report")
	}
	if len(report.ByJob) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(report.ByJob))
	}
	// Jobs should be sorted by cost descending
	if report.ByJob[0].Cost < report.ByJob[1].Cost {
		t.Error("expected jobs sorted by cost descending")
	}
}

func TestTracker_GenerateReport_TimeFilter(t *testing.T) {
	tracker := NewTracker(DefaultCostConfig())

	// Record from "yesterday"
	r := tracker.RecordExecution("e-old", "j1", "old", "default", time.Second, 1, nil)
	tracker.mu.Lock()
	tracker.records[0].Timestamp = time.Now().Add(-48 * time.Hour)
	tracker.mu.Unlock()
	_ = r

	// Record from "now"
	tracker.RecordExecution("e-new", "j2", "new", "default", time.Second, 1, nil)

	report := tracker.GenerateReport(time.Now().Add(-1 * time.Hour))
	if report.Executions != 1 {
		t.Errorf("expected 1 recent execution, got %d", report.Executions)
	}
}

func TestTracker_Budget(t *testing.T) {
	tracker := NewTracker(CostConfig{
		CostPerExecutionSecond: 1.0,
		CostPerWebhookCall:     0,
		Currency:               "USD",
	})

	tracker.SetBudget(&Budget{
		ID:           "b1",
		Name:         "Prod Budget",
		Namespace:    "prod",
		MonthlyLimit: 100.0,
		AlertAt:      80,
	})

	// Record 50 seconds = $50 cost
	tracker.RecordExecution("e1", "j1", "test", "prod", 50*time.Second, 0, nil)

	status, err := tracker.GetBudgetStatus("b1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.CurrentSpend != 50.0 {
		t.Errorf("expected spend 50.0, got %.2f", status.CurrentSpend)
	}
	if status.Percentage != 50.0 {
		t.Errorf("expected 50%%, got %.1f%%", status.Percentage)
	}
	if status.IsWarning {
		t.Error("should not be warning at 50%")
	}
	if status.IsOverBudget {
		t.Error("should not be over budget at 50%")
	}
}

func TestTracker_Budget_Warning(t *testing.T) {
	tracker := NewTracker(CostConfig{
		CostPerExecutionSecond: 1.0,
		CostPerWebhookCall:     0,
		Currency:               "USD",
	})

	tracker.SetBudget(&Budget{
		ID: "b1", Name: "Test", MonthlyLimit: 100.0, AlertAt: 80,
	})

	tracker.RecordExecution("e1", "j1", "test", "", 85*time.Second, 0, nil)

	status, _ := tracker.GetBudgetStatus("b1")
	if !status.IsWarning {
		t.Error("expected warning at 85%")
	}
	if status.IsOverBudget {
		t.Error("should not be over budget at 85%")
	}
}

func TestTracker_Budget_Exceeded(t *testing.T) {
	tracker := NewTracker(CostConfig{
		CostPerExecutionSecond: 1.0,
		CostPerWebhookCall:     0,
		Currency:               "USD",
	})

	tracker.SetBudget(&Budget{
		ID: "b1", Name: "Test", MonthlyLimit: 100.0, AlertAt: 80,
	})

	tracker.RecordExecution("e1", "j1", "test", "", 120*time.Second, 0, nil)

	status, _ := tracker.GetBudgetStatus("b1")
	if !status.IsOverBudget {
		t.Error("expected over budget at 120%")
	}
}

func TestTracker_Budget_NotFound(t *testing.T) {
	tracker := NewTracker(DefaultCostConfig())
	_, err := tracker.GetBudgetStatus("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent budget")
	}
}

func TestTracker_GetTotalCost(t *testing.T) {
	tracker := NewTracker(CostConfig{
		CostPerExecutionSecond: 1.0,
		CostPerWebhookCall:     0,
		Currency:               "USD",
	})

	tracker.RecordExecution("e1", "j1", "test", "", 10*time.Second, 0, nil)
	tracker.RecordExecution("e2", "j2", "test", "", 5*time.Second, 0, nil)

	total := tracker.GetTotalCost(time.Now().Add(-1 * time.Hour))
	if total != 15.0 {
		t.Errorf("expected total 15.0, got %.2f", total)
	}
}

func TestDefaultCostConfig(t *testing.T) {
	cfg := DefaultCostConfig()
	if cfg.Currency != "USD" {
		t.Errorf("expected USD, got %s", cfg.Currency)
	}
	if cfg.CostPerExecutionSecond <= 0 {
		t.Error("expected positive cost per second")
	}
}
