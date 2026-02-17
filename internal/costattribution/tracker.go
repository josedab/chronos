// Package costattribution provides execution cost tracking and reporting.
package costattribution

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// CostConfig defines cost rates.
type CostConfig struct {
	// CostPerExecutionSecond is the cost per second of execution time (in USD cents).
	CostPerExecutionSecond float64 `json:"cost_per_execution_second" yaml:"cost_per_execution_second"`
	// CostPerWebhookCall is the fixed cost per webhook invocation (in USD cents).
	CostPerWebhookCall float64 `json:"cost_per_webhook_call" yaml:"cost_per_webhook_call"`
	// Currency label.
	Currency string `json:"currency" yaml:"currency"`
}

// DefaultCostConfig returns reasonable defaults.
func DefaultCostConfig() CostConfig {
	return CostConfig{
		CostPerExecutionSecond: 0.001, // $0.001 per second
		CostPerWebhookCall:     0.01,  // $0.01 per call
		Currency:               "USD",
	}
}

// ExecutionCostRecord tracks cost for a single execution.
type ExecutionCostRecord struct {
	ExecutionID string        `json:"execution_id"`
	JobID       string        `json:"job_id"`
	JobName     string        `json:"job_name"`
	Namespace   string        `json:"namespace"`
	Duration    time.Duration `json:"duration"`
	Attempts    int           `json:"attempts"`
	Cost        float64       `json:"cost"` // In configured currency
	Timestamp   time.Time     `json:"timestamp"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// CostReport summarizes costs over a period.
type CostReport struct {
	Period      string               `json:"period"`
	TotalCost   float64              `json:"total_cost"`
	Currency    string               `json:"currency"`
	Executions  int                  `json:"executions"`
	TotalTime   time.Duration        `json:"total_time"`
	ByNamespace map[string]float64   `json:"by_namespace"`
	ByJob       []JobCostSummary     `json:"by_job"`
	GeneratedAt time.Time            `json:"generated_at"`
}

// JobCostSummary shows cost per job.
type JobCostSummary struct {
	JobID      string  `json:"job_id"`
	JobName    string  `json:"job_name"`
	Namespace  string  `json:"namespace"`
	Cost       float64 `json:"cost"`
	Executions int     `json:"executions"`
}

// Budget defines a cost budget with alerting thresholds.
type Budget struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Namespace   string  `json:"namespace,omitempty"` // Empty = global
	MonthlyLimit float64 `json:"monthly_limit"`      // In currency units
	AlertAt     float64 `json:"alert_at"`            // Percentage (0-100) to trigger warning
}

// BudgetStatus shows current spend against budget.
type BudgetStatus struct {
	Budget      Budget    `json:"budget"`
	CurrentSpend float64  `json:"current_spend"`
	Percentage   float64  `json:"percentage"`
	IsOverBudget bool     `json:"is_over_budget"`
	IsWarning    bool     `json:"is_warning"`
	AsOf         time.Time `json:"as_of"`
}

// Tracker tracks execution costs and generates reports.
type Tracker struct {
	mu      sync.RWMutex
	config  CostConfig
	records []ExecutionCostRecord
	budgets map[string]*Budget
}

// NewTracker creates a new cost tracker.
func NewTracker(config CostConfig) *Tracker {
	return &Tracker{
		config:  config,
		records: make([]ExecutionCostRecord, 0),
		budgets: make(map[string]*Budget),
	}
}

// RecordExecution records the cost of an execution.
func (t *Tracker) RecordExecution(execID, jobID, jobName, namespace string, duration time.Duration, attempts int, tags map[string]string) ExecutionCostRecord {
	cost := (duration.Seconds() * t.config.CostPerExecutionSecond) +
		(float64(attempts) * t.config.CostPerWebhookCall)

	record := ExecutionCostRecord{
		ExecutionID: execID,
		JobID:       jobID,
		JobName:     jobName,
		Namespace:   namespace,
		Duration:    duration,
		Attempts:    attempts,
		Cost:        cost,
		Timestamp:   time.Now(),
		Tags:        tags,
	}

	t.mu.Lock()
	t.records = append(t.records, record)
	t.mu.Unlock()

	return record
}

// GenerateReport generates a cost report for a time period.
func (t *Tracker) GenerateReport(since time.Time) *CostReport {
	t.mu.RLock()
	defer t.mu.RUnlock()

	report := &CostReport{
		Period:      fmt.Sprintf("since %s", since.Format("2006-01-02")),
		Currency:    t.config.Currency,
		ByNamespace: make(map[string]float64),
		GeneratedAt: time.Now(),
	}

	jobCosts := make(map[string]*JobCostSummary)

	for _, r := range t.records {
		if r.Timestamp.Before(since) {
			continue
		}

		report.TotalCost += r.Cost
		report.Executions++
		report.TotalTime += r.Duration

		ns := r.Namespace
		if ns == "" {
			ns = "default"
		}
		report.ByNamespace[ns] += r.Cost

		key := r.JobID
		if jc, ok := jobCosts[key]; ok {
			jc.Cost += r.Cost
			jc.Executions++
		} else {
			jobCosts[key] = &JobCostSummary{
				JobID:      r.JobID,
				JobName:    r.JobName,
				Namespace:  r.Namespace,
				Cost:       r.Cost,
				Executions: 1,
			}
		}
	}

	for _, jc := range jobCosts {
		report.ByJob = append(report.ByJob, *jc)
	}
	sort.Slice(report.ByJob, func(i, j int) bool {
		return report.ByJob[i].Cost > report.ByJob[j].Cost
	})

	return report
}

// SetBudget sets a cost budget.
func (t *Tracker) SetBudget(budget *Budget) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.budgets[budget.ID] = budget
}

// GetBudgetStatus returns the current status of a budget.
func (t *Tracker) GetBudgetStatus(budgetID string) (*BudgetStatus, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	budget, ok := t.budgets[budgetID]
	if !ok {
		return nil, fmt.Errorf("budget not found: %s", budgetID)
	}

	// Calculate spend for current month
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var spend float64
	for _, r := range t.records {
		if r.Timestamp.Before(monthStart) {
			continue
		}
		if budget.Namespace != "" && r.Namespace != budget.Namespace {
			continue
		}
		spend += r.Cost
	}

	pct := 0.0
	if budget.MonthlyLimit > 0 {
		pct = (spend / budget.MonthlyLimit) * 100
	}

	return &BudgetStatus{
		Budget:       *budget,
		CurrentSpend: spend,
		Percentage:   pct,
		IsOverBudget: pct >= 100,
		IsWarning:    pct >= budget.AlertAt && pct < 100,
		AsOf:         now,
	}, nil
}

// GetTotalCost returns total cost since a given time.
func (t *Tracker) GetTotalCost(since time.Time) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var total float64
	for _, r := range t.records {
		if !r.Timestamp.Before(since) {
			total += r.Cost
		}
	}
	return total
}
