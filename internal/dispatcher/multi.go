// Package dispatcher handles job execution via various protocols.
package dispatcher

import (
	"context"
	"errors"
	"time"

	"github.com/chronos/chronos/internal/models"
)

// DispatcherType represents the type of dispatcher.
type DispatcherType string

const (
	DispatcherTypeHTTP     DispatcherType = "http"
	DispatcherTypeGRPC     DispatcherType = "grpc"
	DispatcherTypeKafka    DispatcherType = "kafka"
	DispatcherTypeNATS     DispatcherType = "nats"
	DispatcherTypeRabbitMQ DispatcherType = "rabbitmq"
)

// Common errors.
var (
	ErrUnsupportedDispatcher = errors.New("unsupported dispatcher type")
	ErrDispatcherNotFound    = errors.New("dispatcher not found")
)

// MultiDispatcher manages multiple dispatcher types and routes jobs accordingly.
type MultiDispatcher struct {
	http     *Dispatcher
	grpc     *GRPCDispatcher
	kafka    *KafkaDispatcher
	nats     *NATSDispatcher
	rabbitmq *RabbitMQDispatcher
}

// MultiDispatcherConfig holds configuration for all dispatcher types.
type MultiDispatcherConfig struct {
	NodeID string

	// HTTP configuration
	HTTP *Config

	// gRPC configuration
	GRPC *GRPCDispatcherConfig

	// Kafka configuration
	Kafka *KafkaDispatcherConfig

	// NATS configuration
	NATS *NATSDispatcherConfig

	// RabbitMQ configuration
	RabbitMQ *RabbitMQDispatcherConfig
}

// DefaultMultiDispatcherConfig returns the default multi-dispatcher configuration.
func DefaultMultiDispatcherConfig(nodeID string) *MultiDispatcherConfig {
	return &MultiDispatcherConfig{
		NodeID: nodeID,
		HTTP: &Config{
			NodeID:          nodeID,
			Timeout:         30 * time.Second,
			MaxIdleConns:    100,
			IdleConnTimeout: 90 * time.Second,
			MaxConcurrent:   100,
		},
		GRPC: &GRPCDispatcherConfig{
			NodeID:            nodeID,
			DefaultTimeout:    30 * time.Second,
			MaxConcurrent:     100,
			MaxConnectionAge:  5 * time.Minute,
			ConnectionTimeout: 10 * time.Second,
		},
		Kafka: &KafkaDispatcherConfig{
			NodeID:           nodeID,
			DefaultTimeout:   30 * time.Second,
			MaxConcurrent:    100,
			ProducerPoolSize: 10,
		},
		NATS: &NATSDispatcherConfig{
			NodeID:             nodeID,
			DefaultTimeout:     30 * time.Second,
			MaxConcurrent:      100,
			ReconnectWait:      2 * time.Second,
			MaxReconnects:      60,
			PingInterval:       2 * time.Minute,
			MaxPingsOut:        2,
			ConnectionPoolSize: 10,
		},
		RabbitMQ: &RabbitMQDispatcherConfig{
			NodeID:             nodeID,
			DefaultTimeout:     30 * time.Second,
			MaxConcurrent:      100,
			HeartbeatInterval:  10 * time.Second,
			ConnectionPoolSize: 10,
			ChannelMax:         2047,
		},
	}
}

// NewMultiDispatcher creates a new multi-dispatcher with all dispatch types.
func NewMultiDispatcher(cfg *MultiDispatcherConfig) *MultiDispatcher {
	if cfg == nil {
		cfg = DefaultMultiDispatcherConfig("chronos-1")
	}

	return &MultiDispatcher{
		http:     New(cfg.HTTP),
		grpc:     NewGRPCDispatcher(cfg.GRPC),
		kafka:    NewKafkaDispatcher(cfg.Kafka),
		nats:     NewNATSDispatcher(cfg.NATS),
		rabbitmq: NewRabbitMQDispatcher(cfg.RabbitMQ),
	}
}

// ExecuteJob executes a job using the appropriate dispatcher based on job configuration.
func (m *MultiDispatcher) ExecuteJob(ctx context.Context, job *models.Job, dispatchConfig *DispatchConfig, scheduledTime time.Time) (*models.Execution, error) {
	if dispatchConfig == nil {
		// Default to HTTP webhook
		if job.Webhook != nil {
			return m.http.Execute(ctx, job, scheduledTime)
		}
		return nil, ErrUnsupportedDispatcher
	}

	switch dispatchConfig.Type {
	case DispatcherTypeHTTP:
		return m.http.Execute(ctx, job, scheduledTime)

	case DispatcherTypeGRPC:
		if dispatchConfig.GRPC == nil {
			return nil, ErrInvalidGRPCConfig
		}
		return m.grpc.Execute(ctx, job, dispatchConfig.GRPC, scheduledTime)

	case DispatcherTypeKafka:
		if dispatchConfig.Kafka == nil {
			return nil, ErrInvalidKafkaConfig
		}
		return m.kafka.Execute(ctx, job, dispatchConfig.Kafka, scheduledTime)

	case DispatcherTypeNATS:
		if dispatchConfig.NATS == nil {
			return nil, ErrInvalidNATSConfig
		}
		return m.nats.Execute(ctx, job, dispatchConfig.NATS, scheduledTime)

	case DispatcherTypeRabbitMQ:
		if dispatchConfig.RabbitMQ == nil {
			return nil, ErrInvalidRabbitMQConfig
		}
		return m.rabbitmq.Execute(ctx, job, dispatchConfig.RabbitMQ, scheduledTime)

	default:
		return nil, ErrUnsupportedDispatcher
	}
}

// DispatchConfig holds the dispatch configuration for a job.
type DispatchConfig struct {
	Type     DispatcherType   `json:"type" yaml:"type"`
	GRPC     *GRPCConfig      `json:"grpc,omitempty" yaml:"grpc,omitempty"`
	Kafka    *KafkaConfig     `json:"kafka,omitempty" yaml:"kafka,omitempty"`
	NATS     *NATSConfig      `json:"nats,omitempty" yaml:"nats,omitempty"`
	RabbitMQ *RabbitMQConfig  `json:"rabbitmq,omitempty" yaml:"rabbitmq,omitempty"`
}

// GetHTTPDispatcher returns the HTTP dispatcher.
func (m *MultiDispatcher) GetHTTPDispatcher() *Dispatcher {
	return m.http
}

// GetGRPCDispatcher returns the gRPC dispatcher.
func (m *MultiDispatcher) GetGRPCDispatcher() *GRPCDispatcher {
	return m.grpc
}

// GetKafkaDispatcher returns the Kafka dispatcher.
func (m *MultiDispatcher) GetKafkaDispatcher() *KafkaDispatcher {
	return m.kafka
}

// GetNATSDispatcher returns the NATS dispatcher.
func (m *MultiDispatcher) GetNATSDispatcher() *NATSDispatcher {
	return m.nats
}

// GetRabbitMQDispatcher returns the RabbitMQ dispatcher.
func (m *MultiDispatcher) GetRabbitMQDispatcher() *RabbitMQDispatcher {
	return m.rabbitmq
}

// GetMetrics returns metrics from all dispatchers.
func (m *MultiDispatcher) GetMetrics() *MultiDispatcherMetrics {
	return &MultiDispatcherMetrics{
		HTTP:     m.http.GetMetrics(),
		GRPC:     m.grpc.GetMetrics(),
		Kafka:    m.kafka.GetMetrics(),
		NATS:     m.nats.GetMetrics(),
		RabbitMQ: m.rabbitmq.GetMetrics(),
	}
}

// MultiDispatcherMetrics holds metrics from all dispatchers.
type MultiDispatcherMetrics struct {
	HTTP     Metrics
	GRPC     GRPCMetrics
	Kafka    KafkaMetrics
	NATS     NATSMetrics
	RabbitMQ RabbitMQMetrics
}

// Close closes all dispatchers and releases resources.
func (m *MultiDispatcher) Close() error {
	var errs []error

	if err := m.grpc.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := m.kafka.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := m.nats.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := m.rabbitmq.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// ValidateDispatchConfig validates dispatch configuration.
func ValidateDispatchConfig(cfg *DispatchConfig) error {
	if cfg == nil {
		return nil // nil config defaults to HTTP
	}

	switch cfg.Type {
	case DispatcherTypeHTTP, "":
		return nil // HTTP validated elsewhere

	case DispatcherTypeGRPC:
		if cfg.GRPC == nil {
			return ErrInvalidGRPCConfig
		}
		if cfg.GRPC.Target == "" || cfg.GRPC.Method == "" {
			return ErrInvalidGRPCConfig
		}
		return nil

	case DispatcherTypeKafka:
		return ValidateKafkaConfig(cfg.Kafka)

	case DispatcherTypeNATS:
		return ValidateNATSConfig(cfg.NATS)

	case DispatcherTypeRabbitMQ:
		return ValidateRabbitMQConfig(cfg.RabbitMQ)

	default:
		return ErrUnsupportedDispatcher
	}
}
