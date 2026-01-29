---
sidebar_position: 1
title: Kubernetes
description: Deploy Chronos on Kubernetes with Helm
---

# Kubernetes Deployment

Deploy a production-ready Chronos cluster on Kubernetes using Helm.

## Prerequisites

- Kubernetes 1.21+
- Helm 3.8+
- kubectl configured

## Quick Start

```bash
# Add Helm repository
helm repo add chronos https://chronos.github.io/charts
helm repo update

# Install with default settings
helm install chronos chronos/chronos -n scheduling --create-namespace

# Verify deployment
kubectl get pods -n scheduling
```

## Production Configuration

```yaml title="values.yaml"
replicaCount: 3

image:
  repository: chronos/chronos
  tag: "1.0.0"
  pullPolicy: IfNotPresent

resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 2000m
    memory: 2Gi

persistence:
  enabled: true
  size: 10Gi
  storageClass: standard

service:
  type: ClusterIP
  port: 8080

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: chronos.example.com
      paths:
        - path: /
          pathType: Prefix

config:
  cluster:
    raft:
      heartbeat_timeout: 500ms
      election_timeout: 1s
  scheduler:
    tick_interval: 1s

podAntiAffinity:
  enabled: true
  type: hard

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
```

```bash
helm install chronos chronos/chronos -f values.yaml -n scheduling
```

## Next Steps

- [High Availability Guide](/docs/guides/high-availability)
- [Monitoring Setup](/docs/guides/monitoring)
