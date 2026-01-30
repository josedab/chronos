# Chronos Kubernetes Operator

Kubernetes operator for managing Chronos distributed cron jobs using Custom Resource Definitions (CRDs).

## Features

- **ChronosJob**: Declarative job scheduling with Kubernetes-native YAML
- **ChronosDAG**: Workflow orchestration with job dependencies
- **ChronosTrigger**: Event-driven triggers (Kafka, SQS, Pub/Sub, webhooks)
- **GitOps Ready**: Manage jobs via Argo CD, Flux, or kubectl
- **Secret Integration**: Reference Kubernetes Secrets for credentials

## Installation

### Prerequisites

- Kubernetes 1.21+
- Helm 3.0+ (for Helm installation)
- Access to a Chronos server

### Install CRDs

```bash
kubectl apply -f config/crd/
```

### Install Operator

```bash
# Using Helm
helm install chronos-operator ./charts/chronos-operator \
  --set chronos.endpoint=http://chronos:8080

# Or using kubectl
kubectl apply -f config/manager/
```

## Usage

### Create a ChronosJob

```yaml
apiVersion: chronos.io/v1alpha1
kind: ChronosJob
metadata:
  name: daily-backup
spec:
  name: daily-backup
  schedule: "0 2 * * *"
  timezone: America/New_York
  webhook:
    url: https://api.example.com/backup
    method: POST
  timeout: "30m"
  concurrency: forbid
  retryPolicy:
    maxAttempts: 3
```

```bash
kubectl apply -f job.yaml
kubectl get chronosjobs
```

### Create a ChronosDAG

```yaml
apiVersion: chronos.io/v1alpha1
kind: ChronosDAG
metadata:
  name: etl-pipeline
spec:
  name: etl-pipeline
  schedule: "0 1 * * *"
  nodes:
    - id: extract
      jobRef:
        name: extract-job
    - id: transform
      jobRef:
        name: transform-job
      dependencies: [extract]
    - id: load
      jobRef:
        name: load-job
      dependencies: [transform]
```

### Create a ChronosTrigger

```yaml
apiVersion: chronos.io/v1alpha1
kind: ChronosTrigger
metadata:
  name: order-processor
spec:
  name: order-processor
  type: kafka
  jobRef:
    name: process-order
  kafka:
    brokers: [kafka:9092]
    topic: orders
    consumerGroup: chronos
```

## CRD Reference

### ChronosJob

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.name` | string | Yes | Human-readable job name |
| `spec.schedule` | string | Yes | Cron expression |
| `spec.timezone` | string | No | IANA timezone (default: UTC) |
| `spec.enabled` | bool | No | Enable/disable (default: true) |
| `spec.webhook` | object | Yes | Webhook configuration |
| `spec.timeout` | string | No | Execution timeout |
| `spec.concurrency` | string | No | allow, forbid, replace |
| `spec.retryPolicy` | object | No | Retry configuration |

### ChronosDAG

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.name` | string | Yes | DAG name |
| `spec.schedule` | string | No | Cron expression (optional) |
| `spec.nodes` | array | Yes | List of DAG nodes |
| `spec.nodes[].id` | string | Yes | Node identifier |
| `spec.nodes[].jobRef` | object | No | Reference to ChronosJob |
| `spec.nodes[].dependencies` | array | No | Dependent node IDs |
| `spec.nodes[].condition` | string | No | Trigger condition |

### ChronosTrigger

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `spec.name` | string | Yes | Trigger name |
| `spec.type` | string | Yes | kafka, sqs, pubsub, rabbitmq, webhook |
| `spec.jobRef` | object | Yes | Reference to ChronosJob |
| `spec.enabled` | bool | No | Enable/disable |
| `spec.kafka` | object | No | Kafka configuration |
| `spec.sqs` | object | No | SQS configuration |
| `spec.filter` | object | No | Event filtering |

## Development

```bash
# Run locally
make run

# Run tests
make test

# Build image
make docker-build IMG=chronos-operator:latest

# Deploy
make deploy IMG=chronos-operator:latest
```

## License

Apache 2.0
