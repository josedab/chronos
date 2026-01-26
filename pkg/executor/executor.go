// Package executor provides execution backends for Chronos jobs.
package executor

import (
	"context"
	"time"

	"github.com/chronos/chronos/pkg/plugin"
)

// Type represents the type of executor.
type Type string

const (
	// TypeHTTP is the default HTTP webhook executor.
	TypeHTTP Type = "http"
	// TypeGRPC is the gRPC executor.
	TypeGRPC Type = "grpc"
	// TypeLambda is the AWS Lambda executor.
	TypeLambda Type = "lambda"
	// TypeDocker is the Docker container executor.
	TypeDocker Type = "docker"
	// TypeKubernetes is the Kubernetes Job executor.
	TypeKubernetes Type = "kubernetes"
)

// Config represents executor configuration.
type Config struct {
	Type     Type                   `json:"type" yaml:"type"`
	Timeout  time.Duration          `json:"timeout" yaml:"timeout"`
	Settings map[string]interface{} `json:"settings" yaml:"settings"`
}

// GRPCConfig configures the gRPC executor.
type GRPCConfig struct {
	// Target is the gRPC server address (host:port).
	Target string `json:"target" yaml:"target"`
	// Service is the gRPC service name to call.
	Service string `json:"service" yaml:"service"`
	// Method is the gRPC method name.
	Method string `json:"method" yaml:"method"`
	// UseTLS enables TLS for the connection.
	UseTLS bool `json:"use_tls" yaml:"use_tls"`
	// TLSCertFile is the path to the TLS certificate.
	TLSCertFile string `json:"tls_cert_file" yaml:"tls_cert_file"`
	// TLSKeyFile is the path to the TLS key.
	TLSKeyFile string `json:"tls_key_file" yaml:"tls_key_file"`
	// TLSCAFile is the path to the CA certificate.
	TLSCAFile string `json:"tls_ca_file" yaml:"tls_ca_file"`
	// Metadata is additional gRPC metadata to send.
	Metadata map[string]string `json:"metadata" yaml:"metadata"`
	// Timeout is the call timeout.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// LambdaConfig configures the AWS Lambda executor.
type LambdaConfig struct {
	// FunctionName is the Lambda function name or ARN.
	FunctionName string `json:"function_name" yaml:"function_name"`
	// Region is the AWS region.
	Region string `json:"region" yaml:"region"`
	// InvocationType is sync (RequestResponse) or async (Event).
	InvocationType string `json:"invocation_type" yaml:"invocation_type"`
	// Qualifier is the function version or alias.
	Qualifier string `json:"qualifier" yaml:"qualifier"`
}

// DockerConfig configures the Docker executor.
type DockerConfig struct {
	// Image is the Docker image to run.
	Image string `json:"image" yaml:"image"`
	// Command overrides the container command.
	Command []string `json:"command" yaml:"command"`
	// Env is environment variables.
	Env map[string]string `json:"env" yaml:"env"`
	// Volumes is volume mounts.
	Volumes []string `json:"volumes" yaml:"volumes"`
	// Network is the Docker network.
	Network string `json:"network" yaml:"network"`
	// Memory limit in bytes.
	Memory int64 `json:"memory" yaml:"memory"`
	// CPUs limit.
	CPUs float64 `json:"cpus" yaml:"cpus"`
	// AutoRemove removes container after execution.
	AutoRemove bool `json:"auto_remove" yaml:"auto_remove"`
}

// KubernetesConfig configures the Kubernetes Job executor.
type KubernetesConfig struct {
	// Namespace is the Kubernetes namespace.
	Namespace string `json:"namespace" yaml:"namespace"`
	// Image is the container image.
	Image string `json:"image" yaml:"image"`
	// Command is the container command.
	Command []string `json:"command" yaml:"command"`
	// Args are command arguments.
	Args []string `json:"args" yaml:"args"`
	// Env is environment variables.
	Env map[string]string `json:"env" yaml:"env"`
	// ServiceAccount is the pod service account.
	ServiceAccount string `json:"service_account" yaml:"service_account"`
	// BackoffLimit is the job backoff limit.
	BackoffLimit int32 `json:"backoff_limit" yaml:"backoff_limit"`
	// TTLSecondsAfterFinished cleans up completed jobs.
	TTLSecondsAfterFinished int32 `json:"ttl_seconds_after_finished" yaml:"ttl_seconds_after_finished"`
	// Resources specifies resource requests/limits.
	Resources *ResourceRequirements `json:"resources" yaml:"resources"`
}

// ResourceRequirements specifies compute resources.
type ResourceRequirements struct {
	Requests ResourceList `json:"requests" yaml:"requests"`
	Limits   ResourceList `json:"limits" yaml:"limits"`
}

// ResourceList is a map of resource name to quantity.
type ResourceList struct {
	CPU    string `json:"cpu" yaml:"cpu"`
	Memory string `json:"memory" yaml:"memory"`
}

// Factory creates executors by type.
type Factory interface {
	// Create creates an executor of the given type with config.
	Create(executorType Type, config map[string]interface{}) (plugin.Executor, error)
	// ListTypes returns available executor types.
	ListTypes() []Type
}

// Manager manages executor instances.
type Manager struct {
	registry *plugin.Registry
	factory  Factory
}

// NewManager creates a new executor manager.
func NewManager(registry *plugin.Registry, factory Factory) *Manager {
	return &Manager{
		registry: registry,
		factory:  factory,
	}
}

// Execute runs a job using the appropriate executor.
func (m *Manager) Execute(ctx context.Context, executorType Type, req *plugin.ExecutionRequest) (*plugin.ExecutionResult, error) {
	executor, err := m.registry.GetExecutor(string(executorType))
	if err != nil {
		return nil, err
	}
	return executor.Execute(ctx, req)
}
