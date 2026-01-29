package analytics

import (
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	tests := []struct {
		name       string
		maxRecords int
		wantMax    int
	}{
		{
			name:       "default max records when zero",
			maxRecords: 0,
			wantMax:    10000,
		},
		{
			name:       "default max records when negative",
			maxRecords: -1,
			wantMax:    10000,
		},
		{
			name:       "custom max records",
			maxRecords: 5000,
			wantMax:    5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCollector(DefaultCostConfig(), tt.maxRecords)
			if c == nil {
				t.Fatal("NewCollector returned nil")
			}
			if c.maxRecords != tt.wantMax {
				t.Errorf("maxRecords = %d, want %d", c.maxRecords, tt.wantMax)
			}
		})
	}
}

func TestDefaultCostConfig(t *testing.T) {
	cfg := DefaultCostConfig()

	if cfg.CostPerExecution != 0.0001 {
		t.Errorf("CostPerExecution = %f, want 0.0001", cfg.CostPerExecution)
	}
	if cfg.CostPerSecond != 0.00001 {
		t.Errorf("CostPerSecond = %f, want 0.00001", cfg.CostPerSecond)
	}
	if cfg.Currency != "USD" {
		t.Errorf("Currency = %s, want USD", cfg.Currency)
	}
}

func TestCollector_Record(t *testing.T) {
	c := NewCollector(DefaultCostConfig(), 100)

	record := ExecutionRecord{
		JobID:       "job-1",
		JobName:     "Test Job",
		ExecutionID: "exec-1",
		Status:      "success",
		Duration:    5 * time.Second,
		StartTime:   time.Now(),
		EndTime:     time.Now().Add(5 * time.Second),
		Region:      "us-east-1",
		Executor:    "http",
	}

	c.Record(record)

	if len(c.records) != 1 {
		t.Errorf("records count = %d, want 1", len(c.records))
	}
	if _, ok := c.jobStats["job-1"]; !ok {
		t.Error("jobStats for job-1 not created")
	}
}

func TestCollector_RecordMaxLimit(t *testing.T) {
	maxRecords := 5
	c := NewCollector(DefaultCostConfig(), maxRecords)

	for i := 0; i < 10; i++ {
		c.Record(ExecutionRecord{
			JobID:       "job-1",
			ExecutionID: string(rune('0' + i)),
			Status:      "success",
			Duration:    time.Second,
			StartTime:   time.Now(),
		})
	}

	if len(c.records) != maxRecords {
		t.Errorf("records count = %d, want %d (max)", len(c.records), maxRecords)
	}
}

func TestCollector_RecordStats(t *testing.T) {
	c := NewCollector(DefaultCostConfig(), 100)

	// Record success
	c.Record(ExecutionRecord{
		JobID:     "job-1",
		Status:    "success",
		Duration:  time.Second,
		StartTime: time.Now(),
	})

	// Record failure
	c.Record(ExecutionRecord{
		JobID:     "job-1",
		Status:    "failed",
		Duration:  2 * time.Second,
		StartTime: time.Now(),
		Error:     "timeout",
	})

	stats := c.jobStats["job-1"]
	if stats.executions != 2 {
		t.Errorf("executions = %d, want 2", stats.executions)
	}
	if stats.successes != 1 {
		t.Errorf("successes = %d, want 1", stats.successes)
	}
	if stats.failures != 1 {
		t.Errorf("failures = %d, want 1", stats.failures)
	}
	if stats.errors["timeout"] != 1 {
		t.Errorf("timeout error count = %d, want 1", stats.errors["timeout"])
	}
}

func TestCollector_GetDashboard(t *testing.T) {
	c := NewCollector(DefaultCostConfig(), 100)

	now := time.Now()
	period := Period{
		Start: now.Add(-24 * time.Hour),
		End:   now.Add(time.Hour),
	}

	// Record some executions within period
	for i := 0; i < 5; i++ {
		status := "success"
		if i%2 == 0 {
			status = "failed"
		}
		c.Record(ExecutionRecord{
			JobID:     "job-1",
			JobName:   "Test Job",
			Status:    status,
			Duration:  time.Duration(i+1) * time.Second,
			StartTime: now.Add(-time.Duration(i) * time.Hour),
		})
	}

	// Record one outside period
	c.Record(ExecutionRecord{
		JobID:     "job-2",
		Status:    "success",
		Duration:  time.Second,
		StartTime: now.Add(-48 * time.Hour),
	})

	dashboard := c.GetDashboard(period)

	if dashboard.Summary.TotalExecutions != 5 {
		t.Errorf("TotalExecutions = %d, want 5", dashboard.Summary.TotalExecutions)
	}
	if dashboard.Summary.SuccessfulRuns != 2 {
		t.Errorf("SuccessfulRuns = %d, want 2", dashboard.Summary.SuccessfulRuns)
	}
	if dashboard.Summary.FailedRuns != 3 {
		t.Errorf("FailedRuns = %d, want 3", dashboard.Summary.FailedRuns)
	}
}

func TestCollector_GetDashboardEmptyPeriod(t *testing.T) {
	c := NewCollector(DefaultCostConfig(), 100)

	period := Period{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	dashboard := c.GetDashboard(period)

	if dashboard.Summary.TotalExecutions != 0 {
		t.Errorf("TotalExecutions = %d, want 0", dashboard.Summary.TotalExecutions)
	}
	if dashboard.Summary.SuccessRate != 0 {
		t.Errorf("SuccessRate = %f, want 0", dashboard.Summary.SuccessRate)
	}
}

func TestCollector_CalculateExecutionCost(t *testing.T) {
	config := CostConfig{
		CostPerExecution: 0.001,
		CostPerSecond:    0.0001,
		Currency:         "USD",
		CostByRegion: map[string]float64{
			"premium-region": 0.002,
		},
		CostByExecutor: map[string]float64{
			"gpu": 0.001,
		},
	}

	c := NewCollector(config, 100)

	tests := []struct {
		name     string
		record   ExecutionRecord
		wantCost float64
	}{
		{
			name: "base cost calculation",
			record: ExecutionRecord{
				Duration: 10 * time.Second,
				Region:   "us-east-1",
				Executor: "http",
			},
			wantCost: 0.001 + 0.0001*10,
		},
		{
			name: "with regional pricing",
			record: ExecutionRecord{
				Duration: 10 * time.Second,
				Region:   "premium-region",
				Executor: "http",
			},
			wantCost: 0.002 + 0.0001*10,
		},
		{
			name: "with executor pricing",
			record: ExecutionRecord{
				Duration: 10 * time.Second,
				Region:   "us-east-1",
				Executor: "gpu",
			},
			wantCost: 0.001 + 0.001*10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := c.calculateExecutionCost(tt.record)
			if cost != tt.wantCost {
				t.Errorf("cost = %f, want %f", cost, tt.wantCost)
			}
		})
	}
}

func TestCollector_CalculateJobMetrics(t *testing.T) {
	c := NewCollector(DefaultCostConfig(), 100)

	now := time.Now()
	records := []ExecutionRecord{
		{JobID: "job-1", JobName: "Job 1", Status: "success", Duration: time.Second, StartTime: now},
		{JobID: "job-1", JobName: "Job 1", Status: "success", Duration: 2 * time.Second, StartTime: now.Add(time.Hour)},
		{JobID: "job-1", JobName: "Job 1", Status: "failed", Duration: 3 * time.Second, StartTime: now.Add(2 * time.Hour)},
		{JobID: "job-2", JobName: "Job 2", Status: "success", Duration: time.Second, StartTime: now},
	}

	metrics := c.calculateJobMetrics(records)

	if len(metrics) != 2 {
		t.Fatalf("expected 2 job metrics, got %d", len(metrics))
	}

	// Find job-1 metrics
	var job1Metric *JobMetric
	for i := range metrics {
		if metrics[i].JobID == "job-1" {
			job1Metric = &metrics[i]
			break
		}
	}

	if job1Metric == nil {
		t.Fatal("job-1 metric not found")
	}

	if job1Metric.Executions != 3 {
		t.Errorf("job-1 executions = %d, want 3", job1Metric.Executions)
	}
	if job1Metric.SuccessCount != 2 {
		t.Errorf("job-1 success count = %d, want 2", job1Metric.SuccessCount)
	}
	if job1Metric.FailureCount != 1 {
		t.Errorf("job-1 failure count = %d, want 1", job1Metric.FailureCount)
	}
	if job1Metric.MinDuration != time.Second {
		t.Errorf("job-1 min duration = %v, want 1s", job1Metric.MinDuration)
	}
	if job1Metric.MaxDuration != 3*time.Second {
		t.Errorf("job-1 max duration = %v, want 3s", job1Metric.MaxDuration)
	}
}

func TestCollector_GenerateTimeSeries(t *testing.T) {
	c := NewCollector(DefaultCostConfig(), 100)

	now := time.Now().Truncate(time.Hour)
	period := Period{
		Start: now.Add(-24 * time.Hour),
		End:   now.Add(time.Hour),
	}

	records := []ExecutionRecord{
		{Status: "success", Duration: time.Second, StartTime: now},
		{Status: "success", Duration: time.Second, StartTime: now.Add(30 * time.Minute)},
		{Status: "failed", Duration: time.Second, StartTime: now},
		{Status: "success", Duration: time.Second, StartTime: now.Add(-time.Hour)},
	}

	series := c.generateTimeSeries(records, period)

	if len(series) == 0 {
		t.Fatal("expected non-empty time series")
	}

	// Find current hour data
	var currentHour *TimeSeriesPoint
	for i := range series {
		if series[i].Timestamp.Equal(now) {
			currentHour = &series[i]
			break
		}
	}

	if currentHour == nil {
		t.Fatal("current hour data not found")
	}

	if currentHour.Executions != 3 {
		t.Errorf("current hour executions = %d, want 3", currentHour.Executions)
	}
	if currentHour.Successes != 2 {
		t.Errorf("current hour successes = %d, want 2", currentHour.Successes)
	}
	if currentHour.Failures != 1 {
		t.Errorf("current hour failures = %d, want 1", currentHour.Failures)
	}
}

func TestCollector_GetTopFailures(t *testing.T) {
	c := NewCollector(DefaultCostConfig(), 100)

	now := time.Now()
	records := []ExecutionRecord{
		{JobID: "job-1", JobName: "Job 1", Status: "failed", Error: "timeout", StartTime: now},
		{JobID: "job-1", JobName: "Job 1", Status: "failed", Error: "timeout", StartTime: now.Add(time.Hour)},
		{JobID: "job-1", JobName: "Job 1", Status: "success", StartTime: now.Add(2 * time.Hour)},
		{JobID: "job-2", JobName: "Job 2", Status: "failed", Error: "network", StartTime: now},
	}

	failures := c.getTopFailures(records)

	if len(failures) != 2 {
		t.Fatalf("expected 2 failure entries, got %d", len(failures))
	}

	// Find job-1 failures
	var job1Failure *FailureMetric
	for i := range failures {
		if failures[i].JobID == "job-1" {
			job1Failure = &failures[i]
			break
		}
	}

	if job1Failure == nil {
		t.Fatal("job-1 failure not found")
	}

	if job1Failure.FailureCount != 2 {
		t.Errorf("job-1 failure count = %d, want 2", job1Failure.FailureCount)
	}
	if len(job1Failure.CommonErrors) != 2 {
		t.Errorf("job-1 common errors = %d, want 2", len(job1Failure.CommonErrors))
	}
}

func TestCollector_GenerateRecommendations(t *testing.T) {
	c := NewCollector(DefaultCostConfig(), 100)

	dashboard := &Dashboard{
		Summary: Summary{
			EstimatedCost: Cost{Total: 100.0},
		},
		TopFailures: []FailureMetric{
			{JobID: "job-1", JobName: "Failing Job", FailureCount: 15},
		},
		JobMetrics: []JobMetric{
			{JobID: "job-2", JobName: "Expensive Job", EstimatedCost: 50.0},
		},
	}

	recs := c.generateRecommendations(dashboard)

	if len(recs) < 2 {
		t.Fatalf("expected at least 2 recommendations, got %d", len(recs))
	}

	// Check for high failure recommendation
	hasFailureRec := false
	hasCostRec := false
	for _, rec := range recs {
		if rec.Type == RecommendationReliability && rec.JobID == "job-1" {
			hasFailureRec = true
		}
		if rec.Type == RecommendationCostSaving && rec.JobID == "job-2" {
			hasCostRec = true
		}
	}

	if !hasFailureRec {
		t.Error("missing high failure rate recommendation")
	}
	if !hasCostRec {
		t.Error("missing high cost recommendation")
	}
}

func TestRecommendationTypes(t *testing.T) {
	types := []RecommendationType{
		RecommendationCostSaving,
		RecommendationPerformance,
		RecommendationReliability,
		RecommendationSchedule,
	}

	expected := []string{"cost_saving", "performance", "reliability", "schedule"}

	for i, rt := range types {
		if string(rt) != expected[i] {
			t.Errorf("RecommendationType %d = %s, want %s", i, rt, expected[i])
		}
	}
}

func TestPriorityTypes(t *testing.T) {
	priorities := []Priority{PriorityHigh, PriorityMedium, PriorityLow}
	expected := []string{"high", "medium", "low"}

	for i, p := range priorities {
		if string(p) != expected[i] {
			t.Errorf("Priority %d = %s, want %s", i, p, expected[i])
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := NewCollector(DefaultCostConfig(), 1000)

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			c.Record(ExecutionRecord{
				JobID:     "job-1",
				Status:    "success",
				Duration:  time.Second,
				StartTime: time.Now(),
			})
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		period := Period{
			Start: time.Now().Add(-time.Hour),
			End:   time.Now().Add(time.Hour),
		}
		for i := 0; i < 100; i++ {
			c.GetDashboard(period)
		}
		done <- true
	}()

	<-done
	<-done
}
