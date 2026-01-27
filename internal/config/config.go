// Package config provides configuration management for Chronos.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/pkg/duration"
	"gopkg.in/yaml.v3"
)

// Duration is an alias for the shared duration.Duration type.
type Duration = duration.Duration

// Config represents the complete Chronos configuration.
type Config struct {
	Cluster    ClusterConfig    `yaml:"cluster"`
	Server     ServerConfig     `yaml:"server"`
	Scheduler  SchedulerConfig  `yaml:"scheduler"`
	Dispatcher DispatcherConfig `yaml:"dispatcher"`
	Metrics    MetricsConfig    `yaml:"metrics"`
	Logging    LoggingConfig    `yaml:"logging"`
	Auth       AuthConfig       `yaml:"auth"`
}

// ClusterConfig contains cluster-related settings.
type ClusterConfig struct {
	NodeID  string     `yaml:"node_id"`
	DataDir string     `yaml:"data_dir"`
	Raft    RaftConfig `yaml:"raft"`
}

// RaftConfig contains Raft consensus settings.
type RaftConfig struct {
	Address          string   `yaml:"address"`
	Peers            []string `yaml:"peers"`
	HeartbeatTimeout Duration `yaml:"heartbeat_timeout"`
	ElectionTimeout  Duration `yaml:"election_timeout"`
	SnapshotInterval Duration `yaml:"snapshot_interval"`
	SnapshotThreshold uint64   `yaml:"snapshot_threshold"`
}

// ServerConfig contains HTTP/gRPC server settings.
type ServerConfig struct {
	HTTP HTTPConfig `yaml:"http"`
	GRPC GRPCConfig `yaml:"grpc"`
}

// HTTPConfig contains HTTP server settings.
type HTTPConfig struct {
	Address      string   `yaml:"address"`
	ReadTimeout  Duration `yaml:"read_timeout"`
	WriteTimeout Duration `yaml:"write_timeout"`
}

// GRPCConfig contains gRPC server settings.
type GRPCConfig struct {
	Address string `yaml:"address"`
}

// SchedulerConfig contains scheduler settings.
type SchedulerConfig struct {
	TickInterval       Duration             `yaml:"tick_interval"`
	ExecutionTimeout   Duration             `yaml:"execution_timeout"`
	DefaultRetryPolicy models.RetryPolicy   `yaml:"default_retry_policy"`
	MissedRunPolicy    MissedRunPolicy      `yaml:"missed_run_policy"`
}

// MissedRunPolicy defines how to handle missed runs.
type MissedRunPolicy string

const (
	MissedRunIgnore     MissedRunPolicy = "ignore"
	MissedRunExecuteOne MissedRunPolicy = "execute_one"
	MissedRunExecuteAll MissedRunPolicy = "execute_all"
)

// DispatcherConfig contains job dispatcher settings.
type DispatcherConfig struct {
	HTTP        HTTPClientConfig  `yaml:"http"`
	Concurrency ConcurrencyConfig `yaml:"concurrency"`
}

// HTTPClientConfig contains HTTP client settings.
type HTTPClientConfig struct {
	Timeout         Duration `yaml:"timeout"`
	MaxIdleConns    int      `yaml:"max_idle_conns"`
	IdleConnTimeout Duration `yaml:"idle_conn_timeout"`
}

// ConcurrencyConfig contains concurrency limits.
type ConcurrencyConfig struct {
	MaxConcurrentJobs int `yaml:"max_concurrent_jobs"`
	PerJobLimit       int `yaml:"per_job_limit"`
}

// MetricsConfig contains metrics settings.
type MetricsConfig struct {
	Prometheus PrometheusConfig `yaml:"prometheus"`
}

// PrometheusConfig contains Prometheus metrics settings.
type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// AuthConfig contains authentication settings.
type AuthConfig struct {
	Type  string            `yaml:"type"`
	Users map[string]string `yaml:"users"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Cluster: ClusterConfig{
			NodeID:  "chronos-1",
			DataDir: "./data",
			Raft: RaftConfig{
				Address:           "127.0.0.1:7000",
				HeartbeatTimeout:  Duration(500 * time.Millisecond),
				ElectionTimeout:   Duration(1 * time.Second),
				SnapshotInterval:  Duration(30 * time.Second),
				SnapshotThreshold: 1000,
			},
		},
		Server: ServerConfig{
			HTTP: HTTPConfig{
				Address:      "0.0.0.0:8080",
				ReadTimeout:  Duration(30 * time.Second),
				WriteTimeout: Duration(30 * time.Second),
			},
			GRPC: GRPCConfig{
				Address: "0.0.0.0:9000",
			},
		},
		Scheduler: SchedulerConfig{
			TickInterval:     Duration(1 * time.Second),
			ExecutionTimeout: Duration(5 * time.Minute),
			DefaultRetryPolicy: models.RetryPolicy{
				MaxAttempts:     3,
				InitialInterval: models.Duration(1 * time.Second),
				MaxInterval:     models.Duration(1 * time.Minute),
				Multiplier:      2.0,
			},
			MissedRunPolicy: MissedRunExecuteOne,
		},
		Dispatcher: DispatcherConfig{
			HTTP: HTTPClientConfig{
				Timeout:         Duration(30 * time.Second),
				MaxIdleConns:    100,
				IdleConnTimeout: Duration(90 * time.Second),
			},
			Concurrency: ConcurrencyConfig{
				MaxConcurrentJobs: 100,
				PerJobLimit:       1,
			},
		},
		Metrics: MetricsConfig{
			Prometheus: PrometheusConfig{
				Enabled: true,
				Path:    "/metrics",
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}

// Load loads configuration from a file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	data = []byte(os.ExpandEnv(string(data)))

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("CHRONOS_NODE_ID"); v != "" {
		c.Cluster.NodeID = v
	}
	if v := os.Getenv("CHRONOS_DATA_DIR"); v != "" {
		c.Cluster.DataDir = v
	}
	if v := os.Getenv("CHRONOS_RAFT_ADDRESS"); v != "" {
		c.Cluster.Raft.Address = v
	}
	if v := os.Getenv("CHRONOS_RAFT_PEERS"); v != "" {
		c.Cluster.Raft.Peers = strings.Split(v, ",")
	}
	if v := os.Getenv("CHRONOS_HTTP_ADDRESS"); v != "" {
		c.Server.HTTP.Address = v
	}
	if v := os.Getenv("CHRONOS_LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Cluster.NodeID == "" {
		return fmt.Errorf("cluster.node_id is required")
	}
	if c.Cluster.DataDir == "" {
		return fmt.Errorf("cluster.data_dir is required")
	}
	if c.Cluster.Raft.Address == "" {
		return fmt.Errorf("cluster.raft.address is required")
	}
	if c.Server.HTTP.Address == "" {
		return fmt.Errorf("server.http.address is required")
	}
	return nil
}
