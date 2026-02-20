---
page_title: "chronos_job Data Source - Chronos Provider"
description: |-
  Reads information about an existing Chronos job.
---

# chronos_job (Data Source)

Reads information about an existing Chronos job by ID.

## Example Usage

```hcl
data "chronos_job" "existing" {
  id = "abc-123"
}

output "job_schedule" {
  value = data.chronos_job.existing.schedule
}
```

## Argument Reference

- `id` - (Required) The ID of the job to look up.

## Attribute Reference

- `name` - The name of the job.
- `schedule` - The cron schedule expression.
- `enabled` - Whether the job is enabled.
- `webhook_url` - The webhook URL.
- `webhook_method` - The HTTP method.
- `namespace` - The job's namespace.
