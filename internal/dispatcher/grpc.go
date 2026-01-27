// Package dispatcher handles job execution via various protocols.
package dispatcher

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Common errors for gRPC dispatcher.
var (
	ErrGRPCConnectionFailed = errors.New("grpc: connection failed")
	ErrGRPCInvocationFailed = errors.New("grpc: invocation failed")
	ErrGRPCTimeout          = errors.New("grpc: timeout")
	ErrInvalidGRPCConfig    = errors.New("grpc: invalid configuration")
)

// GRPCConfig holds gRPC dispatcher configuration.
type GRPCConfig struct {
	// Target is the gRPC server address (host:port)
	Target string `json:"target" yaml:"target"`
	// Method is the full gRPC method name (e.g., /package.Service/Method)
	Method string `json:"method" yaml:"method"`
	// Timeout for the RPC call
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	// TLS configuration
	TLS *GRPCTLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
	// Metadata to send with the request
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	// Request payload (JSON encoded, will be sent as bytes)
	Payload string `json:"payload,omitempty" yaml:"payload,omitempty"`
	// MaxRetries for connection attempts
	MaxRetries int `json:"max_retries" yaml:"max_retries"`
	// KeepAlive configuration
	KeepAlive *GRPCKeepAliveConfig `json:"keep_alive,omitempty" yaml:"keep_alive,omitempty"`
}

// GRPCTLSConfig holds TLS configuration for gRPC.
type GRPCTLSConfig struct {
	// Enabled enables TLS
	Enabled bool `json:"enabled" yaml:"enabled"`
	// InsecureSkipVerify skips certificate verification
	InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	// CACertFile is the path to the CA certificate file
	CACertFile string `json:"ca_cert_file,omitempty" yaml:"ca_cert_file,omitempty"`
	// CertFile is the path to the client certificate file
	CertFile string `json:"cert_file,omitempty" yaml:"cert_file,omitempty"`
	// KeyFile is the path to the client key file
	KeyFile string `json:"key_file,omitempty" yaml:"key_file,omitempty"`
	// ServerName for SNI
	ServerName string `json:"server_name,omitempty" yaml:"server_name,omitempty"`
}

// GRPCKeepAliveConfig holds keep-alive settings.
type GRPCKeepAliveConfig struct {
	Time                time.Duration `json:"time" yaml:"time"`
	Timeout             time.Duration `json:"timeout" yaml:"timeout"`
	PermitWithoutStream bool          `json:"permit_without_stream" yaml:"permit_without_stream"`
}

// GRPCDispatcher executes jobs via gRPC calls.
type GRPCDispatcher struct {
	nodeID string
	config *GRPCDispatcherConfig

	// Connection pool
	connPool   map[string]*grpc.ClientConn
	connPoolMu sync.RWMutex

	// Track running executions
	running   map[string]*runningExecution
	runningMu sync.RWMutex

	// Concurrency control
	semaphore chan struct{}

	// Metrics
	metrics *GRPCMetrics
}

// GRPCDispatcherConfig holds the dispatcher-level configuration.
type GRPCDispatcherConfig struct {
	NodeID            string
	DefaultTimeout    time.Duration
	MaxConcurrent     int
	MaxConnectionAge  time.Duration
	ConnectionTimeout time.Duration
}

// GRPCMetrics tracks gRPC dispatcher metrics.
type GRPCMetrics struct {
	mu                sync.Mutex
	ExecutionsTotal   int64
	ExecutionsSuccess int64
	ExecutionsFailed  int64
	ConnectionsActive int64
	LatencySum        time.Duration
	LatencyCount      int64
}

// DefaultGRPCDispatcherConfig returns the default gRPC dispatcher configuration.
func DefaultGRPCDispatcherConfig() *GRPCDispatcherConfig {
	return &GRPCDispatcherConfig{
		NodeID:            "chronos-1",
		DefaultTimeout:    30 * time.Second,
		MaxConcurrent:     100,
		MaxConnectionAge:  5 * time.Minute,
		ConnectionTimeout: 10 * time.Second,
	}
}

// NewGRPCDispatcher creates a new gRPC dispatcher.
func NewGRPCDispatcher(cfg *GRPCDispatcherConfig) *GRPCDispatcher {
	if cfg == nil {
		cfg = DefaultGRPCDispatcherConfig()
	}

	return &GRPCDispatcher{
		nodeID:    cfg.NodeID,
		config:    cfg,
		connPool:  make(map[string]*grpc.ClientConn),
		running:   make(map[string]*runningExecution),
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
		metrics:   &GRPCMetrics{},
	}
}

// Execute runs a job via gRPC and returns the execution result.
func (d *GRPCDispatcher) Execute(ctx context.Context, job *models.Job, grpcCfg *GRPCConfig, scheduledTime time.Time) (*models.Execution, error) {
	if grpcCfg == nil || grpcCfg.Target == "" || grpcCfg.Method == "" {
		return nil, ErrInvalidGRPCConfig
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

	// Get or create connection
	conn, err := d.getConnection(execCtx, grpcCfg)
	if err != nil {
		execution.Status = models.ExecutionFailed
		execution.Error = fmt.Sprintf("connection failed: %v", err)
		completedAt := time.Now()
		execution.CompletedAt = &completedAt
		execution.Duration = time.Since(execution.StartedAt)
		d.recordMetrics(false, execution.Duration)
		return execution, ErrGRPCConnectionFailed
	}

	// Set timeout
	timeout := grpcCfg.Timeout
	if timeout == 0 {
		timeout = d.config.DefaultTimeout
	}
	callCtx, callCancel := context.WithTimeout(execCtx, timeout)
	defer callCancel()

	// Add metadata
	md := metadata.New(map[string]string{
		"x-chronos-job-id":        execution.JobID,
		"x-chronos-execution-id":  execution.ID,
		"x-chronos-scheduled-time": execution.ScheduledTime.Format(time.RFC3339),
		"x-chronos-attempt":       fmt.Sprintf("%d", execution.Attempts),
	})
	for k, v := range grpcCfg.Metadata {
		md.Set(k, v)
	}
	callCtx = metadata.NewOutgoingContext(callCtx, md)

	// Invoke the RPC
	var response []byte
	err = conn.Invoke(callCtx, grpcCfg.Method, []byte(grpcCfg.Payload), &response)

	completedAt := time.Now()
	execution.CompletedAt = &completedAt
	execution.Duration = time.Since(execution.StartedAt)

	if err != nil {
		execution.Status = models.ExecutionFailed
		st, ok := status.FromError(err)
		if ok {
			execution.StatusCode = int(st.Code())
			execution.Error = fmt.Sprintf("gRPC error [%s]: %s", st.Code().String(), st.Message())

			// Check if retriable
			if d.isRetriableCode(st.Code()) {
				execution.Error += " (retriable)"
			}
		} else {
			execution.Error = err.Error()
		}
		d.recordMetrics(false, execution.Duration)
		return execution, ErrGRPCInvocationFailed
	}

	execution.Status = models.ExecutionSuccess
	execution.Response = string(response)
	execution.StatusCode = int(codes.OK)
	d.recordMetrics(true, execution.Duration)

	return execution, nil
}

// getConnection retrieves or creates a gRPC connection.
func (d *GRPCDispatcher) getConnection(ctx context.Context, cfg *GRPCConfig) (*grpc.ClientConn, error) {
	d.connPoolMu.RLock()
	conn, exists := d.connPool[cfg.Target]
	d.connPoolMu.RUnlock()

	if exists && conn.GetState().String() != "SHUTDOWN" {
		return conn, nil
	}

	d.connPoolMu.Lock()
	defer d.connPoolMu.Unlock()

	// Double-check after acquiring write lock
	conn, exists = d.connPool[cfg.Target]
	if exists && conn.GetState().String() != "SHUTDOWN" {
		return conn, nil
	}

	// Create new connection
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.ForceCodec(rawCodec{})),
	}

	// Configure TLS
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsCreds, err := d.buildTLSCredentials(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(tlsCreds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	connCtx, cancel := context.WithTimeout(ctx, d.config.ConnectionTimeout)
	defer cancel()

	conn, err := grpc.DialContext(connCtx, cfg.Target, opts...)
	if err != nil {
		return nil, err
	}

	d.connPool[cfg.Target] = conn
	d.metrics.mu.Lock()
	d.metrics.ConnectionsActive++
	d.metrics.mu.Unlock()

	return conn, nil
}

// buildTLSCredentials creates TLS credentials from config.
func (d *GRPCDispatcher) buildTLSCredentials(cfg *GRPCTLSConfig) (credentials.TransportCredentials, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         cfg.ServerName,
	}

	// Load CA cert if provided
	if cfg.CACertFile != "" {
		caCert, err := os.ReadFile(cfg.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to parse CA cert")
		}
		tlsConfig.RootCAs = certPool
	}

	// Load client cert if provided
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return credentials.NewTLS(tlsConfig), nil
}

// isRetriableCode checks if a gRPC status code is retriable.
func (d *GRPCDispatcher) isRetriableCode(code codes.Code) bool {
	switch code {
	case codes.Unavailable, codes.ResourceExhausted, codes.Aborted, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

// trackExecution adds an execution to the running map.
func (d *GRPCDispatcher) trackExecution(exec *models.Execution, cancel context.CancelFunc) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	d.running[exec.ID] = &runningExecution{
		Execution: exec,
		Cancel:    cancel,
		StartedAt: time.Now(),
	}
}

// untrackExecution removes an execution from the running map.
func (d *GRPCDispatcher) untrackExecution(execID string) {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	delete(d.running, execID)
}

// recordMetrics records execution metrics.
func (d *GRPCDispatcher) recordMetrics(success bool, latency time.Duration) {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()
	d.metrics.ExecutionsTotal++
	if success {
		d.metrics.ExecutionsSuccess++
	} else {
		d.metrics.ExecutionsFailed++
	}
	d.metrics.LatencySum += latency
	d.metrics.LatencyCount++
}

// GetMetrics returns a snapshot of the current metrics.
func (d *GRPCDispatcher) GetMetrics() GRPCMetrics {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()
	return GRPCMetrics{
		ExecutionsTotal:   d.metrics.ExecutionsTotal,
		ExecutionsSuccess: d.metrics.ExecutionsSuccess,
		ExecutionsFailed:  d.metrics.ExecutionsFailed,
		ConnectionsActive: d.metrics.ConnectionsActive,
		LatencySum:        d.metrics.LatencySum,
		LatencyCount:      d.metrics.LatencyCount,
	}
}

// Close closes all connections and releases resources.
func (d *GRPCDispatcher) Close() error {
	d.connPoolMu.Lock()
	defer d.connPoolMu.Unlock()

	var errs []error
	for target, conn := range d.connPool {
		if err := conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close connection to %s: %w", target, err))
		}
		delete(d.connPool, target)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// rawCodec is a codec that passes bytes through without encoding.
type rawCodec struct{}

func (rawCodec) Marshal(v interface{}) ([]byte, error) {
	if b, ok := v.([]byte); ok {
		return b, nil
	}
	if b, ok := v.(*[]byte); ok {
		return *b, nil
	}
	return nil, fmt.Errorf("rawCodec: unsupported type %T", v)
}

func (rawCodec) Unmarshal(data []byte, v interface{}) error {
	if b, ok := v.(*[]byte); ok {
		*b = data
		return nil
	}
	return fmt.Errorf("rawCodec: unsupported type %T", v)
}

func (rawCodec) Name() string {
	return "raw"
}
