---
sidebar_position: 3
title: Monitoring
description: Monitor Chronos with Prometheus, Grafana, and alerting
---

# Monitoring

Set up comprehensive monitoring for your Chronos deployment with Prometheus metrics, Grafana dashboards, and alerting rules.

## Overview

Chronos exposes Prometheus metrics at `/metrics` by default. These metrics provide visibility into:

- Job scheduling and execution
- Cluster health and leadership
- HTTP dispatcher performance
- Storage and Raft consensus

## Prometheus Setup

### Scrape Configuration

Add Chronos to your Prometheus configuration:

```yaml title="prometheus.yml"
scrape_configs:
  - job_name: 'chronos'
    static_configs:
      - targets: ['chronos-1:8080', 'chronos-2:8080', 'chronos-3:8080']
    relabel_configs:
      - source_labels: [__address__]
        target_label: instance
        regex: '([^:]+):\d+'
        replacement: '${1}'
```

For Kubernetes with ServiceMonitor:

```yaml title="servicemonitor.yaml"
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: chronos
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: chronos
  endpoints:
    - port: http
      path: /metrics
      interval: 15s
  namespaceSelector:
    matchNames:
      - scheduling
```

### Verify Metrics

```bash
curl http://localhost:8080/metrics | grep chronos_
```

## Key Metrics

### Job Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `chronos_jobs_total` | Gauge | Total number of jobs |
| `chronos_jobs_enabled` | Gauge | Number of enabled jobs |
| `chronos_jobs_by_status` | Gauge | Jobs by status (enabled/disabled/paused) |

### Execution Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `chronos_executions_total` | Counter | `status`, `job` | Total executions |
| `chronos_execution_duration_seconds` | Histogram | `job` | Execution duration |
| `chronos_executions_in_progress` | Gauge | - | Currently running executions |
| `chronos_execution_retries_total` | Counter | `job` | Retry attempts |

### Scheduler Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `chronos_scheduler_tick_duration_seconds` | Histogram | Scheduler tick processing time |
| `chronos_scheduler_jobs_due` | Gauge | Jobs due for execution |
| `chronos_scheduler_jobs_skipped_total` | Counter | Skipped due to concurrency |

### Cluster Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `chronos_raft_is_leader` | Gauge | Leader status (1=leader, 0=follower) |
| `chronos_raft_peers` | Gauge | Number of Raft peers |
| `chronos_raft_commit_index` | Gauge | Raft commit index |
| `chronos_raft_applied_index` | Gauge | Raft applied index |
| `chronos_raft_last_contact_seconds` | Gauge | Time since last leader contact |

### HTTP Dispatcher Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `chronos_http_requests_total` | Counter | `method`, `status` | Outbound HTTP requests |
| `chronos_http_request_duration_seconds` | Histogram | `method` | HTTP request latency |
| `chronos_http_circuit_breaker_state` | Gauge | `target` | Circuit breaker state |

## Grafana Dashboard

Import the following dashboard JSON into Grafana for comprehensive Chronos monitoring.

### Dashboard JSON

```json title="chronos-dashboard.json"
{
  "dashboard": {
    "title": "Chronos Overview",
    "uid": "chronos-overview",
    "timezone": "browser",
    "refresh": "30s",
    "panels": [
      {
        "title": "Cluster Status",
        "type": "stat",
        "gridPos": { "x": 0, "y": 0, "w": 6, "h": 4 },
        "targets": [{
          "expr": "sum(chronos_raft_is_leader)",
          "legendFormat": "Leaders"
        }],
        "fieldConfig": {
          "defaults": {
            "thresholds": {
              "steps": [
                { "value": 0, "color": "red" },
                { "value": 1, "color": "green" }
              ]
            }
          }
        }
      },
      {
        "title": "Total Jobs",
        "type": "stat",
        "gridPos": { "x": 6, "y": 0, "w": 6, "h": 4 },
        "targets": [{
          "expr": "chronos_jobs_total",
          "legendFormat": "Jobs"
        }]
      },
      {
        "title": "Enabled Jobs",
        "type": "stat",
        "gridPos": { "x": 12, "y": 0, "w": 6, "h": 4 },
        "targets": [{
          "expr": "chronos_jobs_enabled",
          "legendFormat": "Enabled"
        }]
      },
      {
        "title": "Executions in Progress",
        "type": "stat",
        "gridPos": { "x": 18, "y": 0, "w": 6, "h": 4 },
        "targets": [{
          "expr": "chronos_executions_in_progress",
          "legendFormat": "Running"
        }]
      },
      {
        "title": "Execution Rate",
        "type": "timeseries",
        "gridPos": { "x": 0, "y": 4, "w": 12, "h": 8 },
        "targets": [
          {
            "expr": "sum(rate(chronos_executions_total{status=\"success\"}[5m])) * 60",
            "legendFormat": "Success/min"
          },
          {
            "expr": "sum(rate(chronos_executions_total{status=\"failed\"}[5m])) * 60",
            "legendFormat": "Failed/min"
          }
        ]
      },
      {
        "title": "Execution Duration (p95)",
        "type": "timeseries",
        "gridPos": { "x": 12, "y": 4, "w": 12, "h": 8 },
        "targets": [{
          "expr": "histogram_quantile(0.95, sum(rate(chronos_execution_duration_seconds_bucket[5m])) by (le))",
          "legendFormat": "p95"
        }]
      },
      {
        "title": "Success Rate",
        "type": "gauge",
        "gridPos": { "x": 0, "y": 12, "w": 8, "h": 6 },
        "targets": [{
          "expr": "sum(rate(chronos_executions_total{status=\"success\"}[1h])) / sum(rate(chronos_executions_total[1h])) * 100",
          "legendFormat": "Success %"
        }],
        "fieldConfig": {
          "defaults": {
            "min": 0,
            "max": 100,
            "unit": "percent",
            "thresholds": {
              "steps": [
                { "value": 0, "color": "red" },
                { "value": 95, "color": "yellow" },
                { "value": 99, "color": "green" }
              ]
            }
          }
        }
      },
      {
        "title": "Raft Leader Contact",
        "type": "timeseries",
        "gridPos": { "x": 8, "y": 12, "w": 8, "h": 6 },
        "targets": [{
          "expr": "chronos_raft_last_contact_seconds",
          "legendFormat": "{{ instance }}"
        }]
      },
      {
        "title": "Executions by Status",
        "type": "piechart",
        "gridPos": { "x": 16, "y": 12, "w": 8, "h": 6 },
        "targets": [{
          "expr": "sum(increase(chronos_executions_total[24h])) by (status)",
          "legendFormat": "{{ status }}"
        }]
      },
      {
        "title": "Top 10 Jobs by Execution Count",
        "type": "table",
        "gridPos": { "x": 0, "y": 18, "w": 12, "h": 8 },
        "targets": [{
          "expr": "topk(10, sum(increase(chronos_executions_total[24h])) by (job))",
          "format": "table",
          "instant": true
        }]
      },
      {
        "title": "Failed Jobs (Last 24h)",
        "type": "table",
        "gridPos": { "x": 12, "y": 18, "w": 12, "h": 8 },
        "targets": [{
          "expr": "sum(increase(chronos_executions_total{status=\"failed\"}[24h])) by (job) > 0",
          "format": "table",
          "instant": true
        }]
      }
    ]
  }
}
```

### Import Dashboard

1. In Grafana, go to **Dashboards → Import**
2. Paste the JSON above or upload the file
3. Select your Prometheus data source
4. Click **Import**

## Alerting Rules

Create Prometheus alerting rules for critical conditions:

```yaml title="chronos-alerts.yaml"
groups:
  - name: chronos
    interval: 30s
    rules:
      # No leader elected
      - alert: ChronosNoLeader
        expr: sum(chronos_raft_is_leader) == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Chronos cluster has no leader"
          description: "No Chronos node is currently the leader. Job scheduling is halted."

      # Multiple leaders (split brain)
      - alert: ChronosMultipleLeaders
        expr: sum(chronos_raft_is_leader) > 1
        for: 30s
        labels:
          severity: critical
        annotations:
          summary: "Chronos cluster has multiple leaders"
          description: "Split brain detected. Multiple nodes claim leadership."

      # High execution failure rate
      - alert: ChronosHighFailureRate
        expr: |
          sum(rate(chronos_executions_total{status="failed"}[5m])) 
          / sum(rate(chronos_executions_total[5m])) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Chronos execution failure rate is high"
          description: "More than 10% of executions are failing in the last 5 minutes."

      # Job consistently failing
      - alert: ChronosJobFailing
        expr: |
          sum(increase(chronos_executions_total{status="failed"}[1h])) by (job) > 5
          and
          sum(increase(chronos_executions_total{status="success"}[1h])) by (job) == 0
        for: 0m
        labels:
          severity: warning
        annotations:
          summary: "Job {{ $labels.job }} is consistently failing"
          description: "Job has failed 5+ times in the last hour with no successes."

      # Execution duration too long
      - alert: ChronosSlowExecution
        expr: |
          histogram_quantile(0.95, 
            sum(rate(chronos_execution_duration_seconds_bucket[5m])) by (le, job)
          ) > 300
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Job {{ $labels.job }} executions are slow"
          description: "95th percentile execution time exceeds 5 minutes."

      # Raft peer lost
      - alert: ChronosPeerLost
        expr: chronos_raft_peers < 3
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Chronos cluster has fewer than 3 peers"
          description: "A Raft peer may have disconnected or failed."

      # Leader contact lost
      - alert: ChronosLeaderContactLost
        expr: chronos_raft_last_contact_seconds > 5
        for: 30s
        labels:
          severity: warning
        annotations:
          summary: "Chronos node losing contact with leader"
          description: "Node {{ $labels.instance }} hasn't contacted the leader in over 5 seconds."

      # Too many pending executions
      - alert: ChronosExecutionBacklog
        expr: chronos_scheduler_jobs_due > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Chronos has a large execution backlog"
          description: "More than 100 jobs are due but not yet executed."

      # Circuit breaker open
      - alert: ChronosCircuitBreakerOpen
        expr: chronos_http_circuit_breaker_state == 1
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Circuit breaker open for {{ $labels.target }}"
          description: "HTTP requests to {{ $labels.target }} are being blocked."
```

### Configure Alertmanager

Route alerts to your notification channels:

```yaml title="alertmanager.yml"
route:
  receiver: 'default'
  group_by: ['alertname', 'job']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  routes:
    - match:
        severity: critical
      receiver: 'pagerduty'
    - match:
        severity: warning
      receiver: 'slack'

receivers:
  - name: 'default'
    email_configs:
      - to: 'team@example.com'
  
  - name: 'slack'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/...'
        channel: '#alerts'
        title: '{{ .GroupLabels.alertname }}'
        text: '{{ .CommonAnnotations.description }}'

  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: '<your-service-key>'
```

## Health Checks

### Application Health

```bash
# Basic health check
curl http://localhost:8080/health

# Detailed health with metrics
curl http://localhost:8080/health?verbose=true
```

### Kubernetes Probes

```yaml title="deployment.yaml"
containers:
  - name: chronos
    livenessProbe:
      httpGet:
        path: /live
        port: 8080
      initialDelaySeconds: 10
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
```

## Logging

### Structured Logging

Chronos outputs structured JSON logs by default:

```json
{
  "level": "info",
  "ts": "2026-01-29T12:00:00Z",
  "msg": "job executed",
  "job_id": "daily-backup",
  "execution_id": "exec-abc123",
  "status": "success",
  "duration_ms": 1523
}
```

### Log Levels

| Level | Use Case |
|-------|----------|
| `debug` | Detailed debugging information |
| `info` | Normal operational messages |
| `warn` | Warning conditions |
| `error` | Error conditions |

Configure via:
```yaml
logging:
  level: info
  format: json
```

### Log Aggregation

**Loki + Grafana:**

```yaml title="promtail.yaml"
scrape_configs:
  - job_name: chronos
    static_configs:
      - targets:
          - localhost
        labels:
          job: chronos
          __path__: /var/log/chronos/*.log
```

**Elasticsearch:**

```yaml title="filebeat.yaml"
filebeat.inputs:
  - type: log
    paths:
      - /var/log/chronos/*.log
    json.keys_under_root: true
    json.add_error_key: true

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "chronos-%{+yyyy.MM.dd}"
```

## Distributed Tracing

### OpenTelemetry Setup

Enable tracing in Chronos configuration:

```yaml
tracing:
  enabled: true
  exporter: otlp
  endpoint: "http://jaeger:4318"
  sample_rate: 0.1  # Sample 10% of requests
```

### Trace Propagation

Chronos propagates trace context to webhook targets via headers:

```
traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
tracestate: chronos=exec-abc123
```

### Viewing Traces

Access traces in Jaeger or your preferred tracing backend to:

- Debug slow executions
- Understand execution flow
- Identify bottlenecks in webhook handlers

## Runbooks

### High Failure Rate

1. Check which jobs are failing:
   ```bash
   curl "http://localhost:8080/api/v1/jobs?sort=failure_rate&order=desc&limit=10"
   ```

2. Review execution logs:
   ```bash
   chronosctl execution list <job-name> --status failed --limit 5
   ```

3. Check target service health
4. Review retry policy configuration

### No Leader

1. Check Raft status on all nodes:
   ```bash
   for node in chronos-{1,2,3}; do
     echo "=== $node ==="
     curl http://$node:8080/api/v1/cluster/status
   done
   ```

2. Check network connectivity between nodes
3. Review Raft logs for election issues
4. Restart nodes if necessary (one at a time)

### Execution Backlog

1. Check scheduler status:
   ```bash
   curl http://localhost:8080/metrics | grep chronos_scheduler
   ```

2. Identify slow jobs:
   ```bash
   curl http://localhost:8080/api/v1/jobs?sort=avg_duration&order=desc
   ```

3. Consider scaling or adjusting timeouts

## Next Steps

- [Configuration Reference](/docs/reference/configuration) - Tune metrics and logging settings
- [High Availability Guide](/docs/guides/high-availability) - Production deployment best practices
- [Metrics Reference](/docs/reference/metrics) - Complete metrics documentation
