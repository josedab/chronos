// Package events provides unified event ingestion for Chronos.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Ingestion errors.
var (
	ErrUnsupportedSource  = errors.New("unsupported event source")
	ErrInvalidEvent       = errors.New("invalid event format")
	ErrIngestionFailed    = errors.New("event ingestion failed")
	ErrHandlerNotSet      = errors.New("event handler not set")
)

// EventSource identifies the source of an event.
type EventSource string

const (
	SourceCloudEvents EventSource = "cloudevents"
	SourceS3          EventSource = "s3"
	SourceKafka       EventSource = "kafka"
	SourceSQS         EventSource = "sqs"
	SourcePubSub      EventSource = "pubsub"
	SourceWebhook     EventSource = "webhook"
	SourceNATS        EventSource = "nats"
	SourceRabbitMQ    EventSource = "rabbitmq"
)

// NormalizedEvent is the unified event format used internally.
type NormalizedEvent struct {
	ID              string                 `json:"id"`
	Source          EventSource            `json:"source"`
	Type            string                 `json:"type"`
	Subject         string                 `json:"subject,omitempty"`
	Data            interface{}            `json:"data"`
	DataContentType string                 `json:"data_content_type,omitempty"`
	Time            time.Time              `json:"time"`
	Extensions      map[string]interface{} `json:"extensions,omitempty"`
	RawData         []byte                 `json:"-"`
	Headers         map[string]string      `json:"headers,omitempty"`
	TraceID         string                 `json:"trace_id,omitempty"`
}

// IngestorConfig configures the event ingestor.
type IngestorConfig struct {
	// MaxEventSize is the maximum allowed event size in bytes.
	MaxEventSize int `json:"max_event_size" yaml:"max_event_size"`
	// QueueSize is the size of the internal event queue.
	QueueSize int `json:"queue_size" yaml:"queue_size"`
	// Workers is the number of concurrent event processors.
	Workers int `json:"workers" yaml:"workers"`
	// EnableMetrics enables event ingestion metrics.
	EnableMetrics bool `json:"enable_metrics" yaml:"enable_metrics"`
}

// DefaultIngestorConfig returns sensible defaults.
func DefaultIngestorConfig() IngestorConfig {
	return IngestorConfig{
		MaxEventSize:  1024 * 1024, // 1MB
		QueueSize:     10000,
		Workers:       4,
		EnableMetrics: true,
	}
}

// EventHandler processes normalized events.
type EventHandler func(ctx context.Context, event *NormalizedEvent) error

// Ingestor handles event ingestion from multiple sources.
type Ingestor struct {
	mu            sync.RWMutex
	config        IngestorConfig
	handler       EventHandler
	adapters      map[EventSource]EventAdapter
	eventQueue    chan *NormalizedEvent
	stopCh        chan struct{}
	wg            sync.WaitGroup
	metrics       *IngestorMetrics
}

// IngestorMetrics tracks ingestion statistics.
type IngestorMetrics struct {
	mu               sync.Mutex
	TotalReceived    int64            `json:"total_received"`
	TotalProcessed   int64            `json:"total_processed"`
	TotalFailed      int64            `json:"total_failed"`
	BySource         map[EventSource]int64 `json:"by_source"`
	ByType           map[string]int64 `json:"by_type"`
	ProcessingTimeMs float64          `json:"avg_processing_time_ms"`
	QueueDepth       int64            `json:"queue_depth"`
	LastEventTime    time.Time        `json:"last_event_time"`
}

// IngestorMetricsSnapshot is a point-in-time copy of metrics without sync primitives.
type IngestorMetricsSnapshot struct {
	TotalReceived    int64                 `json:"total_received"`
	TotalProcessed   int64                 `json:"total_processed"`
	TotalFailed      int64                 `json:"total_failed"`
	BySource         map[EventSource]int64 `json:"by_source"`
	ByType           map[string]int64      `json:"by_type"`
	ProcessingTimeMs float64               `json:"avg_processing_time_ms"`
	QueueDepth       int64                 `json:"queue_depth"`
	LastEventTime    time.Time             `json:"last_event_time"`
}

// EventAdapter converts source-specific events to normalized format.
type EventAdapter interface {
	// Parse parses raw data into a normalized event.
	Parse(ctx context.Context, data []byte, headers map[string]string) (*NormalizedEvent, error)
	// Source returns the event source type.
	Source() EventSource
}

// NewIngestor creates a new event ingestor.
func NewIngestor(config IngestorConfig, handler EventHandler) *Ingestor {
	ing := &Ingestor{
		config:     config,
		handler:    handler,
		adapters:   make(map[EventSource]EventAdapter),
		eventQueue: make(chan *NormalizedEvent, config.QueueSize),
		stopCh:     make(chan struct{}),
		metrics: &IngestorMetrics{
			BySource: make(map[EventSource]int64),
			ByType:   make(map[string]int64),
		},
	}

	// Register built-in adapters
	ing.RegisterAdapter(&CloudEventsAdapter{})
	ing.RegisterAdapter(&S3Adapter{})
	ing.RegisterAdapter(&WebhookAdapter{})
	ing.RegisterAdapter(&KafkaAdapter{})

	// Start workers
	for i := 0; i < config.Workers; i++ {
		ing.wg.Add(1)
		go ing.worker()
	}

	return ing
}

// RegisterAdapter registers an event adapter.
func (ing *Ingestor) RegisterAdapter(adapter EventAdapter) {
	ing.mu.Lock()
	defer ing.mu.Unlock()
	ing.adapters[adapter.Source()] = adapter
}

// Ingest ingests an event from a specific source.
func (ing *Ingestor) Ingest(ctx context.Context, source EventSource, data []byte, headers map[string]string) error {
	ing.mu.RLock()
	adapter, exists := ing.adapters[source]
	ing.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %s", ErrUnsupportedSource, source)
	}

	if len(data) > ing.config.MaxEventSize {
		return fmt.Errorf("event too large: %d bytes (max %d)", len(data), ing.config.MaxEventSize)
	}

	// Parse the event
	event, err := adapter.Parse(ctx, data, headers)
	if err != nil {
		ing.recordMetric(source, "", false)
		return fmt.Errorf("%w: %v", ErrInvalidEvent, err)
	}

	// Ensure ID is set
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	event.RawData = data
	event.TraceID = ing.generateTraceID()

	// Queue the event
	select {
	case ing.eventQueue <- event:
		ing.recordMetric(source, event.Type, true)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("%w: queue full", ErrIngestionFailed)
	}
}

// IngestHTTP handles HTTP webhook requests.
func (ing *Ingestor) IngestHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Determine source from content type or headers
	source := ing.detectSource(r)

	// Read body
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(ing.config.MaxEventSize+1)))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	if len(body) > ing.config.MaxEventSize {
		http.Error(w, "Event too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Extract headers
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	// Add request metadata
	headers["X-Request-Method"] = r.Method
	headers["X-Request-Path"] = r.URL.Path
	headers["X-Remote-Addr"] = r.RemoteAddr

	// Ingest
	if err := ing.Ingest(ctx, source, body, headers); err != nil {
		if errors.Is(err, ErrInvalidEvent) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// detectSource determines the event source from HTTP request.
func (ing *Ingestor) detectSource(r *http.Request) EventSource {
	contentType := r.Header.Get("Content-Type")

	// CloudEvents detection
	if strings.Contains(contentType, "cloudevents") ||
		r.Header.Get("ce-specversion") != "" {
		return SourceCloudEvents
	}

	// S3 notification detection
	if r.Header.Get("X-Amz-Sns-Message-Type") != "" ||
		strings.Contains(r.URL.Path, "/s3") {
		return SourceS3
	}

	// Default to generic webhook
	return SourceWebhook
}

// worker processes events from the queue.
func (ing *Ingestor) worker() {
	defer ing.wg.Done()

	for {
		select {
		case event := <-ing.eventQueue:
			ing.processEvent(event)
		case <-ing.stopCh:
			return
		}
	}
}

// processEvent processes a single event.
func (ing *Ingestor) processEvent(event *NormalizedEvent) {
	if ing.handler == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	err := ing.handler(ctx, event)
	duration := time.Since(start)

	ing.updateProcessingMetrics(event.Source, event.Type, err == nil, duration)
}

// recordMetric records an ingestion metric.
func (ing *Ingestor) recordMetric(source EventSource, eventType string, success bool) {
	ing.metrics.mu.Lock()
	defer ing.metrics.mu.Unlock()

	ing.metrics.TotalReceived++
	ing.metrics.BySource[source]++
	if eventType != "" {
		ing.metrics.ByType[eventType]++
	}
	if !success {
		ing.metrics.TotalFailed++
	}
	ing.metrics.LastEventTime = time.Now()
	ing.metrics.QueueDepth = int64(len(ing.eventQueue))
}

// updateProcessingMetrics updates processing metrics.
func (ing *Ingestor) updateProcessingMetrics(source EventSource, eventType string, success bool, duration time.Duration) {
	ing.metrics.mu.Lock()
	defer ing.metrics.mu.Unlock()

	if success {
		ing.metrics.TotalProcessed++
	} else {
		ing.metrics.TotalFailed++
	}

	// Exponential moving average for processing time
	alpha := 0.1
	durationMs := float64(duration.Milliseconds())
	ing.metrics.ProcessingTimeMs = alpha*durationMs + (1-alpha)*ing.metrics.ProcessingTimeMs
}

// generateTraceID generates a trace ID for event tracking.
func (ing *Ingestor) generateTraceID() string {
	return uuid.New().String()[:16]
}

// GetMetrics returns current ingestion metrics.
func (ing *Ingestor) GetMetrics() IngestorMetricsSnapshot {
	ing.metrics.mu.Lock()
	defer ing.metrics.mu.Unlock()

	m := IngestorMetricsSnapshot{
		TotalReceived:    ing.metrics.TotalReceived,
		TotalProcessed:   ing.metrics.TotalProcessed,
		TotalFailed:      ing.metrics.TotalFailed,
		ProcessingTimeMs: ing.metrics.ProcessingTimeMs,
		QueueDepth:       int64(len(ing.eventQueue)),
		LastEventTime:    ing.metrics.LastEventTime,
		BySource:         make(map[EventSource]int64),
		ByType:           make(map[string]int64),
	}
	for k, v := range ing.metrics.BySource {
		m.BySource[k] = v
	}
	for k, v := range ing.metrics.ByType {
		m.ByType[k] = v
	}
	return m
}

// Stop stops the ingestor.
func (ing *Ingestor) Stop() {
	close(ing.stopCh)
	ing.wg.Wait()
}

// CloudEventsAdapter adapts CloudEvents to normalized format.
type CloudEventsAdapter struct{}

func (a *CloudEventsAdapter) Source() EventSource { return SourceCloudEvents }

func (a *CloudEventsAdapter) Parse(ctx context.Context, data []byte, headers map[string]string) (*NormalizedEvent, error) {
	// Check for binary content mode
	if headers["ce-specversion"] != "" {
		return a.parseBinary(data, headers)
	}

	// Structured content mode
	return a.parseStructured(data, headers)
}

func (a *CloudEventsAdapter) parseStructured(data []byte, headers map[string]string) (*NormalizedEvent, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	event := &NormalizedEvent{
		Source:     SourceCloudEvents,
		Extensions: make(map[string]interface{}),
		Headers:    headers,
	}

	// Extract required fields
	if id, ok := raw["id"].(string); ok {
		event.ID = id
	}
	if t, ok := raw["type"].(string); ok {
		event.Type = t
	}
	if s, ok := raw["subject"].(string); ok {
		event.Subject = s
	}
	if ct, ok := raw["datacontenttype"].(string); ok {
		event.DataContentType = ct
	}
	if d, ok := raw["data"]; ok {
		event.Data = d
	}
	if timeStr, ok := raw["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			event.Time = t
		}
	}

	// Extract extensions
	knownFields := map[string]bool{
		"id": true, "source": true, "specversion": true, "type": true,
		"time": true, "datacontenttype": true, "dataschema": true,
		"subject": true, "data": true, "data_base64": true,
	}
	for k, v := range raw {
		if !knownFields[k] {
			event.Extensions[k] = v
		}
	}

	return event, nil
}

func (a *CloudEventsAdapter) parseBinary(data []byte, headers map[string]string) (*NormalizedEvent, error) {
	event := &NormalizedEvent{
		ID:              headers["ce-id"],
		Type:            headers["ce-type"],
		Subject:         headers["ce-subject"],
		DataContentType: headers["Content-Type"],
		Source:          SourceCloudEvents,
		Extensions:      make(map[string]interface{}),
		Headers:         headers,
	}

	if timeStr := headers["ce-time"]; timeStr != "" {
		if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			event.Time = t
		}
	}

	// Parse body as data
	if strings.HasPrefix(event.DataContentType, "application/json") {
		var d interface{}
		if err := json.Unmarshal(data, &d); err == nil {
			event.Data = d
		}
	} else {
		event.Data = string(data)
	}

	// Extract extension headers
	for k, v := range headers {
		if strings.HasPrefix(strings.ToLower(k), "ce-") {
			name := strings.ToLower(k[3:])
			if name != "id" && name != "source" && name != "specversion" &&
				name != "type" && name != "time" && name != "subject" {
				event.Extensions[name] = v
			}
		}
	}

	return event, nil
}

// S3Adapter adapts S3 notifications to normalized format.
type S3Adapter struct{}

func (a *S3Adapter) Source() EventSource { return SourceS3 }

func (a *S3Adapter) Parse(ctx context.Context, data []byte, headers map[string]string) (*NormalizedEvent, error) {
	var notification S3Notification
	if err := json.Unmarshal(data, &notification); err != nil {
		return nil, err
	}

	event := &NormalizedEvent{
		ID:         uuid.New().String(),
		Source:     SourceS3,
		Time:       time.Now().UTC(),
		Extensions: make(map[string]interface{}),
		Headers:    headers,
	}

	if len(notification.Records) > 0 {
		rec := notification.Records[0]
		event.Type = rec.EventName
		event.Subject = fmt.Sprintf("s3://%s/%s", rec.S3.Bucket.Name, rec.S3.Object.Key)
		event.Data = rec

		event.Extensions["bucket"] = rec.S3.Bucket.Name
		event.Extensions["key"] = rec.S3.Object.Key
		event.Extensions["size"] = rec.S3.Object.Size
		event.Extensions["region"] = rec.AWSRegion

		if rec.EventTime != "" {
			if t, err := time.Parse(time.RFC3339, rec.EventTime); err == nil {
				event.Time = t
			}
		}
	}

	return event, nil
}

// S3Notification represents an S3 event notification.
type S3Notification struct {
	Records []S3EventRecord `json:"Records"`
}

// S3EventRecord represents a single S3 event record.
type S3EventRecord struct {
	EventVersion string    `json:"eventVersion"`
	EventSource  string    `json:"eventSource"`
	AWSRegion    string    `json:"awsRegion"`
	EventTime    string    `json:"eventTime"`
	EventName    string    `json:"eventName"`
	S3           S3Entity  `json:"s3"`
}

// S3Entity contains S3-specific event data.
type S3Entity struct {
	Bucket S3Bucket `json:"bucket"`
	Object S3Object `json:"object"`
}

// S3Bucket represents an S3 bucket.
type S3Bucket struct {
	Name string `json:"name"`
	ARN  string `json:"arn"`
}

// S3Object represents an S3 object.
type S3Object struct {
	Key       string `json:"key"`
	Size      int64  `json:"size"`
	ETag      string `json:"eTag"`
	VersionID string `json:"versionId"`
	Sequencer string `json:"sequencer"`
}

// WebhookAdapter adapts generic webhooks to normalized format.
type WebhookAdapter struct{}

func (a *WebhookAdapter) Source() EventSource { return SourceWebhook }

func (a *WebhookAdapter) Parse(ctx context.Context, data []byte, headers map[string]string) (*NormalizedEvent, error) {
	event := &NormalizedEvent{
		ID:         uuid.New().String(),
		Source:     SourceWebhook,
		Type:       "webhook.received",
		Time:       time.Now().UTC(),
		Extensions: make(map[string]interface{}),
		Headers:    headers,
	}

	// Try to parse as JSON
	contentType := headers["Content-Type"]
	if strings.Contains(contentType, "application/json") {
		var d interface{}
		if err := json.Unmarshal(data, &d); err == nil {
			event.Data = d
			event.DataContentType = "application/json"

			// Try to extract event type from common fields
			if m, ok := d.(map[string]interface{}); ok {
				if t, ok := m["event"].(string); ok {
					event.Type = t
				} else if t, ok := m["type"].(string); ok {
					event.Type = t
				} else if t, ok := m["action"].(string); ok {
					event.Type = t
				}
			}
		} else {
			event.Data = string(data)
		}
	} else {
		event.Data = string(data)
		event.DataContentType = contentType
	}

	// Add path as subject
	if path := headers["X-Request-Path"]; path != "" {
		event.Subject = path
	}

	return event, nil
}

// KafkaAdapter adapts Kafka messages to normalized format.
type KafkaAdapter struct{}

func (a *KafkaAdapter) Source() EventSource { return SourceKafka }

func (a *KafkaAdapter) Parse(ctx context.Context, data []byte, headers map[string]string) (*NormalizedEvent, error) {
	event := &NormalizedEvent{
		ID:         uuid.New().String(),
		Source:     SourceKafka,
		Type:       "kafka.message",
		Time:       time.Now().UTC(),
		Extensions: make(map[string]interface{}),
		Headers:    headers,
	}

	// Extract Kafka-specific headers
	if topic := headers["kafka-topic"]; topic != "" {
		event.Subject = topic
		event.Extensions["topic"] = topic
	}
	if partition := headers["kafka-partition"]; partition != "" {
		event.Extensions["partition"] = partition
	}
	if offset := headers["kafka-offset"]; offset != "" {
		event.Extensions["offset"] = offset
	}
	if key := headers["kafka-key"]; key != "" {
		event.Extensions["key"] = key
	}

	// Try to parse as JSON
	var d interface{}
	if err := json.Unmarshal(data, &d); err == nil {
		event.Data = d
		event.DataContentType = "application/json"

		// Try to extract message type
		if m, ok := d.(map[string]interface{}); ok {
			if t, ok := m["type"].(string); ok {
				event.Type = "kafka." + t
			}
		}
	} else {
		event.Data = string(data)
	}

	return event, nil
}
