// Package storage provides storage interfaces and implementations for Chronos.
//
// This package defines the storage abstraction layer and provides two implementations:
// BadgerDB for production use and an in-memory store for testing.
//
// # Storage Interfaces
//
// The package defines several focused interfaces that can be composed:
//
//   - [JobStore]: CRUD operations for job definitions
//   - [ExecutionStore]: Storage for execution history records
//   - [ScheduleStore]: Schedule state persistence (next run times)
//   - [LockStore]: Distributed locking for job executions
//   - [VersionStore]: Job configuration versioning
//   - [SnapshotStore]: Raft snapshot operations
//
// The [Store] interface combines all interfaces for components needing full access.
//
// # BadgerDB Implementation
//
// [BadgerStore] is the production storage implementation using BadgerDB, an
// embeddable LSM-tree key-value store written in pure Go. It provides:
//
//   - ACID transactions
//   - High write throughput (ideal for execution logs)
//   - No CGO dependencies
//   - Automatic value log garbage collection
//
// Key prefixes used in BadgerDB:
//
//   - jobs/           Job definitions
//   - executions/     Execution history records
//   - schedule/       Schedule state (next run times)
//   - locks/          Distributed execution locks
//   - versions/       Job configuration versions
//
// # In-Memory Implementation
//
// [MemoryStore] provides a fully functional in-memory implementation for testing.
// It implements the same interfaces but stores all data in maps. This is useful for:
//
//   - Unit testing without disk I/O
//   - Integration testing with predictable state
//   - Development without persistent data
//
// # Usage
//
// Creating a BadgerDB store:
//
//	store, err := storage.NewBadgerStore("/var/lib/chronos/data")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer store.Close()
//
// Creating an in-memory store for testing:
//
//	store := storage.NewMemoryStore()
//
// Using specific interfaces:
//
//	func SaveJob(js storage.JobStore, job *models.Job) error {
//	    return js.CreateJob(job)
//	}
//
// # Error Handling
//
// Storage operations return typed errors for specific conditions:
//
//   - ErrJobNotFound: Requested job does not exist
//   - ErrJobAlreadyExists: Job with same ID already exists
//   - ErrExecutionNotFound: Requested execution does not exist
//
// # Thread Safety
//
// Both [BadgerStore] and [MemoryStore] are fully thread-safe and can be used
// concurrently from multiple goroutines.
//
// # Raft Integration
//
// The [SnapshotStore] interface enables Raft snapshot operations for state
// replication across cluster nodes. The Snapshot method serializes all data,
// and Restore deserializes it to rebuild state from a snapshot.
package storage
