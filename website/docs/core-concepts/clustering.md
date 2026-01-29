---
sidebar_position: 5
title: Clustering
description: High availability clustering with Raft consensus in Chronos
---

# Clustering

Chronos uses the Raft consensus algorithm to provide high availability and fault tolerance.

## Overview

A Chronos cluster consists of multiple nodes that:
- Share job definitions and execution state
- Automatically elect a leader
- Tolerate node failures without data loss

```
┌─────────────────────────────────────────────────┐
│                 Load Balancer                    │
└─────────────────────────────────────────────────┘
           │           │           │
           ▼           ▼           ▼
     ┌─────────┐ ┌─────────┐ ┌─────────┐
     │ Node 1  │ │ Node 2  │ │ Node 3  │
     │ Leader  │ │Follower │ │Follower │
     └─────────┘ └─────────┘ └─────────┘
           │           │           │
           └───────────┴───────────┘
                   Raft
```

## Cluster Sizing

| Nodes | Failures Tolerated | Use Case |
|-------|-------------------|----------|
| 1 | 0 | Development only |
| 3 | 1 | Standard production |
| 5 | 2 | High availability |
| 7 | 3 | Critical workloads |

:::tip Always Use Odd Numbers
Raft requires a majority (quorum) to operate. Odd numbers prevent split-brain scenarios:
- 3 nodes: quorum = 2, tolerates 1 failure
- 5 nodes: quorum = 3, tolerates 2 failures
:::

## Configuration

### Node 1 (Initial Leader)

```yaml title="chronos-1.yaml"
cluster:
  node_id: chronos-1
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.0.1:7000
    peers: []  # Empty for first node
    bootstrap: true

server:
  http:
    address: 0.0.0.0:8080
```

### Node 2 (Follower)

```yaml title="chronos-2.yaml"
cluster:
  node_id: chronos-2
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.0.2:7000
    peers:
      - 10.0.0.1:7000
    bootstrap: false

server:
  http:
    address: 0.0.0.0:8080
```

### Node 3 (Follower)

```yaml title="chronos-3.yaml"
cluster:
  node_id: chronos-3
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.0.3:7000
    peers:
      - 10.0.0.1:7000
      - 10.0.0.2:7000
    bootstrap: false

server:
  http:
    address: 0.0.0.0:8080
```

## Starting the Cluster

### Bootstrap Order

1. Start Node 1 first (bootstrap node)
2. Wait for it to become leader
3. Start Nodes 2 and 3

```bash
# Node 1
./chronos --config chronos-1.yaml

# Wait for "became leader" in logs, then:

# Node 2
./chronos --config chronos-2.yaml

# Node 3
./chronos --config chronos-3.yaml
```

### Verify Cluster Status

```bash
curl http://10.0.0.1:8080/api/v1/cluster/status
```

```json
{
  "node_id": "chronos-1",
  "is_leader": true,
  "leader_address": "10.0.0.1:7000",
  "peers": [
    {"id": "chronos-1", "address": "10.0.0.1:7000", "state": "Leader"},
    {"id": "chronos-2", "address": "10.0.0.2:7000", "state": "Follower"},
    {"id": "chronos-3", "address": "10.0.0.3:7000", "state": "Follower"}
  ],
  "commit_index": 1234,
  "applied_index": 1234
}
```

## Leader Election

### How It Works

1. Nodes start as followers
2. If no heartbeat from leader, follower starts election
3. Candidate requests votes from peers
4. First to get majority becomes leader
5. Leader sends periodic heartbeats

### Election Timeouts

```yaml
cluster:
  raft:
    heartbeat_timeout: 500ms   # Leader heartbeat interval
    election_timeout: 1000ms   # Time before starting election
```

Typical failover time: **~2-5 seconds**

### Manual Failover

Force a leader change (for maintenance):

```bash
curl -X POST http://10.0.0.1:8080/api/v1/cluster/transfer-leadership
```

## Read vs Write Operations

### Write Operations

All writes go through the leader:
- Create/Update/Delete jobs
- Record executions
- Update schedules

```
Client → Any Node → Forward to Leader → Raft Consensus → Apply
```

### Read Operations

Reads can be served by any node:
- List jobs
- Get job details
- View executions

```
Client → Any Node → Read from Local Store
```

For strongly consistent reads:

```bash
curl "http://localhost:8080/api/v1/jobs?consistent=true"
```

This forwards the read to the leader.

## Adding Nodes

### Add a New Node

```bash
# 1. Start the new node
./chronos --config chronos-4.yaml

# 2. Add to cluster (from any existing node)
curl -X POST http://10.0.0.1:8080/api/v1/cluster/join \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "chronos-4",
    "address": "10.0.0.4:7000"
  }'
```

### Remove a Node

```bash
curl -X DELETE http://10.0.0.1:8080/api/v1/cluster/nodes/chronos-4
```

## Network Partitions

### Partition Handling

If the cluster splits:
- Side with majority continues operating
- Minority side becomes read-only
- When healed, minority rejoins and syncs

```
Normal:          [L] [F] [F]  (Leader + 2 Followers)

Partition:       [L] [F] | [F]
                 ↓
                 Majority continues
                 Minority goes read-only

Healed:          [L] [F] [F]
                 ↓
                 Minority syncs from majority
```

### Split-Brain Prevention

Raft's quorum requirement prevents split-brain:
- 3 nodes: need 2 to write (one side can't get quorum)
- 5 nodes: need 3 to write

## Monitoring Cluster Health

### Key Metrics

| Metric | Description |
|--------|-------------|
| `chronos_raft_is_leader` | 1 if this node is leader |
| `chronos_raft_peers` | Number of cluster peers |
| `chronos_raft_commit_index` | Raft commit index |
| `chronos_raft_applied_index` | Applied index |
| `chronos_raft_last_contact` | Time since last leader contact |

### Health Checks

```bash
# Check if node is healthy
curl http://localhost:8080/health

# Response varies by role:
# Leader: {"status": "healthy", "leader": true}
# Follower: {"status": "healthy", "leader": false, "leader_address": "10.0.0.1:7000"}
```

### Alerting Rules

```yaml
# Prometheus alerting rules
groups:
  - name: chronos-cluster
    rules:
      - alert: ChronosNoLeader
        expr: sum(chronos_raft_is_leader) == 0
        for: 30s
        labels:
          severity: critical
        annotations:
          summary: "No Chronos leader elected"

      - alert: ChronosClusterDegraded
        expr: chronos_raft_peers < 3
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Chronos cluster has fewer than 3 nodes"
```

## Disaster Recovery

### Backup

Snapshots are created automatically. Manual snapshot:

```bash
curl -X POST http://localhost:8080/api/v1/cluster/snapshot
```

### Restore

To restore from snapshot:

1. Stop all nodes
2. Copy snapshot to data directory
3. Start bootstrap node with `--restore-snapshot`

```bash
./chronos --config chronos-1.yaml --restore-snapshot /backup/snapshot-123
```

### Full Cluster Restore

If all nodes lost:

```bash
# Start fresh cluster
./chronos --config chronos-1.yaml --bootstrap

# Restore jobs from backup
chronosctl job import -f jobs-backup.yaml
```

## Best Practices

### Deployment

- Place nodes in different availability zones
- Use dedicated storage (SSD recommended)
- Ensure low-latency network between nodes (under 10ms RTT)

### Sizing

| Cluster Size | Memory per Node | Storage |
|--------------|-----------------|---------|
| Small (under 100 jobs) | 512 MB | 1 GB |
| Medium (100-1000 jobs) | 2 GB | 10 GB |
| Large (1000+ jobs) | 4 GB | 50 GB |

### Security

- Use TLS between nodes
- Firewall Raft port (7000) to cluster only
- Use authentication for HTTP API

```yaml
cluster:
  raft:
    tls:
      cert_file: /etc/chronos/server.crt
      key_file: /etc/chronos/server.key
      ca_file: /etc/chronos/ca.crt
```

## Next Steps

- [Kubernetes Deployment](/docs/guides/kubernetes)
- [High Availability Guide](/docs/guides/high-availability)
- [Monitoring Setup](/docs/guides/monitoring)
