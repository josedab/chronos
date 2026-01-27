package config

import (
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Cluster.NodeID != "chronos-1" {
		t.Errorf("expected default node_id 'chronos-1', got %q", cfg.Cluster.NodeID)
	}
	if cfg.Cluster.DataDir != "./data" {
		t.Errorf("expected default data_dir './data', got %q", cfg.Cluster.DataDir)
	}
	if cfg.Server.HTTP.Address != "0.0.0.0:8080" {
		t.Errorf("expected default HTTP address '0.0.0.0:8080', got %q", cfg.Server.HTTP.Address)
	}
	if cfg.Scheduler.TickInterval.Duration() != time.Second {
		t.Errorf("expected default tick interval 1s, got %v", cfg.Scheduler.TickInterval.Duration())
	}
	if cfg.Metrics.Prometheus.Enabled != true {
		t.Error("expected Prometheus metrics to be enabled by default")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level 'info', got %q", cfg.Logging.Level)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "missing node_id",
			modify: func(c *Config) {
				c.Cluster.NodeID = ""
			},
			wantErr: true,
		},
		{
			name: "missing data_dir",
			modify: func(c *Config) {
				c.Cluster.DataDir = ""
			},
			wantErr: true,
		},
		{
			name: "missing raft address",
			modify: func(c *Config) {
				c.Cluster.Raft.Address = ""
			},
			wantErr: true,
		},
		{
			name: "missing http address",
			modify: func(c *Config) {
				c.Server.HTTP.Address = ""
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_Load(t *testing.T) {
	// Create temp config file
	content := `
cluster:
  node_id: test-node
  data_dir: /tmp/chronos
  raft:
    address: 127.0.0.1:7000

server:
  http:
    address: 0.0.0.0:9080
    read_timeout: 60s
    write_timeout: 60s

scheduler:
  tick_interval: 2s
  execution_timeout: 10m

logging:
  level: debug
  format: text
`
	tmpFile, err := os.CreateTemp("", "chronos-test-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Cluster.NodeID != "test-node" {
		t.Errorf("expected node_id 'test-node', got %q", cfg.Cluster.NodeID)
	}
	if cfg.Cluster.DataDir != "/tmp/chronos" {
		t.Errorf("expected data_dir '/tmp/chronos', got %q", cfg.Cluster.DataDir)
	}
	if cfg.Server.HTTP.Address != "0.0.0.0:9080" {
		t.Errorf("expected HTTP address '0.0.0.0:9080', got %q", cfg.Server.HTTP.Address)
	}
	if cfg.Server.HTTP.ReadTimeout.Duration() != 60*time.Second {
		t.Errorf("expected read timeout 60s, got %v", cfg.Server.HTTP.ReadTimeout.Duration())
	}
	if cfg.Scheduler.TickInterval.Duration() != 2*time.Second {
		t.Errorf("expected tick interval 2s, got %v", cfg.Scheduler.TickInterval.Duration())
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got %q", cfg.Logging.Level)
	}
}

func TestConfig_Load_InvalidFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestConfig_Load_InvalidYAML(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "chronos-test-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write invalid YAML
	if _, err := tmpFile.WriteString("invalid: yaml: content:"); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	_, err = Load(tmpFile.Name())
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestConfig_EnvOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("CHRONOS_NODE_ID", "env-node")
	os.Setenv("CHRONOS_DATA_DIR", "/env/data")
	os.Setenv("CHRONOS_LOG_LEVEL", "warn")
	defer func() {
		os.Unsetenv("CHRONOS_NODE_ID")
		os.Unsetenv("CHRONOS_DATA_DIR")
		os.Unsetenv("CHRONOS_LOG_LEVEL")
	}()

	// Create minimal config file
	content := `
cluster:
  node_id: file-node
  data_dir: /file/data
  raft:
    address: 127.0.0.1:7000

server:
  http:
    address: 0.0.0.0:8080

logging:
  level: info
`
	tmpFile, err := os.CreateTemp("", "chronos-test-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Environment variables should override file values
	if cfg.Cluster.NodeID != "env-node" {
		t.Errorf("expected node_id 'env-node' from env, got %q", cfg.Cluster.NodeID)
	}
	if cfg.Cluster.DataDir != "/env/data" {
		t.Errorf("expected data_dir '/env/data' from env, got %q", cfg.Cluster.DataDir)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("expected log level 'warn' from env, got %q", cfg.Logging.Level)
	}
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"1s", time.Second},
		{"5m", 5 * time.Minute},
		{"1h30m", 90 * time.Minute},
		{"100ms", 100 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			content := "timeout: " + tt.input

			tmpFile, err := os.CreateTemp("", "chronos-test-*.yaml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(content); err != nil {
				t.Fatalf("failed to write: %v", err)
			}
			tmpFile.Close()

			// Test with a simple struct
			type testStruct struct {
				Timeout Duration `yaml:"timeout"`
			}

			data, _ := os.ReadFile(tmpFile.Name())
			var ts testStruct
			if err := yaml.Unmarshal(data, &ts); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if ts.Timeout.Duration() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, ts.Timeout.Duration())
			}
		})
	}
}
