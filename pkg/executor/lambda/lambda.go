// Package lambda provides an AWS Lambda executor for Chronos.
package lambda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/chronos/chronos/pkg/plugin"
)

// Executor executes jobs via AWS Lambda.
type Executor struct {
	config     Config
	httpClient *http.Client
}

// Config configures the Lambda executor.
type Config struct {
	// Region is the AWS region.
	Region string `json:"region" yaml:"region"`
	// FunctionName is the Lambda function name or ARN.
	FunctionName string `json:"function_name" yaml:"function_name"`
	// InvocationType: "RequestResponse" (sync) or "Event" (async).
	InvocationType string `json:"invocation_type" yaml:"invocation_type"`
	// Qualifier is the function version or alias.
	Qualifier string `json:"qualifier" yaml:"qualifier"`
	// AccessKeyID for AWS authentication (optional, uses default chain if empty).
	AccessKeyID string `json:"access_key_id" yaml:"access_key_id"`
	// SecretAccessKey for AWS authentication.
	SecretAccessKey string `json:"secret_access_key" yaml:"secret_access_key"`
	// RoleARN for assuming a role.
	RoleARN string `json:"role_arn" yaml:"role_arn"`
	// Timeout for the invocation.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// New creates a new Lambda executor.
func New(cfg Config) *Executor {
	if cfg.InvocationType == "" {
		cfg.InvocationType = "RequestResponse"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Executor{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Metadata returns plugin metadata.
func (e *Executor) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "lambda",
		Version:     "1.0.0",
		Type:        plugin.TypeExecutor,
		Description: "AWS Lambda function executor",
	}
}

// Init initializes the executor.
func (e *Executor) Init(ctx context.Context, config map[string]interface{}) error {
	if region, ok := config["region"].(string); ok {
		e.config.Region = region
	}
	if fn, ok := config["function_name"].(string); ok {
		e.config.FunctionName = fn
	}
	return nil
}

// Close cleans up resources.
func (e *Executor) Close(ctx context.Context) error {
	e.httpClient.CloseIdleConnections()
	return nil
}

// Health checks executor health.
func (e *Executor) Health(ctx context.Context) error {
	if e.config.FunctionName == "" {
		return fmt.Errorf("function_name is required")
	}
	if e.config.Region == "" {
		return fmt.Errorf("region is required")
	}
	return nil
}

// Execute invokes the Lambda function.
func (e *Executor) Execute(ctx context.Context, req *plugin.ExecutionRequest) (*plugin.ExecutionResult, error) {
	start := time.Now()

	// Build the Lambda payload
	payload := LambdaPayload{
		JobID:       req.JobID,
		JobName:     req.JobName,
		ExecutionID: req.ExecutionID,
		Payload:     req.Payload,
		Metadata:    req.Metadata,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return &plugin.ExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to marshal payload: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Build Lambda invoke URL (simplified - in production use AWS SDK)
	invokeURL := fmt.Sprintf(
		"https://lambda.%s.amazonaws.com/2015-03-31/functions/%s/invocations",
		e.config.Region,
		e.config.FunctionName,
	)

	if e.config.Qualifier != "" {
		invokeURL += "?Qualifier=" + e.config.Qualifier
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", invokeURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return &plugin.ExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to create request: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Amz-Invocation-Type", e.config.InvocationType)

	// Note: In production, use AWS SDK v2 with proper signing
	// This is a simplified implementation for demonstration

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return &plugin.ExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("lambda invocation failed: %v", err),
			Duration: time.Since(start),
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	result := &plugin.ExecutionResult{
		StatusCode: resp.StatusCode,
		Response:   body,
		Duration:   time.Since(start),
		Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
	}

	if !result.Success {
		result.Error = fmt.Sprintf("lambda returned status %d: %s", resp.StatusCode, string(body))
	}

	// Check for function error header
	if funcErr := resp.Header.Get("X-Amz-Function-Error"); funcErr != "" {
		result.Success = false
		result.Error = fmt.Sprintf("lambda function error: %s - %s", funcErr, string(body))
	}

	return result, nil
}

// LambdaPayload is the payload sent to Lambda functions.
type LambdaPayload struct {
	JobID       string                 `json:"job_id"`
	JobName     string                 `json:"job_name"`
	ExecutionID string                 `json:"execution_id"`
	Payload     map[string]interface{} `json:"payload"`
	Metadata    map[string]string      `json:"metadata"`
	Timestamp   string                 `json:"timestamp"`
}

// LambdaResponse is the expected response structure.
type LambdaResponse struct {
	StatusCode int                    `json:"statusCode"`
	Body       string                 `json:"body"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
