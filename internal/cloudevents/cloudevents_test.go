package cloudevents

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewEvent(t *testing.T) {
	event := NewEvent("/test/source", "test.event")

	if event.ID == "" {
		t.Error("expected ID to be set")
	}
	if event.Source != "/test/source" {
		t.Errorf("expected source /test/source, got %s", event.Source)
	}
	if event.Type != "test.event" {
		t.Errorf("expected type test.event, got %s", event.Type)
	}
	if event.SpecVersion != Version {
		t.Errorf("expected spec version %s, got %s", Version, event.SpecVersion)
	}
	if event.Time.IsZero() {
		t.Error("expected time to be set")
	}
}

func TestEvent_SetData(t *testing.T) {
	event := NewEvent("/source", "type")

	data := map[string]string{"key": "value"}
	err := event.SetData("application/json", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.DataContentType != "application/json" {
		t.Errorf("expected content type application/json, got %s", event.DataContentType)
	}
	if event.Data == nil {
		t.Error("expected data to be set")
	}
}

func TestEvent_SetDataBinary(t *testing.T) {
	event := NewEvent("/source", "type")

	data := []byte("binary data")
	err := event.SetDataBinary("application/octet-stream", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.DataBase64 == "" {
		t.Error("expected data_base64 to be set")
	}
	if event.Data != nil {
		t.Error("expected data to be nil")
	}
}

func TestEvent_Extensions(t *testing.T) {
	event := NewEvent("/source", "type")

	event.SetExtension("customext", "value123")
	value := event.GetExtension("customext")

	if value != "value123" {
		t.Errorf("expected value123, got %v", value)
	}

	missing := event.GetExtension("nonexistent")
	if missing != nil {
		t.Errorf("expected nil for missing extension, got %v", missing)
	}
}

func TestEvent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		event   *Event
		wantErr bool
	}{
		{
			name:    "valid event",
			event:   NewEvent("/source", "type"),
			wantErr: false,
		},
		{
			name:    "missing id",
			event:   &Event{Source: "/source", SpecVersion: "1.0", Type: "type"},
			wantErr: true,
		},
		{
			name:    "missing source",
			event:   &Event{ID: "123", SpecVersion: "1.0", Type: "type"},
			wantErr: true,
		},
		{
			name:    "missing specversion",
			event:   &Event{ID: "123", Source: "/source", Type: "type"},
			wantErr: true,
		},
		{
			name:    "missing type",
			event:   &Event{ID: "123", Source: "/source", SpecVersion: "1.0"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEvent_MarshalJSON(t *testing.T) {
	event := NewEvent("/source", "type")
	event.SetExtension("customext", "value")
	event.SetData("application/json", map[string]string{"key": "value"})

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if m["customext"] != "value" {
		t.Error("expected extension to be included")
	}
	if m["source"] != "/source" {
		t.Error("expected source to be included")
	}
}

func TestEvent_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"id": "test-id",
		"source": "/test",
		"specversion": "1.0",
		"type": "test.type",
		"time": "2023-01-01T00:00:00Z",
		"customext": "value",
		"data": {"key": "value"}
	}`

	var event Event
	if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.ID != "test-id" {
		t.Errorf("expected id test-id, got %s", event.ID)
	}
	if event.Extensions["customext"] != "value" {
		t.Error("expected extension to be parsed")
	}
}

func TestClient_Send(t *testing.T) {
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	event := NewEvent("/source", "type")

	err := client.Send(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedContentType != ContentTypeStructuredJSON {
		t.Errorf("expected content type %s, got %s", ContentTypeStructuredJSON, receivedContentType)
	}
}

func TestClient_SendBinary(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	event := NewEvent("/source", "type")
	event.Subject = "test-subject"
	event.SetData("application/json", map[string]string{"key": "value"})

	err := client.SendBinary(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedHeaders.Get("ce-id") == "" {
		t.Error("expected ce-id header")
	}
	if receivedHeaders.Get("ce-source") != "/source" {
		t.Errorf("expected ce-source /source, got %s", receivedHeaders.Get("ce-source"))
	}
	if receivedHeaders.Get("ce-type") != "type" {
		t.Errorf("expected ce-type type, got %s", receivedHeaders.Get("ce-type"))
	}
	if receivedHeaders.Get("ce-subject") != "test-subject" {
		t.Errorf("expected ce-subject test-subject, got %s", receivedHeaders.Get("ce-subject"))
	}
}

func TestClient_SendBatch(t *testing.T) {
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	events := []*Event{
		NewEvent("/source1", "type1"),
		NewEvent("/source2", "type2"),
	}

	err := client.SendBatch(context.Background(), events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedContentType != ContentTypeBatchJSON {
		t.Errorf("expected content type %s, got %s", ContentTypeBatchJSON, receivedContentType)
	}
}

func TestClient_SendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	event := NewEvent("/source", "type")

	err := client.Send(context.Background(), event)
	if err == nil {
		t.Error("expected error for bad request")
	}
}

func TestReceiver_Structured(t *testing.T) {
	var receivedEvent *Event
	receiver := NewReceiver(func(ctx context.Context, event *Event) error {
		receivedEvent = event
		return nil
	})

	event := NewEvent("/source", "test.type")
	data, _ := json.Marshal(event)

	req := httptest.NewRequest("POST", "/", bytes.NewReader(data))
	req.Header.Set("Content-Type", ContentTypeStructuredJSON)
	w := httptest.NewRecorder()

	receiver.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if receivedEvent == nil {
		t.Error("expected event to be received")
	}
	if receivedEvent.Type != "test.type" {
		t.Errorf("expected type test.type, got %s", receivedEvent.Type)
	}
}

func TestReceiver_Binary(t *testing.T) {
	var receivedEvent *Event
	receiver := NewReceiver(func(ctx context.Context, event *Event) error {
		receivedEvent = event
		return nil
	})

	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"key": "value"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ce-id", "test-id")
	req.Header.Set("ce-source", "/source")
	req.Header.Set("ce-specversion", "1.0")
	req.Header.Set("ce-type", "test.type")
	req.Header.Set("ce-time", time.Now().UTC().Format(time.RFC3339))
	w := httptest.NewRecorder()

	receiver.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		body, _ := io.ReadAll(w.Body)
		t.Errorf("expected status 200, got %d: %s", w.Code, string(body))
	}
	if receivedEvent == nil {
		t.Fatal("expected event to be received")
	}
	if receivedEvent.ID != "test-id" {
		t.Errorf("expected id test-id, got %s", receivedEvent.ID)
	}
}

func TestChronosEventTypes(t *testing.T) {
	if ChronosEventTypes.JobCreated != "io.chronos.job.created" {
		t.Error("unexpected JobCreated event type")
	}
	if ChronosEventTypes.JobFailed != "io.chronos.job.failed" {
		t.Error("unexpected JobFailed event type")
	}
}

func TestNewJobEvent(t *testing.T) {
	event := NewJobEvent(ChronosEventTypes.JobCreated, "job-123", "my-job")

	if event.Type != ChronosEventTypes.JobCreated {
		t.Errorf("expected type %s, got %s", ChronosEventTypes.JobCreated, event.Type)
	}
	if event.Subject != "job-123" {
		t.Errorf("expected subject job-123, got %s", event.Subject)
	}
}

func TestNewJobExecutionEvent(t *testing.T) {
	event := NewJobExecutionEvent(
		ChronosEventTypes.JobSucceeded,
		"job-123", "my-job", "exec-456",
		"success", 1500, "",
	)

	if event.Type != ChronosEventTypes.JobSucceeded {
		t.Errorf("expected type %s, got %s", ChronosEventTypes.JobSucceeded, event.Type)
	}
	if event.Subject != "exec-456" {
		t.Errorf("expected subject exec-456, got %s", event.Subject)
	}
}

func TestKafkaAdapter(t *testing.T) {
	adapter := NewKafkaAdapter("/kafka")

	msg := &KafkaMessage{
		Topic:     "test-topic",
		Partition: 1,
		Offset:    100,
		Key:       []byte("key"),
		Value:     []byte(`{"data": "value"}`),
	}

	event, err := adapter.ToCloudEvent(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Source != "/kafka" {
		t.Errorf("expected source /kafka, got %s", event.Source)
	}
	if event.GetExtension("kafkatopic") != "test-topic" {
		t.Error("expected kafkatopic extension")
	}

	// Test FromCloudEvent
	converted, err := adapter.FromCloudEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kafkaMsg, ok := converted.(*KafkaMessage)
	if !ok {
		t.Fatalf("expected *KafkaMessage, got %T", converted)
	}
	if kafkaMsg.Topic != "test-topic" {
		t.Errorf("expected topic test-topic, got %s", kafkaMsg.Topic)
	}
}

func TestKafkaAdapter_InvalidMessage(t *testing.T) {
	adapter := NewKafkaAdapter("/kafka")

	_, err := adapter.ToCloudEvent(context.Background(), "invalid")
	if err == nil {
		t.Error("expected error for invalid message type")
	}
}

func TestSQSAdapter(t *testing.T) {
	adapter := NewSQSAdapter("/sqs")

	msg := &SQSMessage{
		MessageID:     "msg-123",
		ReceiptHandle: "handle-456",
		Body:          `{"data": "value"}`,
	}

	event, err := adapter.ToCloudEvent(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Source != "/sqs" {
		t.Errorf("expected source /sqs, got %s", event.Source)
	}
	if event.GetExtension("sqsmessageid") != "msg-123" {
		t.Error("expected sqsmessageid extension")
	}

	// Test FromCloudEvent
	converted, err := adapter.FromCloudEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sqsMsg, ok := converted.(*SQSMessage)
	if !ok {
		t.Fatalf("expected *SQSMessage, got %T", converted)
	}
	if sqsMsg.MessageID != "msg-123" {
		t.Errorf("expected message id msg-123, got %s", sqsMsg.MessageID)
	}
}

func TestSQSAdapter_InvalidMessage(t *testing.T) {
	adapter := NewSQSAdapter("/sqs")

	_, err := adapter.ToCloudEvent(context.Background(), 123)
	if err == nil {
		t.Error("expected error for invalid message type")
	}
}
