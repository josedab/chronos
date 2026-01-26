# Chronos: Distributed Cron System
## Product Requirements Document

**Version:** 1.0
**Last Updated:** December 2024
**Status:** Draft
**Language:** Go

---

## Executive Summary

Chronos is a distributed cron system that provides reliable job scheduling without operational complexity. Current solutions force teams to choose between simplicity with reliability trade-offs (Kubernetes CronJobs) or powerful features with operational burden (Temporal, Airflow). Chronos delivers dead-simple distributed scheduling: single binary, embedded storage, automatic leader election, and comprehensive observability.

Scheduling recurring jobs in distributed systems remains surprisingly hard. Kubernetes CronJobs have documented limitations—the 100-missed-jobs failure mode that can permanently halt jobs, lack of execution history, and no built-in deduplication. Enterprise solutions like Temporal and Airflow require running distributed databases and complex infrastructure stacks.

Chronos addresses this gap with a Go-native approach: Raft-based consensus using the battle-tested hashicorp/raft library, embedded BadgerDB for state, and HTTP webhook dispatch for language-agnostic job execution.

**Target Market:** Backend engineers, DevOps teams, and platform engineers scheduling periodic jobs at 10-10,000 scale without dedicated infrastructure teams.

**Primary Differentiator:** Zero-dependency reliability—deploy a 3-node cluster and forget about scheduling infrastructure.

---

## Market Analysis

### Total Addressable Market (TAM)

The workflow orchestration and automation market is valued at **$6.1-7.73 billion in 2024**, projected to reach **$18-21 billion by 2030** (CAGR 15-18%). Job scheduling represents approximately 15-20% of this market.

**TAM Estimate:** $1.0-1.5 billion annually for job scheduling solutions

### Serviceable Addressable Market (SAM)

Organizations needing reliable distributed scheduling:
- **Companies with 10-500 scheduled jobs**: ~200,000 globally
- **Average spend on scheduling infrastructure**: $5,000-$50,000/year
- **SAM estimate**: $500 million annually

### Serviceable Obtainable Market (SOM)

Open-source distributed cron targeting:
- Teams outgrowing Kubernetes CronJobs
- Organizations wanting Temporal-like reliability without complexity
- Startups needing scheduling without dedicated ops

**Year 1 Target:** 1,000 production deployments
**Year 3 Target:** 10,000 deployments with 15% enterprise conversion

### Market Trends

1. **Kubernetes CronJob Limitations**: Growing awareness of reliability issues driving alternatives
2. **Serverless Schedulers**: AWS EventBridge, Cloud Scheduler creating category awareness
3. **GitOps for Everything**: Infrastructure-as-code for scheduling configurations
4. **Observability Requirements**: Teams demanding execution visibility and debugging tools

---

## Problem Statement

### Primary Problem

Scheduling recurring jobs in distributed systems has fundamental challenges that existing solutions don't adequately address:

**Kubernetes CronJobs Issues:**
- **100-missed-jobs failure**: If a CronJob misses 100 consecutive scheduled times, it stops scheduling entirely
- **No distributed locking**: Race conditions with cluster-wide scheduling
- **Limited visibility**: No built-in execution history or job status tracking
- **No timezone support**: UTC-only scheduling creates operational friction
- **No dependencies**: Can't express job ordering or DAG relationships

**Enterprise Solutions (Temporal, Airflow):**
- **Operational complexity**: Require running PostgreSQL/MySQL, Redis, multiple components
- **Steep learning curve**: SDK concepts (activities, workflows, DAGs) require significant investment
- **Resource overhead**: Minimum deployments require significant compute
- **Heavyweight for simple jobs**: Using Temporal for "run report at 9am" is overkill

### Secondary Problems

1. **Single Point of Failure**: Traditional cron on VMs creates availability gaps
2. **Clock Drift**: Distributed systems have inconsistent time views
3. **Execution Guarantees**: At-least-once vs exactly-once confusion
4. **Debugging Difficulty**: "Did my job run?" requires log archaeology
5. **Secret Management**: Cron jobs need credentials but shouldn't store them

### User Impact

| User | Impact |
|------|--------|
| Backend Engineers | 5-10 hours/month debugging failed jobs, manual retries |
| DevOps Engineers | Operating multiple scheduling systems, inconsistent monitoring |
| On-Call Engineers | Alert fatigue from transient failures, unclear escalation paths |
| Platform Teams | Building custom solutions, maintaining scheduling infrastructure |

---

## Target Users

### User Segments

| Segment | Description | Size | Priority |
|---------|-------------|------|----------|
| DevOps Engineers | Replacing brittle crontabs | ~500,000 globally | P0 |
| Backend Engineers | Scheduling periodic jobs | ~2M globally | P0 |
| Platform Teams | Providing scheduling-as-a-service | ~50,000 teams | P1 |
| Startups | Need reliability without ops burden | ~500,000 companies | P1 |

### User Personas

#### Primary Persona: Sam (DevOps Engineer)
- **Background**: 4 years experience, manages Kubernetes infrastructure for 20-person team
- **Current State**: Kubernetes CronJobs for 50 scheduled tasks, has experienced missed-job failures
- **Pain Points**: Jobs silently fail, no execution history, can't see what's scheduled
- **Goals**: Reliable scheduling, visibility, alerting on failures
- **Quote**: "I don't want to think about cron. I want it to just work."

#### Secondary Persona: Alex (Backend Engineer)
- **Background**: 3 years experience, Python/Go developer
- **Current State**: Uses Celery Beat for scheduling, frustrated with setup complexity
- **Pain Points**: Complex distributed setup, Redis dependency, unclear failure modes
- **Goals**: Simple API to schedule HTTP calls, see execution logs
- **Quote**: "I just want to schedule a webhook. Why is this so complicated?"

#### Tertiary Persona: Morgan (Platform Engineer)
- **Background**: 6 years experience, building internal developer platform
- **Current State**: Evaluated Temporal, considered too complex for team's needs
- **Pain Points**: Enterprise solutions require dedicated expertise, cost prohibitive
- **Goals**: Provide scheduling capability to developers, multi-tenant isolation
- **Quote**: "Our developers need scheduling but can't all learn Temporal."

---

## Competitive Analysis

### Direct Competitors

| Competitor | Strengths | Weaknesses | Market Position |
|------------|-----------|------------|-----------------|
| **Kubernetes CronJobs** | Native K8s, zero setup | Reliability issues, limited features | Default choice |
| **Temporal** | Powerful workflows, durable | Complex, heavy infrastructure | Enterprise standard |
| **Airflow** | DAG support, Python ecosystem | Heavy, complex UI | Data engineering |
| **Dkron** | Go-based, Raft consensus | Less active development | Niche |
| **Jobrunr** | JVM integration | Java-only | JVM ecosystem |

### Feature Comparison

| Feature | Chronos | K8s CronJob | Temporal | Airflow | Dkron |
|---------|---------|-------------|----------|---------|-------|
| Distributed consensus | ✅ Raft | ❌ | ✅ | ❌ | ✅ |
| Embedded storage | ✅ | N/A | ❌ | ❌ | ❌ |
| At-least-once guarantee | ✅ | ❌ | ✅ | ✅ | ✅ |
| Web UI | ✅ | ❌ | ✅ | ✅ | ✅ |
| Job dependencies | P1 | ❌ | ✅ | ✅ | ❌ |
| Timezone support | ✅ | ❌ | ✅ | ✅ | ✅ |
| Single binary | ✅ | N/A | ❌ | ❌ | ✅ |
| External DB required | ❌ | N/A | ✅ | ✅ | ❌ |

### Market Positioning

**Position:** "Scheduling that just works—no infrastructure required"

Chronos occupies the space between Kubernetes CronJobs (too simple, unreliable) and Temporal/Airflow (too complex). For teams with 10-1,000 scheduled jobs that need reliability without operational burden.

**Competitive Moat:**
1. Zero external dependencies (embedded storage, built-in consensus)
2. 15-minute deployment to production-ready
3. Go-native performance and simplicity
4. Language-agnostic HTTP dispatch

---

## Product Vision

### Vision Statement

*"Make scheduled jobs as reliable as database transactions—fire and forget with complete confidence."*

### Strategic Objectives

1. **Reliability First**: At-least-once execution guarantees, never miss a scheduled job
2. **Zero Infrastructure**: Single binary, no external dependencies
3. **Operational Visibility**: Always know what ran, when, and whether it succeeded
4. **Developer Experience**: Schedule jobs in minutes, not hours

### Design Principles

1. **Fail Loud**: If something goes wrong, make it obvious immediately
2. **Sensible Defaults**: Works great out of the box, customizable when needed
3. **Language Agnostic**: HTTP webhooks work with any stack
4. **Testable**: Easy to verify scheduling behavior in development
5. **Observable**: Built-in metrics, logs, and tracing

---

## Technical Architecture

### System Overview

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           CHRONOS CLUSTER                                     │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│   ┌───────────────────────────────────────────────────────────────────────┐  │
│   │                         NODE TOPOLOGY                                  │  │
│   │                                                                        │  │
│   │    ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐        │  │
│   │    │     NODE 1      │ │     NODE 2      │ │     NODE 3      │        │  │
│   │    │    (LEADER)     │ │   (FOLLOWER)    │ │   (FOLLOWER)    │        │  │
│   │    │                 │ │                 │ │                 │        │  │
│   │    │  ┌───────────┐  │ │  ┌───────────┐  │ │  ┌───────────┐  │        │  │
│   │    │  │ Scheduler │  │ │  │ Scheduler │  │ │  │ Scheduler │  │        │  │
│   │    │  │  (active) │  │ │  │ (standby) │  │ │  │ (standby) │  │        │  │
│   │    │  └───────────┘  │ │  └───────────┘  │ │  └───────────┘  │        │  │
│   │    │  ┌───────────┐  │ │  ┌───────────┐  │ │  ┌───────────┐  │        │  │
│   │    │  │   Raft    │◄─┼─┼──│   Raft    │◄─┼─┼──│   Raft    │  │        │  │
│   │    │  │ Consensus │──┼─┼─►│ Consensus │──┼─┼─►│ Consensus │  │        │  │
│   │    │  └───────────┘  │ │  └───────────┘  │ │  └───────────┘  │        │  │
│   │    │  ┌───────────┐  │ │  ┌───────────┐  │ │  ┌───────────┐  │        │  │
│   │    │  │ BadgerDB  │  │ │  │ BadgerDB  │  │ │  │ BadgerDB  │  │        │  │
│   │    │  │  (state)  │  │ │  │ (replica) │  │ │  │ (replica) │  │        │  │
│   │    │  └───────────┘  │ │  └───────────┘  │ │  └───────────┘  │        │  │
│   │    └─────────────────┘ └─────────────────┘ └─────────────────┘        │  │
│   │                                                                        │  │
│   └───────────────────────────────────────────────────────────────────────┘  │
│                                                                               │
│   ┌───────────────────────────────────────────────────────────────────────┐  │
│   │                         CORE COMPONENTS                                │  │
│   │                                                                        │  │
│   │   ┌─────────────────────────────────────────────────────────────┐     │  │
│   │   │                    SCHEDULER ENGINE                          │     │  │
│   │   │                                                              │     │  │
│   │   │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐    │     │  │
│   │   │  │ Cron Parser │ │ Next-Run    │ │ Execution Planner   │    │     │  │
│   │   │  │             │ │ Calculator  │ │                     │    │     │  │
│   │   │  │ • Standard  │ │             │ │ • Window detection  │    │     │  │
│   │   │  │ • Extended  │ │ • Timezone  │ │ • Overlap handling  │    │     │  │
│   │   │  │ • Intervals │ │ • DST aware │ │ • Miss recovery     │    │     │  │
│   │   │  └─────────────┘ └─────────────┘ └─────────────────────┘    │     │  │
│   │   └─────────────────────────────────────────────────────────────┘     │  │
│   │                                                                        │  │
│   │   ┌─────────────────────────────────────────────────────────────┐     │  │
│   │   │                    JOB DISPATCHER                            │     │  │
│   │   │                                                              │     │  │
│   │   │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐    │     │  │
│   │   │  │ HTTP Client │ │ gRPC Client │ │ Retry Manager       │    │     │  │
│   │   │  │             │ │             │ │                     │    │     │  │
│   │   │  │ • Webhooks  │ │ • Internal  │ │ • Exponential       │    │     │  │
│   │   │  │ • Timeouts  │ │   services  │ │   backoff           │    │     │  │
│   │   │  │ • Auth      │ │ • Streaming │ │ • Max attempts      │    │     │  │
│   │   │  └─────────────┘ └─────────────┘ └─────────────────────┘    │     │  │
│   │   └─────────────────────────────────────────────────────────────┘     │  │
│   │                                                                        │  │
│   │   ┌─────────────────────────────────────────────────────────────┐     │  │
│   │   │                    DISTRIBUTED LOCK                          │     │  │
│   │   │                                                              │     │  │
│   │   │  ┌─────────────────────────────────────────────────────┐    │     │  │
│   │   │  │              Raft-Based Job Locking                  │    │     │  │
│   │   │  │  • Acquire lock before execution                     │    │     │  │
│   │   │  │  • Leader-only scheduling                            │    │     │  │
│   │   │  │  • Automatic lock release on completion              │    │     │  │
│   │   │  │  • Stale lock detection and recovery                 │    │     │  │
│   │   │  └─────────────────────────────────────────────────────┘    │     │  │
│   │   └─────────────────────────────────────────────────────────────┘     │  │
│   │                                                                        │  │
│   └───────────────────────────────────────────────────────────────────────┘  │
│                                                                               │
│   ┌───────────────────────────────────────────────────────────────────────┐  │
│   │                         DATA MODEL                                     │  │
│   │                                                                        │  │
│   │   ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐   │  │
│   │   │   Job Store     │  │ Execution Store │  │   Schedule Store    │   │  │
│   │   │                 │  │                 │  │                     │   │  │
│   │   │ • Job ID        │  │ • Execution ID  │  │ • Next run time     │   │  │
│   │   │ • Cron expr     │  │ • Job ID        │  │ • Last run time     │   │  │
│   │   │ • Webhook URL   │  │ • Started at    │  │ • Miss count        │   │  │
│   │   │ • Payload       │  │ • Completed at  │  │ • Status            │   │  │
│   │   │ • Headers       │  │ • Status        │  │                     │   │  │
│   │   │ • Retry policy  │  │ • Response      │  │                     │   │  │
│   │   │ • Timezone      │  │ • Attempts      │  │                     │   │  │
│   │   │ • Enabled       │  │ • Error         │  │                     │   │  │
│   │   └─────────────────┘  └─────────────────┘  └─────────────────────┘   │  │
│   │                                                                        │  │
│   └───────────────────────────────────────────────────────────────────────┘  │
│                                                                               │
│   ┌───────────────────────────────────────────────────────────────────────┐  │
│   │                         INTERFACES                                     │  │
│   │                                                                        │  │
│   │    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌───────────┐   │  │
│   │    │  REST API   │  │  Web UI     │  │  CLI        │  │ Prometheus│   │  │
│   │    │   :8080     │  │   :8080     │  │  chronosctl │  │  /metrics │   │  │
│   │    └─────────────┘  └─────────────┘  └─────────────┘  └───────────┘   │  │
│   │                                                                        │  │
│   └───────────────────────────────────────────────────────────────────────┘  │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Core Components

#### 1. Raft Consensus Layer

```go
// RaftNode manages distributed consensus
type RaftNode struct {
    raft        *raft.Raft
    fsm         *JobFSM
    transport   *raft.NetworkTransport
    config      *raft.Config

    leaderCh    <-chan bool
    shutdownCh  chan struct{}
}

// NewRaftNode creates a new Raft node
func NewRaftNode(config *Config, fsm *JobFSM) (*RaftNode, error) {
    raftConfig := raft.DefaultConfig()
    raftConfig.LocalID = raft.ServerID(config.NodeID)
    raftConfig.HeartbeatTimeout = 500 * time.Millisecond
    raftConfig.ElectionTimeout = 1 * time.Second
    raftConfig.LeaderLeaseTimeout = 500 * time.Millisecond

    transport, err := raft.NewTCPTransport(
        config.RaftAddress,
        nil,
        3,
        10*time.Second,
        os.Stderr,
    )
    if err != nil {
        return nil, err
    }

    snapshots, err := raft.NewFileSnapshotStore(
        config.DataDir,
        2,
        os.Stderr,
    )
    if err != nil {
        return nil, err
    }

    logStore, err := raftbadger.NewBadgerStore(config.DataDir + "/raft-log")
    if err != nil {
        return nil, err
    }

    stableStore, err := raftbadger.NewBadgerStore(config.DataDir + "/raft-stable")
    if err != nil {
        return nil, err
    }

    r, err := raft.NewRaft(
        raftConfig,
        fsm,
        logStore,
        stableStore,
        snapshots,
        transport,
    )
    if err != nil {
        return nil, err
    }

    return &RaftNode{
        raft:      r,
        fsm:       fsm,
        transport: transport,
        config:    raftConfig,
        leaderCh:  r.LeaderCh(),
    }, nil
}

// JobFSM implements raft.FSM for job state
type JobFSM struct {
    store *BadgerStore
    mu    sync.RWMutex
}

// Apply applies a Raft log entry to the FSM
func (f *JobFSM) Apply(log *raft.Log) interface{} {
    var cmd Command
    if err := json.Unmarshal(log.Data, &cmd); err != nil {
        return err
    }

    f.mu.Lock()
    defer f.mu.Unlock()

    switch cmd.Type {
    case CmdCreateJob:
        return f.applyCreateJob(cmd.Payload)
    case CmdUpdateJob:
        return f.applyUpdateJob(cmd.Payload)
    case CmdDeleteJob:
        return f.applyDeleteJob(cmd.Payload)
    case CmdRecordExecution:
        return f.applyRecordExecution(cmd.Payload)
    case CmdAcquireLock:
        return f.applyAcquireLock(cmd.Payload)
    case CmdReleaseLock:
        return f.applyReleaseLock(cmd.Payload)
    }
    return nil
}

// Command types
type CommandType int

const (
    CmdCreateJob CommandType = iota
    CmdUpdateJob
    CmdDeleteJob
    CmdRecordExecution
    CmdAcquireLock
    CmdReleaseLock
)
```

#### 2. Scheduler Engine

```go
// Scheduler manages job scheduling and execution
type Scheduler struct {
    raft       *RaftNode
    store      *Store
    dispatcher *Dispatcher

    jobs       map[string]*Job
    nextRuns   *priorityqueue.PriorityQueue

    ticker     *time.Ticker
    mu         sync.RWMutex

    metrics    *SchedulerMetrics
}

// Job represents a scheduled job
type Job struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Schedule    string            `json:"schedule"` // Cron expression
    Timezone    string            `json:"timezone"`

    // Execution config
    Webhook     *WebhookConfig    `json:"webhook,omitempty"`
    GRPC        *GRPCConfig       `json:"grpc,omitempty"`

    // Retry policy
    RetryPolicy *RetryPolicy      `json:"retry_policy"`

    // Execution constraints
    Timeout     time.Duration     `json:"timeout"`
    Concurrency ConcurrencyPolicy `json:"concurrency"`

    // Metadata
    Tags        map[string]string `json:"tags"`
    Enabled     bool              `json:"enabled"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
}

// WebhookConfig defines HTTP webhook execution
type WebhookConfig struct {
    URL         string            `json:"url"`
    Method      string            `json:"method"`
    Headers     map[string]string `json:"headers"`
    Body        string            `json:"body"`
    Auth        *AuthConfig       `json:"auth,omitempty"`
    SuccessCodes []int            `json:"success_codes"` // Default: 200-299
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
    MaxAttempts     int           `json:"max_attempts"`
    InitialInterval time.Duration `json:"initial_interval"`
    MaxInterval     time.Duration `json:"max_interval"`
    Multiplier      float64       `json:"multiplier"`
}

// ConcurrencyPolicy handles overlapping executions
type ConcurrencyPolicy string

const (
    ConcurrencyAllow   ConcurrencyPolicy = "allow"   // Allow concurrent runs
    ConcurrencyForbid  ConcurrencyPolicy = "forbid"  // Skip if already running
    ConcurrencyReplace ConcurrencyPolicy = "replace" // Cancel existing, start new
)

// Run starts the scheduler
func (s *Scheduler) Run(ctx context.Context) error {
    s.ticker = time.NewTicker(1 * time.Second)
    defer s.ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case isLeader := <-s.raft.LeaderCh():
            if isLeader {
                s.becomeLeader()
            } else {
                s.becomeFollower()
            }
        case <-s.ticker.C:
            if s.raft.IsLeader() {
                s.tick()
            }
        }
    }
}

// tick checks for jobs that need execution
func (s *Scheduler) tick() {
    now := time.Now()

    for {
        item, ok := s.nextRuns.Peek()
        if !ok {
            break
        }

        scheduled := item.(*ScheduledRun)
        if scheduled.NextRun.After(now) {
            break
        }

        // Remove from queue
        s.nextRuns.Pop()

        // Execute job
        go s.executeJob(scheduled.Job, scheduled.NextRun)

        // Schedule next run
        nextRun := s.calculateNextRun(scheduled.Job, now)
        s.nextRuns.Push(&ScheduledRun{
            Job:     scheduled.Job,
            NextRun: nextRun,
        })
    }
}

// calculateNextRun computes the next execution time
func (s *Scheduler) calculateNextRun(job *Job, from time.Time) time.Time {
    schedule, err := cron.ParseStandard(job.Schedule)
    if err != nil {
        // Extended syntax
        schedule, _ = cron.Parse(job.Schedule)
    }

    loc := time.UTC
    if job.Timezone != "" {
        loc, _ = time.LoadLocation(job.Timezone)
    }

    return schedule.Next(from.In(loc)).UTC()
}
```

#### 3. Job Dispatcher

```go
// Dispatcher executes jobs
type Dispatcher struct {
    httpClient *http.Client
    grpcPool   *GRPCPool
    store      *Store
    metrics    *DispatcherMetrics
}

// Execute runs a job
func (d *Dispatcher) Execute(ctx context.Context, job *Job, scheduledTime time.Time) (*Execution, error) {
    execution := &Execution{
        ID:            uuid.New().String(),
        JobID:         job.ID,
        ScheduledTime: scheduledTime,
        StartedAt:     time.Now(),
        Status:        ExecutionRunning,
        Attempts:      0,
    }

    // Record start
    if err := d.store.RecordExecution(execution); err != nil {
        return nil, err
    }

    // Execute with retries
    var lastErr error
    policy := job.RetryPolicy
    if policy == nil {
        policy = DefaultRetryPolicy
    }

    interval := policy.InitialInterval

    for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
        execution.Attempts = attempt

        // Create context with timeout
        execCtx, cancel := context.WithTimeout(ctx, job.Timeout)

        var result *ExecutionResult
        var err error

        if job.Webhook != nil {
            result, err = d.executeWebhook(execCtx, job.Webhook, execution)
        } else if job.GRPC != nil {
            result, err = d.executeGRPC(execCtx, job.GRPC, execution)
        }
        cancel()

        if err == nil && result.Success {
            execution.Status = ExecutionSuccess
            execution.CompletedAt = time.Now()
            execution.Response = result.Response
            d.store.UpdateExecution(execution)
            d.metrics.JobSucceeded(job)
            return execution, nil
        }

        lastErr = err
        if result != nil {
            execution.Response = result.Response
            execution.Error = result.Error
        }

        // Don't retry on non-retriable errors
        if !isRetriable(err) {
            break
        }

        // Wait before retry
        if attempt < policy.MaxAttempts {
            time.Sleep(interval)
            interval = time.Duration(float64(interval) * policy.Multiplier)
            if interval > policy.MaxInterval {
                interval = policy.MaxInterval
            }
        }
    }

    execution.Status = ExecutionFailed
    execution.CompletedAt = time.Now()
    if lastErr != nil {
        execution.Error = lastErr.Error()
    }
    d.store.UpdateExecution(execution)
    d.metrics.JobFailed(job)

    return execution, lastErr
}

// executeWebhook sends HTTP request
func (d *Dispatcher) executeWebhook(ctx context.Context, config *WebhookConfig, execution *Execution) (*ExecutionResult, error) {
    var body io.Reader
    if config.Body != "" {
        body = strings.NewReader(config.Body)
    }

    req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, body)
    if err != nil {
        return nil, err
    }

    // Add headers
    for k, v := range config.Headers {
        req.Header.Set(k, v)
    }

    // Add execution metadata headers
    req.Header.Set("X-Chronos-Job-ID", execution.JobID)
    req.Header.Set("X-Chronos-Execution-ID", execution.ID)
    req.Header.Set("X-Chronos-Scheduled-Time", execution.ScheduledTime.Format(time.RFC3339))

    // Add auth
    if config.Auth != nil {
        if err := d.applyAuth(req, config.Auth); err != nil {
            return nil, err
        }
    }

    resp, err := d.httpClient.Do(req)
    if err != nil {
        return &ExecutionResult{
            Success: false,
            Error:   err.Error(),
        }, err
    }
    defer resp.Body.Close()

    respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB limit

    success := isSuccessCode(resp.StatusCode, config.SuccessCodes)

    return &ExecutionResult{
        Success:    success,
        StatusCode: resp.StatusCode,
        Response:   string(respBody),
    }, nil
}

// Execution represents a job execution
type Execution struct {
    ID            string          `json:"id"`
    JobID         string          `json:"job_id"`
    ScheduledTime time.Time       `json:"scheduled_time"`
    StartedAt     time.Time       `json:"started_at"`
    CompletedAt   time.Time       `json:"completed_at,omitempty"`
    Status        ExecutionStatus `json:"status"`
    Attempts      int             `json:"attempts"`
    Response      string          `json:"response,omitempty"`
    Error         string          `json:"error,omitempty"`
}

type ExecutionStatus string

const (
    ExecutionPending ExecutionStatus = "pending"
    ExecutionRunning ExecutionStatus = "running"
    ExecutionSuccess ExecutionStatus = "success"
    ExecutionFailed  ExecutionStatus = "failed"
)
```

#### 4. Storage Layer

```go
// Store provides persistent storage using BadgerDB
type Store struct {
    db     *badger.DB
    prefix map[string][]byte
}

// NewStore creates a new BadgerDB store
func NewStore(path string) (*Store, error) {
    opts := badger.DefaultOptions(path)
    opts.Logger = nil // Use custom logger
    opts.SyncWrites = true
    opts.ValueLogFileSize = 64 << 20 // 64MB

    db, err := badger.Open(opts)
    if err != nil {
        return nil, err
    }

    return &Store{
        db: db,
        prefix: map[string][]byte{
            "jobs":       []byte("jobs/"),
            "executions": []byte("executions/"),
            "schedule":   []byte("schedule/"),
            "locks":      []byte("locks/"),
        },
    }, nil
}

// CreateJob stores a new job
func (s *Store) CreateJob(job *Job) error {
    return s.db.Update(func(txn *badger.Txn) error {
        key := append(s.prefix["jobs"], []byte(job.ID)...)

        data, err := json.Marshal(job)
        if err != nil {
            return err
        }

        return txn.Set(key, data)
    })
}

// GetJob retrieves a job by ID
func (s *Store) GetJob(id string) (*Job, error) {
    var job Job

    err := s.db.View(func(txn *badger.Txn) error {
        key := append(s.prefix["jobs"], []byte(id)...)

        item, err := txn.Get(key)
        if err != nil {
            return err
        }

        return item.Value(func(val []byte) error {
            return json.Unmarshal(val, &job)
        })
    })

    if err == badger.ErrKeyNotFound {
        return nil, ErrJobNotFound
    }

    return &job, err
}

// ListExecutions returns executions for a job
func (s *Store) ListExecutions(jobID string, limit int) ([]*Execution, error) {
    var executions []*Execution

    err := s.db.View(func(txn *badger.Txn) error {
        prefix := append(s.prefix["executions"], []byte(jobID+"/")...)

        opts := badger.DefaultIteratorOptions
        opts.Reverse = true // Newest first
        opts.PrefetchSize = limit

        it := txn.NewIterator(opts)
        defer it.Close()

        count := 0
        for it.Seek(append(prefix, 0xFF)); it.ValidForPrefix(prefix) && count < limit; it.Next() {
            item := it.Item()

            err := item.Value(func(val []byte) error {
                var exec Execution
                if err := json.Unmarshal(val, &exec); err != nil {
                    return err
                }
                executions = append(executions, &exec)
                return nil
            })

            if err != nil {
                return err
            }
            count++
        }

        return nil
    })

    return executions, err
}
```

### Data Flow

```
                            USER / CI/CD
                                 │
                    ┌────────────┼────────────┐
                    │            │            │
                    ▼            ▼            ▼
              ┌─────────┐  ┌─────────┐  ┌─────────┐
              │   CLI   │  │   API   │  │  Web UI │
              │chronosctl│ │  :8080  │  │  :8080  │
              └────┬────┘  └────┬────┘  └────┬────┘
                   │            │            │
                   └────────────┼────────────┘
                                │
                                ▼
┌───────────────────────────────────────────────────────────────────┐
│                         CHRONOS CLUSTER                            │
│                                                                    │
│   1. REQUEST HANDLING (API Layer)                                  │
│   ┌────────────────────────────────────────────────────────────┐  │
│   │  • Parse request                                            │  │
│   │  • Validate job definition                                  │  │
│   │  • Forward to Raft leader (if follower)                     │  │
│   └────────────────────────────────────────────────────────────┘  │
│                                │                                   │
│                                ▼                                   │
│   2. CONSENSUS (Raft Layer)                                        │
│   ┌────────────────────────────────────────────────────────────┐  │
│   │  • Propose to Raft log                                      │  │
│   │  • Replicate to followers                                   │  │
│   │  • Apply to FSM on commit                                   │  │
│   │  • Return result                                            │  │
│   └────────────────────────────────────────────────────────────┘  │
│                                │                                   │
│                                ▼                                   │
│   3. SCHEDULING (Scheduler Engine - Leader Only)                   │
│   ┌────────────────────────────────────────────────────────────┐  │
│   │  • Parse cron expression                                    │  │
│   │  • Calculate next run time (timezone-aware)                 │  │
│   │  • Add to priority queue                                    │  │
│   │  • Tick every second to check due jobs                      │  │
│   └────────────────────────────────────────────────────────────┘  │
│                                │                                   │
│                                ▼                                   │
│   4. EXECUTION (Dispatcher)                                        │
│   ┌────────────────────────────────────────────────────────────┐  │
│   │  • Acquire distributed lock for job                         │  │
│   │  • Check concurrency policy                                 │  │
│   │  • Record execution start in store                          │  │
│   │  • Execute webhook/gRPC with timeout                        │  │
│   │  • Retry on failure per policy                              │  │
│   │  • Record execution result                                  │  │
│   │  • Release lock                                             │  │
│   └────────────────────────────────────────────────────────────┘  │
│                                │                                   │
│                                ▼                                   │
│   5. OBSERVABILITY                                                 │
│   ┌────────────────────────────────────────────────────────────┐  │
│   │  • Emit Prometheus metrics                                  │  │
│   │  • Log execution details                                    │  │
│   │  • Update Web UI state                                      │  │
│   │  • Fire alerts on failure                                   │  │
│   └────────────────────────────────────────────────────────────┘  │
│                                                                    │
└───────────────────────────────────────────────────────────────────┘
                                │
                                ▼
                        TARGET SERVICES
                    ┌───────────────────────┐
                    │   HTTP Webhooks       │
                    │   gRPC Services       │
                    │   Kubernetes Jobs     │
                    └───────────────────────┘
```

### Configuration Model

```yaml
# chronos.yaml - Example configuration
cluster:
  node_id: chronos-1
  data_dir: /var/lib/chronos
  raft:
    address: 10.0.0.1:7000
    peers:
      - chronos-2:7000
      - chronos-3:7000

server:
  http:
    address: 0.0.0.0:8080
    read_timeout: 30s
    write_timeout: 30s
  grpc:
    address: 0.0.0.0:9000

scheduler:
  tick_interval: 1s
  execution_timeout: 5m
  default_retry_policy:
    max_attempts: 3
    initial_interval: 1s
    max_interval: 1m
    multiplier: 2.0

dispatcher:
  http:
    timeout: 30s
    max_idle_conns: 100
    idle_conn_timeout: 90s
  concurrency:
    max_concurrent_jobs: 100
    per_job_limit: 1

metrics:
  prometheus:
    enabled: true
    path: /metrics

logging:
  level: info
  format: json
  output: stdout

alerting:
  slack:
    webhook_url: https://hooks.slack.com/...
    channel: "#alerts"
  pagerduty:
    routing_key: xxx

auth:
  type: basic
  users:
    admin: $2a$10$... # bcrypt
```

---

## Feature Requirements

### P0 — MVP (Weeks 1-8)

#### F1: Cron Expression Parsing
**Priority:** P0
**Effort:** 1 week

| Requirement | Description | Acceptance Criteria |
|-------------|-------------|---------------------|
| F1.1 | Standard cron (5 fields) | `* * * * *` minute/hour/day/month/weekday |
| F1.2 | Extended cron (6 fields) | Seconds field support |
| F1.3 | Special characters | `*/5`, `1-10`, `1,15,30`, `@hourly`, `@daily` |
| F1.4 | Predefined schedules | `@yearly`, `@monthly`, `@weekly`, `@daily`, `@hourly` |
| F1.5 | Interval syntax | `@every 5m`, `@every 1h30m` |

#### F2: Distributed Consensus
**Priority:** P0
**Effort:** 2 weeks

| Requirement | Description | Acceptance Criteria |
|-------------|-------------|---------------------|
| F2.1 | Raft implementation | Using hashicorp/raft library |
| F2.2 | Leader election | Automatic leader election <5s |
| F2.3 | State replication | Job state replicated to all nodes |
| F2.4 | Cluster membership | Dynamic node join/leave |
| F2.5 | Snapshot and recovery | State recovery after restart |

#### F3: At-Least-Once Execution
**Priority:** P0
**Effort:** 1 week

| Requirement | Description | Acceptance Criteria |
|-------------|-------------|---------------------|
| F3.1 | Distributed locking | Only one node executes each job run |
| F3.2 | Execution recording | Record start before execution |
| F3.3 | Stale lock recovery | Recover from node failure during execution |
| F3.4 | Missed run detection | Detect and optionally execute missed runs |

#### F4: HTTP Webhook Dispatch
**Priority:** P0
**Effort:** 1 week

| Requirement | Description | Acceptance Criteria |
|-------------|-------------|---------------------|
| F4.1 | HTTP methods | GET, POST, PUT, DELETE support |
| F4.2 | Custom headers | Arbitrary headers per job |
| F4.3 | Request body | Static or templated body |
| F4.4 | Authentication | Basic, Bearer, API key |
| F4.5 | Timeout handling | Configurable per-job timeout |
| F4.6 | Success codes | Configurable success status codes |

#### F5: Job Status Tracking
**Priority:** P0
**Effort:** 1 week

| Requirement | Description | Acceptance Criteria |
|-------------|-------------|---------------------|
| F5.1 | Execution history | Store last N executions per job |
| F5.2 | Execution status | pending, running, success, failed |
| F5.3 | Response capture | Store response body (truncated) |
| F5.4 | Timing metrics | Duration, latency per execution |

#### F6: Web UI
**Priority:** P0
**Effort:** 1 week

| Requirement | Description | Acceptance Criteria |
|-------------|-------------|---------------------|
| F6.1 | Job list | View all jobs with status |
| F6.2 | Job detail | View job config and recent executions |
| F6.3 | Execution log | View execution details and response |
| F6.4 | Manual trigger | Trigger job execution manually |
| F6.5 | Job management | Create, edit, delete, enable/disable |

#### F7: Prometheus Metrics
**Priority:** P0
**Effort:** 0.5 weeks

| Requirement | Description | Acceptance Criteria |
|-------------|-------------|---------------------|
| F7.1 | Job metrics | Executions total, success/failure rate |
| F7.2 | Latency metrics | Execution duration histogram |
| F7.3 | Cluster metrics | Leader status, node health |
| F7.4 | System metrics | Go runtime, memory, goroutines |

### P1 — Post-MVP

#### F8: Job Dependencies
**Priority:** P1
**Effort:** 2 weeks

| Requirement | Description |
|-------------|-------------|
| F8.1 | Simple dependencies | Job B runs after Job A completes |
| F8.2 | DAG scheduling | Support for directed acyclic graphs |
| F8.3 | Failure propagation | Skip dependents on failure |

#### F9: Timezone-Aware Scheduling
**Priority:** P1
**Effort:** 1 week

| Requirement | Description |
|-------------|-------------|
| F9.1 | Per-job timezone | Schedule in any timezone |
| F9.2 | DST handling | Correct behavior during transitions |
| F9.3 | Timezone validation | Validate timezone names |

#### F10: Alerting Integration
**Priority:** P1
**Effort:** 1 week

| Requirement | Description |
|-------------|-------------|
| F10.1 | Slack notifications | Job failure alerts to Slack |
| F10.2 | PagerDuty integration | Critical job alerting |
| F10.3 | Custom webhooks | Generic alert webhooks |

### P2 — Future

| Feature | Description |
|---------|-------------|
| F11 | Kubernetes CRD mode |
| F12 | gRPC job dispatch |
| F13 | Job versioning and rollback |
| F14 | Multi-region scheduling |
| F15 | Natural language scheduling |

---

## Success Metrics

### Technical Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Schedule accuracy | ±1 second | Time deviation from scheduled |
| Failover time | <5 seconds | Leader election duration |
| Jobs per node | 1M+ definitions | Stress testing |
| Memory per 10K jobs | <100MB | Profiling |
| Missed job rate | <0.001% | Production monitoring |
| Execution overhead | <100ms | Time from schedule to dispatch |

### Adoption Metrics

| Metric | Target (Year 1) | Target (Year 3) |
|--------|-----------------|-----------------|
| GitHub stars | 4,000 | 20,000 |
| Production deployments | 1,000 | 10,000 |
| Active contributors | 30 | 120 |
| Docker pulls | 100K/month | 1M/month |
| Enterprise users | 15 | 150 |

### Operational Metrics

| Metric | Target | Description |
|--------|--------|-------------|
| Time to first job | <10 minutes | From install to first scheduled job |
| Mean time to debug | <2 minutes | Using provided UI and logs |
| Upgrade success rate | 99.9% | Zero-downtime upgrades |

---

## Risk Analysis

### Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Raft complexity causing bugs | Medium | High | Use mature hashicorp/raft, extensive testing |
| Clock drift causing scheduling issues | Medium | Medium | Use logical clocks, document NTP requirements |
| Storage corruption | Low | Critical | WAL, checksums, backup/restore tooling |
| Split-brain during network partition | Low | High | Raft prevents this, test partition scenarios |
| Memory pressure with many jobs | Medium | Medium | Pagination, lazy loading, configurable retention |

### Market Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| K8s CronJobs "good enough" | High | High | Clear reliability comparison, migration guide |
| Temporal/Airflow mindshare | High | Medium | Position as simpler alternative |
| Cloud vendor managed schedulers | Medium | Medium | Multi-cloud portability story |
| Enterprise features demand | Medium | Medium | Enterprise tier roadmap |

### Operational Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Difficult cluster operations | Medium | Medium | Excellent documentation, helm chart |
| Data migration between versions | Medium | Medium | Versioned storage format, migration tools |
| Debugging distributed issues | Medium | Medium | Comprehensive logging, debug endpoints |

---

## Development Roadmap

### Phase 1: Core Scheduling (Weeks 1-2)
**Goal:** Single-node scheduler with job execution

**Deliverables:**
- [ ] Cron expression parser
- [ ] Scheduler engine with priority queue
- [ ] HTTP webhook dispatcher
- [ ] BadgerDB storage layer
- [ ] Basic REST API

**Exit Criteria:**
- Create and execute jobs via API
- Jobs execute at scheduled times (±1s)
- Execution history stored and queryable

### Phase 2: Distributed Consensus (Weeks 3-4)
**Goal:** Multi-node cluster with Raft consensus

**Deliverables:**
- [ ] Raft node implementation
- [ ] FSM for job state
- [ ] Leader election
- [ ] State replication
- [ ] Distributed locking

**Exit Criteria:**
- 3-node cluster operational
- Leader failover <5s
- Jobs execute exactly once during failover

### Phase 3: Reliability Features (Weeks 5-6)
**Goal:** Production-grade reliability

**Deliverables:**
- [ ] Retry policies with backoff
- [ ] Concurrency policies
- [ ] Missed run recovery
- [ ] Timezone support
- [ ] Execution timeouts

**Exit Criteria:**
- Jobs retry correctly on failure
- Concurrent executions controlled
- Timezones work correctly including DST

### Phase 4: Operations (Weeks 7-8)
**Goal:** Production readiness

**Deliverables:**
- [ ] Web UI for job management
- [ ] Prometheus metrics
- [ ] Structured logging
- [ ] CLI tool (chronosctl)
- [ ] Documentation
- [ ] Helm chart

**Exit Criteria:**
- Full functionality via Web UI
- Grafana dashboard template
- Kubernetes deployment working
- Zero known P0 bugs

---

## Go-to-Market Strategy

### Phase 1: Developer Awareness (Months 1-3)

**Activities:**
- Launch on GitHub with comparison to K8s CronJobs
- "Why K8s CronJobs Break and How to Fix It" blog post
- Post to HackerNews, r/devops, r/kubernetes
- Demo video showing reliability difference
- Speak at Kubernetes meetups

**Goals:**
- 2,000 GitHub stars
- 100 production trials
- 15 community contributors

### Phase 2: Community Growth (Months 4-6)

**Activities:**
- Weekly reliability-focused blog posts
- Discord community launch
- KubeCon presentation proposal
- Case studies from early adopters
- Integration guides for popular frameworks

**Goals:**
- 4,000 GitHub stars
- 500 production deployments
- Active Discord (800+ members)

### Phase 3: Enterprise Push (Months 7-12)

**Activities:**
- Chronos Enterprise launch
- SOC 2 certification
- Enterprise features (SSO, audit logging)
- Partner integrations (monitoring tools)
- Direct enterprise sales

**Goals:**
- 15 paying enterprise customers
- $400K ARR pipeline
- 1,000 production deployments

### Messaging Framework

**Tagline:** "Cron that just works"

**Key Messages:**
1. **Reliability**: Never miss a scheduled job again
2. **Simplicity**: Single binary, zero dependencies
3. **Visibility**: Know exactly what ran and when
4. **Kubernetes-Native**: Drop-in CronJob replacement

**Competitive Positioning:**
- vs. K8s CronJobs: "100x reliability without complexity"
- vs. Temporal: "When you need scheduling, not a platform"
- vs. Airflow: "Cron without the Python and complexity"

---

## Appendix

### A. Kubernetes CronJob Failure Modes

| Issue | Description | Chronos Solution |
|-------|-------------|------------------|
| 100-miss-limit | Job permanently halts after 100 consecutive misses | No miss limit, configurable catch-up |
| No execution history | Only stores last successful/failed time | Full execution history with responses |
| No distributed locking | Multiple pods can execute same job | Raft-based distributed locking |
| UTC-only | Timezone scheduling requires workarounds | Native timezone support |
| No visibility | Requires kubectl and log diving | Web UI with full history |

### B. Cron Expression Reference

| Expression | Description |
|------------|-------------|
| `* * * * *` | Every minute |
| `0 * * * *` | Every hour |
| `0 0 * * *` | Every day at midnight |
| `0 0 * * 0` | Every Sunday at midnight |
| `*/5 * * * *` | Every 5 minutes |
| `0 9-17 * * 1-5` | Every hour 9am-5pm, Monday-Friday |
| `@hourly` | Every hour (0 * * * *) |
| `@daily` | Every day at midnight |
| `@every 30m` | Every 30 minutes |

### C. API Reference

**Create Job:**
```http
POST /api/v1/jobs
Content-Type: application/json

{
  "name": "daily-report",
  "schedule": "0 9 * * *",
  "timezone": "America/New_York",
  "webhook": {
    "url": "https://api.example.com/reports/generate",
    "method": "POST",
    "headers": {
      "Authorization": "Bearer token123"
    }
  },
  "retry_policy": {
    "max_attempts": 3,
    "initial_interval": "1s",
    "multiplier": 2.0
  },
  "timeout": "5m",
  "enabled": true
}
```

**List Executions:**
```http
GET /api/v1/jobs/{id}/executions?limit=20
```

**Trigger Job:**
```http
POST /api/v1/jobs/{id}/trigger
```

### D. Glossary

| Term | Definition |
|------|------------|
| Cron Expression | String format defining schedule timing |
| Raft | Consensus algorithm for distributed state |
| FSM | Finite State Machine for applying Raft log entries |
| At-Least-Once | Guarantee that job executes minimum once |
| Leader | Raft node responsible for scheduling |
| Follower | Raft node that replicates state from leader |
| Execution | Single run instance of a job |
| Webhook | HTTP endpoint called when job executes |

---

*Document Version: 1.0*
*Last Updated: December 2024*
*Next Review: January 2025*
