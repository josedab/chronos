# ADR-0002: Raft Consensus for Distributed Coordination

## Status

Accepted

## Context

Chronos must provide reliable job scheduling across a cluster of nodes with the following requirements:

- **Leader election**: Exactly one node should be the active scheduler at any time to prevent duplicate job executions
- **State replication**: Job definitions and execution state must survive node failures
- **Consistency guarantees**: All nodes must agree on the current state of jobs and their schedules
- **Automatic failover**: If the leader fails, a new leader must be elected within seconds

We evaluated several approaches:

1. **External coordination service (etcd, ZooKeeper, Consul)**: Proven solutions but add operational complexity and a critical dependency
2. **Database-based locking (PostgreSQL advisory locks)**: Requires external database; lock contention at scale
3. **Custom protocol**: High development cost and risk of subtle distributed systems bugs
4. **Embedded Raft consensus**: Self-contained, well-understood algorithm with production-ready implementations

## Decision

We chose **HashiCorp Raft** as our consensus implementation, embedding it directly into Chronos nodes.

Key implementation details:

```go
// internal/raft/node.go
type Node struct {
    raft      *raft.Raft
    fsm       *FSM
    transport *raft.NetworkTransport
    store     *storage.Store
    // ...
}
```

- Each Chronos node participates in the Raft cluster
- The FSM (Finite State Machine) applies commands to BadgerDB storage
- Leadership changes trigger scheduler activation/deactivation
- BoltDB stores Raft logs and snapshots for durability

## Consequences

### Positive

- **Zero external dependencies**: No need to operate etcd/ZooKeeper alongside Chronos
- **Strong consistency**: Raft provides linearizable reads/writes for job state
- **Automatic leader election**: Leadership transfers complete in ~1 second with default timeouts
- **Proven implementation**: HashiCorp Raft powers Consul, Nomad, and Vault in production at scale
- **Snapshot support**: Large state can be compacted via snapshots, keeping log size bounded

### Negative

- **Odd-number clusters required**: Must deploy 3, 5, or 7 nodes for fault tolerance (2n+1 for n failures)
- **Write amplification**: All writes go through leader and are replicated to followers
- **Network partition sensitivity**: Split-brain scenarios require careful configuration of election timeouts
- **Operational complexity**: Cluster membership changes require careful orchestration

### Tradeoffs Accepted

- **Consistency over availability**: During leader election (~1s), writes are unavailable; we accept this for correctness
- **Embedded vs external**: We trade operational simplicity for tighter coupling between Chronos and its coordination layer
- **Cluster size constraints**: 3-node minimum increases infrastructure cost but ensures high availability

### Configuration

```yaml
# chronos.yaml
cluster:
  raft:
    heartbeat_timeout: 500ms
    election_timeout: 1s
    snapshot_interval: 30s
    snapshot_threshold: 1000
```

- **Heartbeat timeout**: How often leader sends heartbeats (500ms default)
- **Election timeout**: Time before follower starts election (1s default)
- **Snapshot threshold**: Number of log entries before snapshotting

## References

- [Raft Consensus Algorithm](https://raft.github.io/)
- [HashiCorp Raft Library](https://github.com/hashicorp/raft)
- [Raft Paper (Ongaro & Ousterhout)](https://raft.github.io/raft.pdf)
