// Package cost provides cost optimization recommendations.
package cost

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Recommendation provides cost optimization recommendations.
type Recommendation struct {
	ID             string
	Type           RecommendationType
	Severity       RecommendationSeverity
	Title          string
	Description    string
	JobIDs         []string
	CurrentCost    float64
	ProjectedCost  float64
	Savings        float64
	SavingsPercent float64
	Implementation string
	Confidence     float64
	CreatedAt      time.Time
	ValidUntil     time.Time
	Applied        bool
	AppliedAt      *time.Time
}

// RecommendationType defines types of recommendations.
type RecommendationType string

const (
	RecommendationSpotInstance     RecommendationType = "spot_instance"
	RecommendationRightSizing      RecommendationType = "right_sizing"
	RecommendationScheduleShift    RecommendationType = "schedule_shift"
	RecommendationRegionChange     RecommendationType = "region_change"
	RecommendationBatchConsolidate RecommendationType = "batch_consolidate"
	RecommendationIdleResource     RecommendationType = "idle_resource"
)

// RecommendationSeverity defines severity levels.
type RecommendationSeverity string

const (
	SeverityLow      RecommendationSeverity = "low"
	SeverityMedium   RecommendationSeverity = "medium"
	SeverityHigh     RecommendationSeverity = "high"
	SeverityCritical RecommendationSeverity = "critical"
)

// RecommendationEngine generates cost optimization recommendations.
type RecommendationEngine struct {
	executionCosts []ExecutionCost
}

// NewRecommendationEngine creates a new recommendation engine.
func NewRecommendationEngine() *RecommendationEngine {
	return &RecommendationEngine{
		executionCosts: make([]ExecutionCost, 0),
	}
}

// AddCost adds an execution cost for analysis.
func (e *RecommendationEngine) AddCost(cost ExecutionCost) {
	e.executionCosts = append(e.executionCosts, cost)
}

// SetCosts sets all execution costs for analysis.
func (e *RecommendationEngine) SetCosts(costs []ExecutionCost) {
	e.executionCosts = costs
}

// GenerateRecommendations generates cost optimization recommendations.
func (e *RecommendationEngine) GenerateRecommendations(ctx context.Context) ([]Recommendation, error) {
	recommendations := make([]Recommendation, 0)

	// Get recent costs
	recentCosts := e.getRecentCosts(7 * 24 * time.Hour)
	if len(recentCosts) == 0 {
		return recommendations, nil
	}

	// Spot instance opportunities
	spotRecs := e.analyzeSpotOpportunities(recentCosts)
	recommendations = append(recommendations, spotRecs...)

	// Right-sizing opportunities
	sizingRecs := e.analyzeRightSizing(recentCosts)
	recommendations = append(recommendations, sizingRecs...)

	// Schedule optimization opportunities
	scheduleRecs := e.analyzeScheduleOptimization(recentCosts)
	recommendations = append(recommendations, scheduleRecs...)

	// Batch consolidation opportunities
	batchRecs := e.analyzeBatchConsolidation(recentCosts)
	recommendations = append(recommendations, batchRecs...)

	// Sort by savings
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Savings > recommendations[j].Savings
	})

	return recommendations, nil
}

func (e *RecommendationEngine) getRecentCosts(window time.Duration) []ExecutionCost {
	cutoff := time.Now().Add(-window)
	var recent []ExecutionCost
	for _, cost := range e.executionCosts {
		if cost.EndTime.After(cutoff) {
			recent = append(recent, cost)
		}
	}
	return recent
}

func (e *RecommendationEngine) analyzeSpotOpportunities(costs []ExecutionCost) []Recommendation {
	var recs []Recommendation

	// Group by job
	jobCosts := make(map[string][]ExecutionCost)
	for _, cost := range costs {
		jobCosts[cost.JobID] = append(jobCosts[cost.JobID], cost)
	}

	for jobID, jc := range jobCosts {
		// Check if already using spot
		spotUsage := 0
		for _, c := range jc {
			if c.SpotUsed {
				spotUsage++
			}
		}
		spotRatio := float64(spotUsage) / float64(len(jc))
		if spotRatio >= 0.8 {
			continue // Already using spot
		}

		// Calculate potential savings
		var totalOnDemand, totalSpot float64
		for _, c := range jc {
			totalOnDemand += c.OnDemandPrice * c.Duration.Hours()
			totalSpot += c.OnDemandPrice * c.Duration.Hours() * 0.3 // Assume 70% savings
		}

		savings := totalOnDemand - totalSpot
		if savings < 1.0 {
			continue // Not significant
		}

		recs = append(recs, Recommendation{
			ID:             uuid.New().String(),
			Type:           RecommendationSpotInstance,
			Severity:       getSeverity(savings),
			Title:          fmt.Sprintf("Enable Spot Instances for job %s", jobID),
			Description:    fmt.Sprintf("Job '%s' is not using spot instances. Based on %d recent executions, switching to spot could save approximately $%.2f/week.", jobID, len(jc), savings),
			JobIDs:         []string{jobID},
			CurrentCost:    totalOnDemand,
			ProjectedCost:  totalSpot,
			Savings:        savings,
			SavingsPercent: (savings / totalOnDemand) * 100,
			Implementation: "Update job configuration to enable spot instances and set appropriate retry policy",
			Confidence:     0.85,
			CreatedAt:      time.Now(),
			ValidUntil:     time.Now().Add(7 * 24 * time.Hour),
		})
	}

	return recs
}

func (e *RecommendationEngine) analyzeRightSizing(costs []ExecutionCost) []Recommendation {
	var recs []Recommendation

	// Group by job and instance type
	jobInstanceCosts := make(map[string]map[string][]ExecutionCost)
	for _, cost := range costs {
		if jobInstanceCosts[cost.JobID] == nil {
			jobInstanceCosts[cost.JobID] = make(map[string][]ExecutionCost)
		}
		jobInstanceCosts[cost.JobID][cost.InstanceType] = append(jobInstanceCosts[cost.JobID][cost.InstanceType], cost)
	}

	for jobID, instanceTypes := range jobInstanceCosts {
		for instanceType, jc := range instanceTypes {
			var totalDuration time.Duration
			var totalCost float64
			for _, c := range jc {
				totalDuration += c.Duration
				totalCost += c.TotalCost
			}
			avgDuration := totalDuration / time.Duration(len(jc))

			// If job completes very quickly, might be able to use smaller instance
			if avgDuration < 5*time.Minute && len(jc) >= 10 {
				smallerInstance := getSmallerInstance(instanceType)
				if smallerInstance == instanceType {
					continue
				}

				projectedSavings := totalCost * 0.4 // Assume 40% savings
				if projectedSavings < 1.0 {
					continue
				}

				recs = append(recs, Recommendation{
					ID:             uuid.New().String(),
					Type:           RecommendationRightSizing,
					Severity:       getSeverity(projectedSavings),
					Title:          fmt.Sprintf("Right-size instance for job %s", jobID),
					Description:    fmt.Sprintf("Job '%s' completes in avg %.1f minutes on %s. Consider using %s for cost savings.", jobID, avgDuration.Minutes(), instanceType, smallerInstance),
					JobIDs:         []string{jobID},
					CurrentCost:    totalCost,
					ProjectedCost:  totalCost * 0.6,
					Savings:        projectedSavings,
					SavingsPercent: 40.0,
					Implementation: fmt.Sprintf("Update job configuration to use instance type %s", smallerInstance),
					Confidence:     0.7,
					CreatedAt:      time.Now(),
					ValidUntil:     time.Now().Add(7 * 24 * time.Hour),
				})
			}
		}
	}

	return recs
}

func (e *RecommendationEngine) analyzeScheduleOptimization(costs []ExecutionCost) []Recommendation {
	var recs []Recommendation

	// Analyze execution times to find cheaper time slots
	hourCosts := make(map[int]float64)
	for _, cost := range costs {
		hour := cost.StartTime.Hour()
		hourCosts[hour] += cost.TotalCost
	}

	// Find peak hours
	var maxCost float64
	for _, cost := range hourCosts {
		if cost > maxCost {
			maxCost = cost
		}
	}

	// Find off-peak hours
	var offPeakHours []int
	for hour, cost := range hourCosts {
		if cost < maxCost*0.5 {
			offPeakHours = append(offPeakHours, hour)
		}
	}

	if len(offPeakHours) > 0 {
		// Collect jobs running during peak hours
		peakJobs := make(map[string]bool)
		for _, cost := range costs {
			hour := cost.StartTime.Hour()
			if hourCosts[hour] >= maxCost*0.8 {
				peakJobs[cost.JobID] = true
			}
		}

		if len(peakJobs) > 0 {
			jobList := make([]string, 0, len(peakJobs))
			for j := range peakJobs {
				jobList = append(jobList, j)
			}

			recs = append(recs, Recommendation{
				ID:             uuid.New().String(),
				Type:           RecommendationScheduleShift,
				Severity:       SeverityLow,
				Title:          "Shift jobs to off-peak hours",
				Description:    fmt.Sprintf("%d jobs run during peak hours. Consider shifting non-time-sensitive jobs to off-peak hours (%v) for potential savings.", len(jobList), offPeakHours),
				JobIDs:         jobList,
				Savings:        maxCost * 0.1,
				SavingsPercent: 10.0,
				Implementation: "Update job schedules to run during off-peak hours",
				Confidence:     0.6,
				CreatedAt:      time.Now(),
				ValidUntil:     time.Now().Add(7 * 24 * time.Hour),
			})
		}
	}

	return recs
}

func (e *RecommendationEngine) analyzeBatchConsolidation(costs []ExecutionCost) []Recommendation {
	var recs []Recommendation

	// Find jobs that run frequently with short durations
	jobFrequency := make(map[string]int)
	jobDuration := make(map[string]time.Duration)
	jobCost := make(map[string]float64)

	for _, cost := range costs {
		jobFrequency[cost.JobID]++
		jobDuration[cost.JobID] += cost.Duration
		jobCost[cost.JobID] += cost.TotalCost
	}

	for jobID, freq := range jobFrequency {
		if freq < 50 {
			continue // Not frequent enough
		}
		avgDuration := jobDuration[jobID] / time.Duration(freq)
		if avgDuration > 1*time.Minute {
			continue // Not short enough
		}

		totalCost := jobCost[jobID]
		projectedSavings := totalCost * 0.3 // Assume 30% savings

		if projectedSavings < 1.0 {
			continue
		}

		recs = append(recs, Recommendation{
			ID:             uuid.New().String(),
			Type:           RecommendationBatchConsolidate,
			Severity:       getSeverity(projectedSavings),
			Title:          fmt.Sprintf("Consolidate frequent job executions for %s", jobID),
			Description:    fmt.Sprintf("Job '%s' ran %d times with avg duration %.1fs. Consider batching to reduce execution overhead.", jobID, freq, avgDuration.Seconds()),
			JobIDs:         []string{jobID},
			CurrentCost:    totalCost,
			ProjectedCost:  totalCost * 0.7,
			Savings:        projectedSavings,
			SavingsPercent: 30.0,
			Implementation: "Modify job to batch process multiple items per execution",
			Confidence:     0.65,
			CreatedAt:      time.Now(),
			ValidUntil:     time.Now().Add(7 * 24 * time.Hour),
		})
	}

	return recs
}

func getSeverity(savings float64) RecommendationSeverity {
	switch {
	case savings >= 100:
		return SeverityCritical
	case savings >= 50:
		return SeverityHigh
	case savings >= 10:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

func getSmallerInstance(instanceType string) string {
	downsize := map[string]string{
		"t3.large":   "t3.medium",
		"t3.medium":  "t3.small",
		"t3.small":   "t3.micro",
		"m5.2xlarge": "m5.xlarge",
		"m5.xlarge":  "m5.large",
		"c5.xlarge":  "c5.large",
		"r5.xlarge":  "r5.large",
	}
	if smaller, ok := downsize[instanceType]; ok {
		return smaller
	}
	return instanceType
}
