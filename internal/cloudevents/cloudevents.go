// Package cloudevents provides CloudEvents native support for Chronos.
package cloudevents

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Version is the CloudEvents specification version.
const Version = "1.0"

// ContentType constants for CloudEvents.
const (
	ContentTypeStructuredJSON = "application/cloudevents+json"
	ContentTypeBatchJSON      = "application/cloudevents-batch+json"
	ContentTypeJSON           = "application/json"
)

// Event represents a CloudEvents event.
type Event struct {
	// Required attributes
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	SpecVersion string    `json:"specversion"`
	Type        string    `json:"type"`
	Time        time.Time `json:"time,omitempty"`

	// Optional attributes
	DataContentType string `json:"datacontenttype,omitempty"`
	DataSchema      string `json:"dataschema,omitempty"`
	Subject         string `json:"subject,omitempty"`

	// Extension attributes
	Extensions map[string]interface{} `json:"-"`

	// Data
	Data       interface{} `json:"data,omitempty"`
	DataBase64 string      `json:"data_base64,omitempty"`
}

// NewEvent creates a new CloudEvents event with required fields.
func NewEvent(source, eventType string) *Event {
	return &Event{
		ID:          uuid.New().String(),
		Source:      source,
		SpecVersion: Version,
		Type:        eventType,
		Time:        time.Now().UTC(),
		Extensions:  make(map[string]interface{}),
	}
}

// SetData sets the event data.
func (e *Event) SetData(contentType string, data interface{}) error {
	e.DataContentType = contentType
	e.Data = data
	return nil
}

// SetDataBinary sets binary data using base64 encoding.
func (e *Event) SetDataBinary(contentType string, data []byte) error {
	e.DataContentType = contentType
	e.DataBase64 = base64.StdEncoding.EncodeToString(data)
	e.Data = nil
	return nil
}

// SetExtension sets an extension attribute.
func (e *Event) SetExtension(name string, value interface{}) {
	if e.Extensions == nil {
		e.Extensions = make(map[string]interface{})
	}
	e.Extensions[name] = value
}

// GetExtension gets an extension attribute.
func (e *Event) GetExtension(name string) interface{} {
	if e.Extensions == nil {
		return nil
	}
	return e.Extensions[name]
}

// Validate validates the event.
func (e *Event) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("id is required")
	}
	if e.Source == "" {
		return fmt.Errorf("source is required")
	}
	if e.SpecVersion == "" {
		return fmt.Errorf("specversion is required")
	}
	if e.Type == "" {
		return fmt.Errorf("type is required")
	}

	// Validate source is a valid URI
	if _, err := url.Parse(e.Source); err != nil {
		return fmt.Errorf("source must be a valid URI: %w", err)
	}

	return nil
}

// MarshalJSON implements custom JSON marshaling.
func (e *Event) MarshalJSON() ([]byte, error) {
	type Alias Event
	aux := &struct {
		*Alias
		Time string `json:"time,omitempty"`
	}{
		Alias: (*Alias)(e),
	}

	if !e.Time.IsZero() {
		aux.Time = e.Time.Format(time.RFC3339)
	}

	data, err := json.Marshal(aux)
	if err != nil {
		return nil, err
	}

	// Merge extensions
	if len(e.Extensions) > 0 {
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		for k, v := range e.Extensions {
			m[k] = v
		}
		return json.Marshal(m)
	}

	return data, nil
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (e *Event) UnmarshalJSON(data []byte) error {
	type Alias Event
	aux := &struct {
		*Alias
		Time string `json:"time,omitempty"`
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if aux.Time != "" {
		t, err := time.Parse(time.RFC3339, aux.Time)
		if err != nil {
			return fmt.Errorf("invalid time format: %w", err)
		}
		e.Time = t
	}

	// Extract extensions
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	knownFields := map[string]bool{
		"id": true, "source": true, "specversion": true, "type": true,
		"time": true, "datacontenttype": true, "dataschema": true,
		"subject": true, "data": true, "data_base64": true,
	}

	e.Extensions = make(map[string]interface{})
	for k, v := range m {
		if !knownFields[k] {
			e.Extensions[k] = v
		}
	}

	return nil
}

// Client is a CloudEvents HTTP client.
type Client struct {
	httpClient *http.Client
	target     string
}

// NewClient creates a new CloudEvents client.
func NewClient(target string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		target:     target,
	}
}

// Send sends a CloudEvents event using structured content mode.
func (c *Client) Send(ctx context.Context, event *Event) error {
	if err := event.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.target, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", ContentTypeStructuredJSON)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendBinary sends a CloudEvents event using binary content mode.
func (c *Client) SendBinary(ctx context.Context, event *Event) error {
	if err := event.Validate(); err != nil {
		return err
	}

	var body []byte
	var err error

	if event.Data != nil {
		body, err = json.Marshal(event.Data)
		if err != nil {
			return err
		}
	} else if event.DataBase64 != "" {
		body, err = base64.StdEncoding.DecodeString(event.DataBase64)
		if err != nil {
			return err
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.target, bytes.NewReader(body))
	if err != nil {
		return err
	}

	// Set CloudEvents headers
	req.Header.Set("ce-id", event.ID)
	req.Header.Set("ce-source", event.Source)
	req.Header.Set("ce-specversion", event.SpecVersion)
	req.Header.Set("ce-type", event.Type)

	if !event.Time.IsZero() {
		req.Header.Set("ce-time", event.Time.Format(time.RFC3339))
	}
	if event.DataContentType != "" {
		req.Header.Set("Content-Type", event.DataContentType)
	}
	if event.DataSchema != "" {
		req.Header.Set("ce-dataschema", event.DataSchema)
	}
	if event.Subject != "" {
		req.Header.Set("ce-subject", event.Subject)
	}

	for k, v := range event.Extensions {
		if s, ok := v.(string); ok {
			req.Header.Set("ce-"+k, s)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendBatch sends multiple CloudEvents events.
func (c *Client) SendBatch(ctx context.Context, events []*Event) error {
	for _, e := range events {
		if err := e.Validate(); err != nil {
			return err
		}
	}

	data, err := json.Marshal(events)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.target, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", ContentTypeBatchJSON)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("batch send failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Receiver handles incoming CloudEvents.
type Receiver struct {
	handler func(context.Context, *Event) error
}

// NewReceiver creates a new CloudEvents receiver.
func NewReceiver(handler func(context.Context, *Event) error) *Receiver {
	return &Receiver{handler: handler}
}

// ServeHTTP handles incoming HTTP requests.
func (r *Receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	event, err := r.parseRequest(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.handler(req.Context(), event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// parseRequest parses a CloudEvents HTTP request.
func (r *Receiver) parseRequest(req *http.Request) (*Event, error) {
	contentType := req.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, ContentTypeStructuredJSON) {
		return r.parseStructured(req)
	}

	return r.parseBinary(req)
}

// parseStructured parses structured content mode.
func (r *Receiver) parseStructured(req *http.Request) (*Event, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	var event Event
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}

	return &event, event.Validate()
}

// parseBinary parses binary content mode.
func (r *Receiver) parseBinary(req *http.Request) (*Event, error) {
	event := &Event{
		ID:              req.Header.Get("ce-id"),
		Source:          req.Header.Get("ce-source"),
		SpecVersion:     req.Header.Get("ce-specversion"),
		Type:            req.Header.Get("ce-type"),
		DataContentType: req.Header.Get("Content-Type"),
		DataSchema:      req.Header.Get("ce-dataschema"),
		Subject:         req.Header.Get("ce-subject"),
		Extensions:      make(map[string]interface{}),
	}

	if timeStr := req.Header.Get("ce-time"); timeStr != "" {
		t, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid ce-time: %w", err)
		}
		event.Time = t
	}

	// Parse extensions from headers
	for k, v := range req.Header {
		if strings.HasPrefix(strings.ToLower(k), "ce-") && len(v) > 0 {
			name := strings.ToLower(k[3:])
			if name != "id" && name != "source" && name != "specversion" &&
				name != "type" && name != "time" && name != "dataschema" && name != "subject" {
				event.Extensions[name] = v[0]
			}
		}
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	if len(body) > 0 {
		if strings.HasPrefix(event.DataContentType, "application/json") {
			var data interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				return nil, err
			}
			event.Data = data
		} else {
			event.DataBase64 = base64.StdEncoding.EncodeToString(body)
		}
	}

	return event, event.Validate()
}

// ChronosEventTypes defines Chronos-specific event types.
var ChronosEventTypes = struct {
	JobCreated   string
	JobUpdated   string
	JobDeleted   string
	JobTriggered string
	JobSucceeded string
	JobFailed    string
	JobRetrying  string
	JobTimeout   string
}{
	JobCreated:   "io.chronos.job.created",
	JobUpdated:   "io.chronos.job.updated",
	JobDeleted:   "io.chronos.job.deleted",
	JobTriggered: "io.chronos.job.triggered",
	JobSucceeded: "io.chronos.job.succeeded",
	JobFailed:    "io.chronos.job.failed",
	JobRetrying:  "io.chronos.job.retrying",
	JobTimeout:   "io.chronos.job.timeout",
}

// JobEventData is the data payload for job-related events.
type JobEventData struct {
	JobID        string                 `json:"job_id"`
	JobName      string                 `json:"job_name"`
	ExecutionID  string                 `json:"execution_id,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Duration     int64                  `json:"duration_ms,omitempty"`
	Error        string                 `json:"error,omitempty"`
	RetryCount   int                    `json:"retry_count,omitempty"`
	NextRun      *time.Time             `json:"next_run,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// NewJobEvent creates a new Chronos job event.
func NewJobEvent(eventType, jobID, jobName string) *Event {
	event := NewEvent("/chronos", eventType)
	event.Subject = jobID
	event.SetData(ContentTypeJSON, &JobEventData{
		JobID:   jobID,
		JobName: jobName,
	})
	return event
}

// NewJobExecutionEvent creates a job execution event.
func NewJobExecutionEvent(eventType, jobID, jobName, executionID, status string, durationMs int64, errMsg string) *Event {
	event := NewEvent("/chronos", eventType)
	event.Subject = executionID
	event.SetData(ContentTypeJSON, &JobEventData{
		JobID:       jobID,
		JobName:     jobName,
		ExecutionID: executionID,
		Status:      status,
		Duration:    durationMs,
		Error:       errMsg,
	})
	return event
}

// Adapter converts between event sources and CloudEvents.
type Adapter interface {
	ToCloudEvent(ctx context.Context, source interface{}) (*Event, error)
	FromCloudEvent(ctx context.Context, event *Event) (interface{}, error)
}

// KafkaAdapter adapts Kafka messages to CloudEvents.
type KafkaAdapter struct {
	Source string
}

// NewKafkaAdapter creates a new Kafka adapter.
func NewKafkaAdapter(source string) *KafkaAdapter {
	return &KafkaAdapter{Source: source}
}

// ToCloudEvent converts a Kafka message to a CloudEvent.
func (a *KafkaAdapter) ToCloudEvent(ctx context.Context, msg interface{}) (*Event, error) {
	kafkaMsg, ok := msg.(*KafkaMessage)
	if !ok {
		return nil, fmt.Errorf("expected *KafkaMessage, got %T", msg)
	}

	event := NewEvent(a.Source, "io.chronos.kafka.message")
	event.Subject = kafkaMsg.Topic
	event.SetExtension("kafkatopic", kafkaMsg.Topic)
	event.SetExtension("kafkapartition", kafkaMsg.Partition)
	event.SetExtension("kafkaoffset", kafkaMsg.Offset)

	if kafkaMsg.Key != nil {
		event.SetExtension("kafkakey", string(kafkaMsg.Key))
	}

	event.SetData(ContentTypeJSON, kafkaMsg.Value)
	return event, nil
}

// FromCloudEvent extracts a Kafka message from a CloudEvent.
func (a *KafkaAdapter) FromCloudEvent(ctx context.Context, event *Event) (interface{}, error) {
	msg := &KafkaMessage{}

	if topic, ok := event.GetExtension("kafkatopic").(string); ok {
		msg.Topic = topic
	} else if event.Subject != "" {
		msg.Topic = event.Subject
	}

	if key, ok := event.GetExtension("kafkakey").(string); ok {
		msg.Key = []byte(key)
	}

	if event.Data != nil {
		data, err := json.Marshal(event.Data)
		if err != nil {
			return nil, err
		}
		msg.Value = data
	}

	return msg, nil
}

// KafkaMessage represents a Kafka message.
type KafkaMessage struct {
	Topic     string `json:"topic"`
	Partition int    `json:"partition"`
	Offset    int64  `json:"offset"`
	Key       []byte `json:"key,omitempty"`
	Value     []byte `json:"value"`
}

// SQSAdapter adapts SQS messages to CloudEvents.
type SQSAdapter struct {
	Source string
}

// NewSQSAdapter creates a new SQS adapter.
func NewSQSAdapter(source string) *SQSAdapter {
	return &SQSAdapter{Source: source}
}

// ToCloudEvent converts an SQS message to a CloudEvent.
func (a *SQSAdapter) ToCloudEvent(ctx context.Context, msg interface{}) (*Event, error) {
	sqsMsg, ok := msg.(*SQSMessage)
	if !ok {
		return nil, fmt.Errorf("expected *SQSMessage, got %T", msg)
	}

	event := NewEvent(a.Source, "io.chronos.sqs.message")
	event.SetExtension("sqsmessageid", sqsMsg.MessageID)
	event.SetExtension("sqsreceipthandle", sqsMsg.ReceiptHandle)

	event.SetData(ContentTypeJSON, []byte(sqsMsg.Body))
	return event, nil
}

// FromCloudEvent extracts an SQS message from a CloudEvent.
func (a *SQSAdapter) FromCloudEvent(ctx context.Context, event *Event) (interface{}, error) {
	msg := &SQSMessage{}

	if id, ok := event.GetExtension("sqsmessageid").(string); ok {
		msg.MessageID = id
	}

	if event.Data != nil {
		data, err := json.Marshal(event.Data)
		if err != nil {
			return nil, err
		}
		msg.Body = string(data)
	}

	return msg, nil
}

// SQSMessage represents an SQS message.
type SQSMessage struct {
	MessageID     string            `json:"message_id"`
	ReceiptHandle string            `json:"receipt_handle"`
	Body          string            `json:"body"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}
