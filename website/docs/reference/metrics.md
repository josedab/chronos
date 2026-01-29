---
sidebar_position: 4
title: Metrics
description: Prometheus metrics exposed by Chronos
---

# Metrics Reference

Chronos exposes Prometheus metrics at `/metrics`.

## Job Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `chronos_jobs_total` | Gauge | Total number of jobs |
| `chronos_jobs_enabled` | Gauge | Enabled jobs count |

## Execution Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `chronos_executions_total` | Counter | status | Total executions |
| `chronos_execution_duration_seconds` | Histogram | job_name | Execution duration |
| `chronos_execution_attempts` | Counter | job_name | Retry attempts |

## Cluster Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `chronos_raft_is_leader` | Gauge | 1 if leader |
| `chronos_raft_peers` | Gauge | Peer count |
| `chronos_raft_commit_index` | Gauge | Commit index |

## Example Queries

```promql
# Execution success rate
sum(rate(chronos_executions_total{status="success"}[5m])) / 
sum(rate(chronos_executions_total[5m]))

# P99 execution duration
histogram_quantile(0.99, rate(chronos_execution_duration_seconds_bucket[5m]))
```
