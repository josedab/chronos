package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/chronos/chronos/pkg/plugin"
)

func TestNew(t *testing.T) {
	cfg := Config{
		Address: "localhost:50051",
		Method:  "test.Service/Method",
	}

	executor := New(cfg)

	if executor == nil {
		t.Fatal("expected executor, got nil")
	}
	if executor.config.Timeout != 30*time.Second {
		t.Errorf("expected default timeout of 30s, got %v", executor.config.Timeout)
	}
	if executor.config.Codec != "json" {
		t.Errorf("expected default codec json, got %s", executor.config.Codec)
	}
}

func TestExecutor_Metadata(t *testing.T) {
	executor := New(Config{})
	meta := executor.Metadata()

	if meta.Name != "grpc" {
		t.Errorf("expected name grpc, got %s", meta.Name)
	}
	if meta.Type != plugin.TypeExecutor {
		t.Errorf("expected type executor, got %s", meta.Type)
	}
}

func TestExecutor_Init(t *testing.T) {
	executor := New(Config{})

	config := map[string]interface{}{
		"address": "grpc.example.com:443",
		"method":  "api.Service/Call",
		"tls":     true,
	}

	err := executor.Init(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if executor.config.Address != "grpc.example.com:443" {
		t.Errorf("expected address grpc.example.com:443, got %s", executor.config.Address)
	}
	if executor.config.Method != "api.Service/Call" {
		t.Errorf("expected method api.Service/Call, got %s", executor.config.Method)
	}
	if !executor.config.TLS {
		t.Error("expected TLS to be enabled")
	}
}

func TestExecutor_Health_NoAddress(t *testing.T) {
	executor := New(Config{Method: "test.Service/Method"})

	err := executor.Health(context.Background())
	if err == nil {
		t.Error("expected error for missing address")
	}
}

func TestExecutor_Health_NoMethod(t *testing.T) {
	executor := New(Config{Address: "localhost:50051"})

	err := executor.Health(context.Background())
	if err == nil {
		t.Error("expected error for missing method")
	}
}

func TestExecutor_Health_Valid(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	executor := New(Config{
		Address: listener.Addr().String(),
		Method:  "test.Service/Method",
	})

	err = executor.Health(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecutor_Close(t *testing.T) {
	executor := New(Config{})
	err := executor.Close(context.Background())
	if err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy.MaxAttempts != 3 {
		t.Errorf("expected 3 max attempts, got %d", policy.MaxAttempts)
	}
	if policy.BackoffMultiplier != 2.0 {
		t.Errorf("expected 2.0 multiplier, got %f", policy.BackoffMultiplier)
	}
	if len(policy.RetryableStatusCodes) != 2 {
		t.Errorf("expected 2 retryable status codes, got %d", len(policy.RetryableStatusCodes))
	}
}

func TestRoundRobinBalancer(t *testing.T) {
	addresses := []string{"server1:50051", "server2:50051", "server3:50051"}
	balancer := NewRoundRobinBalancer(addresses)

	// First round
	for i, expected := range addresses {
		addr := balancer.Pick()
		if addr != expected {
			t.Errorf("pick %d: expected %s, got %s", i, expected, addr)
		}
	}

	// Should wrap around
	addr := balancer.Pick()
	if addr != "server1:50051" {
		t.Errorf("expected wrap to server1:50051, got %s", addr)
	}
}

func TestRoundRobinBalancer_Update(t *testing.T) {
	balancer := NewRoundRobinBalancer([]string{"old:50051"})

	newAddresses := []string{"new1:50051", "new2:50051"}
	balancer.Update(newAddresses)

	addr := balancer.Pick()
	if addr != "new1:50051" {
		t.Errorf("expected new1:50051, got %s", addr)
	}
}

func TestRoundRobinBalancer_Empty(t *testing.T) {
	balancer := NewRoundRobinBalancer(nil)
	addr := balancer.Pick()
	if addr != "" {
		t.Errorf("expected empty string for empty balancer, got %s", addr)
	}
}

func TestChainInterceptors(t *testing.T) {
	var order []string

	interceptor1 := func(ctx context.Context, method string, req, reply interface{}, next func(ctx context.Context) error) error {
		order = append(order, "before1")
		err := next(ctx)
		order = append(order, "after1")
		return err
	}

	interceptor2 := func(ctx context.Context, method string, req, reply interface{}, next func(ctx context.Context) error) error {
		order = append(order, "before2")
		err := next(ctx)
		order = append(order, "after2")
		return err
	}

	chain := ChainInterceptors(interceptor1, interceptor2)
	chain(context.Background(), "test.Method", nil, nil, func(ctx context.Context) error {
		order = append(order, "handler")
		return nil
	})

	expected := []string{"before1", "before2", "handler", "after2", "after1"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d steps, got %d", len(expected), len(order))
	}

	for i, step := range expected {
		if order[i] != step {
			t.Errorf("step %d: expected %s, got %s", i, step, order[i])
		}
	}
}

func TestLoggingInterceptor(t *testing.T) {
	var logged string
	logger := func(format string, args ...interface{}) {
		logged = format
	}

	interceptor := LoggingInterceptor(logger)
	interceptor(context.Background(), "test.Method", nil, nil, func(ctx context.Context) error {
		return nil
	})

	if logged == "" {
		t.Error("expected log output")
	}
}

func TestMetricsInterceptor(t *testing.T) {
	var recordedMethod string
	var recordedDuration time.Duration

	recorder := func(method string, duration time.Duration, err error) {
		recordedMethod = method
		recordedDuration = duration
	}

	interceptor := MetricsInterceptor(recorder)
	interceptor(context.Background(), "test.Method", nil, nil, func(ctx context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	if recordedMethod != "test.Method" {
		t.Errorf("expected method test.Method, got %s", recordedMethod)
	}
	if recordedDuration < 10*time.Millisecond {
		t.Error("expected duration >= 10ms")
	}
}

func TestConnection_Close(t *testing.T) {
	conn := &Connection{}
	err := conn.Close()
	if err != nil {
		t.Errorf("unexpected error closing nil connection: %v", err)
	}
}

func TestServiceInfo(t *testing.T) {
	info := ServiceInfo{
		Name: "TestService",
		Methods: []MethodInfo{
			{Name: "Call", FullName: "test.TestService/Call"},
		},
	}

	if info.Name != "TestService" {
		t.Errorf("expected name TestService, got %s", info.Name)
	}
	if len(info.Methods) != 1 {
		t.Errorf("expected 1 method, got %d", len(info.Methods))
	}
}

func TestHealthCheckConstants(t *testing.T) {
	if HealthStatusServing != "SERVING" {
		t.Errorf("expected SERVING, got %s", HealthStatusServing)
	}
	if HealthStatusNotServing != "NOT_SERVING" {
		t.Errorf("expected NOT_SERVING, got %s", HealthStatusNotServing)
	}
}
