// Package routing provides ML-based routing predictions.
package routing

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"sync"
	"time"
)

// ML router errors.
var (
	ErrModelNotReady    = errors.New("ML model not ready")
	ErrInsufficientData = errors.New("insufficient data for prediction")
)

// RoutingFeatureSet contains features for ML-based routing prediction.
type RoutingFeatureSet struct {
	// Target features
	TargetSuccessRate     float64 `json:"target_success_rate"`
	TargetAvgLatency      float64 `json:"target_avg_latency_ms"`
	TargetP95Latency      float64 `json:"target_p95_latency_ms"`
	TargetTrend           float64 `json:"target_trend"` // -1 degrading, 0 stable, 1 improving
	TargetHourlyRate      float64 `json:"target_hourly_rate"`
	TargetDailyRate       float64 `json:"target_daily_rate"`

	// Job-specific features
	JobHistoricalRate     float64 `json:"job_historical_rate"`
	JobRecentRate         float64 `json:"job_recent_rate"`
	JobTargetAffinity     float64 `json:"job_target_affinity"` // How well this job performs on this target

	// Temporal features
	HourOfDay             int     `json:"hour_of_day"`
	DayOfWeek             int     `json:"day_of_week"`
	IsWeekend             bool    `json:"is_weekend"`
	IsBusinessHours       bool    `json:"is_business_hours"`

	// Load features
	RecentExecutionCount  int     `json:"recent_execution_count"`
	QueueDepth            int     `json:"queue_depth"`
	ConcurrentExecutions  int     `json:"concurrent_executions"`
}

// RoutingPrediction contains the ML model's routing prediction.
type RoutingPrediction struct {
	TargetID           string    `json:"target_id"`
	PredictedSuccess   float64   `json:"predicted_success_rate"`
	PredictedLatency   float64   `json:"predicted_latency_ms"`
	Confidence         float64   `json:"confidence"`
	Score              float64   `json:"score"` // Combined score for ranking
	FeatureImportance  map[string]float64 `json:"feature_importance,omitempty"`
	Reasons            []string  `json:"reasons,omitempty"`
}

// MLRouter provides ML-based routing predictions.
type MLRouter struct {
	mu           sync.RWMutex
	dataPipeline *DataPipeline
	model        *GradientBoostingRouter
	config       MLRouterConfig
	trained      bool
	lastTrained  time.Time
}

// MLRouterConfig configures the ML router.
type MLRouterConfig struct {
	// MinTrainingSamples is minimum samples before enabling predictions.
	MinTrainingSamples int `json:"min_training_samples" yaml:"min_training_samples"`
	// RetrainInterval is how often to retrain the model.
	RetrainInterval time.Duration `json:"retrain_interval" yaml:"retrain_interval"`
	// NumTrees is the number of trees in the gradient boosting model.
	NumTrees int `json:"num_trees" yaml:"num_trees"`
	// LearningRate for gradient boosting.
	LearningRate float64 `json:"learning_rate" yaml:"learning_rate"`
	// ConfidenceThreshold below which predictions are marked low confidence.
	ConfidenceThreshold float64 `json:"confidence_threshold" yaml:"confidence_threshold"`
}

// DefaultMLRouterConfig returns sensible defaults.
func DefaultMLRouterConfig() MLRouterConfig {
	return MLRouterConfig{
		MinTrainingSamples:  100,
		RetrainInterval:     time.Hour,
		NumTrees:            50,
		LearningRate:        0.1,
		ConfidenceThreshold: 0.7,
	}
}

// NewMLRouter creates a new ML router.
func NewMLRouter(pipeline *DataPipeline, config MLRouterConfig) *MLRouter {
	return &MLRouter{
		dataPipeline: pipeline,
		model:        NewGradientBoostingRouter(config.NumTrees, config.LearningRate),
		config:       config,
	}
}

// Train trains the ML model on historical data.
func (r *MLRouter) Train(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Collect training data
	features, labels, err := r.collectTrainingData()
	if err != nil {
		return err
	}

	if len(features) < r.config.MinTrainingSamples {
		return ErrInsufficientData
	}

	// Train the model
	if err := r.model.Train(features, labels); err != nil {
		return err
	}

	r.trained = true
	r.lastTrained = time.Now().UTC()
	return nil
}

// collectTrainingData collects feature/label pairs from historical executions.
func (r *MLRouter) collectTrainingData() ([][]float64, []float64, error) {
	allMetrics := r.dataPipeline.GetAllTargetMetrics()
	if len(allMetrics) == 0 {
		return nil, nil, ErrInsufficientData
	}

	var features [][]float64
	var labels []float64

	for targetID, metrics := range allMetrics {
		if metrics.TotalExecutions < 10 {
			continue
		}

		// Create features for each historical data point
		for hour := 0; hour < 24; hour++ {
			for day := 0; day < 7; day++ {
				isWeekend := day == 0 || day == 6
				isBusinessHrs := hour >= 9 && hour <= 17 && !isWeekend

				featureVec := []float64{
					metrics.SuccessRate,
					metrics.AvgDurationMs,
					metrics.P95DurationMs,
					trendToFloat(metrics.RecentTrend),
					metrics.HourlySuccessRate[hour],
					metrics.DailySuccessRate[day],
					metrics.SuccessRate, // job historical (using target as proxy)
					metrics.SuccessRate, // job recent
					1.0,                 // affinity
					float64(hour),
					float64(day),
					boolToFloat(isWeekend),
					boolToFloat(isBusinessHrs),
					float64(metrics.TotalExecutions) / 1000.0, // normalized
					0.0, // queue depth
					0.0, // concurrent
				}

				features = append(features, featureVec)
				labels = append(labels, metrics.HourlySuccessRate[hour])
				_ = targetID
			}
		}
	}

	return features, labels, nil
}

// Predict predicts success probability for a job on multiple targets.
func (r *MLRouter) Predict(ctx context.Context, jobID string, targetIDs []string) ([]RoutingPrediction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.trained {
		return r.fallbackPrediction(jobID, targetIDs)
	}

	predictions := make([]RoutingPrediction, 0, len(targetIDs))
	now := time.Now()

	for _, targetID := range targetIDs {
		features, err := r.extractFeatures(jobID, targetID, now)
		if err != nil {
			// Fallback for this target
			pred := r.createFallbackPrediction(targetID)
			predictions = append(predictions, pred)
			continue
		}

		featureVec := r.featureSetToVector(features)
		successProb, err := r.model.Predict(featureVec)
		if err != nil {
			pred := r.createFallbackPrediction(targetID)
			predictions = append(predictions, pred)
			continue
		}

		// Calculate confidence based on data availability
		confidence := r.calculateConfidence(jobID, targetID)

		pred := RoutingPrediction{
			TargetID:          targetID,
			PredictedSuccess:  successProb,
			PredictedLatency:  features.TargetAvgLatency,
			Confidence:        confidence,
			Score:             r.calculateScore(successProb, features.TargetAvgLatency, confidence),
			FeatureImportance: r.getFeatureImportance(features),
			Reasons:           r.generateReasons(features, successProb),
		}
		predictions = append(predictions, pred)
	}

	// Sort by score descending
	sort.Slice(predictions, func(i, j int) bool {
		return predictions[i].Score > predictions[j].Score
	})

	return predictions, nil
}

// extractFeatures extracts features for prediction.
func (r *MLRouter) extractFeatures(jobID, targetID string, t time.Time) (*RoutingFeatureSet, error) {
	targetMetrics, err := r.dataPipeline.GetTargetMetrics(targetID)
	if err != nil {
		return nil, err
	}

	hour := t.Hour()
	day := int(t.Weekday())
	isWeekend := day == 0 || day == 6
	isBusinessHrs := hour >= 9 && hour <= 17 && !isWeekend

	features := &RoutingFeatureSet{
		TargetSuccessRate:    targetMetrics.SuccessRate,
		TargetAvgLatency:     targetMetrics.AvgDurationMs,
		TargetP95Latency:     targetMetrics.P95DurationMs,
		TargetTrend:          trendToFloat(targetMetrics.RecentTrend),
		TargetHourlyRate:     targetMetrics.HourlySuccessRate[hour],
		TargetDailyRate:      targetMetrics.DailySuccessRate[day],
		JobHistoricalRate:    targetMetrics.SuccessRate,
		JobRecentRate:        targetMetrics.SuccessRate,
		JobTargetAffinity:    1.0,
		HourOfDay:            hour,
		DayOfWeek:            day,
		IsWeekend:            isWeekend,
		IsBusinessHours:      isBusinessHrs,
		RecentExecutionCount: int(targetMetrics.TotalExecutions),
	}

	// Try to get job-specific stats
	jobStats, err := r.dataPipeline.GetJobTargetStats(jobID, targetID)
	if err == nil && jobStats != nil {
		if len(jobStats.HistoricalData) > 0 {
			features.JobHistoricalRate = avgFloat64(jobStats.HistoricalData)
			if len(jobStats.HistoricalData) > 5 {
				features.JobRecentRate = avgFloat64(jobStats.HistoricalData[len(jobStats.HistoricalData)-5:])
			}
		}
		features.JobTargetAffinity = jobStats.PredictedRate
	}

	return features, nil
}

// featureSetToVector converts feature set to float vector.
func (r *MLRouter) featureSetToVector(f *RoutingFeatureSet) []float64 {
	return []float64{
		f.TargetSuccessRate,
		f.TargetAvgLatency / 1000.0, // Normalize to seconds
		f.TargetP95Latency / 1000.0,
		f.TargetTrend,
		f.TargetHourlyRate,
		f.TargetDailyRate,
		f.JobHistoricalRate,
		f.JobRecentRate,
		f.JobTargetAffinity,
		float64(f.HourOfDay) / 24.0,
		float64(f.DayOfWeek) / 7.0,
		boolToFloat(f.IsWeekend),
		boolToFloat(f.IsBusinessHours),
		float64(f.RecentExecutionCount) / 1000.0,
		float64(f.QueueDepth) / 100.0,
		float64(f.ConcurrentExecutions) / 10.0,
	}
}

// calculateScore calculates a combined routing score.
func (r *MLRouter) calculateScore(successProb, latency, confidence float64) float64 {
	// Weighted combination: success rate most important, then latency, then confidence
	successWeight := 0.6
	latencyWeight := 0.25
	confidenceWeight := 0.15

	// Normalize latency (lower is better, max 10s)
	latencyScore := math.Max(0, 1.0-latency/10000.0)

	return successWeight*successProb + latencyWeight*latencyScore + confidenceWeight*confidence
}

// calculateConfidence calculates prediction confidence based on data availability.
func (r *MLRouter) calculateConfidence(jobID, targetID string) float64 {
	metrics, err := r.dataPipeline.GetTargetMetrics(targetID)
	if err != nil {
		return 0.3 // Low confidence without data
	}

	// Base confidence from execution count
	execConfidence := math.Min(1.0, float64(metrics.TotalExecutions)/float64(r.config.MinTrainingSamples))

	// Recency bonus
	recencyBonus := 0.0
	if time.Since(metrics.LastUpdated) < 10*time.Minute {
		recencyBonus = 0.1
	}

	// Stability bonus (stable is more predictable)
	stabilityBonus := 0.0
	if metrics.RecentTrend == "stable" {
		stabilityBonus = 0.1
	}

	return math.Min(1.0, 0.5*execConfidence+0.3+recencyBonus+stabilityBonus)
}

// getFeatureImportance returns which features contributed most to the prediction.
func (r *MLRouter) getFeatureImportance(f *RoutingFeatureSet) map[string]float64 {
	importance := make(map[string]float64)
	
	// Simplified importance based on feature deviation from average
	importance["target_success_rate"] = math.Abs(f.TargetSuccessRate - 0.95)
	importance["target_trend"] = math.Abs(f.TargetTrend)
	importance["hourly_rate"] = math.Abs(f.TargetHourlyRate - f.TargetSuccessRate)
	importance["job_affinity"] = math.Abs(f.JobTargetAffinity - 0.5)

	return importance
}

// generateReasons generates human-readable reasons for the prediction.
func (r *MLRouter) generateReasons(f *RoutingFeatureSet, predicted float64) []string {
	var reasons []string

	if f.TargetSuccessRate > 0.98 {
		reasons = append(reasons, "Target has excellent historical success rate (>98%)")
	} else if f.TargetSuccessRate < 0.90 {
		reasons = append(reasons, "Target has below-average success rate (<90%)")
	}

	if f.TargetTrend > 0 {
		reasons = append(reasons, "Target performance is improving")
	} else if f.TargetTrend < 0 {
		reasons = append(reasons, "Target performance is degrading")
	}

	if f.TargetHourlyRate > f.TargetSuccessRate+0.05 {
		reasons = append(reasons, "Target performs better during this hour")
	} else if f.TargetHourlyRate < f.TargetSuccessRate-0.05 {
		reasons = append(reasons, "Target performs worse during this hour")
	}

	if f.TargetP95Latency > 5000 {
		reasons = append(reasons, "Target has high P95 latency (>5s)")
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "Target has stable, average performance")
	}

	return reasons
}

// fallbackPrediction creates predictions without ML model.
func (r *MLRouter) fallbackPrediction(jobID string, targetIDs []string) ([]RoutingPrediction, error) {
	predictions := make([]RoutingPrediction, 0, len(targetIDs))

	for _, targetID := range targetIDs {
		pred := r.createFallbackPrediction(targetID)
		predictions = append(predictions, pred)
	}

	// Sort by score
	sort.Slice(predictions, func(i, j int) bool {
		return predictions[i].Score > predictions[j].Score
	})

	return predictions, nil
}

// createFallbackPrediction creates a prediction using only raw metrics.
func (r *MLRouter) createFallbackPrediction(targetID string) RoutingPrediction {
	pred := RoutingPrediction{
		TargetID:         targetID,
		PredictedSuccess: 0.5,
		PredictedLatency: 1000,
		Confidence:       0.3,
		Score:            0.5,
		Reasons:          []string{"Using fallback prediction (insufficient ML data)"},
	}

	metrics, err := r.dataPipeline.GetTargetMetrics(targetID)
	if err == nil && metrics != nil {
		pred.PredictedSuccess = metrics.SuccessRate
		pred.PredictedLatency = metrics.AvgDurationMs
		pred.Confidence = math.Min(0.8, float64(metrics.TotalExecutions)/100.0)
		pred.Score = metrics.SuccessRate * pred.Confidence
		pred.Reasons = []string{"Using historical metrics (ML model not trained)"}
	}

	return pred
}

// IsReady returns whether the ML model is ready for predictions.
func (r *MLRouter) IsReady() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.trained
}

// GetModelStats returns statistics about the ML model.
func (r *MLRouter) GetModelStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]interface{}{
		"trained":      r.trained,
		"last_trained": r.lastTrained,
		"num_trees":    r.config.NumTrees,
	}

	if r.trained {
		stats["model_accuracy"] = r.model.GetAccuracy()
	}

	return stats
}

// GradientBoostingRouter implements a gradient boosting model for routing.
type GradientBoostingRouter struct {
	mu           sync.RWMutex
	trees        []*RoutingDecisionTree
	learningRate float64
	numTrees     int
	trained      bool
	accuracy     float64
}

// RoutingDecisionTree is a simple decision tree for routing.
type RoutingDecisionTree struct {
	FeatureIndex int     `json:"feature_index"`
	Threshold    float64 `json:"threshold"`
	LeftValue    float64 `json:"left_value"`
	RightValue   float64 `json:"right_value"`
}

// NewGradientBoostingRouter creates a new gradient boosting router.
func NewGradientBoostingRouter(numTrees int, learningRate float64) *GradientBoostingRouter {
	return &GradientBoostingRouter{
		numTrees:     numTrees,
		learningRate: learningRate,
		trees:        make([]*RoutingDecisionTree, 0, numTrees),
	}
}

// Train trains the gradient boosting model.
func (gb *GradientBoostingRouter) Train(features [][]float64, labels []float64) error {
	gb.mu.Lock()
	defer gb.mu.Unlock()

	if len(features) < 10 {
		return ErrInsufficientData
	}

	// Initialize predictions with average
	predictions := make([]float64, len(labels))
	avg := avgFloat64(labels)
	for i := range predictions {
		predictions[i] = avg
	}

	gb.trees = make([]*RoutingDecisionTree, 0, gb.numTrees)

	for t := 0; t < gb.numTrees; t++ {
		// Calculate residuals
		residuals := make([]float64, len(labels))
		for i := range residuals {
			residuals[i] = labels[i] - predictions[i]
		}

		// Fit a tree to the residuals
		tree := gb.fitTree(features, residuals)
		gb.trees = append(gb.trees, tree)

		// Update predictions
		for i := range predictions {
			predictions[i] += gb.learningRate * tree.Predict(features[i])
		}
	}

	// Calculate accuracy (R-squared)
	var ssRes, ssTot float64
	for i := range labels {
		ssRes += (labels[i] - predictions[i]) * (labels[i] - predictions[i])
		ssTot += (labels[i] - avg) * (labels[i] - avg)
	}
	if ssTot > 0 {
		gb.accuracy = 1.0 - ssRes/ssTot
	}

	gb.trained = true
	return nil
}

// fitTree fits a simple decision stump to the residuals.
func (gb *GradientBoostingRouter) fitTree(features [][]float64, residuals []float64) *RoutingDecisionTree {
	if len(features) == 0 || len(features[0]) == 0 {
		return &RoutingDecisionTree{LeftValue: 0, RightValue: 0}
	}

	bestTree := &RoutingDecisionTree{}
	bestError := math.MaxFloat64
	numFeatures := len(features[0])

	for featureIdx := 0; featureIdx < numFeatures; featureIdx++ {
		// Get unique values for this feature
		values := make(map[float64]bool)
		for _, f := range features {
			values[f[featureIdx]] = true
		}

		for threshold := range values {
			leftSum, leftCount := 0.0, 0
			rightSum, rightCount := 0.0, 0

			for i, f := range features {
				if f[featureIdx] <= threshold {
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
			var errorSum float64
			for i, f := range features {
				var pred float64
				if f[featureIdx] <= threshold {
					pred = leftValue
				} else {
					pred = rightValue
				}
				errorSum += (residuals[i] - pred) * (residuals[i] - pred)
			}

			if errorSum < bestError {
				bestError = errorSum
				bestTree = &RoutingDecisionTree{
					FeatureIndex: featureIdx,
					Threshold:    threshold,
					LeftValue:    leftValue,
					RightValue:   rightValue,
				}
			}
		}
	}

	return bestTree
}

// Predict makes a prediction using the ensemble.
func (gb *GradientBoostingRouter) Predict(features []float64) (float64, error) {
	gb.mu.RLock()
	defer gb.mu.RUnlock()

	if !gb.trained || len(gb.trees) == 0 {
		return 0, ErrModelNotReady
	}

	prediction := 0.0
	for _, tree := range gb.trees {
		prediction += gb.learningRate * tree.Predict(features)
	}

	// Clamp to [0, 1] for probability
	return math.Max(0, math.Min(1, prediction)), nil
}

// Predict makes a prediction using a single tree.
func (t *RoutingDecisionTree) Predict(features []float64) float64 {
	if t.FeatureIndex >= len(features) {
		return t.LeftValue
	}
	if features[t.FeatureIndex] <= t.Threshold {
		return t.LeftValue
	}
	return t.RightValue
}

// GetAccuracy returns the model accuracy.
func (gb *GradientBoostingRouter) GetAccuracy() float64 {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return gb.accuracy
}

// Serialize serializes the model to JSON.
func (gb *GradientBoostingRouter) Serialize() ([]byte, error) {
	gb.mu.RLock()
	defer gb.mu.RUnlock()

	data := struct {
		Trees        []*RoutingDecisionTree `json:"trees"`
		LearningRate float64                `json:"learning_rate"`
		Trained      bool                   `json:"trained"`
		Accuracy     float64                `json:"accuracy"`
	}{
		Trees:        gb.trees,
		LearningRate: gb.learningRate,
		Trained:      gb.trained,
		Accuracy:     gb.accuracy,
	}

	return json.Marshal(data)
}

// Helper functions

func trendToFloat(trend string) float64 {
	switch trend {
	case "improving":
		return 1.0
	case "degrading":
		return -1.0
	default:
		return 0.0
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func avgFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
