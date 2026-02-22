---
sidebar_position: 10
title: Alerting
---

# Job Alerting & Notifications

Chronos includes a built-in alert engine for real-time failure notifications.

## Supported Channels

| Channel | Configuration |
|---------|--------------|
| **Slack** | Incoming Webhook URL |
| **PagerDuty** | Events API v2 routing key |
| **Microsoft Teams** | Incoming Webhook URL |
| **Discord** | Webhook URL |
| **Email** | SMTP server configuration |
| **Generic Webhook** | Any HTTP endpoint |
| **Opsgenie** | API key |

## Creating Alert Rules

### Via API

```bash
# Create a Slack channel
curl -X POST http://localhost:8080/api/v1/alerts/channels \
  -H "Content-Type: application/json" \
  -d '{
    "id": "slack-ops",
    "name": "Ops Slack",
    "type": "slack",
    "config": {
      "slack_webhook_url": "https://hooks.slack.com/services/xxx/yyy/zzz"
    },
    "enabled": true
  }'

# Create an alert rule
curl -X POST http://localhost:8080/api/v1/alerts/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-failures",
    "enabled": true,
    "events": ["job_failed"],
    "channel_ids": ["slack-ops"],
    "job_filter": "prod-*",
    "severity": "critical",
    "cooldown_mins": 15
  }'
```

### Via CLI

```bash
chronosctl alert test --channel slack-ops --event job_failed --job-name "test-job"
```

## Alert Rule Options

| Field | Description |
|-------|-------------|
| `events` | Event types to match: `job_failed`, `job_succeeded`, `dag_failed`, `quota_exceeded` |
| `channel_ids` | Notification channels to send to |
| `job_filter` | Glob pattern to match job names (e.g., `prod-*`, `*-backup`) |
| `severity` | Alert severity: `info`, `warning`, `error`, `critical` |
| `cooldown_mins` | Minimum minutes between alerts for the same job |

## Delivery History

```bash
curl http://localhost:8080/api/v1/alerts/history
```

Shows all notification delivery attempts with success/failure status.
