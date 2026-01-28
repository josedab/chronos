# ADR-0007: DAG-Based Workflow Orchestration

## Status

Accepted

## Context

Simple cron scheduling handles independent jobs well, but real-world workflows often involve:

- **Dependencies**: Job B should run only after Job A completes successfully
- **Fan-out/fan-in**: Multiple jobs run in parallel, then results aggregate
- **Conditional execution**: Run cleanup only if the main job fails
- **Complex pipelines**: ETL workflows with multiple stages

Without workflow support, users resort to:
- Chaining jobs via webhooks (tight coupling, no visibility)
- External orchestrators like Airflow (operational complexity)
- Manual scheduling with time gaps (fragile, wastes time)

Requirements:
- **Declarative dependencies**: Define job relationships, not orchestration logic
- **Parallel execution**: Independent jobs run concurrently
- **Failure handling**: Configurable behavior when dependencies fail
- **Visibility**: Track workflow progress across all jobs

## Decision

We implemented **DAG (Directed Acyclic Graph) based workflow orchestration** in the `internal/dag/` package.

### Core Concepts

```go
// internal/dag/dag.go
type DAG struct {
    ID          string           `json:"id"`
    Name        string           `json:"name"`
    Nodes       map[string]*Node `json:"nodes"`
    RootNodes   []string         `json:"root_nodes"`
}

type Node struct {
    ID           string           `json:"id"`
    JobID        string           `json:"job_id"`
    Dependencies []string         `json:"dependencies"`
    Condition    TriggerCondition `json:"condition"`
    Timeout      time.Duration    `json:"timeout"`
}
```

### Trigger Conditions

Nodes specify when they should execute relative to dependencies:

```go
const (
    ConditionAllSuccess  TriggerCondition = "all_success"   // All deps succeeded
    ConditionAllComplete TriggerCondition = "all_complete"  // All deps finished (success or fail)
    ConditionAnySuccess  TriggerCondition = "any_success"   // At least one dep succeeded
    ConditionNone        TriggerCondition = "none"          // No dependencies (root node)
)
```

### Execution Model

1. **Validation**: Check for cycles using DFS; verify all dependencies exist
2. **Topological sort**: Determine valid execution orders
3. **Parallel dispatch**: Execute all ready nodes concurrently
4. **State tracking**: Monitor each node's status (pending/running/success/failed)
5. **Propagation**: When nodes complete, check if dependents are now ready

```go
// internal/dag/dag.go
func (e *Engine) executeRun(ctx context.Context, dag *DAG, run *Run) {
    for {
        ready := dag.GetExecutableNodes(run.NodeStates)
        if len(ready) == 0 {
            // Check if all complete or waiting
            break
        }
        
        var wg sync.WaitGroup
        for _, nodeID := range ready {
            wg.Add(1)
            go func(nID string) {
                defer wg.Done()
                e.executeNode(ctx, dag, run, nID)
            }(nodeID)
        }
        wg.Wait()
    }
}
```

## Consequences

### Positive

- **Complex workflows**: Multi-stage pipelines with branching and merging
- **Efficient execution**: Parallel execution of independent branches
- **Failure isolation**: Failed branches don't block independent paths
- **Clear visualization**: DAG structure maps naturally to UI representation
- **Reusability**: Jobs can participate in multiple DAGs

### Negative

- **Complexity**: DAG semantics more complex than simple cron
- **Debugging**: Failures require understanding dependency chains
- **State management**: More execution state to track and persist
- **Timeout complexity**: Per-node and overall DAG timeouts interact

### Example Workflow

```
┌──────────┐
│  Extract │
└────┬─────┘
     │
     ▼
┌──────────┐     ┌──────────┐
│Transform │────▶│  Notify  │
│   (A)    │     │ (on any) │
└────┬─────┘     └──────────┘
     │
     ▼
┌──────────┐
│Transform │
│   (B)    │
└────┬─────┘
     │
     ▼
┌──────────┐
│   Load   │
└──────────┘
```

### Job Definition with Dependencies

```json
{
  "name": "transform-step-b",
  "schedule": "",
  "dependencies": {
    "depends_on": ["transform-step-a"],
    "condition": "all_success",
    "timeout": "30m"
  },
  "webhook": {
    "url": "https://etl.example.com/transform/b"
  }
}
```

### DAG Run State

```go
type Run struct {
    ID         string                `json:"id"`
    DAGID      string                `json:"dag_id"`
    Status     RunStatus             `json:"status"`
    NodeStates map[string]*NodeState `json:"node_states"`
    StartedAt  time.Time             `json:"started_at"`
    EndedAt    time.Time             `json:"ended_at,omitempty"`
}

type NodeState struct {
    NodeID      string        `json:"node_id"`
    Status      NodeStatus    `json:"status"`
    ExecutionID string        `json:"execution_id,omitempty"`
    StartedAt   time.Time     `json:"started_at,omitempty"`
    Duration    time.Duration `json:"duration,omitempty"`
    Error       string        `json:"error,omitempty"`
}
```

### Cycle Detection

DAGs must be acyclic. We validate using depth-first search:

```go
func (d *DAG) Validate() error {
    visited := make(map[string]bool)
    recStack := make(map[string]bool)

    var hasCycle func(nodeID string) bool
    hasCycle = func(nodeID string) bool {
        visited[nodeID] = true
        recStack[nodeID] = true

        for _, depID := range d.Nodes[nodeID].Dependencies {
            if !visited[depID] {
                if hasCycle(depID) {
                    return true
                }
            } else if recStack[depID] {
                return true  // Back edge = cycle
            }
        }

        recStack[nodeID] = false
        return false
    }
    // ...
}
```

## References

- [Apache Airflow DAGs](https://airflow.apache.org/docs/apache-airflow/stable/core-concepts/dags.html)
- [Directed Acyclic Graphs](https://en.wikipedia.org/wiki/Directed_acyclic_graph)
- [Topological Sorting](https://en.wikipedia.org/wiki/Topological_sorting)
