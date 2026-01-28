// Package studio provides webhook testing and debugging capabilities.
package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrInvalidURL     = errors.New("invalid URL")
	ErrRequestFailed  = errors.New("request failed")
	ErrTimeout        = errors.New("request timeout")
)

// TestRequest represents a webhook test request.
type TestRequest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	FollowRedirects bool          `json:"follow_redirects"`
}

// TestResponse represents the webhook test response.
type TestResponse struct {
	ID            string            `json:"id"`
	RequestID     string            `json:"request_id"`
	StatusCode    int               `json:"status_code"`
	Status        string            `json:"status"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	BodySize      int               `json:"body_size"`
	ContentType   string            `json:"content_type"`
	
	// Timing
	Duration      time.Duration     `json:"duration"`
	DNSTime       time.Duration     `json:"dns_time,omitempty"`
	ConnectTime   time.Duration     `json:"connect_time,omitempty"`
	TLSTime       time.Duration     `json:"tls_time,omitempty"`
	FirstByteTime time.Duration     `json:"first_byte_time,omitempty"`
	
	// Result
	Success       bool              `json:"success"`
	Error         string            `json:"error,omitempty"`
	
	// Request echo
	RequestEcho   *RequestEcho      `json:"request_echo,omitempty"`
	
	Timestamp     time.Time         `json:"timestamp"`
}

// RequestEcho contains the echoed request details.
type RequestEcho struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body,omitempty"`
}

// TemplateGenerator helps generate job configurations.
type TemplateGenerator struct{}

// GeneratedJob is the generated job configuration.
type GeneratedJob struct {
	Name        string            `json:"name"`
	Schedule    string            `json:"schedule"`
	Webhook     WebhookConfig     `json:"webhook"`
	Timeout     string            `json:"timeout,omitempty"`
	Concurrency string            `json:"concurrency,omitempty"`
	RetryPolicy *RetryPolicy      `json:"retry_policy,omitempty"`
}

// WebhookConfig is the webhook configuration.
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// RetryPolicy is the retry configuration.
type RetryPolicy struct {
	MaxAttempts     int     `json:"max_attempts"`
	InitialInterval string  `json:"initial_interval"`
	MaxInterval     string  `json:"max_interval"`
	Multiplier      float64 `json:"multiplier"`
}

// ValidationResult contains URL validation results.
type ValidationResult struct {
	Valid       bool              `json:"valid"`
	URL         string            `json:"url"`
	ParsedURL   *ParsedURL        `json:"parsed_url,omitempty"`
	Warnings    []string          `json:"warnings,omitempty"`
	Errors      []string          `json:"errors,omitempty"`
	Suggestions []string          `json:"suggestions,omitempty"`
}

// ParsedURL contains parsed URL components.
type ParsedURL struct {
	Scheme   string            `json:"scheme"`
	Host     string            `json:"host"`
	Port     string            `json:"port,omitempty"`
	Path     string            `json:"path"`
	Query    map[string]string `json:"query,omitempty"`
	Fragment string            `json:"fragment,omitempty"`
}

// Studio provides webhook testing capabilities.
type Studio struct {
	mu      sync.RWMutex
	history []*TestResponse
	client  *http.Client
}

// NewStudio creates a new webhook studio.
func NewStudio() *Studio {
	return &Studio{
		history: make([]*TestResponse, 0),
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
	}
}

// Test executes a webhook test.
func (s *Studio) Test(ctx context.Context, req *TestRequest) (*TestResponse, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.Timeout == 0 {
		req.Timeout = 30 * time.Second
	}

	// Validate URL
	validation := s.ValidateURL(req.URL)
	if !validation.Valid {
		return &TestResponse{
			ID:        uuid.New().String(),
			RequestID: req.ID,
			Success:   false,
			Error:     strings.Join(validation.Errors, "; "),
			Timestamp: time.Now(),
		}, ErrInvalidURL
	}

	// Create request
	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return nil, err
	}

	// Set headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Set default headers if not provided
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", "Chronos-WebhookStudio/1.0")
	}
	if req.Body != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Configure client
	client := &http.Client{
		Timeout: req.Timeout,
	}
	if !req.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// Execute request
	start := time.Now()
	resp, err := client.Do(httpReq)
	duration := time.Since(start)

	response := &TestResponse{
		ID:        uuid.New().String(),
		RequestID: req.ID,
		Duration:  duration,
		Timestamp: time.Now(),
		RequestEcho: &RequestEcho{
			Method:  req.Method,
			URL:     req.URL,
			Headers: req.Headers,
			Body:    req.Body,
		},
	}

	if err != nil {
		response.Success = false
		response.Error = err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			response.Error = "request timeout"
		}
		s.addToHistory(response)
		return response, nil
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		response.Error = "failed to read response: " + err.Error()
		s.addToHistory(response)
		return response, nil
	}

	// Build response
	response.StatusCode = resp.StatusCode
	response.Status = resp.Status
	response.Body = string(respBody)
	response.BodySize = len(respBody)
	response.ContentType = resp.Header.Get("Content-Type")
	response.Success = resp.StatusCode >= 200 && resp.StatusCode < 300

	// Copy headers
	response.Headers = make(map[string]string)
	for k := range resp.Header {
		response.Headers[k] = resp.Header.Get(k)
	}

	s.addToHistory(response)
	return response, nil
}

// ValidateURL validates a webhook URL.
func (s *Studio) ValidateURL(rawURL string) *ValidationResult {
	result := &ValidationResult{
		URL:      rawURL,
		Warnings: make([]string, 0),
		Errors:   make([]string, 0),
	}

	// Parse URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		result.Errors = append(result.Errors, "invalid URL format: "+err.Error())
		return result
	}

	// Check scheme
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		result.Errors = append(result.Errors, "URL must use http or https scheme")
	}

	// Check host
	if parsed.Host == "" {
		result.Errors = append(result.Errors, "URL must have a host")
	}

	// Build parsed URL info
	result.ParsedURL = &ParsedURL{
		Scheme:   parsed.Scheme,
		Host:     parsed.Hostname(),
		Port:     parsed.Port(),
		Path:     parsed.Path,
		Fragment: parsed.Fragment,
	}

	// Parse query params
	if parsed.RawQuery != "" {
		result.ParsedURL.Query = make(map[string]string)
		for k, v := range parsed.Query() {
			if len(v) > 0 {
				result.ParsedURL.Query[k] = v[0]
			}
		}
	}

	// Warnings
	if parsed.Scheme == "http" {
		result.Warnings = append(result.Warnings, "using HTTP instead of HTTPS is not recommended for production")
	}

	if strings.Contains(parsed.Host, "localhost") || strings.HasPrefix(parsed.Host, "127.") {
		result.Warnings = append(result.Warnings, "localhost URLs will not work in production")
	}

	// Suggestions
	if parsed.Path == "" || parsed.Path == "/" {
		result.Suggestions = append(result.Suggestions, "consider adding a path to make the endpoint more specific")
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// GenerateJobConfig generates a job configuration from a test request.
func (s *Studio) GenerateJobConfig(req *TestRequest, schedule string) *GeneratedJob {
	if schedule == "" {
		schedule = "0 * * * *" // Every hour
	}

	job := &GeneratedJob{
		Name:     req.Name,
		Schedule: schedule,
		Webhook: WebhookConfig{
			URL:     req.URL,
			Method:  req.Method,
			Headers: req.Headers,
			Body:    req.Body,
		},
		Timeout:     "30s",
		Concurrency: "forbid",
	}

	if job.Name == "" {
		// Generate name from URL
		parsed, _ := url.Parse(req.URL)
		if parsed != nil {
			job.Name = strings.ReplaceAll(parsed.Host, ".", "-") + "-job"
		} else {
			job.Name = "webhook-job"
		}
	}

	return job
}

// FormatBody formats a JSON body for display.
func (s *Studio) FormatBody(body string) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return body, nil // Return as-is if not JSON
	}

	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return body, err
	}
	return string(formatted), nil
}

// ParseCurl parses a curl command into a test request.
func (s *Studio) ParseCurl(curlCommand string) (*TestRequest, error) {
	req := &TestRequest{
		ID:      uuid.New().String(),
		Method:  "GET",
		Headers: make(map[string]string),
	}

	// Parse curl command with proper quote handling
	parts := parseCurlArgs(curlCommand)
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		switch {
		case part == "-X" || part == "--request":
			if i+1 < len(parts) {
				req.Method = strings.ToUpper(parts[i+1])
				i++
			}
		case part == "-H" || part == "--header":
			if i+1 < len(parts) {
				header := parts[i+1]
				if idx := strings.Index(header, ":"); idx > 0 {
					key := strings.TrimSpace(header[:idx])
					value := strings.TrimSpace(header[idx+1:])
					req.Headers[key] = value
				}
				i++
			}
		case part == "-d" || part == "--data" || part == "--data-raw":
			if i+1 < len(parts) {
				req.Body = parts[i+1]
				i++
			}
		case strings.HasPrefix(part, "http://") || strings.HasPrefix(part, "https://"):
			req.URL = part
		}
	}

	if req.URL == "" {
		return nil, errors.New("no URL found in curl command")
	}

	return req, nil
}

// parseCurlArgs tokenizes a curl command respecting quotes.
func parseCurlArgs(cmd string) []string {
	var parts []string
	var current strings.Builder
	var inQuote rune
	escaped := false

	for _, r := range cmd {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '"' || r == '\'' {
			inQuote = r
			continue
		}
		if r == ' ' || r == '\t' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// GetHistory returns the test history.
func (s *Studio) GetHistory(limit int) []*TestResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}

	// Return most recent first
	result := make([]*TestResponse, limit)
	for i := 0; i < limit; i++ {
		result[i] = s.history[len(s.history)-1-i]
	}
	return result
}

// addToHistory adds a response to history.
func (s *Studio) addToHistory(resp *TestResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.history = append(s.history, resp)

	// Keep last 100 entries
	if len(s.history) > 100 {
		s.history = s.history[len(s.history)-100:]
	}
}

// ClearHistory clears the test history.
func (s *Studio) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = make([]*TestResponse, 0)
}

// CompareResponses compares two test responses.
func (s *Studio) CompareResponses(a, b *TestResponse) map[string]interface{} {
	diff := make(map[string]interface{})

	if a.StatusCode != b.StatusCode {
		diff["status_code"] = map[string]int{"a": a.StatusCode, "b": b.StatusCode}
	}

	if a.Duration != b.Duration {
		diff["duration"] = map[string]string{
			"a": a.Duration.String(),
			"b": b.Duration.String(),
		}
	}

	if a.Body != b.Body {
		diff["body_differs"] = true
		diff["body_a_size"] = a.BodySize
		diff["body_b_size"] = b.BodySize
	}

	// Compare headers
	headerDiff := make(map[string]interface{})
	allHeaders := make(map[string]bool)
	for k := range a.Headers {
		allHeaders[k] = true
	}
	for k := range b.Headers {
		allHeaders[k] = true
	}

	for k := range allHeaders {
		va, oka := a.Headers[k]
		vb, okb := b.Headers[k]
		if va != vb || oka != okb {
			headerDiff[k] = map[string]interface{}{
				"a": va,
				"b": vb,
			}
		}
	}
	if len(headerDiff) > 0 {
		diff["headers"] = headerDiff
	}

	return diff
}

// HealthCheck performs a quick health check on a URL.
func (s *Studio) HealthCheck(ctx context.Context, url string) (bool, time.Duration, error) {
	req := &TestRequest{
		Method:  "GET",
		URL:     url,
		Timeout: 10 * time.Second,
	}

	resp, err := s.Test(ctx, req)
	if err != nil {
		return false, 0, err
	}

	return resp.Success, resp.Duration, nil
}

// SuggestHeaders returns suggested headers based on URL patterns.
func (s *Studio) SuggestHeaders(urlStr string) map[string]string {
	suggestions := map[string]string{
		"User-Agent":   "Chronos/1.0",
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}

	// Add specific suggestions based on URL
	parsed, _ := url.Parse(urlStr)
	if parsed != nil {
		host := parsed.Host
		
		if strings.Contains(host, "github.com") || strings.Contains(host, "api.github.com") {
			suggestions["Accept"] = "application/vnd.github+json"
			suggestions["X-GitHub-Api-Version"] = "2022-11-28"
		}
		
		if strings.Contains(host, "slack.com") {
			suggestions["Content-Type"] = "application/json"
		}
	}

	return suggestions
}

var _ = bytes.Buffer{} // Used for body reader
