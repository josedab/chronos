# Experimental Features

> ⚠️ **Warning**: The features listed below are experimental and under active development.
> APIs may change without notice. Use at your own risk in production environments.

Chronos includes an extensive set of advanced capabilities beyond the stable core.
These features are functional but not yet fully hardened for production use.

## Available Experimental Features

| Feature | Package | Status | Description |
|---------|---------|--------|-------------|
| 🌐 Cross-Region Federation | `internal/geo` | Beta | Multi-cluster synchronization with conflict resolution and latency-aware routing |
| 🔌 Multi-Protocol Dispatch | `internal/dispatcher` | Beta | gRPC, Kafka, NATS, RabbitMQ in addition to HTTP webhooks |
| 🧠 AI Schedule Optimization | `internal/smartsched` | Alpha | Anomaly detection and optimal timing recommendations |
| 🎨 Visual Workflow Builder | `internal/dag` | Beta | DAG-based job orchestration with 12 node types |
| 📜 Policy-as-Code | `internal/policy` | Beta | Declarative governance rules with 16 operators |
| ⏪ Time-Travel Debugging | `internal/replay` | Alpha | Step through execution history with breakpoints |
| 📦 Job Marketplace | `internal/marketplace` | Beta | 15+ production-ready job templates |
| 👥 Real-Time Collaboration | `internal/realtime` | Alpha | Live cursors, presence, and edit sync via WebSocket |
| 📱 Mobile Support | `internal/mobile` | Alpha | Push notifications and mobile-optimized APIs |
| ☁️ Cloud Platform | `internal/cloud` | Alpha | Multi-tenant control plane with billing and onboarding |
| 🤖 AI Assistant | `internal/assistant` | Alpha | Natural language job creation |
| 📈 Predictive Autoscaling | `internal/autoscale` | Beta | Automatic scaling based on job execution patterns |
| 🔐 Secret Management | `internal/secrets` | Beta | Vault, AWS, GCP, Azure secret injection (4 providers) |
| 🧪 Chaos Engineering | `internal/chaos` | Alpha | Built-in fault injection and GameDay experiments |
| 📊 OpenTelemetry Tracing | `internal/tracing` | Beta | Distributed tracing with W3C traceparent propagation |
| 🔄 GitOps | `internal/gitops` | Alpha | Git-based job configuration with `chronosctl apply -f` |
| 🧩 WASM Plugins | `internal/wasm` | Alpha | Custom job logic via WebAssembly modules |
| 🛡️ Webhook Signing | `internal/dispatcher` | Beta | HMAC-SHA256 payload signing and mTLS |
| 🎯 Response Assertions | `internal/dispatcher` | Beta | JSON path matchers, body checks, timing SLAs |
| 🐤 Canary Execution | `internal/dispatcher` | Alpha | Shadow webhook mode with response comparison |
| 💰 Cost Attribution | `internal/costattribution` | Alpha | Per-execution cost tracking with budgets |
| ⚡ Rate Limiting | `internal/scheduler` | Beta | Per-namespace token bucket rate limiting |

## Status Definitions

- **Beta**: Feature is functional with tests. API may have minor changes.
- **Alpha**: Feature is implemented but not yet integration-tested. API will change.

## Enabling Experimental Features

Most experimental features are available through the API and configuration.
Refer to each package's documentation for enablement instructions.

## Feedback

We welcome feedback on experimental features via [GitHub Issues](https://github.com/chronos/chronos/issues).
Please tag issues with the `experimental` label.
