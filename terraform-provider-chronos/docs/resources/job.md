---
page_title: "chronos_job Resource - Chronos Provider"
description: |-
  Manages a Chronos scheduled job.
---

# chronos_job (Resource)

Manages a Chronos scheduled job with webhook-based execution.

## Example Usage

```hcl
resource "chronos_job" "daily_report" {
  name        = "daily-report"
  description = "Generate daily analytics report"
  schedule    = "0 9 * * *"
  timezone    = "America/New_York"
  enabled     = true

  webhook_url    = "https://api.example.com/reports/generate"
  webhook_method = "POST"
  webhook_body   = jsonencode({ type = "daily" })

  max_retries = 3
  timeout     = "5m"
  concurrency = "forbid"
  namespace   = "production"

  tags = {
    team        = "analytics"
    environment = "production"
  }
}
```

## Argument Reference

- `name` - (Required) Name of the job.
- `schedule` - (Required) Cron expression (e.g., `*/5 * * * *`, `@hourly`).
- `webhook_url` - (Required) URL to call when the job executes.
- `webhook_method` - (Optional) HTTP method. Default: `GET`.
- `webhook_body` - (Optional) HTTP request body.
- `description` - (Optional) Human-readable description.
- `timezone` - (Optional) IANA timezone. Default: `UTC`.
- `enabled` - (Optional) Whether the job is active. Default: `true`.
- `timeout` - (Optional) Execution timeout. Default: `5m`.
- `concurrency` - (Optional) Concurrency policy: `allow`, `forbid`, `replace`. Default: `allow`.
- `namespace` - (Optional) Namespace for multi-tenancy. Default: `default`.
- `max_retries` - (Optional) Maximum retry attempts. Default: `3`.
- `tags` - (Optional) Key-value tags for organization.

## Attribute Reference

- `id` - The unique identifier of the job.

## Import

Jobs can be imported using their ID:

```shell
terraform import chronos_job.daily_report <job-id>
```
