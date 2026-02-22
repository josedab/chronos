# Release Notes: Chronos v0.1.0

**Chronos** — Distributed Cron System with Zero Dependencies

The first public release of Chronos, a distributed cron system built for reliability, security, and developer experience.

## Highlights

🚀 **Zero Dependencies** — Single binary with embedded BadgerDB storage. No etcd, no PostgreSQL, no Redis.

🔒 **Distributed Consensus** — Raft-based leader election with automatic failover (< 2 second recovery, E2E tested).

🌐 **Multi-Protocol Dispatch** — HTTP webhooks, gRPC, Kafka, NATS, RabbitMQ with HMAC-SHA256 signing and mutual TLS.

🔐 **Enterprise Auth** — OIDC/SSO with JWKS RS256 signature verification, 4 built-in roles, namespace isolation.

📊 **Full Observability** — Prometheus metrics, OpenTelemetry tracing (W3C traceparent propagation), failure heatmaps, SLO burn-rate dashboard.

🏗️ **Infrastructure-as-Code** — Terraform provider, Pulumi SDK, and Kubernetes Operator — all in one project.

## What's Included

### Core
- Job scheduling with cron expressions and timezone support
- Retry policies with exponential backoff
- Job dependencies (DAG) with fan-out/fan-in execution
- Concurrency policies (allow, forbid, replace)
- Job versioning with rollback and canary deployments

### Security
- HMAC-SHA256 webhook payload signing
- Mutual TLS (mTLS) for webhook dispatch
- Response assertions (JSON path matchers, body checks, timing SLAs)
- Canary/shadow execution with discrepancy detection

### Operations
- Migration wizard for crontab, K8s CronJobs, and Airflow DAGs
- Alert engine with Slack, PagerDuty, email, and webhook channels
- Cost attribution per job/namespace with budget alerts
- Per-namespace rate limiting (token bucket)
- Declarative job-as-code with `chronosctl apply -f`

### Ecosystem
- Web UI dashboard (React + TypeScript)
- CLI tool (`chronosctl`)
- Go SDK (`pkg/sdk`), Python SDK, TypeScript SDK
- Terraform provider (6 resources)
- Kubernetes Operator with CRDs
- GitHub Action for CI/CD job deployment
- Docker image and Helm chart
- 3 Grafana dashboard templates

### Quality
- 1,037 test functions + 23 benchmarks
- E2E tests including 3-node Raft cluster failover
- Chaos tests (burst load, webhook timeouts, concurrent R/W)
- Zero `go vet` violations
- 100% internal package test coverage (48/48)

## Quick Start

```bash
# Binary
curl -L https://github.com/chronos/chronos/releases/download/v0.1.0/chronos_0.1.0_linux_amd64.tar.gz | tar xz
./chronos --config chronos.yaml

# Docker
docker run -d -p 8080:8080 ghcr.io/chronos/chronos:0.1.0

# Helm
helm install chronos chronos/chronos

# Create your first job
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{"name":"hello","schedule":"* * * * *","webhook":{"url":"https://httpbin.org/post","method":"POST"},"enabled":true}'
```

## Documentation

- [Getting Started](https://chronos.github.io/chronos/docs/getting-started/quickstart)
- [Architecture](https://chronos.github.io/chronos/docs/core-concepts/architecture)
- [API Reference](https://chronos.github.io/chronos/docs/reference/api)
- [Terraform Provider](https://registry.terraform.io/providers/chronos/chronos)
- [Experimental Features](docs/EXPERIMENTAL.md)

## Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for the complete list of features.
