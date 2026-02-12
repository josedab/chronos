---
sidebar_position: 3
title: Configuration
description: Complete configuration reference for chronos.yaml with all fields, types, defaults, and environment variable overrides
---

# Configuration Reference

Chronos is configured via a YAML file (default: `chronos.yaml`). Every option has a sensible default, so you only need to specify what you want to change.

## Loading Configuration

```bash
# Specify config file path
chronos --config /etc/chronos/chronos.yaml

# Default: looks for chronos.yaml in the current directory
chronos
```

## Complete Example

```yaml title="chronos.yaml"
cluster:
  node_id: chronos-1
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.0.1:7000
    peers:
      - chronos-2:7000
      - chronos-3:7000
    heartbeat_timeout: 500ms
    election_timeout: 1s
    snapshot_interval: 30s
    snapshot_threshold: 1000

server:
  http:
    address: 0.0.0.0:8080
    read_timeout: 30s
    write_timeout: 30s
  grpc:
    address: 0.0.0.0:9000

scheduler:
  tick_interval: 1s
  execution_timeout: 5m
  missed_run_policy: execute_one
  default_retry_policy:
    max_attempts: 3
    initial_interval: 1s
    max_interval: 1m
    multiplier: 2.0

dispatcher:
  http:
    timeout: 30s
    max_idle_conns: 100
    idle_conn_timeout: 90s
  concurrency:
    max_concurrent_jobs: 100
    per_job_limit: 1

metrics:
  prometheus:
    enabled: true
    path: /metrics

logging:
  level: info
  format: json
  output: stdout

auth:
  type: none
```

---

## Cluster

Settings that identify the node and control Raft consensus.

### `cluster`

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `node_id` | string | `chronos-1` | **Yes** | Unique identifier for this node. Must be different for each node in the cluster. |
| `data_dir` | string | `./data` | **Yes** | Directory for Raft logs, snapshots, and BadgerDB data. Must be writable. Use SSD storage in production. |

### `cluster.raft`

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `address` | string | `127.0.0.1:7000` | **Yes** | Address this node listens on for Raft communication. Must be reachable by all peers. |
| `peers` | list of strings | `[]` | No | Addresses of other cluster nodes. Leave empty for the bootstrap node. |
| `heartbeat_timeout` | duration | `500ms` | No | How often the leader sends heartbeats to followers. Lower values detect failures faster but increase network traffic. |
| `election_timeout` | duration | `1s` | No | How long a follower waits without a heartbeat before starting an election. Must be greater than `heartbeat_timeout`. |
| `snapshot_interval` | duration | `30s` | No | How often the node creates Raft snapshots for log compaction. |
| `snapshot_threshold` | uint64 | `1000` | No | Number of committed Raft log entries before triggering a snapshot. |

**Example — 3-node production cluster:**

```yaml
# Node 1 (bootstrap)
cluster:
  node_id: chronos-1
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.0.1:7000
    peers: []  # Empty for bootstrap node

# Node 2
cluster:
  node_id: chronos-2
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.0.2:7000
    peers:
      - 10.0.0.1:7000

# Node 3
cluster:
  node_id: chronos-3
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.0.3:7000
    peers:
      - 10.0.0.1:7000
      - 10.0.0.2:7000
```

:::tip Tuning Raft Timeouts
For faster failover, reduce timeouts — but stay above your network latency:
- **Low-latency datacenter** (under 1ms): `heartbeat_timeout: 200ms`, `election_timeout: 500ms`
- **Cross-AZ** (1–5ms): `heartbeat_timeout: 500ms`, `election_timeout: 1s` (defaults)
- **Cross-region** (50–100ms): `heartbeat_timeout: 1s`, `election_timeout: 3s`
:::

---

## Server

HTTP and gRPC server settings.

### `server.http`

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `address` | string | `0.0.0.0:8080` | **Yes** | HTTP API listen address. Use `0.0.0.0` to listen on all interfaces. |
| `read_timeout` | duration | `30s` | No | Maximum time to read the full request including body. |
| `write_timeout` | duration | `30s` | No | Maximum time to write the response. |

### `server.grpc`

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `address` | string | `0.0.0.0:9000` | **Yes** | gRPC server listen address. |

**Example — custom ports with TLS:**

```yaml
server:
  http:
    address: 0.0.0.0:443
    read_timeout: 60s
    write_timeout: 60s
  grpc:
    address: 0.0.0.0:9443
```

---

## Scheduler

Controls how Chronos evaluates and dispatches scheduled jobs.

### `scheduler`

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `tick_interval` | duration | `1s` | No | How often the scheduler checks for due jobs. Lower values increase precision but consume more CPU. |
| `execution_timeout` | duration | `5m` | No | Default maximum execution time for jobs that don't specify their own timeout. |
| `missed_run_policy` | string | `execute_one` | No | How to handle jobs that were missed while the scheduler was down. See below. |

### Missed Run Policies

| Policy | Behavior |
|--------|----------|
| `ignore` | Skip all missed runs. Use when stale executions have no value. |
| `execute_one` | Execute once to catch up, regardless of how many runs were missed. **Default.** |
| `execute_all` | Execute once for each missed run. Use with caution — can cause a burst of executions. |

### `scheduler.default_retry_policy`

Default retry policy applied to jobs that don't specify their own.

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `max_attempts` | int | `3` | No | Maximum number of retry attempts (1–100). |
| `initial_interval` | duration | `1s` | No | Wait time after the first failure. |
| `max_interval` | duration | `1m` | No | Maximum wait between retries (caps exponential growth). |
| `multiplier` | float | `2.0` | No | Exponential backoff multiplier applied to each retry interval. |

**Retry timing example** with defaults (`initial: 1s`, `multiplier: 2.0`, `max: 1m`):

```
Attempt 1: immediate
Attempt 2: wait 1s
Attempt 3: wait 2s
```

**Example — aggressive retry for critical jobs:**

```yaml
scheduler:
  tick_interval: 500ms
  execution_timeout: 30m
  missed_run_policy: execute_all
  default_retry_policy:
    max_attempts: 5
    initial_interval: 5s
    max_interval: 5m
    multiplier: 3.0
```

---

## Dispatcher

Controls how Chronos makes HTTP requests to job targets.

### `dispatcher.http`

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `timeout` | duration | `30s` | No | HTTP request timeout for webhook calls. |
| `max_idle_conns` | int | `100` | No | Maximum number of idle (keep-alive) connections across all hosts. |
| `idle_conn_timeout` | duration | `90s` | No | How long an idle connection is kept before closing. |

### `dispatcher.concurrency`

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `max_concurrent_jobs` | int | `100` | No | Maximum number of job executions running simultaneously across the cluster. |
| `per_job_limit` | int | `1` | No | Maximum concurrent executions for a single job. |

**Example — high-throughput configuration:**

```yaml
dispatcher:
  http:
    timeout: 60s
    max_idle_conns: 500
    idle_conn_timeout: 120s
  concurrency:
    max_concurrent_jobs: 500
    per_job_limit: 5
```

---

## Metrics

Prometheus metrics endpoint configuration.

### `metrics.prometheus`

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `enabled` | bool | `true` | No | Enable or disable the Prometheus metrics endpoint. |
| `path` | string | `/metrics` | No | HTTP path where metrics are served. |

**Example — disable metrics:**

```yaml
metrics:
  prometheus:
    enabled: false
```

---

## Logging

Structured logging configuration.

### `logging`

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `level` | string | `info` | No | Minimum log level. Options: `debug`, `info`, `warn`, `error`. |
| `format` | string | `json` | No | Log output format. Options: `json` (structured), `text` (human-readable). |
| `output` | string | `stdout` | No | Log destination. Options: `stdout`, `stderr`, or a file path (e.g., `/var/log/chronos.log`). |

**Example — debug logging to file:**

```yaml
logging:
  level: debug
  format: json
  output: /var/log/chronos/chronos.log
```

:::tip Production Logging
Use `level: info` and `format: json` in production. JSON logs are easily parsed by log aggregation tools (Loki, Elasticsearch, Datadog). Set `level: debug` only when troubleshooting.
:::

---

## Authentication

API authentication configuration.

### `auth`

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `type` | string | `none` | No | Authentication type. Options: `none` (disabled), `basic` (HTTP Basic Auth). |
| `users` | map | `{}` | When `type: basic` | Map of username → bcrypt password hash. |

**Example — enable basic auth:**

```yaml
auth:
  type: basic
  users:
    admin: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
    readonly: "$2a$10$VEjxo0jq5YRIQGOBqFBk6OG2syihtiDKuFMKs2E44yhMzEeSNqzJm"
```

Generate bcrypt hashes with:

```bash
# Using htpasswd
htpasswd -nbBC 10 "" "your-password" | cut -d: -f2

# Using Python
python3 -c "import bcrypt; print(bcrypt.hashpw(b'your-password', bcrypt.gensalt()).decode())"
```

:::note
Authentication is disabled by default for ease of development. **Always enable authentication in production.** See the [Authentication Guide](/docs/guides/authentication) for advanced setups including JWT and OAuth 2.0.
:::

---

## Environment Variable Overrides

Core settings can be overridden with environment variables. CLI flags and environment variables take precedence over values in `chronos.yaml`.

| Environment Variable | Config Field | Example |
|---------------------|-------------|---------|
| `CHRONOS_NODE_ID` | `cluster.node_id` | `CHRONOS_NODE_ID=chronos-2` |
| `CHRONOS_DATA_DIR` | `cluster.data_dir` | `CHRONOS_DATA_DIR=/mnt/ssd/chronos` |
| `CHRONOS_RAFT_ADDRESS` | `cluster.raft.address` | `CHRONOS_RAFT_ADDRESS=10.0.0.2:7000` |
| `CHRONOS_RAFT_PEERS` | `cluster.raft.peers` | `CHRONOS_RAFT_PEERS=10.0.0.1:7000,10.0.0.3:7000` |
| `CHRONOS_HTTP_ADDRESS` | `server.http.address` | `CHRONOS_HTTP_ADDRESS=0.0.0.0:9090` |
| `CHRONOS_LOG_LEVEL` | `logging.level` | `CHRONOS_LOG_LEVEL=debug` |

:::tip Docker and Kubernetes
Environment variables are ideal for container deployments where mounting config files adds complexity:

```bash
docker run -d \
  -e CHRONOS_NODE_ID=chronos-1 \
  -e CHRONOS_HTTP_ADDRESS=0.0.0.0:8080 \
  -e CHRONOS_RAFT_ADDRESS=0.0.0.0:7000 \
  -e CHRONOS_LOG_LEVEL=info \
  chronos/chronos:latest
```
:::

---

## Validation

Chronos validates configuration at startup and exits with an error if required fields are missing:

```bash
# Validate without starting the server
chronos config validate -f /etc/chronos/chronos.yaml
```

**Required fields:**
- `cluster.node_id` — must not be empty
- `cluster.data_dir` — must not be empty
- `cluster.raft.address` — must not be empty
- `server.http.address` — must not be empty

---

## Duration Format

All duration fields accept Go duration strings:

| Unit | Suffix | Example |
|------|--------|---------|
| Milliseconds | `ms` | `500ms` |
| Seconds | `s` | `30s` |
| Minutes | `m` | `5m` |
| Hours | `h` | `1h` |
| Combined | — | `1h30m`, `2m30s` |

---

## Minimal Configuration

The smallest valid configuration for a single-node development setup:

```yaml title="chronos-dev.yaml"
cluster:
  node_id: dev
  data_dir: ./data
  raft:
    address: 127.0.0.1:7000

server:
  http:
    address: 0.0.0.0:8080
```

All other fields use their defaults.

---

## Production Recommendations

```yaml title="chronos-production.yaml"
cluster:
  node_id: chronos-1           # Unique per node
  data_dir: /var/lib/chronos   # SSD-backed storage
  raft:
    address: 10.0.0.1:7000
    peers:
      - 10.0.0.2:7000
      - 10.0.0.3:7000
    heartbeat_timeout: 500ms
    election_timeout: 1s
    snapshot_interval: 30s
    snapshot_threshold: 1000

server:
  http:
    address: 0.0.0.0:8080
    read_timeout: 30s
    write_timeout: 30s

scheduler:
  tick_interval: 1s
  execution_timeout: 5m
  missed_run_policy: execute_one
  default_retry_policy:
    max_attempts: 3
    initial_interval: 1s
    max_interval: 1m
    multiplier: 2.0

dispatcher:
  http:
    timeout: 30s
    max_idle_conns: 100
    idle_conn_timeout: 90s
  concurrency:
    max_concurrent_jobs: 100
    per_job_limit: 1

metrics:
  prometheus:
    enabled: true
    path: /metrics

logging:
  level: info
  format: json
  output: stdout

auth:
  type: basic
  users:
    admin: "$2a$10$..."  # Generate with htpasswd or bcrypt
```

---

## See Also

- [CLI Reference](/docs/reference/cli) — `chronosctl` command-line options
- [High Availability Guide](/docs/guides/high-availability) — Production cluster setup
- [Monitoring Guide](/docs/guides/monitoring) — Prometheus and Grafana setup
