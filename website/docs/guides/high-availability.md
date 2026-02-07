---
sidebar_position: 2
title: High Availability
description: Configure Chronos for high availability production deployments
---

# High Availability

Deploy Chronos with fault tolerance and automatic failover for mission-critical job scheduling.

## Overview

A highly available Chronos deployment provides:
- **Automatic failover**: New leader elected in ~5 seconds
- **No single point of failure**: Any node can become leader
- **Data durability**: All data replicated across nodes
- **Zero downtime upgrades**: Rolling upgrades supported

## Requirements

### Minimum for Production

| Component | Requirement |
|-----------|-------------|
| Nodes | 3 (tolerates 1 failure) |
| CPU | 2 cores per node |
| Memory | 2GB per node |
| Storage | 10GB SSD per node |
| Network | &lt;10ms latency between nodes |

### For Critical Workloads

| Component | Requirement |
|-----------|-------------|
| Nodes | 5 (tolerates 2 failures) |
| CPU | 4 cores per node |
| Memory | 4GB per node |
| Storage | 50GB SSD per node |
| Network | &lt;5ms latency between nodes |

## Architecture

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    │  (API traffic)  │
                    └────────┬────────┘
                             │
           ┌─────────────────┼─────────────────┐
           │                 │                 │
           ▼                 ▼                 ▼
    ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
    │   Node 1    │   │   Node 2    │   │   Node 3    │
    │   (AZ-a)    │   │   (AZ-b)    │   │   (AZ-c)    │
    │             │   │             │   │             │
    │  ┌───────┐  │   │  ┌───────┐  │   │  ┌───────┐  │
    │  │Leader │  │   │  │Follow │  │   │  │Follow │  │
    │  └───────┘  │   │  └───────┘  │   │  └───────┘  │
    └──────┬──────┘   └──────┬──────┘   └──────┬──────┘
           │                 │                 │
           └────────Raft─────┴────────Raft─────┘
                   (port 7000)
```

## Configuration

### Node 1 (Initial Bootstrap)

```yaml title="/etc/chronos/chronos.yaml"
cluster:
  node_id: chronos-1
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.1.10:7000
    advertise: 10.0.1.10:7000

server:
  http:
    address: 0.0.0.0:8080

storage:
  path: /var/lib/chronos/data

logging:
  level: info
  format: json
```

### Node 2

```yaml title="/etc/chronos/chronos.yaml"
cluster:
  node_id: chronos-2
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.2.10:7000
    advertise: 10.0.2.10:7000
    peers:
      - 10.0.1.10:7000  # Node 1

server:
  http:
    address: 0.0.0.0:8080
```

### Node 3

```yaml title="/etc/chronos/chronos.yaml"
cluster:
  node_id: chronos-3
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.3.10:7000
    advertise: 10.0.3.10:7000
    peers:
      - 10.0.1.10:7000  # Node 1
      - 10.0.2.10:7000  # Node 2

server:
  http:
    address: 0.0.0.0:8080
```

## Deployment Steps

### 1. Provision Infrastructure

Distribute nodes across availability zones:

```bash
# AWS example
aws ec2 run-instances \
  --image-id ami-xxx \
  --instance-type t3.medium \
  --subnet-id subnet-az-a \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=chronos-1}]'

# Repeat for other AZs
```

### 2. Configure Network

Ensure nodes can communicate:

| Port | Protocol | Purpose |
|------|----------|---------|
| 7000 | TCP | Raft consensus (node-to-node) |
| 8080 | TCP | HTTP API |

**Security group rules:**
```bash
# Allow Raft between nodes
aws ec2 authorize-security-group-ingress \
  --group-id sg-xxx \
  --protocol tcp \
  --port 7000 \
  --source-group sg-xxx
```

### 3. Bootstrap the Cluster

**On Node 1 (first node only):**
```bash
# Start with bootstrap flag
chronos --config /etc/chronos/chronos.yaml --raft-bootstrap
```

**On Nodes 2 and 3:**
```bash
# Start normally - will join via configured peers
chronos --config /etc/chronos/chronos.yaml
```

### 4. Verify Cluster Health

```bash
# Check cluster status
curl http://10.0.1.10:8080/api/v1/cluster/status | jq

# Expected output:
{
  "success": true,
  "data": {
    "leader": "chronos-1",
    "state": "healthy",
    "nodes": [
      {"id": "chronos-1", "state": "leader"},
      {"id": "chronos-2", "state": "follower"},
      {"id": "chronos-3", "state": "follower"}
    ]
  }
}
```

### 5. Configure Load Balancer

**AWS ALB example:**
```yaml
resource "aws_lb_target_group" "chronos" {
  name        = "chronos"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = var.vpc_id
  target_type = "instance"

  health_check {
    enabled             = true
    path                = "/health"
    port                = "8080"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    interval            = 10
    timeout             = 5
  }
}
```

**NGINX example:**
```nginx
upstream chronos {
    server 10.0.1.10:8080 weight=5;
    server 10.0.2.10:8080 weight=5;
    server 10.0.3.10:8080 weight=5;
}

server {
    listen 443 ssl;
    server_name chronos.example.com;

    location / {
        proxy_pass http://chronos;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Tuning for HA

### Raft Timeouts

Balance between fast failover and stability:

```yaml
cluster:
  raft:
    # Faster failover (use with low-latency networks)
    heartbeat_timeout: 500ms
    election_timeout: 1s
    leader_lease_timeout: 400ms
    
    # Default (balanced)
    # heartbeat_timeout: 1s
    # election_timeout: 2s
    
    # Conservative (high-latency networks)
    # heartbeat_timeout: 2s
    # election_timeout: 5s
```

### Snapshot Configuration

```yaml
cluster:
  raft:
    snapshot_interval: 30s       # How often to check for snapshot
    snapshot_threshold: 8192     # Entries before snapshot
    retain_snapshots: 3          # Number to keep
```

### Storage Tuning

```yaml
storage:
  badger:
    sync_writes: true            # Durability (required for HA)
    value_log_gc_enabled: true   # Background cleanup
    gc_interval: 10m
```

## Operational Procedures

### Rolling Upgrade

1. **Upgrade followers first:**
   ```bash
   # On chronos-3
   systemctl stop chronos
   # Replace binary
   systemctl start chronos
   # Wait for rejoin
   curl http://localhost:8080/health
   
   # Repeat for chronos-2
   ```

2. **Upgrade leader last:**
   ```bash
   # On chronos-1 (current leader)
   systemctl stop chronos  # Triggers failover
   # Replace binary
   systemctl start chronos
   ```

### Adding a Node

```bash
# On new node
chronos --config /etc/chronos/chronos.yaml
# Node auto-joins via peer discovery
```

### Removing a Node

```bash
# From leader
chronosctl cluster remove-peer chronos-4
# Then stop the removed node
```

### Disaster Recovery

**If quorum is lost (majority of nodes down):**

1. **Identify surviving data:**
   ```bash
   ls -la /var/lib/chronos/
   ```

2. **Bootstrap from survivor:**
   ```bash
   chronos --config /etc/chronos/chronos.yaml --raft-bootstrap
   ```

3. **Rejoin other nodes as they recover**

## Monitoring HA

### Key Metrics

| Metric | Alert Threshold | Meaning |
|--------|-----------------|---------|
| `chronos_raft_is_leader` | sum < 1 for 1m | No leader |
| `chronos_raft_is_leader` | sum > 1 | Split brain |
| `chronos_raft_peers` | < 3 for 2m | Node lost |
| `chronos_raft_last_contact_seconds` | > 5 | Network issues |

### Alerting Rules

```yaml
groups:
  - name: chronos-ha
    rules:
      - alert: ChronosNoLeader
        expr: sum(chronos_raft_is_leader) == 0
        for: 1m
        labels:
          severity: critical
          
      - alert: ChronosQuorumAtRisk
        expr: chronos_raft_peers < 2
        for: 5m
        labels:
          severity: warning
```

## Best Practices

### Do ✅

- Use odd numbers of nodes (3, 5, 7)
- Spread nodes across availability zones
- Use SSDs for storage
- Monitor Raft metrics
- Test failover regularly
- Keep clocks synchronized (NTP)

### Don't ❌

- Run with even numbers of nodes
- Put all nodes in one AZ
- Ignore network latency
- Skip backup procedures
- Use unreliable storage

## Next Steps

- [Kubernetes Deployment](/docs/guides/kubernetes) - Deploy HA cluster on K8s
- [Monitoring Guide](/docs/guides/monitoring) - Set up comprehensive monitoring
- [Troubleshooting](/docs/resources/troubleshooting) - Debug cluster issues
