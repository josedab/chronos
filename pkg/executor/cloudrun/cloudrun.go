// Package cloudrun provides a Google Cloud Run executor for Chronos.
package cloudrun

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

// Executor executes jobs via Google Cloud Run.
type Executor struct {
	config     Config
	httpClient *http.Client
}

// Config configures the Cloud Run executor.
type Config struct {
	// ServiceURL is the Cloud Run service URL.
	ServiceURL string `json:"service_url" yaml:"service_url"`
	// Project is the GCP project ID.
	Project string `json:"project" yaml:"project"`
	// Region is the GCP region.
	Region string `json:"region" yaml:"region"`
	// ServiceName is the Cloud Run service name.
	ServiceName string `json:"service_name" yaml:"service_name"`
	// UseIDToken enables ID token authentication.
	UseIDToken bool `json:"use_id_token" yaml:"use_id_token"`
	// ServiceAccountEmail for authentication.
	ServiceAccountEmail string `json:"service_account_email" yaml:"service_account_email"`
	// Timeout for the invocation.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// New creates a new Cloud Run executor.
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
		Name:        "cloudrun",
		Version:     "1.0.0",
		Type:        plugin.TypeExecutor,
		Description: "Google Cloud Run executor",
	}
}

// Init initializes the executor.
func (e *Executor) Init(ctx context.Context, config map[string]interface{}) error {
	if url, ok := config["service_url"].(string); ok {
		e.config.ServiceURL = url
	}
	if project, ok := config["project"].(string); ok {
		e.config.Project = project
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
	if e.config.ServiceURL == "" {
		return fmt.Errorf("service_url is required")
	}
	return nil
}

// Execute invokes the Cloud Run service.
func (e *Executor) Execute(ctx context.Context, req *plugin.ExecutionRequest) (*plugin.ExecutionResult, error) {
	start := time.Now()

	// Build the payload
	payload := CloudRunPayload{
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.config.ServiceURL, bytes.NewReader(payloadBytes))
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

	// Note: In production, add ID token for authenticated Cloud Run services
	// using google.golang.org/api/idtoken package

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return &plugin.ExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("cloud run invocation failed: %v", err),
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
		result.Error = fmt.Sprintf("cloud run returned status %d: %s", resp.StatusCode, string(body))
	}

	return result, nil
}

// CloudRunPayload is the payload sent to Cloud Run services.
type CloudRunPayload struct {
	JobID       string                 `json:"job_id"`
	JobName     string                 `json:"job_name"`
	ExecutionID string                 `json:"execution_id"`
	Payload     map[string]interface{} `json:"payload"`
	Metadata    map[string]string      `json:"metadata"`
	Timestamp   string                 `json:"timestamp"`
}
