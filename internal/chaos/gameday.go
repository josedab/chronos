// Package chaos provides chaos testing framework extensions.
package chaos

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// GameDay represents a coordinated chaos testing event.
type GameDay struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Experiments []ExperimentSpec `json:"experiments"`
	Schedule    *GameDaySchedule `json:"schedule"`
	SafetyRules []SafetyRule     `json:"safety_rules"`
	Status      GameDayStatus    `json:"status"`
	StartTime   time.Time        `json:"start_time,omitempty"`
	EndTime     time.Time        `json:"end_time,omitempty"`
	Results     *GameDayResults  `json:"results,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// ExperimentSpec defines an experiment in a game day.
type ExperimentSpec struct {
	ID             string        `json:"id"`
	ExperimentID   string        `json:"experiment_id"`
	Order          int           `json:"order"`
	StartDelay     time.Duration `json:"start_delay"`
	Parallel       bool          `json:"parallel"`
	DependsOn      []string      `json:"depends_on,omitempty"`
	AbortOnFailure bool          `json:"abort_on_failure"`
}

// GameDaySchedule defines when a game day runs.
type GameDaySchedule struct {
	StartTime    time.Time     `json:"start_time"`
	MaxDuration  time.Duration `json:"max_duration"`
	Timezone     string        `json:"timezone"`
	NotifyBefore time.Duration `json:"notify_before"`
}

// SafetyRule defines conditions for automatic abort.
type SafetyRule struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Metric      string        `json:"metric"`
	Operator    string        `json:"operator"` // "gt", "lt", "eq"
	Threshold   float64       `json:"threshold"`
	Duration    time.Duration `json:"duration"` // How long condition must persist
	Action      SafetyAction  `json:"action"`
}

// SafetyAction defines what happens when a safety rule triggers.
type SafetyAction string

const (
	SafetyActionAbort   SafetyAction = "abort"
	SafetyActionPause   SafetyAction = "pause"
	SafetyActionNotify  SafetyAction = "notify"
	SafetyActionRollback SafetyAction = "rollback"
)

// GameDayStatus represents game day status.
type GameDayStatus string

const (
	GameDayStatusScheduled GameDayStatus = "scheduled"
	GameDayStatusRunning   GameDayStatus = "running"
	GameDayStatusPaused    GameDayStatus = "paused"
	GameDayStatusCompleted GameDayStatus = "completed"
	GameDayStatusAborted   GameDayStatus = "aborted"
)

// GameDayResults contains game day execution results.
type GameDayResults struct {
	TotalExperiments   int                   `json:"total_experiments"`
	CompletedExperiments int                 `json:"completed_experiments"`
	FailedExperiments  int                   `json:"failed_experiments"`
	SafetyRulesTriggered []string            `json:"safety_rules_triggered"`
	ExperimentResults  []ExperimentResult    `json:"experiment_results"`
	SteadyStateMetrics map[string]float64    `json:"steady_state_metrics"`
	RecoveryTime       time.Duration         `json:"recovery_time"`
	Observations       []Observation         `json:"observations"`
}

// ExperimentResult contains individual experiment results.
type ExperimentResult struct {
	ExperimentID string        `json:"experiment_id"`
	Status       Status        `json:"status"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	Duration     time.Duration `json:"duration"`
	ImpactMetrics map[string]float64 `json:"impact_metrics"`
	Errors       []string      `json:"errors,omitempty"`
}

// Observation is a recorded observation during the game day.
type Observation struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// GameDayRunner runs game days.
type GameDayRunner struct {
	engine       *Engine
	gameDays     map[string]*GameDay
	metricsFunc  func(string) float64
	rollbackFunc func(string) error
	mu           sync.RWMutex
}

// NewGameDayRunner creates a game day runner.
func NewGameDayRunner(engine *Engine) *GameDayRunner {
	return &GameDayRunner{
		engine:   engine,
		gameDays: make(map[string]*GameDay),
	}
}

// SetMetricsFunc sets the function to retrieve metrics.
func (r *GameDayRunner) SetMetricsFunc(f func(string) float64) {
	r.metricsFunc = f
}

// SetRollbackFunc sets the function to rollback changes.
func (r *GameDayRunner) SetRollbackFunc(f func(string) error) {
	r.rollbackFunc = f
}

// CreateGameDay creates a new game day.
func (r *GameDayRunner) CreateGameDay(gd *GameDay) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	gd.Status = GameDayStatusScheduled
	gd.CreatedAt = time.Now()
	gd.UpdatedAt = time.Now()

	r.gameDays[gd.ID] = gd
	return nil
}

// StartGameDay starts a game day.
func (r *GameDayRunner) StartGameDay(ctx context.Context, id string) error {
	r.mu.Lock()
	gd, ok := r.gameDays[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("game day not found: %s", id)
	}
	gd.Status = GameDayStatusRunning
	gd.StartTime = time.Now()
	gd.Results = &GameDayResults{
		TotalExperiments: len(gd.Experiments),
		ExperimentResults: make([]ExperimentResult, 0),
		SteadyStateMetrics: make(map[string]float64),
		Observations: make([]Observation, 0),
	}
	r.mu.Unlock()

	// Capture steady state
	r.captureSteadyState(gd)

	// Run experiments
	return r.runGameDay(ctx, gd)
}

func (r *GameDayRunner) captureSteadyState(gd *GameDay) {
	if r.metricsFunc == nil {
		return
	}

	// Capture baseline metrics
	metrics := []string{"error_rate", "latency_p99", "throughput", "cpu_usage", "memory_usage"}
	for _, metric := range metrics {
		gd.Results.SteadyStateMetrics[metric] = r.metricsFunc(metric)
	}
}

func (r *GameDayRunner) runGameDay(ctx context.Context, gd *GameDay) error {
	// Create context with timeout
	var cancel context.CancelFunc
	if gd.Schedule != nil && gd.Schedule.MaxDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, gd.Schedule.MaxDuration)
		defer cancel()
	}

	// Start safety monitor
	safetyCtx, safetyCancel := context.WithCancel(ctx)
	safetyCh := make(chan string)
	go r.monitorSafetyRules(safetyCtx, gd, safetyCh)

	// Group experiments by order
	orderGroups := make(map[int][]ExperimentSpec)
	for _, exp := range gd.Experiments {
		orderGroups[exp.Order] = append(orderGroups[exp.Order], exp)
	}

	// Run in order
	maxOrder := 0
	for order := range orderGroups {
		if order > maxOrder {
			maxOrder = order
		}
	}

	for order := 0; order <= maxOrder; order++ {
		exps, ok := orderGroups[order]
		if !ok {
			continue
		}

		// Check for abort
		select {
		case rule := <-safetyCh:
			safetyCancel()
			gd.Results.SafetyRulesTriggered = append(gd.Results.SafetyRulesTriggered, rule)
			r.abortGameDay(gd, fmt.Sprintf("Safety rule triggered: %s", rule))
			return nil
		case <-ctx.Done():
			safetyCancel()
			r.abortGameDay(gd, "Context cancelled")
			return ctx.Err()
		default:
		}

		// Run experiments at this order level
		var wg sync.WaitGroup
		for _, expSpec := range exps {
			if expSpec.StartDelay > 0 {
				time.Sleep(expSpec.StartDelay)
			}

			if expSpec.Parallel {
				wg.Add(1)
				go func(spec ExperimentSpec) {
					defer wg.Done()
					r.runExperiment(ctx, gd, spec)
				}(expSpec)
			} else {
				r.runExperiment(ctx, gd, expSpec)
			}
		}
		wg.Wait()
	}

	safetyCancel()

	// Calculate recovery time
	gd.Results.RecoveryTime = r.measureRecovery(gd)

	// Complete
	r.mu.Lock()
	gd.Status = GameDayStatusCompleted
	gd.EndTime = time.Now()
	r.mu.Unlock()

	return nil
}

func (r *GameDayRunner) runExperiment(ctx context.Context, gd *GameDay, spec ExperimentSpec) {
	startTime := time.Now()
	result := ExperimentResult{
		ExperimentID: spec.ExperimentID,
		StartTime:    startTime,
		ImpactMetrics: make(map[string]float64),
	}

	err := r.engine.StartExperiment(ctx, spec.ExperimentID)
	if err != nil {
		result.Status = StatusFailed
		result.Errors = append(result.Errors, err.Error())
	} else {
		// Wait for experiment to complete
		exp, _ := r.engine.GetExperiment(spec.ExperimentID)
		if exp != nil && exp.Config.Duration > 0 {
			select {
			case <-time.After(exp.Config.Duration):
			case <-ctx.Done():
			}
		}
		r.engine.StopExperiment(spec.ExperimentID)
		result.Status = StatusCompleted
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Capture impact metrics
	if r.metricsFunc != nil {
		result.ImpactMetrics["error_rate"] = r.metricsFunc("error_rate")
		result.ImpactMetrics["latency_p99"] = r.metricsFunc("latency_p99")
	}

	r.mu.Lock()
	gd.Results.ExperimentResults = append(gd.Results.ExperimentResults, result)
	if result.Status == StatusCompleted {
		gd.Results.CompletedExperiments++
	} else {
		gd.Results.FailedExperiments++
	}
	r.mu.Unlock()
}

func (r *GameDayRunner) monitorSafetyRules(ctx context.Context, gd *GameDay, ch chan string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ruleViolations := make(map[string]time.Time)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, rule := range gd.SafetyRules {
				if r.metricsFunc == nil {
					continue
				}

				value := r.metricsFunc(rule.Metric)
				violated := false

				switch rule.Operator {
				case "gt":
					violated = value > rule.Threshold
				case "lt":
					violated = value < rule.Threshold
				case "eq":
					violated = math.Abs(value-rule.Threshold) < 0.001
				}

				if violated {
					if startTime, ok := ruleViolations[rule.Name]; ok {
						if time.Since(startTime) >= rule.Duration {
							ch <- rule.Name
							return
						}
					} else {
						ruleViolations[rule.Name] = time.Now()
					}
				} else {
					delete(ruleViolations, rule.Name)
				}
			}
		}
	}
}

func (r *GameDayRunner) measureRecovery(gd *GameDay) time.Duration {
	if r.metricsFunc == nil {
		return 0
	}

	startTime := time.Now()
	maxWait := 10 * time.Minute
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Since(startTime) > maxWait {
				return maxWait
			}

			// Check if metrics returned to steady state
			recovered := true
			for metric, baseline := range gd.Results.SteadyStateMetrics {
				current := r.metricsFunc(metric)
				// Within 10% of baseline
				if math.Abs(current-baseline)/baseline > 0.1 {
					recovered = false
					break
				}
			}

			if recovered {
				return time.Since(startTime)
			}
		}
	}
}

func (r *GameDayRunner) abortGameDay(gd *GameDay, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	gd.Status = GameDayStatusAborted
	gd.EndTime = time.Now()
	gd.Results.Observations = append(gd.Results.Observations, Observation{
		Timestamp:   time.Now(),
		Type:        "abort",
		Description: reason,
		Severity:    "critical",
	})

	// Attempt rollback
	if r.rollbackFunc != nil {
		r.rollbackFunc(gd.ID)
	}
}

// GetGameDay retrieves a game day.
func (r *GameDayRunner) GetGameDay(id string) (*GameDay, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	gd, ok := r.gameDays[id]
	if !ok {
		return nil, fmt.Errorf("game day not found: %s", id)
	}
	return gd, nil
}

// ListGameDays returns all game days.
func (r *GameDayRunner) ListGameDays() []*GameDay {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*GameDay, 0, len(r.gameDays))
	for _, gd := range r.gameDays {
		result = append(result, gd)
	}
	return result
}

// Report generates a game day report.
func (r *GameDayRunner) Report(id string) (*GameDayReport, error) {
	gd, err := r.GetGameDay(id)
	if err != nil {
		return nil, err
	}

	if gd.Results == nil {
		return nil, fmt.Errorf("game day has no results")
	}

	report := &GameDayReport{
		GameDayID:    gd.ID,
		Name:         gd.Name,
		Status:       gd.Status,
		StartTime:    gd.StartTime,
		EndTime:      gd.EndTime,
		Duration:     gd.EndTime.Sub(gd.StartTime),
		Summary:      r.generateSummary(gd),
		Experiments:  gd.Results.ExperimentResults,
		SafetyEvents: gd.Results.SafetyRulesTriggered,
		RecoveryTime: gd.Results.RecoveryTime,
		Findings:     r.analyzeFindins(gd),
		Recommendations: r.generateRecommendations(gd),
		GeneratedAt:  time.Now(),
	}

	return report, nil
}

// GameDayReport is a detailed game day report.
type GameDayReport struct {
	GameDayID       string              `json:"game_day_id"`
	Name            string              `json:"name"`
	Status          GameDayStatus       `json:"status"`
	StartTime       time.Time           `json:"start_time"`
	EndTime         time.Time           `json:"end_time"`
	Duration        time.Duration       `json:"duration"`
	Summary         ReportSummary       `json:"summary"`
	Experiments     []ExperimentResult  `json:"experiments"`
	SafetyEvents    []string            `json:"safety_events"`
	RecoveryTime    time.Duration       `json:"recovery_time"`
	Findings        []Finding           `json:"findings"`
	Recommendations []Recommendation    `json:"recommendations"`
	GeneratedAt     time.Time           `json:"generated_at"`
}

// ReportSummary summarizes the game day.
type ReportSummary struct {
	TotalExperiments     int     `json:"total_experiments"`
	SuccessfulExperiments int    `json:"successful_experiments"`
	FailedExperiments    int     `json:"failed_experiments"`
	SuccessRate          float64 `json:"success_rate"`
	AverageRecoveryTime  time.Duration `json:"average_recovery_time"`
	ResilienceScore      float64 `json:"resilience_score"` // 0-100
}

// Finding is a discovered issue.
type Finding struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // "critical", "high", "medium", "low"
	Category    string `json:"category"` // "reliability", "performance", "recovery"
	Evidence    string `json:"evidence"`
}

// Recommendation is an improvement suggestion.
type Recommendation struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Effort      string `json:"effort"` // "low", "medium", "high"
	Impact      string `json:"impact"` // "low", "medium", "high"
}

func (r *GameDayRunner) generateSummary(gd *GameDay) ReportSummary {
	total := gd.Results.TotalExperiments
	successful := gd.Results.CompletedExperiments
	failed := gd.Results.FailedExperiments

	successRate := 0.0
	if total > 0 {
		successRate = float64(successful) / float64(total) * 100
	}

	// Calculate resilience score
	resilienceScore := r.calculateResilienceScore(gd)

	return ReportSummary{
		TotalExperiments:      total,
		SuccessfulExperiments: successful,
		FailedExperiments:     failed,
		SuccessRate:           successRate,
		AverageRecoveryTime:   gd.Results.RecoveryTime,
		ResilienceScore:       resilienceScore,
	}
}

func (r *GameDayRunner) calculateResilienceScore(gd *GameDay) float64 {
	score := 100.0

	// Deduct for failed experiments
	failRate := float64(gd.Results.FailedExperiments) / float64(gd.Results.TotalExperiments)
	score -= failRate * 30

	// Deduct for slow recovery
	if gd.Results.RecoveryTime > 5*time.Minute {
		score -= 20
	} else if gd.Results.RecoveryTime > 2*time.Minute {
		score -= 10
	}

	// Deduct for safety rule violations
	score -= float64(len(gd.Results.SafetyRulesTriggered)) * 15

	if score < 0 {
		score = 0
	}

	return score
}

func (r *GameDayRunner) analyzeFindins(gd *GameDay) []Finding {
	findings := make([]Finding, 0)

	// Check for slow recovery
	if gd.Results.RecoveryTime > 5*time.Minute {
		findings = append(findings, Finding{
			ID:          "slow-recovery",
			Title:       "Slow System Recovery",
			Description: fmt.Sprintf("System took %s to recover to steady state", gd.Results.RecoveryTime),
			Severity:    "high",
			Category:    "recovery",
			Evidence:    "Recovery time exceeded 5 minute threshold",
		})
	}

	// Check for cascading failures
	consecutiveFailures := 0
	for _, result := range gd.Results.ExperimentResults {
		if result.Status == StatusFailed {
			consecutiveFailures++
			if consecutiveFailures >= 3 {
				findings = append(findings, Finding{
					ID:          "cascading-failures",
					Title:       "Potential Cascading Failures",
					Description: "Multiple consecutive experiment failures detected",
					Severity:    "critical",
					Category:    "reliability",
					Evidence:    fmt.Sprintf("%d consecutive failures", consecutiveFailures),
				})
				break
			}
		} else {
			consecutiveFailures = 0
		}
	}

	// Check for safety rule triggers
	if len(gd.Results.SafetyRulesTriggered) > 0 {
		findings = append(findings, Finding{
			ID:          "safety-triggered",
			Title:       "Safety Rules Triggered",
			Description: "One or more safety rules were triggered during the game day",
			Severity:    "high",
			Category:    "reliability",
			Evidence:    fmt.Sprintf("Triggered rules: %v", gd.Results.SafetyRulesTriggered),
		})
	}

	return findings
}

func (r *GameDayRunner) generateRecommendations(gd *GameDay) []Recommendation {
	recs := make([]Recommendation, 0)

	// Based on findings
	if gd.Results.RecoveryTime > 5*time.Minute {
		recs = append(recs, Recommendation{
			Title:       "Improve Recovery Procedures",
			Description: "Implement automated recovery mechanisms to reduce recovery time",
			Priority:    "high",
			Effort:      "medium",
			Impact:      "high",
		})
	}

	if len(gd.Results.SafetyRulesTriggered) > 0 {
		recs = append(recs, Recommendation{
			Title:       "Add Circuit Breakers",
			Description: "Implement circuit breakers to prevent cascading failures",
			Priority:    "high",
			Effort:      "medium",
			Impact:      "high",
		})
	}

	// General recommendations
	recs = append(recs, Recommendation{
		Title:       "Regular Chaos Testing",
		Description: "Schedule monthly game days to continuously validate system resilience",
		Priority:    "medium",
		Effort:      "low",
		Impact:      "medium",
	})

	return recs
}

// ExportReport exports the report as JSON.
func (r *GameDayReport) ExportJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
