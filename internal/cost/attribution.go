// Package cost provides execution cost attribution and tracking.
package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Attribution tracks cost attribution for executions.
type Attribution struct {
	mu         sync.RWMutex
	costs      map[string]*ExecutionCost     // executionID -> cost
	jobCosts   map[string][]*ExecutionCost   // jobID -> costs
	attrBudgets map[string]*AttributionBudget // budgetID -> budget
	attrAlerts  []*AttributionAlert
	provider   PriceProvider
}

// AttributionBudget defines a cost budget for attribution monitoring.
type AttributionBudget struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Type            AttributionBudgetType `json:"type"`
	Amount          float64              `json:"amount"`
	Currency        string               `json:"currency"`
	Period          string               `json:"period"` // daily, weekly, monthly, yearly
	StartDate       time.Time            `json:"start_date"`
	Filters         *AttributionFilters  `json:"filters,omitempty"`
	Thresholds      []AttributionThreshold `json:"thresholds"`
	CurrentSpend    float64              `json:"current_spend"`
	ForecastedSpend float64              `json:"forecasted_spend"`
	LastUpdated     time.Time            `json:"last_updated"`
	Enabled         bool                 `json:"enabled"`
}

// AttributionBudgetType represents the type of budget.
type AttributionBudgetType string

const (
	AttributionBudgetTypeJob       AttributionBudgetType = "job"
	AttributionBudgetTypeNamespace AttributionBudgetType = "namespace"
	AttributionBudgetTypeTeam      AttributionBudgetType = "team"
	AttributionBudgetTypeGlobal    AttributionBudgetType = "global"
)

// AttributionFilters filters which costs count toward a budget.
type AttributionFilters struct {
	JobIDs      []string          `json:"job_ids,omitempty"`
	Namespaces  []string          `json:"namespaces,omitempty"`
	Teams       []string          `json:"teams,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Regions     []string          `json:"regions,omitempty"`
}

// AttributionThreshold defines when to trigger an alert.
type AttributionThreshold struct {
	Percent      float64    `json:"percent"` // 50, 80, 100, etc.
	Type         string     `json:"type"`    // actual, forecasted
	NotifyEmails []string   `json:"notify_emails,omitempty"`
	NotifyWebhook string    `json:"notify_webhook,omitempty"`
	Triggered    bool       `json:"triggered"`
	TriggeredAt  *time.Time `json:"triggered_at,omitempty"`
}

// AttributionAlert represents a triggered budget alert.
type AttributionAlert struct {
	ID               string    `json:"id"`
	BudgetID         string    `json:"budget_id"`
	BudgetName       string    `json:"budget_name"`
	ThresholdPercent float64   `json:"threshold_percent"`
	CurrentSpend     float64   `json:"current_spend"`
	BudgetAmount     float64   `json:"budget_amount"`
	Percent          float64   `json:"percent_used"`
	Message          string    `json:"message"`
	TriggeredAt      time.Time `json:"triggered_at"`
	Acknowledged     bool      `json:"acknowledged"`
}

// CostSummary provides aggregated cost information.
type CostSummary struct {
	Period          string           `json:"period"`
	TotalCost       float64          `json:"total_cost"`
	ComputeCost     float64          `json:"compute_cost"`
	StorageCost     float64          `json:"storage_cost"`
	NetworkCost     float64          `json:"network_cost"`
	ExecutionCount  int              `json:"execution_count"`
	AvgCostPerExec  float64          `json:"avg_cost_per_execution"`
	Breakdown       *CostBreakdown   `json:"breakdown,omitempty"`
	Trend           *CostTrend       `json:"trend,omitempty"`
}

// CostBreakdown provides detailed cost breakdown.
type CostBreakdown struct {
	ByJob         map[string]float64 `json:"by_job"`
	ByNamespace   map[string]float64 `json:"by_namespace"`
	ByTeam        map[string]float64 `json:"by_team"`
	ByRegion      map[string]float64 `json:"by_region"`
	ByInstance    map[string]float64 `json:"by_instance_type"`
	ByDay         map[string]float64 `json:"by_day"`
	ByHour        map[string]float64 `json:"by_hour"`
}

// CostTrend provides trend information.
type CostTrend struct {
	Direction       string  `json:"direction"` // "up", "down", "stable"
	PercentChange   float64 `json:"percent_change"`
	PreviousPeriod  float64 `json:"previous_period"`
	CurrentPeriod   float64 `json:"current_period"`
}

// ChargebackReport provides chargeback information per team/namespace.
type ChargebackReport struct {
	Period      string                 `json:"period"`
	StartDate   time.Time              `json:"start_date"`
	EndDate     time.Time              `json:"end_date"`
	TotalCost   float64                `json:"total_cost"`
	Entries     []*ChargebackEntry     `json:"entries"`
	GeneratedAt time.Time              `json:"generated_at"`
}

// ChargebackEntry represents a single chargeback entry.
type ChargebackEntry struct {
	Entity      string  `json:"entity"`       // Team or namespace
	EntityType  string  `json:"entity_type"`  // "team" or "namespace"
	TotalCost   float64 `json:"total_cost"`
	Percentage  float64 `json:"percentage"`
	Executions  int     `json:"executions"`
	TopJobs     []struct {
		JobID    string  `json:"job_id"`
		JobName  string  `json:"job_name"`
		Cost     float64 `json:"cost"`
	} `json:"top_jobs"`
}

// NewAttribution creates a new cost attribution tracker.
func NewAttribution(provider PriceProvider) *Attribution {
	return &Attribution{
		costs:       make(map[string]*ExecutionCost),
		jobCosts:    make(map[string][]*ExecutionCost),
		attrBudgets: make(map[string]*AttributionBudget),
		attrAlerts:  make([]*AttributionAlert, 0),
		provider:    provider,
	}
}

// RecordExecution records the cost of an execution.
func (a *Attribution) RecordExecution(ctx context.Context, cost *ExecutionCost) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if cost.ID == "" {
		cost.ID = uuid.New().String()
	}

	// Calculate total cost if not set
	if cost.TotalCost == 0 {
		cost.TotalCost = cost.ComputeCost + cost.StorageCost + cost.NetworkCost
	}

	// Calculate savings
	if cost.OnDemandPrice > 0 && cost.ActualPrice > 0 {
		cost.Savings = cost.OnDemandPrice - cost.ActualPrice
		cost.SavingsPercent = (cost.Savings / cost.OnDemandPrice) * 100
	}

	a.costs[cost.ExecutionID] = cost
	a.jobCosts[cost.JobID] = append(a.jobCosts[cost.JobID], cost)

	// Check budgets
	a.checkBudgets(cost)

	return nil
}

// GetExecutionCost retrieves the cost for an execution.
func (a *Attribution) GetExecutionCost(executionID string) (*ExecutionCost, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	cost, exists := a.costs[executionID]
	if !exists {
		return nil, fmt.Errorf("execution cost not found: %s", executionID)
	}
	return cost, nil
}

// GetJobCosts retrieves all costs for a job.
func (a *Attribution) GetJobCosts(jobID string, limit int) ([]*ExecutionCost, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	costs := a.jobCosts[jobID]
	if len(costs) == 0 {
		return []*ExecutionCost{}, nil
	}

	// Return most recent costs
	if limit > 0 && len(costs) > limit {
		return costs[len(costs)-limit:], nil
	}
	return costs, nil
}

// GetCostSummary returns a cost summary for a time period.
func (a *Attribution) GetCostSummary(ctx context.Context, from, to time.Time) (*CostSummary, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	summary := &CostSummary{
		Period: fmt.Sprintf("%s to %s", from.Format("2006-01-02"), to.Format("2006-01-02")),
		Breakdown: &CostBreakdown{
			ByJob:       make(map[string]float64),
			ByNamespace: make(map[string]float64),
			ByTeam:      make(map[string]float64),
			ByRegion:    make(map[string]float64),
			ByInstance:  make(map[string]float64),
			ByDay:       make(map[string]float64),
			ByHour:      make(map[string]float64),
		},
	}

	for _, cost := range a.costs {
		if cost.StartTime.Before(from) || cost.StartTime.After(to) {
			continue
		}

		summary.TotalCost += cost.TotalCost
		summary.ComputeCost += cost.ComputeCost
		summary.StorageCost += cost.StorageCost
		summary.NetworkCost += cost.NetworkCost
		summary.ExecutionCount++

		// Breakdowns
		summary.Breakdown.ByJob[cost.JobID] += cost.TotalCost
		if ns, ok := cost.Labels["namespace"]; ok {
			summary.Breakdown.ByNamespace[ns] += cost.TotalCost
		}
		if team, ok := cost.Labels["team"]; ok {
			summary.Breakdown.ByTeam[team] += cost.TotalCost
		}
		summary.Breakdown.ByRegion[cost.Region] += cost.TotalCost
		summary.Breakdown.ByInstance[cost.InstanceType] += cost.TotalCost

		day := cost.StartTime.Format("2006-01-02")
		summary.Breakdown.ByDay[day] += cost.TotalCost

		hour := cost.StartTime.Format("2006-01-02 15:00")
		summary.Breakdown.ByHour[hour] += cost.TotalCost
	}

	if summary.ExecutionCount > 0 {
		summary.AvgCostPerExec = summary.TotalCost / float64(summary.ExecutionCount)
	}

	return summary, nil
}

// GetJobCostReport returns a detailed cost report for a job.
func (a *Attribution) GetJobCostReport(ctx context.Context, jobID string, from, to time.Time) (*CostReport, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := &CostReport{
		From:               from,
		To:                 to,
		GeneratedAt:        time.Now().UTC(),
		CostByJob:          make(map[string]float64),
		CostByNamespace:    make(map[string]float64),
		CostByInstanceType: make(map[string]float64),
		DailyBreakdown:     make(map[string]float64),
	}

	costs := a.jobCosts[jobID]
	for _, cost := range costs {
		if cost.StartTime.Before(from) || cost.StartTime.After(to) {
			continue
		}

		report.TotalExecutions++
		if cost.SpotUsed {
			report.SpotExecutions++
		} else {
			report.OnDemandExecutions++
		}

		report.TotalCost += cost.TotalCost
		report.TotalComputeCost += cost.ComputeCost
		report.TotalStorageCost += cost.StorageCost
		report.TotalNetworkCost += cost.NetworkCost
		report.TotalSavings += cost.Savings

		report.CostByJob[cost.JobID] += cost.TotalCost
		report.CostByInstanceType[cost.InstanceType] += cost.TotalCost

		day := cost.StartTime.Format("2006-01-02")
		report.DailyBreakdown[day] += cost.TotalCost
	}

	if report.TotalCost > 0 && report.TotalSavings > 0 {
		expectedCost := report.TotalCost + report.TotalSavings
		report.SavingsPercent = (report.TotalSavings / expectedCost) * 100
	}

	return report, nil
}

// GenerateChargebackReport generates a chargeback report.
func (a *Attribution) GenerateChargebackReport(ctx context.Context, from, to time.Time, groupBy string) (*ChargebackReport, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := &ChargebackReport{
		Period:      fmt.Sprintf("%s to %s", from.Format("2006-01-02"), to.Format("2006-01-02")),
		StartDate:   from,
		EndDate:     to,
		GeneratedAt: time.Now().UTC(),
		Entries:     make([]*ChargebackEntry, 0),
	}

	// Group costs by entity
	entityCosts := make(map[string]*ChargebackEntry)
	entityJobCosts := make(map[string]map[string]float64) // entity -> job -> cost

	for _, cost := range a.costs {
		if cost.StartTime.Before(from) || cost.StartTime.After(to) {
			continue
		}

		var entity string
		var entityType string

		switch groupBy {
		case "team":
			entity = cost.Labels["team"]
			entityType = "team"
		case "namespace":
			entity = cost.Labels["namespace"]
			entityType = "namespace"
		default:
			entity = cost.Labels["namespace"]
			entityType = "namespace"
		}

		if entity == "" {
			entity = "unassigned"
		}

		if _, exists := entityCosts[entity]; !exists {
			entityCosts[entity] = &ChargebackEntry{
				Entity:     entity,
				EntityType: entityType,
			}
			entityJobCosts[entity] = make(map[string]float64)
		}

		entityCosts[entity].TotalCost += cost.TotalCost
		entityCosts[entity].Executions++
		entityJobCosts[entity][cost.JobID] += cost.TotalCost
		report.TotalCost += cost.TotalCost
	}

	// Calculate percentages and top jobs
	for entity, entry := range entityCosts {
		if report.TotalCost > 0 {
			entry.Percentage = (entry.TotalCost / report.TotalCost) * 100
		}

		// Get top jobs
		jobCosts := entityJobCosts[entity]
		type jobCost struct {
			jobID string
			cost  float64
		}
		jobs := make([]jobCost, 0, len(jobCosts))
		for jid, c := range jobCosts {
			jobs = append(jobs, jobCost{jid, c})
		}
		sort.Slice(jobs, func(i, j int) bool {
			return jobs[i].cost > jobs[j].cost
		})

		// Top 5 jobs
		for i := 0; i < 5 && i < len(jobs); i++ {
			entry.TopJobs = append(entry.TopJobs, struct {
				JobID   string  `json:"job_id"`
				JobName string  `json:"job_name"`
				Cost    float64 `json:"cost"`
			}{
				JobID: jobs[i].jobID,
				Cost:  jobs[i].cost,
			})
		}

		report.Entries = append(report.Entries, entry)
	}

	// Sort entries by cost
	sort.Slice(report.Entries, func(i, j int) bool {
		return report.Entries[i].TotalCost > report.Entries[j].TotalCost
	})

	return report, nil
}

// Budget management

// CreateAttrBudget creates a new attribution budget.
func (a *Attribution) CreateAttrBudget(budget *AttributionBudget) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if budget.ID == "" {
		budget.ID = uuid.New().String()
	}
	budget.LastUpdated = time.Now().UTC()

	a.attrBudgets[budget.ID] = budget
	return nil
}

// GetAttrBudget retrieves an attribution budget by ID.
func (a *Attribution) GetAttrBudget(budgetID string) (*AttributionBudget, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	budget, exists := a.attrBudgets[budgetID]
	if !exists {
		return nil, fmt.Errorf("budget not found: %s", budgetID)
	}
	return budget, nil
}

// ListAttrBudgets lists all attribution budgets.
func (a *Attribution) ListAttrBudgets() []*AttributionBudget {
	a.mu.RLock()
	defer a.mu.RUnlock()

	budgets := make([]*AttributionBudget, 0, len(a.attrBudgets))
	for _, b := range a.attrBudgets {
		budgets = append(budgets, b)
	}
	return budgets
}

// UpdateAttrBudget updates an attribution budget.
func (a *Attribution) UpdateAttrBudget(budget *AttributionBudget) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.attrBudgets[budget.ID]; !exists {
		return fmt.Errorf("budget not found: %s", budget.ID)
	}

	budget.LastUpdated = time.Now().UTC()
	a.attrBudgets[budget.ID] = budget
	return nil
}

// DeleteAttrBudget deletes an attribution budget.
func (a *Attribution) DeleteAttrBudget(budgetID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.attrBudgets, budgetID)
	return nil
}

// GetAttrAlerts returns all attribution budget alerts.
func (a *Attribution) GetAttrAlerts(unacknowledgedOnly bool) []*AttributionAlert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !unacknowledgedOnly {
		return a.attrAlerts
	}

	result := make([]*AttributionAlert, 0)
	for _, alert := range a.attrAlerts {
		if !alert.Acknowledged {
			result = append(result, alert)
		}
	}
	return result
}

// AcknowledgeAttrAlert acknowledges an attribution alert.
func (a *Attribution) AcknowledgeAttrAlert(alertID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, alert := range a.attrAlerts {
		if alert.ID == alertID {
			alert.Acknowledged = true
			return nil
		}
	}
	return fmt.Errorf("alert not found: %s", alertID)
}

func (a *Attribution) checkBudgets(cost *ExecutionCost) {
	for _, budget := range a.attrBudgets {
		if !budget.Enabled {
			continue
		}

		if !a.costMatchesBudget(cost, budget) {
			continue
		}

		// Update current spend
		budget.CurrentSpend += cost.TotalCost
		budget.LastUpdated = time.Now().UTC()

		// Check thresholds
		percentUsed := (budget.CurrentSpend / budget.Amount) * 100

		for i, threshold := range budget.Thresholds {
			if threshold.Triggered {
				continue
			}

			if percentUsed >= threshold.Percent {
				now := time.Now().UTC()
				budget.Thresholds[i].Triggered = true
				budget.Thresholds[i].TriggeredAt = &now

				alert := &AttributionAlert{
					ID:               uuid.New().String(),
					BudgetID:         budget.ID,
					BudgetName:       budget.Name,
					ThresholdPercent: threshold.Percent,
					CurrentSpend:     budget.CurrentSpend,
					BudgetAmount:     budget.Amount,
					Percent:          percentUsed,
					Message: fmt.Sprintf("Budget '%s' has reached %.0f%% of limit ($%.2f of $%.2f)",
						budget.Name, percentUsed, budget.CurrentSpend, budget.Amount),
					TriggeredAt: now,
				}

				a.attrAlerts = append(a.attrAlerts, alert)
			}
		}
	}
}

func (a *Attribution) costMatchesBudget(cost *ExecutionCost, budget *AttributionBudget) bool {
	if budget.Filters == nil {
		return true
	}

	filters := budget.Filters

	// Check job IDs
	if len(filters.JobIDs) > 0 {
		found := false
		for _, jid := range filters.JobIDs {
			if jid == cost.JobID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check namespaces
	if len(filters.Namespaces) > 0 {
		ns := cost.Labels["namespace"]
		found := false
		for _, n := range filters.Namespaces {
			if n == ns {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check teams
	if len(filters.Teams) > 0 {
		team := cost.Labels["team"]
		found := false
		for _, t := range filters.Teams {
			if t == team {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check regions
	if len(filters.Regions) > 0 {
		found := false
		for _, r := range filters.Regions {
			if r == cost.Region {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check labels
	if len(filters.Labels) > 0 {
		for k, v := range filters.Labels {
			if cost.Labels[k] != v {
				return false
			}
		}
	}

	return true
}

// PredictCost predicts the cost of a job execution.
func (a *Attribution) PredictCost(ctx context.Context, req *CostPredictionRequest) (*CostPrediction, error) {
	prediction := &CostPrediction{
		JobID:             req.JobID,
		InstanceType:      req.InstanceType,
		Region:            req.Region,
		EstimatedDuration: req.EstimatedDuration,
	}

	// Get pricing from provider
	if a.provider != nil {
		onDemandPrice, err := a.provider.GetOnDemandPrice(ctx, req.InstanceType, req.Region)
		if err == nil {
			hours := req.EstimatedDuration.Hours()
			prediction.OnDemandCost = onDemandPrice * hours
		}

		spotPrice, err := a.provider.GetSpotPrice(ctx, req.InstanceType, req.Region)
		if err == nil && spotPrice != nil {
			hours := req.EstimatedDuration.Hours()
			prediction.SpotCost = spotPrice.Price * hours
			prediction.SpotAvailable = true
			prediction.SpotSavings = prediction.OnDemandCost - prediction.SpotCost
			if prediction.OnDemandCost > 0 {
				prediction.SpotSavingsPercent = (prediction.SpotSavings / prediction.OnDemandCost) * 100
			}
			prediction.InterruptionRisk = spotPrice.InterruptionRate

			// Recommend spot if savings > 50% and interruption risk < 10%
			prediction.RecommendSpot = prediction.SpotSavingsPercent > 50 && spotPrice.InterruptionRate < 0.1
		}
	}

	// Calculate historical averages
	a.mu.RLock()
	costs := a.jobCosts[req.JobID]
	a.mu.RUnlock()

	if len(costs) > 0 {
		var totalCost float64
		sortedCosts := make([]float64, len(costs))
		for i, c := range costs {
			totalCost += c.TotalCost
			sortedCosts[i] = c.TotalCost
		}
		prediction.HistoricalAvgCost = totalCost / float64(len(costs))

		sort.Float64s(sortedCosts)
		p95Index := int(float64(len(sortedCosts)) * 0.95)
		if p95Index >= len(sortedCosts) {
			p95Index = len(sortedCosts) - 1
		}
		prediction.HistoricalP95Cost = sortedCosts[p95Index]

		prediction.Confidence = minFloat(float64(len(costs))/100.0, 1.0)
	}

	// Set total estimated cost
	if prediction.RecommendSpot && prediction.SpotAvailable {
		prediction.TotalEstimatedCost = prediction.SpotCost
	} else {
		prediction.TotalEstimatedCost = prediction.OnDemandCost
	}

	return prediction, nil
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Export exports cost data as JSON.
func (a *Attribution) Export(ctx context.Context, from, to time.Time) ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	costs := make([]*ExecutionCost, 0)
	for _, cost := range a.costs {
		if cost.StartTime.Before(from) || cost.StartTime.After(to) {
			continue
		}
		costs = append(costs, cost)
	}

	return json.MarshalIndent(costs, "", "  ")
}
