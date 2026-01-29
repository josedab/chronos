---
sidebar_position: 3
title: First Job
description: Create and manage your first Chronos job with webhooks, retries, and monitoring
---

# Create Your First Job

This guide walks you through creating a complete job with webhooks, retry policies, and monitoring.

## Job Anatomy

A Chronos job consists of:

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Human-readable identifier | Yes |
| `schedule` | Cron expression or interval | Yes |
| `webhook` | Target endpoint configuration | Yes |
| `retry_policy` | Retry behavior on failure | No |
| `timeout` | Maximum execution time | No |
| `enabled` | Whether the job is active | No |

## Create a Simple Job

Let's create a job that calls an API every hour:

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hourly-health-check",
    "description": "Check API health every hour",
    "schedule": "0 * * * *",
    "webhook": {
      "url": "https://api.example.com/health",
      "method": "GET"
    },
    "enabled": true
  }'
```

## Add Retry Policy

Make your job resilient with automatic retries:

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "critical-backup",
    "schedule": "0 2 * * *",
    "timezone": "America/New_York",
    "webhook": {
      "url": "https://backup.example.com/trigger",
      "method": "POST",
      "headers": {
        "Authorization": "Bearer your-api-key"
      },
      "body": "{\"type\": \"full\"}"
    },
    "retry_policy": {
      "max_attempts": 5,
      "initial_interval": "10s",
      "max_interval": "5m",
      "multiplier": 2.0
    },
    "timeout": "30m",
    "enabled": true
  }'
```

### Retry Policy Fields

| Field | Description | Default |
|-------|-------------|---------|
| `max_attempts` | Maximum retry attempts | 3 |
| `initial_interval` | Wait time after first failure | 1s |
| `max_interval` | Maximum wait between retries | 1m |
| `multiplier` | Exponential backoff multiplier | 2.0 |

With the above configuration:
- 1st retry: 10 seconds
- 2nd retry: 20 seconds  
- 3rd retry: 40 seconds
- 4th retry: 80 seconds
- 5th retry: 5 minutes (capped by max_interval)

## Authentication Methods

### Bearer Token

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "auth": {
      "type": "bearer",
      "token": "your-jwt-token"
    }
  }
}
```

### Basic Auth

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "auth": {
      "type": "basic",
      "username": "user",
      "password": "pass"
    }
  }
}
```

### API Key

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "auth": {
      "type": "api_key",
      "api_key": "your-api-key",
      "header": "X-API-Key"
    }
  }
}
```

## Concurrency Policies

Control what happens when a job is still running when the next execution is due:

```json
{
  "name": "long-running-job",
  "schedule": "*/5 * * * *",
  "concurrency": "forbid",
  "webhook": { ... }
}
```

| Policy | Behavior |
|--------|----------|
| `allow` | Start new execution regardless of running jobs |
| `forbid` | Skip execution if previous is still running (default) |
| `replace` | Cancel running execution and start new one |

## Using the CLI

### Create from YAML

```yaml title="daily-report.yaml"
name: daily-report
description: Generate and send daily report
schedule: "0 9 * * 1-5"
timezone: America/Los_Angeles
webhook:
  url: https://reports.example.com/generate
  method: POST
  headers:
    Content-Type: application/json
  body: |
    {"report_type": "daily", "format": "pdf"}
retry_policy:
  max_attempts: 3
  initial_interval: 30s
timeout: 10m
enabled: true
```

```bash
chronosctl job create -f daily-report.yaml
```

### Common CLI Commands

```bash
# List all jobs
chronosctl job list

# Get job details
chronosctl job get daily-report

# Update a job
chronosctl job update daily-report --schedule "0 10 * * 1-5"

# Enable/disable
chronosctl job enable daily-report
chronosctl job disable daily-report

# Trigger manually
chronosctl job trigger daily-report

# Delete a job
chronosctl job delete daily-report
```

## Monitor Executions

### View Execution History

```bash
# Via API
curl http://localhost:8080/api/v1/jobs/daily-report/executions?limit=10

# Via CLI
chronosctl execution list daily-report --limit 10
```

### Execution Statuses

| Status | Description |
|--------|-------------|
| `pending` | Scheduled, waiting to execute |
| `running` | Currently executing |
| `success` | Completed successfully (2xx response) |
| `failed` | Failed after all retry attempts |
| `skipped` | Skipped due to concurrency policy |

### Check Metrics

Chronos exposes Prometheus metrics at `/metrics`:

```bash
curl http://localhost:8080/metrics | grep chronos_

# Key metrics:
# chronos_jobs_total - Total number of jobs
# chronos_executions_total{status="success"} - Successful executions
# chronos_executions_total{status="failed"} - Failed executions
# chronos_execution_duration_seconds - Execution duration histogram
```

## Best Practices

:::tip Job Naming
Use descriptive, kebab-case names that indicate what the job does:
- ✅ `daily-database-backup`
- ✅ `hourly-cache-refresh`
- ❌ `job1`
- ❌ `my_job`
:::

:::tip Timeouts
Always set appropriate timeouts to prevent hung jobs from blocking resources:
```json
{
  "timeout": "5m"
}
```
:::

:::tip Tags
Use tags for organization and filtering:
```json
{
  "tags": {
    "team": "backend",
    "environment": "production",
    "priority": "critical"
  }
}
```
:::

## Next Steps

- [Understanding Cron Schedules](/docs/core-concepts/scheduling)
- [Webhook Deep Dive](/docs/core-concepts/webhooks)
- [Set up Monitoring](/docs/guides/monitoring)
