---
sidebar_position: 1
title: Comparison
description: How Chronos compares to other scheduling solutions
---

# Chronos vs Alternatives

Choosing a job scheduler is a critical infrastructure decision. This guide provides an honest comparison of Chronos against popular alternatives to help you make the right choice for your use case.

## Quick Decision Matrix

| If you need... | Choose |
|----------------|--------|
| Simple cron replacement with HA | **Chronos** |
| Zero operational overhead | **Chronos** |
| Complex Python DAG workflows | Airflow |
| Long-running stateful workflows | Temporal |
| Single-machine shell scripts | Linux Cron |
| Kubernetes-native simple jobs | Kubernetes CronJobs |

## Detailed Comparison

### Feature Matrix

| Feature | Chronos | Airflow | Temporal | Kubernetes CronJobs | Linux Cron |
|---------|---------|---------|----------|---------------------|------------|
| **Setup Time** | < 5 min | Hours | 30 min | 10 min | < 1 min |
| **Dependencies** | Zero | PostgreSQL, Redis, etc. | PostgreSQL/MySQL/Cassandra | Kubernetes cluster | None |
| **High Availability** | Built-in (Raft) | Requires setup | Built-in | Depends on cluster | None |
| **Failover Time** | ~5 seconds | Minutes | ~10 seconds | Pod restart time | N/A |
| **Web UI** | ✓ Included | ✓ Included | ✓ Included | ✗ (use kubectl) | ✗ |
| **Retry Policies** | ✓ Configurable | ✓ Configurable | ✓ Advanced | Limited | ✗ |
| **Execution Guarantee** | At-least-once | At-least-once | Exactly-once possible | At-least-once | Best-effort |
| **Language Support** | Any (HTTP) | Python | SDK (Go, Java, etc.) | Any container | Shell |
| **Monitoring** | Prometheus native | Prometheus + custom | Prometheus native | Prometheus | Manual |
| **Resource Usage** | ~50MB RAM | 2GB+ RAM | 500MB+ RAM | Varies | Minimal |

### Operational Complexity

| Aspect | Chronos | Airflow | Temporal | K8s CronJobs |
|--------|---------|---------|----------|--------------|
| **Components to manage** | 1 binary | 5+ services | 3+ services | kubectl + etcd |
| **Database required** | No (embedded) | Yes (PostgreSQL) | Yes | Yes (etcd) |
| **Message queue required** | No | Often (Redis/RabbitMQ) | No | No |
| **Upgrade complexity** | Rolling restart | Coordinated migration | Rolling restart | kubectl apply |
| **Backup strategy** | Raft snapshots | DB backup + DAG files | DB backup | etcd backup |

## When to Choose Chronos

### ✅ Chronos is ideal when you need:

**1. Simple, reliable job scheduling**
```yaml
# This is all you need
name: daily-backup
schedule: "0 2 * * *"
webhook:
  url: https://api.example.com/backup
```

**2. Zero operational overhead**
- Single binary deployment
- No external databases to manage
- No message queues to monitor
- Built-in clustering with Raft consensus

**3. Fast time-to-production**
- Download binary → Configure → Run
- Production-ready HA in under 15 minutes
- No complex orchestration required

**4. Predictable resource usage**
- ~50MB memory baseline
- Linear scaling with job count
- No "noisy neighbor" issues

**5. HTTP/webhook-first architecture**
- Language-agnostic job execution
- Works with any HTTP endpoint
- Easy integration with existing services

### ✅ Real-world Chronos use cases:

- Triggering scheduled API calls
- Database backup orchestration
- Report generation and delivery
- Cache warming and invalidation
- Health check monitoring
- Data synchronization between services
- Scheduled notifications
- Cleanup and maintenance jobs

## When to Choose Alternatives

### Airflow: Choose when you need...

**Complex Python-based DAG workflows**
```python
# Airflow excels at complex dependency graphs
with DAG('etl_pipeline') as dag:
    extract = PythonOperator(...)
    transform = PythonOperator(...)
    load = PythonOperator(...)
    extract >> transform >> load
```

**Built-in operators for data tools**
- Native Spark, Hadoop, AWS, GCP operators
- Complex branching and conditional logic
- Extensive Python ecosystem integration

**When operational overhead is acceptable**
- You have a dedicated platform team
- You need complex DAG visualization
- Python is your primary language

### Temporal: Choose when you need...

**Long-running stateful workflows**
```go
// Temporal handles hours/days-long workflows
func OrderWorkflow(ctx workflow.Context, order Order) error {
    // This workflow can run for days with state preserved
    workflow.Sleep(ctx, 24*time.Hour)
    return processRefund(ctx, order)
}
```

**Exactly-once execution guarantees**
- Financial transactions
- Order processing
- Complex saga patterns

**SDK-first development**
- Strong typing in Go, Java, TypeScript
- Version-safe workflow updates
- Replay-based debugging

### Kubernetes CronJobs: Choose when you need...

**Native Kubernetes integration**
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: backup
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: myapp:backup
```

**Container-based job execution**
- Your jobs are containerized applications
- You want jobs to run in the same cluster
- You already have robust Kubernetes operations

**Simple scheduling needs**
- Basic cron expressions
- Jobs that need cluster resources
- Integration with Kubernetes RBAC

### Linux Cron: Choose when you need...

**Single-machine simplicity**
```bash
# /etc/crontab
0 2 * * * root /usr/local/bin/backup.sh
```

**Minimal requirements**
- Shell script execution only
- No high availability needed
- Resource-constrained environment

## Migration Paths

### From Linux Cron to Chronos

1. **Wrap scripts as HTTP endpoints**
   ```python
   # Flask example
   @app.route('/jobs/backup', methods=['POST'])
   def backup():
       subprocess.run(['/usr/local/bin/backup.sh'])
       return {'status': 'ok'}
   ```

2. **Create corresponding Chronos jobs**
   ```bash
   curl -X POST http://chronos:8080/api/v1/jobs -d '{
     "name": "backup",
     "schedule": "0 2 * * *",
     "webhook": {"url": "http://scripts-service/jobs/backup"}
   }'
   ```

3. **Remove crontab entries after verification**

### From Kubernetes CronJobs to Chronos

1. **Keep your container images**
2. **Create a thin HTTP wrapper or use job runner service**
3. **Migrate job definitions to Chronos API**
4. **Benefits: Better observability, retry policies, Web UI**

### From Airflow to Chronos

Chronos is not a direct Airflow replacement. Migrate when:
- Your DAGs are simple linear sequences
- You're not using Airflow operators extensively
- Operational overhead is a pain point

Keep Airflow for complex ETL pipelines.

## Performance Benchmarks

### Test Environment

All benchmarks were run on a standardized test environment:

| Component | Specification |
|-----------|---------------|
| Cluster | 3 nodes |
| CPU | 4 vCPU (AMD EPYC) per node |
| Memory | 8GB RAM per node |
| Storage | NVMe SSD |
| Network | 10 Gbps, &lt;1ms latency between nodes |
| OS | Ubuntu 22.04 LTS |

### Scheduling Throughput

| Metric | Chronos | Airflow | Temporal |
|--------|---------|---------|----------|
| Jobs scheduled/sec | 10,000+ | ~500 | 5,000+ |
| P99 scheduling latency | 5ms | 100ms+ | 10ms |
| Memory (10k jobs) | 200MB | 4GB+ | 1GB |
| Failover time | 5s | 60s+ | 10s |

### Detailed Results

**Job Creation Throughput**
```
Chronos:    10,247 jobs/sec (P50: 0.8ms, P99: 5ms)
Temporal:    5,123 jobs/sec (P50: 2ms, P99: 10ms)
Airflow:       487 jobs/sec (P50: 45ms, P99: 180ms)
```

**Steady-State Memory Usage (10,000 active jobs)**
```
Chronos:     198 MB (all-inclusive)
Temporal:    892 MB (server only, excludes DB)
Airflow:   4,200 MB (scheduler + webserver + workers)
```

**Failover Recovery Time**
```
Chronos:    4.8s  (Raft leader election)
Temporal:   9.2s  (shard rebalancing)
Airflow:   65.0s  (worker heartbeat timeout + task re-queue)
```

### Methodology Notes

- Airflow tested with LocalExecutor (CeleryExecutor would use more resources)
- Temporal tested with default configuration
- All tests used PostgreSQL 15 where applicable
- Job execution was simulated (no actual webhook calls)
- Tests ran for 10 minutes; results are steady-state averages

## Cost Comparison

**Monthly infrastructure cost for 1,000 scheduled jobs:**

| Solution | Infrastructure | Typical Cost |
|----------|---------------|--------------|
| Chronos (3-node HA) | 3x t3.small | ~$45/month |
| Airflow (minimal HA) | 5x t3.medium + RDS | ~$300/month |
| Temporal (self-hosted) | 3x t3.medium + RDS | ~$200/month |
| Kubernetes CronJobs | Existing cluster | $0 incremental |

## Summary

**Choose Chronos if:**
- You want the simplest path to reliable job scheduling
- Operational simplicity is a priority
- Your jobs are HTTP/webhook-based
- You need HA without complex setup

**Choose something else if:**
- You need complex Python DAG workflows (→ Airflow)
- You need long-running stateful workflows (→ Temporal)
- Your jobs must run as containers in K8s (→ CronJobs)
- You only need single-machine scheduling (→ Linux cron)

---

:::tip Still not sure?
Start with Chronos. If you outgrow it, migration is straightforward. Most teams find that Chronos handles 90% of their scheduling needs with 10% of the operational burden.
:::

## Next Steps

- [Quick Start Guide](/docs/getting-started/quickstart) - Get Chronos running in 5 minutes
- [Architecture Overview](/docs/core-concepts/architecture) - Understand how Chronos works
- [High Availability Guide](/docs/guides/high-availability) - Deploy a production cluster
