// Package raft provides distributed consensus using hashicorp/raft.
package raft

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/rs/zerolog"
)

// Node manages distributed consensus.
type Node struct {
	raft      *raft.Raft
	fsm       *FSM
	transport *raft.NetworkTransport
	config    *raft.Config
	store     FSMStore

	nodeID    string
	raftDir   string
	raftAddr  string

	leaderCh   chan bool
	shutdownCh chan struct{}

	logger zerolog.Logger
	mu     sync.RWMutex
}

// Config holds Raft node configuration.
type Config struct {
	NodeID           string
	RaftDir          string
	RaftAddress      string
	Peers            []string
	HeartbeatTimeout time.Duration
	ElectionTimeout  time.Duration
	SnapshotInterval time.Duration
	SnapshotThreshold uint64
}

// DefaultConfig returns the default Raft configuration.
func DefaultConfig() *Config {
	return &Config{
		NodeID:            "node-1",
		RaftDir:           "./data/raft",
		RaftAddress:       "127.0.0.1:7000",
		HeartbeatTimeout:  500 * time.Millisecond,
		ElectionTimeout:   1 * time.Second,
		SnapshotInterval:  30 * time.Second,
		SnapshotThreshold: 1000,
	}
}

// NewNode creates a new Raft node.
func NewNode(cfg *Config, store FSMStore, logger zerolog.Logger) (*Node, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Create raft directory
	if err := os.MkdirAll(cfg.RaftDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create raft directory: %w", err)
	}

	// Create FSM
	fsm := NewFSM(store, logger)

	// Configure Raft
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(cfg.NodeID)
	raftConfig.HeartbeatTimeout = cfg.HeartbeatTimeout
	raftConfig.ElectionTimeout = cfg.ElectionTimeout
	raftConfig.LeaderLeaseTimeout = cfg.HeartbeatTimeout
	raftConfig.SnapshotInterval = cfg.SnapshotInterval
	raftConfig.SnapshotThreshold = cfg.SnapshotThreshold
	raftConfig.LogOutput = io.Discard // Use zerolog instead

	// Create transport
	addr, err := net.ResolveTCPAddr("tcp", cfg.RaftAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	transport, err := raft.NewTCPTransport(cfg.RaftAddress, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Create snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(cfg.RaftDir, 2, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Create log store and stable store using BoltDB
	boltDB, err := raftboltdb.NewBoltStore(filepath.Join(cfg.RaftDir, "raft.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create bolt store: %w", err)
	}

	// Create Raft instance
	r, err := raft.NewRaft(raftConfig, fsm, boltDB, boltDB, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft: %w", err)
	}

	node := &Node{
		raft:       r,
		fsm:        fsm,
		transport:  transport,
		config:     raftConfig,
		store:      store,
		nodeID:     cfg.NodeID,
		raftDir:    cfg.RaftDir,
		raftAddr:   cfg.RaftAddress,
		leaderCh:   make(chan bool, 1),
		shutdownCh: make(chan struct{}),
		logger:     logger.With().Str("component", "raft").Logger(),
	}

	// Start leader observer
	go node.observeLeadership()

	return node, nil
}

// Bootstrap bootstraps the cluster with this node as the only member.
func (n *Node) Bootstrap() error {
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(n.nodeID),
				Address: raft.ServerAddress(n.raftAddr),
			},
		},
	}

	f := n.raft.BootstrapCluster(configuration)
	if err := f.Error(); err != nil && err != raft.ErrCantBootstrap {
		return fmt.Errorf("failed to bootstrap cluster: %w", err)
	}

	n.logger.Info().Str("node_id", n.nodeID).Msg("Cluster bootstrapped")
	return nil
}

// Join joins an existing cluster.
func (n *Node) Join(leaderAddr string) error {
	n.logger.Info().Str("leader", leaderAddr).Msg("Joining cluster")

	// This would typically involve an RPC to the leader
	// For simplicity, we'll just add the server if we're the leader
	if n.IsLeader() {
		return n.AddServer(n.nodeID, n.raftAddr)
	}

	return nil
}

// AddServer adds a server to the cluster.
func (n *Node) AddServer(nodeID, address string) error {
	n.logger.Info().Str("node_id", nodeID).Str("address", address).Msg("Adding server to cluster")

	f := n.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(address), 0, 10*time.Second)
	if err := f.Error(); err != nil {
		return fmt.Errorf("failed to add server: %w", err)
	}

	return nil
}

// RemoveServer removes a server from the cluster.
func (n *Node) RemoveServer(nodeID string) error {
	n.logger.Info().Str("node_id", nodeID).Msg("Removing server from cluster")

	f := n.raft.RemoveServer(raft.ServerID(nodeID), 0, 10*time.Second)
	if err := f.Error(); err != nil {
		return fmt.Errorf("failed to remove server: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the Raft node.
func (n *Node) Shutdown() error {
	close(n.shutdownCh)

	f := n.raft.Shutdown()
	if err := f.Error(); err != nil {
		return fmt.Errorf("failed to shutdown raft: %w", err)
	}

	return nil
}

// IsLeader returns true if this node is the cluster leader.
func (n *Node) IsLeader() bool {
	return n.raft.State() == raft.Leader
}

// LeaderCh returns a channel that receives leadership changes.
func (n *Node) LeaderCh() <-chan bool {
	return n.leaderCh
}

// GetLeader returns the address of the current leader.
func (n *Node) GetLeader() (string, string) {
	addr, id := n.raft.LeaderWithID()
	return string(addr), string(id)
}

// GetServers returns all servers in the cluster.
func (n *Node) GetServers() ([]raft.Server, error) {
	f := n.raft.GetConfiguration()
	if err := f.Error(); err != nil {
		return nil, err
	}
	return f.Configuration().Servers, nil
}

// observeLeadership watches for leadership changes.
func (n *Node) observeLeadership() {
	for {
		select {
		case <-n.shutdownCh:
			return
		case isLeader := <-n.raft.LeaderCh():
			select {
			case n.leaderCh <- isLeader:
			default:
				// Channel full, skip
			}

			if isLeader {
				n.logger.Info().Msg("Became cluster leader")
			} else {
				n.logger.Info().Msg("Lost cluster leadership")
			}
		}
	}
}

// Apply applies a command through Raft consensus.
func (n *Node) Apply(cmd *Command, timeout time.Duration) error {
	if !n.IsLeader() {
		return models.ErrNotLeader
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	f := n.raft.Apply(data, timeout)
	if err := f.Error(); err != nil {
		return fmt.Errorf("failed to apply command: %w", err)
	}

	// Check for application error
	if resp := f.Response(); resp != nil {
		if err, ok := resp.(error); ok {
			return err
		}
	}

	return nil
}

// Stats returns Raft statistics.
func (n *Node) Stats() map[string]string {
	return n.raft.Stats()
}
