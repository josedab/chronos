---
sidebar_position: 1
title: API Reference
description: Complete REST API reference for Chronos
---

# API Reference

Complete REST API documentation for Chronos. All endpoints return JSON responses and accept JSON request bodies where applicable.

## Base URL

```
http://localhost:8080/api/v1
```

In production, replace with your Chronos server address.

## Authentication

Chronos supports multiple authentication methods when security is enabled.

### Bearer Token (JWT)

```bash
curl -H "Authorization: Bearer <your-jwt-token>" \
  http://localhost:8080/api/v1/jobs
```

### API Key

```bash
curl -H "X-API-Key: <your-api-key>" \
  http://localhost:8080/api/v1/jobs
```

### Basic Auth

```bash
curl -u username:password \
  http://localhost:8080/api/v1/jobs
```

:::note
Authentication is optional when running in development mode. Enable it in production via configuration.
:::

## Response Format

All successful responses follow this structure:

```json
{
  "success": true,
  "data": { ... },
  "meta": {
    "total": 100,
    "page": 1,
    "limit": 20
  }
}
```

Error responses:

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid cron expression",
    "details": {
      "field": "schedule",
      "value": "invalid"
    }
  }
}
```

---

## Jobs

Jobs are the core resource in Chronos. A job defines what to execute, when to execute it, and how to handle failures.

### Job Object

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "daily-backup",
  "description": "Backup database to S3 every night",
  "schedule": "0 2 * * *",
  "timezone": "America/New_York",
  "webhook": {
    "url": "https://api.example.com/backup",
    "method": "POST",
    "headers": {
      "Authorization": "Bearer token123",
      "Content-Type": "application/json"
    },
    "body": "{\"type\": \"full\"}",
    "timeout": "30s",
    "success_codes": [200, 201, 202]
  },
  "retry_policy": {
    "max_attempts": 3,
    "initial_interval": "1s",
    "max_interval": "5m",
    "multiplier": 2.0
  },
  "timeout": "30m",
  "concurrency": "forbid",
  "tags": {
    "team": "platform",
    "environment": "production"
  },
  "enabled": true,
  "created_at": "2026-01-29T10:00:00Z",
  "updated_at": "2026-01-29T10:00:00Z",
  "next_run": "2026-01-30T07:00:00Z",
  "last_run": "2026-01-29T07:00:00Z",
  "last_status": "success"
}
```

#### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string (UUID) | Auto | Unique job identifier |
| `name` | string | Yes | Human-readable name (1-128 chars, alphanumeric + hyphens) |
| `description` | string | No | Job description (max 1024 chars) |
| `schedule` | string | Yes | Cron expression or interval (`@every 5m`) |
| `timezone` | string | No | IANA timezone (default: `UTC`) |
| `webhook` | object | Yes | Webhook configuration |
| `retry_policy` | object | No | Retry configuration |
| `timeout` | duration | No | Max execution time (default: `5m`) |
| `concurrency` | string | No | `allow`, `forbid`, or `replace` (default: `forbid`) |
| `tags` | object | No | Key-value metadata |
| `enabled` | boolean | No | Whether job is active (default: `false`) |

#### Webhook Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | Target URL (must be HTTPS in production) |
| `method` | string | No | HTTP method (default: `GET`) |
| `headers` | object | No | Custom HTTP headers |
| `body` | string | No | Request body (for POST/PUT/PATCH) |
| `timeout` | duration | No | Request timeout (default: `30s`) |
| `success_codes` | array | No | Success status codes (default: `[200-299]`) |
| `auth` | object | No | Authentication configuration |

#### Retry Policy

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_attempts` | integer | 3 | Maximum retry attempts (1-100) |
| `initial_interval` | duration | `1s` | Wait time after first failure |
| `max_interval` | duration | `1m` | Maximum wait between retries |
| `multiplier` | float | 2.0 | Exponential backoff multiplier |

---

### List Jobs

```http
GET /api/v1/jobs
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | 100 | Results per page (1-1000) |
| `offset` | integer | 0 | Pagination offset |
| `enabled` | boolean | - | Filter by enabled status |
| `tag.<key>` | string | - | Filter by tag value |
| `search` | string | - | Search by name or description |
| `sort` | string | `created_at` | Sort field |
| `order` | string | `desc` | Sort order (`asc` or `desc`) |

**Example Request:**

```bash
curl "http://localhost:8080/api/v1/jobs?enabled=true&tag.team=backend&limit=10"
```

**Example Response:**

```json
{
  "success": true,
  "data": {
    "jobs": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "name": "daily-backup",
        "schedule": "0 2 * * *",
        "enabled": true,
        "next_run": "2026-01-30T07:00:00Z",
        "last_status": "success"
      }
    ],
    "total": 42
  },
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 42
  }
}
```

---

### Create Job

```http
POST /api/v1/jobs
```

**Request Body:**

```json
{
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
    "initial_interval": "10s"
  },
  "timeout": "30m",
  "tags": {
    "team": "platform"
  },
  "enabled": true
}
```

**Example Request:**

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "health-check",
    "schedule": "*/5 * * * *",
    "webhook": {
      "url": "https://api.example.com/health"
    },
    "enabled": true
  }'
```

**Response:** `201 Created`

```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "health-check",
    "schedule": "*/5 * * * *",
    "enabled": true,
    "next_run": "2026-01-29T12:05:00Z",
    "created_at": "2026-01-29T12:00:00Z"
  }
}
```

---

### Get Job

```http
GET /api/v1/jobs/{id}
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Job ID (UUID) or name |

**Example Request:**

```bash
curl http://localhost:8080/api/v1/jobs/daily-backup
```

**Response:** `200 OK`

Returns the full job object.

---

### Update Job

```http
PUT /api/v1/jobs/{id}
```

Updates a job. Only provided fields are updated (partial update).

**Example Request:**

```bash
curl -X PUT http://localhost:8080/api/v1/jobs/daily-backup \
  -H "Content-Type: application/json" \
  -d '{
    "schedule": "0 3 * * *",
    "timeout": "1h"
  }'
```

**Response:** `200 OK`

Returns the updated job object.

---

### Delete Job

```http
DELETE /api/v1/jobs/{id}
```

**Example Request:**

```bash
curl -X DELETE http://localhost:8080/api/v1/jobs/daily-backup
```

**Response:** `204 No Content`

---

### Trigger Job

Manually trigger a job execution outside of its schedule.

```http
POST /api/v1/jobs/{id}/trigger
```

**Optional Request Body:**

```json
{
  "payload": {
    "custom": "data"
  },
  "reason": "Manual trigger for testing"
}
```

**Example Request:**

```bash
curl -X POST http://localhost:8080/api/v1/jobs/daily-backup/trigger
```

**Response:** `202 Accepted`

```json
{
  "success": true,
  "data": {
    "execution_id": "exec-abc123",
    "job_id": "daily-backup",
    "status": "pending",
    "triggered_at": "2026-01-29T12:00:00Z"
  }
}
```

---

### Enable/Disable Job

```http
POST /api/v1/jobs/{id}/enable
POST /api/v1/jobs/{id}/disable
```

**Example Request:**

```bash
curl -X POST http://localhost:8080/api/v1/jobs/daily-backup/enable
```

**Response:** `200 OK`

```json
{
  "success": true,
  "data": {
    "id": "daily-backup",
    "enabled": true,
    "next_run": "2026-01-30T07:00:00Z"
  }
}
```

---

## Executions

Executions represent individual runs of a job.

### Execution Object

```json
{
  "id": "exec-abc123",
  "job_id": "daily-backup",
  "job_name": "daily-backup",
  "status": "success",
  "trigger": "scheduled",
  "scheduled_at": "2026-01-29T07:00:00Z",
  "started_at": "2026-01-29T07:00:01Z",
  "completed_at": "2026-01-29T07:05:32Z",
  "duration_ms": 331000,
  "attempt": 1,
  "request": {
    "url": "https://api.example.com/backup",
    "method": "POST",
    "headers": { "Content-Type": "application/json" }
  },
  "response": {
    "status_code": 200,
    "headers": { "Content-Type": "application/json" },
    "body": "{\"status\": \"completed\"}",
    "body_size": 25
  },
  "error": null,
  "node_id": "chronos-1"
}
```

#### Execution Status Values

| Status | Description |
|--------|-------------|
| `pending` | Queued, waiting to execute |
| `running` | Currently executing |
| `success` | Completed successfully |
| `failed` | Failed after all retry attempts |
| `skipped` | Skipped due to concurrency policy |
| `cancelled` | Cancelled by user or system |
| `timeout` | Exceeded timeout limit |

#### Trigger Types

| Trigger | Description |
|---------|-------------|
| `scheduled` | Normal scheduled execution |
| `manual` | Triggered via API or CLI |
| `retry` | Automatic retry after failure |
| `catchup` | Catch-up execution for missed runs |

---

### List Job Executions

```http
GET /api/v1/jobs/{id}/executions
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | 20 | Results per page |
| `offset` | integer | 0 | Pagination offset |
| `status` | string | - | Filter by status |
| `from` | datetime | - | Start time filter (ISO 8601) |
| `to` | datetime | - | End time filter (ISO 8601) |

**Example Request:**

```bash
curl "http://localhost:8080/api/v1/jobs/daily-backup/executions?status=failed&limit=5"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "executions": [
      {
        "id": "exec-abc123",
        "status": "failed",
        "started_at": "2026-01-29T07:00:01Z",
        "error": "Connection timeout"
      }
    ],
    "total": 3
  }
}
```

---

### Get Execution Details

```http
GET /api/v1/jobs/{jobId}/executions/{executionId}
```

**Example Request:**

```bash
curl http://localhost:8080/api/v1/jobs/daily-backup/executions/exec-abc123
```

Returns the full execution object including request/response details.

---

### Cancel Execution

Cancel a running or pending execution.

```http
POST /api/v1/jobs/{jobId}/executions/{executionId}/cancel
```

**Example Request:**

```bash
curl -X POST http://localhost:8080/api/v1/jobs/daily-backup/executions/exec-abc123/cancel
```

**Response:** `200 OK`

```json
{
  "success": true,
  "data": {
    "id": "exec-abc123",
    "status": "cancelled",
    "cancelled_at": "2026-01-29T07:02:00Z"
  }
}
```

---

## Cluster

### Get Cluster Status

```http
GET /api/v1/cluster/status
```

**Response:**

```json
{
  "success": true,
  "data": {
    "leader": "chronos-1",
    "state": "healthy",
    "nodes": [
      {
        "id": "chronos-1",
        "address": "10.0.0.1:7000",
        "state": "leader",
        "last_contact": "2026-01-29T12:00:00Z"
      },
      {
        "id": "chronos-2",
        "address": "10.0.0.2:7000",
        "state": "follower",
        "last_contact": "2026-01-29T12:00:00Z"
      },
      {
        "id": "chronos-3",
        "address": "10.0.0.3:7000",
        "state": "follower",
        "last_contact": "2026-01-29T12:00:00Z"
      }
    ],
    "jobs_total": 150,
    "jobs_enabled": 142,
    "version": "1.2.0"
  }
}
```

### Health Check

```http
GET /health
```

**Response:**

```json
{
  "status": "healthy",
  "leader": true,
  "version": "1.2.0"
}
```

For load balancer health checks, returns:
- `200 OK` if healthy
- `503 Service Unavailable` if unhealthy

### Readiness Check

```http
GET /ready
```

Returns `200 OK` when the node is ready to accept traffic. Use for Kubernetes readiness probes.

### Liveness Check

```http
GET /live
```

Returns `200 OK` when the node is alive. Use for Kubernetes liveness probes.

---

## Error Codes

| Code | HTTP Status | Description | Resolution |
|------|-------------|-------------|------------|
| `INVALID_JSON` | 400 | Malformed JSON body | Check JSON syntax |
| `VALIDATION_ERROR` | 400 | Request validation failed | Check field values |
| `INVALID_SCHEDULE` | 400 | Invalid cron expression | Use valid cron syntax |
| `JOB_NOT_FOUND` | 404 | Job does not exist | Check job ID/name |
| `EXECUTION_NOT_FOUND` | 404 | Execution does not exist | Check execution ID |
| `DUPLICATE_JOB` | 409 | Job with this name exists | Use unique name |
| `UNAUTHORIZED` | 401 | Authentication required | Provide valid credentials |
| `FORBIDDEN` | 403 | Insufficient permissions | Check RBAC permissions |
| `RATE_LIMITED` | 429 | Too many requests | Back off and retry |
| `NOT_LEADER` | 503 | Node is not the leader | Retry on leader node |
| `INTERNAL_ERROR` | 500 | Internal server error | Contact support |

---

## Rate Limiting

When rate limiting is enabled, responses include these headers:

```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1706536800
```

When rate limited, you'll receive a `429 Too Many Requests` response:

```json
{
  "success": false,
  "error": {
    "code": "RATE_LIMITED",
    "message": "Rate limit exceeded",
    "details": {
      "retry_after": 60
    }
  }
}
```

---

## Pagination

List endpoints support cursor-based or offset-based pagination:

**Offset-based (default):**
```
GET /api/v1/jobs?limit=20&offset=40
```

**Cursor-based (recommended for large datasets):**
```
GET /api/v1/jobs?limit=20&cursor=eyJpZCI6MTIzfQ
```

The response includes pagination metadata:

```json
{
  "meta": {
    "total": 150,
    "limit": 20,
    "offset": 40,
    "next_cursor": "eyJpZCI6MTYwfQ",
    "has_more": true
  }
}
```

---

## Webhooks (Outgoing)

Chronos can send webhook notifications for job events.

### Configure Webhooks

```http
POST /api/v1/webhooks
```

```json
{
  "url": "https://your-service.com/chronos-events",
  "events": ["job.execution.success", "job.execution.failed"],
  "secret": "your-webhook-secret"
}
```

### Event Types

| Event | Description |
|-------|-------------|
| `job.created` | New job created |
| `job.updated` | Job configuration changed |
| `job.deleted` | Job deleted |
| `job.enabled` | Job enabled |
| `job.disabled` | Job disabled |
| `job.execution.started` | Execution started |
| `job.execution.success` | Execution completed successfully |
| `job.execution.failed` | Execution failed |
| `job.execution.timeout` | Execution timed out |
| `cluster.leader_change` | Leadership changed |

### Webhook Payload

```json
{
  "id": "evt-abc123",
  "type": "job.execution.failed",
  "timestamp": "2026-01-29T12:00:00Z",
  "data": {
    "job_id": "daily-backup",
    "execution_id": "exec-abc123",
    "error": "Connection refused"
  }
}
```

### Webhook Signature

Verify webhook authenticity using the `X-Chronos-Signature` header:

```python
import hmac
import hashlib

def verify_signature(payload, signature, secret):
    expected = hmac.new(
        secret.encode(),
        payload.encode(),
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(f"sha256={expected}", signature)
```

---

## SDK Examples

### Go

```go
import "github.com/chronos/chronos-go"

client := chronos.NewClient("http://localhost:8080")

job, err := client.Jobs.Create(ctx, &chronos.JobCreate{
    Name:     "daily-backup",
    Schedule: "0 2 * * *",
    Webhook: &chronos.Webhook{
        URL:    "https://api.example.com/backup",
        Method: "POST",
    },
    Enabled: true,
})
```

### Python

```python
from chronos import ChronosClient

client = ChronosClient("http://localhost:8080")

job = client.jobs.create(
    name="daily-backup",
    schedule="0 2 * * *",
    webhook={"url": "https://api.example.com/backup"},
    enabled=True
)
```

### JavaScript/TypeScript

```typescript
import { ChronosClient } from '@chronos/sdk';

const client = new ChronosClient('http://localhost:8080');

const job = await client.jobs.create({
  name: 'daily-backup',
  schedule: '0 2 * * *',
  webhook: { url: 'https://api.example.com/backup' },
  enabled: true,
});
```

---

## OpenAPI Specification

The complete OpenAPI 3.1 specification is available at:

```
GET /api/v1/openapi.json
GET /api/v1/openapi.yaml
```

Interactive API documentation (Swagger UI):

```
GET /api/v1/docs
```
