---
sidebar_position: 4
title: Metrics
description: Complete Prometheus metrics reference for Chronos — every metric, label, and PromQL query
---

# Metrics Reference

Chronos exposes Prometheus metrics at the `/metrics` endpoint (configurable via `metrics.prometheus.path`). All metrics use the `chronos_` namespace.

## Quick Start

```bash
# View all Chronos metrics
curl -s http://localhost:8080/metrics | grep "^chronos_"

# Verify metrics endpoint is active
curl -s http://localhost:8080/metrics | head -5
```

---

## Job Metrics

Track the number of jobs registered in the system.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `chronos_jobs_total` | Gauge | — | Total number of jobs (enabled + disabled). |
| `chronos_jobs_enabled` | Gauge | — | Number of currently enabled jobs. |

### PromQL Examples

```promql
# Total jobs
chronos_jobs_total

# Disabled jobs
chronos_jobs_total - chronos_jobs_enabled

# Percentage of jobs enabled
chronos_jobs_enabled / chronos_jobs_total * 100
```

---

## Execution Metrics

Track individual job executions, their outcomes, and performance.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `chronos_executions_total` | Counter | `job_id`, `job_name`, `status` | Total number of job executions. Incremented once per execution completion. |
| `chronos_execution_duration_seconds` | Histogram | `job_id`, `job_name` | Duration of job executions in seconds. Uses exponential buckets from 0.1s to ~1 hour. |

### Status Label Values

The `status` label on `chronos_executions_total` has the following values:

| Value | Description |
|-------|-------------|
| `success` | Execution completed with a success status code (2xx). |
| `failed` | Execution failed after all retry attempts. |
| `skipped` | Execution was skipped due to concurrency policy (`forbid`). |
| `timeout` | Execution exceeded the configured timeout. |
| `cancelled` | Execution was manually cancelled. |

### Histogram Buckets

`chronos_execution_duration_seconds` uses exponential buckets for wide coverage:

```
0.1s, 0.2s, 0.4s, 0.8s, 1.6s, 3.2s, 6.4s, 12.8s, 25.6s, 51.2s,
102.4s (~1.7m), 204.8s (~3.4m), 409.6s (~6.8m), 819.2s (~13.6m), 1638.4s (~27.3m)
```

### PromQL Examples

```promql
# Execution rate (per minute)
sum(rate(chronos_executions_total[5m])) * 60

# Failure rate (percentage)
sum(rate(chronos_executions_total{status="failed"}[5m]))
/ sum(rate(chronos_executions_total[5m])) * 100

# Success rate (percentage)
sum(rate(chronos_executions_total{status="success"}[5m]))
/ sum(rate(chronos_executions_total[5m])) * 100

# Executions per job in the last hour
sum(increase(chronos_executions_total[1h])) by (job_name)

# Failed executions per job in the last hour
sum(increase(chronos_executions_total{status="failed"}[1h])) by (job_name)

# P50 execution duration
histogram_quantile(0.50, sum(rate(chronos_execution_duration_seconds_bucket[5m])) by (le))

# P95 execution duration
histogram_quantile(0.95, sum(rate(chronos_execution_duration_seconds_bucket[5m])) by (le))

# P99 execution duration
histogram_quantile(0.99, sum(rate(chronos_execution_duration_seconds_bucket[5m])) by (le))

# P95 duration per job
histogram_quantile(0.95,
  sum(rate(chronos_execution_duration_seconds_bucket[5m])) by (le, job_name)
)

# Average execution duration per job
sum(rate(chronos_execution_duration_seconds_sum[5m])) by (job_name)
/ sum(rate(chronos_execution_duration_seconds_count[5m])) by (job_name)

# Top 10 slowest jobs by average duration
topk(10,
  sum(rate(chronos_execution_duration_seconds_sum[1h])) by (job_name)
  / sum(rate(chronos_execution_duration_seconds_count[1h])) by (job_name)
)

# Jobs with zero successes in the last hour (all failures)
sum(increase(chronos_executions_total{status="failed"}[1h])) by (job_name) > 0
unless
sum(increase(chronos_executions_total{status="success"}[1h])) by (job_name) > 0
```

---

## Scheduler Metrics

Track scheduler internals — tick performance, scheduled runs, and missed runs.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `chronos_scheduler_tick_duration_seconds` | Histogram | — | Time taken by each scheduler tick to evaluate due jobs. Uses fine-grained exponential buckets from 0.1ms. |
| `chronos_scheduled_runs_total` | Counter | — | Total number of runs scheduled by the scheduler. |
| `chronos_missed_runs_total` | Counter | — | Total number of runs missed (e.g., during downtime or leader transition). |

### PromQL Examples

```promql
# Scheduler tick rate
rate(chronos_scheduled_runs_total[5m]) * 60

# Missed runs in the last hour
increase(chronos_missed_runs_total[1h])

# P99 tick duration (should stay well below tick_interval)
histogram_quantile(0.99, rate(chronos_scheduler_tick_duration_seconds_bucket[5m]))

# Average tick duration
rate(chronos_scheduler_tick_duration_seconds_sum[5m])
/ rate(chronos_scheduler_tick_duration_seconds_count[5m])

# Missed run ratio
rate(chronos_missed_runs_total[5m])
/ (rate(chronos_scheduled_runs_total[5m]) + rate(chronos_missed_runs_total[5m]))
```

:::tip Tick Duration
If P99 tick duration approaches your `tick_interval` (default: `1s`), the scheduler is overloaded. Consider increasing `tick_interval` or reducing job count.
:::

---

## Cluster Metrics (Raft)

Monitor Raft consensus health — leadership, peer count, and replication status.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `chronos_raft_is_leader` | Gauge | — | `1` if this node is the Raft leader, `0` if follower. |
| `chronos_raft_peers` | Gauge | — | Number of Raft peers in the cluster (including this node). |

### PromQL Examples

```promql
# Current leader (should always be exactly 1 across the cluster)
sum(chronos_raft_is_leader)

# Number of followers
sum(1 - chronos_raft_is_leader)

# Cluster size
max(chronos_raft_peers)

# Detect no-leader condition (critical)
sum(chronos_raft_is_leader) == 0

# Detect split-brain (multiple leaders — critical)
sum(chronos_raft_is_leader) > 1

# Leader changes in the last hour (track stability)
changes(chronos_raft_is_leader[1h])
```

---

## HTTP API Metrics

Track the performance and reliability of the Chronos HTTP API itself (inbound requests to the Chronos server, not outbound webhook calls).

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `chronos_http_requests_total` | Counter | `method`, `path`, `status` | Total number of HTTP requests received by the API. |
| `chronos_http_request_duration_seconds` | Histogram | `method`, `path` | HTTP request processing duration. Uses default Prometheus buckets. |

### Label Values

**`method`:** `GET`, `POST`, `PUT`, `DELETE`

**`path`:** API endpoint path (e.g., `/api/v1/jobs`, `/api/v1/jobs/{id}`, `/health`)

**`status`:** HTTP status code as string (e.g., `200`, `201`, `400`, `404`, `500`)

### Default Histogram Buckets

`chronos_http_request_duration_seconds` uses standard Prometheus default buckets:

```
5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s
```

### PromQL Examples

```promql
# API request rate
sum(rate(chronos_http_requests_total[5m])) * 60

# Request rate by endpoint
sum(rate(chronos_http_requests_total[5m])) by (method, path)

# Error rate (4xx + 5xx)
sum(rate(chronos_http_requests_total{status=~"[45].."}[5m]))
/ sum(rate(chronos_http_requests_total[5m])) * 100

# 5xx error rate only
sum(rate(chronos_http_requests_total{status=~"5.."}[5m]))
/ sum(rate(chronos_http_requests_total[5m])) * 100

# P95 API latency
histogram_quantile(0.95, sum(rate(chronos_http_request_duration_seconds_bucket[5m])) by (le))

# P95 latency per endpoint
histogram_quantile(0.95,
  sum(rate(chronos_http_request_duration_seconds_bucket[5m])) by (le, path)
)

# Slowest endpoints by average latency
topk(5,
  sum(rate(chronos_http_request_duration_seconds_sum[5m])) by (path)
  / sum(rate(chronos_http_request_duration_seconds_count[5m])) by (path)
)
```

---

## Process Metrics

Standard Go process metrics are also exposed (provided by the Prometheus Go client library):

| Metric | Type | Description |
|--------|------|-------------|
| `process_resident_memory_bytes` | Gauge | Resident memory size in bytes. |
| `process_cpu_seconds_total` | Counter | Total user and system CPU time. |
| `process_open_fds` | Gauge | Number of open file descriptors. |
| `process_max_fds` | Gauge | Maximum number of open file descriptors. |
| `go_goroutines` | Gauge | Number of goroutines. |
| `go_memstats_alloc_bytes` | Gauge | Bytes allocated and still in use. |
| `go_gc_duration_seconds` | Summary | GC pause duration. |

### PromQL Examples

```promql
# Memory usage
process_resident_memory_bytes{job="chronos"}

# CPU usage rate
rate(process_cpu_seconds_total{job="chronos"}[5m])

# File descriptor usage ratio
process_open_fds{job="chronos"} / process_max_fds{job="chronos"}

# Goroutine count (sudden spikes may indicate leaks)
go_goroutines{job="chronos"}
```

---

## Key SLI/SLO Queries

Use these queries to define Service Level Indicators and Objectives:

### Availability (Is the cluster operational?)

```promql
# Cluster has exactly one leader
sum(chronos_raft_is_leader) == 1
```

### Execution Success Rate

```promql
# 30-day rolling success rate (SLO target: 99.9%)
sum(increase(chronos_executions_total{status="success"}[30d]))
/ sum(increase(chronos_executions_total[30d])) * 100
```

### Scheduling Latency

```promql
# P99 scheduler tick under 100ms (SLO target)
histogram_quantile(0.99, rate(chronos_scheduler_tick_duration_seconds_bucket[5m])) < 0.1
```

### API Latency

```promql
# P99 API latency under 500ms (SLO target)
histogram_quantile(0.99, sum(rate(chronos_http_request_duration_seconds_bucket[5m])) by (le)) < 0.5
```

---

## Alerting Rules

For production alerting rules based on these metrics (including no-leader detection, failure rate alerts, and circuit breaker monitoring), see the [Monitoring Guide](/docs/guides/monitoring#alerting-rules).

## Grafana Dashboard

For a ready-to-import Grafana dashboard JSON with panels for all key metrics, see the [Monitoring Guide](/docs/guides/monitoring#grafana-dashboard).

---

## See Also

- [Monitoring Guide](/docs/guides/monitoring) — Full setup with Prometheus, Grafana, and alerting
- [Configuration Reference](/docs/reference/configuration) — Configure metrics endpoint
- [Troubleshooting Guide](/docs/resources/troubleshooting) — Use metrics for diagnostics
