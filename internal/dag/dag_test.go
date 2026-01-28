package dag

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewDAG(t *testing.T) {
	dag := NewDAG("dag-1", "Test DAG")

	if dag.ID != "dag-1" {
		t.Errorf("expected ID 'dag-1', got %s", dag.ID)
	}
	if dag.Name != "Test DAG" {
		t.Errorf("expected name 'Test DAG', got %s", dag.Name)
	}
	if len(dag.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(dag.Nodes))
	}
}

func TestAddNode(t *testing.T) {
	dag := NewDAG("dag-1", "Test DAG")

	node := &Node{
		ID:    "node-1",
		Name:  "Node 1",
		JobID: "job-1",
	}

	err := dag.AddNode(node)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	if len(dag.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(dag.Nodes))
	}

	if len(dag.RootNodes) != 1 {
		t.Errorf("expected 1 root node, got %d", len(dag.RootNodes))
	}

	// Duplicate should fail
	err = dag.AddNode(node)
	if err == nil {
		t.Error("expected error for duplicate node")
	}
}

func TestValidate(t *testing.T) {
	t.Run("empty DAG", func(t *testing.T) {
		dag := NewDAG("dag-1", "Empty")
		err := dag.Validate()
		if err != ErrInvalidDAG {
			t.Errorf("expected ErrInvalidDAG, got %v", err)
		}
	})

	t.Run("missing dependency", func(t *testing.T) {
		dag := NewDAG("dag-1", "Test")
		dag.AddNode(&Node{ID: "a", Dependencies: []string{"missing"}})

		err := dag.Validate()
		if err == nil {
			t.Error("expected error for missing dependency")
		}
	})

	t.Run("cycle detection", func(t *testing.T) {
		dag := NewDAG("dag-1", "Cyclic")
		dag.Nodes["a"] = &Node{ID: "a", Dependencies: []string{"c"}}
		dag.Nodes["b"] = &Node{ID: "b", Dependencies: []string{"a"}}
		dag.Nodes["c"] = &Node{ID: "c", Dependencies: []string{"b"}}

		err := dag.Validate()
		if err != ErrCycleDetected {
			t.Errorf("expected ErrCycleDetected, got %v", err)
		}
	})

	t.Run("valid DAG", func(t *testing.T) {
		dag := NewDAG("dag-1", "Valid")
		dag.AddNode(&Node{ID: "a"})
		dag.AddNode(&Node{ID: "b", Dependencies: []string{"a"}})
		dag.AddNode(&Node{ID: "c", Dependencies: []string{"a"}})
		dag.AddNode(&Node{ID: "d", Dependencies: []string{"b", "c"}})

		err := dag.Validate()
		if err != nil {
			t.Errorf("expected valid DAG, got error: %v", err)
		}
	})
}

func TestTopologicalSort(t *testing.T) {
	dag := NewDAG("dag-1", "Test")
	dag.AddNode(&Node{ID: "a"})
	dag.AddNode(&Node{ID: "b", Dependencies: []string{"a"}})
	dag.AddNode(&Node{ID: "c", Dependencies: []string{"a"}})
	dag.AddNode(&Node{ID: "d", Dependencies: []string{"b", "c"}})

	sorted, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}

	if len(sorted) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(sorted))
	}

	// 'a' should come before 'b' and 'c'
	// 'd' should come last
	aIdx, bIdx, cIdx, dIdx := -1, -1, -1, -1
	for i, id := range sorted {
		switch id {
		case "a":
			aIdx = i
		case "b":
			bIdx = i
		case "c":
			cIdx = i
		case "d":
			dIdx = i
		}
	}

	if aIdx > bIdx || aIdx > cIdx {
		t.Error("'a' should come before 'b' and 'c'")
	}
	if bIdx > dIdx || cIdx > dIdx {
		t.Error("'d' should come after 'b' and 'c'")
	}
}

func TestGetExecutableNodes(t *testing.T) {
	dag := NewDAG("dag-1", "Test")
	dag.AddNode(&Node{ID: "a"})
	dag.AddNode(&Node{ID: "b", Dependencies: []string{"a"}})
	dag.AddNode(&Node{ID: "c", Dependencies: []string{"a"}})

	states := map[string]*NodeState{
		"a": {NodeID: "a", Status: NodePending},
		"b": {NodeID: "b", Status: NodePending},
		"c": {NodeID: "c", Status: NodePending},
	}

	// Initially only 'a' (root) should be ready
	ready := dag.GetExecutableNodes(states)
	if len(ready) != 1 || ready[0] != "a" {
		t.Errorf("expected ['a'], got %v", ready)
	}

	// After 'a' succeeds, 'b' and 'c' should be ready
	states["a"].Status = NodeSuccess
	ready = dag.GetExecutableNodes(states)
	if len(ready) != 2 {
		t.Errorf("expected 2 ready nodes, got %d", len(ready))
	}
}

// mockExecutor is a test executor.
type mockExecutor struct {
	mu     sync.Mutex
	delay  time.Duration
	fail   map[string]bool
	called map[string]int
}

func (m *mockExecutor) ExecuteNode(ctx context.Context, node *Node) (string, error) {
	m.mu.Lock()
	if m.called == nil {
		m.called = make(map[string]int)
	}
	m.called[node.ID]++
	m.mu.Unlock()

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if m.fail != nil && m.fail[node.ID] {
		return "", context.DeadlineExceeded
	}

	return "exec-" + node.ID, nil
}

func (m *mockExecutor) getCalled(nodeID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.called == nil {
		return 0
	}
	return m.called[nodeID]
}

func TestEngine(t *testing.T) {
	t.Run("RegisterDAG", func(t *testing.T) {
		engine := NewEngine(&mockExecutor{})

		dag := NewDAG("dag-1", "Test")
		dag.AddNode(&Node{ID: "a"})

		err := engine.RegisterDAG(dag)
		if err != nil {
			t.Fatalf("RegisterDAG failed: %v", err)
		}

		retrieved, err := engine.GetDAG("dag-1")
		if err != nil {
			t.Fatalf("GetDAG failed: %v", err)
		}
		if retrieved.Name != "Test" {
			t.Errorf("expected name 'Test', got %s", retrieved.Name)
		}
	})

	t.Run("ListDAGs", func(t *testing.T) {
		engine := NewEngine(&mockExecutor{})

		dag1 := NewDAG("dag-1", "Test 1")
		dag1.AddNode(&Node{ID: "a"})
		engine.RegisterDAG(dag1)

		dag2 := NewDAG("dag-2", "Test 2")
		dag2.AddNode(&Node{ID: "b"})
		engine.RegisterDAG(dag2)

		dags := engine.ListDAGs()
		if len(dags) != 2 {
			t.Errorf("expected 2 DAGs, got %d", len(dags))
		}
	})

	t.Run("StartRun", func(t *testing.T) {
		executor := &mockExecutor{}
		engine := NewEngine(executor)

		dag := NewDAG("dag-1", "Test")
		dag.AddNode(&Node{ID: "a"})
		dag.AddNode(&Node{ID: "b", Dependencies: []string{"a"}})
		engine.RegisterDAG(dag)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		run, err := engine.StartRun(ctx, "dag-1", "run-1")
		if err != nil {
			t.Fatalf("StartRun failed: %v", err)
		}

		// Wait for completion
		time.Sleep(500 * time.Millisecond)

		// Check run status
		run, _ = engine.GetRun("run-1")
		if run.GetStatus() != RunSuccess {
			t.Errorf("expected RunSuccess, got %s", run.GetStatus())
		}

		// Check all nodes executed
		if executor.getCalled("a") != 1 {
			t.Errorf("expected node 'a' to be called once, got %d", executor.getCalled("a"))
		}
		if executor.getCalled("b") != 1 {
			t.Errorf("expected node 'b' to be called once, got %d", executor.getCalled("b"))
		}
	})
}

func TestNodeStatus(t *testing.T) {
	tests := []struct {
		status   NodeStatus
		expected string
	}{
		{NodePending, "pending"},
		{NodeReady, "ready"},
		{NodeRunning, "running"},
		{NodeSuccess, "success"},
		{NodeFailed, "failed"},
		{NodeSkipped, "skipped"},
		{NodeCancelled, "cancelled"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.status)
		}
	}
}
