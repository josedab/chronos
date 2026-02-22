---
sidebar_position: 8
title: Terraform Provider
---

# Managing Jobs with Terraform

Chronos provides an official Terraform provider for declarative job management.

## Installation

```hcl
terraform {
  required_providers {
    chronos = {
      source  = "chronos/chronos"
      version = "~> 0.1"
    }
  }
}

provider "chronos" {
  endpoint = "http://localhost:8080"
  api_key  = var.chronos_api_key
}
```

## Resources

### chronos_job

```hcl
resource "chronos_job" "daily_report" {
  name        = "daily-report"
  schedule    = "0 9 * * *"
  timezone    = "America/New_York"
  enabled     = true
  namespace   = "production"

  webhook_url    = "https://api.example.com/reports"
  webhook_method = "POST"
  webhook_body   = jsonencode({ type = "daily" })

  max_retries = 3
  timeout     = "10m"
  concurrency = "forbid"

  tags = {
    team = "analytics"
  }
}
```

### chronos_namespace

```hcl
resource "chronos_namespace" "production" {
  name        = "production"
  description = "Production workloads"
  labels = {
    environment = "production"
  }
}
```

## Data Sources

### chronos_job

```hcl
data "chronos_job" "existing" {
  id = "abc-123"
}
```

## Import

```bash
terraform import chronos_job.daily_report <job-id>
```

## Full Example

See [`examples/terraform/main.tf`](https://github.com/chronos/chronos/blob/main/examples/terraform/main.tf) for a complete working example.
