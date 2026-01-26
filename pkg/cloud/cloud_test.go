package cloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewAWSClient(t *testing.T) {
	// Test credentials - not real AWS keys
	cfg := AWSConfig{
		Region:          "us-east-1",
		AccessKeyID:     "TESTKEY1234567890123",
		SecretAccessKey: "TESTSECRETKEYabcdefghij1234567890ABCDEF",
	}

	client := NewAWSClient(cfg)

	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if client.config.Region != "us-east-1" {
		t.Errorf("expected region us-east-1, got %s", client.config.Region)
	}
}

func TestAWSClient_InvokeLambda(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type")
		}
		if r.Header.Get("X-Amz-Invocation-Type") != "RequestResponse" {
			t.Errorf("expected RequestResponse invocation type")
		}
		w.Header().Set("X-Amz-Executed-Version", "$LATEST")
		w.WriteHeader(200)
		w.Write([]byte(`{"result": "success"}`))
	}))
	defer server.Close()

	// Test credentials - not real AWS keys
	cfg := AWSConfig{
		Region:          "us-east-1",
		AccessKeyID:     "TESTKEY1234567890123",
		SecretAccessKey: "TESTSECRETKEYabcdefghij1234567890ABCDEF",
	}
	client := NewAWSClient(cfg)

	// Override endpoint for testing
	originalHTTPClient := client.httpClient
	client.httpClient = &http.Client{
		Transport: &testTransport{
			server: server,
		},
	}
	defer func() { client.httpClient = originalHTTPClient }()

	result, err := client.InvokeLambda(context.Background(), "my-function", []byte(`{"test": true}`), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
	if result.ExecutedVersion != "$LATEST" {
		t.Errorf("expected version $LATEST, got %s", result.ExecutedVersion)
	}
}

func TestNewGCPClient(t *testing.T) {
	cfg := GCPConfig{
		ProjectID: "my-project",
		Region:    "us-central1",
	}

	client := NewGCPClient(cfg)

	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if client.config.ProjectID != "my-project" {
		t.Errorf("expected project my-project, got %s", client.config.ProjectID)
	}
}

func TestGCPClient_InvokeCloudRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	cfg := GCPConfig{
		ProjectID: "my-project",
		Region:    "us-central1",
	}
	client := NewGCPClient(cfg)

	result, err := client.InvokeCloudRun(context.Background(), server.URL, []byte(`{"test": true}`), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
	if string(result.Body) != `{"status": "ok"}` {
		t.Errorf("unexpected body: %s", string(result.Body))
	}
}

func TestGCPClient_InvokeCloudFunction(t *testing.T) {
	cfg := GCPConfig{
		ProjectID: "my-project",
		Region:    "us-central1",
	}
	client := NewGCPClient(cfg)

	if client.config.Region != "us-central1" {
		t.Errorf("expected region us-central1, got %s", client.config.Region)
	}
}

func TestNewAzureClient(t *testing.T) {
	cfg := AzureConfig{
		SubscriptionID: "sub-123",
		ResourceGroup:  "my-rg",
		TenantID:       "tenant-abc",
		ClientID:       "client-def",
		ClientSecret:   "secret-xyz",
	}

	client := NewAzureClient(cfg)

	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if client.config.SubscriptionID != "sub-123" {
		t.Errorf("expected subscription sub-123, got %s", client.config.SubscriptionID)
	}
}

func TestAzureClient_InvokeFunction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"result": "azure-success"}`))
	}))
	defer server.Close()

	cfg := AzureConfig{
		SubscriptionID: "sub-123",
		ResourceGroup:  "my-rg",
	}
	client := NewAzureClient(cfg)

	result, err := client.InvokeFunction(context.Background(), server.URL, "myFunction", []byte(`{"input": "test"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
}

func TestAzureClient_SendEventGridEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("aeg-sas-key") != "test-key" {
			t.Errorf("expected aeg-sas-key header")
		}
		w.WriteHeader(202)
	}))
	defer server.Close()

	cfg := AzureConfig{}
	client := NewAzureClient(cfg)

	events := []EventGridEvent{
		{
			ID:          "event-1",
			EventType:   "TestEvent",
			Subject:     "test/subject",
			DataVersion: "1.0",
			Data:        map[string]interface{}{"key": "value"},
		},
	}

	err := client.SendEventGridEvent(context.Background(), server.URL, "test-key", events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSha256Hash(t *testing.T) {
	hash := sha256Hash([]byte("test"))
	if len(hash) != 64 {
		t.Errorf("expected 64 char hash, got %d", len(hash))
	}
}

func TestGetSignatureKey(t *testing.T) {
	key := getSignatureKey("secret", "20230101", "us-east-1", "lambda")
	if len(key) == 0 {
		t.Error("expected non-empty signing key")
	}
}

func TestExtractXMLValue(t *testing.T) {
	xml := "<Response><MessageId>abc123</MessageId></Response>"
	value := extractXMLValue(xml, "MessageId")
	if value != "abc123" {
		t.Errorf("expected abc123, got %s", value)
	}

	value = extractXMLValue(xml, "NotFound")
	if value != "" {
		t.Errorf("expected empty string, got %s", value)
	}
}

func TestFlattenHeaders(t *testing.T) {
	h := http.Header{}
	h.Add("Content-Type", "application/json")
	h.Add("X-Custom", "value1")
	h.Add("X-Custom", "value2")

	result := flattenHeaders(h)

	if result["Content-Type"] != "application/json" {
		t.Errorf("expected application/json, got %s", result["Content-Type"])
	}
	if result["X-Custom"] != "value1" {
		t.Errorf("expected first value value1, got %s", result["X-Custom"])
	}
}

// testTransport redirects requests to a test server
type testTransport struct {
	server *httptest.Server
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.server.URL[7:]
	return http.DefaultTransport.RoundTrip(req)
}
