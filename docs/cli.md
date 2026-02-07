# Chronos CLI Reference

`chronosctl` is the command-line interface for managing Chronos jobs, executions, and clusters.

## Installation

### From Binary

```bash
# Download the latest release
curl -L https://github.com/chronos/chronos/releases/latest/download/chronosctl-linux-amd64.tar.gz | tar xz
sudo mv chronosctl /usr/local/bin/
```

### From Source

```bash
go install github.com/chronos/chronos/cmd/chronosctl@latest
```

### Verify Installation

```bash
chronosctl --version
```

---

## Global Options

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--server` | `-s` | `http://localhost:8080` | Chronos server URL |
| `--output` | `-o` | `table` | Output format: `table`, `json`, `yaml` |
| `--help` | `-h` | | Show help for any command |
| `--version` | | | Show version information |

### Environment Variables

```bash
export CHRONOS_SERVER=http://chronos.example.com:8080
```

---

## Commands

### Job Management

#### `chronosctl job list`

List all jobs in the cluster.

```bash
chronosctl job list
chronosctl job list -o json
chronosctl job list -o yaml
```

**Example Output (table):**
```
Total: 3 jobs

ID        NAME           SCHEDULE      ENABLED  NEXT RUN
550e8400  daily-report   0 9 * * *     yes      2024-01-16 09:00:00
661f9511  hourly-sync    0 * * * *     yes      2024-01-15 16:00:00
772a0622  weekly-backup  0 0 * * 0     no       -
```

---

#### `chronosctl job get <job-id>`

Get detailed information about a specific job.

```bash
chronosctl job get 550e8400-e29b-41d4-a716-446655440000
chronosctl job get 550e8400  # Short ID also works
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
Next Run:    2024-01-16 09:00:00
```

---

#### `chronosctl job create -f <file>`

Create a new job from a YAML or JSON file.

```bash
chronosctl job create -f job.yaml
chronosctl job create -f job.json
```

**Example job.yaml:**
```yaml
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

**Output:**
```
Job created: daily-report (550e8400-e29b-41d4-a716-446655440000)
```

---

#### `chronosctl job delete <job-id>`

Delete a job and all its execution history.

```bash
chronosctl job delete 550e8400-e29b-41d4-a716-446655440000
```

**Output:**
```
Job deleted
```

---

#### `chronosctl job trigger <job-id>`

Manually trigger a job execution immediately.

```bash
chronosctl job trigger 550e8400-e29b-41d4-a716-446655440000
```

**Output:**
```
Job triggered: execution exec-772a0622-1234-5678-9abc-def012345678, status: success
```

---

#### `chronosctl job enable <job-id>`

Enable a disabled job.

```bash
chronosctl job enable 550e8400-e29b-41d4-a716-446655440000
```

**Output:**
```
Job enabled
```

---

#### `chronosctl job disable <job-id>`

Disable an enabled job (prevents scheduled executions).

```bash
chronosctl job disable 550e8400-e29b-41d4-a716-446655440000
```

**Output:**
```
Job disabled
```

---

### Execution Management

#### `chronosctl execution list <job-id>`

List execution history for a job (newest first).

```bash
chronosctl execution list 550e8400-e29b-41d4-a716-446655440000
chronosctl execution list 550e8400 -o json
```

**Example Output:**
```
Total: 5 executions

ID        STATUS   ATTEMPTS  STARTED              DURATION
exec-001  success  1         2024-01-15 09:00:01  1.234s
exec-002  success  1         2024-01-14 09:00:00  1.156s
exec-003  failed   3         2024-01-13 09:00:00  45.678s
exec-004  success  2         2024-01-12 09:00:00  2.345s
exec-005  success  1         2024-01-11 09:00:00  1.089s
```

---

#### `chronosctl execution get <job-id> <execution-id>`

Get detailed information about a specific execution.

```bash
chronosctl execution get 550e8400 exec-001
```

**Example Output:**
```
ID:         exec-001-e29b-41d4-a716-446655440000
Job ID:     550e8400-e29b-41d4-a716-446655440000
Status:     success
Attempts:   1
Started:    2024-01-15 09:00:01
Response:   {"status": "ok", "records_processed": 1234}
```

---

### Cluster Management

#### `chronosctl cluster status`

Get cluster status information.

```bash
chronosctl cluster status
chronosctl cluster status -o json
```

**Example Output:**
```
Is Leader:  true
Jobs Total: 42
```

**JSON Output:**
```json
{
  "is_leader": true,
  "jobs_total": 42,
  "running": 2,
  "next_runs": [
    {
      "job_id": "550e8400-e29b-41d4-a716-446655440000",
      "next_run": "2024-01-15T15:00:00Z"
    }
  ]
}
```

---

### Migration Commands

Chronos supports migrating jobs from external schedulers.

#### `chronosctl migrate k8s-discover`

Discover Kubernetes CronJobs in a cluster.

```bash
# Discover all CronJobs
chronosctl migrate k8s-discover

# Filter by namespace
chronosctl migrate k8s-discover -n production

# Filter by label selector
chronosctl migrate k8s-discover -l app=myapp

# Use specific kubeconfig
chronosctl migrate k8s-discover --kubeconfig ~/.kube/prod-config --context prod-cluster

# Include suspended CronJobs
chronosctl migrate k8s-discover --include-disabled
```

**Options:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--kubeconfig` | | `~/.kube/config` | Path to kubeconfig file |
| `--context` | | | Kubernetes context to use |
| `--namespace` | `-n` | all | Filter by namespace |
| `--selector` | `-l` | | Label selector (e.g., `app=myapp`) |
| `--include-disabled` | | `false` | Include suspended CronJobs |

**Example Output:**
```
Discovered 5 CronJobs across 2 namespaces

NAMESPACE    NAME              SCHEDULE      IMAGE                           SUSPENDED  WARNINGS
production   daily-backup      0 2 * * *     backup:latest                   no         0
production   hourly-cleanup    0 * * * *     cleanup:v1.2.3                  no         1
staging      test-job          */5 * * * *   test:latest                     no         0
staging      weekly-report     0 0 * * 0     report:v2.0.0                   yes        0
default      legacy-cron       0 6 * * *     legacy-app:old                  no         2
```

---

#### `chronosctl migrate k8s-plan`

Create a migration plan for Kubernetes CronJobs (dry-run).

```bash
# Create a plan
chronosctl migrate k8s-plan -n production

# Save plan to file
chronosctl migrate k8s-plan -n production -f migration-plan.yaml

# Specify target namespace and adapter URL
chronosctl migrate k8s-plan \
  -n production \
  --target-namespace prod-jobs \
  --adapter-url http://chronos-k8s-adapter:8081
```

**Options:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--kubeconfig` | | `~/.kube/config` | Path to kubeconfig file |
| `--context` | | | Kubernetes context to use |
| `--namespace` | `-n` | all | Filter by namespace |
| `--selector` | `-l` | | Label selector |
| `--target-namespace` | | | Target Chronos namespace |
| `--adapter-url` | | `http://chronos-k8s-adapter:8081` | Webhook adapter base URL |
| `--output-file` | `-f` | | Write plan to file (YAML) |

**Example Output:**
```
Migration Plan: k8s-migration-2024-01-15
Source Cluster: prod-cluster
Target Namespace: prod-jobs
Total Jobs: 3

SOURCE                    TARGET NAME            ACTION  WARNINGS
production/daily-backup   prod-daily-backup      create  0
production/hourly-cleanup prod-hourly-cleanup    create  1
production/weekly-report  prod-weekly-report     create  0
```

---

#### `chronosctl migrate k8s-apply`

Apply a migration plan to import CronJobs.

```bash
# Apply a migration plan
chronosctl migrate k8s-apply -f migration-plan.yaml

# Preview changes without applying
chronosctl migrate k8s-apply -f migration-plan.yaml --dry-run
```

**Options:**

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--file` | `-f` | Yes | Migration plan file to apply |
| `--dry-run` | | No | Preview changes without applying |

**Example Output:**
```
Migration Results:
  Created: 3
  Updated: 0
  Skipped: 0
  Failed:  0
```

---

#### `chronosctl migrate from-file`

Import jobs from various external scheduler formats.

```bash
# Migrate from Kubernetes CronJob YAML
chronosctl migrate from-file -f cronjob.yaml --source-type kubernetes

# Migrate from Airflow DAG
chronosctl migrate from-file -f dag.py --source-type airflow

# Migrate from AWS EventBridge
chronosctl migrate from-file -f rule.json --source-type eventbridge

# Migrate from Temporal
chronosctl migrate from-file -f workflow.json --source-type temporal

# Migrate from GitHub Actions
chronosctl migrate from-file -f workflow.yml --source-type github-actions

# Preview only
chronosctl migrate from-file -f cronjob.yaml --source-type kubernetes --dry-run
```

**Supported Source Types:**

| Type | Description |
|------|-------------|
| `kubernetes` | Kubernetes CronJob YAML |
| `airflow` | Apache Airflow DAG definition |
| `eventbridge` | AWS EventBridge rule JSON |
| `temporal` | Temporal workflow JSON |
| `github-actions` | GitHub Actions workflow YAML |

---

## Output Formats

### Table (default)

Human-readable tabular format, suitable for terminal viewing.

```bash
chronosctl job list
```

### JSON

Machine-readable JSON format, suitable for scripting and automation.

```bash
chronosctl job list -o json
chronosctl job get 550e8400 -o json | jq '.name'
```

### YAML

Human-readable structured format, useful for configuration management.

```bash
chronosctl job list -o yaml
chronosctl job get 550e8400 -o yaml > job-backup.yaml
```

---

## Common Workflows

### Create and Enable a Job

```bash
# Create job from YAML
chronosctl job create -f daily-report.yaml

# Verify it was created
chronosctl job list

# Enable the job
chronosctl job enable <job-id>
```

### Test a Job Before Scheduling

```bash
# Create job (disabled by default if not specified)
chronosctl job create -f job.yaml

# Trigger manually to test
chronosctl job trigger <job-id>

# Check execution result
chronosctl execution list <job-id>

# If successful, enable for scheduling
chronosctl job enable <job-id>
```

### Export Jobs for Backup

```bash
# Export all jobs
chronosctl job list -o yaml > jobs-backup.yaml

# Export specific job
chronosctl job get <job-id> -o yaml > job-backup.yaml
```

### Migrate from Kubernetes CronJobs

```bash
# 1. Discover existing CronJobs
chronosctl migrate k8s-discover -n production

# 2. Create migration plan
chronosctl migrate k8s-plan -n production -f plan.yaml

# 3. Review the plan
cat plan.yaml

# 4. Apply the plan (dry-run first)
chronosctl migrate k8s-apply -f plan.yaml --dry-run

# 5. Apply for real
chronosctl migrate k8s-apply -f plan.yaml
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
- Ensure no firewall blocking the connection

### Job Not Found

```
Error: NOT_FOUND: job not found
```

**Solutions:**
- Verify the job ID: `chronosctl job list`
- Use the full UUID or at least 8 characters of the ID

### Invalid Configuration File

```
Error: failed to parse YAML: ...
```

**Solutions:**
- Validate YAML syntax: `yamllint job.yaml`
- Check required fields: `name`, `schedule`, `webhook.url`
- Ensure proper indentation

---

## See Also

- [API Reference](api.md) - REST API documentation
- [Configuration](../README.md#configuration) - Server configuration
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
