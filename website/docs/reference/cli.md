---
sidebar_position: 2
title: CLI Reference
description: Complete chronosctl command-line interface reference with flags, output examples, and workflows
---

# CLI Reference

`chronosctl` is the command-line interface for managing Chronos jobs, executions, and clusters.

## Installation

```bash
# Included with Chronos binary release
chronosctl --version

# Or install from source
go install github.com/chronos/chronos/cmd/chronosctl@latest
```

## Configuration

### Server Connection

`chronosctl` connects to a Chronos server to execute commands. Configure the server address using either an environment variable or a CLI flag:

```bash
# Environment variable (recommended for persistent config)
export CHRONOS_SERVER=http://chronos.example.com:8080

# CLI flag (overrides environment variable)
chronosctl --server http://localhost:8080 job list
```

The CLI flag takes precedence over the environment variable. The default server address is `http://localhost:8080`.

### Global Flags

These flags apply to all commands:

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--server` | `-s` | string | `http://localhost:8080` | Chronos server URL |
| `--output` | `-o` | string | `table` | Output format: `table`, `json`, `yaml` |
| `--help` | `-h` | — | — | Show help for any command |
| `--version` | — | — | — | Show version information |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CHRONOS_SERVER` | Chronos server URL | `http://localhost:8080` |

---

## Output Formats

All commands support three output formats via the `-o` flag:

### Table (default)

Human-readable tabular format for terminal use:

```bash
chronosctl job list
```

### JSON

Machine-readable format for scripting and automation:

```bash
chronosctl job list -o json
chronosctl job get daily-backup -o json | jq '.name'
```

### YAML

Structured format useful for configuration management and backups:

```bash
chronosctl job get daily-backup -o yaml > job-backup.yaml
```

---

## Job Commands

### `chronosctl job list`

List all jobs in the cluster.

```bash
chronosctl job list
chronosctl job list -o json
chronosctl job list -o yaml
```

**Example Output:**

```
Total: 3 jobs

ID        NAME           SCHEDULE      ENABLED  NEXT RUN
550e8400  daily-report   0 9 * * *     yes      2026-01-16 09:00:00
661f9511  hourly-sync    0 * * * *     yes      2026-01-15 16:00:00
772a0622  weekly-backup  0 0 * * 0     no       -
```

---

### `chronosctl job get <job-id>`

Get detailed information about a specific job. Accepts a full UUID or a short prefix (minimum 8 characters).

```bash
chronosctl job get 550e8400-e29b-41d4-a716-446655440000
chronosctl job get 550e8400   # Short ID works too
chronosctl job get daily-report  # Name also works
```

**Example Output:**

```
ID:          550e8400-e29b-41d4-a716-446655440000
Name:        daily-report
Description: Generate daily sales report
Schedule:    0 9 * * *
Timezone:    America/New_York
Enabled:     true
Webhook URL: https://api.example.com/reports
Method:      POST
Next Run:    2026-01-16 09:00:00
```

---

### `chronosctl job create -f <file>`

Create a new job from a YAML or JSON definition file.

| Flag | Short | Type | Required | Description |
|------|-------|------|----------|-------------|
| `--file` | `-f` | string | Yes | Path to job definition file (YAML or JSON) |

```bash
chronosctl job create -f job.yaml
chronosctl job create -f job.json
```

**Example job.yaml:**

```yaml title="daily-report.yaml"
name: daily-report
description: Generate daily sales report
schedule: "0 9 * * *"
timezone: America/New_York
webhook:
  url: https://api.example.com/reports
  method: POST
  headers:
    Authorization: Bearer token123
    Content-Type: application/json
  body: '{"type": "daily"}'
retry_policy:
  max_attempts: 3
  initial_interval: 1s
  max_interval: 1m
  multiplier: 2.0
timeout: 5m
concurrency: forbid
tags:
  team: backend
  environment: production
enabled: true
```

**Example Output:**

```
Job created: daily-report (550e8400-e29b-41d4-a716-446655440000)
```

---

### `chronosctl job update <job-id> [flags]`

Update an existing job's properties.

```bash
chronosctl job update daily-report --schedule "0 10 * * *"
```

**Example Output:**

```
Job updated: daily-report
```

---

### `chronosctl job delete <job-id>`

Delete a job and all its execution history.

```bash
chronosctl job delete daily-report
```

**Example Output:**

```
Job deleted
```

---

### `chronosctl job trigger <job-id>`

Manually trigger a job execution immediately, outside of its schedule.

```bash
chronosctl job trigger daily-report
```

**Example Output:**

```
Job triggered: execution exec-772a0622-1234-5678-9abc-def012345678, status: success
```

---

### `chronosctl job enable <job-id>`

Enable a disabled job so it runs on schedule.

```bash
chronosctl job enable daily-report
```

**Example Output:**

```
Job enabled
```

---

### `chronosctl job disable <job-id>`

Disable a job to prevent scheduled executions.

```bash
chronosctl job disable daily-report
```

**Example Output:**

```
Job disabled
```

---

## Execution Commands

### `chronosctl execution list <job-id>`

List execution history for a job, sorted newest first.

```bash
chronosctl execution list daily-report
chronosctl execution list daily-report -o json
```

**Example Output:**

```
Total: 5 executions

ID        STATUS   ATTEMPTS  STARTED              DURATION
exec-001  success  1         2026-01-15 09:00:01  1.234s
exec-002  success  1         2026-01-14 09:00:00  1.156s
exec-003  failed   3         2026-01-13 09:00:00  45.678s
exec-004  success  2         2026-01-12 09:00:00  2.345s
exec-005  success  1         2026-01-11 09:00:00  1.089s
```

---

### `chronosctl execution get <job-id> <execution-id>`

Get detailed information about a specific execution, including request and response data.

```bash
chronosctl execution get daily-report exec-001
```

**Example Output:**

```
ID:         exec-001-e29b-41d4-a716-446655440000
Job ID:     550e8400-e29b-41d4-a716-446655440000
Status:     success
Attempts:   1
Started:    2026-01-15 09:00:01
Response:   {"status": "ok", "records_processed": 1234}
```

---

## Cluster Commands

### `chronosctl cluster status`

Display cluster status including leadership, node count, and active jobs.

```bash
chronosctl cluster status
chronosctl cluster status -o json
```

**Example Output (table):**

```
Is Leader:  true
Jobs Total: 42
```

**Example Output (JSON):**

```json
{
  "is_leader": true,
  "jobs_total": 42,
  "running": 2,
  "next_runs": [
    {
      "job_id": "550e8400-e29b-41d4-a716-446655440000",
      "next_run": "2026-01-15T15:00:00Z"
    }
  ]
}
```

---

## Migration Commands

Migrate jobs from external schedulers to Chronos.

### `chronosctl migrate k8s-discover`

Discover Kubernetes CronJobs in a cluster.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--kubeconfig` | — | string | `~/.kube/config` | Path to kubeconfig file |
| `--context` | — | string | — | Kubernetes context to use |
| `--namespace` | `-n` | string | all | Filter by namespace |
| `--selector` | `-l` | string | — | Label selector (e.g., `app=myapp`) |
| `--include-disabled` | — | bool | `false` | Include suspended CronJobs |

```bash
# Discover all CronJobs
chronosctl migrate k8s-discover

# Filter by namespace and labels
chronosctl migrate k8s-discover -n production -l app=myapp

# Use specific kubeconfig and include suspended jobs
chronosctl migrate k8s-discover --kubeconfig ~/.kube/prod-config --include-disabled
```

**Example Output:**

```
Discovered 5 CronJobs across 2 namespaces

NAMESPACE    NAME              SCHEDULE      IMAGE                  SUSPENDED  WARNINGS
production   daily-backup      0 2 * * *     backup:latest          no         0
production   hourly-cleanup    0 * * * *     cleanup:v1.2.3         no         1
staging      test-job          */5 * * * *   test:latest            no         0
staging      weekly-report     0 0 * * 0     report:v2.0.0          yes        0
default      legacy-cron       0 6 * * *     legacy-app:old         no         2
```

---

### `chronosctl migrate k8s-plan`

Create a migration plan for Kubernetes CronJobs without making changes.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--kubeconfig` | — | string | `~/.kube/config` | Path to kubeconfig file |
| `--context` | — | string | — | Kubernetes context to use |
| `--namespace` | `-n` | string | all | Filter by namespace |
| `--selector` | `-l` | string | — | Label selector |
| `--target-namespace` | — | string | — | Target Chronos namespace |
| `--adapter-url` | — | string | `http://chronos-k8s-adapter:8081` | Webhook adapter base URL |
| `--output-file` | `-f` | string | — | Write plan to file (YAML) |

```bash
# Create plan and save to file
chronosctl migrate k8s-plan -n production -f migration-plan.yaml

# Specify custom adapter URL
chronosctl migrate k8s-plan -n production \
  --target-namespace prod-jobs \
  --adapter-url http://chronos-k8s-adapter:8081
```

**Example Output:**

```
Migration Plan: k8s-migration-2026-01-15
Source Cluster: prod-cluster
Target Namespace: prod-jobs
Total Jobs: 3

SOURCE                    TARGET NAME            ACTION  WARNINGS
production/daily-backup   prod-daily-backup      create  0
production/hourly-cleanup prod-hourly-cleanup    create  1
production/weekly-report  prod-weekly-report     create  0
```

---

### `chronosctl migrate k8s-apply`

Apply a migration plan to import CronJobs into Chronos.

| Flag | Short | Type | Required | Description |
|------|-------|------|----------|-------------|
| `--file` | `-f` | string | Yes | Migration plan file to apply |
| `--dry-run` | — | bool | No | Preview changes without applying |

```bash
# Preview changes first
chronosctl migrate k8s-apply -f migration-plan.yaml --dry-run

# Apply the migration
chronosctl migrate k8s-apply -f migration-plan.yaml
```

**Example Output:**

```
Migration Results:
  Created: 3
  Updated: 0
  Skipped: 0
  Failed:  0
```

---

### `chronosctl migrate from-file`

Import jobs from various external scheduler formats.

| Flag | Short | Type | Required | Description |
|------|-------|------|----------|-------------|
| `--file` | `-f` | string | Yes | Source file to import |
| `--source-type` | — | string | Yes | Source scheduler format (see table below) |
| `--dry-run` | — | bool | No | Preview without applying |

**Supported Source Types:**

| Type | Description |
|------|-------------|
| `kubernetes` | Kubernetes CronJob YAML |
| `airflow` | Apache Airflow DAG definition |
| `eventbridge` | AWS EventBridge rule JSON |
| `temporal` | Temporal workflow JSON |
| `github-actions` | GitHub Actions workflow YAML |

```bash
# Migrate from Kubernetes CronJob
chronosctl migrate from-file -f cronjob.yaml --source-type kubernetes

# Migrate from Airflow DAG
chronosctl migrate from-file -f dag.py --source-type airflow

# Migrate from AWS EventBridge
chronosctl migrate from-file -f rule.json --source-type eventbridge

# Preview only
chronosctl migrate from-file -f cronjob.yaml --source-type kubernetes --dry-run
```

---

## Common Workflows

### Create and Test a Job

```bash
# Create the job (disabled by default if enabled not set)
chronosctl job create -f daily-report.yaml

# Trigger manually to test
chronosctl job trigger daily-report

# Check execution result
chronosctl execution list daily-report

# If successful, enable for scheduling
chronosctl job enable daily-report
```

### Export Jobs for Backup

```bash
# Export all jobs as YAML
chronosctl job list -o yaml > all-jobs-backup.yaml

# Export a specific job
chronosctl job get daily-report -o yaml > daily-report-backup.yaml
```

### Migrate from Kubernetes CronJobs

```bash
# 1. Discover existing CronJobs
chronosctl migrate k8s-discover -n production

# 2. Create a migration plan
chronosctl migrate k8s-plan -n production -f plan.yaml

# 3. Review the plan
cat plan.yaml

# 4. Dry-run to preview
chronosctl migrate k8s-apply -f plan.yaml --dry-run

# 5. Apply the migration
chronosctl migrate k8s-apply -f plan.yaml
```

### Scripting with JSON Output

```bash
# Get all failed executions for a job
chronosctl execution list daily-report -o json | \
  jq '.[] | select(.status == "failed")'

# List all enabled job names
chronosctl job list -o json | jq -r '.[] | select(.enabled) | .name'

# Check cluster health in a script
if chronosctl cluster status -o json | jq -e '.is_leader' > /dev/null; then
  echo "This node is the leader"
fi
```

---

## Troubleshooting

### Connection Refused

```
Error: failed to connect to server: connection refused
```

**Solutions:**
- Verify Chronos server is running: `curl http://localhost:8080/health`
- Check the server URL: `chronosctl --server http://correct-host:8080 job list`
- Ensure no firewall is blocking the connection

### Job Not Found

```
Error: NOT_FOUND: job not found
```

**Solutions:**
- Verify the job ID or name: `chronosctl job list`
- Use the full UUID or at least 8 characters of the short ID

### Invalid Configuration File

```
Error: failed to parse YAML: ...
```

**Solutions:**
- Validate YAML syntax with a linter: `yamllint job.yaml`
- Check required fields: `name`, `schedule`, `webhook.url`
- Ensure proper YAML indentation (spaces, not tabs)

---

## See Also

- [API Reference](/docs/reference/api) — REST API documentation
- [Configuration Reference](/docs/reference/configuration) — Server configuration options
- [Troubleshooting Guide](/docs/resources/troubleshooting) — Common issues and solutions
