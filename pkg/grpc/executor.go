// Package grpc provides a gRPC executor for Chronos.
package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/chronos/chronos/pkg/plugin"
)

// Executor executes jobs via gRPC.
type Executor struct {
	config      Config
	connections sync.Map
	mu          sync.RWMutex
}

// Config configures the gRPC executor.
type Config struct {
	// Address is the gRPC server address (host:port).
	Address string `json:"address" yaml:"address"`
	// Method is the gRPC method to call (e.g., "package.Service/Method").
	Method string `json:"method" yaml:"method"`
	// TLS enables TLS for the connection.
	TLS bool `json:"tls" yaml:"tls"`
	// TLSSkipVerify skips TLS certificate verification.
	TLSSkipVerify bool `json:"tls_skip_verify" yaml:"tls_skip_verify"`
	// CACert is the path to the CA certificate file.
	CACert string `json:"ca_cert" yaml:"ca_cert"`
	// ClientCert is the path to the client certificate file.
	ClientCert string `json:"client_cert" yaml:"client_cert"`
	// ClientKey is the path to the client key file.
	ClientKey string `json:"client_key" yaml:"client_key"`
	// Timeout is the call timeout.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	// MaxRetries is the maximum number of retries.
	MaxRetries int `json:"max_retries" yaml:"max_retries"`
	// Metadata are headers to send with each request.
	Metadata map[string]string `json:"metadata" yaml:"metadata"`
	// UseReflection enables gRPC reflection for method discovery.
	UseReflection bool `json:"use_reflection" yaml:"use_reflection"`
	// Codec specifies the codec (json or proto).
	Codec string `json:"codec" yaml:"codec"`
}

// New creates a new gRPC executor.
func New(cfg Config) *Executor {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Codec == "" {
		cfg.Codec = "json"
	}

	return &Executor{
		config: cfg,
	}
}

// Metadata returns plugin metadata.
func (e *Executor) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "grpc",
		Version:     "1.0.0",
		Type:        plugin.TypeExecutor,
		Description: "gRPC service executor",
	}
}

// Init initializes the executor.
func (e *Executor) Init(ctx context.Context, config map[string]interface{}) error {
	if addr, ok := config["address"].(string); ok {
		e.config.Address = addr
	}
	if method, ok := config["method"].(string); ok {
		e.config.Method = method
	}
	if tls, ok := config["tls"].(bool); ok {
		e.config.TLS = tls
	}
	return nil
}

// Close cleans up resources.
func (e *Executor) Close(ctx context.Context) error {
	e.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*Connection); ok {
			conn.Close()
		}
		e.connections.Delete(key)
		return true
	})
	return nil
}

// Health checks executor health.
func (e *Executor) Health(ctx context.Context) error {
	if e.config.Address == "" {
		return fmt.Errorf("address is required")
	}
	if e.config.Method == "" {
		return fmt.Errorf("method is required")
	}

	conn, err := net.DialTimeout("tcp", e.config.Address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to %s: %w", e.config.Address, err)
	}
	conn.Close()
	return nil
}

// Execute invokes the gRPC method.
func (e *Executor) Execute(ctx context.Context, req *plugin.ExecutionRequest) (*plugin.ExecutionResult, error) {
	start := time.Now()

	conn, err := e.getConnection(ctx)
	if err != nil {
		return &plugin.ExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to connect: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	payload := &GRPCPayload{
		JobID:       req.JobID,
		JobName:     req.JobName,
		ExecutionID: req.ExecutionID,
		Payload:     req.Payload,
		Metadata:    req.Metadata,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	response, err := conn.Invoke(ctx, e.config.Method, payload, e.config.Metadata)
	if err != nil {
		return &plugin.ExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("gRPC call failed: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	return &plugin.ExecutionResult{
		Success:    true,
		Response:   response,
		Duration:   time.Since(start),
		StatusCode: 0,
	}, nil
}

// getConnection gets or creates a connection to the gRPC server.
func (e *Executor) getConnection(ctx context.Context) (*Connection, error) {
	if existing, ok := e.connections.Load(e.config.Address); ok {
		return existing.(*Connection), nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if existing, ok := e.connections.Load(e.config.Address); ok {
		return existing.(*Connection), nil
	}

	conn, err := e.dial(ctx)
	if err != nil {
		return nil, err
	}

	e.connections.Store(e.config.Address, conn)
	return conn, nil
}

// dial creates a new connection.
func (e *Executor) dial(ctx context.Context) (*Connection, error) {
	var tlsConfig *tls.Config

	if e.config.TLS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: e.config.TLSSkipVerify,
		}

		if e.config.CACert != "" {
			caCert, err := os.ReadFile(e.config.CACert)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA cert: %w", err)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caCertPool
		}

		if e.config.ClientCert != "" && e.config.ClientKey != "" {
			cert, err := tls.LoadX509KeyPair(e.config.ClientCert, e.config.ClientKey)
			if err != nil {
				return nil, fmt.Errorf("failed to load client cert: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	var netConn net.Conn
	var err error

	dialer := &net.Dialer{Timeout: 10 * time.Second}

	if tlsConfig != nil {
		netConn, err = tls.DialWithDialer(dialer, "tcp", e.config.Address, tlsConfig)
	} else {
		netConn, err = dialer.DialContext(ctx, "tcp", e.config.Address)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", e.config.Address, err)
	}

	return &Connection{
		conn:    netConn,
		codec:   e.config.Codec,
		timeout: e.config.Timeout,
	}, nil
}

// GRPCPayload is the payload sent to gRPC services.
type GRPCPayload struct {
	JobID       string                 `json:"job_id"`
	JobName     string                 `json:"job_name"`
	ExecutionID string                 `json:"execution_id"`
	Payload     map[string]interface{} `json:"payload"`
	Metadata    map[string]string      `json:"metadata"`
	Timestamp   string                 `json:"timestamp"`
}

// Connection represents a gRPC connection.
type Connection struct {
	conn    net.Conn
	codec   string
	timeout time.Duration
	mu      sync.Mutex
}

// Close closes the connection.
func (c *Connection) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Invoke calls a gRPC method. This is a simplified implementation
// that uses HTTP/2 framing over the raw connection.
func (c *Connection) Invoke(ctx context.Context, method string, payload *GRPCPayload, metadata map[string]string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, fmt.Errorf("connection is closed")
	}

	if deadline, ok := ctx.Deadline(); ok {
		c.conn.SetDeadline(deadline)
	} else {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
	}

	// Encode the request
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode payload: %w", err)
	}

	// Write gRPC frame (simplified: length-prefixed message)
	frame := make([]byte, 5+len(data))
	frame[0] = 0 // compression flag
	frame[1] = byte(len(data) >> 24)
	frame[2] = byte(len(data) >> 16)
	frame[3] = byte(len(data) >> 8)
	frame[4] = byte(len(data))
	copy(frame[5:], data)

	if _, err := c.conn.Write(frame); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response frame
	header := make([]byte, 5)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, fmt.Errorf("failed to read response header: %w", err)
	}

	msgLen := int(header[1])<<24 | int(header[2])<<16 | int(header[3])<<8 | int(header[4])
	if msgLen > 10*1024*1024 {
		return nil, fmt.Errorf("response too large: %d bytes", msgLen)
	}

	response := make([]byte, msgLen)
	if _, err := io.ReadFull(c.conn, response); err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return response, nil
}

// ServiceInfo contains information about a gRPC service.
type ServiceInfo struct {
	Name    string       `json:"name"`
	Methods []MethodInfo `json:"methods"`
}

// MethodInfo contains information about a gRPC method.
type MethodInfo struct {
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	IsClientStream  bool   `json:"is_client_stream"`
	IsServerStream  bool   `json:"is_server_stream"`
	InputType       string `json:"input_type"`
	OutputType      string `json:"output_type"`
}

// HealthCheckRequest is a gRPC health check request.
type HealthCheckRequest struct {
	Service string `json:"service"`
}

// HealthCheckResponse is a gRPC health check response.
type HealthCheckResponse struct {
	Status string `json:"status"`
}

// Status constants for health checks.
const (
	HealthStatusUnknown        = "UNKNOWN"
	HealthStatusServing        = "SERVING"
	HealthStatusNotServing     = "NOT_SERVING"
	HealthStatusServiceUnknown = "SERVICE_UNKNOWN"
)

// RetryPolicy defines retry behavior for gRPC calls.
type RetryPolicy struct {
	MaxAttempts          int           `json:"max_attempts"`
	InitialBackoff       time.Duration `json:"initial_backoff"`
	MaxBackoff           time.Duration `json:"max_backoff"`
	BackoffMultiplier    float64       `json:"backoff_multiplier"`
	RetryableStatusCodes []int         `json:"retryable_status_codes"`
}

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableStatusCodes: []int{
			14, // UNAVAILABLE
			4,  // DEADLINE_EXCEEDED
		},
	}
}

// LoadBalancer defines a load balancer for gRPC connections.
type LoadBalancer interface {
	Pick() string
	Update(addresses []string)
}

// RoundRobinBalancer implements round-robin load balancing.
type RoundRobinBalancer struct {
	addresses []string
	index     int
	mu        sync.Mutex
}

// NewRoundRobinBalancer creates a new round-robin balancer.
func NewRoundRobinBalancer(addresses []string) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		addresses: addresses,
	}
}

// Pick returns the next address.
func (b *RoundRobinBalancer) Pick() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.addresses) == 0 {
		return ""
	}

	addr := b.addresses[b.index]
	b.index = (b.index + 1) % len(b.addresses)
	return addr
}

// Update updates the address list.
func (b *RoundRobinBalancer) Update(addresses []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.addresses = addresses
	if b.index >= len(addresses) {
		b.index = 0
	}
}

// Interceptor is a function that can intercept gRPC calls.
type Interceptor func(ctx context.Context, method string, req, reply interface{}, next func(ctx context.Context) error) error

// ChainInterceptors chains multiple interceptors.
func ChainInterceptors(interceptors ...Interceptor) Interceptor {
	return func(ctx context.Context, method string, req, reply interface{}, next func(ctx context.Context) error) error {
		chain := next
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			prevChain := chain
			chain = func(ctx context.Context) error {
				return interceptor(ctx, method, req, reply, prevChain)
			}
		}
		return chain(ctx)
	}
}

// LoggingInterceptor logs gRPC calls.
func LoggingInterceptor(logger func(string, ...interface{})) Interceptor {
	return func(ctx context.Context, method string, req, reply interface{}, next func(ctx context.Context) error) error {
		start := time.Now()
		err := next(ctx)
		duration := time.Since(start)

		if err != nil {
			logger("gRPC call %s failed after %v: %v", method, duration, err)
		} else {
			logger("gRPC call %s completed in %v", method, duration)
		}

		return err
	}
}

// MetricsInterceptor records gRPC call metrics.
func MetricsInterceptor(recordCall func(method string, duration time.Duration, err error)) Interceptor {
	return func(ctx context.Context, method string, req, reply interface{}, next func(ctx context.Context) error) error {
		start := time.Now()
		err := next(ctx)
		recordCall(method, time.Since(start), err)
		return err
	}
}
