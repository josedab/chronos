---
sidebar_position: 7
title: GitOps
description: Manage Chronos jobs with Git-based configuration
---

# GitOps

Manage Chronos job configurations through Git repositories for version-controlled, auditable, and reviewable infrastructure-as-code workflows.

## Overview

GitOps brings the benefits of version control to your job scheduling infrastructure:

- **Version history**: Track who changed what and when
- **Pull request reviews**: Require approval before job changes go live
- **Rollback capability**: Revert to any previous configuration
- **Consistency**: Ensure all environments match your Git repository
- **Audit trail**: Complete history for compliance requirements

## Prerequisites

Before setting up GitOps:

1. A Git repository to store job configurations
2. Network access from Chronos to your Git provider
3. Authentication credentials (SSH key or token)

## Configuration

### Basic Setup

Enable GitOps in your Chronos configuration:

```yaml
# chronos.yaml
gitops:
  enabled: true
  repository: https://github.com/org/chronos-jobs
  branch: main
  sync_interval: 60s
  path: jobs/  # Optional: subdirectory containing job files
```

### Authentication

#### HTTPS with Token

```yaml
gitops:
  enabled: true
  repository: https://github.com/org/chronos-jobs
  auth:
    type: token
    token_env: GITHUB_TOKEN  # Environment variable containing the token
```

#### SSH Key

```yaml
gitops:
  enabled: true
  repository: git@github.com:org/chronos-jobs.git
  auth:
    type: ssh
    private_key_path: /etc/chronos/ssh/id_ed25519
    # Or use environment variable
    private_key_env: GIT_SSH_KEY
```

### Advanced Options

```yaml
gitops:
  enabled: true
  repository: https://github.com/org/chronos-jobs
  branch: main
  sync_interval: 60s
  
  # Sync behavior
  delete_orphans: true      # Remove jobs not in Git (default: false)
  dry_run: false            # Preview changes without applying (default: false)
  
  # File handling
  path: jobs/production/
  file_patterns:
    - "*.yaml"
    - "*.yml"
  exclude_patterns:
    - "*_test.yaml"
    - "templates/*"
  
  # Conflict resolution
  conflict_strategy: git-wins  # Options: git-wins, chronos-wins, fail
  
  # Webhook for instant sync (optional)
  webhook:
    enabled: true
    secret_env: WEBHOOK_SECRET
```

## Repository Structure

Organize your job definitions in a clear directory structure:

```
chronos-jobs/
├── README.md
├── production/
│   ├── data-pipeline/
│   │   ├── etl-daily.yaml
│   │   ├── reports.yaml
│   │   └── cleanup.yaml
│   ├── maintenance/
│   │   ├── backups.yaml
│   │   └── log-rotation.yaml
│   └── notifications/
│       └── digest-emails.yaml
├── staging/
│   └── ...
└── templates/
    └── job-template.yaml  # Not synced (excluded)
```

## Job Definition Format

### Single Job per File

```yaml
# production/data-pipeline/etl-daily.yaml
name: etl-daily
description: Daily ETL pipeline for analytics
schedule: "0 2 * * *"
timezone: UTC
enabled: true

command: |
  /opt/scripts/run-etl.sh \
    --date $(date -d "yesterday" +%Y-%m-%d) \
    --output s3://data-lake/processed/

retry:
  attempts: 3
  delay: 5m
  exponential: true

metadata:
  owner: data-team
  slack_channel: "#data-alerts"
```

### Multiple Jobs per File

```yaml
# production/maintenance/database-jobs.yaml
jobs:
  - name: db-backup-hourly
    schedule: "0 * * * *"
    command: /scripts/backup.sh --incremental
    
  - name: db-backup-daily
    schedule: "0 0 * * *"
    command: /scripts/backup.sh --full
    
  - name: db-vacuum
    schedule: "0 4 * * 0"  # Weekly on Sunday
    command: /scripts/vacuum.sh
```

### Using Environment-Specific Values

```yaml
# production/api/health-checks.yaml
name: api-health-check
schedule: "*/5 * * * *"

# Environment variables from Chronos secrets
env:
  API_URL: "{{secrets.API_ENDPOINT}}"
  API_KEY: "{{secrets.API_KEY}}"

command: |
  curl -sf "${API_URL}/health" \
    -H "Authorization: Bearer ${API_KEY}"
```

## Sync Behavior

### Initial Sync

When GitOps is first enabled, Chronos:

1. Clones the repository
2. Parses all job files matching the patterns
3. Creates jobs that don't exist in Chronos
4. Updates jobs that differ from Git definitions
5. Optionally deletes orphaned jobs (if `delete_orphans: true`)

### Ongoing Sync

On each sync interval (or webhook trigger):

1. Pulls latest changes from the branch
2. Compares Git definitions with Chronos state
3. Applies changes (create/update/delete)
4. Logs all changes with commit reference

### Conflict Resolution Strategies

| Strategy | Behavior |
|----------|----------|
| `git-wins` | Git definitions always override Chronos (recommended) |
| `chronos-wins` | Manual changes in Chronos are preserved |
| `fail` | Sync fails if conflicts exist, requires manual resolution |

## Webhook Integration

Enable instant sync on push for faster feedback:

### GitHub Webhook Setup

1. Go to Repository → Settings → Webhooks → Add webhook
2. Configure:
   - **Payload URL**: `https://chronos.example.com/api/v1/gitops/webhook`
   - **Content type**: `application/json`
   - **Secret**: Generate and save in Chronos config
   - **Events**: Just the push event

### GitLab Webhook Setup

1. Go to Settings → Webhooks
2. Configure:
   - **URL**: `https://chronos.example.com/api/v1/gitops/webhook`
   - **Secret token**: Your webhook secret
   - **Trigger**: Push events

### Webhook Handler Response

```json
{
  "status": "synced",
  "commit": "abc123def",
  "changes": {
    "created": ["job-1", "job-2"],
    "updated": ["job-3"],
    "deleted": []
  },
  "duration_ms": 1250
}
```

## CI/CD Integration

### GitHub Actions Workflow

```yaml
# .github/workflows/chronos-validate.yml
name: Validate Chronos Jobs

on:
  pull_request:
    paths:
      - 'jobs/**'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install chronosctl
        run: |
          curl -sSL https://chronos.example.com/install.sh | bash
      
      - name: Validate job definitions
        run: |
          chronosctl gitops validate --path jobs/
      
      - name: Dry-run sync
        env:
          CHRONOS_API_URL: ${{ secrets.CHRONOS_STAGING_URL }}
          CHRONOS_API_KEY: ${{ secrets.CHRONOS_API_KEY }}
        run: |
          chronosctl gitops sync --dry-run --path jobs/
```

### Pre-commit Hook

```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: validate-chronos-jobs
        name: Validate Chronos Jobs
        entry: chronosctl gitops validate --path jobs/
        language: system
        files: ^jobs/.*\.(yaml|yml)$
```

## Monitoring GitOps

### Prometheus Metrics

```
# Sync success/failure
chronos_gitops_sync_total{status="success|failure"}

# Sync duration
chronos_gitops_sync_duration_seconds

# Job changes
chronos_gitops_jobs_created_total
chronos_gitops_jobs_updated_total
chronos_gitops_jobs_deleted_total

# Repository status
chronos_gitops_last_sync_timestamp
chronos_gitops_commits_behind
```

### Alerting Rules

```yaml
groups:
  - name: gitops
    rules:
      - alert: GitOpsSyncFailing
        expr: increase(chronos_gitops_sync_total{status="failure"}[1h]) > 3
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: GitOps sync is failing repeatedly
          
      - alert: GitOpsSyncStale
        expr: time() - chronos_gitops_last_sync_timestamp > 600
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: GitOps hasn't synced in over 10 minutes
```

## Troubleshooting

### Common Issues

**Sync fails with authentication error**
```bash
# Check Git credentials
chronosctl gitops test-auth

# Verify token has correct permissions
# GitHub: repo scope required
# GitLab: read_repository scope required
```

**Jobs not appearing after sync**
```bash
# Check file patterns match
chronosctl gitops validate --path jobs/ --verbose

# Verify job YAML is valid
chronosctl job validate -f problematic-job.yaml
```

**Conflicts between Git and Chronos**
```bash
# View current differences
chronosctl gitops diff

# Force sync from Git (with git-wins strategy)
chronosctl gitops sync --force
```

### Debug Mode

Enable verbose logging:

```yaml
gitops:
  enabled: true
  debug: true  # Logs every file parsed and comparison made
```

### View Sync History

```bash
# Recent sync events
chronosctl gitops history --limit 10

# Details of a specific sync
chronosctl gitops history --sync-id abc123
```

## Best Practices

### Repository Organization

1. **One environment per branch** or **one directory per environment**
2. **Group related jobs** in subdirectories by team or function
3. **Use descriptive filenames** that match job names
4. **Include a README** documenting the structure and ownership

### Review Process

1. **Require PR reviews** for production job changes
2. **Run validation** in CI before merge
3. **Use dry-run** to preview changes
4. **Tag releases** for major configuration changes

### Security

1. **Never commit secrets** - use Chronos secret references
2. **Use deploy keys** with minimal permissions
3. **Rotate credentials** regularly
4. **Audit webhook access** periodically

### Recovery

1. **Keep `delete_orphans: false`** until confident in your setup
2. **Use `conflict_strategy: fail`** initially to catch issues
3. **Maintain manual override capability** for emergencies
4. **Back up Chronos state** before major GitOps changes
