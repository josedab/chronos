// Package smartsched provides intelligent scheduling optimization for Chronos.
package smartsched

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Anomaly represents a detected anomaly in job execution.
type Anomaly struct {
	ID          string        `json:"id"`
	JobID       string        `json:"job_id"`
	Type        AnomalyType   `json:"type"`
	Severity    AnomalySeverity `json:"severity"`
	Description string        `json:"description"`
	DetectedAt  time.Time     `json:"detected_at"`
	Metric      string        `json:"metric"`
	ExpectedVal float64       `json:"expected_value"`
	ActualVal   float64       `json:"actual_value"`
	Deviation   float64       `json:"deviation"` // Standard deviations from mean
	Suggestions []string      `json:"suggestions,omitempty"`
}

// AnomalyType represents the type of anomaly.
type AnomalyType string

const (
	AnomalyDurationSpike   AnomalyType = "duration_spike"
	AnomalyFailureSpike    AnomalyType = "failure_spike"
	AnomalyPatternChange   AnomalyType = "pattern_change"
	AnomalyResourceSpike   AnomalyType = "resource_spike"
	AnomalyMissedExecution AnomalyType = "missed_execution"
)

// AnomalySeverity represents the severity of an anomaly.
type AnomalySeverity string

const (
	AnomalyLow      AnomalySeverity = "low"
	AnomalyMedium   AnomalySeverity = "medium"
	AnomalyHigh     AnomalySeverity = "high"
	AnomalyCritical AnomalySeverity = "critical"
)

// AnomalyDetector detects anomalies in job execution patterns.
type AnomalyDetector struct {
	mu            sync.RWMutex
	jobStats      map[string]*JobStats
	anomalies     []Anomaly
	maxAnomalies  int
	config        AnomalyConfig
}

// AnomalyConfig configures the anomaly detector.
type AnomalyConfig struct {
	// DurationDeviationThreshold is the number of standard deviations for duration anomaly
	DurationDeviationThreshold float64 `json:"duration_deviation_threshold" yaml:"duration_deviation_threshold"`
	// FailureRateThreshold triggers anomaly when failure rate exceeds this
	FailureRateThreshold float64 `json:"failure_rate_threshold" yaml:"failure_rate_threshold"`
	// MinSamplesForDetection requires this many samples before detecting anomalies
	MinSamplesForDetection int `json:"min_samples_for_detection" yaml:"min_samples_for_detection"`
	// WindowSize for rolling calculations
	WindowSize int `json:"window_size" yaml:"window_size"`
	// MaxAnomalies to keep in memory
	MaxAnomalies int `json:"max_anomalies" yaml:"max_anomalies"`
}

// DefaultAnomalyConfig returns default anomaly detection configuration.
func DefaultAnomalyConfig() AnomalyConfig {
	return AnomalyConfig{
		DurationDeviationThreshold: 3.0, // 3 standard deviations
		FailureRateThreshold:       0.3, // 30% failure rate
		MinSamplesForDetection:     10,
		WindowSize:                 100,
		MaxAnomalies:               1000,
	}
}

// JobStats tracks running statistics for a job.
type JobStats struct {
	JobID           string
	SampleCount     int
	DurationSum     float64
	DurationSumSq   float64 // Sum of squares for variance
	DurationMean    float64
	DurationStdDev  float64
	SuccessCount    int
	FailureCount    int
	LastExecution   time.Time
	HourlyStats     [24]*HourlyJobStats
	RecentDurations []float64 // Circular buffer
	bufferIdx       int
}

// HourlyJobStats tracks per-hour statistics.
type HourlyJobStats struct {
	ExecutionCount int
	SuccessCount   int
	FailureCount   int
	DurationSum    float64
	DurationSumSq  float64
}

// NewAnomalyDetector creates a new anomaly detector.
func NewAnomalyDetector(cfg AnomalyConfig) *AnomalyDetector {
	if cfg.WindowSize == 0 {
		cfg = DefaultAnomalyConfig()
	}

	return &AnomalyDetector{
		jobStats:     make(map[string]*JobStats),
		anomalies:    make([]Anomaly, 0, cfg.MaxAnomalies),
		maxAnomalies: cfg.MaxAnomalies,
		config:       cfg,
	}
}

// RecordExecution records an execution and checks for anomalies.
func (d *AnomalyDetector) RecordExecution(jobID string, duration time.Duration, success bool, timestamp time.Time) []Anomaly {
	d.mu.Lock()
	defer d.mu.Unlock()

	stats, exists := d.jobStats[jobID]
	if !exists {
		stats = &JobStats{
			JobID:           jobID,
			RecentDurations: make([]float64, d.config.WindowSize),
		}
		for i := 0; i < 24; i++ {
			stats.HourlyStats[i] = &HourlyJobStats{}
		}
		d.jobStats[jobID] = stats
	}

	durationSec := duration.Seconds()
	var detected []Anomaly

	// Check for duration anomaly if we have enough samples
	if stats.SampleCount >= d.config.MinSamplesForDetection && stats.DurationStdDev > 0 {
		deviation := (durationSec - stats.DurationMean) / stats.DurationStdDev
		if math.Abs(deviation) > d.config.DurationDeviationThreshold {
			severity := d.calculateSeverity(deviation)
			anomaly := Anomaly{
				ID:          fmt.Sprintf("anomaly-%d", time.Now().UnixNano()),
				JobID:       jobID,
				Type:        AnomalyDurationSpike,
				Severity:    severity,
				Description: fmt.Sprintf("Execution duration %.1fs is %.1f standard deviations from mean %.1fs", 
					durationSec, deviation, stats.DurationMean),
				DetectedAt:  time.Now(),
				Metric:      "duration",
				ExpectedVal: stats.DurationMean,
				ActualVal:   durationSec,
				Deviation:   deviation,
				Suggestions: d.generateDurationSuggestions(deviation, durationSec, stats),
			}
			detected = append(detected, anomaly)
			d.recordAnomaly(anomaly)
		}
	}

	// Update running statistics (Welford's online algorithm)
	stats.SampleCount++
	delta := durationSec - stats.DurationMean
	stats.DurationMean += delta / float64(stats.SampleCount)
	delta2 := durationSec - stats.DurationMean
	stats.DurationSum += durationSec
	stats.DurationSumSq += delta * delta2

	if stats.SampleCount > 1 {
		variance := stats.DurationSumSq / float64(stats.SampleCount-1)
		stats.DurationStdDev = math.Sqrt(variance)
	}

	// Update success/failure counts
	if success {
		stats.SuccessCount++
	} else {
		stats.FailureCount++
	}

	// Check for failure rate anomaly
	failureRate := float64(stats.FailureCount) / float64(stats.SampleCount)
	if stats.SampleCount >= d.config.MinSamplesForDetection && failureRate > d.config.FailureRateThreshold {
		// Check if this is a new spike (not already detected recently)
		if !d.hasRecentAnomaly(jobID, AnomalyFailureSpike, 1*time.Hour) {
			anomaly := Anomaly{
				ID:          fmt.Sprintf("anomaly-%d", time.Now().UnixNano()),
				JobID:       jobID,
				Type:        AnomalyFailureSpike,
				Severity:    d.failureRateSeverity(failureRate),
				Description: fmt.Sprintf("Failure rate %.1f%% exceeds threshold %.1f%%", 
					failureRate*100, d.config.FailureRateThreshold*100),
				DetectedAt:  time.Now(),
				Metric:      "failure_rate",
				ExpectedVal: d.config.FailureRateThreshold,
				ActualVal:   failureRate,
				Suggestions: d.generateFailureSuggestions(failureRate),
			}
			detected = append(detected, anomaly)
			d.recordAnomaly(anomaly)
		}
	}

	// Update hourly stats
	hour := timestamp.Hour()
	hourStats := stats.HourlyStats[hour]
	hourStats.ExecutionCount++
	hourStats.DurationSum += durationSec
	hourStats.DurationSumSq += durationSec * durationSec
	if success {
		hourStats.SuccessCount++
	} else {
		hourStats.FailureCount++
	}

	// Update circular buffer
	stats.RecentDurations[stats.bufferIdx] = durationSec
	stats.bufferIdx = (stats.bufferIdx + 1) % d.config.WindowSize

	stats.LastExecution = timestamp

	return detected
}

// calculateSeverity determines anomaly severity based on deviation.
func (d *AnomalyDetector) calculateSeverity(deviation float64) AnomalySeverity {
	absDeviation := math.Abs(deviation)
	switch {
	case absDeviation >= 6:
		return AnomalyCritical
	case absDeviation >= 5:
		return AnomalyHigh
	case absDeviation >= 4:
		return AnomalyMedium
	default:
		return AnomalyLow
	}
}

// failureRateSeverity determines severity based on failure rate.
func (d *AnomalyDetector) failureRateSeverity(rate float64) AnomalySeverity {
	switch {
	case rate >= 0.8:
		return AnomalyCritical
	case rate >= 0.6:
		return AnomalyHigh
	case rate >= 0.4:
		return AnomalyMedium
	default:
		return AnomalyLow
	}
}

// generateDurationSuggestions generates suggestions for duration anomalies.
func (d *AnomalyDetector) generateDurationSuggestions(deviation, actualDuration float64, stats *JobStats) []string {
	var suggestions []string

	if deviation > 0 {
		// Slow execution
		suggestions = append(suggestions, "Consider increasing timeout to accommodate longer executions")
		if actualDuration > stats.DurationMean*2 {
			suggestions = append(suggestions, "Investigate potential performance bottlenecks in the target service")
		}
		suggestions = append(suggestions, "Check for external dependencies that may be slow")
	} else {
		// Fast execution (could indicate skipped work)
		suggestions = append(suggestions, "Verify job completed all expected work")
		suggestions = append(suggestions, "Check if caching or early termination is causing faster execution")
	}

	return suggestions
}

// generateFailureSuggestions generates suggestions for failure anomalies.
func (d *AnomalyDetector) generateFailureSuggestions(failureRate float64) []string {
	suggestions := []string{
		"Review recent changes to the target webhook endpoint",
		"Check target service health and availability",
		"Verify authentication credentials are still valid",
	}

	if failureRate > 0.5 {
		suggestions = append(suggestions, "Consider temporarily disabling the job until the issue is resolved")
		suggestions = append(suggestions, "Set up an alert for persistent failures")
	}

	return suggestions
}

// hasRecentAnomaly checks if a similar anomaly was recently detected.
func (d *AnomalyDetector) hasRecentAnomaly(jobID string, anomalyType AnomalyType, window time.Duration) bool {
	cutoff := time.Now().Add(-window)
	for i := len(d.anomalies) - 1; i >= 0; i-- {
		a := d.anomalies[i]
		if a.DetectedAt.Before(cutoff) {
			break
		}
		if a.JobID == jobID && a.Type == anomalyType {
			return true
		}
	}
	return false
}

// recordAnomaly adds an anomaly to the list.
func (d *AnomalyDetector) recordAnomaly(anomaly Anomaly) {
	d.anomalies = append(d.anomalies, anomaly)
	if len(d.anomalies) > d.maxAnomalies {
		d.anomalies = d.anomalies[1:]
	}
}

// GetAnomalies returns recent anomalies.
func (d *AnomalyDetector) GetAnomalies(limit int) []Anomaly {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 || limit > len(d.anomalies) {
		limit = len(d.anomalies)
	}

	result := make([]Anomaly, limit)
	copy(result, d.anomalies[len(d.anomalies)-limit:])
	return result
}

// GetJobAnomalies returns anomalies for a specific job.
func (d *AnomalyDetector) GetJobAnomalies(jobID string, limit int) []Anomaly {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []Anomaly
	for i := len(d.anomalies) - 1; i >= 0 && len(result) < limit; i-- {
		if d.anomalies[i].JobID == jobID {
			result = append(result, d.anomalies[i])
		}
	}

	// Reverse to get chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// GetJobStats returns statistics for a job.
func (d *AnomalyDetector) GetJobStats(jobID string) (*JobStats, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats, exists := d.jobStats[jobID]
	return stats, exists
}

// OptimalTimeWindow calculates the optimal time window for job execution.
type OptimalTimeWindow struct {
	StartHour     int           `json:"start_hour"`
	EndHour       int           `json:"end_hour"`
	Score         float64       `json:"score"`
	AvgDuration   time.Duration `json:"avg_duration"`
	SuccessRate   float64       `json:"success_rate"`
	LoadLevel     string        `json:"load_level"` // "low", "medium", "high"
	Confidence    float64       `json:"confidence"` // 0.0-1.0
	SuggestedCron string        `json:"suggested_cron"`
}

// CalculateOptimalWindows calculates optimal execution time windows.
func (d *AnomalyDetector) CalculateOptimalWindows(jobID string, optimizer *Optimizer) []OptimalTimeWindow {
	d.mu.RLock()
	stats, exists := d.jobStats[jobID]
	d.mu.RUnlock()

	if !exists || stats.SampleCount < 10 {
		return nil
	}

	loadPattern := optimizer.GetLoadPattern()
	
	type hourScore struct {
		hour        int
		successRate float64
		avgDuration float64
		count       int
		loadLevel   float64
		score       float64
	}

	var scores []hourScore
	for hour := 0; hour < 24; hour++ {
		hs := stats.HourlyStats[hour]
		if hs.ExecutionCount < 2 {
			continue
		}

		successRate := float64(hs.SuccessCount) / float64(hs.ExecutionCount)
		avgDuration := hs.DurationSum / float64(hs.ExecutionCount)
		loadLevel := loadPattern.HourlyLoad[hour]

		// Score formula: high success rate + low duration + low load = high score
		// Weights: success rate 50%, inverse duration 30%, inverse load 20%
		durationScore := 1.0 - math.Min(avgDuration/3600.0, 1.0) // Normalize to 1 hour max
		loadScore := 1.0 - loadLevel

		score := (successRate * 0.5) + (durationScore * 0.3) + (loadScore * 0.2)

		scores = append(scores, hourScore{
			hour:        hour,
			successRate: successRate,
			avgDuration: avgDuration,
			count:       hs.ExecutionCount,
			loadLevel:   loadLevel,
			score:       score,
		})
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Convert to windows (could group consecutive hours)
	var windows []OptimalTimeWindow
	for i, s := range scores {
		if i >= 3 { // Top 3 windows
			break
		}

		loadLevelStr := "medium"
		if s.loadLevel < 0.3 {
			loadLevelStr = "low"
		} else if s.loadLevel > 0.7 {
			loadLevelStr = "high"
		}

		confidence := math.Min(float64(s.count)/50.0, 1.0) // Full confidence at 50 samples

		windows = append(windows, OptimalTimeWindow{
			StartHour:     s.hour,
			EndHour:       (s.hour + 1) % 24,
			Score:         s.score,
			AvgDuration:   time.Duration(s.avgDuration * float64(time.Second)),
			SuccessRate:   s.successRate,
			LoadLevel:     loadLevelStr,
			Confidence:    confidence,
			SuggestedCron: fmt.Sprintf("0 %d * * *", s.hour),
		})
	}

	return windows
}

// TrendAnalysis provides trend analysis for a job.
type TrendAnalysis struct {
	JobID           string    `json:"job_id"`
	DurationTrend   string    `json:"duration_trend"`   // "increasing", "decreasing", "stable"
	FailureTrend    string    `json:"failure_trend"`    // "increasing", "decreasing", "stable"
	DurationSlope   float64   `json:"duration_slope"`   // Change per execution
	FailureSlope    float64   `json:"failure_slope"`    // Change per execution
	Forecast        *Forecast `json:"forecast,omitempty"`
	AnalyzedAt      time.Time `json:"analyzed_at"`
}

// Forecast provides execution forecasting.
type Forecast struct {
	NextDuration    time.Duration `json:"next_duration"`
	DurationRange   [2]time.Duration `json:"duration_range"` // [min, max]
	FailureProbability float64 `json:"failure_probability"`
	Confidence      float64 `json:"confidence"`
}

// AnalyzeTrends analyzes execution trends for a job.
func (d *AnomalyDetector) AnalyzeTrends(jobID string) *TrendAnalysis {
	d.mu.RLock()
	stats, exists := d.jobStats[jobID]
	d.mu.RUnlock()

	if !exists || stats.SampleCount < 20 {
		return nil
	}

	// Get recent durations from circular buffer
	n := min(stats.SampleCount, d.config.WindowSize)
	durations := make([]float64, n)
	for i := 0; i < n; i++ {
		idx := (stats.bufferIdx - n + i + d.config.WindowSize) % d.config.WindowSize
		durations[i] = stats.RecentDurations[idx]
	}

	// Calculate duration trend using simple linear regression
	durationSlope := calculateSlope(durations)
	durationTrend := "stable"
	if math.Abs(durationSlope) > stats.DurationStdDev*0.1 {
		if durationSlope > 0 {
			durationTrend = "increasing"
		} else {
			durationTrend = "decreasing"
		}
	}

	// Calculate failure trend
	failureRate := float64(stats.FailureCount) / float64(stats.SampleCount)
	failureTrend := "stable"
	if failureRate > 0.3 {
		failureTrend = "increasing"
	} else if failureRate < 0.1 {
		failureTrend = "decreasing"
	}

	// Simple forecast
	forecast := &Forecast{
		NextDuration:       time.Duration((stats.DurationMean + durationSlope) * float64(time.Second)),
		DurationRange:      [2]time.Duration{
			time.Duration(math.Max(0, (stats.DurationMean-2*stats.DurationStdDev)) * float64(time.Second)),
			time.Duration((stats.DurationMean + 2*stats.DurationStdDev) * float64(time.Second)),
		},
		FailureProbability: failureRate,
		Confidence:         math.Min(float64(stats.SampleCount)/100.0, 1.0),
	}

	return &TrendAnalysis{
		JobID:         jobID,
		DurationTrend: durationTrend,
		FailureTrend:  failureTrend,
		DurationSlope: durationSlope,
		FailureSlope:  0, // Would need historical failure data
		Forecast:      forecast,
		AnalyzedAt:    time.Now(),
	}
}

// calculateSlope performs simple linear regression to find trend slope.
func calculateSlope(values []float64) float64 {
	n := float64(len(values))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	return (n*sumXY - sumX*sumY) / denominator
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
