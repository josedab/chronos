// Package dispatcher handles job execution via various protocols.
package dispatcher

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/google/uuid"
)

// Common errors for RabbitMQ dispatcher.
var (
	ErrRabbitMQConnectionFailed = errors.New("rabbitmq: connection failed")
	ErrRabbitMQPublishFailed    = errors.New("rabbitmq: publish failed")
	ErrRabbitMQTimeout          = errors.New("rabbitmq: timeout")
	ErrInvalidRabbitMQConfig    = errors.New("rabbitmq: invalid configuration")
)

// RabbitMQConfig holds RabbitMQ dispatcher configuration for a job.
type RabbitMQConfig struct {
	// URL is the RabbitMQ connection URL (amqp://user:pass@host:port/vhost)
	URL string `json:"url" yaml:"url"`
	// Exchange to publish to
	Exchange string `json:"exchange" yaml:"exchange"`
	// RoutingKey for the message
	RoutingKey string `json:"routing_key" yaml:"routing_key"`
	// Payload is the message body
	Payload string `json:"payload" yaml:"payload"`
	// Headers to include in the message
	Headers map[string]interface{} `json:"headers,omitempty" yaml:"headers,omitempty"`
	// ContentType of the message
	ContentType string `json:"content_type,omitempty" yaml:"content_type,omitempty"`
	// ContentEncoding of the message
	ContentEncoding string `json:"content_encoding,omitempty" yaml:"content_encoding,omitempty"`
	// Priority of the message (0-9)
	Priority uint8 `json:"priority,omitempty" yaml:"priority,omitempty"`
	// DeliveryMode (1=transient, 2=persistent)
	DeliveryMode uint8 `json:"delivery_mode" yaml:"delivery_mode"`
	// Expiration TTL for the message
	Expiration string `json:"expiration,omitempty" yaml:"expiration,omitempty"`
	// MessageID for the message
	MessageID string `json:"message_id,omitempty" yaml:"message_id,omitempty"`
	// Timeout for the publish operation
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	// TLS configuration
	TLS *RabbitMQTLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
	// Mandatory flag
	Mandatory bool `json:"mandatory" yaml:"mandatory"`
	// Immediate flag (deprecated in RabbitMQ 3.0+)
	Immediate bool `json:"immediate" yaml:"immediate"`
	// ConfirmMode enables publisher confirms
	ConfirmMode bool `json:"confirm_mode" yaml:"confirm_mode"`
	// Queue for direct queue publishing (alternative to exchange/routing key)
	Queue string `json:"queue,omitempty" yaml:"queue,omitempty"`
}

// RabbitMQTLSConfig holds TLS configuration for RabbitMQ.
type RabbitMQTLSConfig struct {
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	CACertFile         string `json:"ca_cert_file,omitempty" yaml:"ca_cert_file,omitempty"`
	CertFile           string `json:"cert_file,omitempty" yaml:"cert_file,omitempty"`
	KeyFile            string `json:"key_file,omitempty" yaml:"key_file,omitempty"`
	ServerName         string `json:"server_name,omitempty" yaml:"server_name,omitempty"`
}

// RabbitMQDispatcher executes jobs by publishing to RabbitMQ exchanges/queues.
type RabbitMQDispatcher struct {
	nodeID string
	config *RabbitMQDispatcherConfig

	// Connection pool
	connections   map[string]*rabbitMQConnection
	connectionsMu sync.RWMutex

	// Track running executions
	running   map[string]*runningExecution
	runningMu sync.RWMutex

	// Concurrency control
	semaphore chan struct{}

	// Metrics
	metrics *RabbitMQMetrics
}

// rabbitMQConnection wraps a RabbitMQ connection with metadata.
type rabbitMQConnection struct {
	url       string
	tlsConfig *tls.Config
	createdAt time.Time
	connected bool
}

// RabbitMQDispatcherConfig holds dispatcher-level configuration.
type RabbitMQDispatcherConfig struct {
	NodeID             string
	DefaultTimeout     time.Duration
	MaxConcurrent      int
	HeartbeatInterval  time.Duration
	ConnectionPoolSize int
	ChannelMax         int
}

// RabbitMQMetrics tracks RabbitMQ dispatcher metrics.
type RabbitMQMetrics struct {
	mu                sync.Mutex
	MessagesPublished int64
	MessagesFailed    int64
	MessagesConfirmed int64
	MessagesReturned  int64
	BytesSent         int64
	LatencySum        time.Duration
	LatencyCount      int64
}

// RabbitMQMessage represents a message to be published.
type RabbitMQMessage struct {
	Exchange        string
	RoutingKey      string
	Body            []byte
	Headers         map[string]interface{}
	ContentType     string
	ContentEncoding string
	Priority        uint8
	DeliveryMode    uint8
	Expiration      string
	MessageID       string
	Timestamp       time.Time
	AppID           string
}

// RabbitMQPublishResult represents the result of a publish operation.
type RabbitMQPublishResult struct {
	Exchange   string
	RoutingKey string
	Confirmed  bool
	Returned   bool
	Timestamp  time.Time
}

// DefaultRabbitMQDispatcherConfig returns the default RabbitMQ dispatcher configuration.
func DefaultRabbitMQDispatcherConfig() *RabbitMQDispatcherConfig {
	return &RabbitMQDispatcherConfig{
		NodeID:             "chronos-1",
		DefaultTimeout:     30 * time.Second,
		MaxConcurrent:      100,
		HeartbeatInterval:  10 * time.Second,
		ConnectionPoolSize: 10,
		ChannelMax:         2047,
	}
}

// NewRabbitMQDispatcher creates a new RabbitMQ dispatcher.
func NewRabbitMQDispatcher(cfg *RabbitMQDispatcherConfig) *RabbitMQDispatcher {
	if cfg == nil {
		cfg = DefaultRabbitMQDispatcherConfig()
	}

	return &RabbitMQDispatcher{
		nodeID:      cfg.NodeID,
		config:      cfg,
		connections: make(map[string]*rabbitMQConnection),
		running:     make(map[string]*runningExecution),
		semaphore:   make(chan struct{}, cfg.MaxConcurrent),
		metrics:     &RabbitMQMetrics{},
	}
}

// Execute runs a job by publishing to RabbitMQ and returns the execution result.
func (d *RabbitMQDispatcher) Execute(ctx context.Context, job *models.Job, rmqCfg *RabbitMQConfig, scheduledTime time.Time) (*models.Execution, error) {
	if err := ValidateRabbitMQConfig(rmqCfg); err != nil {
		return nil, err
	}

	// Acquire semaphore
	select {
	case d.semaphore <- struct{}{}:
		defer func() { <-d.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	execution := &models.Execution{
		ID:            uuid.New().String(),
		JobID:         job.ID,
		JobName:       job.Name,
		ScheduledTime: scheduledTime,
		StartedAt:     time.Now(),
		Status:        models.ExecutionRunning,
		Attempts:      1,
		NodeID:        d.nodeID,
	}

	// Track running execution
	execCtx, cancel := context.WithCancel(ctx)
	d.trackExecution(execution, cancel)
	defer d.untrackExecution(execution.ID)

	// Set timeout
	timeout := rmqCfg.Timeout
	if timeout == 0 {
		timeout = d.config.DefaultTimeout
	}
	publishCtx, publishCancel := context.WithTimeout(execCtx, timeout)
	defer publishCancel()

	// Build message
	msg := &RabbitMQMessage{
		Exchange:        rmqCfg.Exchange,
		RoutingKey:      rmqCfg.RoutingKey,
		Body:            []byte(rmqCfg.Payload),
		ContentType:     rmqCfg.ContentType,
		ContentEncoding: rmqCfg.ContentEncoding,
		Priority:        rmqCfg.Priority,
		DeliveryMode:    rmqCfg.DeliveryMode,
		Expiration:      rmqCfg.Expiration,
		MessageID:       rmqCfg.MessageID,
		Timestamp:       time.Now(),
		AppID:           "chronos",
		Headers: map[string]interface{}{
			"x-chronos-job-id":         execution.JobID,
			"x-chronos-execution-id":   execution.ID,
			"x-chronos-scheduled-time": execution.ScheduledTime.Format(time.RFC3339),
			"x-chronos-attempt":        fmt.Sprintf("%d", execution.Attempts),
		},
	}

	// Set default content type
	if msg.ContentType == "" {
		msg.ContentType = "application/json"
	}

	// Set default delivery mode to persistent
	if msg.DeliveryMode == 0 {
		msg.DeliveryMode = 2
	}

	// Generate message ID if not provided
	if msg.MessageID == "" {
		msg.MessageID = execution.ID
	}

	// Merge custom headers
	for k, v := range rmqCfg.Headers {
		msg.Headers[k] = v
	}

	// If Queue is specified, use direct queue publishing
	if rmqCfg.Queue != "" {
		msg.Exchange = ""
		msg.RoutingKey = rmqCfg.Queue
	}

	// Publish the message
	result, err := d.publish(publishCtx, rmqCfg, msg)

	completedAt := time.Now()
	execution.CompletedAt = &completedAt
	execution.Duration = time.Since(execution.StartedAt)

	if err != nil {
		execution.Status = models.ExecutionFailed
		execution.Error = fmt.Sprintf("rabbitmq publish failed: %v", err)
		d.recordMetrics(false, false, false, 0, execution.Duration)
		return execution, ErrRabbitMQPublishFailed
	}

	if result.Returned {
		execution.Status = models.ExecutionFailed
		execution.Error = "message was returned (no matching queue)"
		d.recordMetrics(false, false, true, 0, execution.Duration)
		return execution, ErrRabbitMQPublishFailed
	}

	execution.Status = models.ExecutionSuccess
	if rmqCfg.ConfirmMode && result.Confirmed {
		execution.Response = fmt.Sprintf("Published to exchange '%s' with routing key '%s' (confirmed)", result.Exchange, result.RoutingKey)
	} else {
		execution.Response = fmt.Sprintf("Published to exchange '%s' with routing key '%s'", result.Exchange, result.RoutingKey)
	}
	d.recordMetrics(true, result.Confirmed, false, int64(len(msg.Body)), execution.Duration)

	return execution, nil
}

// publish sends a message to RabbitMQ.
func (d *RabbitMQDispatcher) publish(ctx context.Context, cfg *RabbitMQConfig, msg *RabbitMQMessage) (*RabbitMQPublishResult, error) {
	// Simulated implementation - real implementation would use amqp091-go
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(15 * time.Millisecond): // Simulate network latency + confirm
	}

	return &RabbitMQPublishResult{
		Exchange:   msg.Exchange,
		RoutingKey: msg.RoutingKey,
		Confirmed:  cfg.ConfirmMode,
		Returned:   false,
		Timestamp:  time.Now(),
	}, nil
}

// trackExecution adds an execution to the running map.
func (d *RabbitMQDispatcher) trackExecution(exec *models.Execution, cancel context.CancelFunc) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	d.running[exec.ID] = &runningExecution{
		Execution: exec,
		Cancel:    cancel,
		StartedAt: time.Now(),
	}
}

// untrackExecution removes an execution from the running map.
func (d *RabbitMQDispatcher) untrackExecution(execID string) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	delete(d.running, execID)
}

// recordMetrics records execution metrics.
func (d *RabbitMQDispatcher) recordMetrics(success, confirmed, returned bool, bytes int64, latency time.Duration) {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()
	if success {
		d.metrics.MessagesPublished++
		d.metrics.BytesSent += bytes
	} else {
		d.metrics.MessagesFailed++
	}
	if confirmed {
		d.metrics.MessagesConfirmed++
	}
	if returned {
		d.metrics.MessagesReturned++
	}
	d.metrics.LatencySum += latency
	d.metrics.LatencyCount++
}

// GetMetrics returns a snapshot of the current metrics.
func (d *RabbitMQDispatcher) GetMetrics() RabbitMQMetrics {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()
	return RabbitMQMetrics{
		MessagesPublished: d.metrics.MessagesPublished,
		MessagesFailed:    d.metrics.MessagesFailed,
		MessagesConfirmed: d.metrics.MessagesConfirmed,
		MessagesReturned:  d.metrics.MessagesReturned,
		BytesSent:         d.metrics.BytesSent,
		LatencySum:        d.metrics.LatencySum,
		LatencyCount:      d.metrics.LatencyCount,
	}
}

// Close closes all connections and releases resources.
func (d *RabbitMQDispatcher) Close() error {
	d.connectionsMu.Lock()
	defer d.connectionsMu.Unlock()

	for key := range d.connections {
		delete(d.connections, key)
	}

	return nil
}

// ValidateRabbitMQConfig validates a RabbitMQ configuration.
func ValidateRabbitMQConfig(cfg *RabbitMQConfig) error {
	if cfg == nil {
		return ErrInvalidRabbitMQConfig
	}
	if cfg.URL == "" {
		return fmt.Errorf("%w: URL required", ErrInvalidRabbitMQConfig)
	}
	// Either exchange+routing_key or queue must be specified
	if cfg.Exchange == "" && cfg.Queue == "" {
		return fmt.Errorf("%w: exchange or queue required", ErrInvalidRabbitMQConfig)
	}
	if cfg.Priority > 9 {
		return fmt.Errorf("%w: priority must be 0-9", ErrInvalidRabbitMQConfig)
	}
	if cfg.DeliveryMode != 0 && cfg.DeliveryMode != 1 && cfg.DeliveryMode != 2 {
		return fmt.Errorf("%w: delivery_mode must be 0, 1, or 2", ErrInvalidRabbitMQConfig)
	}
	return nil
}

// RabbitMQConfigFromJSON parses RabbitMQ configuration from JSON.
func RabbitMQConfigFromJSON(data []byte) (*RabbitMQConfig, error) {
	var cfg RabbitMQConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse RabbitMQ config: %w", err)
	}
	if err := ValidateRabbitMQConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
