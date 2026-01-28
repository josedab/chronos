# Chronos Architecture

This document describes the architecture of Chronos, including its core components and the next-generation features.

## System Overview

```mermaid
graph TB
    subgraph "Client Layer"
        WebUI[Web UI]
        CLI[chronosctl CLI]
        API[REST API Clients]
        Mobile[Mobile App]
    end

    subgraph "API Gateway"
        HTTP[HTTP Server]
        WS[WebSocket Hub]
        GRPC[gRPC Server]
    end

    subgraph "Core Services"
        Scheduler[Scheduler]
        PolicyEngine[Policy Engine]
        WorkflowBuilder[Workflow Builder]
        SmartSched[Smart Scheduler]
    end

    subgraph "Dispatch Layer"
        MultiDispatcher[Multi-Dispatcher]
        HTTPDispatch[HTTP Webhook]
        GRPCDispatch[gRPC]
        KafkaDispatch[Kafka]
        NATSDispatch[NATS]
        RabbitDispatch[RabbitMQ]
    end

    subgraph "Storage Layer"
        Raft[Raft Consensus]
        BadgerDB[(BadgerDB)]
    end

    subgraph "Federation"
        FedManager[Federation Manager]
        GeoRouter[Geo Router]
    end

    WebUI --> HTTP
    CLI --> HTTP
    API --> HTTP
    Mobile --> HTTP
    Mobile --> WS

    HTTP --> Scheduler
    HTTP --> PolicyEngine
    HTTP --> WorkflowBuilder
    WS --> Realtime[Real-Time Hub]

    Scheduler --> MultiDispatcher
    MultiDispatcher --> HTTPDispatch
    MultiDispatcher --> GRPCDispatch
    MultiDispatcher --> KafkaDispatch
    MultiDispatcher --> NATSDispatch
    MultiDispatcher --> RabbitDispatch

    Scheduler --> Raft
    Raft --> BadgerDB

    Scheduler --> FedManager
    FedManager --> GeoRouter
```

## Core Components

### Scheduler

The scheduler is responsible for:
- Parsing cron expressions and calculating next run times
- Triggering job executions at the appropriate time
- Managing job state (enabled/disabled, running, etc.)
- Enforcing concurrency policies (allow, forbid, replace)

```mermaid
sequenceDiagram
    participant S as Scheduler
    participant R as Raft
    participant D as Dispatcher
    participant T as Target Service

    loop Every tick (1s)
        S->>R: Get due jobs
        R-->>S: Job list
        S->>S: Filter by concurrency policy
        S->>D: Execute job
        D->>T: HTTP/gRPC/MQ request
        T-->>D: Response
        D-->>S: Execution result
        S->>R: Store execution record
    end
```

### Raft Consensus

Chronos uses HashiCorp Raft for distributed consensus:
- Leader election ensures only one node schedules jobs
- Log replication keeps all nodes in sync
- Snapshot and restore for state recovery

### Storage (BadgerDB)

All state is persisted in BadgerDB:
- Jobs and their configurations
- Execution history
- Cluster membership
- User sessions and API keys

## Next-Gen Feature Architecture

### Multi-Protocol Dispatch

```mermaid
graph LR
    subgraph "Job Configuration"
        Job[Job Definition]
    end

    subgraph "Multi-Dispatcher"
        Router{Protocol Router}
    end

    subgraph "Dispatchers"
        HTTP[HTTP Webhook]
        GRPC[gRPC Client]
        Kafka[Kafka Producer]
        NATS[NATS Publisher]
        RabbitMQ[RabbitMQ Publisher]
    end

    subgraph "Targets"
        WebService[Web Services]
        GRPCService[gRPC Services]
        KafkaTopic[Kafka Topics]
        NATSSubject[NATS Subjects]
        RabbitQueue[RabbitMQ Queues]
    end

    Job --> Router
    Router -->|webhook config| HTTP
    Router -->|grpc config| GRPC
    Router -->|kafka config| Kafka
    Router -->|nats config| NATS
    Router -->|rabbitmq config| RabbitMQ

    HTTP --> WebService
    GRPC --> GRPCService
    Kafka --> KafkaTopic
    NATS --> NATSSubject
    RabbitMQ --> RabbitQueue
```

### Visual Workflow Builder (DAG)

```mermaid
graph TD
    subgraph "Visual Canvas"
        T[Trigger Node]
        J1[Job Node 1]
        J2[Job Node 2]
        C{Condition}
        P[Parallel Split]
        J3[Job Node 3]
        J4[Job Node 4]
        Join[Join]
        N[Notification]
    end

    T --> J1
    J1 --> C
    C -->|success| P
    C -->|failure| N
    P --> J3
    P --> J4
    J3 --> Join
    J4 --> Join
    Join --> J2
    J2 --> N
```

**Node Types:**
| Type | Description |
|------|-------------|
| `trigger` | Entry point (cron, webhook, manual) |
| `job` | Execute a Chronos job |
| `condition` | Branch based on expression |
| `parallel` | Split into concurrent branches |
| `join` | Wait for parallel branches |
| `delay` | Wait for duration |
| `approval` | Pause for human approval |
| `notification` | Send alert (Slack, email, etc.) |
| `script` | Execute inline script |
| `http` | Make HTTP request |
| `transform` | Transform data |
| `subworkflow` | Execute nested workflow |

### Policy-as-Code Engine

```mermaid
graph TB
    subgraph "Policy Definition"
        YAML[policies.yaml]
    end

    subgraph "Policy Engine"
        Parser[Policy Parser]
        Compiler[Condition Compiler]
        Evaluator[Policy Evaluator]
    end

    subgraph "Evaluation Context"
        Job[Job Object]
        Env[Environment]
        User[User Context]
    end

    subgraph "Results"
        Allow[✓ Allow]
        Deny[✗ Deny + Violations]
    end

    YAML --> Parser
    Parser --> Compiler
    Compiler --> Evaluator

    Job --> Evaluator
    Env --> Evaluator
    User --> Evaluator

    Evaluator --> Allow
    Evaluator --> Deny
```

**Example Policy:**
```yaml
name: production-security
description: Security rules for production jobs
rules:
  - name: https-only
    condition:
      field: webhook.url
      operator: starts_with
      value: "https://"
    action: deny
    message: "Production jobs must use HTTPS"
    
  - name: naming-convention
    condition:
      field: name
      operator: regex_matches
      value: "^[a-z][a-z0-9-]*$"
    action: deny
    message: "Job names must be lowercase with hyphens"
```

### Cross-Region Federation

```mermaid
graph TB
    subgraph "Region: US-East"
        C1[Chronos Cluster 1]
        DB1[(BadgerDB)]
    end

    subgraph "Region: US-West"
        C2[Chronos Cluster 2]
        DB2[(BadgerDB)]
    end

    subgraph "Region: EU-West"
        C3[Chronos Cluster 3]
        DB3[(BadgerDB)]
    end

    subgraph "Federation Layer"
        FM[Federation Manager]
        CR[Conflict Resolution]
        HR[Health Router]
    end

    C1 <-->|sync| FM
    C2 <-->|sync| FM
    C3 <-->|sync| FM

    FM --> CR
    FM --> HR

    C1 --- DB1
    C2 --- DB2
    C3 --- DB3
```

**Conflict Resolution Strategies:**
- `newest` - Most recent modification wins
- `owner_wins` - Originating cluster wins
- `merge` - Combine non-conflicting changes

### AI-Powered Schedule Optimization

```mermaid
graph LR
    subgraph "Data Collection"
        Exec[Execution Records]
        Metrics[Prometheus Metrics]
    end

    subgraph "Analysis Engine"
        Stats[Running Statistics<br/>Welford's Algorithm]
        Trend[Trend Analysis<br/>Linear Regression]
        Anomaly[Anomaly Detection]
    end

    subgraph "Recommendations"
        Windows[Optimal Windows]
        Alerts[Anomaly Alerts]
        Forecast[Capacity Forecast]
    end

    Exec --> Stats
    Metrics --> Stats
    Stats --> Trend
    Stats --> Anomaly
    Trend --> Windows
    Trend --> Forecast
    Anomaly --> Alerts
```

### Real-Time Collaboration

```mermaid
sequenceDiagram
    participant U1 as User 1
    participant WS as WebSocket Hub
    participant U2 as User 2

    U1->>WS: Join room (job-123)
    WS->>U2: presence_update (U1 joined)
    
    U1->>WS: cursor_move (x, y)
    WS->>U2: cursor_update (U1: x, y)
    
    U1->>WS: selection_change (field)
    WS->>U2: selection_update (U1: field)
    
    Note over WS: Job execution starts
    WS->>U1: execution_started
    WS->>U2: execution_started
    
    Note over WS: Job completes
    WS->>U1: execution_completed
    WS->>U2: execution_completed
```

### Time-Travel Debugging

```mermaid
stateDiagram-v2
    [*] --> CreateSession
    CreateSession --> Active
    
    Active --> StepForward: step_forward()
    Active --> StepBackward: step_backward()
    Active --> JumpTo: jump_to_step(n)
    
    StepForward --> Active
    StepBackward --> Active
    JumpTo --> Active
    
    Active --> Breakpoint: hit breakpoint
    Breakpoint --> Active: continue()
    
    Active --> Export: export_session()
    Export --> [*]
```

**Breakpoint Types:**
- `step` - Break at specific step number
- `type` - Break on node type (e.g., all HTTP nodes)
- `condition` - Break when expression is true
- `error` - Break on any error

### Cloud Control Plane

```mermaid
graph TB
    subgraph "Control Plane"
        API[Control Plane API]
        TM[Tenant Manager]
        CM[Cluster Manager]
        BM[Billing Manager]
    end

    subgraph "Tenants"
        T1[Tenant: Acme Corp]
        T2[Tenant: Globex Inc]
        T3[Tenant: Initech]
    end

    subgraph "Managed Clusters"
        C1[Cluster: acme-prod]
        C2[Cluster: acme-dev]
        C3[Cluster: globex-prod]
    end

    subgraph "Cloud Providers"
        AWS[AWS EKS]
        GCP[GCP GKE]
        Azure[Azure AKS]
    end

    API --> TM
    API --> CM
    API --> BM

    TM --> T1
    TM --> T2
    TM --> T3

    T1 --> C1
    T1 --> C2
    T2 --> C3

    CM --> AWS
    CM --> GCP
    CM --> Azure
```

**Subscription Plans:**
| Plan | Jobs | Executions/mo | Retention | Features |
|------|------|---------------|-----------|----------|
| Free | 10 | 1,000 | 1 day | Basic |
| Starter | 100 | 50,000 | 7 days | + Webhooks |
| Pro | 1,000 | 500,000 | 30 days | + Federation |
| Enterprise | Unlimited | Unlimited | 90 days | + SSO, SLA |

## Data Flow

### Job Execution Flow

```mermaid
flowchart TD
    A[Scheduler Tick] --> B{Job Due?}
    B -->|No| A
    B -->|Yes| C{Policy Check}
    C -->|Denied| D[Log Violation]
    C -->|Allowed| E{Concurrency Check}
    E -->|Blocked| F[Skip Execution]
    E -->|Allowed| G[Create Execution Record]
    G --> H[Multi-Dispatcher]
    H --> I{Protocol?}
    I -->|HTTP| J[HTTP Webhook]
    I -->|gRPC| K[gRPC Call]
    I -->|Kafka| L[Kafka Publish]
    I -->|NATS| M[NATS Publish]
    I -->|RabbitMQ| N[RabbitMQ Publish]
    J --> O[Record Result]
    K --> O
    L --> O
    M --> O
    N --> O
    O --> P{Success?}
    P -->|Yes| Q[Mark Complete]
    P -->|No| R{Retry?}
    R -->|Yes| H
    R -->|No| S[Mark Failed]
    Q --> T[Broadcast Update]
    S --> T
    T --> U[Push Notification]
```

## Package Structure

```
chronos/
├── cmd/
│   ├── chronos/          # Main server binary
│   └── chronosctl/       # CLI tool
├── internal/
│   ├── analytics/        # Execution analytics and reporting
│   ├── api/              # HTTP handlers
│   ├── assistant/        # AI-powered job creation
│   ├── autoscale/        # Predictive cluster autoscaling
│   ├── chaos/            # Chaos engineering framework
│   ├── cloudevents/      # CloudEvents protocol support
│   ├── compliance/       # Compliance and audit framework
│   ├── config/           # Configuration management
│   ├── cost/             # Multi-cloud cost optimization
│   ├── dag/              # DAG engine + workflow builder
│   ├── dispatcher/       # Multi-protocol dispatchers
│   ├── eventmesh/        # Event mesh integration
│   ├── events/           # Event-driven triggers
│   ├── geo/              # Federation + geo routing
│   ├── gitops/           # Git-based job synchronization
│   ├── marketplace/      # Job templates
│   ├── metrics/          # Prometheus metrics
│   ├── mobile/           # Push notifications
│   ├── models/           # Domain models
│   ├── notify/           # Notification system
│   ├── policy/           # Policy-as-code engine
│   ├── prediction/       # ML-based failure prediction
│   ├── raft/             # Raft consensus
│   ├── rbac/             # Role-based access control
│   ├── realtime/         # WebSocket collaboration
│   ├── replay/           # Time-travel debugging
│   ├── sandbox/          # Isolated container execution
│   ├── scheduler/        # Core scheduler
│   ├── search/           # Semantic job search
│   ├── secrets/          # Secret injection (Vault, cloud)
│   ├── smartsched/       # AI optimization
│   ├── storage/          # BadgerDB wrapper
│   ├── studio/           # Webhook testing
│   ├── tenant/           # Multi-tenant support
│   ├── tracing/          # OpenTelemetry integration
│   └── wasm/             # WebAssembly plugin runtime
├── pkg/
│   ├── clock/            # Time utilities
│   ├── cloud/            # Cloud control plane
│   ├── cron/             # Cron parser
│   ├── duration/         # Duration parsing
│   ├── executor/         # Cloud function executors
│   ├── grpc/             # gRPC service definitions
│   └── plugin/           # Plugin interfaces
└── web/                  # React UI
```

## Security Considerations

1. **Authentication**: API keys, JWT tokens, OAuth2
2. **Authorization**: Role-based access control (RBAC)
3. **Encryption**: TLS for all network communication
4. **Secrets**: Webhook credentials stored encrypted with Vault/cloud integration
5. **Audit**: All actions logged with user context
6. **Policy Enforcement**: Pre-execution policy checks
7. **Compliance**: SOC2, HIPAA, GDPR policy support
8. **Sandbox Execution**: Isolated container environments for untrusted jobs

## Performance Characteristics

| Metric | Target |
|--------|--------|
| Scheduling latency | < 100ms |
| Job throughput | 10,000 jobs/sec |
| Cluster failover | < 5 seconds |
| API response time | < 50ms (p99) |
| WebSocket broadcast | < 10ms |
