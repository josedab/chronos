// Package sandbox provides isolated container execution for Chronos jobs.
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Common errors.
var (
	ErrContainerFailed = errors.New("container execution failed")
	ErrTimeout         = errors.New("execution timeout")
	ErrResourceLimit   = errors.New("resource limit exceeded")
)

// Config configures the sandbox executor.
type Config struct {
	// Enabled enables sandbox execution.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Runtime is the container runtime (docker, podman, containerd).
	Runtime string `json:"runtime" yaml:"runtime"`
	// DefaultImage is the default container image.
	DefaultImage string `json:"default_image" yaml:"default_image"`
	// DefaultTimeout is the default execution timeout.
	DefaultTimeout time.Duration `json:"default_timeout" yaml:"default_timeout"`
	// MaxMemory is the maximum memory limit.
	MaxMemory string `json:"max_memory" yaml:"max_memory"`
	// MaxCPU is the maximum CPU limit.
	MaxCPU string `json:"max_cpu" yaml:"max_cpu"`
	// NetworkEnabled allows network access.
	NetworkEnabled bool `json:"network_enabled" yaml:"network_enabled"`
	// TempDir is the directory for temporary files.
	TempDir string `json:"temp_dir" yaml:"temp_dir"`
}

// DefaultConfig returns default sandbox configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		Runtime:        "docker",
		DefaultImage:   "alpine:latest",
		DefaultTimeout: 5 * time.Minute,
		MaxMemory:      "256m",
		MaxCPU:         "0.5",
		NetworkEnabled: false,
		TempDir:        "/tmp/chronos-sandbox",
	}
}

// Request contains the sandbox execution request.
type Request struct {
	// Image is the container image to use.
	Image string `json:"image"`
	// Command is the command to run.
	Command []string `json:"command"`
	// Script is inline script content to execute.
	Script string `json:"script,omitempty"`
	// ScriptType is the script language (bash, python, node).
	ScriptType string `json:"script_type,omitempty"`
	// Environment variables.
	Env map[string]string `json:"env,omitempty"`
	// WorkDir is the working directory.
	WorkDir string `json:"work_dir,omitempty"`
	// Timeout for execution.
	Timeout time.Duration `json:"timeout,omitempty"`
	// Memory limit.
	Memory string `json:"memory,omitempty"`
	// CPU limit.
	CPU string `json:"cpu,omitempty"`
	// NetworkEnabled allows network access.
	NetworkEnabled bool `json:"network_enabled,omitempty"`
	// Metadata for tracking.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Result contains the sandbox execution result.
type Result struct {
	// ExitCode is the container exit code.
	ExitCode int `json:"exit_code"`
	// Stdout is the standard output.
	Stdout string `json:"stdout"`
	// Stderr is the standard error.
	Stderr string `json:"stderr"`
	// Duration is the execution duration.
	Duration time.Duration `json:"duration"`
	// ContainerID is the container ID.
	ContainerID string `json:"container_id,omitempty"`
	// Error message if execution failed.
	Error string `json:"error,omitempty"`
	// Success indicates successful execution.
	Success bool `json:"success"`
}

// Executor executes jobs in isolated containers.
type Executor struct {
	mu      sync.RWMutex
	config  Config
	running map[string]*exec.Cmd
}

// NewExecutor creates a new sandbox executor.
func NewExecutor(cfg Config) *Executor {
	return &Executor{
		config:  cfg,
		running: make(map[string]*exec.Cmd),
	}
}

// Execute runs a command in a sandboxed container.
func (e *Executor) Execute(ctx context.Context, req *Request) (*Result, error) {
	start := time.Now()

	// Apply defaults
	if req.Image == "" {
		req.Image = e.config.DefaultImage
	}
	if req.Timeout == 0 {
		req.Timeout = e.config.DefaultTimeout
	}
	if req.Memory == "" {
		req.Memory = e.config.MaxMemory
	}
	if req.CPU == "" {
		req.CPU = e.config.MaxCPU
	}

	// Build command arguments
	args := e.buildContainerArgs(req)

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	// Execute container
	cmd := exec.CommandContext(execCtx, e.config.Runtime, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Track running container
	containerID := fmt.Sprintf("%d", time.Now().UnixNano())
	e.mu.Lock()
	e.running[containerID] = cmd
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.running, containerID)
		e.mu.Unlock()
	}()

	// Run the command
	err := cmd.Run()

	result := &Result{
		Stdout:      stdout.String(),
		Stderr:      stderr.String(),
		Duration:    time.Since(start),
		ContainerID: containerID,
	}

	if err != nil {
		result.Success = false

		// Check for timeout
		if execCtx.Err() == context.DeadlineExceeded {
			result.Error = ErrTimeout.Error()
			result.ExitCode = -1
			return result, ErrTimeout
		}

		// Extract exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			}
		}
		result.Error = err.Error()
		return result, nil
	}

	result.Success = true
	result.ExitCode = 0
	return result, nil
}

// buildContainerArgs builds the container runtime arguments.
func (e *Executor) buildContainerArgs(req *Request) []string {
	args := []string{"run", "--rm"}

	// Resource limits
	if req.Memory != "" {
		args = append(args, "--memory", req.Memory)
	}
	if req.CPU != "" {
		args = append(args, "--cpus", req.CPU)
	}

	// Network
	if !req.NetworkEnabled && !e.config.NetworkEnabled {
		args = append(args, "--network", "none")
	}

	// Environment variables
	for k, v := range req.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Working directory
	if req.WorkDir != "" {
		args = append(args, "-w", req.WorkDir)
	}

	// Image
	args = append(args, req.Image)

	// Command
	if len(req.Command) > 0 {
		args = append(args, req.Command...)
	} else if req.Script != "" {
		// Execute inline script
		interpreter := getInterpreter(req.ScriptType)
		args = append(args, interpreter, "-c", req.Script)
	}

	return args
}

// getInterpreter returns the interpreter for a script type.
func getInterpreter(scriptType string) string {
	switch scriptType {
	case "python":
		return "python3"
	case "node", "javascript":
		return "node"
	case "ruby":
		return "ruby"
	case "perl":
		return "perl"
	default:
		return "sh"
	}
}

// Kill terminates a running container.
func (e *Executor) Kill(containerID string) error {
	e.mu.RLock()
	cmd, exists := e.running[containerID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("container %s not found", containerID)
	}

	if cmd.Process != nil {
		return cmd.Process.Kill()
	}

	return nil
}

// ListRunning returns IDs of running containers.
func (e *Executor) ListRunning() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make([]string, 0, len(e.running))
	for id := range e.running {
		ids = append(ids, id)
	}
	return ids
}

// StreamExecute executes with streaming output.
func (e *Executor) StreamExecute(ctx context.Context, req *Request, stdout, stderr io.Writer) (*Result, error) {
	start := time.Now()

	if req.Image == "" {
		req.Image = e.config.DefaultImage
	}
	if req.Timeout == 0 {
		req.Timeout = e.config.DefaultTimeout
	}

	args := e.buildContainerArgs(req)

	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, e.config.Runtime, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	containerID := fmt.Sprintf("%d", time.Now().UnixNano())
	e.mu.Lock()
	e.running[containerID] = cmd
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.running, containerID)
		e.mu.Unlock()
	}()

	err := cmd.Run()

	result := &Result{
		Duration:    time.Since(start),
		ContainerID: containerID,
	}

	if err != nil {
		result.Success = false
		if execCtx.Err() == context.DeadlineExceeded {
			result.Error = ErrTimeout.Error()
			result.ExitCode = -1
			return result, ErrTimeout
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			}
		}
		result.Error = err.Error()
		return result, nil
	}

	result.Success = true
	result.ExitCode = 0
	return result, nil
}

// Health checks if the container runtime is available.
func (e *Executor) Health(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, e.config.Runtime, "version")
	return cmd.Run()
}
