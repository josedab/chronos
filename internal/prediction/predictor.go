// Package prediction provides smart failure prediction for Chronos.
package prediction

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Common errors.
var (
	ErrInsufficientData = errors.New("insufficient data for prediction")
	ErrModelNotTrained  = errors.New("model not trained")
)

// ExecutionRecord represents a historical execution.
type ExecutionRecord struct {
	JobID         string        `json:"job_id"`
	ExecutionID   string        `json:"execution_id"`
	Timestamp     time.Time     `json:"timestamp"`
	Duration      time.Duration `json:"duration"`
	Success       bool          `json:"success"`
	StatusCode    int           `json:"status_code,omitempty"`
	ErrorType     string        `json:"error_type,omitempty"`
	RetryCount    int           `json:"retry_count"`
	DayOfWeek     int           `json:"day_of_week"`
	HourOfDay     int           `json:"hour_of_day"`
}

// Prediction represents a failure prediction.
type Prediction struct {
	JobID              string    `json:"job_id"`
	FailureProbability float64   `json:"failure_probability"`
	Confidence         float64   `json:"confidence"`
	RiskLevel          RiskLevel `json:"risk_level"`
	Reasons            []string  `json:"reasons"`
	RecommendedActions []string  `json:"recommended_actions"`
	PredictedAt        time.Time `json:"predicted_at"`
}

// RiskLevel represents the risk level of failure.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// Alert represents a proactive alert.
type Alert struct {
	ID          string    `json:"id"`
	JobID       string    `json:"job_id"`
	JobName     string    `json:"job_name"`
	Type        AlertType `json:"type"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	Prediction  *Prediction `json:"prediction,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Acknowledged bool     `json:"acknowledged"`
}

// AlertType represents the type of alert.
type AlertType string

const (
	AlertPredictedFailure AlertType = "predicted_failure"
	AlertDegradedPerformance AlertType = "degraded_performance"
	AlertPatternAnomaly   AlertType = "pattern_anomaly"
	AlertHighRetryRate    AlertType = "high_retry_rate"
)

// JobStats contains statistics for a job.
type JobStats struct {
	JobID              string        `json:"job_id"`
	TotalExecutions    int           `json:"total_executions"`
	SuccessCount       int           `json:"success_count"`
	FailureCount       int           `json:"failure_count"`
	SuccessRate        float64       `json:"success_rate"`
	AvgDuration        time.Duration `json:"avg_duration"`
	P95Duration        time.Duration `json:"p95_duration"`
	P99Duration        time.Duration `json:"p99_duration"`
	AvgRetries         float64       `json:"avg_retries"`
	FailuresByHour     [24]int       `json:"failures_by_hour"`
	FailuresByDayOfWeek [7]int       `json:"failures_by_day_of_week"`
	RecentTrend        string        `json:"recent_trend"`
	LastUpdated        time.Time     `json:"last_updated"`
}

// Predictor provides failure prediction capabilities.
type Predictor struct {
	mu               sync.RWMutex
	records          map[string][]ExecutionRecord
	stats            map[string]*JobStats
	alerts           []Alert
	minDataPoints    int
	featureExtractor *FeatureExtractor
	mlPredictor      *GradientBoostingPredictor
	notificationMgr  *NotificationManager
	useMLPrediction  bool
}

// PredictorConfig configures the predictor.
type PredictorConfig struct {
	MinDataPoints    int  `json:"min_data_points" yaml:"min_data_points"`
	UseMLPrediction  bool `json:"use_ml_prediction" yaml:"use_ml_prediction"`
	MLNumTrees       int  `json:"ml_num_trees" yaml:"ml_num_trees"`
	MLLearningRate   float64 `json:"ml_learning_rate" yaml:"ml_learning_rate"`
}

// DefaultPredictorConfig returns a default predictor configuration.
func DefaultPredictorConfig() PredictorConfig {
	return PredictorConfig{
		MinDataPoints:   10,
		UseMLPrediction: true,
		MLNumTrees:      50,
		MLLearningRate:  0.1,
	}
}

// NewPredictor creates a new predictor.
func NewPredictor() *Predictor {
	return NewPredictorWithConfig(DefaultPredictorConfig())
}

// NewPredictorWithConfig creates a new predictor with configuration.
func NewPredictorWithConfig(cfg PredictorConfig) *Predictor {
	return &Predictor{
		records:          make(map[string][]ExecutionRecord),
		stats:            make(map[string]*JobStats),
		alerts:           make([]Alert, 0),
		minDataPoints:    cfg.MinDataPoints,
		featureExtractor: NewFeatureExtractor(),
		mlPredictor:      NewGradientBoostingPredictor(cfg.MLNumTrees, cfg.MLLearningRate),
		notificationMgr:  NewNotificationManager(),
		useMLPrediction:  cfg.UseMLPrediction,
	}
}

// GetNotificationManager returns the notification manager for configuration.
func (p *Predictor) GetNotificationManager() *NotificationManager {
	return p.notificationMgr
}

// SetMLPrediction enables or disables ML-based prediction.
func (p *Predictor) SetMLPrediction(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.useMLPrediction = enabled
}

// TrainMLModel trains the ML model on historical data.
func (p *Predictor) TrainMLModel(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var allFeatures []*FeatureSet
	var allLabels []bool

	for jobID, records := range p.records {
		if len(records) < p.minDataPoints {
			continue
		}

		// Create feature sets for each execution (using previous records to predict)
		for i := p.minDataPoints; i < len(records); i++ {
			historicalRecords := records[:i]
			currentRecord := records[i]

			features, err := p.featureExtractor.ExtractFeatures(historicalRecords, currentRecord.Timestamp)
			if err != nil {
				continue
			}

			allFeatures = append(allFeatures, features)
			allLabels = append(allLabels, !currentRecord.Success)
			_ = jobID // jobID used indirectly in the loop
		}
	}

	if len(allFeatures) < 100 {
		return fmt.Errorf("insufficient training data: need at least 100 samples, have %d", len(allFeatures))
	}

	return p.mlPredictor.Train(allFeatures, allLabels)
}

// GetMLModelMetrics returns ML model performance metrics.
func (p *Predictor) GetMLModelMetrics(ctx context.Context) (*ModelMetrics, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var allFeatures []*FeatureSet
	var allLabels []bool

	for _, records := range p.records {
		if len(records) < p.minDataPoints {
			continue
		}

		for i := p.minDataPoints; i < len(records); i++ {
			features, err := p.featureExtractor.ExtractFeatures(records[:i], records[i].Timestamp)
			if err != nil {
				continue
			}
			allFeatures = append(allFeatures, features)
			allLabels = append(allLabels, !records[i].Success)
		}
	}

	return p.mlPredictor.Evaluate(allFeatures, allLabels)
}

// RecordExecution records an execution for analysis.
func (p *Predictor) RecordExecution(ctx context.Context, record ExecutionRecord) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Set derived fields
	record.DayOfWeek = int(record.Timestamp.Weekday())
	record.HourOfDay = record.Timestamp.Hour()

	p.records[record.JobID] = append(p.records[record.JobID], record)

	// Keep last 1000 records per job
	if len(p.records[record.JobID]) > 1000 {
		p.records[record.JobID] = p.records[record.JobID][len(p.records[record.JobID])-1000:]
	}

	// Update stats
	p.updateStats(record.JobID)

	// Check for alerts
	p.checkForAlerts(record.JobID)

	return nil
}

// Predict predicts failure probability for a job.
func (p *Predictor) Predict(ctx context.Context, jobID string) (*Prediction, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	records, ok := p.records[jobID]
	if !ok || len(records) < p.minDataPoints {
		return nil, ErrInsufficientData
	}

	stats := p.stats[jobID]
	if stats == nil {
		return nil, ErrModelNotTrained
	}

	prediction := &Prediction{
		JobID:       jobID,
		PredictedAt: time.Now(),
		Reasons:     make([]string, 0),
		RecommendedActions: make([]string, 0),
	}

	// Try ML-based prediction if enabled and trained
	if p.useMLPrediction && p.mlPredictor != nil {
		features, err := p.featureExtractor.ExtractFeatures(records, time.Now())
		if err == nil {
			mlProb, mlErr := p.mlPredictor.Predict(features)
			if mlErr == nil {
				prediction.FailureProbability = mlProb
				prediction.Confidence = 0.85 // ML models generally have higher confidence

				// Add ML-derived reasons
				if features.ConsecutiveFailures > 2 {
					prediction.Reasons = append(prediction.Reasons, fmt.Sprintf("%d consecutive failures detected", features.ConsecutiveFailures))
				}
				if features.RecentFailureRate > features.HistoricalFailureRate*1.5 {
					prediction.Reasons = append(prediction.Reasons, "recent failure rate significantly higher than historical")
				}
				if features.NewErrorTypeRecent {
					prediction.Reasons = append(prediction.Reasons, "new error type detected recently")
				}
				if features.RecentDurationTrend > 0.3 {
					prediction.Reasons = append(prediction.Reasons, "execution duration increasing trend")
				}
				if features.StabilityScore < 0.5 {
					prediction.Reasons = append(prediction.Reasons, "low stability score")
				}

				// Set risk level based on ML prediction
				prediction.RiskLevel = p.calculateRiskLevel(mlProb)
				prediction.RecommendedActions = p.getRecommendedActions(prediction.RiskLevel, features)

				return prediction, nil
			}
		}
	}

	// Fall back to rule-based prediction
	var probability float64
	var confidence float64

	// Factor 1: Historical failure rate (weight: 40%)
	failureRate := 1.0 - stats.SuccessRate
	probability += failureRate * 0.4
	confidence += 0.4

	// Factor 2: Recent trend (weight: 30%)
	recentRecords := records[max(0, len(records)-20):]
	recentFailures := 0
	for _, r := range recentRecords {
		if !r.Success {
			recentFailures++
		}
	}
	recentFailureRate := float64(recentFailures) / float64(len(recentRecords))
	probability += recentFailureRate * 0.3

	if recentFailureRate > stats.SuccessRate {
		prediction.Reasons = append(prediction.Reasons, "recent failure rate is increasing")
	}
	confidence += 0.3

	// Factor 3: Time-based patterns (weight: 20%)
	currentHour := time.Now().Hour()
	currentDay := int(time.Now().Weekday())
	
	hourlyFailureRate := float64(stats.FailuresByHour[currentHour]) / float64(stats.TotalExecutions)
	dailyFailureRate := float64(stats.FailuresByDayOfWeek[currentDay]) / float64(stats.TotalExecutions)
	
	timeBasedProb := (hourlyFailureRate + dailyFailureRate) / 2
	probability += timeBasedProb * 0.2

	if stats.FailuresByHour[currentHour] > stats.TotalExecutions/24*2 {
		prediction.Reasons = append(prediction.Reasons, "higher failure rate during this hour")
	}
	confidence += 0.2

	// Factor 4: Retry patterns (weight: 10%)
	if stats.AvgRetries > 1.5 {
		probability += 0.1
		prediction.Reasons = append(prediction.Reasons, "high retry rate indicates instability")
	}
	confidence += 0.1

	// Normalize probability
	prediction.FailureProbability = math.Min(probability, 1.0)
	prediction.Confidence = confidence

	// Determine risk level
	prediction.RiskLevel = p.calculateRiskLevel(prediction.FailureProbability)
	prediction.RecommendedActions = p.getRecommendedActions(prediction.RiskLevel, nil)

	return prediction, nil
}

// calculateRiskLevel determines risk level from probability.
func (p *Predictor) calculateRiskLevel(probability float64) RiskLevel {
	switch {
	case probability >= 0.7:
		return RiskCritical
	case probability >= 0.5:
		return RiskHigh
	case probability >= 0.3:
		return RiskMedium
	default:
		return RiskLow
	}
}

// getRecommendedActions returns actions based on risk level and features.
func (p *Predictor) getRecommendedActions(risk RiskLevel, features *FeatureSet) []string {
	var actions []string

	switch risk {
	case RiskCritical:
		actions = append(actions, 
			"consider disabling job until issue is resolved",
			"review recent changes to the webhook endpoint",
			"increase retry attempts or timeout")
		if features != nil && features.ConsecutiveFailures > 5 {
			actions = append(actions, "consider circuit breaker pattern")
		}
	case RiskHigh:
		actions = append(actions,
			"monitor job closely",
			"prepare fallback mechanism",
			"review error logs")
		if features != nil && features.NewErrorTypeRecent {
			actions = append(actions, "investigate new error type")
		}
	case RiskMedium:
		actions = append(actions,
			"review recent execution logs",
			"verify endpoint availability")
		if features != nil && features.RecentDurationTrend > 0.2 {
			actions = append(actions, "investigate performance degradation")
		}
	case RiskLow:
		// No specific actions needed for low risk
	}

	return actions
}

// GetStats returns statistics for a job.
func (p *Predictor) GetStats(ctx context.Context, jobID string) (*JobStats, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats, ok := p.stats[jobID]
	if !ok {
		return nil, ErrInsufficientData
	}
	return stats, nil
}

// GetAlerts returns pending alerts.
func (p *Predictor) GetAlerts(ctx context.Context, acknowledged bool) ([]Alert, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []Alert
	for _, a := range p.alerts {
		if a.Acknowledged == acknowledged {
			result = append(result, a)
		}
	}
	return result, nil
}

// AcknowledgeAlert acknowledges an alert.
func (p *Predictor) AcknowledgeAlert(ctx context.Context, alertID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.alerts {
		if p.alerts[i].ID == alertID {
			p.alerts[i].Acknowledged = true
			return nil
		}
	}
	return errors.New("alert not found")
}

// DetectAnomalies detects anomalies in recent executions.
func (p *Predictor) DetectAnomalies(ctx context.Context, jobID string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	records, ok := p.records[jobID]
	if !ok || len(records) < p.minDataPoints {
		return nil, ErrInsufficientData
	}

	stats := p.stats[jobID]
	if stats == nil {
		return nil, ErrModelNotTrained
	}

	var anomalies []string

	// Check for duration anomalies
	recentRecords := records[max(0, len(records)-5):]
	for _, r := range recentRecords {
		if r.Duration > stats.P99Duration {
			anomalies = append(anomalies, "execution duration exceeded P99 threshold")
			break
		}
	}

	// Check for sudden failure spike
	recentFailures := 0
	for _, r := range recentRecords {
		if !r.Success {
			recentFailures++
		}
	}
	if float64(recentFailures)/float64(len(recentRecords)) > 0.5 && stats.SuccessRate > 0.8 {
		anomalies = append(anomalies, "sudden spike in failures detected")
	}

	// Check for new error types
	errorTypes := make(map[string]bool)
	for _, r := range records[:len(records)-len(recentRecords)] {
		if r.ErrorType != "" {
			errorTypes[r.ErrorType] = true
		}
	}
	for _, r := range recentRecords {
		if r.ErrorType != "" && !errorTypes[r.ErrorType] {
			anomalies = append(anomalies, "new error type detected: "+r.ErrorType)
		}
	}

	return anomalies, nil
}

// updateStats updates statistics for a job.
func (p *Predictor) updateStats(jobID string) {
	records := p.records[jobID]
	if len(records) == 0 {
		return
	}

	stats := &JobStats{
		JobID:          jobID,
		TotalExecutions: len(records),
		LastUpdated:    time.Now(),
	}

	var totalDuration time.Duration
	var totalRetries int
	durations := make([]time.Duration, 0, len(records))

	for _, r := range records {
		if r.Success {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
			stats.FailuresByHour[r.HourOfDay]++
			stats.FailuresByDayOfWeek[r.DayOfWeek]++
		}
		totalDuration += r.Duration
		totalRetries += r.RetryCount
		durations = append(durations, r.Duration)
	}

	stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalExecutions)
	stats.AvgDuration = totalDuration / time.Duration(len(records))
	stats.AvgRetries = float64(totalRetries) / float64(len(records))

	// Calculate percentiles
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})
	p95Index := int(float64(len(durations)) * 0.95)
	p99Index := int(float64(len(durations)) * 0.99)
	if p95Index < len(durations) {
		stats.P95Duration = durations[p95Index]
	}
	if p99Index < len(durations) {
		stats.P99Duration = durations[p99Index]
	}

	// Determine trend
	if len(records) >= 20 {
		first10 := records[:10]
		last10 := records[len(records)-10:]
		
		firstFailures := 0
		lastFailures := 0
		for _, r := range first10 {
			if !r.Success {
				firstFailures++
			}
		}
		for _, r := range last10 {
			if !r.Success {
				lastFailures++
			}
		}

		switch {
		case lastFailures > firstFailures+2:
			stats.RecentTrend = "degrading"
		case lastFailures < firstFailures-2:
			stats.RecentTrend = "improving"
		default:
			stats.RecentTrend = "stable"
		}
	}

	p.stats[jobID] = stats
}

// checkForAlerts checks if any alerts should be raised.
func (p *Predictor) checkForAlerts(jobID string) {
	stats := p.stats[jobID]
	if stats == nil {
		return
	}

	// Check for high failure rate
	if stats.SuccessRate < 0.5 && stats.TotalExecutions >= 10 {
		p.alerts = append(p.alerts, Alert{
			ID:       generateID(),
			JobID:    jobID,
			Type:     AlertPredictedFailure,
			Severity: "high",
			Message:  "job has low success rate",
			CreatedAt: time.Now(),
		})
	}

	// Check for degrading trend
	if stats.RecentTrend == "degrading" {
		p.alerts = append(p.alerts, Alert{
			ID:       generateID(),
			JobID:    jobID,
			Type:     AlertDegradedPerformance,
			Severity: "medium",
			Message:  "job performance is degrading",
			CreatedAt: time.Now(),
		})
	}

	// Check for high retry rate
	if stats.AvgRetries > 2.0 {
		p.alerts = append(p.alerts, Alert{
			ID:       generateID(),
			JobID:    jobID,
			Type:     AlertHighRetryRate,
			Severity: "medium",
			Message:  "job has high retry rate",
			CreatedAt: time.Now(),
		})
	}

	// Keep last 100 alerts
	if len(p.alerts) > 100 {
		p.alerts = p.alerts[len(p.alerts)-100:]
	}
}

// generateID generates a simple ID.
func generateID() string {
	return time.Now().Format("20060102150405.000")
}

// max returns the maximum of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
