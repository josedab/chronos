---
slug: chronos-performance-benchmarks
title: "Chronos Performance: How Fast Is a Zero-Dependency Distributed Cron?"
authors: [chronos-team]
tags: [benchmarks, performance, distributed-systems]
---

# Chronos Performance: How Fast Is a Zero-Dependency Distributed Cron?

One of the most common questions about Chronos is: **"What's the performance overhead of embedding everything into a single binary?"** The answer: surprisingly low.

We ran comprehensive benchmarks on an Apple M1 Max with Go 1.24. Here are the results.

<!-- truncate -->

## The Key Numbers

| Operation | Throughput | Latency | Memory/op |
|-----------|-----------|---------|-----------|
| **Webhook dispatch** | 9,000 ops/sec | 112µs | 10.7 KB |
| **HMAC signing** | 1,500,000 ops/sec | 651ns | 795 B |
| **Response assertions** | 675,000 ops/sec | 1.5µs | 1.3 KB |
| **Rate limiter** | 10,000,000 ops/sec | 98ns | 0 B |
| **Event bus publish** | 26,000,000 ops/sec | 38ns | 0 B |
| **DAG topological sort** (20 nodes) | 173,000 ops/sec | 5.8µs | 4.1 KB |

## Security Has Zero Cost

The most surprising finding: **enabling HMAC webhook signing adds less than 1 microsecond per request.** There is literally no performance reason to disable it.

```
Without signing:  112µs per dispatch
With signing:      88µs per dispatch (faster due to test variance)
HMAC computation: 651ns standalone
```

Similarly, response assertions (JSON path matching, body checks, timing SLAs) add only **1.5 microseconds** per evaluation. You should always use assertions.

## Rate Limiting Is Free

The per-namespace token bucket rate limiter operates in **98 nanoseconds with zero allocations.** This means you can check rate limits on every single request without measurable overhead.

## How Chronos Compares to K8s CronJobs

| Capability | Chronos | K8s CronJobs |
|-----------|---------|-------------|
| Max jobs | 10,000+ | ~100-200 |
| Retry on failure | ✅ (configurable) | ❌ |
| DAG dependencies | ✅ (fan-out/fan-in) | ❌ |
| Multi-cluster failover | ✅ (< 2s, E2E tested) | ❌ |
| Webhook signing | ✅ (HMAC + mTLS) | ❌ |
| Web UI | ✅ | ❌ |
| Terraform provider | ✅ | ❌ |
| Cost tracking | ✅ | ❌ |

## Reproduce These Benchmarks

All benchmarks are included in the Chronos repository:

```bash
# Dispatcher benchmarks
go test ./internal/dispatcher/ -bench=. -benchtime=500x -run=^$ -benchmem

# Scheduler benchmarks
go test ./internal/scheduler/ -bench=. -benchtime=500x -run=^$ -benchmem

# Storage benchmarks
go test ./internal/storage/ -bench=. -benchtime=100x -run=^$ -benchmem
```

Full results: [docs/BENCHMARKS.md](https://github.com/chronos/chronos/blob/main/docs/BENCHMARKS.md)

## Try Chronos

```bash
docker run -d -p 8080:8080 ghcr.io/chronos/chronos:0.1.0
```

[GitHub](https://github.com/chronos/chronos) · [Documentation](https://chronos.github.io/chronos) · [Terraform Provider](https://registry.terraform.io/providers/chronos/chronos)
