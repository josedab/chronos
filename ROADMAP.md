# Chronos Roadmap

This document outlines the development roadmap for Chronos. Items are grouped by priority and may shift based on community feedback.

## ✅ Released in v0.1.0

- Distributed job scheduling (Raft consensus)
- Multi-protocol dispatch (HTTP, gRPC, Kafka, NATS, RabbitMQ)
- HMAC webhook signing + mTLS
- Response assertions (JSON path, body, timing)
- Canary/shadow execution
- DAG workflows with fan-out/fan-in
- RBAC + OIDC/SSO with JWKS verification
- OpenTelemetry tracing with traceparent propagation
- Terraform provider + Pulumi SDK + K8s Operator
- Declarative YAML job-as-code (`chronosctl apply -f`)
- Migration wizard (crontab, K8s, Airflow)
- Web UI with failure heatmaps, SLO burn-rate, one-click retry
- Plugin SDK (5 extension points)
- Python + TypeScript SDKs
- Alert engine with cooldown and filtering
- Cost attribution with budget alerts
- Rate limiting (per-namespace token bucket)

## 🚧 In Progress (v0.2.0)

- [ ] **Helm chart publication** — Publish to Artifact Hub
- [ ] **Grafana marketplace** — Submit dashboards to grafana.com
- [ ] **GitHub Actions marketplace** — Publish deploy-jobs action
- [ ] **Documentation site** — Docusaurus with Getting Started, SDK guides
- [ ] **Chaos testing in CI** — Network partition, node crash, split-brain tests

## 📋 Planned (v0.3.0)

- [ ] **Managed cloud MVP** — Hosted free tier at chronos.dev
- [ ] **Complete Helm chart** — Multi-node StatefulSet with auto-discovery
- [ ] **Alerting UI** — Web UI for alert channel and rule management
- [ ] **Federation UI** — Dashboard for multi-cluster management
- [ ] **Webhook testing studio UI** — Visual request builder in dashboard

## 🔮 Future

- [ ] **Execution log streaming** — Real-time WebSocket log viewer
- [ ] **AI schedule advisor UI** — Recommendations surfaced in job detail
- [ ] **CLI TUI dashboard** — `chronosctl dashboard` with Bubble Tea
- [ ] **Global event mesh** — Cross-service event routing
- [ ] **ML-powered autoscaling** — Predictive cluster scaling

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

Have a feature idea? [Open a discussion](https://github.com/chronos/chronos/discussions/new?category=ideas).
