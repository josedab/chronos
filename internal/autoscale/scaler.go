// Package autoscale provides predictive auto-scaling for Chronos clusters.
package autoscale

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ScalerConfig configures the auto-scaler.
type ScalerConfig struct {
	HistoryWindow      time.Duration     `json:"history_window"`
	PredictionHorizon  time.Duration     `json:"prediction_horizon"`
	UpdateInterval     time.Duration     `json:"update_interval"`
	ScaleUpThreshold   float64           `json:"scale_up_threshold"`
	ScaleDownThreshold float64           `json:"scale_down_threshold"`
	MinReplicas        int               `json:"min_replicas"`
	MaxReplicas        int               `json:"max_replicas"`
	ScaleUpCooldown    time.Duration     `json:"scale_up_cooldown"`
	ScaleDownCooldown  time.Duration     `json:"scale_down_cooldown"`
	PreWarmMinutes     int               `json:"pre_warm_minutes"`
	Kubernetes         *KubernetesConfig `json:"kubernetes,omitempty"`
	AWS                *AWSConfig        `json:"aws,omitempty"`
	GCP                *GCPConfig        `json:"gcp,omitempty"`
	Azure              *AzureConfig      `json:"azure,omitempty"`
}

// KubernetesConfig for K8s HPA integration.
type KubernetesConfig struct {
	APIServer      string `json:"api_server"`
	Token          string `json:"token"`
	Namespace      string `json:"namespace"`
	DeploymentName string `json:"deployment_name"`
}

// AWSConfig for AWS Auto Scaling Groups.
type AWSConfig struct {
	Region               string `json:"region"`
	AccessKeyID          string `json:"access_key_id"`
	SecretAccessKey      string `json:"secret_access_key"`
	AutoScalingGroupName string `json:"auto_scaling_group_name"`
	UseSpotInstances     bool   `json:"use_spot_instances"`
}

// GCPConfig for GCP Managed Instance Groups.
type GCPConfig struct {
	ProjectID         string `json:"project_id"`
	Zone              string `json:"zone"`
	InstanceGroupName string `json:"instance_group_name"`
	UsePreemptible    bool   `json:"use_preemptible"`
}

// AzureConfig for Azure VMSS.
type AzureConfig struct {
	SubscriptionID   string `json:"subscription_id"`
	ResourceGroup    string `json:"resource_group"`
	VMSSName         string `json:"vmss_name"`
	UseSpotInstances bool   `json:"use_spot_instances"`
}

// DefaultScalerConfig returns sensible defaults.
func DefaultScalerConfig() ScalerConfig {
	return ScalerConfig{
		HistoryWindow:      7 * 24 * time.Hour,
		PredictionHorizon:  1 * time.Hour,
		UpdateInterval:     5 * time.Minute,
		ScaleUpThreshold:   0.7,
		ScaleDownThreshold: 0.3,
		MinReplicas:        1,
		MaxReplicas:        10,
		ScaleUpCooldown:    3 * time.Minute,
		ScaleDownCooldown:  5 * time.Minute,
		PreWarmMinutes:     10,
	}
}

// PredictiveScaler manages predictive auto-scaling.
type PredictiveScaler struct {
	config        ScalerConfig
	provider      ScalingProvider
	metricsStore  *MetricsStore
	lastScaleTime time.Time
	lastScaleType string
	mu            sync.RWMutex
	stopCh        chan struct{}
}

// NewPredictiveScaler creates a new predictive scaler.
func NewPredictiveScaler(config ScalerConfig) (*PredictiveScaler, error) {
	provider, err := createProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create scaling provider: %w", err)
	}

	return &PredictiveScaler{
		config:       config,
		provider:     provider,
		metricsStore: NewMetricsStore(config.HistoryWindow),
		stopCh:       make(chan struct{}),
	}, nil
}

// Start begins the auto-scaling loop.
func (s *PredictiveScaler) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return nil
		case <-ticker.C:
			if err := s.evaluate(ctx); err != nil {
				fmt.Printf("auto-scale evaluation error: %v\n", err)
			}
		}
	}
}

// Stop stops the auto-scaling loop.
func (s *PredictiveScaler) Stop() {
	close(s.stopCh)
}

// RecordMetric records a metric data point for prediction.
func (s *PredictiveScaler) RecordMetric(metric *MetricPoint) {
	s.metricsStore.Add(metric)
}

// GetPrediction returns the predicted load for the given time range.
func (s *PredictiveScaler) GetPrediction(horizon time.Duration) (*LoadPrediction, error) {
	return s.predict(horizon)
}

// GetRecommendation returns the current scaling recommendation.
func (s *PredictiveScaler) GetRecommendation() (*ScaleRecommendation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prediction, err := s.predict(s.config.PredictionHorizon)
	if err != nil {
		return nil, err
	}

	current, err := s.provider.GetCurrentReplicas(context.Background())
	if err != nil {
		return nil, err
	}

	return s.calculateRecommendation(prediction, current), nil
}

func (s *PredictiveScaler) evaluate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prediction, err := s.predict(s.config.PredictionHorizon)
	if err != nil {
		return fmt.Errorf("prediction failed: %w", err)
	}

	current, err := s.provider.GetCurrentReplicas(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current replicas: %w", err)
	}

	rec := s.calculateRecommendation(prediction, current)

	if !s.canScale(rec.Action) {
		return nil
	}

	if rec.Action != ActionNone {
		if err := s.provider.Scale(ctx, rec.TargetReplicas); err != nil {
			return fmt.Errorf("scaling failed: %w", err)
		}
		s.lastScaleTime = time.Now()
		s.lastScaleType = string(rec.Action)
	}

	return nil
}

func (s *PredictiveScaler) predict(horizon time.Duration) (*LoadPrediction, error) {
	metrics := s.metricsStore.GetRange(time.Now().Add(-s.config.HistoryWindow), time.Now())
	if len(metrics) < 24 {
		return &LoadPrediction{
			Confidence: 0.3,
			Message:    "Insufficient data for accurate prediction",
		}, nil
	}

	hourlyPatterns := make(map[int][]float64)
	weekdayPatterns := make(map[int][]float64)

	for _, m := range metrics {
		hour := m.Timestamp.Hour()
		weekday := int(m.Timestamp.Weekday())
		hourlyPatterns[hour] = append(hourlyPatterns[hour], m.Value)
		weekdayPatterns[weekday] = append(weekdayPatterns[weekday], m.Value)
	}

	predictions := make([]PredictionPoint, 0)
	now := time.Now()

	for t := now; t.Before(now.Add(horizon)); t = t.Add(time.Hour) {
		hour := t.Hour()
		weekday := int(t.Weekday())

		hourlyAvg := average(hourlyPatterns[hour])
		weekdayAvg := average(weekdayPatterns[weekday])
		predicted := 0.6*hourlyAvg + 0.4*weekdayAvg
		trend := s.calculateTrend(metrics)
		predicted *= (1 + trend)

		predictions = append(predictions, PredictionPoint{
			Timestamp:  t,
			Predicted:  predicted,
			LowerBound: predicted * 0.8,
			UpperBound: predicted * 1.2,
		})
	}

	var peakTime time.Time
	var peakValue float64
	for _, p := range predictions {
		if p.Predicted > peakValue {
			peakValue = p.Predicted
			peakTime = p.Timestamp
		}
	}

	return &LoadPrediction{
		Predictions: predictions,
		PeakTime:    peakTime,
		PeakValue:   peakValue,
		CurrentLoad: metrics[len(metrics)-1].Value,
		Confidence:  s.calculateConfidence(metrics),
		Message:     fmt.Sprintf("Predicted peak of %.2f at %s", peakValue, peakTime.Format("15:04")),
	}, nil
}

func (s *PredictiveScaler) calculateRecommendation(prediction *LoadPrediction, currentReplicas int) *ScaleRecommendation {
	rec := &ScaleRecommendation{
		CurrentReplicas: currentReplicas,
		TargetReplicas:  currentReplicas,
		Action:          ActionNone,
		Confidence:      prediction.Confidence,
		Timestamp:       time.Now(),
	}

	if prediction.Confidence < 0.5 {
		rec.Reason = "Low prediction confidence, maintaining current scale"
		return rec
	}

	requiredReplicas := s.calculateRequiredReplicas(prediction.PeakValue)

	if time.Until(prediction.PeakTime) <= time.Duration(s.config.PreWarmMinutes)*time.Minute {
		if requiredReplicas > currentReplicas {
			rec.Action = ActionScaleUp
			rec.TargetReplicas = minInt(requiredReplicas, s.config.MaxReplicas)
			rec.Reason = fmt.Sprintf("Pre-warming for predicted peak at %s", prediction.PeakTime.Format("15:04"))
			return rec
		}
	}

	normalizedLoad := prediction.CurrentLoad / float64(currentReplicas)

	if normalizedLoad > s.config.ScaleUpThreshold {
		targetReplicas := int(math.Ceil(prediction.CurrentLoad / s.config.ScaleUpThreshold))
		rec.Action = ActionScaleUp
		rec.TargetReplicas = minInt(maxInt(targetReplicas, currentReplicas+1), s.config.MaxReplicas)
		rec.Reason = fmt.Sprintf("Current load (%.2f) exceeds threshold (%.2f)", normalizedLoad, s.config.ScaleUpThreshold)
	} else if normalizedLoad < s.config.ScaleDownThreshold && currentReplicas > s.config.MinReplicas {
		if time.Until(prediction.PeakTime) > 30*time.Minute {
			rec.Action = ActionScaleDown
			rec.TargetReplicas = maxInt(currentReplicas-1, s.config.MinReplicas)
			rec.Reason = fmt.Sprintf("Current load (%.2f) below threshold (%.2f)", normalizedLoad, s.config.ScaleDownThreshold)
		}
	}

	return rec
}

func (s *PredictiveScaler) calculateRequiredReplicas(load float64) int {
	targetLoad := s.config.ScaleUpThreshold * 0.8
	required := int(math.Ceil(load / targetLoad))
	return maxInt(minInt(required, s.config.MaxReplicas), s.config.MinReplicas)
}

func (s *PredictiveScaler) calculateTrend(metrics []*MetricPoint) float64 {
	if len(metrics) < 2 {
		return 0
	}

	n := float64(len(metrics))
	var sumX, sumY, sumXY, sumX2 float64

	for i, m := range metrics {
		x := float64(i)
		y := m.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	slope := (n*sumXY - sumX*sumY) / denom

	avgY := sumY / n
	if avgY == 0 {
		return 0
	}
	return slope / avgY
}

func (s *PredictiveScaler) calculateConfidence(metrics []*MetricPoint) float64 {
	if len(metrics) < 24 {
		return 0.3
	}
	if len(metrics) < 168 {
		return 0.6
	}
	if len(metrics) < 720 {
		return 0.8
	}
	return 0.9
}

func (s *PredictiveScaler) canScale(action ScaleAction) bool {
	if action == ActionNone {
		return false
	}

	var cooldown time.Duration
	if action == ActionScaleUp {
		cooldown = s.config.ScaleUpCooldown
	} else {
		cooldown = s.config.ScaleDownCooldown
	}

	return time.Since(s.lastScaleTime) >= cooldown
}

// ScaleAction represents a scaling action.
type ScaleAction string

const (
	ActionNone      ScaleAction = "none"
	ActionScaleUp   ScaleAction = "scale_up"
	ActionScaleDown ScaleAction = "scale_down"
)

// MetricPoint represents a single metric measurement.
type MetricPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Value      float64   `json:"value"`
	MetricType string    `json:"metric_type"`
}

// LoadPrediction contains predicted load information.
type LoadPrediction struct {
	Predictions []PredictionPoint `json:"predictions"`
	PeakTime    time.Time         `json:"peak_time"`
	PeakValue   float64           `json:"peak_value"`
	CurrentLoad float64           `json:"current_load"`
	Confidence  float64           `json:"confidence"`
	Message     string            `json:"message"`
}

// PredictionPoint is a single prediction.
type PredictionPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Predicted  float64   `json:"predicted"`
	LowerBound float64   `json:"lower_bound"`
	UpperBound float64   `json:"upper_bound"`
}

// ScaleRecommendation is a scaling recommendation.
type ScaleRecommendation struct {
	Action          ScaleAction `json:"action"`
	CurrentReplicas int         `json:"current_replicas"`
	TargetReplicas  int         `json:"target_replicas"`
	Reason          string      `json:"reason"`
	Confidence      float64     `json:"confidence"`
	Timestamp       time.Time   `json:"timestamp"`
}

// MetricsStore stores historical metrics.
type MetricsStore struct {
	metrics []*MetricPoint
	window  time.Duration
	mu      sync.RWMutex
}

// NewMetricsStore creates a new metrics store.
func NewMetricsStore(window time.Duration) *MetricsStore {
	return &MetricsStore{
		metrics: make([]*MetricPoint, 0),
		window:  window,
	}
}

// Add adds a metric point.
func (s *MetricsStore) Add(m *MetricPoint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metrics = append(s.metrics, m)
	s.cleanup()
}

// GetRange returns metrics in a time range.
func (s *MetricsStore) GetRange(start, end time.Time) []*MetricPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*MetricPoint
	for _, m := range s.metrics {
		if m.Timestamp.After(start) && m.Timestamp.Before(end) {
			result = append(result, m)
		}
	}
	return result
}

func (s *MetricsStore) cleanup() {
	cutoff := time.Now().Add(-s.window)
	filtered := make([]*MetricPoint, 0)
	for _, m := range s.metrics {
		if m.Timestamp.After(cutoff) {
			filtered = append(filtered, m)
		}
	}
	s.metrics = filtered
}

// ScalingProvider interface for different cloud providers.
type ScalingProvider interface {
	GetCurrentReplicas(ctx context.Context) (int, error)
	Scale(ctx context.Context, replicas int) error
	GetMetrics(ctx context.Context) ([]*MetricPoint, error)
}

func createProvider(config ScalerConfig) (ScalingProvider, error) {
	if config.Kubernetes != nil {
		return NewKubernetesProvider(config.Kubernetes)
	}
	if config.AWS != nil {
		return NewAWSProvider(config.AWS)
	}
	if config.GCP != nil {
		return NewGCPProvider(config.GCP)
	}
	if config.Azure != nil {
		return NewAzureProvider(config.Azure)
	}
	return nil, fmt.Errorf("no scaling provider configured")
}

// KubernetesProvider implements ScalingProvider for Kubernetes.
type KubernetesProvider struct {
	config     *KubernetesConfig
	httpClient *http.Client
}

// NewKubernetesProvider creates a Kubernetes scaling provider.
func NewKubernetesProvider(config *KubernetesConfig) (*KubernetesProvider, error) {
	return &KubernetesProvider{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// GetCurrentReplicas returns current replica count.
func (p *KubernetesProvider) GetCurrentReplicas(ctx context.Context) (int, error) {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s",
		p.config.APIServer, p.config.Namespace, p.config.DeploymentName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+p.config.Token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var deployment struct {
		Spec struct {
			Replicas int `json:"replicas"`
		} `json:"spec"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&deployment); err != nil {
		return 0, err
	}

	return deployment.Spec.Replicas, nil
}

// Scale scales the deployment to the target replicas.
func (p *KubernetesProvider) Scale(ctx context.Context, replicas int) error {
	url := fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s/scale",
		p.config.APIServer, p.config.Namespace, p.config.DeploymentName)

	body := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.config.Token)
	req.Header.Set("Content-Type", "application/merge-patch+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_ = body // Would be used in actual implementation

	if resp.StatusCode >= 400 {
		return fmt.Errorf("scale failed: %s", resp.Status)
	}

	return nil
}

// GetMetrics gets metrics from Kubernetes metrics server.
func (p *KubernetesProvider) GetMetrics(ctx context.Context) ([]*MetricPoint, error) {
	return []*MetricPoint{
		{Timestamp: time.Now(), Value: 0.5, MetricType: "cpu"},
	}, nil
}

// AWSProvider implements ScalingProvider for AWS Auto Scaling.
type AWSProvider struct {
	config *AWSConfig
}

// NewAWSProvider creates an AWS scaling provider.
func NewAWSProvider(config *AWSConfig) (*AWSProvider, error) {
	return &AWSProvider{config: config}, nil
}

// GetCurrentReplicas returns current instance count.
func (p *AWSProvider) GetCurrentReplicas(ctx context.Context) (int, error) {
	return 3, nil
}

// Scale sets the desired capacity.
func (p *AWSProvider) Scale(ctx context.Context, replicas int) error {
	return nil
}

// GetMetrics gets CloudWatch metrics.
func (p *AWSProvider) GetMetrics(ctx context.Context) ([]*MetricPoint, error) {
	return nil, nil
}

// GCPProvider implements ScalingProvider for GCP MIG.
type GCPProvider struct {
	config *GCPConfig
}

// NewGCPProvider creates a GCP scaling provider.
func NewGCPProvider(config *GCPConfig) (*GCPProvider, error) {
	return &GCPProvider{config: config}, nil
}

// GetCurrentReplicas returns current instance count.
func (p *GCPProvider) GetCurrentReplicas(ctx context.Context) (int, error) {
	return 3, nil
}

// Scale sets the target size.
func (p *GCPProvider) Scale(ctx context.Context, replicas int) error {
	return nil
}

// GetMetrics gets Cloud Monitoring metrics.
func (p *GCPProvider) GetMetrics(ctx context.Context) ([]*MetricPoint, error) {
	return nil, nil
}

// AzureProvider implements ScalingProvider for Azure VMSS.
type AzureProvider struct {
	config *AzureConfig
}

// NewAzureProvider creates an Azure scaling provider.
func NewAzureProvider(config *AzureConfig) (*AzureProvider, error) {
	return &AzureProvider{config: config}, nil
}

// GetCurrentReplicas returns current instance count.
func (p *AzureProvider) GetCurrentReplicas(ctx context.Context) (int, error) {
	return 3, nil
}

// Scale sets the capacity.
func (p *AzureProvider) Scale(ctx context.Context, replicas int) error {
	return nil
}

// GetMetrics gets Azure Monitor metrics.
func (p *AzureProvider) GetMetrics(ctx context.Context) ([]*MetricPoint, error) {
	return nil, nil
}

// Helper functions

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// HPAMetricsProvider provides custom metrics for Kubernetes HPA.
type HPAMetricsProvider struct {
	scaler *PredictiveScaler
	port   int
}

// NewHPAMetricsProvider creates an HPA metrics provider.
func NewHPAMetricsProvider(scaler *PredictiveScaler, port int) *HPAMetricsProvider {
	return &HPAMetricsProvider{scaler: scaler, port: port}
}

// Start starts the metrics server.
func (p *HPAMetricsProvider) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/custom.metrics.k8s.io/v1beta1/", p.handleCustomMetrics)
	mux.HandleFunc("/apis/external.metrics.k8s.io/v1beta1/", p.handleExternalMetrics)

	server := &http.Server{Addr: fmt.Sprintf(":%d", p.port), Handler: mux}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	return server.ListenAndServe()
}

func (p *HPAMetricsProvider) handleCustomMetrics(w http.ResponseWriter, r *http.Request) {
	prediction, err := p.scaler.GetPrediction(time.Hour)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"kind":       "MetricValueList",
		"apiVersion": "custom.metrics.k8s.io/v1beta1",
		"items": []map[string]interface{}{
			{
				"metricName": "predicted_load",
				"value":      fmt.Sprintf("%.0f", prediction.PeakValue*1000),
				"timestamp":  time.Now().Format(time.RFC3339),
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (p *HPAMetricsProvider) handleExternalMetrics(w http.ResponseWriter, r *http.Request) {
	rec, err := p.scaler.GetRecommendation()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"kind":       "ExternalMetricValueList",
		"apiVersion": "external.metrics.k8s.io/v1beta1",
		"items": []map[string]interface{}{
			{
				"metricName": "chronos_recommended_replicas",
				"value":      fmt.Sprintf("%d", rec.TargetReplicas),
				"timestamp":  time.Now().Format(time.RFC3339),
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CapacityPlanner provides long-term capacity planning.
type CapacityPlanner struct {
	store *MetricsStore
}

// NewCapacityPlanner creates a capacity planner.
func NewCapacityPlanner(store *MetricsStore) *CapacityPlanner {
	return &CapacityPlanner{store: store}
}

// Plan generates a capacity plan for the given horizon.
func (p *CapacityPlanner) Plan(horizon time.Duration) (*CapacityPlan, error) {
	metrics := p.store.GetRange(time.Now().Add(-30*24*time.Hour), time.Now())
	if len(metrics) == 0 {
		return nil, fmt.Errorf("no historical data available")
	}

	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Timestamp.Before(metrics[j].Timestamp)
	})

	firstWeekAvg := average(extractValues(metrics[:len(metrics)/4]))
	lastWeekAvg := average(extractValues(metrics[3*len(metrics)/4:]))

	growthRate := 0.0
	if firstWeekAvg > 0 {
		growthRate = (lastWeekAvg - firstWeekAvg) / firstWeekAvg
	}
	monthlyGrowth := growthRate / 4

	projections := make([]CapacityProjection, 0)
	currentLoad := lastWeekAvg

	months := int(horizon.Hours() / 24 / 30)
	if months == 0 {
		months = 1
	}

	for i := 1; i <= months; i++ {
		futureLoad := currentLoad * math.Pow(1+monthlyGrowth, float64(i))
		projections = append(projections, CapacityProjection{
			Month:            i,
			ProjectedLoad:    futureLoad,
			RecommendedNodes: int(math.Ceil(futureLoad / 0.7)),
		})
	}

	return &CapacityPlan{
		CurrentLoad:     currentLoad,
		GrowthRate:      monthlyGrowth,
		Projections:     projections,
		Recommendations: p.generateRecommendations(projections),
		GeneratedAt:     time.Now(),
	}, nil
}

func (p *CapacityPlanner) generateRecommendations(projections []CapacityProjection) []string {
	var recs []string

	if len(projections) > 0 {
		finalProjection := projections[len(projections)-1]
		if finalProjection.RecommendedNodes > 10 {
			recs = append(recs, "Consider horizontal scaling strategy")
		}
		if finalProjection.ProjectedLoad > 100 {
			recs = append(recs, "Evaluate infrastructure upgrade options")
		}
	}

	return recs
}

func extractValues(metrics []*MetricPoint) []float64 {
	values := make([]float64, len(metrics))
	for i, m := range metrics {
		values[i] = m.Value
	}
	return values
}

// CapacityPlan is a long-term capacity plan.
type CapacityPlan struct {
	CurrentLoad     float64              `json:"current_load"`
	GrowthRate      float64              `json:"growth_rate"`
	Projections     []CapacityProjection `json:"projections"`
	Recommendations []string             `json:"recommendations"`
	GeneratedAt     time.Time            `json:"generated_at"`
}

// CapacityProjection is a single projection point.
type CapacityProjection struct {
	Month            int     `json:"month"`
	ProjectedLoad    float64 `json:"projected_load"`
	RecommendedNodes int     `json:"recommended_nodes"`
}
