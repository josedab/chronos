---
sidebar_position: 2
title: Jobs
description: Understanding Chronos jobs - the core unit of scheduled work
---

# Jobs

A **job** is the fundamental unit of work in Chronos. It defines what to execute, when to execute it, and how to handle failures.

## Job Model

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "daily-backup",
  "description": "Backup database to S3",
  "schedule": "0 2 * * *",
  "timezone": "America/New_York",
  "webhook": {
    "url": "https://api.example.com/backup",
    "method": "POST",
    "headers": {
      "Authorization": "Bearer token123"
    },
    "body": "{\"type\": \"full\"}"
  },
  "retry_policy": {
    "max_attempts": 3,
    "initial_interval": "1s",
    "multiplier": 2.0
  },
  "timeout": "30m",
  "concurrency": "forbid",
  "tags": {
    "team": "platform",
    "priority": "critical"
  },
  "enabled": true,
  "created_at": "2026-01-29T10:00:00Z",
  "updated_at": "2026-01-29T10:00:00Z",
  "next_run": "2026-01-30T07:00:00Z"
}
```

## Field Reference

### Core Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique human-readable identifier |
| `description` | string | No | Job description |
| `schedule` | string | Yes | Cron expression or interval |
| `timezone` | string | No | IANA timezone (default: UTC) |
| `enabled` | boolean | No | Whether the job is active (default: false) |

### Webhook Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `webhook.url` | string | Yes | Target URL |
| `webhook.method` | string | No | HTTP method (default: GET) |
| `webhook.headers` | object | No | Custom HTTP headers |
| `webhook.body` | string | No | Request body |
| `webhook.auth` | object | No | Authentication configuration |
| `webhook.success_codes` | array | No | Success status codes (default: 200-299) |

### Execution Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `timeout` | duration | 5m | Maximum execution time |
| `concurrency` | string | forbid | Concurrency policy |
| `retry_policy` | object | - | Retry configuration |
| `tags` | object | - | Custom key-value tags |

## Job Lifecycle

```
┌─────────┐     create      ┌──────────┐
│         │────────────────▶│          │
│  Draft  │                 │ Disabled │
│         │◀────────────────│          │
└─────────┘     disable     └──────────┘
                                 │
                              enable
                                 │
                                 ▼
                            ┌──────────┐
                            │          │
                            │ Enabled  │◀──────┐
                            │          │       │
                            └──────────┘       │
                                 │             │
                            scheduled          │
                                 │             │
                                 ▼             │
                            ┌──────────┐       │
                            │          │       │
                            │ Pending  │       │
                            │          │       │
                            └──────────┘       │
                                 │             │
                              execute          │
                                 │             │
                                 ▼             │
                            ┌──────────┐       │
                            │          │       │
                            │ Running  │───────┤
                            │          │       │
                            └──────────┘       │
                                 │             │
                    ┌────────────┼────────────┐│
                    ▼            ▼            ▼│
               ┌─────────┐ ┌─────────┐ ┌─────────┐
               │ Success │ │ Failed  │ │ Skipped │
               └─────────┘ └─────────┘ └─────────┘
                    │            │            │
                    └────────────┴────────────┘
                                 │
                          reschedule
                                 │
                                 └─────────────┘
```

## Concurrency Policies

Control behavior when a job is due but still running from a previous execution:

### `allow`
Start a new execution regardless of running jobs. Use when:
- Executions are independent
- Multiple concurrent runs are acceptable
- Short-running jobs

```json
{
  "concurrency": "allow"
}
```

### `forbid` (default)
Skip execution if previous is still running. Use when:
- Jobs access shared resources
- Order matters
- Preventing resource exhaustion

```json
{
  "concurrency": "forbid"
}
```

### `replace`
Cancel running execution and start a new one. Use when:
- Only the latest data matters
- Stale executions should be abandoned
- Long-running jobs with frequently changing requirements

```json
{
  "concurrency": "replace"
}
```

## Tags

Tags are key-value metadata for organization and filtering:

```json
{
  "tags": {
    "team": "backend",
    "environment": "production",
    "priority": "critical",
    "owner": "alice@example.com"
  }
}
```

### Query by Tags

```bash
# List jobs by tag
curl "http://localhost:8080/api/v1/jobs?tag.team=backend"

# Multiple tags
curl "http://localhost:8080/api/v1/jobs?tag.team=backend&tag.environment=production"
```

## Job Versioning

Chronos automatically tracks job configuration changes:

```bash
# List versions
curl http://localhost:8080/api/v1/jobs/daily-backup/versions

# Get specific version
curl http://localhost:8080/api/v1/jobs/daily-backup/versions/2

# Rollback to previous version
curl -X POST http://localhost:8080/api/v1/jobs/daily-backup/rollback/1
```

## Job States

| State | Description |
|-------|-------------|
| `enabled` | Job is active and will execute on schedule |
| `disabled` | Job exists but won't execute |
| `paused` | Temporarily suspended (manual) |

## Best Practices

### Naming Conventions

Use descriptive kebab-case names:

```
✅ daily-database-backup
✅ hourly-metrics-aggregation
✅ weekly-report-generation

❌ job1
❌ myJob
❌ backup
```

### Idempotency

Design webhook handlers to be idempotent—running the same job twice should produce the same result:

```python
# Good: Check if work is already done
def handle_backup(request):
    if backup_exists_for_today():
        return {"status": "already_done"}
    perform_backup()
    return {"status": "completed"}

# Bad: Always create new backup
def handle_backup(request):
    perform_backup()  # Creates duplicate if retried
```

### Timeouts

Always set appropriate timeouts:

```json
{
  "timeout": "5m"  // Don't let jobs run forever
}
```

### Error Handling

Return appropriate HTTP status codes from your webhook:

| Code | Chronos Behavior |
|------|------------------|
| 2xx | Success, job complete |
| 4xx | Failure, no retry (bad request) |
| 5xx | Failure, will retry |
| Timeout | Failure, will retry |

## Next Steps

- [Scheduling Reference](/docs/core-concepts/scheduling)
- [Webhook Configuration](/docs/core-concepts/webhooks)
- [Monitoring Jobs](/docs/guides/monitoring)
