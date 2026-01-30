// Package smartsched provides intelligent scheduling optimization for Chronos.
package smartsched

import (
	"math"
	"sort"
	"sync"
	"time"
)

// Optimizer provides intelligent scheduling recommendations.
type Optimizer struct {
	mu           sync.RWMutex
	history      map[string]*JobHistory
	windowSize   int
	config       Config
}

// Config configures the optimizer.
type Config struct {
	// WindowSize is the number of executions to analyze.
	WindowSize int `json:"window_size" yaml:"window_size"`
	// MinSamples is the minimum samples required for predictions.
	MinSamples int `json:"min_samples" yaml:"min_samples"`
	// PeakThreshold is the load threshold to avoid (0.0-1.0).
	PeakThreshold float64 `json:"peak_threshold" yaml:"peak_threshold"`
	// FailureThreshold is the failure rate to trigger alerts (0.0-1.0).
	FailureThreshold float64 `json:"failure_threshold" yaml:"failure_threshold"`
}

// DefaultConfig returns default optimizer configuration.
func DefaultConfig() Config {
	return Config{
		WindowSize:       100,
		MinSamples:       10,
		PeakThreshold:    0.8,
		FailureThreshold: 0.3,
	}
}

// JobHistory tracks execution history for a job.
type JobHistory struct {
	JobID          string
	Executions     []ExecutionRecord
	AvgDuration    time.Duration
	P95Duration    time.Duration
	P99Duration    time.Duration
	SuccessRate    float64
	FailureRate    float64
	PeakHours      []int
	BestTimeSlots  []TimeSlot
}

// ExecutionRecord is a single execution record.
type ExecutionRecord struct {
	Timestamp time.Time
	Duration  time.Duration
	Status    string
	Hour      int
	DayOfWeek int
}

// TimeSlot represents an optimal time slot.
type TimeSlot struct {
	Hour        int     `json:"hour"`
	DayOfWeek   int     `json:"day_of_week,omitempty"` // 0=Sunday
	Score       float64 `json:"score"`
	AvgDuration time.Duration `json:"avg_duration"`
	SuccessRate float64 `json:"success_rate"`
}

// Recommendation contains scheduling recommendations.
type Recommendation struct {
	JobID             string      `json:"job_id"`
	OptimalTimeSlots  []TimeSlot  `json:"optimal_time_slots"`
	AvoidTimeSlots    []TimeSlot  `json:"avoid_time_slots"`
	SuggestedSchedule string      `json:"suggested_schedule,omitempty"`
	RetryAdjustment   *RetryAdjustment `json:"retry_adjustment,omitempty"`
	Insights          []string    `json:"insights"`
	Confidence        float64     `json:"confidence"` // 0.0-1.0
}

// RetryAdjustment suggests retry policy changes.
type RetryAdjustment struct {
	MaxAttempts     int           `json:"max_attempts"`
	InitialInterval time.Duration `json:"initial_interval"`
	Reason          string        `json:"reason"`
}

// LoadPattern represents system load patterns.
type LoadPattern struct {
	HourlyLoad     [24]float64 // Average load by hour
	DailyLoad      [7]float64  // Average load by day of week
	CurrentLoad    float64
	PeakHours      []int
	LowLoadHours   []int
}

// NewOptimizer creates a new scheduler optimizer.
func NewOptimizer(cfg Config) *Optimizer {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 100
	}
	if cfg.MinSamples == 0 {
		cfg.MinSamples = 10
	}

	return &Optimizer{
		history:    make(map[string]*JobHistory),
		windowSize: cfg.WindowSize,
		config:     cfg,
	}
}

// RecordExecution records a job execution for analysis.
func (o *Optimizer) RecordExecution(jobID string, timestamp time.Time, duration time.Duration, status string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	history, exists := o.history[jobID]
	if !exists {
		history = &JobHistory{
			JobID:      jobID,
			Executions: make([]ExecutionRecord, 0, o.windowSize),
		}
		o.history[jobID] = history
	}

	record := ExecutionRecord{
		Timestamp: timestamp,
		Duration:  duration,
		Status:    status,
		Hour:      timestamp.Hour(),
		DayOfWeek: int(timestamp.Weekday()),
	}

	history.Executions = append(history.Executions, record)

	// Keep only last N executions
	if len(history.Executions) > o.windowSize {
		history.Executions = history.Executions[len(history.Executions)-o.windowSize:]
	}

	// Update statistics
	o.updateStats(history)
}

// updateStats recalculates job statistics.
func (o *Optimizer) updateStats(h *JobHistory) {
	if len(h.Executions) == 0 {
		return
	}

	// Calculate durations
	durations := make([]time.Duration, len(h.Executions))
	successCount := 0

	for i, exec := range h.Executions {
		durations[i] = exec.Duration
		if exec.Status == "success" {
			successCount++
		}
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	// Average duration
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	h.AvgDuration = total / time.Duration(len(durations))

	// P95 and P99
	p95Idx := int(float64(len(durations)) * 0.95)
	p99Idx := int(float64(len(durations)) * 0.99)
	if p95Idx >= len(durations) {
		p95Idx = len(durations) - 1
	}
	if p99Idx >= len(durations) {
		p99Idx = len(durations) - 1
	}
	h.P95Duration = durations[p95Idx]
	h.P99Duration = durations[p99Idx]

	// Success rate
	h.SuccessRate = float64(successCount) / float64(len(h.Executions))
	h.FailureRate = 1.0 - h.SuccessRate

	// Analyze time patterns
	h.BestTimeSlots = o.findBestTimeSlots(h.Executions)
}

// findBestTimeSlots analyzes execution history to find optimal times.
func (o *Optimizer) findBestTimeSlots(executions []ExecutionRecord) []TimeSlot {
	// Group by hour
	hourStats := make(map[int]*hourlyStats)

	for _, exec := range executions {
		stats, exists := hourStats[exec.Hour]
		if !exists {
			stats = &hourlyStats{}
			hourStats[exec.Hour] = stats
		}

		stats.count++
		stats.totalDuration += exec.Duration
		if exec.Status == "success" {
			stats.successCount++
		}
	}

	// Calculate scores
	var slots []TimeSlot
	for hour, stats := range hourStats {
		if stats.count < 2 {
			continue
		}

		avgDuration := stats.totalDuration / time.Duration(stats.count)
		successRate := float64(stats.successCount) / float64(stats.count)

		// Score: higher success rate + lower duration = better
		// Normalize duration (assuming most jobs take < 1 hour)
		durationScore := 1.0 - math.Min(float64(avgDuration)/float64(time.Hour), 1.0)
		score := (successRate * 0.7) + (durationScore * 0.3)

		slots = append(slots, TimeSlot{
			Hour:        hour,
			Score:       score,
			AvgDuration: avgDuration,
			SuccessRate: successRate,
		})
	}

	// Sort by score descending
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Score > slots[j].Score
	})

	// Return top 3
	if len(slots) > 3 {
		slots = slots[:3]
	}

	return slots
}

type hourlyStats struct {
	count         int
	successCount  int
	totalDuration time.Duration
}

// GetRecommendation returns scheduling recommendations for a job.
func (o *Optimizer) GetRecommendation(jobID string) (*Recommendation, error) {
	o.mu.RLock()
	history, exists := o.history[jobID]
	o.mu.RUnlock()

	rec := &Recommendation{
		JobID:    jobID,
		Insights: []string{},
	}

	if !exists || len(history.Executions) < o.config.MinSamples {
		rec.Confidence = 0.0
		rec.Insights = append(rec.Insights, "Insufficient data for recommendations")
		return rec, nil
	}

	rec.OptimalTimeSlots = history.BestTimeSlots
	rec.Confidence = math.Min(float64(len(history.Executions))/float64(o.config.WindowSize), 1.0)

	// Generate insights
	if history.FailureRate > o.config.FailureThreshold {
		rec.Insights = append(rec.Insights,
			"High failure rate detected - consider reviewing retry policy")
		rec.RetryAdjustment = &RetryAdjustment{
			MaxAttempts:     5,
			InitialInterval: 5 * time.Second,
			Reason:          "High failure rate warrants more retries",
		}
	}

	if history.P99Duration > history.AvgDuration*3 {
		rec.Insights = append(rec.Insights,
			"High P99 latency detected - execution time varies significantly")
	}

	if len(history.BestTimeSlots) > 0 {
		best := history.BestTimeSlots[0]
		rec.Insights = append(rec.Insights,
			"Best performance at hour "+formatHour(best.Hour)+
				" with "+formatPercent(best.SuccessRate)+" success rate")

		// Suggest schedule
		rec.SuggestedSchedule = generateCronForHour(best.Hour)
	}

	return rec, nil
}

// GetJobHistory returns history for a job.
func (o *Optimizer) GetJobHistory(jobID string) (*JobHistory, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	history, exists := o.history[jobID]
	return history, exists
}

// GetLoadPattern returns current system load patterns.
func (o *Optimizer) GetLoadPattern() *LoadPattern {
	o.mu.RLock()
	defer o.mu.RUnlock()

	pattern := &LoadPattern{}
	hourCounts := [24]int{}
	dayCounts := [7]int{}

	for _, history := range o.history {
		for _, exec := range history.Executions {
			hourCounts[exec.Hour]++
			dayCounts[exec.DayOfWeek]++
		}
	}

	// Find max for normalization
	maxHour := 1
	maxDay := 1
	for i := 0; i < 24; i++ {
		if hourCounts[i] > maxHour {
			maxHour = hourCounts[i]
		}
	}
	for i := 0; i < 7; i++ {
		if dayCounts[i] > maxDay {
			maxDay = dayCounts[i]
		}
	}

	// Normalize to 0-1
	for i := 0; i < 24; i++ {
		pattern.HourlyLoad[i] = float64(hourCounts[i]) / float64(maxHour)
		if pattern.HourlyLoad[i] > 0.8 {
			pattern.PeakHours = append(pattern.PeakHours, i)
		} else if pattern.HourlyLoad[i] < 0.2 {
			pattern.LowLoadHours = append(pattern.LowLoadHours, i)
		}
	}

	for i := 0; i < 7; i++ {
		pattern.DailyLoad[i] = float64(dayCounts[i]) / float64(maxDay)
	}

	return pattern
}

func formatHour(hour int) string {
	if hour == 0 {
		return "12:00 AM"
	} else if hour < 12 {
		return string(rune('0'+hour/10)) + string(rune('0'+hour%10)) + ":00 AM"
	} else if hour == 12 {
		return "12:00 PM"
	} else {
		h := hour - 12
		return string(rune('0'+h/10)) + string(rune('0'+h%10)) + ":00 PM"
	}
}

func formatPercent(f float64) string {
	pct := int(f * 100)
	return string(rune('0'+pct/10)) + string(rune('0'+pct%10)) + "%"
}

func generateCronForHour(hour int) string {
	return "0 " + string(rune('0'+hour/10)) + string(rune('0'+hour%10)) + " * * *"
}
