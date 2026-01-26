package executor

import (
	"testing"
	"time"
)

func TestGRPCConfig(t *testing.T) {
	cfg := GRPCConfig{
		Target:  "localhost:50051",
		Service: "example.JobService",
		Method:  "Execute",
		UseTLS:  true,
		Timeout: 30 * time.Second,
		Metadata: map[string]string{
			"api-key": "test-key",
		},
	}

	if cfg.Target != "localhost:50051" {
		t.Errorf("expected target localhost:50051, got %s", cfg.Target)
	}
	if cfg.Service != "example.JobService" {
		t.Errorf("expected service example.JobService, got %s", cfg.Service)
	}
	if !cfg.UseTLS {
		t.Error("expected UseTLS to be true")
	}
}

func TestLambdaConfig(t *testing.T) {
	cfg := LambdaConfig{
		FunctionName:   "my-function",
		Region:         "us-east-1",
		InvocationType: "RequestResponse",
		Qualifier:      "prod",
	}

	if cfg.FunctionName != "my-function" {
		t.Errorf("expected function name my-function, got %s", cfg.FunctionName)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("expected region us-east-1, got %s", cfg.Region)
	}
}

func TestDockerConfig(t *testing.T) {
	cfg := DockerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
		Env: map[string]string{
			"FOO": "bar",
		},
		Memory:     512 * 1024 * 1024, // 512MB
		CPUs:       0.5,
		AutoRemove: true,
	}

	if cfg.Image != "alpine:latest" {
		t.Errorf("expected image alpine:latest, got %s", cfg.Image)
	}
	if len(cfg.Command) != 2 {
		t.Errorf("expected 2 command args, got %d", len(cfg.Command))
	}
	if !cfg.AutoRemove {
		t.Error("expected AutoRemove to be true")
	}
}

func TestKubernetesConfig(t *testing.T) {
	cfg := KubernetesConfig{
		Namespace:      "default",
		Image:          "busybox:latest",
		Command:        []string{"/bin/sh"},
		Args:           []string{"-c", "echo hello"},
		ServiceAccount: "job-runner",
		BackoffLimit:   3,
		TTLSecondsAfterFinished: 600,
		Resources: &ResourceRequirements{
			Requests: ResourceList{
				CPU:    "100m",
				Memory: "128Mi",
			},
			Limits: ResourceList{
				CPU:    "500m",
				Memory: "512Mi",
			},
		},
	}

	if cfg.Namespace != "default" {
		t.Errorf("expected namespace default, got %s", cfg.Namespace)
	}
	if cfg.BackoffLimit != 3 {
		t.Errorf("expected backoff limit 3, got %d", cfg.BackoffLimit)
	}
	if cfg.Resources == nil {
		t.Fatal("expected resources to be set")
	}
	if cfg.Resources.Requests.CPU != "100m" {
		t.Errorf("expected CPU request 100m, got %s", cfg.Resources.Requests.CPU)
	}
}

func TestExecutorType(t *testing.T) {
	tests := []struct {
		name     string
		execType Type
		expected string
	}{
		{"HTTP", TypeHTTP, "http"},
		{"gRPC", TypeGRPC, "grpc"},
		{"Lambda", TypeLambda, "lambda"},
		{"Docker", TypeDocker, "docker"},
		{"Kubernetes", TypeKubernetes, "kubernetes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.execType) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.execType)
			}
		})
	}
}

func TestConfig(t *testing.T) {
	cfg := Config{
		Type:    TypeGRPC,
		Timeout: 30 * time.Second,
		Settings: map[string]interface{}{
			"target":  "localhost:50051",
			"service": "example.JobService",
		},
	}

	if cfg.Type != TypeGRPC {
		t.Errorf("expected type grpc, got %s", cfg.Type)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %s", cfg.Timeout)
	}
	if cfg.Settings["target"] != "localhost:50051" {
		t.Error("expected settings to contain target")
	}
}
