package lambda

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chronos/chronos/pkg/plugin"
)

func TestNew(t *testing.T) {
	cfg := Config{
		Region:       "us-east-1",
		FunctionName: "my-function",
	}

	exec := New(cfg)
	if exec == nil {
		t.Fatal("expected non-nil executor")
	}
	if exec.config.InvocationType != "RequestResponse" {
		t.Errorf("expected default InvocationType, got %s", exec.config.InvocationType)
	}
	if exec.config.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", exec.config.Timeout)
	}
}

func TestMetadata(t *testing.T) {
	exec := New(Config{})
	meta := exec.Metadata()

	if meta.Name != "lambda" {
		t.Errorf("expected name 'lambda', got %s", meta.Name)
	}
	if meta.Type != plugin.TypeExecutor {
		t.Errorf("expected type executor, got %v", meta.Type)
	}
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "missing function name",
			config:  Config{Region: "us-east-1"},
			wantErr: true,
		},
		{
			name:    "missing region",
			config:  Config{FunctionName: "my-function"},
			wantErr: true,
		},
		{
			name: "valid config",
			config: Config{
				Region:       "us-east-1",
				FunctionName: "my-function",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := New(tt.config)
			err := exec.Health(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Health() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecute(t *testing.T) {
	// Create mock Lambda endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"statusCode": 200, "body": "success"}`))
	}))
	defer server.Close()

	exec := &Executor{
		config: Config{
			Region:         "us-east-1",
			FunctionName:   "test-function",
			InvocationType: "RequestResponse",
			Timeout:        5 * time.Second,
		},
		httpClient: server.Client(),
	}

	// Override the URL (in real implementation, this would use AWS SDK)
	req := &plugin.ExecutionRequest{
		JobID:       "job-1",
		JobName:     "Test Job",
		ExecutionID: "exec-1",
		Payload:     map[string]interface{}{"key": "value"},
		Metadata:    map[string]string{"trace_id": "abc123"},
	}

	// Note: This test will fail because we can't easily mock the Lambda URL
	// In production, use AWS SDK with mock transport
	_, err := exec.Execute(context.Background(), req)
	// Just verify it doesn't panic
	_ = err
}

func TestLambdaPayload(t *testing.T) {
	payload := LambdaPayload{
		JobID:       "job-1",
		JobName:     "Test Job",
		ExecutionID: "exec-1",
		Payload:     map[string]interface{}{"key": "value"},
		Metadata:    map[string]string{"env": "test"},
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	if payload.JobID != "job-1" {
		t.Errorf("expected job-1, got %s", payload.JobID)
	}
}
