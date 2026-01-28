# Chronos API Reference

## Overview

The Chronos API is a RESTful API that allows you to manage jobs and view executions. All endpoints return JSON responses.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Authentication is optional and can be configured in `chronos.yaml`. When enabled, use HTTP Basic Auth.

## Response Format

All responses follow this format:

```json
{
  "success": true,
  "data": { ... },
  "error": null
}
```

Error responses:

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message"
  }
}
```

## Endpoints

### Health Check

```
GET /health
```

Returns server health status.

**Response:**

```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

---

### Jobs

#### List Jobs

```
GET /api/v1/jobs
```

Returns all jobs.

**Response:**

```json
{
  "success": true,
  "data": {
    "jobs": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "name": "daily-report",
        "description": "Generate daily report",
        "schedule": "0 9 * * *",
        "timezone": "America/New_York",
        "webhook": {
          "url": "https://api.example.com/reports",
          "method": "POST"
        },
        "enabled": true,
        "created_at": "2024-01-15T10:00:00Z",
        "updated_at": "2024-01-15T10:00:00Z",
        "next_run": "2024-01-16T14:00:00Z"
      }
    ],
    "total": 1
  }
}
```

#### Create Job

```
POST /api/v1/jobs
```

Creates a new job.

**Request Body:**

```json
{
  "name": "daily-report",
  "description": "Generate daily report",
  "schedule": "0 9 * * *",
  "timezone": "America/New_York",
  "webhook": {
    "url": "https://api.example.com/reports",
    "method": "POST",
    "headers": {
      "Authorization": "Bearer token123",
      "Content-Type": "application/json"
    },
    "body": "{\"type\": \"daily\"}",
    "auth": {
      "type": "bearer",
      "token": "my-secret-token"
    },
    "success_codes": [200, 201, 202]
  },
  "retry_policy": {
    "max_attempts": 3,
    "initial_interval": "1s",
    "max_interval": "1m",
    "multiplier": 2.0
  },
  "timeout": "5m",
  "concurrency": "forbid",
  "tags": {
    "team": "backend",
    "environment": "production"
  },
  "enabled": true
}
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | Yes | Job name |
| description | string | No | Job description |
| schedule | string | Yes | Cron expression |
| timezone | string | No | IANA timezone (default: UTC) |
| webhook | object | Yes | Webhook configuration |
| webhook.url | string | Yes | Target URL |
| webhook.method | string | No | HTTP method (default: GET) |
| webhook.headers | object | No | Custom headers |
| webhook.body | string | No | Request body |
| webhook.auth | object | No | Authentication config |
| webhook.success_codes | array | No | Success status codes (default: 200-299) |
| retry_policy | object | No | Retry configuration |
| timeout | string | No | Execution timeout (default: 5m) |
| concurrency | string | No | allow, forbid, replace (default: forbid) |
| tags | object | No | Custom tags |
| enabled | boolean | No | Whether job is active (default: false) |

**Auth Types:**

- `basic`: Uses `username` and `password`
- `bearer`: Uses `token`
- `api_key`: Uses `api_key` and optional `header` (default: X-API-Key)

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "daily-report",
    ...
  }
}
```

#### Get Job

```
GET /api/v1/jobs/{id}
```

Returns a specific job.

#### Update Job

```
PUT /api/v1/jobs/{id}
```

Updates a job. Request body same as Create Job.

#### Delete Job

```
DELETE /api/v1/jobs/{id}
```

Deletes a job and all its executions.

#### Trigger Job

```
POST /api/v1/jobs/{id}/trigger
```

Manually triggers a job execution.

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "exec-550e8400-e29b-41d4-a716-446655440000",
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "success",
    "attempts": 1,
    "started_at": "2024-01-15T10:30:00Z",
    "completed_at": "2024-01-15T10:30:01Z",
    "status_code": 200,
    "response": "OK",
    "duration": 1000000000
  }
}
```

#### Enable Job

```
POST /api/v1/jobs/{id}/enable
```

Enables a job.

#### Disable Job

```
POST /api/v1/jobs/{id}/disable
```

Disables a job.

---

### Executions

#### List Executions

```
GET /api/v1/jobs/{id}/executions?limit=20
```

Returns executions for a job (newest first).

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| limit | integer | 20 | Max results (1-100) |

**Response:**

```json
{
  "success": true,
  "data": {
    "executions": [
      {
        "id": "exec-550e8400-e29b-41d4-a716-446655440000",
        "job_id": "550e8400-e29b-41d4-a716-446655440000",
        "job_name": "daily-report",
        "scheduled_time": "2024-01-15T14:00:00Z",
        "started_at": "2024-01-15T14:00:00Z",
        "completed_at": "2024-01-15T14:00:01Z",
        "status": "success",
        "attempts": 1,
        "status_code": 200,
        "response": "OK",
        "duration": 1000000000,
        "node_id": "chronos-1"
      }
    ],
    "total": 1
  }
}
```

**Execution Status:**

- `pending`: Waiting to be executed
- `running`: Currently executing
- `success`: Completed successfully
- `failed`: Failed after all retries
- `skipped`: Skipped due to concurrency policy

#### Get Execution

```
GET /api/v1/jobs/{id}/executions/{execId}
```

Returns a specific execution.

#### Replay Execution

```
POST /api/v1/jobs/{id}/executions/{execId}/replay
```

Replays a previous execution for debugging.

**Response:**

```json
{
  "success": true,
  "data": {
    "replay_execution_id": "exec-new-id",
    "original_execution_id": "exec-original-id",
    "message": "Execution replay triggered"
  }
}
```

---

### Job Versioning

#### List Job Versions

```
GET /api/v1/jobs/{id}/versions
```

Returns all versions of a job configuration.

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "job_id": "550e8400-e29b-41d4-a716-446655440000",
      "version": 2,
      "config": { ... },
      "created_at": "2024-01-15T14:00:00Z",
      "created_by": "admin"
    }
  ]
}
```

#### Get Job Version

```
GET /api/v1/jobs/{id}/versions/{version}
```

Returns a specific version of a job configuration.

#### Rollback Job

```
POST /api/v1/jobs/{id}/rollback/{version}
```

Rolls back a job to a previous version.

**Response:**

```json
{
  "success": true,
  "data": {
    "job": { ... },
    "rolled_back_from": 1,
    "new_version": 3
  }
}
```

---

### Cluster

#### Cluster Status

```
GET /api/v1/cluster/status
```

Returns cluster status information.

**Response:**

```json
{
  "success": true,
  "data": {
    "is_leader": true,
    "jobs_total": 10,
    "running": 2,
    "next_runs": [
      {
        "job_id": "550e8400-e29b-41d4-a716-446655440000",
        "next_run": "2024-01-15T15:00:00Z"
      }
    ]
  }
}
```

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| INVALID_JSON | 400 | Invalid JSON body |
| VALIDATION_ERROR | 400 | Request validation failed |
| INVALID_TIMEOUT | 400 | Invalid timeout format |
| NOT_FOUND | 404 | Resource not found |
| STORE_ERROR | 500 | Storage operation failed |
| EXECUTION_ERROR | 500 | Job execution failed |
