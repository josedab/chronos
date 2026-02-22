# Chronos

**Distributed Cron System** - Reliable job scheduling without operational complexity.

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![CI](https://github.com/chronos/chronos/actions/workflows/ci.yml/badge.svg)](https://github.com/chronos/chronos/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/chronos/chronos)](https://github.com/chronos/chronos/releases)
[![Terraform Registry](https://img.shields.io/badge/terraform-registry-blueviolet)](https://registry.terraform.io/providers/chronos/chronos)
[![Go Report Card](https://goreportcard.com/badge/github.com/chronos/chronos)](https://goreportcard.com/report/github.com/chronos/chronos)

## Overview

Chronos is a distributed cron system that provides reliable job scheduling with:

- **Zero dependencies** - Single binary with embedded storage (BadgerDB)
- **Distributed consensus** - Raft-based leader election using hashicorp/raft
- **At-least-once execution** - Jobs run even during node failures
- **HTTP webhook dispatch** - Language-agnostic job execution
- **Built-in observability** - Prometheus metrics, OpenTelemetry tracing, Web UI
- **RBAC & SSO** - OIDC authentication, role-based access control, namespace isolation
- **DAG workflows** - Job dependencies with fan-out/fan-in execution
- **Infrastructure-as-Code** - Terraform provider, Pulumi SDK, Kubernetes Operator
- **Migration tooling** - Import from crontab, K8s CronJobs, and Airflow DAGs

> 📘 See [Experimental Features](docs/EXPERIMENTAL.md) for advanced capabilities under active development
> including cross-region federation, AI schedule optimization, and more.

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

See [CLI Reference](docs/cli.md) for the `chronosctl` command-line tool.

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

Looking for a place to start? Check out our [Good First Issues](docs/GOOD_FIRST_ISSUES.md).

## Community

- 💬 [GitHub Discussions](https://github.com/chronos/chronos/discussions) — Questions, ideas, show & tell
- 🐛 [Issue Tracker](https://github.com/chronos/chronos/issues) — Bug reports and feature requests
- 📋 [Roadmap](ROADMAP.md) — What's planned for future releases
- 📖 [Documentation](https://chronos.github.io/chronos) — Guides, API reference, SDK docs

## License

Apache 2.0 - see [LICENSE](LICENSE) for details.
