# Chronos

**Distributed Cron System** - Reliable job scheduling without operational complexity.

[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![CI](https://github.com/chronos/chronos/actions/workflows/ci.yml/badge.svg)](https://github.com/chronos/chronos/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/chronos/chronos/branch/main/graph/badge.svg)](https://codecov.io/gh/chronos/chronos)
[![Go Report Card](https://goreportcard.com/badge/github.com/chronos/chronos)](https://goreportcard.com/report/github.com/chronos/chronos)

## Overview

Chronos is a distributed cron system that provides reliable job scheduling with:

- **Zero dependencies** - Single binary with embedded storage (BadgerDB)
- **Distributed consensus** - Raft-based leader election using hashicorp/raft
- **At-least-once execution** - Jobs run even during node failures
- **HTTP webhook dispatch** - Language-agnostic job execution
- **Built-in observability** - Prometheus metrics, structured logging, Web UI

### Next-Gen Features (Experimental)

> ⚠️ **Note**: The features listed below are experimental and under active development.
> They are scaffolded but not yet integrated into the main application. Use at your own risk
> in production environments. APIs may change without notice.

- **🌐 Cross-Region Federation** - Multi-cluster synchronization with conflict resolution
- **🔌 Multi-Protocol Dispatch** - gRPC, Kafka, NATS, RabbitMQ in addition to HTTP
- **🧠 AI Schedule Optimization** - Anomaly detection and optimal timing recommendations
- **🎨 Visual Workflow Builder** - DAG-based job orchestration with 12 node types
- **📜 Policy-as-Code** - Declarative governance rules with 16 operators
- **⏪ Time-Travel Debugging** - Step through execution history with breakpoints
- **📦 Job Marketplace** - 15+ production-ready templates
- **👥 Real-Time Collaboration** - Live cursors, presence, and edit sync
- **📱 Mobile Support** - Push notifications and mobile-optimized APIs
- **☁️ Cloud Platform** - Multi-tenant control plane with billing
- **🤖 AI Assistant** - Natural language job creation and optimization
- **📈 Predictive Autoscaling** - Automatic scaling based on job patterns
- **🔐 Secret Management** - Vault and cloud provider secret injection
- **🔍 Semantic Search** - Natural language job discovery
- **🧪 Chaos Engineering** - Built-in fault injection testing
- **📊 OpenTelemetry** - Distributed tracing integration
- **🔄 GitOps** - Git-based job configuration management
- **🧩 WASM Plugins** - Custom job logic via WebAssembly

## Quick Start

### Binary Installation

```bash
# Download the latest release
curl -L https://github.com/chronos/chronos/releases/latest/download/chronos-linux-amd64.tar.gz | tar xz
./chronos --config chronos.yaml
```

### Docker

```bash
docker run -d -p 8080:8080 -v chronos-data:/var/lib/chronos chronos/chronos:latest
```

### Kubernetes (Helm)

```bash
helm repo add chronos https://chronos.github.io/charts
helm install chronos chronos/chronos
```

## Usage

### Create a Job

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "daily-report",
    "schedule": "0 9 * * *",
    "timezone": "America/New_York",
    "webhook": {
      "url": "https://api.example.com/reports/generate",
      "method": "POST",
      "headers": {
        "Authorization": "Bearer token123"
      }
    },
    "retry_policy": {
      "max_attempts": 3,
      "initial_interval": "1s",
      "multiplier": 2.0
    },
    "timeout": "5m",
    "enabled": true
  }'
```

### Using the CLI

```bash
# List all jobs
chronosctl job list

# Get job details
chronosctl job get <job-id>

# Create a job from YAML
chronosctl job create -f job.yaml

# Trigger a job manually
chronosctl job trigger <job-id>

# View executions
chronosctl execution list <job-id>
```

## Configuration

```yaml
# chronos.yaml
cluster:
  node_id: chronos-1
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.0.1:7000
    peers:
      - chronos-2:7000
      - chronos-3:7000

server:
  http:
    address: 0.0.0.0:8080
    read_timeout: 30s
    write_timeout: 30s

scheduler:
  tick_interval: 1s
  execution_timeout: 5m
  default_retry_policy:
    max_attempts: 3
    initial_interval: 1s
    max_interval: 1m
    multiplier: 2.0

dispatcher:
  http:
    timeout: 30s
    max_idle_conns: 100

metrics:
  prometheus:
    enabled: true
    path: /metrics

logging:
  level: info
  format: json
```

## Cron Expression Reference

| Expression | Description |
|------------|-------------|
| `* * * * *` | Every minute |
| `0 * * * *` | Every hour |
| `0 0 * * *` | Every day at midnight |
| `0 0 * * 0` | Every Sunday at midnight |
| `*/5 * * * *` | Every 5 minutes |
| `0 9-17 * * 1-5` | Every hour 9am-5pm, Monday-Friday |
| `@hourly` | Every hour (0 * * * *) |
| `@daily` | Every day at midnight |
| `@every 30m` | Every 30 minutes |

## Architecture

```
┌──────────────────────────────────────────────────┐
│                  CHRONOS CLUSTER                  │
│                                                   │
│   ┌─────────────┐ ┌─────────────┐ ┌────────────┐ │
│   │   Node 1    │ │   Node 2    │ │   Node 3   │ │
│   │  (Leader)   │ │ (Follower)  │ │ (Follower) │ │
│   │             │ │             │ │            │ │
│   │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌────────┐ │ │
│   │ │Scheduler│ │ │ │Scheduler│ │ │ │Scheduler│ │ │
│   │ │ (active)│ │ │ │(standby)│ │ │ │(standby)│ │ │
│   │ └─────────┘ │ │ └─────────┘ │ │ └────────┘ │ │
│   │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌────────┐ │ │
│   │ │  Raft   │◄┼─┼─│  Raft   │◄┼─┼─│  Raft  │ │ │
│   │ └─────────┘ │ │ └─────────┘ │ │ └────────┘ │ │
│   │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌────────┐ │ │
│   │ │BadgerDB │ │ │ │BadgerDB │ │ │ │BadgerDB│ │ │
│   │ └─────────┘ │ │ └─────────┘ │ │ └────────┘ │ │
│   └─────────────┘ └─────────────┘ └────────────┘ │
└──────────────────────────────────────────────────┘
                        │
                        ▼
              ┌─────────────────┐
              │ Target Services │
              │  (HTTP Webhooks)│
              └─────────────────┘
```

## API Reference

See [API Documentation](docs/api.md) for the complete API reference.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/api/v1/jobs` | List all jobs |
| POST | `/api/v1/jobs` | Create a job |
| GET | `/api/v1/jobs/{id}` | Get job details |
| PUT | `/api/v1/jobs/{id}` | Update a job |
| DELETE | `/api/v1/jobs/{id}` | Delete a job |
| POST | `/api/v1/jobs/{id}/trigger` | Trigger job execution |
| POST | `/api/v1/jobs/{id}/enable` | Enable a job |
| POST | `/api/v1/jobs/{id}/disable` | Disable a job |
| GET | `/api/v1/jobs/{id}/executions` | List job executions |
| GET | `/api/v1/cluster/status` | Get cluster status |
| GET | `/metrics` | Prometheus metrics |

## Metrics

Chronos exposes Prometheus metrics at `/metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| `chronos_jobs_total` | Gauge | Total number of jobs |
| `chronos_jobs_enabled` | Gauge | Number of enabled jobs |
| `chronos_executions_total` | Counter | Total executions by status |
| `chronos_execution_duration_seconds` | Histogram | Execution duration |
| `chronos_raft_is_leader` | Gauge | Leader status (0/1) |
| `chronos_raft_peers` | Gauge | Number of cluster peers |

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details.

## License

Apache 2.0 - see [LICENSE](LICENSE) for details.
