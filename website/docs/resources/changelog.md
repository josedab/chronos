---
sidebar_position: 4
title: Changelog
description: Release notes and version history
---

# Changelog

All notable changes to Chronos are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and we adhere to [Semantic Versioning](https://semver.org/).

For the complete changelog, see [CHANGELOG.md on GitHub](https://github.com/chronos/chronos/blob/main/CHANGELOG.md).

---

## [1.0.0] - 2026-01-29

🎉 **First stable release of Chronos!**

### Highlights

- **Production-ready** distributed cron system
- **Zero dependencies** - single binary with embedded storage
- **High availability** via Raft consensus
- **At-least-once execution** guarantees

### Core Features

#### Distributed Consensus
- Raft-based leader election using HashiCorp Raft
- Automatic failover in ~5 seconds
- Log replication across all nodes
- Periodic snapshots for fast recovery

#### Job Scheduling
- Standard 5-field cron expressions
- Extended 6-field cron (with seconds)
- Special descriptors: `@hourly`, `@daily`, `@weekly`, `@monthly`, `@yearly`
- Interval syntax: `@every 5m`, `@every 1h30m`
- IANA timezone support
- Concurrency policies: `allow`, `forbid`, `replace`

#### Webhook Dispatch
- HTTP/HTTPS webhook execution
- Configurable retry policies with exponential backoff
- Custom headers and request bodies
- Multiple authentication methods (Bearer, Basic, API Key)
- Timeout configuration per job
- Circuit breaker for target protection

#### Storage
- Embedded BadgerDB (no external database required)
- Execution history with configurable retention
- Job versioning and rollback support

#### Observability
- Prometheus metrics at `/metrics`
- Structured JSON logging
- Health and readiness endpoints
- Execution duration histograms
- Cluster status API

#### Web UI
- React + TypeScript dashboard
- Job management (create, edit, delete)
- Execution history visualization
- Cluster health overview
- Real-time updates

#### CLI Tool (`chronosctl`)
- Job CRUD operations
- Execution history queries
- Cluster management
- YAML-based job definitions
- Output formats: table, JSON, YAML

#### API
- RESTful API with OpenAPI 3.1 specification
- Pagination support
- Tag-based filtering
- Consistent error responses

### Deployment

- **Binary**: Linux, macOS, Windows (amd64, arm64)
- **Docker**: Multi-arch images on Docker Hub
- **Kubernetes**: Helm chart with StatefulSet
- **Terraform**: Provider for infrastructure-as-code
- **Pulumi**: SDK for programmatic deployment

### Security

- Optional authentication (API keys, JWT, Basic)
- TLS support for API and Raft communication
- Non-root container execution
- Secret masking in logs and API responses

---

## [0.1.0] - 2026-01-01

🚧 **Initial development release**

### Added
- Core scheduling engine
- Basic REST API
- Single-node operation mode
- Proof-of-concept implementation

---

## Upgrade Guide

### From 0.x to 1.0

1. **Backup your data**
   ```bash
   tar -czf chronos-backup.tar.gz /var/lib/chronos/
   ```

2. **Stop all nodes**
   ```bash
   systemctl stop chronos
   ```

3. **Replace binaries**
   ```bash
   curl -L https://github.com/chronos/chronos/releases/download/v1.0.0/chronos-linux-amd64.tar.gz | tar xz
   ```

4. **Update configuration** (if needed)
   ```yaml
   # New configuration options in 1.0
   scheduler:
     missed_run_policy: execute_one  # New option
   ```

5. **Start nodes** (one at a time for rolling upgrade)
   ```bash
   systemctl start chronos
   ```

---

## Versioning Policy

Chronos follows [Semantic Versioning](https://semver.org/):

- **MAJOR** (1.0.0 → 2.0.0): Breaking changes to API or configuration
- **MINOR** (1.0.0 → 1.1.0): New features, backward compatible
- **PATCH** (1.0.0 → 1.0.1): Bug fixes, backward compatible

### Deprecation Policy

- Deprecated features are announced in release notes
- Deprecated features work for at least 2 minor versions
- Removal happens in the next major version

### Support Policy

| Version | Status | Support Until |
|---------|--------|---------------|
| 1.x | Active | Current |
| 0.x | EOL | 2026-03-01 |

---

## Release Schedule

We aim to release:
- **Patch releases**: As needed for bug fixes
- **Minor releases**: Monthly with new features
- **Major releases**: Annually (if breaking changes needed)

Subscribe to [GitHub releases](https://github.com/chronos/chronos/releases) for notifications.

---

## Experimental Features

The following features are in active development and available behind feature flags:

| Feature | Flag | Status |
|---------|------|--------|
| Cross-region federation | `--enable-federation` | Alpha |
| Visual workflow builder | `--enable-workflows` | Alpha |
| AI schedule optimization | `--enable-ai` | Alpha |
| WASM plugins | `--enable-wasm` | Alpha |
| Policy-as-code | `--enable-policies` | Beta |
| Secret management | `--enable-secrets` | Beta |

:::warning
Experimental features may change or be removed without notice. Do not use in production without understanding the risks.
:::

See [Advanced Features](/docs/advanced/policy-as-code) for documentation on experimental capabilities.
