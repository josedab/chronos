---
sidebar_position: 5
title: Workflows
description: Create DAG-based workflows in Chronos
---

# Workflows

Orchestrate complex job dependencies with DAG-based workflows for multi-step data pipelines and coordinated tasks.

:::info Experimental Feature
Workflows are currently an experimental feature. Enable with `--enable-workflows` flag.
:::

## Overview

While individual jobs handle single scheduled tasks, **workflows** allow you to:

- Define dependencies between jobs
- Execute jobs in sequence or parallel
- Handle conditional logic and branching
- Implement retry and error handling at the workflow level
- Visualize complex pipelines

## Basic Concepts

### DAG (Directed Acyclic Graph)

Workflows are represented as DAGs where:
- **Nodes** are jobs or control operations
- **Edges** define execution order
- **No cycles** allowed (prevents infinite loops)

```
    ┌─────────┐
    │ Extract │
    └────┬────┘
         │
    ┌────▼────┐
    │Transform│
    └────┬────┘
         │
    ┌────▼────┐
    │  Load   │
    └─────────┘
```

## Creating a Workflow

### YAML Definition

```yaml title="data-pipeline.yaml"
name: daily-etl-pipeline
description: Extract, transform, and load daily data
schedule: "0 2 * * *"
timezone: America/New_York

nodes:
  # Step 1: Extract data from source
  - id: extract
    type: job
    job:
      webhook:
        url: https://api.example.com/extract
        method: POST
        body: |
          {"date": "{{ .ScheduledTime | date \"2006-01-02\" }}"}
      timeout: 30m
      retry_policy:
        max_attempts: 3
        
  # Step 2: Transform data
  - id: transform
    type: job
    depends_on: [extract]
    job:
      webhook:
        url: https://api.example.com/transform
        method: POST
      timeout: 1h
      
  # Step 3: Load to data warehouse
  - id: load
    type: job
    depends_on: [transform]
    job:
      webhook:
        url: https://api.example.com/load
        method: POST
      timeout: 2h
      
  # Step 4: Send notification
  - id: notify
    type: job
    depends_on: [load]
    job:
      webhook:
        url: https://slack.com/api/chat.postMessage
        method: POST
        headers:
          Authorization: "Bearer {{ .Secrets.SLACK_TOKEN }}"
        body: |
          {"channel": "#data-team", "text": "ETL completed successfully"}
```

### Create via CLI

```bash
chronosctl workflow create -f data-pipeline.yaml
```

### Create via API

```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d @data-pipeline.json
```

## Workflow Node Types

### Job Node

Execute a webhook-based job:

```yaml
- id: my-job
  type: job
  job:
    webhook:
      url: https://api.example.com/endpoint
      method: POST
    timeout: 5m
```

### Parallel Node

Execute multiple branches concurrently:

```yaml
- id: parallel-tasks
  type: parallel
  depends_on: [extract]
  branches:
    - id: process-a
      job:
        webhook:
          url: https://api.example.com/process-a
    - id: process-b
      job:
        webhook:
          url: https://api.example.com/process-b
    - id: process-c
      job:
        webhook:
          url: https://api.example.com/process-c
```

```
      ┌─────────┐
      │ Extract │
      └────┬────┘
           │
    ┌──────┼──────┐
    ▼      ▼      ▼
  ┌───┐  ┌───┐  ┌───┐
  │ A │  │ B │  │ C │
  └─┬─┘  └─┬─┘  └─┬─┘
    └──────┼──────┘
           │
      ┌────▼────┐
      │  Join   │
      └─────────┘
```

### Condition Node

Conditional branching based on previous job output:

```yaml
- id: check-result
  type: condition
  depends_on: [validate]
  conditions:
    - expression: "{{ .PreviousOutput.valid }} == true"
      next: process-valid
    - expression: "{{ .PreviousOutput.valid }} == false"
      next: handle-invalid
  default: handle-unknown
```

### Delay Node

Pause execution for a duration:

```yaml
- id: wait-for-sync
  type: delay
  depends_on: [trigger-sync]
  duration: 5m
```

### Approval Node

Wait for manual approval:

```yaml
- id: approval-gate
  type: approval
  depends_on: [deploy-staging]
  config:
    approvers: ["admin@example.com"]
    timeout: 24h
    notification:
      channel: "#deployments"
      message: "Production deployment awaiting approval"
```

## Dependency Patterns

### Sequential

Jobs run one after another:

```yaml
nodes:
  - id: step-1
    type: job
    # ...
  - id: step-2
    depends_on: [step-1]
    # ...
  - id: step-3
    depends_on: [step-2]
    # ...
```

### Fan-out / Fan-in

Parallel execution with synchronization:

```yaml
nodes:
  - id: prepare
    type: job
    # ...
    
  - id: worker-1
    depends_on: [prepare]
    # ...
  - id: worker-2
    depends_on: [prepare]
    # ...
  - id: worker-3
    depends_on: [prepare]
    # ...
    
  - id: aggregate
    depends_on: [worker-1, worker-2, worker-3]
    # ...
```

### Conditional Paths

Different paths based on conditions:

```yaml
nodes:
  - id: check
    type: job
    # ...
    
  - id: router
    type: condition
    depends_on: [check]
    conditions:
      - expression: "{{ .PreviousOutput.size }} > 1000"
        next: large-batch
      - expression: "{{ .PreviousOutput.size }} <= 1000"
        next: small-batch
        
  - id: large-batch
    depends_on: [router]
    condition_match: true
    # ...
    
  - id: small-batch
    depends_on: [router]
    condition_match: true
    # ...
```

## Error Handling

### Retry at Node Level

```yaml
- id: flaky-api
  type: job
  job:
    webhook:
      url: https://flaky-api.example.com/endpoint
    retry_policy:
      max_attempts: 5
      initial_interval: 10s
      max_interval: 5m
      multiplier: 2.0
```

### Workflow-Level Error Handling

```yaml
name: resilient-pipeline
on_failure:
  strategy: continue  # or: abort, retry
  notification:
    webhook:
      url: https://slack.com/api/chat.postMessage
      body: |
        {"text": "Workflow {{ .WorkflowName }} failed at {{ .FailedNode }}"}
```

### Error Nodes

Handle specific failure scenarios:

```yaml
nodes:
  - id: risky-operation
    type: job
    # ...
    on_error: handle-error
    
  - id: handle-error
    type: job
    job:
      webhook:
        url: https://api.example.com/cleanup
```

## Templating

Use Go templates for dynamic values:

### Available Variables

| Variable | Description |
|----------|-------------|
| `{{ .WorkflowName }}` | Workflow name |
| `{{ .WorkflowID }}` | Workflow instance ID |
| `{{ .NodeID }}` | Current node ID |
| `{{ .ScheduledTime }}` | Scheduled execution time |
| `{{ .StartTime }}` | Actual start time |
| `{{ .PreviousOutput }}` | Output from previous node |
| `{{ .Secrets.<name> }}` | Secret value |
| `{{ .Env.<name> }}` | Environment variable |

### Example

```yaml
- id: report
  type: job
  job:
    webhook:
      url: https://api.example.com/report
      body: |
        {
          "workflow": "{{ .WorkflowName }}",
          "date": "{{ .ScheduledTime | date \"2006-01-02\" }}",
          "previous_status": "{{ .PreviousOutput.status }}"
        }
```

## Managing Workflows

### List Workflows

```bash
chronosctl workflow list

# Output:
# NAME                STATUS    LAST RUN              NEXT RUN
# daily-etl-pipeline  enabled   2026-01-29T02:00:00Z  2026-01-30T02:00:00Z
# weekly-reports      enabled   2026-01-27T09:00:00Z  2026-02-03T09:00:00Z
```

### View Workflow Details

```bash
chronosctl workflow get daily-etl-pipeline
```

### Trigger Manual Run

```bash
chronosctl workflow trigger daily-etl-pipeline
```

### View Run History

```bash
chronosctl workflow runs daily-etl-pipeline --limit 10
```

### View Run Details

```bash
chronosctl workflow run-get daily-etl-pipeline run-abc123
```

## Monitoring Workflows

### Metrics

| Metric | Description |
|--------|-------------|
| `chronos_workflow_runs_total` | Total workflow runs by status |
| `chronos_workflow_duration_seconds` | Workflow execution duration |
| `chronos_workflow_node_duration_seconds` | Per-node execution duration |
| `chronos_workflow_active_runs` | Currently running workflows |

### Visualization

The Web UI provides:
- Visual DAG representation
- Real-time execution status
- Node-by-node timing breakdown
- Error highlighting

## Examples

### ETL Pipeline

```yaml
name: etl-pipeline
schedule: "0 3 * * *"

nodes:
  - id: extract-sales
    type: job
    job:
      webhook:
        url: https://data.example.com/extract/sales
        
  - id: extract-inventory
    type: job
    job:
      webhook:
        url: https://data.example.com/extract/inventory
        
  - id: transform
    type: job
    depends_on: [extract-sales, extract-inventory]
    job:
      webhook:
        url: https://data.example.com/transform
        
  - id: load
    type: job
    depends_on: [transform]
    job:
      webhook:
        url: https://warehouse.example.com/load
```

### Deployment Pipeline

```yaml
name: deploy-production
# Triggered manually or via CI

nodes:
  - id: run-tests
    type: job
    job:
      webhook:
        url: https://ci.example.com/run-tests
        
  - id: deploy-staging
    type: job
    depends_on: [run-tests]
    job:
      webhook:
        url: https://deploy.example.com/staging
        
  - id: smoke-test
    type: job
    depends_on: [deploy-staging]
    job:
      webhook:
        url: https://ci.example.com/smoke-test
        
  - id: approval
    type: approval
    depends_on: [smoke-test]
    config:
      approvers: ["team-leads"]
      timeout: 4h
      
  - id: deploy-production
    type: job
    depends_on: [approval]
    job:
      webhook:
        url: https://deploy.example.com/production
```

## Limitations

Current experimental limitations:
- Maximum 100 nodes per workflow
- Maximum 24-hour workflow timeout
- No sub-workflow nesting (yet)
- Limited to webhook-based jobs

## Next Steps

- [Visual Workflow Builder](/docs/advanced/policy-as-code) - Build workflows visually
- [API Reference](/docs/reference/api) - Workflow API endpoints
- [Monitoring](/docs/guides/monitoring) - Monitor workflow metrics
