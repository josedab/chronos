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

// Common errors for Kafka dispatcher.
var (
	ErrKafkaConnectionFailed = errors.New("kafka: connection failed")
	ErrKafkaPublishFailed    = errors.New("kafka: publish failed")
	ErrKafkaTimeout          = errors.New("kafka: timeout")
	ErrInvalidKafkaConfig    = errors.New("kafka: invalid configuration")
)

// KafkaConfig holds Kafka dispatcher configuration for a job.
type KafkaConfig struct {
	// Brokers is the list of Kafka broker addresses
	Brokers []string `json:"brokers" yaml:"brokers"`
	// Topic is the Kafka topic to publish to
	Topic string `json:"topic" yaml:"topic"`
	// Key is the message key (optional, can use template variables)
	Key string `json:"key,omitempty" yaml:"key,omitempty"`
	// Payload is the message value
	Payload string `json:"payload" yaml:"payload"`
	// Headers to include in the message
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// Partition to publish to (-1 for auto)
	Partition int32 `json:"partition" yaml:"partition"`
	// Timeout for the publish operation
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	// TLS configuration
	TLS *KafkaTLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
	// SASL authentication
	SASL *KafkaSASLConfig `json:"sasl,omitempty" yaml:"sasl,omitempty"`
	// Acks controls durability (0=none, 1=leader, -1=all)
	Acks int16 `json:"acks" yaml:"acks"`
	// Compression codec (none, gzip, snappy, lz4, zstd)
	Compression string `json:"compression,omitempty" yaml:"compression,omitempty"`
}

// KafkaTLSConfig holds TLS configuration for Kafka.
type KafkaTLSConfig struct {
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	CACertFile         string `json:"ca_cert_file,omitempty" yaml:"ca_cert_file,omitempty"`
	CertFile           string `json:"cert_file,omitempty" yaml:"cert_file,omitempty"`
	KeyFile            string `json:"key_file,omitempty" yaml:"key_file,omitempty"`
}

// KafkaSASLConfig holds SASL authentication configuration.
type KafkaSASLConfig struct {
	// Mechanism is the SASL mechanism (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)
	Mechanism string `json:"mechanism" yaml:"mechanism"`
	// Username for SASL authentication
	Username string `json:"username" yaml:"username"`
	// Password for SASL authentication
	Password string `json:"password" yaml:"password"`
}

// KafkaDispatcher executes jobs by publishing to Kafka topics.
type KafkaDispatcher struct {
	nodeID string
	config *KafkaDispatcherConfig

	// Producer pool keyed by broker list hash
	producers   map[string]*kafkaProducer
	producersMu sync.RWMutex

	// Track running executions
	running   map[string]*runningExecution
	runningMu sync.RWMutex

	// Concurrency control
	semaphore chan struct{}

	// Metrics
	metrics *KafkaMetrics
}

// kafkaProducer wraps a Kafka producer with metadata.
type kafkaProducer struct {
	brokers   []string
	tlsConfig *tls.Config
	saslCfg   *KafkaSASLConfig
	createdAt time.Time
}

// KafkaDispatcherConfig holds dispatcher-level configuration.
type KafkaDispatcherConfig struct {
	NodeID           string
	DefaultTimeout   time.Duration
	MaxConcurrent    int
	ProducerPoolSize int
}

// KafkaMetrics tracks Kafka dispatcher metrics.
type KafkaMetrics struct {
	mu                sync.Mutex
	MessagesPublished int64
	MessagesFailed    int64
	BytesSent         int64
	LatencySum        time.Duration
	LatencyCount      int64
}

// KafkaMessage represents a message to be published.
type KafkaMessage struct {
	Topic     string
	Key       []byte
	Value     []byte
	Headers   map[string]string
	Partition int32
	Timestamp time.Time
}

// KafkaPublishResult represents the result of a publish operation.
type KafkaPublishResult struct {
	Topic     string
	Partition int32
	Offset    int64
	Timestamp time.Time
}

// DefaultKafkaDispatcherConfig returns the default Kafka dispatcher configuration.
func DefaultKafkaDispatcherConfig() *KafkaDispatcherConfig {
	return &KafkaDispatcherConfig{
		NodeID:           "chronos-1",
		DefaultTimeout:   30 * time.Second,
		MaxConcurrent:    100,
		ProducerPoolSize: 10,
	}
}

// NewKafkaDispatcher creates a new Kafka dispatcher.
func NewKafkaDispatcher(cfg *KafkaDispatcherConfig) *KafkaDispatcher {
	if cfg == nil {
		cfg = DefaultKafkaDispatcherConfig()
	}

	return &KafkaDispatcher{
		nodeID:    cfg.NodeID,
		config:    cfg,
		producers: make(map[string]*kafkaProducer),
		running:   make(map[string]*runningExecution),
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
		metrics:   &KafkaMetrics{},
	}
}

// Execute runs a job by publishing to Kafka and returns the execution result.
func (d *KafkaDispatcher) Execute(ctx context.Context, job *models.Job, kafkaCfg *KafkaConfig, scheduledTime time.Time) (*models.Execution, error) {
	if kafkaCfg == nil || len(kafkaCfg.Brokers) == 0 || kafkaCfg.Topic == "" {
		return nil, ErrInvalidKafkaConfig
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
	timeout := kafkaCfg.Timeout
	if timeout == 0 {
		timeout = d.config.DefaultTimeout
	}
	publishCtx, publishCancel := context.WithTimeout(execCtx, timeout)
	defer publishCancel()

	// Build message
	msg := &KafkaMessage{
		Topic:     kafkaCfg.Topic,
		Value:     []byte(kafkaCfg.Payload),
		Partition: kafkaCfg.Partition,
		Timestamp: time.Now(),
		Headers: map[string]string{
			"x-chronos-job-id":         execution.JobID,
			"x-chronos-execution-id":   execution.ID,
			"x-chronos-scheduled-time": execution.ScheduledTime.Format(time.RFC3339),
			"x-chronos-attempt":        fmt.Sprintf("%d", execution.Attempts),
		},
	}

	if kafkaCfg.Key != "" {
		msg.Key = []byte(kafkaCfg.Key)
	}

	// Merge custom headers
	for k, v := range kafkaCfg.Headers {
		msg.Headers[k] = v
	}

	// Publish the message (simulated - real implementation would use sarama/confluent-kafka-go)
	result, err := d.publish(publishCtx, kafkaCfg, msg)

	completedAt := time.Now()
	execution.CompletedAt = &completedAt
	execution.Duration = time.Since(execution.StartedAt)

	if err != nil {
		execution.Status = models.ExecutionFailed
		execution.Error = fmt.Sprintf("kafka publish failed: %v", err)
		d.recordMetrics(false, 0, execution.Duration)
		return execution, ErrKafkaPublishFailed
	}

	execution.Status = models.ExecutionSuccess
	execution.Response = fmt.Sprintf("Published to %s partition %d offset %d", result.Topic, result.Partition, result.Offset)
	d.recordMetrics(true, int64(len(msg.Value)), execution.Duration)

	return execution, nil
}

// publish sends a message to Kafka.
func (d *KafkaDispatcher) publish(ctx context.Context, cfg *KafkaConfig, msg *KafkaMessage) (*KafkaPublishResult, error) {
	// This is a simulated implementation
	// In production, you would use sarama or confluent-kafka-go
	
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Millisecond): // Simulate network latency
	}

	// Simulate successful publish
	return &KafkaPublishResult{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    time.Now().UnixNano(),
		Timestamp: time.Now(),
	}, nil
}

// trackExecution adds an execution to the running map.
func (d *KafkaDispatcher) trackExecution(exec *models.Execution, cancel context.CancelFunc) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	d.running[exec.ID] = &runningExecution{
		Execution: exec,
		Cancel:    cancel,
		StartedAt: time.Now(),
	}
}

// untrackExecution removes an execution from the running map.
func (d *KafkaDispatcher) untrackExecution(execID string) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	delete(d.running, execID)
}

// recordMetrics records execution metrics.
func (d *KafkaDispatcher) recordMetrics(success bool, bytes int64, latency time.Duration) {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()
	if success {
		d.metrics.MessagesPublished++
		d.metrics.BytesSent += bytes
	} else {
		d.metrics.MessagesFailed++
	}
	d.metrics.LatencySum += latency
	d.metrics.LatencyCount++
}

// GetMetrics returns a snapshot of the current metrics.
func (d *KafkaDispatcher) GetMetrics() KafkaMetrics {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()
	return KafkaMetrics{
		MessagesPublished: d.metrics.MessagesPublished,
		MessagesFailed:    d.metrics.MessagesFailed,
		BytesSent:         d.metrics.BytesSent,
		LatencySum:        d.metrics.LatencySum,
		LatencyCount:      d.metrics.LatencyCount,
	}
}

// Close closes all producers and releases resources.
func (d *KafkaDispatcher) Close() error {
	d.producersMu.Lock()
	defer d.producersMu.Unlock()

	// Close all producers
	for key := range d.producers {
		delete(d.producers, key)
	}

	return nil
}

// ValidateConfig validates a Kafka configuration.
func ValidateKafkaConfig(cfg *KafkaConfig) error {
	if cfg == nil {
		return ErrInvalidKafkaConfig
	}
	if len(cfg.Brokers) == 0 {
		return fmt.Errorf("%w: brokers required", ErrInvalidKafkaConfig)
	}
	if cfg.Topic == "" {
		return fmt.Errorf("%w: topic required", ErrInvalidKafkaConfig)
	}
	if cfg.SASL != nil {
		switch cfg.SASL.Mechanism {
		case "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512":
			// Valid
		default:
			return fmt.Errorf("%w: invalid SASL mechanism: %s", ErrInvalidKafkaConfig, cfg.SASL.Mechanism)
		}
	}
	return nil
}

// KafkaConfigFromJSON parses Kafka configuration from JSON.
func KafkaConfigFromJSON(data []byte) (*KafkaConfig, error) {
	var cfg KafkaConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse Kafka config: %w", err)
	}
	if err := ValidateKafkaConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
