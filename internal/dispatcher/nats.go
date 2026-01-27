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

// Common errors for NATS dispatcher.
var (
	ErrNATSConnectionFailed = errors.New("nats: connection failed")
	ErrNATSPublishFailed    = errors.New("nats: publish failed")
	ErrNATSTimeout          = errors.New("nats: timeout")
	ErrInvalidNATSConfig    = errors.New("nats: invalid configuration")
)

// NATSConfig holds NATS dispatcher configuration for a job.
type NATSConfig struct {
	// URL is the NATS server URL (e.g., nats://localhost:4222)
	URL string `json:"url" yaml:"url"`
	// Subject is the NATS subject to publish to
	Subject string `json:"subject" yaml:"subject"`
	// Payload is the message data
	Payload string `json:"payload" yaml:"payload"`
	// Headers to include in the message
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	// Timeout for the publish operation
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	// TLS configuration
	TLS *NATSTLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
	// Auth configuration
	Auth *NATSAuthConfig `json:"auth,omitempty" yaml:"auth,omitempty"`
	// JetStream configuration (optional, for durable messaging)
	JetStream *NATSJetStreamConfig `json:"jetstream,omitempty" yaml:"jetstream,omitempty"`
	// RequestReply enables request-reply pattern
	RequestReply bool `json:"request_reply" yaml:"request_reply"`
	// ReplyTimeout for request-reply pattern
	ReplyTimeout time.Duration `json:"reply_timeout,omitempty" yaml:"reply_timeout,omitempty"`
}

// NATSTLSConfig holds TLS configuration for NATS.
type NATSTLSConfig struct {
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	CACertFile         string `json:"ca_cert_file,omitempty" yaml:"ca_cert_file,omitempty"`
	CertFile           string `json:"cert_file,omitempty" yaml:"cert_file,omitempty"`
	KeyFile            string `json:"key_file,omitempty" yaml:"key_file,omitempty"`
}

// NATSAuthConfig holds authentication configuration for NATS.
type NATSAuthConfig struct {
	// Type is the authentication type (token, userpass, nkey, jwt)
	Type string `json:"type" yaml:"type"`
	// Token for token authentication
	Token string `json:"token,omitempty" yaml:"token,omitempty"`
	// Username for userpass authentication
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	// Password for userpass authentication
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	// NKey seed for nkey authentication
	NKeySeed string `json:"nkey_seed,omitempty" yaml:"nkey_seed,omitempty"`
	// JWT for JWT authentication
	JWT string `json:"jwt,omitempty" yaml:"jwt,omitempty"`
	// CredentialsFile path to credentials file
	CredentialsFile string `json:"credentials_file,omitempty" yaml:"credentials_file,omitempty"`
}

// NATSJetStreamConfig holds JetStream configuration.
type NATSJetStreamConfig struct {
	// Enabled enables JetStream publishing
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Stream is the stream name
	Stream string `json:"stream,omitempty" yaml:"stream,omitempty"`
	// MsgID for deduplication
	MsgID string `json:"msg_id,omitempty" yaml:"msg_id,omitempty"`
	// ExpectStream ensures the message goes to a specific stream
	ExpectStream string `json:"expect_stream,omitempty" yaml:"expect_stream,omitempty"`
	// ExpectLastMsgID for optimistic concurrency
	ExpectLastMsgID string `json:"expect_last_msg_id,omitempty" yaml:"expect_last_msg_id,omitempty"`
}

// NATSDispatcher executes jobs by publishing to NATS subjects.
type NATSDispatcher struct {
	nodeID string
	config *NATSDispatcherConfig

	// Connection pool
	connections   map[string]*natsConnection
	connectionsMu sync.RWMutex

	// Track running executions
	running   map[string]*runningExecution
	runningMu sync.RWMutex

	// Concurrency control
	semaphore chan struct{}

	// Metrics
	metrics *NATSMetrics
}

// natsConnection wraps a NATS connection with metadata.
type natsConnection struct {
	url       string
	tlsConfig *tls.Config
	authCfg   *NATSAuthConfig
	createdAt time.Time
	connected bool
}

// NATSDispatcherConfig holds dispatcher-level configuration.
type NATSDispatcherConfig struct {
	NodeID             string
	DefaultTimeout     time.Duration
	MaxConcurrent      int
	ReconnectWait      time.Duration
	MaxReconnects      int
	PingInterval       time.Duration
	MaxPingsOut        int
	ConnectionPoolSize int
}

// NATSMetrics tracks NATS dispatcher metrics.
type NATSMetrics struct {
	mu                sync.Mutex
	MessagesPublished int64
	MessagesFailed    int64
	BytesSent         int64
	RequestReplies    int64
	LatencySum        time.Duration
	LatencyCount      int64
}

// NATSMessage represents a message to be published.
type NATSMessage struct {
	Subject string
	Reply   string
	Data    []byte
	Headers map[string]string
}

// NATSPublishResult represents the result of a publish operation.
type NATSPublishResult struct {
	Subject   string
	Reply     string
	Timestamp time.Time
	// JetStream-specific
	Stream   string
	Sequence uint64
}

// DefaultNATSDispatcherConfig returns the default NATS dispatcher configuration.
func DefaultNATSDispatcherConfig() *NATSDispatcherConfig {
	return &NATSDispatcherConfig{
		NodeID:             "chronos-1",
		DefaultTimeout:     30 * time.Second,
		MaxConcurrent:      100,
		ReconnectWait:      2 * time.Second,
		MaxReconnects:      60,
		PingInterval:       2 * time.Minute,
		MaxPingsOut:        2,
		ConnectionPoolSize: 10,
	}
}

// NewNATSDispatcher creates a new NATS dispatcher.
func NewNATSDispatcher(cfg *NATSDispatcherConfig) *NATSDispatcher {
	if cfg == nil {
		cfg = DefaultNATSDispatcherConfig()
	}

	return &NATSDispatcher{
		nodeID:      cfg.NodeID,
		config:      cfg,
		connections: make(map[string]*natsConnection),
		running:     make(map[string]*runningExecution),
		semaphore:   make(chan struct{}, cfg.MaxConcurrent),
		metrics:     &NATSMetrics{},
	}
}

// Execute runs a job by publishing to NATS and returns the execution result.
func (d *NATSDispatcher) Execute(ctx context.Context, job *models.Job, natsCfg *NATSConfig, scheduledTime time.Time) (*models.Execution, error) {
	if natsCfg == nil || natsCfg.URL == "" || natsCfg.Subject == "" {
		return nil, ErrInvalidNATSConfig
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
	timeout := natsCfg.Timeout
	if timeout == 0 {
		timeout = d.config.DefaultTimeout
	}
	publishCtx, publishCancel := context.WithTimeout(execCtx, timeout)
	defer publishCancel()

	// Build message
	msg := &NATSMessage{
		Subject: natsCfg.Subject,
		Data:    []byte(natsCfg.Payload),
		Headers: map[string]string{
			"X-Chronos-Job-ID":         execution.JobID,
			"X-Chronos-Execution-ID":   execution.ID,
			"X-Chronos-Scheduled-Time": execution.ScheduledTime.Format(time.RFC3339),
			"X-Chronos-Attempt":        fmt.Sprintf("%d", execution.Attempts),
		},
	}

	// Merge custom headers
	for k, v := range natsCfg.Headers {
		msg.Headers[k] = v
	}

	var result *NATSPublishResult
	var err error

	if natsCfg.RequestReply {
		// Request-reply pattern
		replyTimeout := natsCfg.ReplyTimeout
		if replyTimeout == 0 {
			replyTimeout = 5 * time.Second
		}
		result, err = d.request(publishCtx, natsCfg, msg, replyTimeout)
		if err == nil {
			d.metrics.mu.Lock()
			d.metrics.RequestReplies++
			d.metrics.mu.Unlock()
		}
	} else if natsCfg.JetStream != nil && natsCfg.JetStream.Enabled {
		// JetStream publish
		result, err = d.publishJetStream(publishCtx, natsCfg, msg)
	} else {
		// Standard publish
		result, err = d.publish(publishCtx, natsCfg, msg)
	}

	completedAt := time.Now()
	execution.CompletedAt = &completedAt
	execution.Duration = time.Since(execution.StartedAt)

	if err != nil {
		execution.Status = models.ExecutionFailed
		execution.Error = fmt.Sprintf("nats publish failed: %v", err)
		d.recordMetrics(false, 0, execution.Duration)
		return execution, ErrNATSPublishFailed
	}

	execution.Status = models.ExecutionSuccess
	if result.Stream != "" {
		execution.Response = fmt.Sprintf("Published to %s (stream: %s, seq: %d)", result.Subject, result.Stream, result.Sequence)
	} else {
		execution.Response = fmt.Sprintf("Published to %s", result.Subject)
	}
	d.recordMetrics(true, int64(len(msg.Data)), execution.Duration)

	return execution, nil
}

// publish sends a message to NATS.
func (d *NATSDispatcher) publish(ctx context.Context, cfg *NATSConfig, msg *NATSMessage) (*NATSPublishResult, error) {
	// Simulated implementation - real implementation would use nats.go
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Millisecond): // Simulate network latency
	}

	return &NATSPublishResult{
		Subject:   msg.Subject,
		Timestamp: time.Now(),
	}, nil
}

// publishJetStream sends a message to NATS JetStream.
func (d *NATSDispatcher) publishJetStream(ctx context.Context, cfg *NATSConfig, msg *NATSMessage) (*NATSPublishResult, error) {
	// Simulated implementation - real implementation would use nats.go JetStream
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Millisecond): // Simulate network latency
	}

	return &NATSPublishResult{
		Subject:   msg.Subject,
		Stream:    cfg.JetStream.Stream,
		Sequence:  uint64(time.Now().UnixNano()),
		Timestamp: time.Now(),
	}, nil
}

// request sends a request and waits for a reply.
func (d *NATSDispatcher) request(ctx context.Context, cfg *NATSConfig, msg *NATSMessage, timeout time.Duration) (*NATSPublishResult, error) {
	// Simulated implementation - real implementation would use nats.go
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-reqCtx.Done():
		return nil, reqCtx.Err()
	case <-time.After(20 * time.Millisecond): // Simulate request-reply latency
	}

	return &NATSPublishResult{
		Subject:   msg.Subject,
		Reply:     "reply-data",
		Timestamp: time.Now(),
	}, nil
}

// trackExecution adds an execution to the running map.
func (d *NATSDispatcher) trackExecution(exec *models.Execution, cancel context.CancelFunc) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	d.running[exec.ID] = &runningExecution{
		Execution: exec,
		Cancel:    cancel,
		StartedAt: time.Now(),
	}
}

// untrackExecution removes an execution from the running map.
func (d *NATSDispatcher) untrackExecution(execID string) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	delete(d.running, execID)
}

// recordMetrics records execution metrics.
func (d *NATSDispatcher) recordMetrics(success bool, bytes int64, latency time.Duration) {
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
func (d *NATSDispatcher) GetMetrics() NATSMetrics {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()
	return NATSMetrics{
		MessagesPublished: d.metrics.MessagesPublished,
		MessagesFailed:    d.metrics.MessagesFailed,
		BytesSent:         d.metrics.BytesSent,
		RequestReplies:    d.metrics.RequestReplies,
		LatencySum:        d.metrics.LatencySum,
		LatencyCount:      d.metrics.LatencyCount,
	}
}

// Close closes all connections and releases resources.
func (d *NATSDispatcher) Close() error {
	d.connectionsMu.Lock()
	defer d.connectionsMu.Unlock()

	for key := range d.connections {
		delete(d.connections, key)
	}

	return nil
}

// ValidateNATSConfig validates a NATS configuration.
func ValidateNATSConfig(cfg *NATSConfig) error {
	if cfg == nil {
		return ErrInvalidNATSConfig
	}
	if cfg.URL == "" {
		return fmt.Errorf("%w: URL required", ErrInvalidNATSConfig)
	}
	if cfg.Subject == "" {
		return fmt.Errorf("%w: subject required", ErrInvalidNATSConfig)
	}
	if cfg.Auth != nil {
		switch cfg.Auth.Type {
		case "token", "userpass", "nkey", "jwt", "credentials":
			// Valid
		case "":
			// No auth type specified
		default:
			return fmt.Errorf("%w: invalid auth type: %s", ErrInvalidNATSConfig, cfg.Auth.Type)
		}
	}
	return nil
}

// NATSConfigFromJSON parses NATS configuration from JSON.
func NATSConfigFromJSON(data []byte) (*NATSConfig, error) {
	var cfg NATSConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse NATS config: %w", err)
	}
	if err := ValidateNATSConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
