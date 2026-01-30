# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Managed Cloud Platform (`pkg/cloud`)
- Multi-tenant control plane with tenant isolation and management
- Subscription plans: Free, Starter, Pro, Enterprise with configurable quotas
- Cluster provisioning and lifecycle management (AWS, GCP, Azure)
- Usage metering and billing integration support
- API key management with scoped permissions

#### Visual Workflow Builder (`internal/dag/builder`)
- Canvas-based workflow builder API for visual job orchestration
- 12 node types: Trigger, Job, Condition, Parallel, Join, Delay, Approval, Notification, Script, HTTP, Transform, SubWorkflow
- Edge management with validation and cycle detection
- Topological sort for execution order determination
- Import/Export workflows as JSON
- Convert visual workflows to executable DAGs

#### Native gRPC & Message Queue Dispatch (`internal/dispatcher`)
- gRPC dispatcher with TLS support, connection pooling, and retry logic
- Apache Kafka producer with SASL authentication and partition routing
- NATS dispatcher with JetStream support and request-reply patterns
- RabbitMQ dispatcher with publisher confirms and exchange routing
- Multi-dispatcher manager for protocol-agnostic job execution
- Automatic protocol detection from job configuration

#### AI-Powered Schedule Optimization (`internal/smartsched`)
- Real-time anomaly detection for job duration and failure patterns
- Welford's online algorithm for streaming statistics (mean, variance)
- Trend analysis with linear regression for performance degradation
- Optimal execution window calculation based on historical success rates
- Execution forecasting for capacity planning
- Configurable sensitivity thresholds per job

#### Cross-Region Federation (`internal/geo`)
- Federation protocol for multi-cluster synchronization
- Conflict resolution strategies: newest-wins, owner-wins, merge
- Automatic health monitoring with configurable intervals
- Execution routing based on latency and availability
- Job replication across federated clusters
- Leader election per federation group

#### Execution Replay & Time-Travel Debugging (`internal/replay`)
- Debug session management with execution snapshots
- Step forward/backward through execution history
- Breakpoint types: step, node-type, condition-based, error
- Watch expressions for variable inspection
- Side-by-side execution comparison (diff mode)
- Session export for sharing and analysis

#### Job Marketplace & Template Hub (`internal/marketplace`)
- 15 production-ready job templates:
  - Slack notifications (channel, DM, webhook)
  - Database backups (PostgreSQL, MySQL, MongoDB)
  - Health checks (HTTP, TCP, DNS)
  - GitHub Actions workflow triggers
  - Datadog metric submission
  - PagerDuty heartbeats
  - S3 bucket cleanup
  - Docker image cleanup
  - SSL certificate monitoring
  - Elasticsearch index management
  - Redis cache warming
  - Report generation
- Template instantiation with variable substitution
- Template versioning and categorization

#### Policy-as-Code Engine (`internal/policy`)
- Declarative policy definition with YAML/JSON support
- 16 condition operators: equals, contains, regex, gt, lt, in, etc.
- Logical operators: AND, OR, NOT for complex conditions
- 7 built-in policies:
  - HTTPS-only webhooks
  - Job naming conventions
  - Timeout limits (min/max)
  - Retry attempt limits
  - Domain allowlists
  - Schedule frequency limits
  - Required labels
- Policy evaluation with detailed violation reporting
- Compiled conditions for high-performance evaluation

#### Real-Time Collaboration (`internal/realtime`)
- WebSocket hub for live updates
- Room-based architecture for job/workflow scoping
- Presence tracking (who's viewing what)
- Cursor and selection synchronization
- Live job execution broadcasts
- Typing indicators and edit notifications

#### Mobile Companion App Support (`internal/mobile`)
- Push notification service with provider abstraction
- Device registration and management
- Notification preferences per device:
  - Job completion/failure alerts
  - Approval requests
  - System alerts
  - Quiet hours configuration
- Mobile-optimized API response formats
- Badge count management

#### AI-Powered Job Assistant (`internal/assistant`)
- Natural language job creation via LLM integration
- Intelligent schedule suggestions based on job description
- Auto-generation of webhook configurations
- Job optimization recommendations

#### Predictive Autoscaling (`internal/autoscale`)
- Automatic cluster scaling based on job load patterns
- Predictive scaling using historical execution data
- Resource utilization monitoring and optimization
- Cost-aware scaling decisions

#### Chaos Engineering Framework (`internal/chaos`)
- Fault injection for resilience testing
- Network partition simulation
- Latency injection for dispatcher testing
- Automated chaos experiments with rollback

#### CloudEvents Support (`internal/cloudevents`)
- Native CloudEvents protocol support
- Event-driven job triggering
- CloudEvents emission for job lifecycle events
- Integration with cloud-native event meshes

#### Compliance & Audit Framework (`internal/compliance`)
- Comprehensive audit logging for all operations
- Compliance policy enforcement (SOC2, HIPAA, GDPR)
- Data retention policies
- Automated compliance reporting

#### Cost Optimization Engine (`internal/cost`)
- Multi-cloud cost tracking (AWS, GCP, Azure)
- Execution cost attribution per job
- Budget management and alerts
- Cost optimization recommendations

#### Event Mesh Integration (`internal/eventmesh`)
- Integration with enterprise event meshes
- Event routing and transformation
- Schema registry support
- Dead-letter queue handling

#### GitOps Synchronization (`internal/gitops`)
- Git-based job configuration management
- Automatic sync from Git repositories
- Pull request-based job changes
- Drift detection and remediation

#### Failure Prediction (`internal/prediction`)
- ML-based failure prediction
- Proactive alerting before failures occur
- Pattern recognition for job anomalies
- Recommended remediation actions

#### Sandbox Execution (`internal/sandbox`)
- Isolated container execution for jobs
- Resource limits and security constraints
- Custom runtime environments
- Execution artifact management

#### Semantic Search (`internal/search`)
- Natural language job search
- Semantic similarity matching
- Full-text search across job configurations
- Advanced filtering and faceting

#### Secret Management (`internal/secrets`)
- Vault integration for secret injection
- Multiple secret providers (AWS Secrets Manager, GCP Secret Manager, Azure Key Vault)
- Secret rotation support
- Encrypted secret storage

#### Webhook Testing Studio (`internal/studio`)
- Interactive webhook testing interface
- Request/response inspection
- Mock server for development
- Webhook replay and debugging

#### WebAssembly Plugin Runtime (`internal/wasm`)
- Custom job logic via WASM modules
- Sandboxed plugin execution
- Hot-reload of plugins
- Plugin marketplace integration

#### OpenTelemetry Tracing (`internal/tracing`)
- Distributed tracing for job executions
- Span propagation across services
- Integration with Jaeger, Zipkin, and cloud providers
- Trace-based debugging

#### Analytics & Insights (`internal/analytics`)
- Execution analytics and reporting
- Performance trend analysis
- Custom dashboards and visualizations
- Export to BI tools

- Initial release of Chronos distributed cron system
- Raft-based distributed consensus using hashicorp/raft
- Embedded BadgerDB storage (zero external dependencies)
- HTTP webhook job execution with configurable retry policies
- Cron expression parser with standard 5-field and extended 6-field support
- Special schedule descriptors (@hourly, @daily, @weekly, @monthly, @yearly, @every)
- REST API for job management
- Web UI (React + TypeScript) for job visualization and management
- CLI tool (chronosctl) for command-line job management
- Prometheus metrics endpoint at /metrics
- Kubernetes deployment support via Helm chart
- Docker image with multi-stage build
- Timezone support for job schedules
- Concurrency policies: allow, forbid, replace
- Configurable execution timeouts
- Webhook authentication support (Bearer token, Basic auth, custom headers)

### Security
- Non-root user in Docker image
- Configurable HTTP timeouts
- Request context cancellation support

## [0.1.0] - 2024-01-01

### Added
- Initial development release
- Core scheduling engine
- Basic API endpoints
- Single-node operation mode

[Unreleased]: https://github.com/chronos/chronos/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/chronos/chronos/releases/tag/v0.1.0
