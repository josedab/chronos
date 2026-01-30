# Chronos Terraform Provider

Terraform provider for managing [Chronos](https://github.com/chronos/chronos) distributed cron jobs.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for building)

## Installation

```hcl
terraform {
  required_providers {
    chronos = {
      source  = "chronos/chronos"
      version = "~> 1.0"
    }
  }
}
```

## Configuration

```hcl
provider "chronos" {
  endpoint = "http://localhost:8080"  # Or use CHRONOS_ENDPOINT env var
  api_key  = "your-api-key"           # Or use CHRONOS_API_KEY env var
  timeout  = 30                        # Request timeout in seconds
}
```

## Resources

### chronos_job

Manages a Chronos scheduled job.

```hcl
resource "chronos_job" "example" {
  name        = "my-job"
  description = "My scheduled job"
  schedule    = "*/5 * * * *"
  timezone    = "America/New_York"
  
  webhook_url    = "https://api.example.com/trigger"
  webhook_method = "POST"
  
  timeout     = "5m"
  concurrency = "forbid"
  max_retries = 3
  
  tags = {
    environment = "production"
  }
}
```

### chronos_dag

Manages a Chronos DAG workflow.

```hcl
resource "chronos_dag" "example" {
  name        = "my-pipeline"
  description = "ETL pipeline"
  
  nodes {
    id     = "extract"
    job_id = chronos_job.extract.id
  }
  
  nodes {
    id           = "transform"
    job_id       = chronos_job.transform.id
    dependencies = ["extract"]
    condition    = "all_success"
  }
}
```

### chronos_trigger

Manages an event trigger.

```hcl
resource "chronos_trigger" "example" {
  name    = "kafka-trigger"
  type    = "kafka"
  job_id  = chronos_job.example.id
  enabled = true
  
  config = {
    brokers        = "kafka:9092"
    topic          = "events"
    consumer_group = "chronos"
  }
}
```

### chronos_namespace

Manages a namespace for multi-tenant isolation.

```hcl
resource "chronos_namespace" "example" {
  name        = "team-a"
  description = "Team A namespace"
  max_jobs    = 50
  
  labels = {
    team = "team-a"
  }
}
```

## Data Sources

### chronos_job

```hcl
data "chronos_job" "example" {
  id = "job-123"
}
```

### chronos_jobs

```hcl
data "chronos_jobs" "all" {
  namespace = "production"
}
```

### chronos_dag

```hcl
data "chronos_dag" "example" {
  id = "dag-123"
}
```

## Building

```bash
go build -o terraform-provider-chronos
```

## Development

```bash
# Run tests
go test ./...

# Generate documentation
go generate ./...
```

## License

Apache 2.0
