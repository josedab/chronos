package autoscale

import (
	"context"
	"testing"
	"time"
)

func TestDefaultScalerConfig(t *testing.T) {
	cfg := DefaultScalerConfig()

	if cfg.MinReplicas != 1 {
		t.Errorf("expected MinReplicas=1, got %d", cfg.MinReplicas)
	}
	if cfg.MaxReplicas != 10 {
		t.Errorf("expected MaxReplicas=10, got %d", cfg.MaxReplicas)
	}
	if cfg.ScaleUpThreshold != 0.7 {
		t.Errorf("expected ScaleUpThreshold=0.7, got %f", cfg.ScaleUpThreshold)
	}
	if cfg.ScaleDownThreshold != 0.3 {
		t.Errorf("expected ScaleDownThreshold=0.3, got %f", cfg.ScaleDownThreshold)
	}
}

func TestMetricsStore_AddAndGet(t *testing.T) {
	store := NewMetricsStore(time.Hour)

	now := time.Now()
	store.Add(&MetricPoint{Timestamp: now, Value: 10.0, MetricType: "cpu"})
	store.Add(&MetricPoint{Timestamp: now.Add(time.Minute), Value: 20.0, MetricType: "cpu"})
	store.Add(&MetricPoint{Timestamp: now.Add(2 * time.Minute), Value: 30.0, MetricType: "cpu"})

	metrics := store.GetRange(now.Add(-time.Second), now.Add(3*time.Minute))
	if len(metrics) != 3 {
		t.Errorf("expected 3 metrics, got %d", len(metrics))
	}
}

func TestMetricsStore_Cleanup(t *testing.T) {
	store := NewMetricsStore(time.Minute)

	old := time.Now().Add(-2 * time.Minute)
	recent := time.Now()

	store.Add(&MetricPoint{Timestamp: old, Value: 10.0, MetricType: "cpu"})
	store.Add(&MetricPoint{Timestamp: recent, Value: 20.0, MetricType: "cpu"})

	// Cleanup happens on Add, so old metrics should be removed
	metrics := store.GetRange(old.Add(-time.Hour), recent.Add(time.Hour))
	if len(metrics) != 1 {
		t.Errorf("expected 1 metric after cleanup, got %d", len(metrics))
	}
}

func TestMetricsStore_GetRange_Empty(t *testing.T) {
	store := NewMetricsStore(time.Hour)
	metrics := store.GetRange(time.Now().Add(-time.Hour), time.Now())
	if len(metrics) != 0 {
		t.Errorf("expected empty result, got %d metrics", len(metrics))
	}
}

func TestScaleAction_Constants(t *testing.T) {
	if ActionNone != "none" {
		t.Errorf("ActionNone = %q, want 'none'", ActionNone)
	}
	if ActionScaleUp != "scale_up" {
		t.Errorf("ActionScaleUp = %q, want 'scale_up'", ActionScaleUp)
	}
	if ActionScaleDown != "scale_down" {
		t.Errorf("ActionScaleDown = %q, want 'scale_down'", ActionScaleDown)
	}
}

func TestAverage(t *testing.T) {
	tests := []struct {
		values   []float64
		expected float64
	}{
		{[]float64{}, 0},
		{[]float64{10}, 10},
		{[]float64{10, 20, 30}, 20},
		{[]float64{1, 2, 3, 4, 5}, 3},
	}

	for _, tt := range tests {
		got := average(tt.values)
		if got != tt.expected {
			t.Errorf("average(%v) = %f, want %f", tt.values, got, tt.expected)
		}
	}
}

func TestMinInt(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{4, 4, 4},
		{-1, 1, -1},
	}

	for _, tt := range tests {
		if got := minInt(tt.a, tt.b); got != tt.expected {
			t.Errorf("minInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestMaxInt(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 2},
		{5, 3, 5},
		{4, 4, 4},
		{-1, 1, 1},
	}

	for _, tt := range tests {
		if got := maxInt(tt.a, tt.b); got != tt.expected {
			t.Errorf("maxInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.expected)
		}
	}
}

// MockProvider implements ScalingProvider for testing.
type MockProvider struct {
	replicas int
	scaled   []int
}

func (m *MockProvider) GetCurrentReplicas(ctx context.Context) (int, error) {
	return m.replicas, nil
}

func (m *MockProvider) Scale(ctx context.Context, replicas int) error {
	m.scaled = append(m.scaled, replicas)
	m.replicas = replicas
	return nil
}

func (m *MockProvider) GetMetrics(ctx context.Context) ([]*MetricPoint, error) {
	return nil, nil
}

func TestNewPredictiveScaler_NoProvider(t *testing.T) {
	cfg := DefaultScalerConfig()
	_, err := NewPredictiveScaler(cfg)
	if err == nil {
		t.Error("expected error when no provider configured")
	}
}

func TestNewPredictiveScaler_WithKubernetes(t *testing.T) {
	cfg := DefaultScalerConfig()
	cfg.Kubernetes = &KubernetesConfig{
		APIServer:      "https://localhost:6443",
		Namespace:      "default",
		DeploymentName: "chronos",
	}
	scaler, err := NewPredictiveScaler(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if scaler == nil {
		t.Error("expected scaler to be created")
	}
}

func TestNewPredictiveScaler_WithAWS(t *testing.T) {
	cfg := DefaultScalerConfig()
	cfg.AWS = &AWSConfig{
		Region:               "us-east-1",
		AutoScalingGroupName: "chronos-asg",
	}
	scaler, err := NewPredictiveScaler(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if scaler == nil {
		t.Error("expected scaler to be created")
	}
}

func TestNewPredictiveScaler_WithGCP(t *testing.T) {
	cfg := DefaultScalerConfig()
	cfg.GCP = &GCPConfig{
		ProjectID:         "my-project",
		Zone:              "us-central1-a",
		InstanceGroupName: "chronos-mig",
	}
	scaler, err := NewPredictiveScaler(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if scaler == nil {
		t.Error("expected scaler to be created")
	}
}

func TestNewPredictiveScaler_WithAzure(t *testing.T) {
	cfg := DefaultScalerConfig()
	cfg.Azure = &AzureConfig{
		SubscriptionID: "sub-123",
		ResourceGroup:  "chronos-rg",
		VMSSName:       "chronos-vmss",
	}
	scaler, err := NewPredictiveScaler(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if scaler == nil {
		t.Error("expected scaler to be created")
	}
}

func TestPredictiveScaler_RecordMetric(t *testing.T) {
	cfg := DefaultScalerConfig()
	cfg.Kubernetes = &KubernetesConfig{
		APIServer:      "https://localhost:6443",
		Namespace:      "default",
		DeploymentName: "chronos",
	}
	scaler, err := NewPredictiveScaler(cfg)
	if err != nil {
		t.Fatalf("failed to create scaler: %v", err)
	}

	scaler.RecordMetric(&MetricPoint{
		Timestamp:  time.Now(),
		Value:      0.5,
		MetricType: "cpu",
	})

	// Verify metric was stored
	metrics := scaler.metricsStore.GetRange(time.Now().Add(-time.Minute), time.Now().Add(time.Minute))
	if len(metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(metrics))
	}
}

func TestPredictiveScaler_GetPrediction_InsufficientData(t *testing.T) {
	cfg := DefaultScalerConfig()
	cfg.Kubernetes = &KubernetesConfig{
		APIServer:      "https://localhost:6443",
		Namespace:      "default",
		DeploymentName: "chronos",
	}
	scaler, err := NewPredictiveScaler(cfg)
	if err != nil {
		t.Fatalf("failed to create scaler: %v", err)
	}

	prediction, err := scaler.GetPrediction(time.Hour)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if prediction.Confidence != 0.3 {
		t.Errorf("expected confidence 0.3 for insufficient data, got %f", prediction.Confidence)
	}
	if prediction.Message != "Insufficient data for accurate prediction" {
		t.Errorf("unexpected message: %s", prediction.Message)
	}
}

func TestPredictiveScaler_CanScale_Cooldown(t *testing.T) {
	cfg := DefaultScalerConfig()
	cfg.ScaleUpCooldown = 5 * time.Minute
	cfg.ScaleDownCooldown = 10 * time.Minute
	cfg.Kubernetes = &KubernetesConfig{
		APIServer:      "https://localhost:6443",
		Namespace:      "default",
		DeploymentName: "chronos",
	}

	scaler, err := NewPredictiveScaler(cfg)
	if err != nil {
		t.Fatalf("failed to create scaler: %v", err)
	}

	// No last scale time - should be able to scale
	if !scaler.canScale(ActionScaleUp) {
		t.Error("expected to be able to scale up initially")
	}

	// Set last scale time to now
	scaler.lastScaleTime = time.Now()

	// Should not be able to scale immediately after
	if scaler.canScale(ActionScaleUp) {
		t.Error("expected cooldown to prevent scaling")
	}

	// ActionNone should always return false
	if scaler.canScale(ActionNone) {
		t.Error("ActionNone should always return false")
	}
}

func TestCapacityPlanner_Plan_NoData(t *testing.T) {
	store := NewMetricsStore(time.Hour)
	planner := NewCapacityPlanner(store)

	_, err := planner.Plan(30 * 24 * time.Hour)
	if err == nil {
		t.Error("expected error when no data available")
	}
}

func TestCapacityPlanner_Plan_WithData(t *testing.T) {
	store := NewMetricsStore(60 * 24 * time.Hour)
	planner := NewCapacityPlanner(store)

	// Add 30 days of simulated data
	now := time.Now()
	for i := 0; i < 30*24; i++ {
		store.Add(&MetricPoint{
			Timestamp:  now.Add(-time.Duration(30*24-i) * time.Hour),
			Value:      float64(50 + i/24), // Slowly increasing load
			MetricType: "jobs",
		})
	}

	plan, err := planner.Plan(90 * 24 * time.Hour)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected plan to be generated")
	}
	if plan.CurrentLoad <= 0 {
		t.Error("expected positive current load")
	}
	if len(plan.Projections) == 0 {
		t.Error("expected projections to be generated")
	}
}

func TestExtractValues(t *testing.T) {
	metrics := []*MetricPoint{
		{Value: 1.0},
		{Value: 2.0},
		{Value: 3.0},
	}

	values := extractValues(metrics)
	if len(values) != 3 {
		t.Errorf("expected 3 values, got %d", len(values))
	}
	if values[0] != 1.0 || values[1] != 2.0 || values[2] != 3.0 {
		t.Errorf("unexpected values: %v", values)
	}
}

func TestPredictiveScaler_CalculateTrend(t *testing.T) {
	cfg := DefaultScalerConfig()
	cfg.Kubernetes = &KubernetesConfig{
		APIServer:      "https://localhost:6443",
		Namespace:      "default",
		DeploymentName: "chronos",
	}
	scaler, err := NewPredictiveScaler(cfg)
	if err != nil {
		t.Fatalf("failed to create scaler: %v", err)
	}

	// Empty metrics - should return 0
	trend := scaler.calculateTrend(nil)
	if trend != 0 {
		t.Errorf("expected 0 trend for empty metrics, got %f", trend)
	}

	// Single metric - should return 0
	trend = scaler.calculateTrend([]*MetricPoint{{Value: 10}})
	if trend != 0 {
		t.Errorf("expected 0 trend for single metric, got %f", trend)
	}

	// Increasing trend
	increasing := []*MetricPoint{
		{Value: 10},
		{Value: 20},
		{Value: 30},
		{Value: 40},
	}
	trend = scaler.calculateTrend(increasing)
	if trend <= 0 {
		t.Errorf("expected positive trend for increasing data, got %f", trend)
	}
}

func TestPredictiveScaler_CalculateConfidence(t *testing.T) {
	cfg := DefaultScalerConfig()
	cfg.Kubernetes = &KubernetesConfig{
		APIServer:      "https://localhost:6443",
		Namespace:      "default",
		DeploymentName: "chronos",
	}
	scaler, err := NewPredictiveScaler(cfg)
	if err != nil {
		t.Fatalf("failed to create scaler: %v", err)
	}

	tests := []struct {
		metricCount int
		minConf     float64
		maxConf     float64
	}{
		{10, 0.3, 0.3},    // < 24: 0.3
		{50, 0.6, 0.6},    // < 168: 0.6
		{200, 0.8, 0.8},   // < 720: 0.8
		{1000, 0.9, 0.9},  // >= 720: 0.9
	}

	for _, tt := range tests {
		metrics := make([]*MetricPoint, tt.metricCount)
		for i := range metrics {
			metrics[i] = &MetricPoint{Value: float64(i)}
		}
		conf := scaler.calculateConfidence(metrics)
		if conf < tt.minConf || conf > tt.maxConf {
			t.Errorf("with %d metrics, expected confidence [%f, %f], got %f",
				tt.metricCount, tt.minConf, tt.maxConf, conf)
		}
	}
}

func TestPredictiveScaler_CalculateRequiredReplicas(t *testing.T) {
	cfg := DefaultScalerConfig()
	cfg.MinReplicas = 2
	cfg.MaxReplicas = 10
	cfg.ScaleUpThreshold = 0.7
	cfg.Kubernetes = &KubernetesConfig{
		APIServer:      "https://localhost:6443",
		Namespace:      "default",
		DeploymentName: "chronos",
	}

	scaler, err := NewPredictiveScaler(cfg)
	if err != nil {
		t.Fatalf("failed to create scaler: %v", err)
	}

	tests := []struct {
		load     float64
		expected int
	}{
		{0.5, 2},   // Low load, capped at min
		{5.0, 9},   // Medium load
		{100.0, 10}, // High load, capped at max
	}

	for _, tt := range tests {
		got := scaler.calculateRequiredReplicas(tt.load)
		if got != tt.expected {
			t.Errorf("calculateRequiredReplicas(%f) = %d, want %d", tt.load, got, tt.expected)
		}
	}
}
