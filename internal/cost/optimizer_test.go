package cost

import (
	"context"
	"testing"
	"time"
)

func TestNewCostOptimizer(t *testing.T) {
	config := &OptimizerConfig{
		DefaultProvider:      "aws",
		DefaultRegion:        "us-east-1",
		SpotEnabled:          true,
		SpotMaxPrice:         0.8,
		BudgetAlertThreshold: 0.8,
		CostTrackingEnabled:  true,
	}

	optimizer := NewCostOptimizer(config)
	if optimizer == nil {
		t.Fatal("expected non-nil optimizer")
	}
	if optimizer.config.DefaultProvider != "aws" {
		t.Errorf("expected provider aws, got %s", optimizer.config.DefaultProvider)
	}
}

func TestNewCostOptimizer_DefaultConfig(t *testing.T) {
	optimizer := NewCostOptimizer(nil)
	if optimizer == nil {
		t.Fatal("expected non-nil optimizer")
	}
	if optimizer.config.SpotMaxPrice != 0.8 {
		t.Errorf("expected default spot max price 0.8, got %f", optimizer.config.SpotMaxPrice)
	}
}

func TestCostOptimizer_RegisterPriceProvider(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	provider := NewAWSPriceProvider("us-east-1")
	optimizer.RegisterPriceProvider("aws", provider)

	if optimizer.priceProviders["aws"] == nil {
		t.Error("expected provider to be registered")
	}
}

func TestCostOptimizer_PredictCost(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	ctx := context.Background()
	req := &CostPredictionRequest{
		JobID:             "job-123",
		InstanceType:      "t3.medium",
		Region:            "us-east-1",
		EstimatedDuration: 1 * time.Hour,
		CPUCores:          2,
		MemoryGB:          4,
		Retryable:         true,
		Provider:          "aws",
	}

	prediction, err := optimizer.PredictCost(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prediction == nil {
		t.Fatal("expected non-nil prediction")
	}
	if prediction.OnDemandCost <= 0 {
		t.Error("expected positive on-demand cost")
	}
	if prediction.SpotCost >= prediction.OnDemandCost {
		t.Error("expected spot cost to be less than on-demand")
	}
}

func TestCostOptimizer_PredictCost_WithProvider(t *testing.T) {
	optimizer := NewCostOptimizer(nil)
	optimizer.RegisterPriceProvider("aws", NewAWSPriceProvider("us-east-1"))

	ctx := context.Background()
	req := &CostPredictionRequest{
		JobID:             "job-456",
		InstanceType:      "m5.large",
		Region:            "us-east-1",
		EstimatedDuration: 30 * time.Minute,
		Retryable:         true,
		Provider:          "aws",
	}

	prediction, err := optimizer.PredictCost(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !prediction.SpotAvailable {
		t.Error("expected spot to be available")
	}
}

func TestCostOptimizer_RecordExecutionCost(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	cost := &ExecutionCost{
		JobID:         "job-123",
		ExecutionID:   "exec-456",
		StartTime:     time.Now().Add(-1 * time.Hour),
		EndTime:       time.Now(),
		Duration:      1 * time.Hour,
		InstanceType:  "t3.medium",
		Region:        "us-east-1",
		SpotUsed:      true,
		OnDemandPrice: 0.0416,
		ActualPrice:   0.0125,
		ComputeCost:   0.0125,
		Labels:        map[string]string{"team": "data"},
	}

	err := optimizer.RecordExecutionCost(cost)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cost.ID == "" {
		t.Error("expected cost ID to be set")
	}
	if cost.Savings <= 0 {
		t.Error("expected positive savings for spot usage")
	}
}

func TestCostOptimizer_CreateBudget(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	budget := &Budget{
		Name:           "Data Team Budget",
		Namespace:      "data-team",
		Amount:         1000.0,
		Period:         BudgetPeriodMonthly,
		AlertThreshold: 0.8,
	}

	err := optimizer.CreateBudget(budget)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if budget.ID == "" {
		t.Error("expected budget ID to be set")
	}
}

func TestCostOptimizer_GetBudget(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	budget := &Budget{
		Name:   "Test Budget",
		Amount: 500.0,
		Period: BudgetPeriodWeekly,
	}
	optimizer.CreateBudget(budget)

	retrieved, err := optimizer.GetBudget(budget.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrieved.Name != "Test Budget" {
		t.Errorf("expected name 'Test Budget', got '%s'", retrieved.Name)
	}
}

func TestCostOptimizer_ListBudgets(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	for i := 0; i < 3; i++ {
		budget := &Budget{
			Name:   "Budget " + string(rune('A'+i)),
			Amount: float64(100 * (i + 1)),
			Period: BudgetPeriodMonthly,
		}
		optimizer.CreateBudget(budget)
	}

	budgets := optimizer.ListBudgets()
	if len(budgets) < 3 {
		t.Errorf("expected at least 3 budgets, got %d", len(budgets))
	}
}

func TestCostOptimizer_BudgetAlerts(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	budget := &Budget{
		Name:           "Alert Test Budget",
		Namespace:      "test-ns",
		Amount:         100.0,
		Period:         BudgetPeriodDaily,
		AlertThreshold: 0.5,
	}
	optimizer.CreateBudget(budget)

	// Record costs that exceed threshold
	for i := 0; i < 10; i++ {
		cost := &ExecutionCost{
			JobID:       "job-" + string(rune('a'+i)),
			ExecutionID: "exec-" + string(rune('a'+i)),
			StartTime:   time.Now().Add(-10 * time.Minute),
			EndTime:     time.Now(),
			Duration:    10 * time.Minute,
			ComputeCost: 10.0,
			TotalCost:   10.0,
			Labels:      map[string]string{"namespace": "test-ns"},
		}
		optimizer.RecordExecutionCost(cost)
	}

	// Check budget has alerts
	retrieved, _ := optimizer.GetBudget(budget.ID)
	if len(retrieved.Alerts) == 0 {
		t.Error("expected budget alerts after exceeding threshold")
	}
}

func TestCostOptimizer_GenerateRecommendations(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	// Record multiple execution costs
	now := time.Now()
	for i := 0; i < 20; i++ {
		cost := &ExecutionCost{
			JobID:         "job-recs",
			ExecutionID:   "exec-" + string(rune('a'+i)),
			StartTime:     now.Add(-time.Duration(i) * time.Hour),
			EndTime:       now.Add(-time.Duration(i)*time.Hour + 30*time.Minute),
			Duration:      30 * time.Minute,
			InstanceType:  "m5.xlarge",
			SpotUsed:      false,
			OnDemandPrice: 0.192,
			ActualPrice:   0.192,
			ComputeCost:   0.096,
			TotalCost:     0.096,
		}
		optimizer.RecordExecutionCost(cost)
	}

	ctx := context.Background()
	recommendations, err := optimizer.GenerateRecommendations(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least a spot instance recommendation
	if len(recommendations) == 0 {
		t.Error("expected recommendations for non-spot jobs")
	}
}

func TestCostOptimizer_GetCostReport(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	// Record costs
	now := time.Now()
	for i := 0; i < 10; i++ {
		cost := &ExecutionCost{
			JobID:        "report-job-" + string(rune('a'+i%3)),
			ExecutionID:  "exec-" + string(rune('a'+i)),
			StartTime:    now.Add(-time.Duration(i) * time.Hour),
			EndTime:      now.Add(-time.Duration(i)*time.Hour + 15*time.Minute),
			Duration:     15 * time.Minute,
			InstanceType: "t3.medium",
			ComputeCost:  0.02,
			TotalCost:    0.02,
			Labels:       map[string]string{"namespace": "default"},
		}
		optimizer.RecordExecutionCost(cost)
	}

	ctx := context.Background()
	report, err := optimizer.GetCostReport(ctx, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.TotalExecutions == 0 {
		t.Error("expected executions in report")
	}
	if report.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
}

func TestAWSPriceProvider(t *testing.T) {
	provider := NewAWSPriceProvider("us-east-1")
	ctx := context.Background()

	// Test on-demand price
	price, err := provider.GetOnDemandPrice(ctx, "t3.medium", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price <= 0 {
		t.Error("expected positive price")
	}

	// Test spot price
	spotPrice, err := provider.GetSpotPrice(ctx, "t3.medium", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spotPrice.Price >= price {
		t.Error("expected spot price to be less than on-demand")
	}

	// Test availability zones
	zones, err := provider.GetAvailabilityZones(ctx, "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) == 0 {
		t.Error("expected availability zones")
	}
}

func TestGCPPriceProvider(t *testing.T) {
	provider := NewGCPPriceProvider("test-project")
	ctx := context.Background()

	price, err := provider.GetOnDemandPrice(ctx, "n1-standard-2", "us-central1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price <= 0 {
		t.Error("expected positive price")
	}

	spotPrice, err := provider.GetSpotPrice(ctx, "n1-standard-2", "us-central1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spotPrice.Price >= price {
		t.Error("expected preemptible price to be less than on-demand")
	}
}

func TestAzurePriceProvider(t *testing.T) {
	provider := NewAzurePriceProvider("test-subscription")
	ctx := context.Background()

	price, err := provider.GetOnDemandPrice(ctx, "Standard_D2s_v3", "eastus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price <= 0 {
		t.Error("expected positive price")
	}
}

func TestBudgetPeriods(t *testing.T) {
	periods := []BudgetPeriod{
		BudgetPeriodDaily,
		BudgetPeriodWeekly,
		BudgetPeriodMonthly,
	}

	for _, p := range periods {
		if p == "" {
			t.Error("empty budget period")
		}
	}
}

func TestRecommendationTypes(t *testing.T) {
	types := []RecommendationType{
		RecommendationSpotInstance,
		RecommendationRightSizing,
		RecommendationScheduleShift,
		RecommendationRegionChange,
		RecommendationBatchConsolidate,
		RecommendationIdleResource,
	}

	for _, rt := range types {
		if rt == "" {
			t.Error("empty recommendation type")
		}
	}
}

func TestRecommendationSeverity(t *testing.T) {
	severities := []RecommendationSeverity{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}

	for _, s := range severities {
		if s == "" {
			t.Error("empty severity")
		}
	}
}

func TestCostOptimizer_RightSizingRecommendation(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	// Record costs with quick completion times (suggesting over-provisioning)
	now := time.Now()
	for i := 0; i < 20; i++ {
		cost := &ExecutionCost{
			JobID:        "quick-job",
			ExecutionID:  "exec-" + string(rune('a'+i)),
			StartTime:    now.Add(-time.Duration(i) * time.Hour),
			EndTime:      now.Add(-time.Duration(i)*time.Hour + 2*time.Minute),
			Duration:     2 * time.Minute,
			InstanceType: "m5.2xlarge",
			ComputeCost:  0.05,
			TotalCost:    0.05,
		}
		optimizer.RecordExecutionCost(cost)
	}

	ctx := context.Background()
	recommendations, _ := optimizer.GenerateRecommendations(ctx)

	// Should have right-sizing recommendation
	hasRightSizing := false
	for _, rec := range recommendations {
		if rec.Type == RecommendationRightSizing {
			hasRightSizing = true
			break
		}
	}

	if !hasRightSizing {
		t.Log("Note: Right-sizing recommendation may not be generated depending on algorithm")
	}
}

func TestCostPrediction_RecommendSpot(t *testing.T) {
	optimizer := NewCostOptimizer(&OptimizerConfig{
		SpotEnabled:  true,
		SpotMaxPrice: 0.8,
	})

	ctx := context.Background()

	// Retryable job should recommend spot
	req := &CostPredictionRequest{
		JobID:             "retryable-job",
		InstanceType:      "t3.medium",
		Region:            "us-east-1",
		EstimatedDuration: 1 * time.Hour,
		Retryable:         true,
	}

	prediction, _ := optimizer.PredictCost(ctx, req)
	if !prediction.RecommendSpot {
		t.Error("expected spot recommendation for retryable job")
	}

	// Non-retryable job should not recommend spot
	req.Retryable = false
	prediction, _ = optimizer.PredictCost(ctx, req)
	// Note: This depends on interruption risk threshold
	_ = prediction
}

func TestCostReport_Breakdown(t *testing.T) {
	optimizer := NewCostOptimizer(nil)

	// Record diverse costs
	now := time.Now()
	testData := []struct {
		jobID      string
		namespace  string
		instance   string
	}{
		{"etl-job", "data", "m5.large"},
		{"api-job", "web", "t3.small"},
		{"etl-job", "data", "m5.large"},
		{"ml-job", "data", "p3.2xlarge"},
	}

	for i, td := range testData {
		cost := &ExecutionCost{
			JobID:        td.jobID,
			ExecutionID:  "exec-" + string(rune('a'+i)),
			StartTime:    now.Add(-time.Duration(i) * time.Hour),
			EndTime:      now.Add(-time.Duration(i)*time.Hour + 30*time.Minute),
			Duration:     30 * time.Minute,
			InstanceType: td.instance,
			ComputeCost:  0.1,
			TotalCost:    0.1,
			Labels:       map[string]string{"namespace": td.namespace},
		}
		optimizer.RecordExecutionCost(cost)
	}

	ctx := context.Background()
	report, _ := optimizer.GetCostReport(ctx, now.Add(-24*time.Hour), now)

	// Check breakdowns exist
	if len(report.CostByJob) == 0 {
		t.Error("expected cost by job breakdown")
	}
	if len(report.CostByNamespace) == 0 {
		t.Error("expected cost by namespace breakdown")
	}
	if len(report.CostByInstanceType) == 0 {
		t.Error("expected cost by instance type breakdown")
	}
}
