// Package provider implements the Chronos Terraform provider.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the Chronos API client.
type Client struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Chronos API client.
func NewClient(endpoint, apiKey string, timeoutSeconds int64) *Client {
	return &Client{
		endpoint: endpoint,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}
}

// Job represents a Chronos job.
type Job struct {
	ID          string            `json:"id,omitempty"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Schedule    string            `json:"schedule"`
	Timezone    string            `json:"timezone,omitempty"`
	Enabled     bool              `json:"enabled"`
	Webhook     *Webhook          `json:"webhook,omitempty"`
	RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	Concurrency string            `json:"concurrency,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
}

// Webhook defines the webhook configuration.
type Webhook struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// RetryPolicy defines retry behavior.
type RetryPolicy struct {
	MaxAttempts     int     `json:"max_attempts"`
	InitialInterval string  `json:"initial_interval"`
	MaxInterval     string  `json:"max_interval"`
	Multiplier      float64 `json:"multiplier"`
}

// DAG represents a Chronos DAG workflow.
type DAG struct {
	ID          string            `json:"id,omitempty"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Nodes       []DAGNode         `json:"nodes"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// DAGNode represents a node in a DAG.
type DAGNode struct {
	ID           string   `json:"id"`
	JobID        string   `json:"job_id"`
	Dependencies []string `json:"dependencies,omitempty"`
	Condition    string   `json:"condition,omitempty"`
}

// Trigger represents an event trigger.
type Trigger struct {
	ID      string            `json:"id,omitempty"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	JobID   string            `json:"job_id"`
	Config  map[string]string `json:"config,omitempty"`
	Enabled bool              `json:"enabled"`
}

// Namespace represents a tenant namespace.
type Namespace struct {
	ID          string            `json:"id,omitempty"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	MaxJobs     int               `json:"max_jobs,omitempty"`
}

// doRequest executes an HTTP request.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bodyReader)
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
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// CreateJob creates a new job.
func (c *Client) CreateJob(ctx context.Context, job *Job) (*Job, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/v1/jobs", job)
	if err != nil {
		return nil, err
	}

	var result Job
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// GetJob retrieves a job by ID.
func (c *Client) GetJob(ctx context.Context, id string) (*Job, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/jobs/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result Job
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// UpdateJob updates an existing job.
func (c *Client) UpdateJob(ctx context.Context, id string, job *Job) (*Job, error) {
	respBody, err := c.doRequest(ctx, http.MethodPut, "/api/v1/jobs/"+id, job)
	if err != nil {
		return nil, err
	}

	var result Job
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// DeleteJob deletes a job.
func (c *Client) DeleteJob(ctx context.Context, id string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/jobs/"+id, nil)
	return err
}

// ListJobs lists all jobs.
func (c *Client) ListJobs(ctx context.Context, namespace string) ([]Job, error) {
	path := "/api/v1/jobs"
	if namespace != "" {
		path += "?namespace=" + namespace
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return result.Jobs, nil
}

// CreateDAG creates a new DAG.
func (c *Client) CreateDAG(ctx context.Context, dag *DAG) (*DAG, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/v1/dags", dag)
	if err != nil {
		return nil, err
	}

	var result DAG
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// GetDAG retrieves a DAG by ID.
func (c *Client) GetDAG(ctx context.Context, id string) (*DAG, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/dags/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result DAG
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// UpdateDAG updates an existing DAG.
func (c *Client) UpdateDAG(ctx context.Context, id string, dag *DAG) (*DAG, error) {
	respBody, err := c.doRequest(ctx, http.MethodPut, "/api/v1/dags/"+id, dag)
	if err != nil {
		return nil, err
	}

	var result DAG
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// DeleteDAG deletes a DAG.
func (c *Client) DeleteDAG(ctx context.Context, id string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/dags/"+id, nil)
	return err
}

// CreateTrigger creates a new trigger.
func (c *Client) CreateTrigger(ctx context.Context, trigger *Trigger) (*Trigger, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/v1/triggers", trigger)
	if err != nil {
		return nil, err
	}

	var result Trigger
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// GetTrigger retrieves a trigger by ID.
func (c *Client) GetTrigger(ctx context.Context, id string) (*Trigger, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/triggers/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result Trigger
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// UpdateTrigger updates an existing trigger.
func (c *Client) UpdateTrigger(ctx context.Context, id string, trigger *Trigger) (*Trigger, error) {
	respBody, err := c.doRequest(ctx, http.MethodPut, "/api/v1/triggers/"+id, trigger)
	if err != nil {
		return nil, err
	}

	var result Trigger
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// DeleteTrigger deletes a trigger.
func (c *Client) DeleteTrigger(ctx context.Context, id string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/triggers/"+id, nil)
	return err
}

// CreateNamespace creates a new namespace.
func (c *Client) CreateNamespace(ctx context.Context, ns *Namespace) (*Namespace, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/v1/namespaces", ns)
	if err != nil {
		return nil, err
	}

	var result Namespace
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// GetNamespace retrieves a namespace by ID.
func (c *Client) GetNamespace(ctx context.Context, id string) (*Namespace, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/namespaces/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result Namespace
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// UpdateNamespace updates an existing namespace.
func (c *Client) UpdateNamespace(ctx context.Context, id string, ns *Namespace) (*Namespace, error) {
	respBody, err := c.doRequest(ctx, http.MethodPut, "/api/v1/namespaces/"+id, ns)
	if err != nil {
		return nil, err
	}

	var result Namespace
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// DeleteNamespace deletes a namespace.
func (c *Client) DeleteNamespace(ctx context.Context, id string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/namespaces/"+id, nil)
	return err
}

// CreateWorkflow creates a new workflow.
func (c *Client) CreateWorkflow(ctx context.Context, workflow *APIWorkflow) (*APIWorkflow, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/v1/workflows", workflow)
	if err != nil {
		return nil, err
	}

	var result APIWorkflow
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// GetWorkflow retrieves a workflow by ID.
func (c *Client) GetWorkflow(ctx context.Context, id string) (*APIWorkflow, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/workflows/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result APIWorkflow
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// UpdateWorkflow updates an existing workflow.
func (c *Client) UpdateWorkflow(ctx context.Context, id string, workflow *APIWorkflow) (*APIWorkflow, error) {
	respBody, err := c.doRequest(ctx, http.MethodPut, "/api/v1/workflows/"+id, workflow)
	if err != nil {
		return nil, err
	}

	var result APIWorkflow
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// DeleteWorkflow deletes a workflow.
func (c *Client) DeleteWorkflow(ctx context.Context, id string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/workflows/"+id, nil)
	return err
}

// CreatePolicy creates a new policy.
func (c *Client) CreatePolicy(ctx context.Context, policy *APIPolicy) (*APIPolicy, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, "/api/v1/policies", policy)
	if err != nil {
		return nil, err
	}

	var result APIPolicy
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// GetPolicy retrieves a policy by ID.
func (c *Client) GetPolicy(ctx context.Context, id string) (*APIPolicy, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, "/api/v1/policies/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result APIPolicy
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// UpdatePolicy updates an existing policy.
func (c *Client) UpdatePolicy(ctx context.Context, id string, policy *APIPolicy) (*APIPolicy, error) {
	respBody, err := c.doRequest(ctx, http.MethodPut, "/api/v1/policies/"+id, policy)
	if err != nil {
		return nil, err
	}

	var result APIPolicy
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &result, nil
}

// DeletePolicy deletes a policy.
func (c *Client) DeletePolicy(ctx context.Context, id string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/policies/"+id, nil)
	return err
}
