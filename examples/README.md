# Examples

This directory contains example job configurations and usage patterns for Chronos.

## Files

- `simple-job.json` - Basic job with webhook
- `job-with-retry.json` - Job with retry policy
- `job-with-auth.json` - Job with authentication headers
- `jobs.yaml` - Multiple jobs in YAML format
- `docker-compose.yml` - Development setup with Chronos

## Quick Start

### Create a simple job

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d @simple-job.json
```

### Create jobs from YAML

```bash
chronosctl job create -f jobs.yaml
```

### Run local development environment

```bash
docker-compose up -d
```

## Job Configuration Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique job name |
| `schedule` | string | Yes | Cron expression or descriptor |
| `timezone` | string | No | IANA timezone (default: UTC) |
| `webhook.url` | string | Yes | Target webhook URL |
| `webhook.method` | string | No | HTTP method (default: POST) |
| `webhook.headers` | object | No | Request headers |
| `webhook.body` | string | No | Request body |
| `retry_policy.max_attempts` | int | No | Max retry attempts (default: 1) |
| `retry_policy.initial_interval` | duration | No | Initial retry delay |
| `retry_policy.multiplier` | float | No | Backoff multiplier |
| `timeout` | duration | No | Execution timeout |
| `concurrency_policy` | string | No | allow, forbid, or replace |
| `enabled` | bool | No | Job enabled state (default: true) |
