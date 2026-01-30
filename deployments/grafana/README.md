# Chronos Grafana Dashboard

This directory contains a pre-built Grafana dashboard for monitoring Chronos.

## Installation

### Method 1: Import via Grafana UI

1. Open Grafana and go to **Dashboards** → **Import**
2. Upload `dashboard.json` or paste its contents
3. Select your Prometheus datasource
4. Click **Import**

### Method 2: Provisioning

Add to your Grafana provisioning configuration:

```yaml
# /etc/grafana/provisioning/dashboards/chronos.yaml
apiVersion: 1
providers:
  - name: "Chronos"
    orgId: 1
    folder: "Chronos"
    type: file
    disableDeletion: false
    editable: true
    options:
      path: /var/lib/grafana/dashboards/chronos
```

Then copy `dashboard.json` to `/var/lib/grafana/dashboards/chronos/`.

### Method 3: Kubernetes ConfigMap

```bash
kubectl create configmap chronos-dashboard \
  --from-file=dashboard.json \
  -n monitoring
```

## Panels

| Panel | Description |
|-------|-------------|
| **Total Jobs** | Current number of registered jobs |
| **Enabled Jobs** | Number of jobs that are enabled |
| **Leader Status** | Whether this instance is the Raft leader |
| **Cluster Peers** | Number of nodes in the Raft cluster |
| **Executions Rate** | Rate of job executions per second (5m avg) |
| **Execution Duration** | p50, p95, p99 latency percentiles |
| **Executions by Status** | Stacked bar chart of success/failed executions |

## Variables

- **datasource**: Select your Prometheus datasource
- **instance**: Filter by Chronos instance (supports multi-select)

## Prometheus Configuration

Ensure Prometheus is scraping Chronos metrics:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'chronos'
    static_configs:
      - targets: ['chronos:8080']
    metrics_path: /metrics
```

## Alerting

Example Prometheus alerting rules:

```yaml
groups:
  - name: chronos
    rules:
      - alert: ChronosNoLeader
        expr: sum(chronos_raft_is_leader) == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "No Chronos leader elected"
          
      - alert: ChronosHighFailureRate
        expr: rate(chronos_executions_total{status="failed"}[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High job failure rate"
```
