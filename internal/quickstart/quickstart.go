// Package quickstart provides an interactive getting-started experience for Chronos.
// It starts an echo webhook server and creates demo jobs to demonstrate core features.
package quickstart

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Config configures the quickstart experience.
type Config struct {
	// ChronosURL is the Chronos server URL.
	ChronosURL string
	// EchoPort is the port for the built-in echo server. 0 = auto-detect.
	EchoPort int
}

// DemoJob defines a job to create during quickstart.
type DemoJob struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Schedule    string            `json:"schedule"`
	Webhook     map[string]string `json:"webhook"`
	Enabled     bool              `json:"enabled"`
}

// EchoServer is a simple webhook receiver that logs requests.
type EchoServer struct {
	server   *http.Server
	port     int
	requests []EchoRequest
}

// EchoRequest records a received webhook call.
type EchoRequest struct {
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
	Timestamp time.Time         `json:"timestamp"`
}

// NewEchoServer creates a new echo webhook server.
func NewEchoServer(port int) (*EchoServer, error) {
	if port == 0 {
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			return nil, fmt.Errorf("finding available port: %w", err)
		}
		port = listener.Addr().(*net.TCPAddr).Port
		listener.Close()
	}

	es := &EchoServer{port: port}

	mux := http.NewServeMux()
	mux.HandleFunc("/", es.handleRequest)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	es.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return es, nil
}

// Start starts the echo server in the background.
func (es *EchoServer) Start() error {
	listener, err := net.Listen("tcp", es.server.Addr)
	if err != nil {
		return fmt.Errorf("starting echo server: %w", err)
	}

	go es.server.Serve(listener)
	return nil
}

// Stop stops the echo server.
func (es *EchoServer) Stop(ctx context.Context) error {
	return es.server.Shutdown(ctx)
}

// Port returns the server port.
func (es *EchoServer) Port() int {
	return es.port
}

// URL returns the server base URL.
func (es *EchoServer) URL() string {
	return fmt.Sprintf("http://localhost:%d", es.port)
}

// Requests returns all received requests.
func (es *EchoServer) Requests() []EchoRequest {
	return es.requests
}

func (es *EchoServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	es.requests = append(es.requests, EchoRequest{
		Method:    r.Method,
		Path:      r.URL.Path,
		Headers:   headers,
		Body:      string(body),
		Timestamp: time.Now(),
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"echo":      true,
		"method":    r.Method,
		"path":      r.URL.Path,
		"body_size": len(body),
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// DemoJobs returns the list of demo jobs to create.
func DemoJobs(echoURL string) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "hello-world",
			"description": "Simple health check — runs every minute",
			"schedule":    "* * * * *",
			"webhook": map[string]interface{}{
				"url":    echoURL + "/hello",
				"method": "GET",
			},
			"enabled": true,
		},
		{
			"name":        "daily-report",
			"description": "Daily report generation — runs at 9 AM",
			"schedule":    "0 9 * * *",
			"webhook": map[string]interface{}{
				"url":    echoURL + "/reports/generate",
				"method": "POST",
				"body":   `{"type":"daily","format":"pdf"}`,
			},
			"retry_policy": map[string]interface{}{
				"max_attempts":     3,
				"initial_interval": "1s",
				"max_interval":     "30s",
				"multiplier":       2.0,
			},
			"timeout": "5m",
			"enabled": true,
		},
		{
			"name":        "cleanup-job",
			"description": "Database cleanup — runs every 6 hours",
			"schedule":    "0 */6 * * *",
			"webhook": map[string]interface{}{
				"url":    echoURL + "/cleanup",
				"method": "POST",
				"body":   `{"tables":["sessions","temp_data"],"older_than":"7d"}`,
			},
			"timeout": "10m",
			"enabled": true,
		},
	}
}

// CreateDemoJob creates a job via the Chronos API.
func CreateDemoJob(ctx context.Context, chronosURL string, job map[string]interface{}) (string, error) {
	body, err := json.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("marshaling job: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", chronosURL+"/api/v1/jobs", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("creating job: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if data, ok := result["data"].(map[string]interface{}); ok {
		if id, ok := data["id"].(string); ok {
			return id, nil
		}
	}

	return "", nil
}

// RunScript returns the formatted quickstart output text.
func RunScript(echoURL, chronosURL string, jobIDs []string) string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════╗
║                 CHRONOS QUICKSTART                       ║
╠══════════════════════════════════════════════════════════╣
║                                                          ║
║  Echo server:    %s                      ║
║  Chronos API:    %s                      ║
║  Web UI:         %s                      ║
║                                                          ║
║  Created %d demo jobs:                                    ║
║    • hello-world     (every minute)                      ║
║    • daily-report    (9 AM daily)                        ║
║    • cleanup-job     (every 6 hours)                     ║
║                                                          ║
║  Try these commands:                                     ║
║    chronosctl job list                                   ║
║    chronosctl job trigger %s                ║
║    curl %s/api/v1/jobs        ║
║                                                          ║
║  Press Ctrl+C to stop                                    ║
╚══════════════════════════════════════════════════════════╝
`, echoURL, chronosURL, chronosURL, len(jobIDs),
		truncID(jobIDs), chronosURL)
}

func truncID(ids []string) string {
	if len(ids) == 0 {
		return "<job-id>"
	}
	id := ids[0]
	if len(id) > 20 {
		id = id[:8] + "..."
	}
	return id
}
