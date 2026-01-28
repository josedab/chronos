# ADR-0004: HTTP Webhook-First Job Execution Model

## Status

Accepted

## Context

A distributed cron system needs a mechanism to execute jobs. The fundamental question is: **where does the job logic run?**

Options considered:

1. **Embedded scripts**: Jobs contain shell/Python scripts executed by Chronos
   - Risk: Arbitrary code execution on scheduler nodes; security nightmare
   - Complexity: Managing runtimes, sandboxing, resource limits
   
2. **Container execution**: Spawn containers for each job
   - Heavyweight: Container startup latency (seconds) unsuitable for frequent jobs
   - Dependency: Requires container runtime on all nodes
   
3. **Agent-based**: Deploy agents that pull and execute jobs
   - Complexity: Additional component to deploy and manage
   - State: Agents need to track what's running, handle failures
   
4. **Webhook dispatch**: Trigger external HTTP endpoints
   - Simplicity: HTTP is universal; any service can receive webhooks
   - Decoupling: Job logic lives in the target service, not in Chronos
   - Security: No arbitrary code execution on scheduler infrastructure

## Decision

Chronos uses an **HTTP webhook-first execution model**. Jobs are defined with a webhook configuration, and execution consists of making an HTTP request to the specified endpoint.

```go
// internal/models/job.go
type WebhookConfig struct {
    URL          string            `json:"url"`
    Method       string            `json:"method"`
    Headers      map[string]string `json:"headers,omitempty"`
    Body         string            `json:"body,omitempty"`
    Auth         *AuthConfig       `json:"auth,omitempty"`
    SuccessCodes []int             `json:"success_codes,omitempty"`
}
```

The dispatcher adds metadata headers to every request:
- `X-Chronos-Job-ID`: Job identifier
- `X-Chronos-Execution-ID`: Unique execution identifier
- `X-Chronos-Scheduled-Time`: When the job was scheduled to run
- `X-Chronos-Attempt`: Current retry attempt number

## Consequences

### Positive

- **Language agnostic**: Any service that speaks HTTP can be a job target—Python, Java, Go, serverless functions, etc.
- **Security isolation**: Chronos never executes arbitrary code; job logic runs in the target environment
- **Existing infrastructure**: Leverage existing API gateways, load balancers, and monitoring
- **Observability**: HTTP requests are easy to trace, log, and debug
- **Retry semantics**: HTTP status codes provide clear success/failure signals
- **Authentication flexibility**: Support for Basic, Bearer, and API Key authentication

### Negative

- **Network dependency**: Job execution requires network connectivity to targets
- **Latency overhead**: HTTP round-trip adds milliseconds vs. local execution
- **Target availability**: Jobs fail if the target service is unavailable
- **Payload limitations**: Large data must be passed by reference (URLs) rather than inline

### Tradeoffs Accepted

- **Simplicity over flexibility**: We can't run arbitrary scripts, but we gain security and operational simplicity
- **HTTP success codes**: We interpret 2xx as success by default; targets must return appropriate codes
- **Timeout handling**: Long-running jobs need appropriate timeout configuration

### Retry Policy

Jobs can specify retry behavior for transient failures:

```go
type RetryPolicy struct {
    MaxAttempts     int      `json:"max_attempts"`
    InitialInterval Duration `json:"initial_interval"`
    MaxInterval     Duration `json:"max_interval"`
    Multiplier      float64  `json:"multiplier"`
}
```

Retries occur for:
- Network errors (connection refused, timeout)
- 5xx server errors
- 429 Too Many Requests

Client errors (4xx except 429) are **not** retried as they indicate a configuration problem.

### Authentication Support

```go
type AuthConfig struct {
    Type     AuthType `json:"type"`      // basic, bearer, api_key
    Token    string   `json:"token"`     // For bearer auth
    APIKey   string   `json:"api_key"`   // For API key auth
    Header   string   `json:"header"`    // Header name for API key
    Username string   `json:"username"`  // For basic auth
    Password string   `json:"password"`
}
```

### Example Job Definition

```json
{
  "name": "daily-report",
  "schedule": "0 9 * * *",
  "webhook": {
    "url": "https://api.example.com/reports/generate",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json"
    },
    "auth": {
      "type": "bearer",
      "token": "${secret:vault:api/tokens:report-service}"
    }
  },
  "retry_policy": {
    "max_attempts": 3,
    "initial_interval": "1s",
    "multiplier": 2.0
  },
  "timeout": "5m"
}
```

## References

- [Webhooks vs Polling](https://hookdeck.com/webhooks/guides/webhooks-vs-polling)
- [GitHub Webhooks](https://docs.github.com/en/webhooks)
- [Stripe Webhook Best Practices](https://stripe.com/docs/webhooks/best-practices)
