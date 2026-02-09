// Package slo provides SLA-based alerting with SLO definitions, error budgets, and burn rate calculations.
package slo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrSLONotFound         = errors.New("SLO not found")
	ErrSLOAlreadyExists    = errors.New("SLO already exists")
	ErrInvalidSLO          = errors.New("invalid SLO configuration")
	ErrInvalidTarget       = errors.New("SLO target must be between 0 and 100")
	ErrInvalidWindow       = errors.New("SLO window must be positive")
	ErrBudgetExhausted     = errors.New("error budget exhausted")
)

// SLOType represents the type of SLO being measured.
type SLOType string

const (
	SLOTypeAvailability SLOType = "availability" // Success rate of executions
	SLOTypeLatency      SLOType = "latency"      // P99/P95/P50 latency targets
	SLOTypeThroughput   SLOType = "throughput"   // Executions per time period
)

// AlertSeverity represents alert severity levels.
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
	AlertSeverityPage     AlertSeverity = "page"
)

// SLO represents a Service Level Objective.
type SLO struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        SLOType           `json:"type"`
	Target      float64           `json:"target"` // Target percentage (e.g., 99.9)
	Window      time.Duration     `json:"window"` // Rolling window (e.g., 30 days)
	JobFilter   *JobFilter        `json:"job_filter,omitempty"`
	Thresholds  []AlertThreshold  `json:"thresholds"`
	Enabled     bool              `json:"enabled"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// JobFilter specifies which jobs this SLO applies to.
type JobFilter struct {
	JobIDs     []string          `json:"job_ids,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	Namespaces []string          `json:"namespaces,omitempty"`
	MatchAll   bool              `json:"match_all"` // If true, all filters must match
}

// AlertThreshold defines when to alert based on error budget consumption.
type AlertThreshold struct {
	BurnRate    float64       `json:"burn_rate"`    // How fast budget is being consumed
	Window      time.Duration `json:"window"`       // Time window for burn rate calculation
	Severity    AlertSeverity `json:"severity"`
	Consume     float64       `json:"consume"`      // Error budget % that triggers alert
}

// LatencyConfig holds latency-specific SLO configuration.
type LatencyConfig struct {
	Percentile    float64       `json:"percentile"`    // 50, 90, 95, 99, etc.
	ThresholdMs   int64         `json:"threshold_ms"`  // Target latency in milliseconds
}

// Validate validates the SLO configuration.
func (s *SLO) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidSLO)
	}
	if s.Target <= 0 || s.Target > 100 {
		return ErrInvalidTarget
	}
	if s.Window <= 0 {
		return ErrInvalidWindow
	}
	if s.Type == "" {
		return fmt.Errorf("%w: type is required", ErrInvalidSLO)
	}
	return nil
}

// ErrorBudget calculates the error budget for this SLO.
func (s *SLO) ErrorBudget() float64 {
	return 100.0 - s.Target
}

// SLOStatus represents the current status of an SLO.
type SLOStatus struct {
	SLOID              string        `json:"slo_id"`
	SLOName            string        `json:"slo_name"`
	Current            float64       `json:"current"`              // Current SLI value
	Target             float64       `json:"target"`               // Target value
	ErrorBudgetTotal   float64       `json:"error_budget_total"`   // Total error budget %
	ErrorBudgetUsed    float64       `json:"error_budget_used"`    // Used error budget %
	ErrorBudgetRemain  float64       `json:"error_budget_remaining"` // Remaining error budget %
	BurnRate           float64       `json:"burn_rate"`            // Current burn rate
	BurnRateWindow     time.Duration `json:"burn_rate_window"`
	TimeToExhaustion   time.Duration `json:"time_to_exhaustion"`   // Time until budget exhausted
	Status             string        `json:"status"`               // "ok", "warning", "critical", "exhausted"
	TotalEvents        int64         `json:"total_events"`
	GoodEvents         int64         `json:"good_events"`
	BadEvents          int64         `json:"bad_events"`
	WindowStart        time.Time     `json:"window_start"`
	WindowEnd          time.Time     `json:"window_end"`
	LastUpdated        time.Time     `json:"last_updated"`
}

// BurnRateAlert represents an alert triggered by burn rate.
type BurnRateAlert struct {
	ID            string        `json:"id"`
	SLOID         string        `json:"slo_id"`
	SLOName       string        `json:"slo_name"`
	Severity      AlertSeverity `json:"severity"`
	BurnRate      float64       `json:"burn_rate"`
	Threshold     float64       `json:"threshold"`
	Window        time.Duration `json:"window"`
	BudgetConsumed float64      `json:"budget_consumed"`
	Message       string        `json:"message"`
	TriggeredAt   time.Time     `json:"triggered_at"`
	ResolvedAt    *time.Time    `json:"resolved_at,omitempty"`
}

// ExecutionMetric represents a single execution metric for SLI calculation.
type ExecutionMetric struct {
	JobID       string        `json:"job_id"`
	ExecutionID string        `json:"execution_id"`
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration"`
	Timestamp   time.Time     `json:"timestamp"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// Manager manages SLOs and calculates SLI/error budgets.
type Manager struct {
	mu           sync.RWMutex
	slos         map[string]*SLO
	metrics      []ExecutionMetric
	status       map[string]*SLOStatus
	alerts       []*BurnRateAlert
	alertHandler AlertHandler
	metricsLimit int
}

// AlertHandler is called when an alert is triggered or resolved.
type AlertHandler func(ctx context.Context, alert *BurnRateAlert) error

// NewManager creates a new SLO manager.
func NewManager(alertHandler AlertHandler) *Manager {
	return &Manager{
		slos:         make(map[string]*SLO),
		metrics:      make([]ExecutionMetric, 0),
		status:       make(map[string]*SLOStatus),
		alerts:       make([]*BurnRateAlert, 0),
		alertHandler: alertHandler,
		metricsLimit: 100000, // Keep last 100k metrics
	}
}

// CreateSLO creates a new SLO.
func (m *Manager) CreateSLO(ctx context.Context, slo *SLO) error {
	if err := slo.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if slo.ID == "" {
		slo.ID = uuid.New().String()
	}

	if _, exists := m.slos[slo.ID]; exists {
		return ErrSLOAlreadyExists
	}

	now := time.Now().UTC()
	slo.CreatedAt = now
	slo.UpdatedAt = now

	m.slos[slo.ID] = slo
	return nil
}

// UpdateSLO updates an existing SLO.
func (m *Manager) UpdateSLO(ctx context.Context, slo *SLO) error {
	if err := slo.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.slos[slo.ID]; !exists {
		return ErrSLONotFound
	}

	slo.UpdatedAt = time.Now().UTC()
	m.slos[slo.ID] = slo
	return nil
}

// GetSLO retrieves an SLO by ID.
func (m *Manager) GetSLO(ctx context.Context, id string) (*SLO, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slo, exists := m.slos[id]
	if !exists {
		return nil, ErrSLONotFound
	}
	return slo, nil
}

// DeleteSLO deletes an SLO.
func (m *Manager) DeleteSLO(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.slos[id]; !exists {
		return ErrSLONotFound
	}

	delete(m.slos, id)
	delete(m.status, id)
	return nil
}

// ListSLOs returns all SLOs.
func (m *Manager) ListSLOs(ctx context.Context) []*SLO {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slos := make([]*SLO, 0, len(m.slos))
	for _, slo := range m.slos {
		slos = append(slos, slo)
	}
	return slos
}

// RecordMetric records an execution metric for SLI calculation.
func (m *Manager) RecordMetric(ctx context.Context, metric ExecutionMetric) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = append(m.metrics, metric)

	// Trim old metrics if over limit
	if len(m.metrics) > m.metricsLimit {
		m.metrics = m.metrics[len(m.metrics)-m.metricsLimit:]
	}

	return nil
}

// CalculateStatus calculates the current status of an SLO.
func (m *Manager) CalculateStatus(ctx context.Context, sloID string) (*SLOStatus, error) {
	m.mu.RLock()
	slo, exists := m.slos[sloID]
	metrics := m.metrics
	m.mu.RUnlock()

	if !exists {
		return nil, ErrSLONotFound
	}

	now := time.Now().UTC()
	windowStart := now.Add(-slo.Window)

	// Filter metrics within window and matching filter
	var totalEvents, goodEvents, badEvents int64
	var totalDuration time.Duration

	for _, metric := range metrics {
		if metric.Timestamp.Before(windowStart) {
			continue
		}

		if !m.matchesFilter(metric, slo.JobFilter) {
			continue
		}

		totalEvents++
		if metric.Success {
			goodEvents++
		} else {
			badEvents++
		}
		totalDuration += metric.Duration
	}

	// Calculate SLI based on type
	var current float64
	if totalEvents > 0 {
		switch slo.Type {
		case SLOTypeAvailability:
			current = (float64(goodEvents) / float64(totalEvents)) * 100
		case SLOTypeLatency:
			// For latency, we'd need percentile calculation
			avgLatency := totalDuration / time.Duration(totalEvents)
			current = float64(avgLatency.Milliseconds())
		case SLOTypeThroughput:
			current = float64(totalEvents)
		}
	}

	// Calculate error budget
	errorBudgetTotal := slo.ErrorBudget()
	var errorBudgetUsed float64
	if slo.Type == SLOTypeAvailability && totalEvents > 0 {
		failureRate := (float64(badEvents) / float64(totalEvents)) * 100
		errorBudgetUsed = (failureRate / errorBudgetTotal) * 100
		if errorBudgetUsed > 100 {
			errorBudgetUsed = 100
		}
	}
	errorBudgetRemain := 100 - errorBudgetUsed

	// Calculate burn rate (how fast we're consuming budget)
	burnRate := m.calculateBurnRate(slo, metrics, now)

	// Calculate time to exhaustion
	var timeToExhaustion time.Duration
	if burnRate > 0 && errorBudgetRemain > 0 {
		hoursRemaining := (errorBudgetRemain / 100) * float64(slo.Window.Hours()) / burnRate
		timeToExhaustion = time.Duration(hoursRemaining * float64(time.Hour))
	}

	// Determine status
	statusStr := "ok"
	if errorBudgetRemain <= 0 {
		statusStr = "exhausted"
	} else if burnRate > 14.4 { // 14.4x burn rate = budget exhausted in ~2 hours
		statusStr = "critical"
	} else if burnRate > 6 { // 6x burn rate = budget exhausted in ~5 hours
		statusStr = "warning"
	}

	status := &SLOStatus{
		SLOID:             sloID,
		SLOName:           slo.Name,
		Current:           current,
		Target:            slo.Target,
		ErrorBudgetTotal:  errorBudgetTotal,
		ErrorBudgetUsed:   errorBudgetUsed,
		ErrorBudgetRemain: errorBudgetRemain,
		BurnRate:          burnRate,
		BurnRateWindow:    time.Hour, // Default 1-hour window for burn rate
		TimeToExhaustion:  timeToExhaustion,
		Status:            statusStr,
		TotalEvents:       totalEvents,
		GoodEvents:        goodEvents,
		BadEvents:         badEvents,
		WindowStart:       windowStart,
		WindowEnd:         now,
		LastUpdated:       now,
	}

	// Store status
	m.mu.Lock()
	m.status[sloID] = status
	m.mu.Unlock()

	return status, nil
}

// calculateBurnRate calculates how fast the error budget is being consumed.
func (m *Manager) calculateBurnRate(slo *SLO, metrics []ExecutionMetric, now time.Time) float64 {
	// Use 1-hour window for burn rate calculation
	burnWindow := time.Hour
	windowStart := now.Add(-burnWindow)

	var totalEvents, badEvents int64
	for _, metric := range metrics {
		if metric.Timestamp.Before(windowStart) {
			continue
		}
		if !m.matchesFilter(metric, slo.JobFilter) {
			continue
		}
		totalEvents++
		if !metric.Success {
			badEvents++
		}
	}

	if totalEvents == 0 {
		return 0
	}

	// Burn rate = actual failure rate / allowed failure rate
	actualFailureRate := float64(badEvents) / float64(totalEvents)
	allowedFailureRate := (100 - slo.Target) / 100

	if allowedFailureRate == 0 {
		if actualFailureRate > 0 {
			return math.Inf(1)
		}
		return 0
	}

	return actualFailureRate / allowedFailureRate
}

// matchesFilter checks if a metric matches the SLO's job filter.
func (m *Manager) matchesFilter(metric ExecutionMetric, filter *JobFilter) bool {
	if filter == nil {
		return true // No filter means match all
	}

	// Check job IDs
	if len(filter.JobIDs) > 0 {
		found := false
		for _, id := range filter.JobIDs {
			if id == metric.JobID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tags
	if len(filter.Tags) > 0 {
		for k, v := range filter.Tags {
			if metric.Tags[k] != v {
				return false
			}
		}
	}

	return true
}

// CheckAlerts evaluates all SLOs and triggers alerts as needed.
func (m *Manager) CheckAlerts(ctx context.Context) ([]*BurnRateAlert, error) {
	m.mu.RLock()
	slos := make([]*SLO, 0, len(m.slos))
	for _, slo := range m.slos {
		if slo.Enabled {
			slos = append(slos, slo)
		}
	}
	m.mu.RUnlock()

	var alerts []*BurnRateAlert

	for _, slo := range slos {
		status, err := m.CalculateStatus(ctx, slo.ID)
		if err != nil {
			continue
		}

		for _, threshold := range slo.Thresholds {
			if status.BurnRate >= threshold.BurnRate {
				alert := &BurnRateAlert{
					ID:             uuid.New().String(),
					SLOID:          slo.ID,
					SLOName:        slo.Name,
					Severity:       threshold.Severity,
					BurnRate:       status.BurnRate,
					Threshold:      threshold.BurnRate,
					Window:         threshold.Window,
					BudgetConsumed: status.ErrorBudgetUsed,
					Message:        m.formatAlertMessage(slo, status, threshold),
					TriggeredAt:    time.Now().UTC(),
				}

				alerts = append(alerts, alert)

				// Call alert handler
				if m.alertHandler != nil {
					m.alertHandler(ctx, alert)
				}
			}
		}
	}

	// Store alerts
	m.mu.Lock()
	m.alerts = append(m.alerts, alerts...)
	// Keep only last 1000 alerts
	if len(m.alerts) > 1000 {
		m.alerts = m.alerts[len(m.alerts)-1000:]
	}
	m.mu.Unlock()

	return alerts, nil
}

// formatAlertMessage creates a human-readable alert message.
func (m *Manager) formatAlertMessage(slo *SLO, status *SLOStatus, threshold AlertThreshold) string {
	return fmt.Sprintf(
		"SLO '%s' is burning error budget at %.1fx rate (threshold: %.1fx). "+
			"%.1f%% of error budget consumed. At current rate, budget exhausts in %s.",
		slo.Name,
		status.BurnRate,
		threshold.BurnRate,
		status.ErrorBudgetUsed,
		status.TimeToExhaustion.Round(time.Minute),
	)
}

// GetStatus retrieves the cached status for an SLO.
func (m *Manager) GetStatus(ctx context.Context, sloID string) (*SLOStatus, error) {
	m.mu.RLock()
	status, exists := m.status[sloID]
	m.mu.RUnlock()

	if !exists {
		// Calculate if not cached
		return m.CalculateStatus(ctx, sloID)
	}
	return status, nil
}

// GetAllStatus returns status for all SLOs.
func (m *Manager) GetAllStatus(ctx context.Context) []*SLOStatus {
	m.mu.RLock()
	sloIDs := make([]string, 0, len(m.slos))
	for id := range m.slos {
		sloIDs = append(sloIDs, id)
	}
	m.mu.RUnlock()

	statuses := make([]*SLOStatus, 0, len(sloIDs))
	for _, id := range sloIDs {
		if status, err := m.CalculateStatus(ctx, id); err == nil {
			statuses = append(statuses, status)
		}
	}
	return statuses
}

// GetAlerts returns recent alerts.
func (m *Manager) GetAlerts(ctx context.Context, limit int) []*BurnRateAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}

	// Return most recent alerts
	start := len(m.alerts) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*BurnRateAlert, limit)
	copy(result, m.alerts[start:])
	return result
}

// GetErrorBudgetHistory returns error budget consumption over time.
func (m *Manager) GetErrorBudgetHistory(ctx context.Context, sloID string, points int) ([]ErrorBudgetPoint, error) {
	m.mu.RLock()
	slo, exists := m.slos[sloID]
	metrics := m.metrics
	m.mu.RUnlock()

	if !exists {
		return nil, ErrSLONotFound
	}

	if points <= 0 {
		points = 24 // Default to 24 points (hourly for a day)
	}

	now := time.Now().UTC()
	interval := slo.Window / time.Duration(points)
	history := make([]ErrorBudgetPoint, points)

	for i := 0; i < points; i++ {
		pointTime := now.Add(-time.Duration(points-i-1) * interval)
		windowStart := pointTime.Add(-slo.Window)

		var total, bad int64
		for _, metric := range metrics {
			if metric.Timestamp.After(windowStart) && metric.Timestamp.Before(pointTime) {
				if m.matchesFilter(metric, slo.JobFilter) {
					total++
					if !metric.Success {
						bad++
					}
				}
			}
		}

		var budgetUsed float64
		if total > 0 {
			failureRate := (float64(bad) / float64(total)) * 100
			budgetUsed = (failureRate / slo.ErrorBudget()) * 100
			if budgetUsed > 100 {
				budgetUsed = 100
			}
		}

		history[i] = ErrorBudgetPoint{
			Timestamp:         pointTime,
			BudgetUsed:        budgetUsed,
			BudgetRemaining:   100 - budgetUsed,
			TotalEvents:       total,
			FailedEvents:      bad,
		}
	}

	return history, nil
}

// ErrorBudgetPoint represents error budget at a point in time.
type ErrorBudgetPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	BudgetUsed      float64   `json:"budget_used"`
	BudgetRemaining float64   `json:"budget_remaining"`
	TotalEvents     int64     `json:"total_events"`
	FailedEvents    int64     `json:"failed_events"`
}

// CreateDefaultThresholds returns default alert thresholds for multi-window alerting.
func CreateDefaultThresholds() []AlertThreshold {
	return []AlertThreshold{
		{
			// Fast burn: 14.4x burn rate over 1 hour = 2% budget in 1 hour
			BurnRate: 14.4,
			Window:   time.Hour,
			Severity: AlertSeverityCritical,
			Consume:  2,
		},
		{
			// Fast burn: 6x burn rate over 6 hours = 5% budget
			BurnRate: 6,
			Window:   6 * time.Hour,
			Severity: AlertSeverityWarning,
			Consume:  5,
		},
		{
			// Slow burn: 3x burn rate over 1 day = 10% budget
			BurnRate: 3,
			Window:   24 * time.Hour,
			Severity: AlertSeverityWarning,
			Consume:  10,
		},
		{
			// Slow burn: 1x burn rate over 3 days = 10% budget
			BurnRate: 1,
			Window:   72 * time.Hour,
			Severity: AlertSeverityInfo,
			Consume:  10,
		},
	}
}
