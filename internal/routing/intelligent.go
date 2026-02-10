// Package routing provides intelligent routing with ML-based decisions and A/B testing.
package routing

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Intelligent router errors.
var (
	ErrNoTargetsAvailable = errors.New("no targets available for routing")
	ErrExperimentNotFound = errors.New("A/B experiment not found")
)

// IntelligentRouter provides ML-augmented routing decisions.
type IntelligentRouter struct {
	mu             sync.RWMutex
	baseRouter     *Router
	dataPipeline   *DataPipeline
	mlRouter       *MLRouter
	experiments    map[string]*ABExperiment
	config         IntelligentRouterConfig
	decisionLog    []RoutingDecisionLog
	maxDecisionLog int
}

// IntelligentRouterConfig configures the intelligent router.
type IntelligentRouterConfig struct {
	// EnableML enables ML-based routing predictions.
	EnableML bool `json:"enable_ml" yaml:"enable_ml"`
	// MLWeight is the weight given to ML predictions vs rule-based (0-1).
	MLWeight float64 `json:"ml_weight" yaml:"ml_weight"`
	// MinConfidenceForML is minimum confidence to use ML routing.
	MinConfidenceForML float64 `json:"min_confidence_for_ml" yaml:"min_confidence_for_ml"`
	// EnableABTesting enables A/B testing for routing strategies.
	EnableABTesting bool `json:"enable_ab_testing" yaml:"enable_ab_testing"`
	// FallbackStrategy is used when ML is unavailable.
	FallbackStrategy RoutingStrategy `json:"fallback_strategy" yaml:"fallback_strategy"`
}

// DefaultIntelligentRouterConfig returns sensible defaults.
func DefaultIntelligentRouterConfig() IntelligentRouterConfig {
	return IntelligentRouterConfig{
		EnableML:           true,
		MLWeight:           0.7,
		MinConfidenceForML: 0.6,
		EnableABTesting:    true,
		FallbackStrategy:   StrategyHealthBased,
	}
}

// NewIntelligentRouter creates a new intelligent router.
func NewIntelligentRouter(config IntelligentRouterConfig) *IntelligentRouter {
	dpConfig := DefaultDataPipelineConfig()
	dataPipeline := NewDataPipeline(dpConfig)

	mlConfig := DefaultMLRouterConfig()
	mlRouter := NewMLRouter(dataPipeline, mlConfig)

	return &IntelligentRouter{
		baseRouter:     NewRouter(),
		dataPipeline:   dataPipeline,
		mlRouter:       mlRouter,
		experiments:    make(map[string]*ABExperiment),
		config:         config,
		decisionLog:    make([]RoutingDecisionLog, 0),
		maxDecisionLog: 10000,
	}
}

// RoutingDecisionLog logs routing decisions for analysis.
type RoutingDecisionLog struct {
	ID               string            `json:"id"`
	Timestamp        time.Time         `json:"timestamp"`
	JobID            string            `json:"job_id"`
	SelectedTarget   string            `json:"selected_target"`
	Strategy         string            `json:"strategy"`
	MLPrediction     *RoutingPrediction `json:"ml_prediction,omitempty"`
	RuleBasedScore   float64           `json:"rule_based_score"`
	FinalScore       float64           `json:"final_score"`
	ExperimentID     string            `json:"experiment_id,omitempty"`
	ExperimentVariant string           `json:"experiment_variant,omitempty"`
	Outcome          *RoutingOutcome   `json:"outcome,omitempty"`
}

// RoutingOutcome records the actual outcome of a routing decision.
type RoutingOutcome struct {
	Success      bool          `json:"success"`
	Duration     time.Duration `json:"duration"`
	StatusCode   int           `json:"status_code"`
	RecordedAt   time.Time     `json:"recorded_at"`
}

// ABExperiment defines an A/B test for routing strategies.
type ABExperiment struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Status         ExperimentStatus   `json:"status"`
	Variants       []*ExperimentVariant `json:"variants"`
	StartTime      time.Time          `json:"start_time"`
	EndTime        time.Time          `json:"end_time,omitempty"`
	TargetJobIDs   []string           `json:"target_job_ids,omitempty"` // Empty = all jobs
	TrafficPercent float64            `json:"traffic_percent"`          // % of traffic in experiment
	Results        *ExperimentResults `json:"results,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// ExperimentStatus represents the status of an experiment.
type ExperimentStatus string

const (
	ExperimentDraft     ExperimentStatus = "draft"
	ExperimentRunning   ExperimentStatus = "running"
	ExperimentPaused    ExperimentStatus = "paused"
	ExperimentCompleted ExperimentStatus = "completed"
)

// ExperimentVariant represents a variant in an A/B test.
type ExperimentVariant struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Strategy       RoutingStrategy `json:"strategy"`
	EnableML       bool            `json:"enable_ml"`
	MLWeight       float64         `json:"ml_weight"`
	TrafficWeight  float64         `json:"traffic_weight"` // Relative weight
	Metrics        *VariantMetrics `json:"metrics"`
}

// VariantMetrics tracks performance metrics for a variant.
type VariantMetrics struct {
	TotalDecisions   int64         `json:"total_decisions"`
	SuccessCount     int64         `json:"success_count"`
	FailureCount     int64         `json:"failure_count"`
	SuccessRate      float64       `json:"success_rate"`
	AvgLatencyMs     float64       `json:"avg_latency_ms"`
	P95LatencyMs     float64       `json:"p95_latency_ms"`
	LastUpdated      time.Time     `json:"last_updated"`
}

// ExperimentResults contains the final results of an experiment.
type ExperimentResults struct {
	WinningVariant   string             `json:"winning_variant"`
	Confidence       float64            `json:"confidence"`
	Improvement      float64            `json:"improvement_percent"`
	VariantMetrics   map[string]*VariantMetrics `json:"variant_metrics"`
	StatisticalSig   bool               `json:"statistically_significant"`
	CalculatedAt     time.Time          `json:"calculated_at"`
}

// Route makes an intelligent routing decision.
func (ir *IntelligentRouter) Route(ctx context.Context, jobID string, policyID string) (*IntelligentRouteDecision, error) {
	ir.mu.RLock()
	config := ir.config
	ir.mu.RUnlock()

	decision := &IntelligentRouteDecision{
		ID:        uuid.New().String(),
		JobID:     jobID,
		Timestamp: time.Now().UTC(),
	}

	// Get available targets
	targets := ir.getAvailableTargets(policyID)
	if len(targets) == 0 {
		return nil, ErrNoTargetsAvailable
	}

	targetIDs := make([]string, len(targets))
	for i, t := range targets {
		targetIDs[i] = t.ID
	}

	// Check for active experiment
	var experiment *ABExperiment
	var variant *ExperimentVariant
	if config.EnableABTesting {
		experiment, variant = ir.selectExperiment(jobID)
		if experiment != nil {
			decision.ExperimentID = experiment.ID
			decision.ExperimentVariant = variant.Name
			config.EnableML = variant.EnableML
			config.MLWeight = variant.MLWeight
		}
	}

	// Get ML predictions if enabled
	var mlPredictions []RoutingPrediction
	if config.EnableML && ir.mlRouter.IsReady() {
		var err error
		mlPredictions, err = ir.mlRouter.Predict(ctx, jobID, targetIDs)
		if err == nil && len(mlPredictions) > 0 {
			decision.MLPredictions = mlPredictions
		}
	}

	// Get rule-based scores
	ruleScores := ir.calculateRuleBasedScores(targets, policyID)
	decision.RuleBasedScores = ruleScores

	// Combine ML and rule-based scores
	finalScores := ir.combineScores(targetIDs, mlPredictions, ruleScores, config)
	decision.FinalScores = finalScores

	// Select best target
	bestTarget, bestScore := ir.selectBestTarget(finalScores)
	decision.SelectedTarget = bestTarget
	decision.FinalScore = bestScore
	decision.Strategy = ir.determineStrategy(mlPredictions, config)

	// Log the decision
	ir.logDecision(decision, experiment, variant)

	return decision, nil
}

// IntelligentRouteDecision contains the routing decision details.
type IntelligentRouteDecision struct {
	ID                string                    `json:"id"`
	JobID             string                    `json:"job_id"`
	Timestamp         time.Time                 `json:"timestamp"`
	SelectedTarget    string                    `json:"selected_target"`
	Strategy          string                    `json:"strategy"`
	MLPredictions     []RoutingPrediction       `json:"ml_predictions,omitempty"`
	RuleBasedScores   map[string]float64        `json:"rule_based_scores"`
	FinalScores       map[string]float64        `json:"final_scores"`
	FinalScore        float64                   `json:"final_score"`
	ExperimentID      string                    `json:"experiment_id,omitempty"`
	ExperimentVariant string                    `json:"experiment_variant,omitempty"`
	Confidence        float64                   `json:"confidence"`
	Reasons           []string                  `json:"reasons,omitempty"`
}

// getAvailableTargets returns available routing targets.
func (ir *IntelligentRouter) getAvailableTargets(policyID string) []*Region {
	ir.baseRouter.mu.RLock()
	defer ir.baseRouter.mu.RUnlock()

	policy := ir.baseRouter.defaultPolicy
	if policyID != "" {
		if p, exists := ir.baseRouter.policies[policyID]; exists {
			policy = p
		}
	}

	return ir.baseRouter.getAvailableRegions(policy)
}

// calculateRuleBasedScores calculates scores using the base router logic.
func (ir *IntelligentRouter) calculateRuleBasedScores(regions []*Region, policyID string) map[string]float64 {
	ir.baseRouter.mu.RLock()
	defer ir.baseRouter.mu.RUnlock()

	policy := ir.baseRouter.defaultPolicy
	if policyID != "" {
		if p, exists := ir.baseRouter.policies[policyID]; exists {
			policy = p
		}
	}

	scores := make(map[string]float64)
	for _, region := range regions {
		score := ir.baseRouter.calculateScore(region, policy)
		scores[region.ID] = score / 100.0 // Normalize to 0-1
	}

	return scores
}

// combineScores combines ML predictions with rule-based scores.
func (ir *IntelligentRouter) combineScores(
	targetIDs []string,
	mlPredictions []RoutingPrediction,
	ruleScores map[string]float64,
	config IntelligentRouterConfig,
) map[string]float64 {
	combined := make(map[string]float64)

	// Create ML score map
	mlScores := make(map[string]float64)
	mlConfidence := make(map[string]float64)
	for _, pred := range mlPredictions {
		mlScores[pred.TargetID] = pred.Score
		mlConfidence[pred.TargetID] = pred.Confidence
	}

	for _, targetID := range targetIDs {
		ruleScore := ruleScores[targetID]
		mlScore := mlScores[targetID]
		confidence := mlConfidence[targetID]

		// Determine weights based on confidence
		mlWeight := config.MLWeight
		if confidence < config.MinConfidenceForML {
			mlWeight *= confidence / config.MinConfidenceForML
		}
		ruleWeight := 1.0 - mlWeight

		// Combined score
		if mlScore > 0 {
			combined[targetID] = mlWeight*mlScore + ruleWeight*ruleScore
		} else {
			combined[targetID] = ruleScore
		}
	}

	return combined
}

// selectBestTarget selects the target with the highest score.
func (ir *IntelligentRouter) selectBestTarget(scores map[string]float64) (string, float64) {
	var bestTarget string
	var bestScore float64

	for targetID, score := range scores {
		if score > bestScore || bestTarget == "" {
			bestTarget = targetID
			bestScore = score
		}
	}

	return bestTarget, bestScore
}

// determineStrategy returns the strategy used for this decision.
func (ir *IntelligentRouter) determineStrategy(mlPredictions []RoutingPrediction, config IntelligentRouterConfig) string {
	if len(mlPredictions) > 0 && config.EnableML {
		return "intelligent_ml"
	}
	return string(config.FallbackStrategy)
}

// selectExperiment selects an active experiment and variant for a job.
func (ir *IntelligentRouter) selectExperiment(jobID string) (*ABExperiment, *ExperimentVariant) {
	ir.mu.RLock()
	defer ir.mu.RUnlock()

	for _, exp := range ir.experiments {
		if exp.Status != ExperimentRunning {
			continue
		}

		// Check if job is targeted
		if len(exp.TargetJobIDs) > 0 {
			targeted := false
			for _, id := range exp.TargetJobIDs {
				if id == jobID {
					targeted = true
					break
				}
			}
			if !targeted {
				continue
			}
		}

		// Check traffic allocation
		if rand.Float64() > exp.TrafficPercent {
			continue
		}

		// Select variant based on weights
		variant := ir.selectVariant(exp)
		return exp, variant
	}

	return nil, nil
}

// selectVariant selects a variant based on traffic weights.
func (ir *IntelligentRouter) selectVariant(exp *ABExperiment) *ExperimentVariant {
	totalWeight := 0.0
	for _, v := range exp.Variants {
		totalWeight += v.TrafficWeight
	}

	r := rand.Float64() * totalWeight
	cumulative := 0.0
	for _, v := range exp.Variants {
		cumulative += v.TrafficWeight
		if r <= cumulative {
			return v
		}
	}

	return exp.Variants[0]
}

// logDecision logs a routing decision.
func (ir *IntelligentRouter) logDecision(decision *IntelligentRouteDecision, exp *ABExperiment, variant *ExperimentVariant) {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	log := RoutingDecisionLog{
		ID:             decision.ID,
		Timestamp:      decision.Timestamp,
		JobID:          decision.JobID,
		SelectedTarget: decision.SelectedTarget,
		Strategy:       decision.Strategy,
		FinalScore:     decision.FinalScore,
	}

	if len(decision.MLPredictions) > 0 {
		log.MLPrediction = &decision.MLPredictions[0]
	}
	if decision.RuleBasedScores != nil {
		log.RuleBasedScore = decision.RuleBasedScores[decision.SelectedTarget]
	}
	if exp != nil {
		log.ExperimentID = exp.ID
	}
	if variant != nil {
		log.ExperimentVariant = variant.Name
	}

	ir.decisionLog = append(ir.decisionLog, log)
	if len(ir.decisionLog) > ir.maxDecisionLog {
		ir.decisionLog = ir.decisionLog[len(ir.decisionLog)-ir.maxDecisionLog:]
	}
}

// RecordOutcome records the outcome of a routing decision.
func (ir *IntelligentRouter) RecordOutcome(decisionID string, outcome RoutingOutcome) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	// Find and update the decision log
	for i := len(ir.decisionLog) - 1; i >= 0; i-- {
		if ir.decisionLog[i].ID == decisionID {
			ir.decisionLog[i].Outcome = &outcome

			// Update experiment metrics if applicable
			if ir.decisionLog[i].ExperimentID != "" {
				ir.updateExperimentMetrics(
					ir.decisionLog[i].ExperimentID,
					ir.decisionLog[i].ExperimentVariant,
					outcome,
				)
			}

			// Record in data pipeline
			exec := ExecutionResult{
				Success:    outcome.Success,
				StatusCode: outcome.StatusCode,
				Duration:   outcome.Duration,
				ExecutedAt: outcome.RecordedAt,
			}
			_ = ir.dataPipeline.RecordExecution(
				context.Background(),
				ir.decisionLog[i].JobID,
				ir.decisionLog[i].SelectedTarget,
				exec,
			)

			return nil
		}
	}

	return errors.New("decision not found")
}

// updateExperimentMetrics updates metrics for an experiment variant.
func (ir *IntelligentRouter) updateExperimentMetrics(expID, variantName string, outcome RoutingOutcome) {
	exp, exists := ir.experiments[expID]
	if !exists {
		return
	}

	for _, v := range exp.Variants {
		if v.Name == variantName {
			if v.Metrics == nil {
				v.Metrics = &VariantMetrics{}
			}

			v.Metrics.TotalDecisions++
			if outcome.Success {
				v.Metrics.SuccessCount++
			} else {
				v.Metrics.FailureCount++
			}
			v.Metrics.SuccessRate = float64(v.Metrics.SuccessCount) / float64(v.Metrics.TotalDecisions)

			// Update latency (EMA)
			alpha := 0.1
			latencyMs := float64(outcome.Duration.Milliseconds())
			if v.Metrics.AvgLatencyMs == 0 {
				v.Metrics.AvgLatencyMs = latencyMs
			} else {
				v.Metrics.AvgLatencyMs = alpha*latencyMs + (1-alpha)*v.Metrics.AvgLatencyMs
			}

			v.Metrics.LastUpdated = time.Now().UTC()
			break
		}
	}

	exp.UpdatedAt = time.Now().UTC()
}

// CreateExperiment creates a new A/B experiment.
func (ir *IntelligentRouter) CreateExperiment(exp *ABExperiment) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	if exp.ID == "" {
		exp.ID = uuid.New().String()
	}

	now := time.Now().UTC()
	exp.CreatedAt = now
	exp.UpdatedAt = now
	exp.Status = ExperimentDraft

	// Initialize variant metrics
	for _, v := range exp.Variants {
		if v.ID == "" {
			v.ID = uuid.New().String()
		}
		v.Metrics = &VariantMetrics{}
	}

	ir.experiments[exp.ID] = exp
	return nil
}

// StartExperiment starts an experiment.
func (ir *IntelligentRouter) StartExperiment(expID string) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	exp, exists := ir.experiments[expID]
	if !exists {
		return ErrExperimentNotFound
	}

	exp.Status = ExperimentRunning
	exp.StartTime = time.Now().UTC()
	exp.UpdatedAt = exp.StartTime

	return nil
}

// StopExperiment stops an experiment.
func (ir *IntelligentRouter) StopExperiment(expID string) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	exp, exists := ir.experiments[expID]
	if !exists {
		return ErrExperimentNotFound
	}

	exp.Status = ExperimentCompleted
	exp.EndTime = time.Now().UTC()
	exp.UpdatedAt = exp.EndTime

	// Calculate results
	exp.Results = ir.calculateExperimentResults(exp)

	return nil
}

// calculateExperimentResults calculates the final results of an experiment.
func (ir *IntelligentRouter) calculateExperimentResults(exp *ABExperiment) *ExperimentResults {
	results := &ExperimentResults{
		VariantMetrics: make(map[string]*VariantMetrics),
		CalculatedAt:   time.Now().UTC(),
	}

	var bestVariant *ExperimentVariant
	var controlVariant *ExperimentVariant

	for _, v := range exp.Variants {
		results.VariantMetrics[v.Name] = v.Metrics

		if v.Name == "control" {
			controlVariant = v
		}
		if bestVariant == nil || (v.Metrics != nil && v.Metrics.SuccessRate > bestVariant.Metrics.SuccessRate) {
			bestVariant = v
		}
	}

	if bestVariant != nil {
		results.WinningVariant = bestVariant.Name

		// Calculate improvement over control
		if controlVariant != nil && controlVariant.Metrics != nil && controlVariant.Metrics.SuccessRate > 0 {
			results.Improvement = (bestVariant.Metrics.SuccessRate - controlVariant.Metrics.SuccessRate) /
				controlVariant.Metrics.SuccessRate * 100
		}

		// Simple statistical significance check (would use proper test in production)
		if bestVariant.Metrics != nil && bestVariant.Metrics.TotalDecisions > 100 {
			results.StatisticalSig = true
			results.Confidence = 0.95
		}
	}

	return results
}

// GetExperiment returns an experiment by ID.
func (ir *IntelligentRouter) GetExperiment(expID string) (*ABExperiment, error) {
	ir.mu.RLock()
	defer ir.mu.RUnlock()

	exp, exists := ir.experiments[expID]
	if !exists {
		return nil, ErrExperimentNotFound
	}
	return exp, nil
}

// ListExperiments returns all experiments.
func (ir *IntelligentRouter) ListExperiments() []*ABExperiment {
	ir.mu.RLock()
	defer ir.mu.RUnlock()

	exps := make([]*ABExperiment, 0, len(ir.experiments))
	for _, exp := range ir.experiments {
		exps = append(exps, exp)
	}

	// Sort by created time descending
	sort.Slice(exps, func(i, j int) bool {
		return exps[i].CreatedAt.After(exps[j].CreatedAt)
	})

	return exps
}

// GetDecisionLog returns recent routing decisions.
func (ir *IntelligentRouter) GetDecisionLog(limit int) []RoutingDecisionLog {
	ir.mu.RLock()
	defer ir.mu.RUnlock()

	if limit <= 0 || limit > len(ir.decisionLog) {
		limit = len(ir.decisionLog)
	}

	result := make([]RoutingDecisionLog, limit)
	copy(result, ir.decisionLog[len(ir.decisionLog)-limit:])

	return result
}

// TrainMLModel triggers ML model training.
func (ir *IntelligentRouter) TrainMLModel(ctx context.Context) error {
	return ir.mlRouter.Train(ctx)
}

// GetMLModelStats returns ML model statistics.
func (ir *IntelligentRouter) GetMLModelStats() map[string]interface{} {
	return ir.mlRouter.GetModelStats()
}

// GetDataPipelineStats returns data pipeline statistics.
func (ir *IntelligentRouter) GetDataPipelineStats() map[string]interface{} {
	allMetrics := ir.dataPipeline.GetAllTargetMetrics()

	return map[string]interface{}{
		"total_targets":  len(allMetrics),
		"target_metrics": allMetrics,
	}
}

// RegisterRegion registers a region with the base router.
func (ir *IntelligentRouter) RegisterRegion(region *Region) error {
	return ir.baseRouter.RegisterRegion(region)
}

// SetPolicy sets a routing policy on the base router.
func (ir *IntelligentRouter) SetPolicy(policy *RoutingPolicy) {
	ir.baseRouter.SetPolicy(policy)
}

// Stop stops the intelligent router.
func (ir *IntelligentRouter) Stop() {
	ir.dataPipeline.Stop()
}

// Export exports router state.
func (ir *IntelligentRouter) Export() ([]byte, error) {
	ir.mu.RLock()
	defer ir.mu.RUnlock()

	data := struct {
		Experiments  map[string]*ABExperiment `json:"experiments"`
		DecisionLog  []RoutingDecisionLog     `json:"decision_log"`
	}{
		Experiments: ir.experiments,
		DecisionLog: ir.decisionLog,
	}

	return json.Marshal(data)
}
