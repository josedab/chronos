// Package cost provides cost-related type definitions.
package cost

import (
	"context"
	"time"
)

// PriceProvider provides pricing information from cloud providers.
type PriceProvider interface {
	// GetOnDemandPrice returns the on-demand price for an instance type
	GetOnDemandPrice(ctx context.Context, instanceType, region string) (float64, error)
	// GetSpotPrice returns the current spot price for an instance type
	GetSpotPrice(ctx context.Context, instanceType, region string) (*SpotPrice, error)
	// GetSpotPriceHistory returns historical spot prices
	GetSpotPriceHistory(ctx context.Context, instanceType, region string, since time.Time) ([]SpotPrice, error)
	// GetAvailabilityZones returns available zones for a region
	GetAvailabilityZones(ctx context.Context, region string) ([]string, error)
}

// SpotPrice represents a spot instance price.
type SpotPrice struct {
	InstanceType     string
	Region           string
	AvailabilityZone string
	Price            float64
	Timestamp        time.Time
	InterruptionRate float64 // Historical interruption rate
}

// ExecutionCost tracks the cost of a job execution.
type ExecutionCost struct {
	ID               string
	JobID            string
	ExecutionID      string
	StartTime        time.Time
	EndTime          time.Time
	Duration         time.Duration
	InstanceType     string
	Region           string
	AvailabilityZone string
	SpotUsed         bool
	OnDemandPrice    float64
	ActualPrice      float64
	Savings          float64
	SavingsPercent   float64
	ComputeCost      float64
	StorageCost      float64
	NetworkCost      float64
	TotalCost        float64
	Labels           map[string]string
}

// CostPredictionRequest is a request to predict execution cost.
type CostPredictionRequest struct {
	JobID             string
	InstanceType      string
	Region            string
	EstimatedDuration time.Duration
	CPUCores          int
	MemoryGB          int
	StorageGB         int
	NetworkGB         int
	Retryable         bool
	Provider          string
}

// CostPrediction is the predicted cost of an execution.
type CostPrediction struct {
	JobID                   string
	InstanceType            string
	Region                  string
	EstimatedDuration       time.Duration
	OnDemandCost            float64
	SpotCost                float64
	SpotAvailable           bool
	SpotSavings             float64
	SpotSavingsPercent      float64
	InterruptionRisk        float64
	RecommendSpot           bool
	TotalEstimatedCost      float64
	HistoricalAvgCost       float64
	HistoricalP95Cost       float64
	RecommendedInstanceType string
	Confidence              float64
}

// CostModel predicts execution costs.
type CostModel struct {
	JobID        string
	InstanceType string
	AvgDuration  time.Duration
	AvgCost      float64
	P50Duration  time.Duration
	P95Duration  time.Duration
	P99Duration  time.Duration
	P50Cost      float64
	P95Cost      float64
	P99Cost      float64
	SampleCount  int
	LastUpdated  time.Time
	Confidence   float64
}

// CostReport provides a summary of execution costs.
type CostReport struct {
	From               time.Time
	To                 time.Time
	GeneratedAt        time.Time
	TotalExecutions    int
	SpotExecutions     int
	OnDemandExecutions int
	TotalCost          float64
	TotalComputeCost   float64
	TotalStorageCost   float64
	TotalNetworkCost   float64
	TotalSavings       float64
	SavingsPercent     float64
	CostByJob          map[string]float64
	CostByNamespace    map[string]float64
	CostByInstanceType map[string]float64
	DailyBreakdown     map[string]float64
}
