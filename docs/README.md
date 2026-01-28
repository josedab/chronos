# Chronos Documentation

## Contents

- [API Reference](api.md) - Complete REST API documentation
- [Architecture](architecture.md) - System architecture with diagrams
- [Features API Reference](features-api.md) - Next-gen features API documentation
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
- [Architecture Decision Records](adr/) - Key design decisions
- [Features](#features) - Feature documentation

## Features

### Core Features
- **Cron Scheduling** - Standard 5-field and extended 6-field cron expressions
- **HTTP Webhook Dispatch** - Execute jobs via HTTP webhooks
- **Distributed Consensus** - Raft-based cluster for high availability
- **Retry Policies** - Exponential backoff with configurable limits
- **Concurrency Control** - Allow, forbid, or replace concurrent executions

### Next-Gen Features (v2.0+)

#### 🌐 Cross-Region Federation
Synchronize jobs across multiple clusters with conflict resolution:
- Multi-cluster synchronization
- Conflict resolution (newest, owner-wins, merge)
- Automatic failover and health monitoring
- [API Reference →](features-api.md#cross-region-federation)

#### 🔌 Multi-Protocol Dispatch
Execute jobs via multiple protocols beyond HTTP:
- gRPC with TLS and connection pooling
- Apache Kafka with SASL authentication
- NATS with JetStream support
- RabbitMQ with publisher confirms
- [API Reference →](features-api.md#multi-protocol-dispatch)

#### 🧠 AI Schedule Optimization
Intelligent scheduling recommendations:
- Anomaly detection for duration and failure patterns
- Optimal execution window calculation
- Trend analysis and forecasting
- [API Reference →](features-api.md#ai-schedule-optimization)

#### 🎨 Visual Workflow Builder
DAG-based job orchestration:
- 12 node types (trigger, job, condition, parallel, join, etc.)
- Visual canvas with drag-and-drop
- Cycle detection and validation
- Export/import workflows as JSON
- [API Reference →](features-api.md#visual-workflow-builder)

#### 📜 Policy-as-Code Engine
Declarative governance rules:
- 16 condition operators
- AND/OR/NOT logical expressions
- 7 built-in policies (HTTPS-only, naming conventions, etc.)
- Pre-execution policy enforcement
- [API Reference →](features-api.md#policy-as-code-engine)

#### ⏪ Time-Travel Debugging
Step through execution history:
- Step forward/backward through snapshots
- Breakpoints (step, type, condition, error)
- Watch expressions for variable inspection
- Execution comparison (diff mode)
- [API Reference →](features-api.md#time-travel-debugging)

#### 📦 Job Marketplace
15+ production-ready templates:
- Slack notifications
- Database backups (PostgreSQL, MySQL, MongoDB)
- Health checks (HTTP, TCP, DNS)
- GitHub Actions triggers
- Datadog metrics, PagerDuty heartbeats
- [API Reference →](features-api.md#job-marketplace)

#### 👥 Real-Time Collaboration
WebSocket-based live updates:
- Room-based architecture
- Presence tracking
- Cursor and selection sync
- Live execution broadcasts
- [API Reference →](features-api.md#real-time-collaboration)

#### 📱 Mobile Companion App
Push notifications and mobile APIs:
- Device registration
- Notification preferences with quiet hours
- Job alerts (completion, failure, approval)
- Mobile-optimized API responses
- [API Reference →](features-api.md#mobile-push-notifications)

#### ☁️ Cloud Control Plane
Multi-tenant managed platform:
- Subscription plans (Free, Starter, Pro, Enterprise)
- Cluster provisioning (AWS, GCP, Azure)
- Usage metering and billing
- Quota enforcement
- [API Reference →](features-api.md#cloud-control-plane)

#### Execution Replay
Replay failed or successful executions for debugging:
```
POST /api/v1/jobs/{id}/executions/{execId}/replay
```

#### Job Versioning
Track job configuration changes with version history:
```
GET /api/v1/jobs/{id}/versions
GET /api/v1/jobs/{id}/versions/{version}
POST /api/v1/jobs/{id}/rollback/{version}
```

#### OpenTelemetry Tracing
Distributed tracing with OpenTelemetry support:
- Enable via configuration: `tracing.enabled: true`
- OTLP exporter to any compatible backend
- Trace context propagation to webhooks

#### Secret Injection
Inject secrets from various providers:
- Environment variables: `${secret:env:MY_SECRET}`
- Files: `${secret:file:api-key}`
- Kubernetes: `${secret:kubernetes:secret-name/key}`

#### Plugin Architecture
Extensible plugin system for:
- Custom executors (gRPC, Lambda, Docker)
- Triggers (Kafka, SQS, webhooks)
- Notifiers (Slack, PagerDuty)
- Secret providers (Vault, AWS Secrets Manager)

#### Job Dependencies (DAG)
Define job dependencies for workflow orchestration:
```yaml
dependencies:
  depends_on:
    - job-a
    - job-b
  condition: all_success
```

#### Multi-Tenant Namespaces
Isolate jobs by namespace with resource quotas:
```yaml
namespace: production
```

## Quick Links

- [README](../README.md) - Project overview and quick start
- [Contributing Guide](../CONTRIBUTING.md) - How to contribute
- [Examples](../examples/) - Sample job configurations
- [Changelog](../CHANGELOG.md) - Release history

## External Resources

- [Cron Expression Reference](https://crontab.guru/) - Interactive cron editor
- [Raft Consensus](https://raft.github.io/) - Understanding distributed consensus
- [BadgerDB](https://dgraph.io/docs/badger/) - Storage engine documentation
- [OpenTelemetry](https://opentelemetry.io/) - Distributed tracing
