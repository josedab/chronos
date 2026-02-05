// Package raft provides distributed consensus using HashiCorp Raft.
//
// This package implements the distributed coordination layer for Chronos,
// ensuring consistent state across cluster nodes and providing automatic
// leader election for job scheduling.
//
// # Architecture
//
// The raft package wraps [hashicorp/raft] with Chronos-specific functionality:
//
//   - [Node]: Manages the Raft cluster membership and lifecycle
//   - [FSM]: Finite State Machine that applies commands to storage
//   - [Command]: Strongly-typed commands for state mutations
//
// # Consensus Model
//
// Chronos uses Raft consensus to ensure:
//
//   - Only one node (the leader) actively schedules jobs
//   - All state changes (job creates, updates, executions) are replicated
//   - Cluster survives minority node failures (e.g., 1 of 3, 2 of 5)
//   - Automatic leader election on failure (~5 seconds typical)
//
// # State Machine Commands
//
// All mutations to cluster state go through Raft as commands:
//
//   - CreateJobCommand: Add a new job
//   - UpdateJobCommand: Modify job configuration
//   - DeleteJobCommand: Remove a job
//   - RecordExecutionCommand: Store execution result
//   - UpdateScheduleCommand: Update next run time
//
// # Leader Election
//
// The package provides a leader notification channel for components that
// need to respond to leadership changes (e.g., the scheduler):
//
//	node, _ := raft.NewNode(cfg, store, logger)
//	go func() {
//	    for isLeader := range node.LeaderCh() {
//	        if isLeader {
//	            scheduler.BecomeLeader()
//	        } else {
//	            scheduler.BecomeFollower()
//	        }
//	    }
//	}()
//
// # Snapshots
//
// Raft periodically creates snapshots of the FSM state to:
//
//   - Compact the Raft log
//   - Enable fast node recovery
//   - Support new node bootstrap
//
// Snapshot configuration:
//
//   - SnapshotInterval: Time between snapshots (default: 30s)
//   - SnapshotThreshold: Commits between snapshots (default: 1000)
//
// # Usage
//
// Creating a Raft node:
//
//	cfg := &raft.Config{
//	    NodeID:      "chronos-1",
//	    RaftDir:     "/var/lib/chronos/raft",
//	    RaftAddress: "10.0.0.1:7000",
//	    Peers:       []string{"10.0.0.2:7000", "10.0.0.3:7000"},
//	}
//
//	store := storage.NewBadgerStore("/var/lib/chronos/data")
//	node, err := raft.NewNode(cfg, store, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Apply a command (only works on leader)
//	err = node.Apply(raft.CreateJobCommand{Job: job})
//
// # Cluster Bootstrap
//
// For a new cluster, the first node bootstraps itself. Subsequent nodes
// join the existing cluster:
//
//	// First node - bootstrap
//	node.Bootstrap()
//
//	// Additional nodes - join
//	// (automatically handled via peer configuration)
//
// # Metrics
//
// Raft exposes metrics for monitoring:
//
//   - chronos_raft_is_leader: Whether this node is the leader (0/1)
//   - chronos_raft_peers: Number of peers in the cluster
//   - chronos_raft_applied_index: Last applied log index
//   - chronos_raft_commit_index: Last committed log index
//
// # Thread Safety
//
// The Node and FSM are thread-safe. Commands are serialized through the
// Raft log, ensuring consistent ordering across all nodes.
package raft
