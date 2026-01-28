# ADR-0010: Cross-Region Federation for Global Deployment

## Status

Accepted

## Context

Global organizations have requirements that single-region deployments cannot satisfy:

- **Latency**: Jobs should execute close to their target services
- **Data residency**: Some jobs must run in specific geographic regions
- **Disaster recovery**: Survive entire region outages
- **Follow-the-sun**: Operations teams in different time zones manage their jobs

Options considered:

1. **Independent clusters**: Separate Chronos deployments per region
   - Simple isolation
   - No coordination or failover
   - Duplicate job definitions

2. **Active-passive replication**: One primary, multiple standby regions
   - Clear leadership
   - Wasted standby capacity
   - Failover requires intervention

3. **Federated clusters**: Connected peers with job synchronization
   - Each region operates independently
   - Job definitions replicate across regions
   - Configurable execution policies

## Decision

We implemented **cross-region federation** in `internal/geo/`:

```go
// internal/geo/federation.go
type FederationManager struct {
    localClusterID string
    clusters       map[string]*FederatedCluster
    status         map[string]*ClusterStatus
    jobs           map[string]*FederatedJob
    syncQueue      chan *SyncMessage
}

type FederatedCluster struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Region      string   `json:"region"`
    Endpoints   []string `json:"endpoints"`
    APIKey      string   `json:"api_key,omitempty"`
    Priority    int      `json:"priority"`
    Enabled     bool     `json:"enabled"`
}
```

### Federation Modes

Jobs specify how they federate:

```go
type FederationMode string

const (
    FederationModeReplicate FederationMode = "replicate"  // Copy to all regions
    FederationModeFailover  FederationMode = "failover"   // Standby replicas
    FederationModePartition FederationMode = "partition"  // Shard across regions
)
```

### Execution Policies

Where should a federated job execute?

```go
type ExecutionPolicy string

const (
    ExecutionPolicyOwnerOnly   ExecutionPolicy = "owner_only"    // Only on origin cluster
    ExecutionPolicyAnyCluster  ExecutionPolicy = "any_cluster"   // Wherever healthy
    ExecutionPolicyAllClusters ExecutionPolicy = "all_clusters"  // Execute everywhere
    ExecutionPolicyPreferred   ExecutionPolicy = "preferred"     // Owner first, failover ok
)
```

### Conflict Resolution

When the same job is modified in multiple regions:

```go
type ConflictResolution string

const (
    ConflictResolutionNewest    ConflictResolution = "newest"     // Latest timestamp wins
    ConflictResolutionOwnerWins ConflictResolution = "owner_wins" // Origin cluster wins
    ConflictResolutionMerge     ConflictResolution = "merge"      // Attempt merge
    ConflictResolutionManual    ConflictResolution = "manual"     // Flag for human review
)
```

## Consequences

### Positive

- **Global availability**: Jobs run even during regional outages
- **Latency optimization**: Execute jobs in the nearest region
- **Data sovereignty**: Comply with regional data requirements
- **Operational flexibility**: Teams manage jobs in their preferred region
- **Gradual rollout**: Federation is opt-in per job

### Negative

- **Complexity**: Distributed state introduces consistency challenges
- **Network dependency**: Cross-region sync requires reliable connectivity
- **Conflict resolution**: Concurrent edits need resolution strategy
- **Debugging**: Issues may span multiple regions

### Synchronization Protocol

```go
type SyncMessage struct {
    ID            string          `json:"id"`
    Type          SyncMessageType `json:"type"`
    SourceCluster string          `json:"source_cluster"`
    TargetCluster string          `json:"target_cluster,omitempty"`
    Payload       json.RawMessage `json:"payload"`
    Version       int64           `json:"version"`
    Timestamp     time.Time       `json:"timestamp"`
}

const (
    SyncTypeJobCreate    SyncMessageType = "job_create"
    SyncTypeJobUpdate    SyncMessageType = "job_update"
    SyncTypeJobDelete    SyncMessageType = "job_delete"
    SyncTypeHeartbeat    SyncMessageType = "heartbeat"
    SyncTypeClusterJoin  SyncMessageType = "cluster_join"
    SyncTypeClusterLeave SyncMessageType = "cluster_leave"
)
```

### Health Monitoring

Clusters continuously monitor each other:

```go
type ClusterStatus struct {
    ClusterID   string        `json:"cluster_id"`
    Region      string        `json:"region"`
    Healthy     bool          `json:"healthy"`
    IsLeader    bool          `json:"is_leader"`
    NodeCount   int           `json:"node_count"`
    Latency     time.Duration `json:"latency"`
    LastSync    time.Time     `json:"last_sync"`
    LastCheck   time.Time     `json:"last_check"`
}
```

### Cluster Selection

For jobs that can execute anywhere:

```go
func (f *FederationManager) SelectExecutionCluster(jobID string) (string, error) {
    fedJob := f.jobs[jobID]
    
    switch fedJob.FederationConfig.ExecutionPolicy {
    case ExecutionPolicyOwnerOnly:
        return fedJob.OwnerCluster, nil
        
    case ExecutionPolicyAnyCluster:
        return f.selectHealthiestCluster(fedJob.ReplicaClusters)
        
    case ExecutionPolicyPreferred:
        if f.isHealthy(fedJob.OwnerCluster) {
            return fedJob.OwnerCluster, nil
        }
        return f.selectHealthiestCluster(fedJob.ReplicaClusters)
    }
}
```

### Configuration

```yaml
# chronos.yaml
federation:
  enabled: true
  local_cluster_id: us-east-1-prod
  local_region: us-east-1
  sync_interval: 30s
  health_check_interval: 15s
  sync_timeout: 10s
  conflict_resolution: newest
  
  clusters:
    - id: eu-west-1-prod
      region: eu-west-1
      endpoints:
        - https://chronos-eu.example.com
      api_key: ${secret:vault:federation/eu-west-1}
      priority: 2
      
    - id: ap-southeast-1-prod
      region: ap-southeast-1
      endpoints:
        - https://chronos-ap.example.com
      priority: 3
```

### Job Federation Configuration

```json
{
  "name": "global-health-check",
  "federation_config": {
    "enabled": true,
    "mode": "replicate",
    "execution_policy": "any_cluster",
    "conflict_resolution": "newest"
  }
}
```

## References

- [CAP Theorem](https://en.wikipedia.org/wiki/CAP_theorem)
- [CRDTs for Conflict Resolution](https://crdt.tech/)
- [Multi-Region Deployment Patterns](https://aws.amazon.com/blogs/architecture/disaster-recovery-dr-architecture-on-aws-part-i-strategies-for-recovery-in-the-cloud/)
