---
sidebar_position: 1
title: Quick Start
description: Get Chronos running and create your first job in under 5 minutes
---

# Quick Start

Get Chronos running and schedule your first job in under 5 minutes.

## Installation

Choose your preferred installation method:

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

<Tabs>
<TabItem value="binary" label="Binary" default>

```bash
# Download and extract the latest release
curl -L https://github.com/chronos/chronos/releases/latest/download/chronos-linux-amd64.tar.gz | tar xz

# Start Chronos
./chronos --config chronos.yaml
```

</TabItem>
<TabItem value="docker" label="Docker">

```bash
# Run Chronos with persistent storage
docker run -d \
  --name chronos \
  -p 8080:8080 \
  -v chronos-data:/var/lib/chronos \
  chronos/chronos:latest
```

</TabItem>
<TabItem value="kubernetes" label="Kubernetes (Helm)">

```bash
# Add the Chronos Helm repository
helm repo add chronos https://chronos.github.io/charts

# Install Chronos
helm install chronos chronos/chronos
```

</TabItem>
<TabItem value="homebrew" label="Homebrew (macOS)">

```bash
# Install via Homebrew
brew install chronos/tap/chronos

# Start Chronos
chronos --config /usr/local/etc/chronos/chronos.yaml
```

</TabItem>
</Tabs>

## Verify Installation

Check that Chronos is running:

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{
  "status": "healthy",
  "leader": true
}
```

## Create Your First Job

Create a job that triggers a webhook every minute:

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hello-world",
    "schedule": "* * * * *",
    "webhook": {
      "url": "https://httpbin.org/post",
      "method": "POST",
      "body": "{\"message\": \"Hello from Chronos!\"}"
    },
    "enabled": true
  }'
```

You'll receive a response with the created job:

```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "hello-world",
    "schedule": "* * * * *",
    "enabled": true,
    "next_run": "2026-01-29T12:01:00Z"
  }
}
```

## View Job Executions

After a minute, check the execution history:

```bash
curl http://localhost:8080/api/v1/jobs/550e8400-e29b-41d4-a716-446655440000/executions
```

```json
{
  "success": true,
  "data": {
    "executions": [
      {
        "id": "exec-123",
        "status": "success",
        "status_code": 200,
        "started_at": "2026-01-29T12:01:00Z",
        "completed_at": "2026-01-29T12:01:01Z",
        "duration": 1000000000
      }
    ]
  }
}
```

## Using the CLI

Install the Chronos CLI for easier job management:

```bash
# List all jobs
chronosctl job list

# Get job details
chronosctl job get hello-world

# Trigger a job manually
chronosctl job trigger hello-world

# View recent executions
chronosctl execution list hello-world --limit 10
```

## Access the Web UI

Open your browser to [http://localhost:8080](http://localhost:8080) to access the Chronos Web UI, where you can:

- View all jobs and their status
- Monitor execution history
- Create and edit jobs visually
- See cluster health and metrics

## What's Next?

<div className="row">
  <div className="col col--6">
    <div className="card">
      <div className="card__header">
        <h3>📖 Core Concepts</h3>
      </div>
      <div className="card__body">
        <p>Learn how Chronos works under the hood—jobs, scheduling, webhooks, and clustering.</p>
      </div>
      <div className="card__footer">
        <a className="button button--primary button--block" href="/docs/core-concepts/architecture">
          Explore Architecture
        </a>
      </div>
    </div>
  </div>
  <div className="col col--6">
    <div className="card">
      <div className="card__header">
        <h3>🚀 Deployment Guides</h3>
      </div>
      <div className="card__body">
        <p>Deploy Chronos in production with high availability on Kubernetes or bare metal.</p>
      </div>
      <div className="card__footer">
        <a className="button button--primary button--block" href="/docs/guides/kubernetes">
          Deploy on Kubernetes
        </a>
      </div>
    </div>
  </div>
</div>

---

:::tip Need Help?
- [GitHub Discussions](https://github.com/chronos/chronos/discussions) - Ask questions and share ideas
- [Discord](https://discord.gg/chronos) - Real-time chat with the community
- [Troubleshooting Guide](/docs/resources/troubleshooting) - Common issues and solutions
:::
