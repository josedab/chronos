# Next-Gen Features API Reference

This document provides API reference for Chronos next-generation features.

## Table of Contents

- [Multi-Protocol Dispatch](#multi-protocol-dispatch)
- [Policy-as-Code Engine](#policy-as-code-engine)
- [Visual Workflow Builder](#visual-workflow-builder)
- [AI Schedule Optimization](#ai-schedule-optimization)
- [Cross-Region Federation](#cross-region-federation)
- [Time-Travel Debugging](#time-travel-debugging)
- [Real-Time Collaboration](#real-time-collaboration)
- [Job Marketplace](#job-marketplace)
- [Mobile Push Notifications](#mobile-push-notifications)
- [Cloud Control Plane](#cloud-control-plane)

---

## Multi-Protocol Dispatch

Package: `internal/dispatcher`

### Overview

The multi-dispatcher routes job executions to the appropriate protocol handler based on job configuration.

### Types

#### MultiDispatcher

```go
type MultiDispatcher struct {
    // contains filtered or unexported fields
}

// NewMultiDispatcher creates a dispatcher that routes to HTTP, gRPC, Kafka, NATS, or RabbitMQ
func NewMultiDispatcher(cfg MultiDispatcherConfig) *MultiDispatcher

// ExecuteJob routes the job to the appropriate dispatcher based on configuration
func (m *MultiDispatcher) ExecuteJob(ctx context.Context, job *models.Job) (*models.Execution, error)
```

#### DispatchConfig

```go
// DispatchConfig determines which protocol to use for a job
type DispatchConfig struct {
    Type     DispatchType           `json:"type"`     // "http", "grpc", "kafka", "nats", "rabbitmq"
    HTTP     *models.WebhookConfig  `json:"http,omitempty"`
    GRPC     *GRPCConfig           `json:"grpc,omitempty"`
    Kafka    *KafkaConfig          `json:"kafka,omitempty"`
    NATS     *NATSConfig           `json:"nats,omitempty"`
    RabbitMQ *RabbitMQConfig       `json:"rabbitmq,omitempty"`
}
```

### Protocol-Specific Configurations

#### gRPC

```go
type GRPCConfig struct {
    Address     string            `json:"address"`      // host:port
    Method      string            `json:"method"`       // /package.Service/Method
    TLS         *TLSConfig        `json:"tls,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    Timeout     time.Duration     `json:"timeout"`
    MaxRetries  int               `json:"max_retries"`
}
```

#### Kafka

```go
type KafkaConfig struct {
    Brokers   []string          `json:"brokers"`
    Topic     string            `json:"topic"`
    Key       string            `json:"key,omitempty"`
    Headers   map[string]string `json:"headers,omitempty"`
    Partition *int32            `json:"partition,omitempty"`
    SASL      *SASLConfig       `json:"sasl,omitempty"`
}
```

#### NATS

```go
type NATSConfig struct {
    URL           string        `json:"url"`
    Subject       string        `json:"subject"`
    ReplySubject  string        `json:"reply_subject,omitempty"`
    Headers       map[string]string `json:"headers,omitempty"`
    JetStream     *JetStreamConfig  `json:"jetstream,omitempty"`
}
```

#### RabbitMQ

```go
type RabbitMQConfig struct {
    URL          string            `json:"url"`
    Exchange     string            `json:"exchange"`
    RoutingKey   string            `json:"routing_key"`
    Headers      map[string]string `json:"headers,omitempty"`
    Mandatory    bool              `json:"mandatory"`
    Immediate    bool              `json:"immediate"`
}
```

### Example Usage

```go
dispatcher := dispatcher.NewMultiDispatcher(dispatcher.MultiDispatcherConfig{
    HTTP: httpDispatcher,
    GRPC: grpcDispatcher,
    Kafka: kafkaDispatcher,
})

execution, err := dispatcher.ExecuteJob(ctx, job)
```

---

## Policy-as-Code Engine

Package: `internal/policy`

### Overview

The policy engine evaluates jobs against declarative rules before execution.

### Types

#### Engine

```go
type Engine struct {
    // contains filtered or unexported fields
}

// NewEngine creates a new policy evaluation engine
func NewEngine() *Engine

// AddPolicy registers a policy with the engine
func (e *Engine) AddPolicy(policy *Policy) error

// RemovePolicy removes a policy by name
func (e *Engine) RemovePolicy(name string)

// Evaluate checks a job against all registered policies
func (e *Engine) Evaluate(ctx context.Context, job *models.Job) (*EvaluationResult, error)
```

#### Policy

```go
type Policy struct {
    Name        string  `json:"name"`
    Description string  `json:"description"`
    Enabled     bool    `json:"enabled"`
    Priority    int     `json:"priority"`    // Higher = evaluated first
    Rules       []Rule  `json:"rules"`
    Tags        []string `json:"tags,omitempty"`
}
```

#### Rule

```go
type Rule struct {
    Name      string     `json:"name"`
    Condition *Condition `json:"condition"`
    Action    Action     `json:"action"`   // "allow", "deny", "warn"
    Message   string     `json:"message"`
}
```

#### Condition

```go
type Condition struct {
    Field    string      `json:"field,omitempty"`    // Dot-notation path
    Operator Operator    `json:"operator,omitempty"` // See operators below
    Value    interface{} `json:"value,omitempty"`
    
    // Logical operators
    And []*Condition `json:"and,omitempty"`
    Or  []*Condition `json:"or,omitempty"`
    Not *Condition   `json:"not,omitempty"`
}
```

### Supported Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `equals` | Exact match | `field: "status", value: "active"` |
| `not_equals` | Not equal | `field: "env", value: "prod"` |
| `contains` | String contains | `field: "name", value: "test"` |
| `starts_with` | String prefix | `field: "webhook.url", value: "https://"` |
| `ends_with` | String suffix | `field: "name", value: "-job"` |
| `regex_matches` | Regex match | `field: "name", value: "^[a-z]+$"` |
| `greater_than` | Numeric > | `field: "timeout", value: 60` |
| `less_than` | Numeric < | `field: "retry.max", value: 10` |
| `greater_than_or_equal` | Numeric >= | `field: "priority", value: 5` |
| `less_than_or_equal` | Numeric <= | `field: "timeout", value: 300` |
| `in` | Value in list | `field: "env", value: ["prod", "staging"]` |
| `not_in` | Value not in list | `field: "region", value: ["us-gov"]` |
| `exists` | Field exists | `field: "metadata.owner"` |
| `not_exists` | Field missing | `field: "deprecated"` |
| `is_empty` | Empty string/list | `field: "tags"` |
| `is_not_empty` | Non-empty | `field: "description"` |

### Built-in Policies

```go
// Get pre-defined policies
policies := policy.GetBuiltInPolicies()

// Available built-in policies:
// - HTTPSOnlyPolicy: Require HTTPS for webhooks
// - NamingConventionPolicy: Enforce job naming patterns
// - TimeoutLimitPolicy: Min/max timeout enforcement
// - RetryLimitPolicy: Max retry attempts
// - RequiredLabelsPolicy: Require specific labels
// - ScheduleFrequencyPolicy: Minimum schedule interval
// - WebhookDomainPolicy: Allowlisted domains only
```

### Example Usage

```go
engine := policy.NewEngine()

// Add built-in policies
for _, p := range policy.GetBuiltInPolicies() {
    engine.AddPolicy(p)
}

// Add custom policy
engine.AddPolicy(&policy.Policy{
    Name: "require-owner",
    Rules: []policy.Rule{{
        Name: "owner-label-required",
        Condition: &policy.Condition{
            Field:    "labels.owner",
            Operator: policy.OpExists,
        },
        Action:  policy.ActionDeny,
        Message: "All jobs must have an owner label",
    }},
})

// Evaluate
result, err := engine.Evaluate(ctx, job)
if !result.Allowed {
    for _, v := range result.Violations {
        log.Printf("Policy violation: %s - %s", v.Rule, v.Message)
    }
}
```

---

## Visual Workflow Builder

Package: `internal/dag/builder`

### Overview

Build complex job workflows visually with nodes and edges, then convert to executable DAGs.

### Types

#### WorkflowBuilder

```go
type WorkflowBuilder struct {
    // contains filtered or unexported fields
}

// NewWorkflowBuilder creates a new builder instance
func NewWorkflowBuilder(id, name string) *WorkflowBuilder

// AddNode adds a node to the workflow canvas
func (b *WorkflowBuilder) AddNode(node *WorkflowNode) error

// RemoveNode removes a node and its connected edges
func (b *WorkflowBuilder) RemoveNode(nodeID string) error

// AddEdge connects two nodes
func (b *WorkflowBuilder) AddEdge(edge *WorkflowEdge) error

// Validate checks the workflow for errors
func (b *WorkflowBuilder) Validate() []ValidationError

// ToDAG converts the visual workflow to an executable DAG
func (b *WorkflowBuilder) ToDAG() (*dag.DAG, error)

// Export serializes the workflow to JSON
func (b *WorkflowBuilder) Export() ([]byte, error)

// Import loads a workflow from JSON
func (b *WorkflowBuilder) Import(data []byte) error
```

#### WorkflowNode

```go
type WorkflowNode struct {
    ID       string            `json:"id"`
    Type     NodeType          `json:"type"`
    Name     string            `json:"name"`
    Position Position          `json:"position"`  // Canvas x, y
    Config   map[string]any    `json:"config"`
    Inputs   []string          `json:"inputs"`    // Input port IDs
    Outputs  []string          `json:"outputs"`   // Output port IDs
}

type Position struct {
    X float64 `json:"x"`
    Y float64 `json:"y"`
}
```

#### Node Types

| Type | Description | Config Fields |
|------|-------------|---------------|
| `trigger` | Workflow entry point | `trigger_type`, `schedule`, `webhook_path` |
| `job` | Execute existing job | `job_id`, `timeout`, `retry_policy` |
| `condition` | Conditional branch | `expression`, `true_output`, `false_output` |
| `parallel` | Split to parallel branches | `branches` (output port IDs) |
| `join` | Wait for parallel branches | `join_type` ("all", "any", "n_of_m") |
| `delay` | Wait for duration | `duration`, `until_time` |
| `approval` | Human approval gate | `approvers`, `timeout`, `auto_action` |
| `notification` | Send notification | `channel`, `template`, `recipients` |
| `script` | Execute inline script | `runtime`, `code`, `env` |
| `http` | HTTP request | `url`, `method`, `headers`, `body` |
| `transform` | Data transformation | `expression`, `output_var` |
| `subworkflow` | Nested workflow | `workflow_id`, `inputs` |

#### WorkflowEdge

```go
type WorkflowEdge struct {
    ID         string `json:"id"`
    SourceNode string `json:"source_node"`
    SourcePort string `json:"source_port"`
    TargetNode string `json:"target_node"`
    TargetPort string `json:"target_port"`
    Label      string `json:"label,omitempty"`
    Condition  string `json:"condition,omitempty"`  // For conditional edges
}
```

### Example Usage

```go
builder := dag.NewWorkflowBuilder("wf-1", "Daily ETL Pipeline")

// Add trigger
builder.AddNode(&dag.WorkflowNode{
    ID:   "trigger-1",
    Type: dag.NodeTypeTrigger,
    Name: "Daily 9am",
    Config: map[string]any{
        "trigger_type": "cron",
        "schedule":     "0 9 * * *",
    },
})

// Add extraction job
builder.AddNode(&dag.WorkflowNode{
    ID:   "extract-1",
    Type: dag.NodeTypeJob,
    Name: "Extract Data",
    Config: map[string]any{
        "job_id":  "extract-customers",
        "timeout": "10m",
    },
})

// Connect nodes
builder.AddEdge(&dag.WorkflowEdge{
    ID:         "e1",
    SourceNode: "trigger-1",
    TargetNode: "extract-1",
})

// Validate
if errors := builder.Validate(); len(errors) > 0 {
    // Handle validation errors
}

// Convert to executable DAG
executable, err := builder.ToDAG()
```

---

## AI Schedule Optimization

Package: `internal/smartsched`

### Overview

Analyze execution patterns to detect anomalies and recommend optimal scheduling windows.

### Types

#### AnomalyDetector

```go
type AnomalyDetector struct {
    // contains filtered or unexported fields
}

// NewAnomalyDetector creates a detector with configurable thresholds
func NewAnomalyDetector(config AnomalyConfig) *AnomalyDetector

// RecordExecution records a job execution for analysis
func (d *AnomalyDetector) RecordExecution(jobID string, exec ExecutionRecord)

// DetectAnomalies checks for anomalies in a job's recent executions
func (d *AnomalyDetector) DetectAnomalies(jobID string) []Anomaly

// CalculateOptimalWindows returns recommended execution times
func (d *AnomalyDetector) CalculateOptimalWindows(jobID string) []TimeWindow

// Forecast predicts future execution characteristics
func (d *AnomalyDetector) Forecast(jobID string, horizon time.Duration) *Forecast
```

#### AnomalyConfig

```go
type AnomalyConfig struct {
    DurationThreshold  float64       // Std devs for duration anomaly (default: 2.0)
    FailureThreshold   float64       // Failure rate threshold (default: 0.1)
    MinSamples         int           // Minimum executions before detection (default: 10)
    WindowSize         time.Duration // Analysis window (default: 7 days)
}
```

#### Anomaly

```go
type Anomaly struct {
    Type       AnomalyType `json:"type"`       // "duration", "failure", "pattern"
    Severity   Severity    `json:"severity"`   // "low", "medium", "high", "critical"
    JobID      string      `json:"job_id"`
    Message    string      `json:"message"`
    Value      float64     `json:"value"`      // Observed value
    Expected   float64     `json:"expected"`   // Expected value
    Deviation  float64     `json:"deviation"`  // Standard deviations from mean
    DetectedAt time.Time   `json:"detected_at"`
}
```

#### TimeWindow

```go
type TimeWindow struct {
    Start       time.Time `json:"start"`
    End         time.Time `json:"end"`
    Score       float64   `json:"score"`        // 0-1, higher is better
    SuccessRate float64   `json:"success_rate"` // Historical success rate
    AvgDuration time.Duration `json:"avg_duration"`
    Reason      string    `json:"reason"`       // Why this window is good
}
```

### Example Usage

```go
detector := smartsched.NewAnomalyDetector(smartsched.AnomalyConfig{
    DurationThreshold: 2.5,
    FailureThreshold:  0.15,
    MinSamples:        20,
    WindowSize:        14 * 24 * time.Hour,
})

// Record executions (typically from execution history)
for _, exec := range executionHistory {
    detector.RecordExecution(exec.JobID, smartsched.ExecutionRecord{
        StartTime: exec.StartedAt,
        Duration:  exec.Duration,
        Success:   exec.Status == "completed",
    })
}

// Check for anomalies
anomalies := detector.DetectAnomalies("daily-backup")
for _, a := range anomalies {
    if a.Severity == smartsched.SeverityHigh {
        alertOps(a)
    }
}

// Get scheduling recommendations
windows := detector.CalculateOptimalWindows("daily-backup")
// windows[0] is the best recommended time window
```

---

## Cross-Region Federation

Package: `internal/geo`

### Overview

Synchronize jobs across multiple Chronos clusters in different regions.

### Types

#### FederationManager

```go
type FederationManager struct {
    // contains filtered or unexported fields
}

// NewFederationManager creates a federation coordinator
func NewFederationManager(config FederationConfig) *FederationManager

// Start begins health monitoring and synchronization
func (m *FederationManager) Start(ctx context.Context) error

// Stop gracefully shuts down the federation manager
func (m *FederationManager) Stop() error

// AddCluster registers a remote cluster
func (m *FederationManager) AddCluster(cluster *ClusterInfo) error

// RemoveCluster unregisters a cluster
func (m *FederationManager) RemoveCluster(clusterID string) error

// FederateJob replicates a job to federated clusters
func (m *FederationManager) FederateJob(ctx context.Context, job *models.Job) error

// RouteExecution determines which cluster should execute a job
func (m *FederationManager) RouteExecution(job *models.Job) (*ClusterInfo, error)
```

#### FederationConfig

```go
type FederationConfig struct {
    LocalClusterID     string                `json:"local_cluster_id"`
    LocalRegion        string                `json:"local_region"`
    ConflictResolution ConflictStrategy      `json:"conflict_resolution"`
    SyncInterval       time.Duration         `json:"sync_interval"`
    HealthCheckInterval time.Duration        `json:"health_check_interval"`
    Clusters           []*ClusterInfo        `json:"clusters"`
}
```

#### ClusterInfo

```go
type ClusterInfo struct {
    ID        string            `json:"id"`
    Name      string            `json:"name"`
    Region    string            `json:"region"`
    Endpoint  string            `json:"endpoint"`
    Priority  int               `json:"priority"`
    Labels    map[string]string `json:"labels"`
    Status    ClusterStatus     `json:"status"`
    LastSeen  time.Time         `json:"last_seen"`
}
```

#### Conflict Resolution

```go
type ConflictStrategy string

const (
    // ConflictNewest - most recent modification wins
    ConflictNewest ConflictStrategy = "newest"
    
    // ConflictOwnerWins - originating cluster wins
    ConflictOwnerWins ConflictStrategy = "owner_wins"
    
    // ConflictMerge - merge non-conflicting fields
    ConflictMerge ConflictStrategy = "merge"
)
```

### Example Usage

```go
federation := geo.NewFederationManager(geo.FederationConfig{
    LocalClusterID:     "us-east-1",
    LocalRegion:        "us-east",
    ConflictResolution: geo.ConflictNewest,
    SyncInterval:       30 * time.Second,
    HealthCheckInterval: 10 * time.Second,
})

// Add remote clusters
federation.AddCluster(&geo.ClusterInfo{
    ID:       "us-west-1",
    Region:   "us-west",
    Endpoint: "https://chronos-us-west.example.com",
    Priority: 1,
})

federation.AddCluster(&geo.ClusterInfo{
    ID:       "eu-west-1",
    Region:   "eu-west",
    Endpoint: "https://chronos-eu.example.com",
    Priority: 2,
})

// Start federation
federation.Start(ctx)

// Federate a job across clusters
job := &models.Job{Name: "global-report", ...}
federation.FederateJob(ctx, job)
```

---

## Time-Travel Debugging

Package: `internal/replay`

### Overview

Step through job execution history with breakpoints and variable inspection.

### Types

#### DebugSessionManager

```go
type DebugSessionManager struct {
    // contains filtered or unexported fields
}

// NewDebugSessionManager creates a debug session coordinator
func NewDebugSessionManager() *DebugSessionManager

// CreateSession starts a new debug session for an execution
func (m *DebugSessionManager) CreateSession(executionID string) (*DebugSession, error)

// GetSession retrieves an active debug session
func (m *DebugSessionManager) GetSession(sessionID string) (*DebugSession, error)

// CloseSession ends a debug session
func (m *DebugSessionManager) CloseSession(sessionID string) error
```

#### DebugSession

```go
type DebugSession struct {
    ID          string         `json:"id"`
    ExecutionID string         `json:"execution_id"`
    CurrentStep int            `json:"current_step"`
    TotalSteps  int            `json:"total_steps"`
    Status      SessionStatus  `json:"status"`
    Breakpoints []Breakpoint   `json:"breakpoints"`
    Watches     []WatchExpr    `json:"watches"`
}

// StepForward moves to the next execution step
func (s *DebugSession) StepForward() (*StepResult, error)

// StepBackward moves to the previous step
func (s *DebugSession) StepBackward() (*StepResult, error)

// JumpToStep moves to a specific step
func (s *DebugSession) JumpToStep(step int) (*StepResult, error)

// AddBreakpoint sets a breakpoint
func (s *DebugSession) AddBreakpoint(bp Breakpoint) error

// RemoveBreakpoint removes a breakpoint
func (s *DebugSession) RemoveBreakpoint(id string) error

// AddWatch adds a variable watch expression
func (s *DebugSession) AddWatch(expr string) error

// GetVariables returns variables at current step
func (s *DebugSession) GetVariables() map[string]any

// Export exports the session for sharing
func (s *DebugSession) Export() (*SessionExport, error)
```

#### Breakpoint

```go
type Breakpoint struct {
    ID        string         `json:"id"`
    Type      BreakpointType `json:"type"`
    Enabled   bool           `json:"enabled"`
    Condition string         `json:"condition,omitempty"`
    NodeType  string         `json:"node_type,omitempty"`
    StepNum   int            `json:"step_num,omitempty"`
}

type BreakpointType string

const (
    BreakpointStep      BreakpointType = "step"      // Break at step N
    BreakpointType      BreakpointType = "type"      // Break on node type
    BreakpointCondition BreakpointType = "condition" // Break when expression true
    BreakpointError     BreakpointType = "error"     // Break on any error
)
```

### Example Usage

```go
manager := replay.NewDebugSessionManager()

// Create session for a past execution
session, err := manager.CreateSession("exec-123")

// Add breakpoints
session.AddBreakpoint(replay.Breakpoint{
    Type:     replay.BreakpointError,
    Enabled:  true,
})

session.AddBreakpoint(replay.Breakpoint{
    Type:      replay.BreakpointType,
    NodeType:  "http",
    Enabled:   true,
})

// Add watches
session.AddWatch("response.status_code")
session.AddWatch("job.config.timeout")

// Step through execution
for {
    result, err := session.StepForward()
    if err != nil {
        break
    }
    
    if result.BreakpointHit {
        // Inspect state
        vars := session.GetVariables()
        fmt.Printf("Step %d: %v\n", result.Step, vars)
    }
}
```

---

## Real-Time Collaboration

Package: `internal/realtime`

### Overview

WebSocket-based real-time updates for collaborative editing and live execution monitoring.

### Types

#### Hub

```go
type Hub struct {
    // contains filtered or unexported fields
}

// NewHub creates a WebSocket hub
func NewHub() *Hub

// Run starts the hub's message processing loop
func (h *Hub) Run(ctx context.Context)

// HandleConnection processes a new WebSocket connection
func (h *Hub) HandleConnection(conn WebSocketConn, userID string)

// BroadcastToRoom sends a message to all clients in a room
func (h *Hub) BroadcastToRoom(roomID string, msg *Message)

// BroadcastJobUpdate notifies watchers of job changes
func (h *Hub) BroadcastJobUpdate(jobID string, update *JobUpdate)

// BroadcastExecutionUpdate notifies watchers of execution progress
func (h *Hub) BroadcastExecutionUpdate(execID string, update *ExecutionUpdate)
```

#### Message Types

```go
type MessageType string

const (
    // Room management
    MsgJoinRoom       MessageType = "join_room"
    MsgLeaveRoom      MessageType = "leave_room"
    
    // Presence
    MsgPresenceUpdate MessageType = "presence_update"
    MsgCursorMove     MessageType = "cursor_move"
    MsgSelectionChange MessageType = "selection_change"
    
    // Live updates
    MsgJobUpdate      MessageType = "job_update"
    MsgExecutionStart MessageType = "execution_start"
    MsgExecutionProgress MessageType = "execution_progress"
    MsgExecutionComplete MessageType = "execution_complete"
    
    // Collaboration
    MsgTypingStart    MessageType = "typing_start"
    MsgTypingStop     MessageType = "typing_stop"
    MsgEditConflict   MessageType = "edit_conflict"
)
```

#### Client Message Format

```json
{
    "type": "join_room",
    "room_id": "job:daily-backup",
    "payload": {}
}
```

#### Server Message Format

```json
{
    "type": "presence_update",
    "room_id": "job:daily-backup",
    "payload": {
        "users": [
            {"id": "user-1", "name": "Alice", "cursor": {"x": 100, "y": 200}},
            {"id": "user-2", "name": "Bob", "cursor": {"x": 150, "y": 250}}
        ]
    },
    "timestamp": "2024-01-15T10:30:00Z"
}
```

### Example Server Integration

```go
hub := realtime.NewHub()
go hub.Run(ctx)

// WebSocket upgrade handler
http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    
    userID := getUserFromRequest(r)
    hub.HandleConnection(conn, userID)
})

// Broadcast job updates from other handlers
func (h *JobHandler) UpdateJob(ctx context.Context, job *models.Job) error {
    // ... save job ...
    
    // Notify connected clients
    hub.BroadcastJobUpdate(job.ID, &realtime.JobUpdate{
        Type:   "updated",
        Job:    job,
        UserID: ctx.Value("user_id").(string),
    })
    
    return nil
}
```

---

## Job Marketplace

Package: `internal/marketplace`

### Overview

Pre-built job templates for common use cases.

### Available Templates

| Template | Description |
|----------|-------------|
| `slack-notification` | Send Slack messages |
| `postgres-backup` | PostgreSQL database backup |
| `mysql-backup` | MySQL database backup |
| `mongodb-backup` | MongoDB database backup |
| `http-health-check` | HTTP endpoint monitoring |
| `tcp-health-check` | TCP port monitoring |
| `dns-health-check` | DNS resolution check |
| `github-actions-trigger` | Trigger GitHub workflow |
| `datadog-metric` | Submit Datadog metrics |
| `pagerduty-heartbeat` | PagerDuty heartbeat |
| `s3-cleanup` | S3 bucket cleanup |
| `docker-cleanup` | Docker image cleanup |
| `ssl-cert-check` | SSL certificate monitoring |
| `elasticsearch-index` | ES index management |
| `redis-cache-warm` | Redis cache warming |

### Types

#### TemplateRegistry

```go
type TemplateRegistry struct {
    // contains filtered or unexported fields
}

// NewTemplateRegistry creates a registry with built-in templates
func NewTemplateRegistry() *TemplateRegistry

// GetTemplate retrieves a template by ID
func (r *TemplateRegistry) GetTemplate(id string) (*JobTemplate, error)

// ListTemplates returns all available templates
func (r *TemplateRegistry) ListTemplates() []*JobTemplate

// ListByCategory returns templates in a category
func (r *TemplateRegistry) ListByCategory(category string) []*JobTemplate

// InstantiateTemplate creates a job from a template
func (r *TemplateRegistry) InstantiateTemplate(id string, vars map[string]string) (*models.Job, error)
```

#### JobTemplate

```go
type JobTemplate struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Category    string            `json:"category"`
    Version     string            `json:"version"`
    Variables   []TemplateVar     `json:"variables"`
    JobSpec     *models.Job       `json:"job_spec"`
}

type TemplateVar struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Required    bool   `json:"required"`
    Default     string `json:"default,omitempty"`
    Type        string `json:"type"` // "string", "number", "boolean", "secret"
}
```

### Example Usage

```go
registry := marketplace.NewTemplateRegistry()

// List available templates
templates := registry.ListTemplates()
for _, t := range templates {
    fmt.Printf("%s: %s\n", t.ID, t.Description)
}

// Instantiate a template
job, err := registry.InstantiateTemplate("slack-notification", map[string]string{
    "webhook_url": "https://hooks.slack.com/services/...",
    "channel":     "#alerts",
    "message":     "Daily report completed",
})

// job is ready to be saved and scheduled
```

---

## Mobile Push Notifications

Package: `internal/mobile`

### Overview

Push notification service for mobile companion apps.

### Types

#### PushService

```go
type PushService struct {
    // contains filtered or unexported fields
}

// NewPushService creates a push notification service
func NewPushService(config PushConfig) *PushService

// RegisterDevice registers a device for push notifications
func (s *PushService) RegisterDevice(device *DeviceRegistration) error

// UnregisterDevice removes a device
func (s *PushService) UnregisterDevice(deviceID string) error

// UpdatePreferences updates notification preferences
func (s *PushService) UpdatePreferences(deviceID string, prefs *NotificationPrefs) error

// SendNotification sends a notification to a user's devices
func (s *PushService) SendNotification(userID string, notif *Notification) error

// SendJobAlert sends a job-related notification
func (s *PushService) SendJobAlert(userID string, jobID string, alertType AlertType, message string) error
```

#### DeviceRegistration

```go
type DeviceRegistration struct {
    DeviceID    string   `json:"device_id"`
    UserID      string   `json:"user_id"`
    Platform    Platform `json:"platform"`   // "ios", "android"
    PushToken   string   `json:"push_token"`
    AppVersion  string   `json:"app_version"`
    DeviceModel string   `json:"device_model"`
}
```

#### NotificationPrefs

```go
type NotificationPrefs struct {
    JobCompleted     bool     `json:"job_completed"`
    JobFailed        bool     `json:"job_failed"`
    ApprovalRequired bool     `json:"approval_required"`
    SystemAlerts     bool     `json:"system_alerts"`
    QuietHoursStart  string   `json:"quiet_hours_start,omitempty"` // "22:00"
    QuietHoursEnd    string   `json:"quiet_hours_end,omitempty"`   // "07:00"
    EnabledJobs      []string `json:"enabled_jobs,omitempty"`      // Specific job IDs
}
```

#### Alert Types

```go
type AlertType string

const (
    AlertJobCompleted   AlertType = "job_completed"
    AlertJobFailed      AlertType = "job_failed"
    AlertJobTimeout     AlertType = "job_timeout"
    AlertApprovalNeeded AlertType = "approval_needed"
    AlertAnomalyDetected AlertType = "anomaly_detected"
    AlertClusterHealth  AlertType = "cluster_health"
)
```

### Example Usage

```go
pushService := mobile.NewPushService(mobile.PushConfig{
    APNs: &mobile.APNsConfig{
        KeyID:    "...",
        TeamID:   "...",
        BundleID: "com.chronos.app",
    },
    FCM: &mobile.FCMConfig{
        ProjectID: "chronos-mobile",
    },
})

// Register device
pushService.RegisterDevice(&mobile.DeviceRegistration{
    DeviceID:  "device-abc",
    UserID:    "user-123",
    Platform:  mobile.PlatformIOS,
    PushToken: "apns-token-...",
})

// Set preferences
pushService.UpdatePreferences("device-abc", &mobile.NotificationPrefs{
    JobCompleted:     true,
    JobFailed:        true,
    ApprovalRequired: true,
    QuietHoursStart:  "22:00",
    QuietHoursEnd:    "07:00",
})

// Send alert (called from job execution handler)
pushService.SendJobAlert("user-123", "daily-backup", mobile.AlertJobFailed, 
    "Daily backup failed: connection timeout")
```

---

## Cloud Control Plane

Package: `pkg/cloud`

### Overview

Multi-tenant control plane for managed Chronos clusters.

### Types

#### ControlPlane

```go
type ControlPlane struct {
    // contains filtered or unexported fields
}

// NewControlPlane creates a control plane instance
func NewControlPlane(config ControlPlaneConfig) *ControlPlane

// CreateTenant provisions a new tenant
func (cp *ControlPlane) CreateTenant(ctx context.Context, req *CreateTenantRequest) (*Tenant, error)

// GetTenant retrieves tenant details
func (cp *ControlPlane) GetTenant(ctx context.Context, tenantID string) (*Tenant, error)

// UpdateTenant modifies tenant settings
func (cp *ControlPlane) UpdateTenant(ctx context.Context, tenantID string, req *UpdateTenantRequest) (*Tenant, error)

// CreateCluster provisions a new cluster for a tenant
func (cp *ControlPlane) CreateCluster(ctx context.Context, tenantID string, req *CreateClusterRequest) (*Cluster, error)

// GetUsage retrieves usage metrics for billing
func (cp *ControlPlane) GetUsage(ctx context.Context, tenantID string, period UsagePeriod) (*Usage, error)

// CheckQuota validates quota availability
func (cp *ControlPlane) CheckQuota(ctx context.Context, tenantID string, resource string, amount int) (bool, error)
```

#### Tenant

```go
type Tenant struct {
    ID           string            `json:"id"`
    Name         string            `json:"name"`
    Slug         string            `json:"slug"`
    Plan         Plan              `json:"plan"`
    Status       TenantStatus      `json:"status"`
    Quotas       *Quotas           `json:"quotas"`
    Usage        *Usage            `json:"usage"`
    Clusters     []*Cluster        `json:"clusters"`
    CreatedAt    time.Time         `json:"created_at"`
}
```

#### Plans and Quotas

```go
type Plan string

const (
    PlanFree       Plan = "free"
    PlanStarter    Plan = "starter"
    PlanPro        Plan = "pro"
    PlanEnterprise Plan = "enterprise"
)

type Quotas struct {
    MaxJobs              int           `json:"max_jobs"`
    MaxExecutionsPerMonth int          `json:"max_executions_per_month"`
    MaxClusters          int           `json:"max_clusters"`
    RetentionDays        int           `json:"retention_days"`
    MaxConcurrentJobs    int           `json:"max_concurrent_jobs"`
}

// Default quotas per plan
var PlanQuotas = map[Plan]*Quotas{
    PlanFree:       {MaxJobs: 10, MaxExecutionsPerMonth: 1000, MaxClusters: 1, RetentionDays: 1},
    PlanStarter:    {MaxJobs: 100, MaxExecutionsPerMonth: 50000, MaxClusters: 2, RetentionDays: 7},
    PlanPro:        {MaxJobs: 1000, MaxExecutionsPerMonth: 500000, MaxClusters: 5, RetentionDays: 30},
    PlanEnterprise: {MaxJobs: -1, MaxExecutionsPerMonth: -1, MaxClusters: -1, RetentionDays: 90}, // -1 = unlimited
}
```

#### Cluster

```go
type Cluster struct {
    ID        string            `json:"id"`
    TenantID  string            `json:"tenant_id"`
    Name      string            `json:"name"`
    Region    string            `json:"region"`
    Provider  CloudProvider     `json:"provider"`  // "aws", "gcp", "azure"
    Size      ClusterSize       `json:"size"`
    Status    ClusterStatus     `json:"status"`
    Endpoint  string            `json:"endpoint"`
    CreatedAt time.Time         `json:"created_at"`
}

type ClusterSize string

const (
    ClusterSizeSmall  ClusterSize = "small"   // 1 node
    ClusterSizeMedium ClusterSize = "medium"  // 3 nodes
    ClusterSizeLarge  ClusterSize = "large"   // 5 nodes
)
```

### Example Usage

```go
cp := cloud.NewControlPlane(cloud.ControlPlaneConfig{
    DatabaseURL: "postgres://...",
    Providers: map[cloud.CloudProvider]cloud.ProviderConfig{
        cloud.ProviderAWS: {Region: "us-east-1"},
        cloud.ProviderGCP: {Project: "chronos-prod"},
    },
})

// Create tenant
tenant, err := cp.CreateTenant(ctx, &cloud.CreateTenantRequest{
    Name:  "Acme Corp",
    Email: "admin@acme.com",
    Plan:  cloud.PlanPro,
})

// Provision cluster
cluster, err := cp.CreateCluster(ctx, tenant.ID, &cloud.CreateClusterRequest{
    Name:     "production",
    Region:   "us-east-1",
    Provider: cloud.ProviderAWS,
    Size:     cloud.ClusterSizeMedium,
})

// Check quota before creating job
allowed, err := cp.CheckQuota(ctx, tenant.ID, "jobs", 1)
if !allowed {
    return errors.New("job quota exceeded")
}

// Get usage for billing
usage, err := cp.GetUsage(ctx, tenant.ID, cloud.UsagePeriodMonth)
fmt.Printf("Executions this month: %d\n", usage.Executions)
```

---

## Questions / Clarifications

1. **Message Queue Integration**: The Kafka, NATS, and RabbitMQ dispatchers have simulated publish functions. Should actual client library integrations (sarama, nats.go, amqp091-go) be added?

2. **Push Notification Providers**: The mobile package defines a provider interface. Should APNs and FCM implementations be fully implemented?

3. **Control Plane Persistence**: Currently uses in-memory storage. Should PostgreSQL schema and migrations be added?

4. **WebSocket Library**: The realtime hub defines a `WebSocketConn` interface. Should gorilla/websocket integration be included?

5. **Policy Language**: Should OPA/Rego support be added alongside the built-in condition language?
