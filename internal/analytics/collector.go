// Package analytics provides cost and performance analytics for Chronos.
package analytics

import (
	"sync"
	"time"
)

// Dashboard contains aggregated analytics data.
type Dashboard struct {
	Period        Period        `json:"period"`
	Summary       Summary       `json:"summary"`
	JobMetrics    []JobMetric   `json:"job_metrics"`
	TimeSeriesData []TimeSeriesPoint `json:"time_series"`
	TopFailures   []FailureMetric `json:"top_failures"`
	Recommendations []Recommendation `json:"recommendations"`
}

// Period represents an analytics time period.
type Period struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Summary contains high-level summary metrics.
type Summary struct {
	TotalJobs          int           `json:"total_jobs"`
	TotalExecutions    int           `json:"total_executions"`
	SuccessfulRuns     int           `json:"successful_runs"`
	FailedRuns         int           `json:"failed_runs"`
	SuccessRate        float64       `json:"success_rate"`
	AvgDuration        time.Duration `json:"avg_duration"`
	P95Duration        time.Duration `json:"p95_duration"`
	TotalComputeTime   time.Duration `json:"total_compute_time"`
	EstimatedCost      Cost          `json:"estimated_cost"`
	TrendVsPrevPeriod  Trend         `json:"trend_vs_prev_period"`
}

// Cost represents estimated costs.
type Cost struct {
	Total    float64            `json:"total"`
	Currency string             `json:"currency"`
	ByJob    map[string]float64 `json:"by_job,omitempty"`
	ByRegion map[string]float64 `json:"by_region,omitempty"`
}

// Trend represents change vs. previous period.
type Trend struct {
	ExecutionsChange float64 `json:"executions_change"` // Percentage
	SuccessRateChange float64 `json:"success_rate_change"`
	DurationChange   float64 `json:"duration_change"`
	CostChange       float64 `json:"cost_change"`
}

// JobMetric contains metrics for a single job.
type JobMetric struct {
	JobID           string        `json:"job_id"`
	JobName         string        `json:"job_name"`
	Executions      int           `json:"executions"`
	SuccessCount    int           `json:"success_count"`
	FailureCount    int           `json:"failure_count"`
	SuccessRate     float64       `json:"success_rate"`
	AvgDuration     time.Duration `json:"avg_duration"`
	MinDuration     time.Duration `json:"min_duration"`
	MaxDuration     time.Duration `json:"max_duration"`
	P50Duration     time.Duration `json:"p50_duration"`
	P95Duration     time.Duration `json:"p95_duration"`
	P99Duration     time.Duration `json:"p99_duration"`
	TotalDuration   time.Duration `json:"total_duration"`
	EstimatedCost   float64       `json:"estimated_cost"`
	LastExecution   time.Time     `json:"last_execution"`
	LastStatus      string        `json:"last_status"`
}

// TimeSeriesPoint is a single data point in a time series.
type TimeSeriesPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Executions  int       `json:"executions"`
	Successes   int       `json:"successes"`
	Failures    int       `json:"failures"`
	AvgDuration float64   `json:"avg_duration_seconds"`
	Cost        float64   `json:"cost"`
}

// FailureMetric tracks failure patterns.
type FailureMetric struct {
	JobID         string   `json:"job_id"`
	JobName       string   `json:"job_name"`
	FailureCount  int      `json:"failure_count"`
	FailureRate   float64  `json:"failure_rate"`
	CommonErrors  []string `json:"common_errors"`
	LastFailure   time.Time `json:"last_failure"`
}

// Recommendation is an optimization recommendation.
type Recommendation struct {
	Type        RecommendationType `json:"type"`
	Priority    Priority           `json:"priority"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	JobID       string             `json:"job_id,omitempty"`
	Potential   string             `json:"potential"` // e.g., "Save $50/month"
}

// RecommendationType categorizes recommendations.
type RecommendationType string

const (
	RecommendationCostSaving    RecommendationType = "cost_saving"
	RecommendationPerformance   RecommendationType = "performance"
	RecommendationReliability   RecommendationType = "reliability"
	RecommendationSchedule      RecommendationType = "schedule"
)

// Priority indicates recommendation urgency.
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// CostConfig configures cost estimation.
type CostConfig struct {
	// CostPerExecution is the base cost per job execution.
	CostPerExecution float64 `json:"cost_per_execution" yaml:"cost_per_execution"`
	// CostPerSecond is the cost per second of execution time.
	CostPerSecond float64 `json:"cost_per_second" yaml:"cost_per_second"`
	// CostByRegion overrides cost per region.
	CostByRegion map[string]float64 `json:"cost_by_region" yaml:"cost_by_region"`
	// CostByExecutor overrides cost per executor type.
	CostByExecutor map[string]float64 `json:"cost_by_executor" yaml:"cost_by_executor"`
	// Currency for cost display.
	Currency string `json:"currency" yaml:"currency"`
}

// DefaultCostConfig returns default cost configuration.
func DefaultCostConfig() CostConfig {
	return CostConfig{
		CostPerExecution: 0.0001, // $0.0001 per execution
		CostPerSecond:    0.00001, // $0.00001 per second
		Currency:         "USD",
		CostByRegion:     make(map[string]float64),
		CostByExecutor:   make(map[string]float64),
	}
}

// ExecutionRecord is a record for analytics.
type ExecutionRecord struct {
	JobID       string
	JobName     string
	ExecutionID string
	Status      string
	Duration    time.Duration
	StartTime   time.Time
	EndTime     time.Time
	Region      string
	Executor    string
	Error       string
}

// Collector collects and aggregates analytics data.
type Collector struct {
	mu         sync.RWMutex
	records    []ExecutionRecord
	jobStats   map[string]*jobStats
	costConfig CostConfig
	maxRecords int
}

type jobStats struct {
	executions   int
	successes    int
	failures     int
	totalDuration time.Duration
	durations    []time.Duration
	lastExec     time.Time
	lastStatus   string
	errors       map[string]int
}

// NewCollector creates a new analytics collector.
func NewCollector(costConfig CostConfig, maxRecords int) *Collector {
	if maxRecords <= 0 {
		maxRecords = 10000
	}
	return &Collector{
		records:    make([]ExecutionRecord, 0, maxRecords),
		jobStats:   make(map[string]*jobStats),
		costConfig: costConfig,
		maxRecords: maxRecords,
	}
}

// Record records an execution for analytics.
func (c *Collector) Record(r ExecutionRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.records = append(c.records, r)
	if len(c.records) > c.maxRecords {
		c.records = c.records[len(c.records)-c.maxRecords:]
	}

	// Update job stats
	stats, exists := c.jobStats[r.JobID]
	if !exists {
		stats = &jobStats{
			durations: make([]time.Duration, 0),
			errors:    make(map[string]int),
		}
		c.jobStats[r.JobID] = stats
	}

	stats.executions++
	stats.totalDuration += r.Duration
	stats.durations = append(stats.durations, r.Duration)
	stats.lastExec = r.StartTime
	stats.lastStatus = r.Status

	if r.Status == "success" {
		stats.successes++
	} else {
		stats.failures++
		if r.Error != "" {
			stats.errors[r.Error]++
		}
	}
}

// GetDashboard returns the analytics dashboard.
func (c *Collector) GetDashboard(period Period) *Dashboard {
	c.mu.RLock()
	defer c.mu.RUnlock()

	dashboard := &Dashboard{
		Period:      period,
		JobMetrics:  make([]JobMetric, 0),
		TopFailures: make([]FailureMetric, 0),
	}

	// Filter records by period
	var periodRecords []ExecutionRecord
	for _, r := range c.records {
		if r.StartTime.After(period.Start) && r.StartTime.Before(period.End) {
			periodRecords = append(periodRecords, r)
		}
	}

	// Calculate summary
	dashboard.Summary = c.calculateSummary(periodRecords)

	// Calculate per-job metrics
	dashboard.JobMetrics = c.calculateJobMetrics(periodRecords)

	// Generate time series
	dashboard.TimeSeriesData = c.generateTimeSeries(periodRecords, period)

	// Get top failures
	dashboard.TopFailures = c.getTopFailures(periodRecords)

	// Generate recommendations
	dashboard.Recommendations = c.generateRecommendations(dashboard)

	return dashboard
}

func (c *Collector) calculateSummary(records []ExecutionRecord) Summary {
	summary := Summary{
		EstimatedCost: Cost{
			Currency: c.costConfig.Currency,
			ByJob:    make(map[string]float64),
		},
	}

	if len(records) == 0 {
		return summary
	}

	var totalDuration time.Duration
	jobSet := make(map[string]bool)

	for _, r := range records {
		summary.TotalExecutions++
		jobSet[r.JobID] = true
		totalDuration += r.Duration

		if r.Status == "success" {
			summary.SuccessfulRuns++
		} else {
			summary.FailedRuns++
		}

		// Calculate cost
		cost := c.calculateExecutionCost(r)
		summary.EstimatedCost.Total += cost
		summary.EstimatedCost.ByJob[r.JobID] += cost
	}

	summary.TotalJobs = len(jobSet)
	summary.TotalComputeTime = totalDuration

	if summary.TotalExecutions > 0 {
		summary.SuccessRate = float64(summary.SuccessfulRuns) / float64(summary.TotalExecutions)
		summary.AvgDuration = totalDuration / time.Duration(summary.TotalExecutions)
	}

	return summary
}

func (c *Collector) calculateExecutionCost(r ExecutionRecord) float64 {
	baseCost := c.costConfig.CostPerExecution
	timeCost := c.costConfig.CostPerSecond * r.Duration.Seconds()

	// Apply regional pricing
	if regionCost, ok := c.costConfig.CostByRegion[r.Region]; ok {
		baseCost = regionCost
	}

	// Apply executor pricing
	if execCost, ok := c.costConfig.CostByExecutor[r.Executor]; ok {
		timeCost = execCost * r.Duration.Seconds()
	}

	return baseCost + timeCost
}

func (c *Collector) calculateJobMetrics(records []ExecutionRecord) []JobMetric {
	jobRecords := make(map[string][]ExecutionRecord)
	for _, r := range records {
		jobRecords[r.JobID] = append(jobRecords[r.JobID], r)
	}

	var metrics []JobMetric
	for jobID, recs := range jobRecords {
		if len(recs) == 0 {
			continue
		}

		metric := JobMetric{
			JobID:      jobID,
			JobName:    recs[0].JobName,
			Executions: len(recs),
		}

		var totalDuration time.Duration
		for _, r := range recs {
			totalDuration += r.Duration
			if r.Status == "success" {
				metric.SuccessCount++
			} else {
				metric.FailureCount++
			}

			if metric.MinDuration == 0 || r.Duration < metric.MinDuration {
				metric.MinDuration = r.Duration
			}
			if r.Duration > metric.MaxDuration {
				metric.MaxDuration = r.Duration
			}

			metric.EstimatedCost += c.calculateExecutionCost(r)
		}

		metric.TotalDuration = totalDuration
		metric.AvgDuration = totalDuration / time.Duration(len(recs))
		metric.SuccessRate = float64(metric.SuccessCount) / float64(len(recs))
		metric.LastExecution = recs[len(recs)-1].StartTime
		metric.LastStatus = recs[len(recs)-1].Status

		metrics = append(metrics, metric)
	}

	return metrics
}

func (c *Collector) generateTimeSeries(records []ExecutionRecord, period Period) []TimeSeriesPoint {
	// Group by hour
	hourlyData := make(map[time.Time]*TimeSeriesPoint)

	for _, r := range records {
		hour := r.StartTime.Truncate(time.Hour)
		point, exists := hourlyData[hour]
		if !exists {
			point = &TimeSeriesPoint{Timestamp: hour}
			hourlyData[hour] = point
		}

		point.Executions++
		if r.Status == "success" {
			point.Successes++
		} else {
			point.Failures++
		}
		point.Cost += c.calculateExecutionCost(r)
	}

	// Convert to slice
	var points []TimeSeriesPoint
	for _, point := range hourlyData {
		if point.Executions > 0 {
			point.AvgDuration = float64(point.Executions) // Simplified
		}
		points = append(points, *point)
	}

	return points
}

func (c *Collector) getTopFailures(records []ExecutionRecord) []FailureMetric {
	failures := make(map[string]*FailureMetric)

	for _, r := range records {
		if r.Status != "success" {
			fm, exists := failures[r.JobID]
			if !exists {
				fm = &FailureMetric{
					JobID:   r.JobID,
					JobName: r.JobName,
				}
				failures[r.JobID] = fm
			}
			fm.FailureCount++
			fm.LastFailure = r.StartTime
			if r.Error != "" && len(fm.CommonErrors) < 5 {
				fm.CommonErrors = append(fm.CommonErrors, r.Error)
			}
		}
	}

	var result []FailureMetric
	for _, fm := range failures {
		result = append(result, *fm)
	}

	return result
}

func (c *Collector) generateRecommendations(dashboard *Dashboard) []Recommendation {
	var recs []Recommendation

	// High failure rate recommendation
	for _, fm := range dashboard.TopFailures {
		if fm.FailureCount > 10 {
			recs = append(recs, Recommendation{
				Type:        RecommendationReliability,
				Priority:    PriorityHigh,
				Title:       "High failure rate detected",
				Description: "Job '" + fm.JobName + "' has " + string(rune('0'+fm.FailureCount)) + " failures. Review error logs and consider adjusting retry policy.",
				JobID:       fm.JobID,
			})
		}
	}

	// High cost jobs
	for _, jm := range dashboard.JobMetrics {
		if jm.EstimatedCost > dashboard.Summary.EstimatedCost.Total*0.3 {
			recs = append(recs, Recommendation{
				Type:        RecommendationCostSaving,
				Priority:    PriorityMedium,
				Title:       "High-cost job identified",
				Description: "Job '" + jm.JobName + "' accounts for >30% of total costs. Consider optimizing execution time.",
				JobID:       jm.JobID,
				Potential:   "Potential 30% cost reduction",
			})
		}
	}

	return recs
}
