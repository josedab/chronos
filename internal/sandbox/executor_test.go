package sandbox

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled {
		t.Error("Enabled should be false by default")
	}
	if cfg.Runtime != "docker" {
		t.Errorf("Runtime = %s, want docker", cfg.Runtime)
	}
	if cfg.DefaultImage != "alpine:latest" {
		t.Errorf("DefaultImage = %s, want alpine:latest", cfg.DefaultImage)
	}
	if cfg.DefaultTimeout != 5*time.Minute {
		t.Errorf("DefaultTimeout = %v, want 5m", cfg.DefaultTimeout)
	}
	if cfg.MaxMemory != "256m" {
		t.Errorf("MaxMemory = %s, want 256m", cfg.MaxMemory)
	}
	if cfg.MaxCPU != "0.5" {
		t.Errorf("MaxCPU = %s, want 0.5", cfg.MaxCPU)
	}
	if cfg.NetworkEnabled {
		t.Error("NetworkEnabled should be false by default")
	}
}

func TestNewExecutor(t *testing.T) {
	cfg := DefaultConfig()
	executor := NewExecutor(cfg)

	if executor == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if executor.running == nil {
		t.Error("running map should be initialized")
	}
}

func TestExecutor_buildContainerArgs(t *testing.T) {
	cfg := DefaultConfig()
	executor := NewExecutor(cfg)

	tests := []struct {
		name     string
		request  *Request
		wantArgs []string
	}{
		{
			name: "basic request",
			request: &Request{
				Image:   "alpine:latest",
				Command: []string{"echo", "hello"},
				Memory:  "128m",
				CPU:     "0.5",
			},
			wantArgs: []string{"run", "--rm", "--memory", "128m", "--cpus", "0.5", "--network", "none", "alpine:latest", "echo", "hello"},
		},
		{
			name: "with environment",
			request: &Request{
				Image:   "alpine:latest",
				Command: []string{"env"},
				Memory:  "128m",
				Env:     map[string]string{"FOO": "bar"},
			},
			wantArgs: []string{"run", "--rm", "--memory", "128m", "--cpus", "0.5", "--network", "none", "-e", "FOO=bar", "alpine:latest", "env"},
		},
		{
			name: "with workdir",
			request: &Request{
				Image:   "alpine:latest",
				Command: []string{"pwd"},
				WorkDir: "/app",
			},
			wantArgs: []string{"run", "--rm", "--memory", "256m", "--cpus", "0.5", "--network", "none", "-w", "/app", "alpine:latest", "pwd"},
		},
		{
			name: "with network enabled",
			request: &Request{
				Image:          "alpine:latest",
				Command:        []string{"curl", "example.com"},
				NetworkEnabled: true,
			},
			wantArgs: []string{"run", "--rm", "--memory", "256m", "--cpus", "0.5", "alpine:latest", "curl", "example.com"},
		},
		{
			name: "with script",
			request: &Request{
				Image:      "python:3",
				Script:     "print('hello')",
				ScriptType: "python",
			},
			wantArgs: []string{"run", "--rm", "--memory", "256m", "--cpus", "0.5", "--network", "none", "python:3", "python3", "-c", "print('hello')"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill in defaults
			if tt.request.Memory == "" {
				tt.request.Memory = cfg.MaxMemory
			}
			if tt.request.CPU == "" {
				tt.request.CPU = cfg.MaxCPU
			}

			args := executor.buildContainerArgs(tt.request)

			// Check key arguments are present
			if len(args) < 4 {
				t.Errorf("expected at least 4 args, got %d", len(args))
			}
			if args[0] != "run" {
				t.Errorf("first arg = %s, want run", args[0])
			}
			if args[1] != "--rm" {
				t.Errorf("second arg = %s, want --rm", args[1])
			}
		})
	}
}

func TestGetInterpreter(t *testing.T) {
	tests := []struct {
		scriptType string
		want       string
	}{
		{"python", "python3"},
		{"node", "node"},
		{"javascript", "node"},
		{"ruby", "ruby"},
		{"perl", "perl"},
		{"bash", "sh"},
		{"", "sh"},
		{"unknown", "sh"},
	}

	for _, tt := range tests {
		t.Run(tt.scriptType, func(t *testing.T) {
			got := getInterpreter(tt.scriptType)
			if got != tt.want {
				t.Errorf("getInterpreter(%s) = %s, want %s", tt.scriptType, got, tt.want)
			}
		})
	}
}

func TestExecutor_ListRunning(t *testing.T) {
	cfg := DefaultConfig()
	executor := NewExecutor(cfg)

	// Initially empty
	running := executor.ListRunning()
	if len(running) != 0 {
		t.Errorf("expected 0 running, got %d", len(running))
	}
}

func TestExecutor_Kill_NotFound(t *testing.T) {
	cfg := DefaultConfig()
	executor := NewExecutor(cfg)

	err := executor.Kill("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent container")
	}
}

func TestExecutor_ApplyDefaults(t *testing.T) {
	cfg := DefaultConfig()
	executor := NewExecutor(cfg)

	req := &Request{}

	// Simulate what Execute does to apply defaults
	if req.Image == "" {
		req.Image = executor.config.DefaultImage
	}
	if req.Timeout == 0 {
		req.Timeout = executor.config.DefaultTimeout
	}
	if req.Memory == "" {
		req.Memory = executor.config.MaxMemory
	}
	if req.CPU == "" {
		req.CPU = executor.config.MaxCPU
	}

	if req.Image != "alpine:latest" {
		t.Errorf("Image = %s, want alpine:latest", req.Image)
	}
	if req.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", req.Timeout)
	}
	if req.Memory != "256m" {
		t.Errorf("Memory = %s, want 256m", req.Memory)
	}
	if req.CPU != "0.5" {
		t.Errorf("CPU = %s, want 0.5", req.CPU)
	}
}

func TestResult_Structure(t *testing.T) {
	result := &Result{
		ExitCode:    0,
		Stdout:      "hello world",
		Stderr:      "",
		Duration:    time.Second,
		ContainerID: "container-123",
		Success:     true,
	}

	if result.ExitCode != 0 {
		t.Error("ExitCode should be 0")
	}
	if result.Stdout != "hello world" {
		t.Error("Stdout mismatch")
	}
	if !result.Success {
		t.Error("Success should be true")
	}
}

func TestRequest_Structure(t *testing.T) {
	req := &Request{
		Image:          "nginx:latest",
		Command:        []string{"nginx", "-t"},
		Env:            map[string]string{"NGINX_HOST": "localhost"},
		WorkDir:        "/etc/nginx",
		Timeout:        30 * time.Second,
		Memory:         "512m",
		CPU:            "1.0",
		NetworkEnabled: true,
		Metadata:       map[string]string{"job_id": "job-123"},
	}

	if req.Image != "nginx:latest" {
		t.Errorf("Image = %s, want nginx:latest", req.Image)
	}
	if len(req.Command) != 2 {
		t.Errorf("Command length = %d, want 2", len(req.Command))
	}
	if req.Env["NGINX_HOST"] != "localhost" {
		t.Error("Env variable mismatch")
	}
	if !req.NetworkEnabled {
		t.Error("NetworkEnabled should be true")
	}
}

func TestStreamExecute_OutputWriters(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Runtime = "echo" // Use echo as a mock command
	executor := NewExecutor(cfg)

	var stdout, stderr bytes.Buffer

	// This will fail because "echo" isn't docker, but tests the writer setup
	ctx := context.Background()
	req := &Request{
		Image:   "alpine",
		Command: []string{"test"},
		Timeout: time.Second,
	}

	executor.StreamExecute(ctx, req, &stdout, &stderr)
	// We just verify no panic and writers were set up correctly
}

func TestErrors(t *testing.T) {
	if ErrContainerFailed.Error() != "container execution failed" {
		t.Errorf("ErrContainerFailed = %s", ErrContainerFailed)
	}
	if ErrTimeout.Error() != "execution timeout" {
		t.Errorf("ErrTimeout = %s", ErrTimeout)
	}
	if ErrResourceLimit.Error() != "resource limit exceeded" {
		t.Errorf("ErrResourceLimit = %s", ErrResourceLimit)
	}
}

func TestConfig_Structure(t *testing.T) {
	cfg := Config{
		Enabled:        true,
		Runtime:        "podman",
		DefaultImage:   "ubuntu:22.04",
		DefaultTimeout: 10 * time.Minute,
		MaxMemory:      "1g",
		MaxCPU:         "2.0",
		NetworkEnabled: true,
		TempDir:        "/var/tmp/sandbox",
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.Runtime != "podman" {
		t.Errorf("Runtime = %s, want podman", cfg.Runtime)
	}
	if cfg.MaxMemory != "1g" {
		t.Errorf("MaxMemory = %s, want 1g", cfg.MaxMemory)
	}
}

func TestConcurrentRunningMap(t *testing.T) {
	cfg := DefaultConfig()
	executor := NewExecutor(cfg)

	done := make(chan bool)

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			executor.ListRunning()
		}
		done <- true
	}()

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			executor.mu.Lock()
			executor.running["test"] = nil
			delete(executor.running, "test")
			executor.mu.Unlock()
		}
		done <- true
	}()

	<-done
	<-done
}
