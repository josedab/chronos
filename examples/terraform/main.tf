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

variable "chronos_api_key" {
  type      = string
  sensitive = true
}

# Production namespace with quotas
resource "chronos_namespace" "production" {
  name        = "production"
  description = "Production workloads"

  labels = {
    environment = "production"
    team        = "platform"
  }
}

# Daily report job
resource "chronos_job" "daily_report" {
  name        = "daily-report"
  description = "Generate daily analytics report"
  schedule    = "0 9 * * *"
  timezone    = "America/New_York"
  enabled     = true
  namespace   = chronos_namespace.production.id

  webhook_url    = "https://api.example.com/reports/generate"
  webhook_method = "POST"
  webhook_body   = jsonencode({ type = "daily", format = "pdf" })

  max_retries = 3
  timeout     = "10m"
  concurrency = "forbid"

  tags = {
    team     = "analytics"
    priority = "high"
  }
}

# Health check running every 5 minutes
resource "chronos_job" "health_check" {
  name     = "api-health-check"
  schedule = "*/5 * * * *"
  enabled  = true

  webhook_url    = "https://api.example.com/health"
  webhook_method = "GET"
  timeout        = "30s"
  max_retries    = 1

  tags = {
    type = "monitoring"
  }
}

# Database backup every night
resource "chronos_job" "db_backup" {
  name        = "nightly-db-backup"
  description = "PostgreSQL database backup"
  schedule    = "0 2 * * *"
  timezone    = "UTC"
  enabled     = true
  namespace   = chronos_namespace.production.id

  webhook_url    = "https://api.example.com/backup/postgres"
  webhook_method = "POST"
  timeout        = "30m"
  max_retries    = 2
  concurrency    = "forbid"

  tags = {
    team     = "infrastructure"
    priority = "critical"
  }
}

output "report_job_id" {
  value = chronos_job.daily_report.id
}

output "health_check_id" {
  value = chronos_job.health_check.id
}
