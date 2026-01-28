# ADR-0003: BadgerDB as Embedded Storage Engine

## Status

Accepted

## Context

Chronos requires persistent storage for:

- **Job definitions**: Schedule, webhook config, retry policies, metadata
- **Execution history**: Start/end times, status, response data, error messages
- **Schedule state**: Last execution time, next scheduled time for each job
- **Distributed locks**: Preventing concurrent execution of the same job

Storage requirements:

- **Embedded**: No external database dependency to maintain operational simplicity
- **High write throughput**: Recording execution results at scale
- **Key-value access patterns**: Jobs indexed by ID; executions by job ID + timestamp
- **Snapshot support**: Integration with Raft for state machine snapshots
- **Durable**: Data must survive process restarts and node failures

We evaluated:

1. **SQLite**: Mature but write contention issues in high-concurrency scenarios
2. **BoltDB**: Good for read-heavy workloads but single-writer limits throughput
3. **LevelDB/RocksDB**: Excellent performance but C++ dependencies complicate builds
4. **BadgerDB**: Pure Go LSM-tree with high write throughput and SSD optimization

## Decision

We chose **BadgerDB v4** as the embedded storage engine.

Key implementation:

```go
// internal/storage/badger.go
func NewStore(dataDir string) (*Store, error) {
    opts := badger.DefaultOptions(dbPath)
    opts.SyncWrites = true           // Durability guarantee
    opts.ValueLogFileSize = 64 << 20 // 64MB value log files
    
    db, err := badger.Open(opts)
    // ...
}
```

Data organization uses key prefixes:
- `jobs/{id}` - Job definitions
- `executions/{job_id}/{exec_id}` - Execution records  
- `schedule/{job_id}` - Schedule state
- `locks/{job_id}` - Execution locks
- `versions/{job_id}/{version}` - Job version history

## Consequences

### Positive

- **Pure Go**: No CGO dependencies; simple cross-compilation and static binaries
- **High write throughput**: LSM-tree architecture optimized for write-heavy workloads
- **SSD-optimized**: Separates keys and values to reduce write amplification on SSDs
- **Built-in compression**: Reduces storage footprint with Snappy/ZSTD compression
- **ACID transactions**: Full transaction support for atomic multi-key operations
- **Snapshot support**: Easy integration with Raft via `Snapshot()` and `Restore()` methods

### Negative

- **Memory usage**: LSM-tree requires memory for memtables and block cache
- **Compaction overhead**: Background compaction can cause latency spikes under heavy load
- **Less mature than alternatives**: Fewer production deployments than RocksDB/LevelDB
- **Value log garbage collection**: Requires periodic GC to reclaim space from deleted values

### Tradeoffs Accepted

- **Memory for performance**: We allocate sufficient memory for BadgerDB's caches to ensure low-latency reads
- **Compaction tuning**: Production deployments may need to tune compaction settings based on workload
- **GC overhead**: We run background garbage collection every 5 minutes to manage value log growth

### Garbage Collection

```go
// internal/storage/badger.go
func (s *Store) runGC() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        for {
            err := s.db.RunValueLogGC(0.5)
            if err == badger.ErrNoRewrite {
                break
            }
        }
    }
}
```

### Raft Integration

The store provides `Snapshot()` and `Restore()` methods for Raft state machine snapshots:

```go
func (s *Store) Snapshot() ([]byte, error) {
    // Serialize all data to JSON for Raft snapshot
}

func (s *Store) Restore(snapshot []byte) error {
    // Clear existing data and restore from snapshot
}
```

## References

- [BadgerDB Documentation](https://dgraph.io/docs/badger/)
- [WiscKey: Separating Keys from Values (FAST '16)](https://www.usenix.org/system/files/conference/fast16/fast16-papers-lu.pdf)
- [LSM-Tree Overview](https://www.cs.umb.edu/~pon} )
