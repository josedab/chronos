// Package prediction provides smart failure prediction for Chronos.
package prediction

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// FeatureSet contains extracted features for ML-based prediction.
type FeatureSet struct {
	// Time-based features
	HourOfDay        int     `json:"hour_of_day"`
	DayOfWeek        int     `json:"day_of_week"`
	IsWeekend        bool    `json:"is_weekend"`
	IsBusinessHours  bool    `json:"is_business_hours"`
	MinutesSinceMidnight int `json:"minutes_since_midnight"`

	// Historical performance features
	HistoricalFailureRate float64 `json:"historical_failure_rate"`
	RecentFailureRate     float64 `json:"recent_failure_rate"` // Last 10 executions
	FailureRateTrend      float64 `json:"failure_rate_trend"`  // Positive = worsening
	ConsecutiveFailures   int     `json:"consecutive_failures"`
	ConsecutiveSuccesses  int     `json:"consecutive_successes"`

	// Duration features
	AvgDuration           float64 `json:"avg_duration_ms"`
	DurationVariance      float64 `json:"duration_variance"`
	RecentDurationTrend   float64 `json:"recent_duration_trend"` // Positive = slowing
	LastDuration          float64 `json:"last_duration_ms"`
	DurationP95           float64 `json:"duration_p95_ms"`

	// Retry features
	AvgRetryCount         float64 `json:"avg_retry_count"`
	RetryRateTrend        float64 `json:"retry_rate_trend"`
	LastRetryCount        int     `json:"last_retry_count"`

	// Error pattern features
	UniqueErrorTypes      int     `json:"unique_error_types"`
	DominantErrorType     string  `json:"dominant_error_type"`
	NewErrorTypeRecent    bool    `json:"new_error_type_recent"`
	ErrorDiversity        float64 `json:"error_diversity"` // Shannon entropy

	// Temporal pattern features
	HourlyFailureRate     float64 `json:"hourly_failure_rate"`
	DayOfWeekFailureRate  float64 `json:"day_of_week_failure_rate"`
	TimePatternScore      float64 `json:"time_pattern_score"`

	// Volume features
	TotalExecutions       int     `json:"total_executions"`
	ExecutionsLastHour    int     `json:"executions_last_hour"`
	ExecutionsLastDay     int     `json:"executions_last_day"`

	// Computed composite features
	StabilityScore        float64 `json:"stability_score"` // 0-1, higher = more stable
	HealthScore           float64 `json:"health_score"`    // 0-1, higher = healthier
}

// FeatureExtractor extracts features from execution records.
type FeatureExtractor struct {
	mu sync.RWMutex
}

// NewFeatureExtractor creates a new feature extractor.
func NewFeatureExtractor() *FeatureExtractor {
	return &FeatureExtractor{}
}

// ExtractFeatures extracts a feature set from execution records.
func (e *FeatureExtractor) ExtractFeatures(records []ExecutionRecord, targetTime time.Time) (*FeatureSet, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("no records to extract features from")
	}

	fs := &FeatureSet{}

	// Time-based features
	fs.HourOfDay = targetTime.Hour()
	fs.DayOfWeek = int(targetTime.Weekday())
	fs.IsWeekend = fs.DayOfWeek == 0 || fs.DayOfWeek == 6
	fs.IsBusinessHours = fs.HourOfDay >= 9 && fs.HourOfDay <= 17 && !fs.IsWeekend
	fs.MinutesSinceMidnight = targetTime.Hour()*60 + targetTime.Minute()

	// Calculate historical failure rate
	failures := 0
	for _, r := range records {
		if !r.Success {
			failures++
		}
	}
	fs.HistoricalFailureRate = float64(failures) / float64(len(records))

	// Recent failure rate (last 10)
	recentCount := min(10, len(records))
	recentRecords := records[len(records)-recentCount:]
	recentFailures := 0
	for _, r := range recentRecords {
		if !r.Success {
			recentFailures++
		}
	}
	fs.RecentFailureRate = float64(recentFailures) / float64(recentCount)

	// Failure rate trend
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
		fs.FailureRateTrend = float64(lastFailures-firstFailures) / 10.0
	}

	// Consecutive failures/successes
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Success {
			if fs.ConsecutiveFailures == 0 {
				fs.ConsecutiveSuccesses++
			} else {
				break
			}
		} else {
			if fs.ConsecutiveSuccesses == 0 {
				fs.ConsecutiveFailures++
			} else {
				break
			}
		}
	}

	// Duration features
	var totalDuration float64
	durations := make([]float64, 0, len(records))
	for _, r := range records {
		d := float64(r.Duration.Milliseconds())
		totalDuration += d
		durations = append(durations, d)
	}
	fs.AvgDuration = totalDuration / float64(len(records))

	// Duration variance
	var variance float64
	for _, d := range durations {
		variance += (d - fs.AvgDuration) * (d - fs.AvgDuration)
	}
	fs.DurationVariance = variance / float64(len(durations))

	// Last duration
	fs.LastDuration = float64(records[len(records)-1].Duration.Milliseconds())

	// Duration P95
	sortedDurations := make([]float64, len(durations))
	copy(sortedDurations, durations)
	for i := 0; i < len(sortedDurations)-1; i++ {
		for j := i + 1; j < len(sortedDurations); j++ {
			if sortedDurations[i] > sortedDurations[j] {
				sortedDurations[i], sortedDurations[j] = sortedDurations[j], sortedDurations[i]
			}
		}
	}
	p95Index := int(float64(len(sortedDurations)) * 0.95)
	if p95Index >= len(sortedDurations) {
		p95Index = len(sortedDurations) - 1
	}
	fs.DurationP95 = sortedDurations[p95Index]

	// Recent duration trend
	if len(records) >= 10 {
		firstAvg := 0.0
		lastAvg := 0.0
		for i := 0; i < 5; i++ {
			firstAvg += float64(records[i].Duration.Milliseconds())
			lastAvg += float64(records[len(records)-5+i].Duration.Milliseconds())
		}
		firstAvg /= 5
		lastAvg /= 5
		if firstAvg > 0 {
			fs.RecentDurationTrend = (lastAvg - firstAvg) / firstAvg
		}
	}

	// Retry features
	var totalRetries int
	for _, r := range records {
		totalRetries += r.RetryCount
	}
	fs.AvgRetryCount = float64(totalRetries) / float64(len(records))
	fs.LastRetryCount = records[len(records)-1].RetryCount

	// Retry trend
	if len(records) >= 10 {
		firstRetries := 0
		lastRetries := 0
		for i := 0; i < 5; i++ {
			firstRetries += records[i].RetryCount
			lastRetries += records[len(records)-5+i].RetryCount
		}
		fs.RetryRateTrend = float64(lastRetries-firstRetries) / 5.0
	}

	// Error pattern features
	errorCounts := make(map[string]int)
	for _, r := range records {
		if r.ErrorType != "" {
			errorCounts[r.ErrorType]++
		}
	}
	fs.UniqueErrorTypes = len(errorCounts)

	// Find dominant error type
	maxCount := 0
	for errType, count := range errorCounts {
		if count > maxCount {
			maxCount = count
			fs.DominantErrorType = errType
		}
	}

	// Check for new error types
	oldErrorTypes := make(map[string]bool)
	cutoff := len(records) - min(5, len(records))
	for i := 0; i < cutoff; i++ {
		if records[i].ErrorType != "" {
			oldErrorTypes[records[i].ErrorType] = true
		}
	}
	for i := cutoff; i < len(records); i++ {
		if records[i].ErrorType != "" && !oldErrorTypes[records[i].ErrorType] {
			fs.NewErrorTypeRecent = true
			break
		}
	}

	// Error diversity (Shannon entropy)
	if len(errorCounts) > 0 {
		totalErrors := 0
		for _, count := range errorCounts {
			totalErrors += count
		}
		for _, count := range errorCounts {
			p := float64(count) / float64(totalErrors)
			if p > 0 {
				fs.ErrorDiversity -= p * math.Log2(p)
			}
		}
	}

	// Temporal pattern features
	hourlyFailures := [24]int{}
	hourlyTotal := [24]int{}
	dayFailures := [7]int{}
	dayTotal := [7]int{}

	for _, r := range records {
		h := r.HourOfDay
		d := r.DayOfWeek
		hourlyTotal[h]++
		dayTotal[d]++
		if !r.Success {
			hourlyFailures[h]++
			dayFailures[d]++
		}
	}

	if hourlyTotal[fs.HourOfDay] > 0 {
		fs.HourlyFailureRate = float64(hourlyFailures[fs.HourOfDay]) / float64(hourlyTotal[fs.HourOfDay])
	}
	if dayTotal[fs.DayOfWeek] > 0 {
		fs.DayOfWeekFailureRate = float64(dayFailures[fs.DayOfWeek]) / float64(dayTotal[fs.DayOfWeek])
	}

	// Time pattern score (how much time affects this job)
	if fs.HistoricalFailureRate > 0 {
		fs.TimePatternScore = math.Abs(fs.HourlyFailureRate-fs.HistoricalFailureRate) +
			math.Abs(fs.DayOfWeekFailureRate-fs.HistoricalFailureRate)
	}

	// Volume features
	fs.TotalExecutions = len(records)
	now := time.Now()
	for i := len(records) - 1; i >= 0; i-- {
		age := now.Sub(records[i].Timestamp)
		if age <= time.Hour {
			fs.ExecutionsLastHour++
		}
		if age <= 24*time.Hour {
			fs.ExecutionsLastDay++
		} else {
			break // Records are assumed to be chronologically ordered
		}
	}

	// Composite features
	fs.StabilityScore = calculateStabilityScore(fs)
	fs.HealthScore = calculateHealthScore(fs)

	return fs, nil
}

// calculateStabilityScore computes a stability score from 0-1.
func calculateStabilityScore(fs *FeatureSet) float64 {
	score := 1.0

	// Penalize high variance
	if fs.AvgDuration > 0 {
		cv := math.Sqrt(fs.DurationVariance) / fs.AvgDuration
		score -= math.Min(cv*0.2, 0.3)
	}

	// Penalize high retry rate
	score -= math.Min(fs.AvgRetryCount*0.1, 0.2)

	// Penalize trend degradation
	if fs.FailureRateTrend > 0 {
		score -= math.Min(fs.FailureRateTrend*0.5, 0.2)
	}

	// Penalize consecutive failures
	score -= math.Min(float64(fs.ConsecutiveFailures)*0.05, 0.3)

	return math.Max(0, score)
}

// calculateHealthScore computes a health score from 0-1.
func calculateHealthScore(fs *FeatureSet) float64 {
	score := 1.0

	// Main factor: success rate
	score -= fs.HistoricalFailureRate * 0.5

	// Recent performance matters more
	score -= fs.RecentFailureRate * 0.3

	// Penalize error diversity (many different errors = unstable)
	score -= math.Min(fs.ErrorDiversity*0.1, 0.1)

	// Reward consecutive successes
	score += math.Min(float64(fs.ConsecutiveSuccesses)*0.01, 0.1)

	return math.Max(0, math.Min(1, score))
}

// GradientBoostingPredictor implements a gradient boosting-like ensemble predictor.
type GradientBoostingPredictor struct {
	mu           sync.RWMutex
	trees        []*DecisionStump
	learningRate float64
	numTrees     int
	trained      bool
	featureNames []string
}

// DecisionStump represents a simple decision tree with one split.
type DecisionStump struct {
	Feature    string  `json:"feature"`
	Threshold  float64 `json:"threshold"`
	LeftValue  float64 `json:"left_value"`  // Value if feature <= threshold
	RightValue float64 `json:"right_value"` // Value if feature > threshold
	Weight     float64 `json:"weight"`
}

// NewGradientBoostingPredictor creates a new gradient boosting predictor.
func NewGradientBoostingPredictor(numTrees int, learningRate float64) *GradientBoostingPredictor {
	return &GradientBoostingPredictor{
		learningRate: learningRate,
		numTrees:     numTrees,
		featureNames: []string{
			"historical_failure_rate",
			"recent_failure_rate",
			"failure_rate_trend",
			"consecutive_failures",
			"duration_variance",
			"recent_duration_trend",
			"avg_retry_count",
			"retry_rate_trend",
			"unique_error_types",
			"new_error_type_recent",
			"hourly_failure_rate",
			"day_of_week_failure_rate",
			"stability_score",
			"health_score",
		},
	}
}

// Train trains the gradient boosting model on historical data.
func (gb *GradientBoostingPredictor) Train(features []*FeatureSet, labels []bool) error {
	if len(features) < 10 {
		return fmt.Errorf("insufficient training data: need at least 10 samples, got %d", len(features))
	}

	gb.mu.Lock()
	defer gb.mu.Unlock()

	// Convert labels to float targets (1.0 = failure, 0.0 = success)
	targets := make([]float64, len(labels))
	for i, label := range labels {
		if label {
			targets[i] = 1.0
		}
	}

	// Initialize predictions
	predictions := make([]float64, len(targets))
	avgTarget := 0.0
	for _, t := range targets {
		avgTarget += t
	}
	avgTarget /= float64(len(targets))
	for i := range predictions {
		predictions[i] = avgTarget
	}

	// Train stumps iteratively
	gb.trees = make([]*DecisionStump, 0, gb.numTrees)
	for t := 0; t < gb.numTrees; t++ {
		// Calculate residuals
		residuals := make([]float64, len(targets))
		for i := range residuals {
			residuals[i] = targets[i] - predictions[i]
		}

		// Fit a stump to the residuals
		stump := gb.fitStump(features, residuals)
		gb.trees = append(gb.trees, stump)

		// Update predictions
		for i := range predictions {
			predictions[i] += gb.learningRate * stump.Predict(features[i])
		}
	}

	gb.trained = true
	return nil
}

// fitStump fits a decision stump to the residuals.
func (gb *GradientBoostingPredictor) fitStump(features []*FeatureSet, residuals []float64) *DecisionStump {
	bestStump := &DecisionStump{}
	bestError := math.MaxFloat64

	// Try each feature
	for _, featureName := range gb.featureNames {
		// Get feature values
		values := make([]float64, len(features))
		for i, f := range features {
			values[i] = getFeatureValue(f, featureName)
		}

		// Try different thresholds (using unique values)
		uniqueValues := make(map[float64]bool)
		for _, v := range values {
			uniqueValues[v] = true
		}

		for threshold := range uniqueValues {
			// Calculate left and right averages
			leftSum, leftCount := 0.0, 0
			rightSum, rightCount := 0.0, 0

			for i, v := range values {
				if v <= threshold {
					leftSum += residuals[i]
					leftCount++
				} else {
					rightSum += residuals[i]
					rightCount++
				}
			}

			if leftCount == 0 || rightCount == 0 {
				continue
			}

			leftValue := leftSum / float64(leftCount)
			rightValue := rightSum / float64(rightCount)

			// Calculate error
			var error float64
			for i, v := range values {
				var pred float64
				if v <= threshold {
					pred = leftValue
				} else {
					pred = rightValue
				}
				error += (residuals[i] - pred) * (residuals[i] - pred)
			}

			if error < bestError {
				bestError = error
				bestStump = &DecisionStump{
					Feature:    featureName,
					Threshold:  threshold,
					LeftValue:  leftValue,
					RightValue: rightValue,
					Weight:     1.0,
				}
			}
		}
	}

	return bestStump
}

// Predict predicts failure probability for a feature set.
func (gb *GradientBoostingPredictor) Predict(fs *FeatureSet) (float64, error) {
	gb.mu.RLock()
	defer gb.mu.RUnlock()

	if !gb.trained || len(gb.trees) == 0 {
		return 0, ErrModelNotTrained
	}

	prediction := 0.0
	for _, tree := range gb.trees {
		prediction += gb.learningRate * tree.Predict(fs)
	}

	// Sigmoid to convert to probability
	probability := 1.0 / (1.0 + math.Exp(-prediction))
	return probability, nil
}

// Predict makes a prediction using a decision stump.
func (s *DecisionStump) Predict(fs *FeatureSet) float64 {
	value := getFeatureValue(fs, s.Feature)
	if value <= s.Threshold {
		return s.LeftValue
	}
	return s.RightValue
}

// getFeatureValue extracts a feature value by name.
func getFeatureValue(fs *FeatureSet, name string) float64 {
	switch name {
	case "historical_failure_rate":
		return fs.HistoricalFailureRate
	case "recent_failure_rate":
		return fs.RecentFailureRate
	case "failure_rate_trend":
		return fs.FailureRateTrend
	case "consecutive_failures":
		return float64(fs.ConsecutiveFailures)
	case "duration_variance":
		return fs.DurationVariance
	case "recent_duration_trend":
		return fs.RecentDurationTrend
	case "avg_retry_count":
		return fs.AvgRetryCount
	case "retry_rate_trend":
		return fs.RetryRateTrend
	case "unique_error_types":
		return float64(fs.UniqueErrorTypes)
	case "new_error_type_recent":
		if fs.NewErrorTypeRecent {
			return 1.0
		}
		return 0.0
	case "hourly_failure_rate":
		return fs.HourlyFailureRate
	case "day_of_week_failure_rate":
		return fs.DayOfWeekFailureRate
	case "stability_score":
		return fs.StabilityScore
	case "health_score":
		return fs.HealthScore
	default:
		return 0
	}
}

// ModelMetrics contains model performance metrics.
type ModelMetrics struct {
	Accuracy    float64 `json:"accuracy"`
	Precision   float64 `json:"precision"`
	Recall      float64 `json:"recall"`
	F1Score     float64 `json:"f1_score"`
	AUC         float64 `json:"auc"`
	SampleCount int     `json:"sample_count"`
}

// Evaluate evaluates the model on test data.
func (gb *GradientBoostingPredictor) Evaluate(features []*FeatureSet, labels []bool) (*ModelMetrics, error) {
	if !gb.trained {
		return nil, ErrModelNotTrained
	}

	var tp, tn, fp, fn int
	threshold := 0.5

	for i, fs := range features {
		prob, _ := gb.Predict(fs)
		predicted := prob >= threshold
		actual := labels[i]

		if predicted && actual {
			tp++
		} else if !predicted && !actual {
			tn++
		} else if predicted && !actual {
			fp++
		} else {
			fn++
		}
	}

	metrics := &ModelMetrics{
		SampleCount: len(features),
	}

	total := float64(tp + tn + fp + fn)
	if total > 0 {
		metrics.Accuracy = float64(tp+tn) / total
	}

	if tp+fp > 0 {
		metrics.Precision = float64(tp) / float64(tp+fp)
	}

	if tp+fn > 0 {
		metrics.Recall = float64(tp) / float64(tp+fn)
	}

	if metrics.Precision+metrics.Recall > 0 {
		metrics.F1Score = 2 * metrics.Precision * metrics.Recall / (metrics.Precision + metrics.Recall)
	}

	// Simple AUC approximation
	metrics.AUC = (metrics.Precision + metrics.Recall) / 2

	return metrics, nil
}

// Serialize serializes the model to JSON.
func (gb *GradientBoostingPredictor) Serialize() ([]byte, error) {
	gb.mu.RLock()
	defer gb.mu.RUnlock()

	data := struct {
		Trees        []*DecisionStump `json:"trees"`
		LearningRate float64          `json:"learning_rate"`
		NumTrees     int              `json:"num_trees"`
		Trained      bool             `json:"trained"`
	}{
		Trees:        gb.trees,
		LearningRate: gb.learningRate,
		NumTrees:     gb.numTrees,
		Trained:      gb.trained,
	}

	return json.Marshal(data)
}

// Deserialize deserializes the model from JSON.
func (gb *GradientBoostingPredictor) Deserialize(data []byte) error {
	gb.mu.Lock()
	defer gb.mu.Unlock()

	var modelData struct {
		Trees        []*DecisionStump `json:"trees"`
		LearningRate float64          `json:"learning_rate"`
		NumTrees     int              `json:"num_trees"`
		Trained      bool             `json:"trained"`
	}

	if err := json.Unmarshal(data, &modelData); err != nil {
		return err
	}

	gb.trees = modelData.Trees
	gb.learningRate = modelData.LearningRate
	gb.numTrees = modelData.NumTrees
	gb.trained = modelData.Trained

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
