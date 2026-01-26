// Package azurefunc provides an Azure Functions executor for Chronos.
package azurefunc

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

// Executor executes jobs via Azure Functions.
type Executor struct {
	config     Config
	httpClient *http.Client
}

// Config configures the Azure Functions executor.
type Config struct {
	// FunctionURL is the Azure Function HTTP trigger URL.
	FunctionURL string `json:"function_url" yaml:"function_url"`
	// FunctionKey is the function-level API key.
	FunctionKey string `json:"function_key" yaml:"function_key"`
	// HostKey is the host-level API key.
	HostKey string `json:"host_key" yaml:"host_key"`
	// UseManagedIdentity enables managed identity authentication.
	UseManagedIdentity bool `json:"use_managed_identity" yaml:"use_managed_identity"`
	// Timeout for the invocation.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// New creates a new Azure Functions executor.
func New(cfg Config) *Executor {
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
		Name:        "azure-functions",
		Version:     "1.0.0",
		Type:        plugin.TypeExecutor,
		Description: "Azure Functions executor",
	}
}

// Init initializes the executor.
func (e *Executor) Init(ctx context.Context, config map[string]interface{}) error {
	if url, ok := config["function_url"].(string); ok {
		e.config.FunctionURL = url
	}
	if key, ok := config["function_key"].(string); ok {
		e.config.FunctionKey = key
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
	if e.config.FunctionURL == "" {
		return fmt.Errorf("function_url is required")
	}
	return nil
}

// Execute invokes the Azure Function.
func (e *Executor) Execute(ctx context.Context, req *plugin.ExecutionRequest) (*plugin.ExecutionResult, error) {
	start := time.Now()

	// Build the payload
	payload := AzurePayload{
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.config.FunctionURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return &plugin.ExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to create request: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Chronos-Job-ID", req.JobID)
	httpReq.Header.Set("X-Chronos-Execution-ID", req.ExecutionID)

	// Add authentication
	if e.config.FunctionKey != "" {
		httpReq.Header.Set("x-functions-key", e.config.FunctionKey)
	} else if e.config.HostKey != "" {
		httpReq.Header.Set("x-functions-key", e.config.HostKey)
	}

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return &plugin.ExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("azure function invocation failed: %v", err),
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
		result.Error = fmt.Sprintf("azure function returned status %d: %s", resp.StatusCode, string(body))
	}

	return result, nil
}

// AzurePayload is the payload sent to Azure Functions.
type AzurePayload struct {
	JobID       string                 `json:"job_id"`
	JobName     string                 `json:"job_name"`
	ExecutionID string                 `json:"execution_id"`
	Payload     map[string]interface{} `json:"payload"`
	Metadata    map[string]string      `json:"metadata"`
	Timestamp   string                 `json:"timestamp"`
}
