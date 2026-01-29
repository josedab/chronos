---
sidebar_position: 3
title: Configuration
description: Complete configuration reference for chronos.yaml
---

# Configuration Reference

Complete reference for `chronos.yaml` configuration options.

## Example Configuration

```yaml
cluster:
  node_id: chronos-1
  data_dir: /var/lib/chronos
  raft:
    address: 0.0.0.0:7000
    peers: []
    heartbeat_timeout: 500ms
    election_timeout: 1s
    snapshot_interval: 30s
    snapshot_threshold: 1000

server:
  http:
    address: 0.0.0.0:8080
    read_timeout: 30s
    write_timeout: 30s
    tls:
      enabled: false
      cert_file: ""
      key_file: ""

scheduler:
  tick_interval: 1s
  execution_timeout: 5m
  missed_run_policy: execute_one

dispatcher:
  http:
    timeout: 30s
    max_idle_conns: 100

metrics:
  prometheus:
    enabled: true
    path: /metrics

logging:
  level: info
  format: json
```

## Environment Variables

All config options can be set via environment variables:

```bash
CHRONOS_CLUSTER_NODE_ID=chronos-1
CHRONOS_SERVER_HTTP_ADDRESS=0.0.0.0:8080
CHRONOS_LOGGING_LEVEL=debug
```
