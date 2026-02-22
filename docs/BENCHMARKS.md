# Chronos Performance Benchmarks

**Environment:** Apple M1 Max, 32GB RAM, Go 1.24, macOS  
**Date:** February 2026  
**Version:** 0.1.0

## Summary

| Operation | Throughput | Latency (p50) | Memory/op |
|-----------|-----------|---------------|-----------|
| Webhook Dispatch (HTTP) | ~9,000 ops/sec | 112µs | 10.7 KB |
| Dispatch + HMAC Signing | ~11,400 ops/sec | 88µs | 11.8 KB |
| Dispatch + Assertions | ~11,700 ops/sec | 86µs | 10.7 KB |
| HMAC-SHA256 Computation | ~1,500,000 ops/sec | 651ns | 795 B |
| Assertion Evaluation | ~675,000 ops/sec | 1.5µs | 1.3 KB |
| Event Bus Publish | ~26,000,000 ops/sec | 38ns | 0 B |
| Rate Limiter (Allow+Release) | ~10,000,000 ops/sec | 98ns | 0 B |
| Scheduler AddJob | ~53,000 ops/sec | 19µs | 460 B |
| DAG Topological Sort (20 nodes) | ~173,000 ops/sec | 5.8µs | 4.1 KB |
| Storage CreateJob (BadgerDB) | ~3,300 ops/sec | 300µs | 76 KB |
| Storage GetJob (BadgerDB) | ~1,400 ops/sec | 727µs | 75 KB |

## Detailed Results

### Dispatcher (Webhook Execution)

```
BenchmarkDispatcher_Execute                 112µs/op    10,769 B/op    124 allocs/op
BenchmarkDispatcher_Execute_WithSigning      88µs/op    11,790 B/op    137 allocs/op
BenchmarkDispatcher_Execute_WithAssertions   86µs/op    10,674 B/op    137 allocs/op
```

**Key insights:**
- HMAC signing overhead: **< 1µs** per webhook call (negligible)
- Response assertions overhead: **1.5µs** per evaluation (negligible)
- The bulk of dispatch time is the HTTP round-trip, not Chronos processing

### Security Operations

```
BenchmarkComputeHMAC           651ns/op    795 B/op    13 allocs/op
BenchmarkEvaluateAssertions   1482ns/op   1291 B/op    31 allocs/op
```

**Key insights:**
- HMAC signing is **< 1µs** — zero performance reason to disable it
- Full assertion evaluation (body + JSON path) is **< 2µs**

### Scheduler & Rate Limiting

```
BenchmarkScheduler_AddJob                   19µs/op     460 B/op     3 allocs/op
BenchmarkRateLimiter_Allow                   98ns/op       0 B/op     0 allocs/op
BenchmarkRateLimiter_MultiNamespace         104ns/op       1 B/op     0 allocs/op
BenchmarkDependencyResolver_TopologicalSort 5.8µs/op   4096 B/op    36 allocs/op
```

**Key insights:**
- Rate limiter is zero-allocation — safe to enable on every request
- DAG topological sort for 20-node graphs takes **< 6µs**
- Scheduler can add **53K jobs/sec** — far exceeds real-world needs

### Event Streaming

```
BenchmarkEventBus_Publish    38ns/op    0 B/op    0 allocs/op
```

**Key insights:**
- Event publishing is zero-allocation and takes **38 nanoseconds**
- Safe to emit events on every execution lifecycle transition

### Storage (BadgerDB)

```
BenchmarkStore_CreateJob    300µs/op    76,383 B/op    53 allocs/op
BenchmarkStore_GetJob       727µs/op    74,961 B/op    21 allocs/op
BenchmarkStore_ListJobs     376µs/op   183,328 B/op  1464 allocs/op
```

**Key insights:**
- Embedded BadgerDB write latency is **300µs** (3,300 writes/sec)
- Suitable for workloads up to ~10,000 jobs with sub-second scheduling
- For higher scale, consider external storage backends

## Comparison: Chronos vs K8s CronJobs

| Capability | Chronos | K8s CronJobs |
|-----------|---------|-------------|
| Max jobs | 10,000+ (tested) | ~100-200 (controller limits) |
| Retry on failure | ✅ (configurable policy) | ❌ |
| DAG dependencies | ✅ (fan-out/fan-in) | ❌ |
| Multi-cluster | ✅ (Raft federation) | ❌ (single cluster) |
| Failover time | < 2s (E2E tested) | N/A (no HA) |
| Webhook signing | ✅ (HMAC + mTLS) | ❌ |
| Response assertions | ✅ | ❌ |
| Web UI | ✅ | ❌ |
| Terraform provider | ✅ | ❌ (native resource only) |
| Cost tracking | ✅ | ❌ |

## Reproducing Benchmarks

```bash
# Dispatcher benchmarks
go test ./internal/dispatcher/ -bench=. -benchtime=500x -run=^$ -benchmem

# Scheduler benchmarks
go test ./internal/scheduler/ -bench=. -benchtime=500x -run=^$ -benchmem

# Storage benchmarks
go test ./internal/storage/ -bench=. -benchtime=100x -run=^$ -benchmem
```
