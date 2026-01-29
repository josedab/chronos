---
sidebar_position: 1
title: Architecture
description: How Chronos works - distributed consensus, scheduling, and high availability
---

# Architecture

Chronos is designed around three core principles: **zero dependencies**, **distributed consensus**, and **at-least-once execution**. This page explains how these principles are implemented.

## High-Level Overview

```
┌──────────────────────────────────────────────────┐
│                  CHRONOS CLUSTER                  │
│                                                   │
│   ┌─────────────┐ ┌─────────────┐ ┌────────────┐ │
│   │   Node 1    │ │   Node 2    │ │   Node 3   │ │
│   │  (Leader)   │ │ (Follower)  │ │ (Follower) │ │
│   │             │ │             │ │            │ │
│   │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌────────┐ │ │
│   │ │Scheduler│ │ │ │Scheduler│ │ │ │Scheduler│ │ │
│   │ │ (active)│ │ │ │(standby)│ │ │ │(standby)│ │ │
│   │ └─────────┘ │ │ └─────────┘ │ │ └────────┘ │ │
│   │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌────────┐ │ │
│   │ │  Raft   │◄┼─┼─│  Raft   │◄┼─┼─│  Raft  │ │ │
│   │ └─────────┘ │ │ └─────────┘ │ │ └────────┘ │ │
│   │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌────────┐ │ │
│   │ │BadgerDB │ │ │ │BadgerDB │ │ │ │BadgerDB│ │ │
│   │ └─────────┘ │ │ └─────────┘ │ │ └────────┘ │ │
│   └─────────────┘ └─────────────┘ └────────────┘ │
└──────────────────────────────────────────────────┘
                        │
                        ▼
              ┌─────────────────┐
              │ Target Services │
              │  (HTTP/gRPC/MQ) │
              └─────────────────┘
```

### Key Characteristics

| Characteristic | Implementation |
|---------------|----------------|
| **Leader-based scheduling** | Only the Raft leader runs the active scheduler |
| **Read scalability** | Any node can serve read requests |
| **Write consistency** | All writes go through Raft consensus |
| **Automatic failover** | Leader failure triggers election within ~5 seconds |

## Core Components

### 1. Scheduler

The scheduler is responsible for determining when jobs should run and triggering their execution.

**How it works:**
1. Every tick (default: 1 second), the scheduler checks for due jobs
2. Jobs are stored in a priority queue ordered by next run time
3. When a job is due, the scheduler creates an execution record and dispatches it
4. After execution, the next run time is calculated and updated

```yaml
scheduler:
  tick_interval: 1s          # How often to check for due jobs
  execution_timeout: 5m      # Default job timeout
  missed_run_policy: execute_one  # What to do with missed runs
```

**Missed Run Policies:**

| Policy | Behavior |
|--------|----------|
| `ignore` | Skip missed executions |
| `execute_one` | Execute once for all missed runs (default) |
| `execute_all` | Execute once for each missed run |

### 2. Raft Consensus

Chronos uses [HashiCorp Raft](https://github.com/hashicorp/raft) for distributed consensus, the same library that powers Consul and Nomad.

**Why Raft?**

| Factor | Embedded Raft | External Etcd/Consul |
|--------|---------------|----------------------|
| Dependencies | None | Requires separate cluster |
| Operational complexity | Low | High |
| Latency | ~1ms | ~10ms |
| Deployment | Single binary | Multiple components |

**How Raft works in Chronos:**

1. **Leader Election**: When starting, nodes elect a leader via Raft consensus
2. **Log Replication**: All writes (job create/update/delete) go through the leader and are replicated
3. **State Machine**: The Raft FSM applies committed log entries to the local store
4. **Snapshots**: Periodic snapshots prevent unbounded log growth

**Raft FSM Commands:**

| Command | Description |
|---------|-------------|
| `CreateJob` | Add a new job to the store |
| `UpdateJob` | Modify an existing job |
| `DeleteJob` | Remove a job |
| `RecordExecution` | Store execution results |
| `UpdateSchedule` | Update next run time |

### 3. Storage Layer (BadgerDB)

Chronos uses [BadgerDB](https://github.com/dgraph-io/badger) for embedded storage.

**Why BadgerDB?**

| Factor | BadgerDB | SQLite | PostgreSQL |
|--------|----------|--------|------------|
| Embedded | ✅ | ✅ | ❌ |
| Write performance | Excellent | Good | Excellent |
| LSM-tree optimized | ✅ | ❌ | ❌ |
| Go-native | ✅ | CGO required | ❌ |

BadgerDB is optimized for write-heavy workloads, which is ideal for storing execution logs.

**Data Layout:**

| Key Prefix | Data |
|------------|------|
| `jobs/` | Job definitions |
| `executions/` | Execution history |
| `schedule/` | Next run times |
| `locks/` | Distributed locks |

### 4. Dispatcher

The dispatcher handles the actual execution of jobs, supporting multiple protocols:

- **HTTP/HTTPS** - Webhook calls with full header/body support
- **gRPC** - For high-performance internal services
- **Kafka** - Publish messages to Kafka topics
- **NATS** - Publish to NATS subjects
- **RabbitMQ** - Publish to RabbitMQ queues

**Circuit Breaker:**

The HTTP dispatcher includes a circuit breaker to prevent cascading failures:

```
        ┌───────────────────────────────────────┐
        │                                       │
        │    ┌─────────┐      failure      ┌──────────┐
        │    │         │   threshold       │          │
        └────│ Closed  │─────exceeded──────▶│   Open   │
             │         │                    │          │
             └─────────┘                    └──────────┘
                  ▲                              │
                  │                              │ timeout
                  │                              │ elapsed
                  │         success              ▼
                  └──────────────────────┌───────────┐
                                         │ Half-Open │
                           failure       │           │
                           ─────────────▶│           │
                                         └───────────┘
```

## Data Flow

### Job Creation

```
1. Client sends POST /api/v1/jobs
2. API validates request and checks policies
3. If allowed, API proposes CreateJob to Raft
4. Leader replicates to followers
5. Once majority ack, entry is committed
6. FSM applies to local store
7. Scheduler adds job to priority queue
8. API returns success to client
```

### Job Execution

```
1. Scheduler tick fires (every 1s)
2. Scheduler queries priority queue for due jobs
3. For each due job:
   a. Check concurrency policy
   b. Create execution record via Raft
   c. Dispatch to target service
   d. Record result via Raft
   e. Calculate and store next run time
```

## Deployment Topologies

### Single Node (Development)

```
┌─────────────────────────┐
│     Chronos Node        │
│  ┌───────────────────┐  │
│  │ API + Scheduler   │  │
│  │ + Raft (single)   │  │
│  │ + BadgerDB        │  │
│  └───────────────────┘  │
└─────────────────────────┘
```

Suitable for development and testing. No high availability.

### Three-Node Cluster (Production)

```
        Load Balancer
             │
    ┌────────┼────────┐
    ▼        ▼        ▼
┌───────┐ ┌───────┐ ┌───────┐
│Node 1 │ │Node 2 │ │Node 3 │
│Leader │ │Follow │ │Follow │
└───────┘ └───────┘ └───────┘
    │         │         │
    └────Raft─┼───Raft──┘
              │
        (consensus)
```

**Recommended for production:**
- Tolerates 1 node failure
- Distribute across availability zones
- Use odd numbers (3, 5, 7) for quorum

### Five-Node Cluster (High Availability)

For mission-critical deployments:
- Tolerates 2 node failures
- Higher read throughput
- Slightly higher write latency (more nodes to replicate)

## Design Decisions

### Why HTTP Webhooks as Primary Dispatch?

1. **Language agnostic** - Works with any HTTP server
2. **Debuggable** - Standard tooling (curl, Postman)
3. **Retry-friendly** - HTTP status codes indicate retry behavior
4. **Secure** - TLS, authentication headers built-in

### Why Not Use a Message Queue?

Message queues add operational complexity. Chronos's embedded architecture means:
- No Kafka/RabbitMQ cluster to manage
- No additional failure points
- Simpler debugging and operations

If you need MQ integration, Chronos can publish to Kafka, NATS, or RabbitMQ as dispatch targets.

## Next Steps

- [Understanding Jobs](/docs/core-concepts/jobs)
- [Scheduling Reference](/docs/core-concepts/scheduling)
- [High Availability Guide](/docs/guides/high-availability)
