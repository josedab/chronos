---
sidebar_position: 1
title: API Reference
description: Complete REST API reference for Chronos
---

# API Reference

Complete REST API documentation for Chronos.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

When authentication is enabled, include your API key:

```bash
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/jobs
```

## Jobs

### List Jobs

```
GET /api/v1/jobs
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | int | Max results (default: 100) |
| `offset` | int | Pagination offset |
| `enabled` | bool | Filter by enabled status |

**Response:**

```json
{
  "success": true,
  "data": {
    "jobs": [...],
    "total": 42
  }
}
```

### Create Job

```
POST /api/v1/jobs
```

**Request Body:**

```json
{
  "name": "my-job",
  "schedule": "0 * * * *",
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "method": "POST"
  },
  "enabled": true
}
```

### Get Job

```
GET /api/v1/jobs/{id}
```

### Update Job

```
PUT /api/v1/jobs/{id}
```

### Delete Job

```
DELETE /api/v1/jobs/{id}
```

### Trigger Job

```
POST /api/v1/jobs/{id}/trigger
```

### Enable/Disable Job

```
POST /api/v1/jobs/{id}/enable
POST /api/v1/jobs/{id}/disable
```

## Executions

### List Executions

```
GET /api/v1/jobs/{id}/executions
```

### Get Execution

```
GET /api/v1/jobs/{id}/executions/{execId}
```

## Cluster

### Get Status

```
GET /api/v1/cluster/status
```

### Health Check

```
GET /health
```

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_JSON` | 400 | Invalid JSON body |
| `VALIDATION_ERROR` | 400 | Request validation failed |
| `NOT_FOUND` | 404 | Resource not found |
| `UNAUTHORIZED` | 401 | Authentication required |
| `INTERNAL_ERROR` | 500 | Server error |
