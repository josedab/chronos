// Package sdk provides a client for connecting to a Chronos server.
package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a Chronos API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	namespace  string
}

// ClientOption configures the client.
type ClientOption func(*Client)

// WithAPIKey sets the API key for authentication.
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// WithNamespace sets the default namespace.
func WithNamespace(ns string) ClientOption {
	return func(c *Client) {
		c.namespace = ns
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// NewClient creates a new Chronos API client.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		namespace: "default",
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// JobDefinition is a job definition for the API.
type JobDefinition struct {
	Name        string            `json:"name"`
	Schedule    string            `json:"schedule"`
	Timezone    string            `json:"timezone,omitempty"`
	WebhookURL  string            `json:"webhook_url"`
	Method      string            `json:"method,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
}

// RetryPolicy defines retry behavior.
type RetryPolicy struct {
	MaxAttempts     int     `json:"max_attempts"`
	InitialInterval string  `json:"initial_interval,omitempty"`
	MaxInterval     string  `json:"max_interval,omitempty"`
	Multiplier      float64 `json:"multiplier,omitempty"`
}

// JobResponse is the response for job operations.
type JobResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Schedule    string            `json:"schedule"`
	Timezone    string            `json:"timezone"`
	Status      string            `json:"status"`
	WebhookURL  string            `json:"webhook_url"`
	Method      string            `json:"method"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastRun     *time.Time        `json:"last_run,omitempty"`
	NextRun     *time.Time        `json:"next_run,omitempty"`
	RunCount    int64             `json:"run_count"`
	ErrorCount  int64             `json:"error_count"`
}

// ExecutionResponse is the response for execution operations.
type ExecutionResponse struct {
	ID          string        `json:"id"`
	JobID       string        `json:"job_id"`
	Status      string        `json:"status"`
	StatusCode  int           `json:"status_code"`
	Duration    time.Duration `json:"duration"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Error       string        `json:"error,omitempty"`
	Attempt     int           `json:"attempt"`
}

// APIError represents an API error.
type APIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s (status %d)", e.Code, e.Message, e.StatusCode)
}

// CreateJob creates a new job.
func (c *Client) CreateJob(ctx context.Context, job *JobDefinition) (*JobResponse, error) {
	if job.Namespace == "" {
		job.Namespace = c.namespace
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v1/jobs", job)
	if err != nil {
		return nil, err
	}

	var result JobResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetJob retrieves a job by ID.
func (c *Client) GetJob(ctx context.Context, jobID string) (*JobResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/jobs/"+jobID, nil)
	if err != nil {
		return nil, err
	}

	var result JobResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListJobs lists all jobs.
func (c *Client) ListJobs(ctx context.Context) ([]*JobResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/jobs", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Jobs []*JobResponse `json:"jobs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Jobs, nil
}

// UpdateJob updates a job.
func (c *Client) UpdateJob(ctx context.Context, jobID string, job *JobDefinition) (*JobResponse, error) {
	resp, err := c.doRequest(ctx, "PUT", "/api/v1/jobs/"+jobID, job)
	if err != nil {
		return nil, err
	}

	var result JobResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteJob deletes a job.
func (c *Client) DeleteJob(ctx context.Context, jobID string) error {
	_, err := c.doRequest(ctx, "DELETE", "/api/v1/jobs/"+jobID, nil)
	return err
}

// PauseJob pauses a job.
func (c *Client) PauseJob(ctx context.Context, jobID string) error {
	_, err := c.doRequest(ctx, "POST", "/api/v1/jobs/"+jobID+"/pause", nil)
	return err
}

// ResumeJob resumes a paused job.
func (c *Client) ResumeJob(ctx context.Context, jobID string) error {
	_, err := c.doRequest(ctx, "POST", "/api/v1/jobs/"+jobID+"/resume", nil)
	return err
}

// TriggerJob triggers immediate execution of a job.
func (c *Client) TriggerJob(ctx context.Context, jobID string) (*ExecutionResponse, error) {
	resp, err := c.doRequest(ctx, "POST", "/api/v1/jobs/"+jobID+"/trigger", nil)
	if err != nil {
		return nil, err
	}

	var result ExecutionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetExecution retrieves an execution by ID.
func (c *Client) GetExecution(ctx context.Context, executionID string) (*ExecutionResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/executions/"+executionID, nil)
	if err != nil {
		return nil, err
	}

	var result ExecutionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListExecutions lists executions for a job.
func (c *Client) ListExecutions(ctx context.Context, jobID string, limit int) ([]*ExecutionResponse, error) {
	path := fmt.Sprintf("/api/v1/jobs/%s/executions?limit=%d", jobID, limit)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Executions []*ExecutionResponse `json:"executions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Executions, nil
}

// Health checks the server health.
func (c *Client) Health(ctx context.Context) error {
	_, err := c.doRequest(ctx, "GET", "/health", nil)
	return err
}

// ClusterStatus returns the cluster status.
func (c *Client) ClusterStatus(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/cluster/status", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var apiErr APIError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Code:       "unknown",
				Message:    fmt.Sprintf("request failed with status %d", resp.StatusCode),
			}
		}
		apiErr.StatusCode = resp.StatusCode
		return nil, &apiErr
	}

	return resp, nil
}
