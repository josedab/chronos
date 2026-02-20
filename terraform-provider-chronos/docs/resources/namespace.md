---
page_title: "chronos_namespace Resource - Chronos Provider"
description: |-
  Manages a Chronos namespace for multi-tenant job isolation.
---

# chronos_namespace (Resource)

Manages a Chronos namespace for multi-tenant job isolation.

## Example Usage

```hcl
resource "chronos_namespace" "production" {
  name        = "production"
  description = "Production workloads"

  labels = {
    environment = "production"
    team        = "platform"
  }
}

resource "chronos_job" "prod_job" {
  name      = "prod-health-check"
  schedule  = "*/5 * * * *"
  namespace = chronos_namespace.production.id

  webhook_url    = "https://api.example.com/health"
  webhook_method = "GET"
}
```

## Argument Reference

- `name` - (Required) Name of the namespace.
- `description` - (Optional) Human-readable description.
- `labels` - (Optional) Key-value labels for organization.

## Attribute Reference

- `id` - The unique identifier of the namespace.

## Import

```shell
terraform import chronos_namespace.production <namespace-id>
```
