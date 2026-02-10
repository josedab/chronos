// Package routing provides intelligent job routing with ML-based predictions.
package routing

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors for data pipeline.
var (
	ErrNoExecutionData   = errors.New("no execution data available")
	ErrInvalidTimeWindow = errors.New("invalid time window")
)

// ExecutionFeatures contains extracted features from job executions.
type ExecutionFeatures struct {
	JobID         string    `json:"job_id"`
	TargetID      string    `json:"target_id"`
	Timestamp     time.Time `json:"timestamp"`

	// Timing features
	DurationMs    int64   `json:"duration_ms"`
	HourOfDay     int     `json:"hour_of_day"`
	DayOfWeek     int     `json:"day_of_week"`
	IsWeekend     bool    `json:"is_weekend"`
	IsBusinessHrs bool    `json:"is_business_hours"`

	// Response features
	StatusCode    int     `json:"status_code"`
	Success       bool    `json:"success"`
	ResponseSize  int64   `json:"response_size_bytes"`
	ResponseTime  int64   `json:"response_time_ms"`

	// Context features
	RetryAttempt  int     `json:"retry_attempt"`
	PayloadSize   int64   `json:"payload_size_bytes"`
	QueueWaitTime int64   `json:"queue_wait_time_ms"`
}

// TargetMetrics holds aggregated metrics for a specific target.
type TargetMetrics struct {
	TargetID        string        `json:"target_id"`
	TotalExecutions int64         `json:"total_executions"`
	SuccessCount    int64         `json:"success_count"`
	FailureCount    int64         `json:"failure_count"`
	SuccessRate     float64       `json:"success_rate"`
	AvgDurationMs   float64       `json:"avg_duration_ms"`
	P50DurationMs   float64       `json:"p50_duration_ms"`
	P95DurationMs   float64       `json:"p95_duration_ms"`
	P99DurationMs   float64       `json:"p99_duration_ms"`
	AvgResponseTime float64       `json:"avg_response_time_ms"`
	LastUpdated     time.Time     `json:"last_updated"`

	// Time-series data for pattern detection
	HourlySuccessRate  [24]float64 `json:"hourly_success_rate"`
	DailySuccessRate   [7]float64  `json:"daily_success_rate"`
	RecentTrend        string      `json:"recent_trend"` // "improving", "stable", "degrading"
}

// JobTargetStats contains per-job, per-target statistics.
type JobTargetStats struct {
	JobID           string         `json:"job_id"`
	TargetMetrics   *TargetMetrics `json:"target_metrics"`
	HistoricalData  []float64      `json:"historical_success_rates"`
	PredictedRate   float64        `json:"predicted_success_rate"`
	Confidence      float64        `json:"confidence"`
	LastExecution   time.Time      `json:"last_execution"`
}

// TimeSeriesPoint represents a point in time-series data.
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// DataPipeline collects and processes execution data for ML-based routing.
type DataPipeline struct {
	mu              sync.RWMutex
	executions      map[string][]ExecutionFeatures // jobID -> features
	targetMetrics   map[string]*TargetMetrics      // targetID -> metrics
	jobTargetStats  map[string]map[string]*JobTargetStats // jobID -> targetID -> stats
	timeSeries      map[string][]TimeSeriesPoint   // key -> time series

	config          DataPipelineConfig
	aggregationTick *time.Ticker
	stopCh          chan struct{}
}

// DataPipelineConfig configures the data pipeline.
type DataPipelineConfig struct {
	// MaxExecutionsPerJob limits stored executions per job.
	MaxExecutionsPerJob int `json:"max_executions_per_job" yaml:"max_executions_per_job"`
	// AggregationInterval is how often to recalculate metrics.
	AggregationInterval time.Duration `json:"aggregation_interval" yaml:"aggregation_interval"`
	// RetentionPeriod is how long to keep execution data.
	RetentionPeriod time.Duration `json:"retention_period" yaml:"retention_period"`
	// MinDataPointsForPrediction is minimum executions needed.
	MinDataPointsForPrediction int `json:"min_data_points" yaml:"min_data_points"`
}

// DefaultDataPipelineConfig returns sensible defaults.
func DefaultDataPipelineConfig() DataPipelineConfig {
	return DataPipelineConfig{
		MaxExecutionsPerJob:        5000,
		AggregationInterval:        5 * time.Minute,
		RetentionPeriod:            30 * 24 * time.Hour, // 30 days
		MinDataPointsForPrediction: 20,
	}
}

// NewDataPipeline creates a new data pipeline.
func NewDataPipeline(config DataPipelineConfig) *DataPipeline {
	dp := &DataPipeline{
		executions:     make(map[string][]ExecutionFeatures),
		targetMetrics:  make(map[string]*TargetMetrics),
		jobTargetStats: make(map[string]map[string]*JobTargetStats),
		timeSeries:     make(map[string][]TimeSeriesPoint),
		config:         config,
		stopCh:         make(chan struct{}),
	}

	// Start background aggregation
	dp.aggregationTick = time.NewTicker(config.AggregationInterval)
	go dp.runAggregation()

	return dp
}

// RecordExecution records a job execution for analysis.
func (dp *DataPipeline) RecordExecution(ctx context.Context, jobID, targetID string, exec ExecutionResult) error {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	features := dp.extractFeatures(jobID, targetID, exec)
	
	// Store execution
	dp.executions[jobID] = append(dp.executions[jobID], features)

	// Trim if exceeds max
	if len(dp.executions[jobID]) > dp.config.MaxExecutionsPerJob {
		dp.executions[jobID] = dp.executions[jobID][len(dp.executions[jobID])-dp.config.MaxExecutionsPerJob:]
	}

	// Update metrics incrementally
	dp.updateMetricsIncremental(targetID, features)

	// Update job-target stats
	if dp.jobTargetStats[jobID] == nil {
		dp.jobTargetStats[jobID] = make(map[string]*JobTargetStats)
	}
	if dp.jobTargetStats[jobID][targetID] == nil {
		dp.jobTargetStats[jobID][targetID] = &JobTargetStats{
			JobID:          jobID,
			HistoricalData: make([]float64, 0),
		}
	}
	stats := dp.jobTargetStats[jobID][targetID]
	stats.LastExecution = features.Timestamp
	stats.TargetMetrics = dp.targetMetrics[targetID]

	return nil
}

// ExecutionResult captures the outcome of a job execution.
type ExecutionResult struct {
	Success       bool          `json:"success"`
	StatusCode    int           `json:"status_code"`
	Duration      time.Duration `json:"duration"`
	ResponseTime  time.Duration `json:"response_time"`
	ResponseSize  int64         `json:"response_size"`
	PayloadSize   int64         `json:"payload_size"`
	RetryAttempt  int           `json:"retry_attempt"`
	QueueWaitTime time.Duration `json:"queue_wait_time"`
	ExecutedAt    time.Time     `json:"executed_at"`
	Error         string        `json:"error,omitempty"`
}

// extractFeatures extracts ML features from an execution result.
func (dp *DataPipeline) extractFeatures(jobID, targetID string, exec ExecutionResult) ExecutionFeatures {
	t := exec.ExecutedAt
	if t.IsZero() {
		t = time.Now().UTC()
	}

	hour := t.Hour()
	dayOfWeek := int(t.Weekday())

	return ExecutionFeatures{
		JobID:         jobID,
		TargetID:      targetID,
		Timestamp:     t,
		DurationMs:    exec.Duration.Milliseconds(),
		HourOfDay:     hour,
		DayOfWeek:     dayOfWeek,
		IsWeekend:     dayOfWeek == 0 || dayOfWeek == 6,
		IsBusinessHrs: hour >= 9 && hour <= 17 && dayOfWeek >= 1 && dayOfWeek <= 5,
		StatusCode:    exec.StatusCode,
		Success:       exec.Success,
		ResponseSize:  exec.ResponseSize,
		ResponseTime:  exec.ResponseTime.Milliseconds(),
		RetryAttempt:  exec.RetryAttempt,
		PayloadSize:   exec.PayloadSize,
		QueueWaitTime: exec.QueueWaitTime.Milliseconds(),
	}
}

// updateMetricsIncremental updates target metrics with a new execution.
func (dp *DataPipeline) updateMetricsIncremental(targetID string, features ExecutionFeatures) {
	metrics := dp.targetMetrics[targetID]
	if metrics == nil {
		metrics = &TargetMetrics{
			TargetID: targetID,
		}
		dp.targetMetrics[targetID] = metrics
	}

	metrics.TotalExecutions++
	if features.Success {
		metrics.SuccessCount++
	} else {
		metrics.FailureCount++
	}
	metrics.SuccessRate = float64(metrics.SuccessCount) / float64(metrics.TotalExecutions)

	// Exponential moving average for duration
	alpha := 0.1
	if metrics.AvgDurationMs == 0 {
		metrics.AvgDurationMs = float64(features.DurationMs)
		metrics.AvgResponseTime = float64(features.ResponseTime)
	} else {
		metrics.AvgDurationMs = alpha*float64(features.DurationMs) + (1-alpha)*metrics.AvgDurationMs
		metrics.AvgResponseTime = alpha*float64(features.ResponseTime) + (1-alpha)*metrics.AvgResponseTime
	}

	// Update hourly/daily rates
	hour := features.HourOfDay
	day := features.DayOfWeek
	if features.Success {
		metrics.HourlySuccessRate[hour] = 0.9*metrics.HourlySuccessRate[hour] + 0.1*1.0
		metrics.DailySuccessRate[day] = 0.9*metrics.DailySuccessRate[day] + 0.1*1.0
	} else {
		metrics.HourlySuccessRate[hour] = 0.9*metrics.HourlySuccessRate[hour] + 0.1*0.0
		metrics.DailySuccessRate[day] = 0.9*metrics.DailySuccessRate[day] + 0.1*0.0
	}

	metrics.LastUpdated = time.Now().UTC()
}

// GetTargetMetrics returns metrics for a specific target.
func (dp *DataPipeline) GetTargetMetrics(targetID string) (*TargetMetrics, error) {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	metrics, exists := dp.targetMetrics[targetID]
	if !exists {
		return nil, ErrNoExecutionData
	}
	return metrics, nil
}

// GetJobTargetStats returns stats for a specific job-target combination.
func (dp *DataPipeline) GetJobTargetStats(jobID, targetID string) (*JobTargetStats, error) {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	jobStats, exists := dp.jobTargetStats[jobID]
	if !exists {
		return nil, ErrNoExecutionData
	}
	stats, exists := jobStats[targetID]
	if !exists {
		return nil, ErrNoExecutionData
	}
	return stats, nil
}

// GetAllTargetMetrics returns metrics for all targets.
func (dp *DataPipeline) GetAllTargetMetrics() map[string]*TargetMetrics {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	result := make(map[string]*TargetMetrics, len(dp.targetMetrics))
	for k, v := range dp.targetMetrics {
		result[k] = v
	}
	return result
}

// GetExecutionHistory returns recent executions for a job.
func (dp *DataPipeline) GetExecutionHistory(jobID string, limit int) []ExecutionFeatures {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	execs := dp.executions[jobID]
	if len(execs) == 0 {
		return nil
	}

	if limit <= 0 || limit > len(execs) {
		limit = len(execs)
	}

	// Return most recent
	result := make([]ExecutionFeatures, limit)
	copy(result, execs[len(execs)-limit:])
	return result
}

// GetSuccessRateTimeSeries returns success rate time series for a target.
func (dp *DataPipeline) GetSuccessRateTimeSeries(targetID string, window time.Duration) ([]TimeSeriesPoint, error) {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	key := "success_rate:" + targetID
	series := dp.timeSeries[key]
	if len(series) == 0 {
		return nil, ErrNoExecutionData
	}

	cutoff := time.Now().Add(-window)
	result := make([]TimeSeriesPoint, 0)
	for _, point := range series {
		if point.Timestamp.After(cutoff) {
			result = append(result, point)
		}
	}

	return result, nil
}

// runAggregation runs periodic aggregation of metrics.
func (dp *DataPipeline) runAggregation() {
	for {
		select {
		case <-dp.aggregationTick.C:
			dp.aggregate()
		case <-dp.stopCh:
			return
		}
	}
}

// aggregate recalculates all metrics and cleans old data.
func (dp *DataPipeline) aggregate() {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	now := time.Now().UTC()
	cutoff := now.Add(-dp.config.RetentionPeriod)

	// Clean old executions and recalculate percentiles
	for jobID, execs := range dp.executions {
		// Filter old executions
		filtered := make([]ExecutionFeatures, 0, len(execs))
		for _, e := range execs {
			if e.Timestamp.After(cutoff) {
				filtered = append(filtered, e)
			}
		}
		dp.executions[jobID] = filtered
	}

	// Recalculate percentiles for each target
	for targetID := range dp.targetMetrics {
		dp.recalculatePercentiles(targetID)
		dp.calculateTrend(targetID)
	}

	// Update time series
	for targetID, metrics := range dp.targetMetrics {
		key := "success_rate:" + targetID
		dp.timeSeries[key] = append(dp.timeSeries[key], TimeSeriesPoint{
			Timestamp: now,
			Value:     metrics.SuccessRate,
		})

		// Trim old time series points
		series := dp.timeSeries[key]
		filtered := make([]TimeSeriesPoint, 0, len(series))
		for _, p := range series {
			if p.Timestamp.After(cutoff) {
				filtered = append(filtered, p)
			}
		}
		dp.timeSeries[key] = filtered
	}
}

// recalculatePercentiles calculates duration percentiles for a target.
func (dp *DataPipeline) recalculatePercentiles(targetID string) {
	metrics := dp.targetMetrics[targetID]
	if metrics == nil {
		return
	}

	// Collect all durations for this target
	var durations []float64
	for _, execs := range dp.executions {
		for _, e := range execs {
			if e.TargetID == targetID {
				durations = append(durations, float64(e.DurationMs))
			}
		}
	}

	if len(durations) == 0 {
		return
	}

	sort.Float64s(durations)

	metrics.P50DurationMs = percentile(durations, 0.50)
	metrics.P95DurationMs = percentile(durations, 0.95)
	metrics.P99DurationMs = percentile(durations, 0.99)
}

// calculateTrend determines if target performance is improving or degrading.
func (dp *DataPipeline) calculateTrend(targetID string) {
	metrics := dp.targetMetrics[targetID]
	if metrics == nil {
		return
	}

	key := "success_rate:" + targetID
	series := dp.timeSeries[key]
	if len(series) < 10 {
		metrics.RecentTrend = "stable"
		return
	}

	// Compare first half to second half
	mid := len(series) / 2
	firstHalf := series[:mid]
	secondHalf := series[mid:]

	firstAvg := avgTimeSeries(firstHalf)
	secondAvg := avgTimeSeries(secondHalf)

	diff := secondAvg - firstAvg
	if diff > 0.05 {
		metrics.RecentTrend = "improving"
	} else if diff < -0.05 {
		metrics.RecentTrend = "degrading"
	} else {
		metrics.RecentTrend = "stable"
	}
}

// Stop stops the data pipeline.
func (dp *DataPipeline) Stop() {
	dp.aggregationTick.Stop()
	close(dp.stopCh)
}

// Export exports pipeline data for persistence.
func (dp *DataPipeline) Export() ([]byte, error) {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	data := struct {
		Executions     map[string][]ExecutionFeatures       `json:"executions"`
		TargetMetrics  map[string]*TargetMetrics            `json:"target_metrics"`
		JobTargetStats map[string]map[string]*JobTargetStats `json:"job_target_stats"`
		TimeSeries     map[string][]TimeSeriesPoint         `json:"time_series"`
	}{
		Executions:     dp.executions,
		TargetMetrics:  dp.targetMetrics,
		JobTargetStats: dp.jobTargetStats,
		TimeSeries:     dp.timeSeries,
	}

	return json.Marshal(data)
}

// Import imports pipeline data from persistence.
func (dp *DataPipeline) Import(data []byte) error {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	var imported struct {
		Executions     map[string][]ExecutionFeatures       `json:"executions"`
		TargetMetrics  map[string]*TargetMetrics            `json:"target_metrics"`
		JobTargetStats map[string]map[string]*JobTargetStats `json:"job_target_stats"`
		TimeSeries     map[string][]TimeSeriesPoint         `json:"time_series"`
	}

	if err := json.Unmarshal(data, &imported); err != nil {
		return err
	}

	dp.executions = imported.Executions
	dp.targetMetrics = imported.TargetMetrics
	dp.jobTargetStats = imported.JobTargetStats
	dp.timeSeries = imported.TimeSeries

	return nil
}

// Helper functions

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(len(sorted))*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func avgTimeSeries(series []TimeSeriesPoint) float64 {
	if len(series) == 0 {
		return 0
	}
	var sum float64
	for _, p := range series {
		sum += p.Value
	}
	return sum / float64(len(series))
}

// Ensure uuid is used
var _ = uuid.New
