---
sidebar_position: 4
title: Webhooks
description: Configuring HTTP webhooks for job execution in Chronos
---

# Webhooks

Webhooks are the primary dispatch mechanism in Chronos. When a job is due, Chronos makes an HTTP request to your specified endpoint.

## Basic Configuration

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "method": "POST"
  }
}
```

## Full Configuration

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json",
      "X-Custom-Header": "value"
    },
    "body": "{\"action\": \"process\"}",
    "auth": {
      "type": "bearer",
      "token": "your-api-token"
    },
    "success_codes": [200, 201, 202],
    "timeout": "30s"
  }
}
```

## Authentication

### Bearer Token

Most common for API authentication:

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "auth": {
      "type": "bearer",
      "token": "eyJhbGciOiJIUzI1NiIs..."
    }
  }
}
```

Sends header: `Authorization: Bearer eyJhbGciOiJIUzI1NiIs...`

### Basic Auth

For services using username/password:

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "auth": {
      "type": "basic",
      "username": "api_user",
      "password": "api_password"
    }
  }
}
```

Sends header: `Authorization: Basic base64(username:password)`

### API Key

For services using custom API key headers:

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "auth": {
      "type": "api_key",
      "api_key": "your-api-key-here",
      "header": "X-API-Key"
    }
  }
}
```

Sends header: `X-API-Key: your-api-key-here`

Default header is `X-API-Key` if not specified.

### Custom Headers

For other authentication schemes, use custom headers:

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "headers": {
      "Authorization": "CustomScheme token123",
      "X-Signature": "sha256=abc123"
    }
  }
}
```

## Request Body

### Static Body

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json"
    },
    "body": "{\"action\": \"backup\", \"type\": \"full\"}"
  }
}
```

### Template Variables

Chronos supports template variables in the body:

```json
{
  "webhook": {
    "body": "{\"job_id\": \"{{.JobID}}\", \"execution_id\": \"{{.ExecutionID}}\", \"scheduled_time\": \"{{.ScheduledTime}}\"}"
  }
}
```

Available variables:

| Variable | Description |
|----------|-------------|
| `{{.JobID}}` | Job unique identifier |
| `{{.JobName}}` | Job name |
| `{{.ExecutionID}}` | Current execution ID |
| `{{.ScheduledTime}}` | Scheduled run time (RFC3339) |
| `{{.Attempt}}` | Current attempt number |

## Success Codes

By default, any 2xx status code is considered success. Customize this:

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "success_codes": [200, 201, 202, 204]
  }
}
```

### Response Handling

| Status Code | Chronos Behavior |
|-------------|------------------|
| 2xx (or custom success) | Success, record completion |
| 4xx | Failure, **no retry** (client error) |
| 5xx | Failure, **will retry** (server error) |
| Timeout | Failure, **will retry** |
| Connection error | Failure, **will retry** |

:::note 4xx No Retry
Chronos assumes 4xx errors are client errors (bad request, unauthorized) that won't be fixed by retrying. If your API returns 4xx for temporary failures, consider returning 5xx or 503 instead.
:::

## Timeouts

### Per-Job Timeout

```json
{
  "timeout": "5m",
  "webhook": {
    "url": "https://api.example.com/long-running"
  }
}
```

### Webhook-Specific Timeout

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "timeout": "30s"
  }
}
```

Webhook timeout takes precedence over job timeout.

## Retry Behavior

Configure retry behavior in the job's retry policy:

```json
{
  "retry_policy": {
    "max_attempts": 5,
    "initial_interval": "10s",
    "max_interval": "5m",
    "multiplier": 2.0
  },
  "webhook": {
    "url": "https://api.example.com/endpoint"
  }
}
```

Retry sequence with above config:
1. Initial request fails
2. Wait 10s, retry (attempt 2)
3. Wait 20s, retry (attempt 3)
4. Wait 40s, retry (attempt 4)
5. Wait 80s, retry (attempt 5)
6. Marked as failed

## Request Headers

Chronos adds these headers automatically:

| Header | Value |
|--------|-------|
| `User-Agent` | `Chronos/1.0` |
| `X-Chronos-Job-ID` | Job ID |
| `X-Chronos-Execution-ID` | Execution ID |
| `X-Chronos-Attempt` | Attempt number |

Your custom headers override defaults (except `User-Agent`).

## TLS Configuration

### Skip Certificate Verification (Development Only)

```json
{
  "webhook": {
    "url": "https://self-signed.example.com",
    "tls_skip_verify": true
  }
}
```

:::danger Production Warning
Never use `tls_skip_verify: true` in production. It disables certificate validation and is vulnerable to man-in-the-middle attacks.
:::

### Custom CA Certificate

For internal CAs, configure at the Chronos server level:

```yaml
# chronos.yaml
dispatcher:
  http:
    tls:
      ca_file: /etc/chronos/ca.crt
```

## Debugging Webhooks

### Test Webhook Configuration

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/test \
  -H "Content-Type: application/json" \
  -d '{
    "webhook": {
      "url": "https://api.example.com/endpoint",
      "method": "POST",
      "headers": {
        "Content-Type": "application/json"
      },
      "body": "{\"test\": true}"
    }
  }'
```

Response:

```json
{
  "success": true,
  "status_code": 200,
  "response_body": "{\"received\": true}",
  "duration_ms": 145
}
```

### View Execution Details

```bash
curl http://localhost:8080/api/v1/jobs/my-job/executions/exec-123
```

```json
{
  "id": "exec-123",
  "status": "failed",
  "attempts": 3,
  "last_error": "connection timeout",
  "request": {
    "url": "https://api.example.com/endpoint",
    "method": "POST",
    "headers": {"Content-Type": "application/json"}
  },
  "response": {
    "status_code": 0,
    "error": "dial tcp: i/o timeout"
  }
}
```

## Best Practices

### Idempotent Handlers

Design your webhook handlers to be idempotent:

```python
@app.route('/process', methods=['POST'])
def process():
    execution_id = request.headers.get('X-Chronos-Execution-ID')
    
    # Check if already processed
    if is_processed(execution_id):
        return jsonify({"status": "already_processed"}), 200
    
    # Process and record
    do_work()
    mark_processed(execution_id)
    
    return jsonify({"status": "completed"}), 200
```

### Return Meaningful Status Codes

| Scenario | Recommended Code |
|----------|------------------|
| Success | 200, 201, 204 |
| Invalid input | 400 |
| Authentication failed | 401, 403 |
| Resource not found | 404 |
| Temporary failure | 503 |
| Rate limited | 429 (Chronos will retry) |

### Set Appropriate Timeouts

```json
{
  "timeout": "30s",  // Job-level default
  "webhook": {
    "timeout": "10s"  // Override for fast endpoints
  }
}
```

### Use Structured Logging

Include correlation IDs in your handler logs:

```python
import logging

@app.route('/webhook', methods=['POST'])
def handle_webhook():
    job_id = request.headers.get('X-Chronos-Job-ID')
    exec_id = request.headers.get('X-Chronos-Execution-ID')
    
    logging.info(f"Processing job={job_id} execution={exec_id}")
    # ...
```

## Next Steps

- [Clustering Configuration](/docs/core-concepts/clustering)
- [Monitoring and Metrics](/docs/guides/monitoring)
