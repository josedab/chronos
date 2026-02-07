---
sidebar_position: 6
title: Multi-Region
description: Deploy Chronos across multiple regions with federation
---

# Multi-Region Federation

Deploy Chronos clusters across multiple geographic regions for global availability, disaster recovery, and data locality compliance.

## Overview

Multi-region federation enables:

- **Global availability**: Jobs continue running if an entire region goes down
- **Data locality**: Run jobs close to your data for compliance and performance
- **Follow-the-sun**: Execute jobs in the region where it's business hours
- **Disaster recovery**: Automatic failover to healthy regions

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Federation Control Plane                          │
│                     (metadata sync, routing decisions)                   │
└─────────────────────────────────────────────────────────────────────────┘
         │                          │                          │
         ▼                          ▼                          ▼
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   US-EAST-1     │      │   EU-WEST-1     │      │   AP-SOUTH-1    │
│   Chronos       │◄────►│   Chronos       │◄────►│   Chronos       │
│   Cluster       │      │   Cluster       │      │   Cluster       │
│                 │      │                 │      │                 │
│  ┌───────────┐  │      │  ┌───────────┐  │      │  ┌───────────┐  │
│  │ Node 1    │  │      │  │ Node 1    │  │      │  │ Node 1    │  │
│  │ Node 2    │  │      │  │ Node 2    │  │      │  │ Node 2    │  │
│  │ Node 3    │  │      │  │ Node 3    │  │      │  │ Node 3    │  │
│  └───────────┘  │      │  └───────────┘  │      │  └───────────┘  │
└─────────────────┘      └─────────────────┘      └─────────────────┘
```

## Federation Modes

### Replicate Mode

All jobs exist in all regions. Best for disaster recovery.

```yaml
federation:
  enabled: true
  mode: replicate
  
  # Jobs are automatically synced to all clusters
  sync_interval: 30s
  conflict_resolution: primary-wins
```

**Use cases**:
- Mission-critical jobs that must survive regional outages
- Jobs with no data locality requirements
- Simple disaster recovery setups

### Partition Mode

Jobs are assigned to specific regions. Best for data locality.

```yaml
federation:
  enabled: true
  mode: partition
  
  # Job routing based on metadata
  routing:
    strategy: metadata  # or: round-robin, latency-based
    metadata_key: region
```

**Use cases**:
- GDPR compliance (EU data stays in EU)
- Performance optimization (run jobs near data)
- Cost optimization (use cheaper regions for batch jobs)

### Active-Active Mode

Jobs can execute in any region, with intelligent routing.

```yaml
federation:
  enabled: true
  mode: active-active
  
  routing:
    strategy: latency-based  # Route to fastest responding cluster
    health_check_interval: 10s
    
  # Prevent duplicate execution
  execution_lock:
    enabled: true
    provider: redis  # or: dynamodb, consul
    ttl: 300s
```

**Use cases**:
- Global services needing lowest latency
- High availability with automatic failover
- Load distribution across regions

## Configuration

### Primary Cluster (US-East)

```yaml
# chronos-us-east.yaml
cluster:
  name: us-east-1
  role: primary  # First cluster to initialize

federation:
  enabled: true
  mode: replicate
  
  # This cluster's endpoint (for other clusters to connect)
  advertise:
    endpoint: https://chronos-us-east.example.com
    port: 8080
  
  # Other clusters in the federation
  clusters:
    - name: eu-west-1
      endpoint: https://chronos-eu-west.example.com
      port: 8080
    - name: ap-south-1
      endpoint: https://chronos-ap-south.example.com
      port: 8080
  
  # Authentication between clusters
  auth:
    type: mtls
    ca_cert: /etc/chronos/certs/ca.crt
    cert: /etc/chronos/certs/cluster.crt
    key: /etc/chronos/certs/cluster.key

  # Sync settings
  sync:
    interval: 30s
    batch_size: 100
    timeout: 10s
```

### Secondary Cluster (EU-West)

```yaml
# chronos-eu-west.yaml
cluster:
  name: eu-west-1
  role: secondary

federation:
  enabled: true
  mode: replicate
  
  advertise:
    endpoint: https://chronos-eu-west.example.com
    port: 8080
  
  clusters:
    - name: us-east-1
      endpoint: https://chronos-us-east.example.com
      port: 8080
    - name: ap-south-1
      endpoint: https://chronos-ap-south.example.com
      port: 8080
  
  auth:
    type: mtls
    ca_cert: /etc/chronos/certs/ca.crt
    cert: /etc/chronos/certs/cluster.crt
    key: /etc/chronos/certs/cluster.key
```

## Job Configuration for Multi-Region

### Region-Specific Jobs (Partition Mode)

```yaml
name: eu-gdpr-cleanup
description: GDPR data cleanup - EU only
schedule: "0 2 * * *"
timezone: Europe/Berlin

# Assign to specific region
region: eu-west-1

# Prevent execution elsewhere
constraints:
  require_region: true
  
command: /scripts/gdpr-cleanup.sh
```

### Replicated Jobs with Regional Preferences

```yaml
name: database-backup
description: Database backup - prefer local region
schedule: "0 * * * *"
timezone: UTC

# Run in any region, prefer us-east
region_preference:
  - us-east-1
  - eu-west-1
  - ap-south-1

# Only one execution globally
execution_mode: single

command: /scripts/backup.sh
```

### Follow-the-Sun Jobs

```yaml
name: support-queue-processor
description: Process support tickets during business hours
schedule: "0 9-17 * * 1-5"  # 9 AM - 5 PM, Mon-Fri

# Execute in the region where it's business hours
routing:
  strategy: business-hours
  timezones:
    us-east-1: America/New_York
    eu-west-1: Europe/London
    ap-south-1: Asia/Kolkata

command: /scripts/process-tickets.sh
```

## Distributed Execution Lock

Prevent duplicate job execution across regions:

### Redis-Based Lock

```yaml
federation:
  execution_lock:
    enabled: true
    provider: redis
    
    redis:
      addresses:
        - redis-us-east.example.com:6379
        - redis-eu-west.example.com:6379
        - redis-ap-south.example.com:6379
      cluster_mode: true
      password_env: REDIS_PASSWORD
      
    ttl: 300s  # Lock timeout
    retry_interval: 100ms
```

### DynamoDB-Based Lock (AWS)

```yaml
federation:
  execution_lock:
    enabled: true
    provider: dynamodb
    
    dynamodb:
      table_name: chronos-locks
      region: us-east-1  # Global table with replicas
      
    ttl: 300s
```

## Failover Behavior

### Automatic Failover

When a region becomes unavailable:

1. **Detection**: Health checks fail for 30 seconds
2. **Promotion**: Jobs are redistributed to healthy regions
3. **Execution**: Pending jobs execute in alternative region
4. **Recovery**: When original region recovers, jobs migrate back

```yaml
federation:
  failover:
    enabled: true
    detection_threshold: 30s  # Time before declaring region unhealthy
    
    # Behavior during failover
    on_failover:
      migrate_pending_jobs: true
      migrate_running_jobs: false  # Let them complete or timeout
      
    # Behavior when region recovers
    on_recovery:
      rebalance_jobs: true
      rebalance_delay: 60s  # Wait before moving jobs back
```

### Manual Failover

Force failover for maintenance:

```bash
# Drain region before maintenance
chronosctl federation drain --region us-east-1 --timeout 5m

# Perform maintenance...

# Restore region
chronosctl federation restore --region us-east-1
```

## Monitoring Federation

### Key Metrics

```
# Cluster health
chronos_federation_cluster_healthy{cluster="us-east-1"} 1

# Sync status
chronos_federation_sync_lag_seconds{from="us-east-1", to="eu-west-1"}
chronos_federation_sync_failures_total

# Cross-region execution
chronos_federation_cross_region_executions_total{from="us-east-1", to="eu-west-1"}

# Lock contention
chronos_federation_lock_acquisitions_total
chronos_federation_lock_contentions_total
chronos_federation_lock_failures_total
```

### Alerting Rules

```yaml
groups:
  - name: federation
    rules:
      - alert: FederationClusterUnhealthy
        expr: chronos_federation_cluster_healthy == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Federation cluster {{ $labels.cluster }} is unhealthy"
          
      - alert: FederationSyncLagging
        expr: chronos_federation_sync_lag_seconds > 60
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Federation sync lag exceeds 60 seconds"
          
      - alert: HighLockContention
        expr: rate(chronos_federation_lock_contentions_total[5m]) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High lock contention in federation"
```

### Federation Dashboard

View federation status:

```bash
# Cluster status overview
chronosctl federation status

# Output:
# CLUSTER      ROLE       STATUS    JOBS    LAST SYNC
# us-east-1    primary    healthy   1,234   2s ago
# eu-west-1    secondary  healthy   1,234   5s ago  
# ap-south-1   secondary  healthy   1,234   3s ago

# Detailed sync status
chronosctl federation sync-status

# Cross-region job distribution
chronosctl federation jobs --by-region
```

## Network Requirements

### Latency

| Connection | Maximum Latency |
|------------|-----------------|
| Intra-cluster (Raft) | &lt;10ms |
| Inter-cluster sync | &lt;200ms |
| Lock acquisition | &lt;100ms |

### Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 8080 | HTTPS | API and federation sync |
| 8081 | gRPC | Inter-cluster communication |
| 7946 | TCP/UDP | Cluster membership (optional) |

### Firewall Rules

```bash
# Allow federation traffic between clusters
# From: Any Chronos node
# To: Other cluster endpoints
# Port: 8080, 8081

# Example: us-east to eu-west
iptables -A OUTPUT -d chronos-eu-west.example.com -p tcp --dport 8080 -j ACCEPT
iptables -A OUTPUT -d chronos-eu-west.example.com -p tcp --dport 8081 -j ACCEPT
```

## Disaster Recovery

### Planned Failover

For maintenance windows:

```bash
# 1. Disable scheduling in the region
chronosctl federation pause --region us-east-1

# 2. Wait for running jobs to complete
chronosctl federation drain --region us-east-1 --wait

# 3. Perform maintenance
# ...

# 4. Re-enable the region
chronosctl federation resume --region us-east-1
```

### Unplanned Failover

If a region fails unexpectedly:

1. Federation automatically detects the failure
2. Jobs are redistributed to healthy regions
3. Distributed locks prevent duplicate execution
4. When region recovers, jobs automatically rebalance

### Full Federation Recovery

If all regions fail simultaneously:

```bash
# 1. Start primary cluster
chronos --config /etc/chronos/primary.yaml

# 2. Wait for primary to be healthy
chronosctl health --wait

# 3. Start secondary clusters
# They will automatically sync from primary

# 4. Verify federation
chronosctl federation status
```

## Troubleshooting

### Clusters Not Syncing

```bash
# Check connectivity
chronosctl federation ping --all

# View sync errors
chronosctl federation logs --level error

# Force sync
chronosctl federation sync --force
```

### Split Brain Detection

If clusters diverge:

```bash
# Check for conflicts
chronosctl federation conflicts

# Resolve by choosing authoritative cluster
chronosctl federation resolve --authoritative us-east-1
```

### Lock Failures

```bash
# Check lock provider health
chronosctl federation lock-status

# Clear stale locks (use with caution)
chronosctl federation locks clear --stale --older-than 1h
```

## Best Practices

1. **Start with replicate mode** - simplest to reason about
2. **Use mTLS** between clusters - never expose federation without encryption
3. **Deploy lock provider** close to clusters - minimize lock latency
4. **Monitor sync lag** - catch issues before they cause problems
5. **Test failover regularly** - don't wait for a real disaster
6. **Document your topology** - know which region serves which purpose
7. **Plan for network partitions** - they will happen
