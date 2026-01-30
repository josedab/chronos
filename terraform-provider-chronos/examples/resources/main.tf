terraform {
  required_providers {
    chronos = {
      source = "registry.terraform.io/chronos/chronos"
    }
  }
}

provider "chronos" {
  endpoint = "http://localhost:8080"
  # api_key = "your-api-key"  # Or use CHRONOS_API_KEY env var
}

# Create a simple job
resource "chronos_job" "backup" {
  name        = "daily-backup"
  description = "Daily database backup"
  schedule    = "0 2 * * *"
  timezone    = "America/New_York"

  webhook_url    = "https://api.example.com/backup"
  webhook_method = "POST"
  webhook_body   = jsonencode({
    type = "full"
  })

  timeout     = "30m"
  concurrency = "forbid"
  max_retries = 3

  tags = {
    environment = "production"
    team        = "platform"
  }
}

# Create a namespace for team isolation
resource "chronos_namespace" "platform" {
  name        = "platform"
  description = "Platform team namespace"
  max_jobs    = 100

  labels = {
    team  = "platform"
    owner = "ops@example.com"
  }
}

# Create an event trigger
resource "chronos_trigger" "order_events" {
  name    = "order-processor"
  type    = "kafka"
  job_id  = chronos_job.backup.id
  enabled = true

  config = {
    brokers        = "kafka:9092"
    topic          = "orders"
    consumer_group = "chronos-orders"
  }
}

# Create a DAG workflow
resource "chronos_dag" "etl_pipeline" {
  name        = "etl-pipeline"
  description = "Daily ETL pipeline"

  nodes {
    id     = "extract"
    job_id = "job-extract-123"
  }

  nodes {
    id           = "transform"
    job_id       = "job-transform-456"
    dependencies = ["extract"]
    condition    = "all_success"
  }

  nodes {
    id           = "load"
    job_id       = "job-load-789"
    dependencies = ["transform"]
    condition    = "all_success"
  }
}

# Data source example
data "chronos_job" "existing" {
  id = "job-abc-123"
}

output "existing_job_schedule" {
  value = data.chronos_job.existing.schedule
}
