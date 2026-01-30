// Package cost provides budget management for execution costs.
package cost

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Budget defines a cost budget for jobs.
type Budget struct {
	ID             string
	Name           string
	Namespace      string
	LabelSelector  map[string]string
	Amount         float64
	Period         BudgetPeriod
	CurrentSpend   float64
	AlertThreshold float64
	Alerts         []BudgetAlert
	CreatedAt      time.Time
	ResetAt        time.Time
}

// BudgetPeriod defines the budget reset period.
type BudgetPeriod string

const (
	BudgetPeriodDaily   BudgetPeriod = "daily"
	BudgetPeriodWeekly  BudgetPeriod = "weekly"
	BudgetPeriodMonthly BudgetPeriod = "monthly"
)

// BudgetAlert represents a budget alert.
type BudgetAlert struct {
	Timestamp    time.Time
	Threshold    float64
	CurrentSpend float64
	Message      string
	Notified     bool
}

// BudgetManager manages cost budgets.
type BudgetManager struct {
	budgets              map[string]*Budget
	defaultAlertThreshold float64
}

// NewBudgetManager creates a new budget manager.
func NewBudgetManager(defaultAlertThreshold float64) *BudgetManager {
	if defaultAlertThreshold <= 0 {
		defaultAlertThreshold = 0.8 // 80% default
	}
	return &BudgetManager{
		budgets:              make(map[string]*Budget),
		defaultAlertThreshold: defaultAlertThreshold,
	}
}

// CreateBudget creates a new cost budget.
func (m *BudgetManager) CreateBudget(budget *Budget) error {
	budget.ID = uuid.New().String()
	budget.CreatedAt = time.Now()
	budget.ResetAt = m.calculateResetTime(budget.Period)

	if budget.AlertThreshold == 0 {
		budget.AlertThreshold = m.defaultAlertThreshold
	}

	m.budgets[budget.ID] = budget
	return nil
}

// GetBudget retrieves a budget by ID.
func (m *BudgetManager) GetBudget(id string) (*Budget, error) {
	budget, ok := m.budgets[id]
	if !ok {
		return nil, fmt.Errorf("budget not found: %s", id)
	}
	return budget, nil
}

// ListBudgets lists all budgets.
func (m *BudgetManager) ListBudgets() []*Budget {
	budgets := make([]*Budget, 0, len(m.budgets))
	for _, b := range m.budgets {
		budgets = append(budgets, b)
	}
	return budgets
}

// DeleteBudget removes a budget.
func (m *BudgetManager) DeleteBudget(id string) error {
	if _, ok := m.budgets[id]; !ok {
		return fmt.Errorf("budget not found: %s", id)
	}
	delete(m.budgets, id)
	return nil
}

// RecordCost records cost against matching budgets.
func (m *BudgetManager) RecordCost(cost *ExecutionCost) []BudgetAlert {
	var alerts []BudgetAlert
	
	for _, budget := range m.budgets {
		if !m.matchesBudget(cost, budget) {
			continue
		}

		budget.CurrentSpend += cost.TotalCost

		// Check alert threshold
		spendRatio := budget.CurrentSpend / budget.Amount
		if spendRatio >= budget.AlertThreshold && !m.hasRecentAlert(budget) {
			alert := BudgetAlert{
				Timestamp:    time.Now(),
				Threshold:    budget.AlertThreshold,
				CurrentSpend: budget.CurrentSpend,
				Message:      fmt.Sprintf("Budget '%s' has reached %.1f%% of limit", budget.Name, spendRatio*100),
			}
			budget.Alerts = append(budget.Alerts, alert)
			alerts = append(alerts, alert)
		}
	}
	
	return alerts
}

// ResetExpiredBudgets resets budgets that have passed their reset time.
func (m *BudgetManager) ResetExpiredBudgets() int {
	count := 0
	now := time.Now()
	
	for _, budget := range m.budgets {
		if now.After(budget.ResetAt) {
			budget.CurrentSpend = 0
			budget.ResetAt = m.calculateResetTime(budget.Period)
			count++
		}
	}
	
	return count
}

func (m *BudgetManager) matchesBudget(cost *ExecutionCost, budget *Budget) bool {
	if budget.Namespace != "" && budget.Namespace != cost.Labels["namespace"] {
		return false
	}
	for k, v := range budget.LabelSelector {
		if cost.Labels[k] != v {
			return false
		}
	}
	return true
}

func (m *BudgetManager) hasRecentAlert(budget *Budget) bool {
	if len(budget.Alerts) == 0 {
		return false
	}
	lastAlert := budget.Alerts[len(budget.Alerts)-1]
	return time.Since(lastAlert.Timestamp) < 1*time.Hour
}

func (m *BudgetManager) calculateResetTime(period BudgetPeriod) time.Time {
	now := time.Now()
	switch period {
	case BudgetPeriodDaily:
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	case BudgetPeriodWeekly:
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		return time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 0, 0, 0, 0, now.Location())
	case BudgetPeriodMonthly:
		return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	default:
		return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	}
}
