// Package cost provides execution cost optimization for the Chronos distributed cron system.
// It integrates with cloud provider spot pricing APIs and provides cost prediction,
// optimization recommendations, and budget tracking.
package cost

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CostOptimizer provides cost optimization capabilities for job execution.
type CostOptimizer struct {
	mu              sync.RWMutex
	config          *OptimizerConfig
	priceProviders  map[string]PriceProvider
	executionCosts  []ExecutionCost
	budgetManager   *BudgetManager
	recEngine       *RecommendationEngine
	costModels      map[string]*CostModel
}

// OptimizerConfig configures the cost optimizer.
type OptimizerConfig struct {
	// DefaultProvider is the default cloud provider
	DefaultProvider string
	// DefaultRegion is the default region
	DefaultRegion string
	// SpotEnabled enables spot/preemptible instances
	SpotEnabled bool
	// SpotMaxPrice is the maximum spot price as a percentage of on-demand
	SpotMaxPrice float64
	// BudgetAlertThreshold is the percentage at which to alert for budget
	BudgetAlertThreshold float64
	// CostTrackingEnabled enables detailed cost tracking
	CostTrackingEnabled bool
	// OptimizationInterval is how often to run optimization
	OptimizationInterval time.Duration
}

// DefaultOptimizerConfig returns sensible defaults.
func DefaultOptimizerConfig() *OptimizerConfig {
	return &OptimizerConfig{
		DefaultProvider:      "aws",
		DefaultRegion:        "us-east-1",
		SpotEnabled:          true,
		SpotMaxPrice:         0.8, // 80% of on-demand
		BudgetAlertThreshold: 0.8,
		CostTrackingEnabled:  true,
		OptimizationInterval: 1 * time.Hour,
	}
}

// NewCostOptimizer creates a new cost optimizer.
func NewCostOptimizer(config *OptimizerConfig) *CostOptimizer {
	if config == nil {
		config = DefaultOptimizerConfig()
	}

	return &CostOptimizer{
		config:          config,
		priceProviders:  make(map[string]PriceProvider),
		executionCosts:  make([]ExecutionCost, 0),
		budgetManager:   NewBudgetManager(config.BudgetAlertThreshold),
		recEngine:       NewRecommendationEngine(),
		costModels:      make(map[string]*CostModel),
	}
}

// RegisterPriceProvider registers a cloud price provider.
func (o *CostOptimizer) RegisterPriceProvider(name string, provider PriceProvider) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.priceProviders[name] = provider
}

// PredictCost predicts the cost of running a job.
func (o *CostOptimizer) PredictCost(ctx context.Context, req *CostPredictionRequest) (*CostPrediction, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	provider, ok := o.priceProviders[req.Provider]
	if !ok {
		return o.simulateCostPrediction(ctx, req)
	}

	// Get on-demand price
	onDemandPrice, err := provider.GetOnDemandPrice(ctx, req.InstanceType, req.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to get on-demand price: %w", err)
	}

	prediction := &CostPrediction{
		JobID:             req.JobID,
		InstanceType:      req.InstanceType,
		Region:            req.Region,
		EstimatedDuration: req.EstimatedDuration,
		OnDemandCost:      onDemandPrice * req.EstimatedDuration.Hours(),
	}

	// Get spot price if enabled
	if o.config.SpotEnabled {
		spotPrice, err := provider.GetSpotPrice(ctx, req.InstanceType, req.Region)
		if err == nil && spotPrice.Price < onDemandPrice*o.config.SpotMaxPrice {
			prediction.SpotCost = spotPrice.Price * req.EstimatedDuration.Hours()
			prediction.SpotAvailable = true
			prediction.SpotSavings = prediction.OnDemandCost - prediction.SpotCost
			prediction.SpotSavingsPercent = prediction.SpotSavings / prediction.OnDemandCost * 100
			prediction.InterruptionRisk = spotPrice.InterruptionRate
			prediction.RecommendSpot = spotPrice.InterruptionRate < 0.1 && req.Retryable
		}
	}

	// Check cost model for historical data
	if model, ok := o.costModels[req.JobID]; ok {
		prediction.HistoricalAvgCost = model.AvgCost
		prediction.HistoricalP95Cost = model.P95Cost
		prediction.Confidence = model.Confidence
	}

	prediction.RecommendedInstanceType = o.recommendInstanceType(req)
	prediction.TotalEstimatedCost = prediction.OnDemandCost
	if prediction.RecommendSpot {
		prediction.TotalEstimatedCost = prediction.SpotCost
	}

	return prediction, nil
}

func (o *CostOptimizer) simulateCostPrediction(ctx context.Context, req *CostPredictionRequest) (*CostPrediction, error) {
	basePrices := map[string]float64{
		"t3.micro": 0.0104, "t3.small": 0.0208, "t3.medium": 0.0416, "t3.large": 0.0832,
		"m5.large": 0.096, "m5.xlarge": 0.192, "m5.2xlarge": 0.384,
		"c5.large": 0.085, "c5.xlarge": 0.17, "r5.large": 0.126,
	}

	onDemandPrice, ok := basePrices[req.InstanceType]
	if !ok {
		onDemandPrice = 0.1
	}

	// Adjust by region
	regionMultiplier := map[string]float64{
		"us-east-1": 1.0, "us-west-2": 1.02, "eu-west-1": 1.05, "ap-southeast-1": 1.08,
	}
	if mult, ok := regionMultiplier[req.Region]; ok {
		onDemandPrice *= mult
	}

	hours := req.EstimatedDuration.Hours()
	onDemandCost := onDemandPrice * hours
	spotCost := onDemandPrice * 0.3 * hours

	return &CostPrediction{
		JobID:                   req.JobID,
		InstanceType:            req.InstanceType,
		Region:                  req.Region,
		EstimatedDuration:       req.EstimatedDuration,
		OnDemandCost:            onDemandCost,
		SpotCost:                spotCost,
		SpotAvailable:           true,
		SpotSavings:             onDemandCost - spotCost,
		SpotSavingsPercent:      70.0,
		InterruptionRisk:        0.05,
		RecommendSpot:           req.Retryable,
		TotalEstimatedCost:      spotCost,
		RecommendedInstanceType: req.InstanceType,
		Confidence:              0.85,
	}, nil
}

func (o *CostOptimizer) recommendInstanceType(req *CostPredictionRequest) string {
	if req.CPUCores <= 1 && req.MemoryGB <= 2 {
		return "t3.small"
	} else if req.CPUCores <= 2 && req.MemoryGB <= 4 {
		return "t3.medium"
	} else if req.CPUCores <= 2 && req.MemoryGB <= 8 {
		return "t3.large"
	} else if req.CPUCores <= 4 && req.MemoryGB <= 16 {
		return "m5.xlarge"
	}
	return req.InstanceType
}

// RecordExecutionCost records the cost of a completed execution.
func (o *CostOptimizer) RecordExecutionCost(cost *ExecutionCost) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	cost.ID = uuid.New().String()

	// Calculate savings
	if cost.SpotUsed && cost.OnDemandPrice > 0 {
		cost.Savings = (cost.OnDemandPrice - cost.ActualPrice) * cost.Duration.Hours()
		cost.SavingsPercent = cost.Savings / (cost.OnDemandPrice * cost.Duration.Hours()) * 100
	}

	// Calculate total cost
	cost.TotalCost = cost.ComputeCost + cost.StorageCost + cost.NetworkCost

	o.executionCosts = append(o.executionCosts, *cost)

	// Update cost model
	o.updateCostModel(cost)

	// Check budgets
	o.budgetManager.RecordCost(cost)

	// Update recommendation engine
	o.recEngine.AddCost(*cost)

	return nil
}

func (o *CostOptimizer) updateCostModel(cost *ExecutionCost) {
	key := cost.JobID + ":" + cost.InstanceType
	model, ok := o.costModels[key]
	if !ok {
		model = &CostModel{
			JobID:        cost.JobID,
			InstanceType: cost.InstanceType,
		}
		o.costModels[key] = model
	}

	// Update running averages
	model.SampleCount++
	n := float64(model.SampleCount)
	model.AvgDuration = time.Duration(float64(model.AvgDuration)*(n-1)/n + float64(cost.Duration)/n)
	model.AvgCost = model.AvgCost*(n-1)/n + cost.TotalCost/n
	model.LastUpdated = time.Now()

	// Update confidence based on sample count
	model.Confidence = math.Min(0.95, float64(model.SampleCount)/100.0)
}

// Budget management delegated to BudgetManager

// CreateBudget creates a new cost budget.
func (o *CostOptimizer) CreateBudget(budget *Budget) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.budgetManager.CreateBudget(budget)
}

// GetBudget retrieves a budget by ID.
func (o *CostOptimizer) GetBudget(id string) (*Budget, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.budgetManager.GetBudget(id)
}

// ListBudgets lists all budgets.
func (o *CostOptimizer) ListBudgets() []*Budget {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.budgetManager.ListBudgets()
}

// GenerateRecommendations generates cost optimization recommendations.
func (o *CostOptimizer) GenerateRecommendations(ctx context.Context) ([]Recommendation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.recEngine.SetCosts(o.executionCosts)
	return o.recEngine.GenerateRecommendations(ctx)
}

// GetCostReport generates a cost report for the specified time range.
func (o *CostOptimizer) GetCostReport(ctx context.Context, from, to time.Time) (*CostReport, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	report := &CostReport{
		From:               from,
		To:                 to,
		GeneratedAt:        time.Now(),
		CostByJob:          make(map[string]float64),
		CostByNamespace:    make(map[string]float64),
		CostByInstanceType: make(map[string]float64),
		DailyBreakdown:     make(map[string]float64),
	}

	// Filter costs in range
	var costs []ExecutionCost
	for _, cost := range o.executionCosts {
		if cost.EndTime.After(from) && cost.EndTime.Before(to) {
			costs = append(costs, cost)
		}
	}

	if len(costs) == 0 {
		return report, nil
	}

	// Calculate totals
	for _, cost := range costs {
		report.TotalCost += cost.TotalCost
		report.TotalComputeCost += cost.ComputeCost
		report.TotalStorageCost += cost.StorageCost
		report.TotalNetworkCost += cost.NetworkCost
		report.TotalSavings += cost.Savings
		if cost.SpotUsed {
			report.SpotExecutions++
		} else {
			report.OnDemandExecutions++
		}

		// Cost by job
		report.CostByJob[cost.JobID] += cost.TotalCost

		// Cost by namespace
		ns := cost.Labels["namespace"]
		if ns == "" {
			ns = "default"
		}
		report.CostByNamespace[ns] += cost.TotalCost

		// Cost by instance type
		report.CostByInstanceType[cost.InstanceType] += cost.TotalCost

		// Daily breakdown
		day := cost.EndTime.Format("2006-01-02")
		report.DailyBreakdown[day] += cost.TotalCost
	}

	report.TotalExecutions = len(costs)
	if report.TotalCost > 0 {
		report.SavingsPercent = report.TotalSavings / (report.TotalCost + report.TotalSavings) * 100
	}

	return report, nil
}

// GetCostModels returns all cost models for jobs.
func (o *CostOptimizer) GetCostModels() map[string]*CostModel {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	models := make(map[string]*CostModel, len(o.costModels))
	for k, v := range o.costModels {
		models[k] = v
	}
	return models
}

// GetRecentCosts returns execution costs from the specified time window.
func (o *CostOptimizer) GetRecentCosts(window time.Duration) []ExecutionCost {
	o.mu.RLock()
	defer o.mu.RUnlock()
	
	cutoff := time.Now().Add(-window)
	var recent []ExecutionCost
	for _, cost := range o.executionCosts {
		if cost.EndTime.After(cutoff) {
			recent = append(recent, cost)
		}
	}
	return recent
}
