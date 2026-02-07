---
sidebar_position: 3
title: FAQ
description: Frequently asked questions about Chronos
---

# Frequently Asked Questions

## General

### What is Chronos?

Chronos is a distributed cron system designed for reliable job scheduling across distributed infrastructure. It replaces traditional cron with a highly available, fault-tolerant solution that guarantees at-least-once execution.

### How is Chronos different from regular cron?

| Feature | Linux Cron | Chronos |
|---------|-----------|---------|
| High availability | ❌ Single point of failure | ✅ Built-in clustering |
| Distributed | ❌ Single machine only | ✅ Multi-node with Raft |
| Monitoring | ❌ Manual log parsing | ✅ Web UI, Prometheus metrics |
| Retry policies | ❌ None | ✅ Configurable exponential backoff |
| Execution history | ❌ None | ✅ Full history with logs |
| API access | ❌ File-based only | ✅ REST API and CLI |

### Is Chronos open source?

Yes! Chronos is licensed under Apache 2.0. The source code is available at [github.com/chronos/chronos](https://github.com/chronos/chronos).

### What languages/frameworks does Chronos support?

Chronos is language-agnostic. Jobs are triggered via HTTP webhooks, so any service that can receive HTTP requests can be scheduled—regardless of programming language or framework.

---

## Architecture

### How does Chronos achieve high availability?

Chronos uses [HashiCorp Raft](https://github.com/hashicorp/raft) for distributed consensus. In a 3-node cluster:

- One node is elected leader and runs the scheduler
- Followers replicate all data
- If the leader fails, a new leader is elected in ~5 seconds
- No external coordination service required

### What happens during leader failover?

1. Followers detect leader failure (heartbeat timeout)
2. Election begins among remaining nodes
3. New leader is elected (typically 3-5 seconds)
4. New leader resumes scheduling
5. Jobs may be slightly delayed but not lost

### Why does Chronos use embedded storage instead of an external database?

Benefits of embedded BadgerDB:
- **Zero dependencies**: No external database to manage
- **Simpler operations**: Fewer failure points
- **Lower latency**: No network hop for storage
- **Easy deployment**: Single binary with data

Trade-offs:
- Storage tied to nodes (Raft handles replication)
- No query language (API provides filtered access)

### Can I use Chronos with an external database?

Currently, Chronos only supports embedded BadgerDB. External database support (PostgreSQL, MySQL) is on the roadmap for enterprise deployments requiring separate storage scaling.

---

## Deployment

### How many nodes do I need?

| Environment | Nodes | Fault Tolerance |
|------------|-------|-----------------|
| Development | 1 | None |
| Production | 3 | 1 node failure |
| High availability | 5 | 2 node failures |

Always use an **odd number** of nodes for proper quorum.

### Can I run Chronos in a single-node configuration?

Yes, for development or non-critical workloads. However, single-node mode provides no high availability—if the node fails, jobs stop running.

### What are the resource requirements?

**Minimum (per node):**
- 1 CPU core
- 512MB RAM
- 1GB disk

**Recommended (per node):**
- 2 CPU cores
- 2GB RAM
- 10GB SSD

Actual requirements depend on job count and execution frequency.

### Does Chronos support Kubernetes?

Yes! We provide:
- [Helm chart](https://github.com/chronos/charts) for easy deployment
- StatefulSet configuration for persistent storage
- ServiceMonitor for Prometheus integration

```bash
helm repo add chronos https://chronos.github.io/charts
helm install chronos chronos/chronos
```

---

## Jobs & Scheduling

### What schedule formats does Chronos support?

**Standard cron expressions:**
```
* * * * *       # Every minute
0 * * * *       # Every hour
0 0 * * *       # Every day at midnight
0 0 * * 0       # Every Sunday at midnight
0 9-17 * * 1-5  # Hourly, 9am-5pm, Mon-Fri
```

**Predefined schedules:**
```
@hourly         # Every hour (0 * * * *)
@daily          # Every day at midnight
@weekly         # Every Sunday at midnight
@monthly        # First day of every month
@yearly         # January 1st
```

**Interval syntax:**
```
@every 5m       # Every 5 minutes
@every 1h30m    # Every 1.5 hours
@every 24h      # Every 24 hours
```

### How do timezones work?

Jobs can specify an IANA timezone:

```json
{
  "schedule": "0 9 * * *",
  "timezone": "America/New_York"
}
```

If no timezone is specified, UTC is used. Chronos handles daylight saving time transitions automatically.

### What happens if a job takes longer than expected?

Depends on the concurrency policy:

| Policy | Behavior |
|--------|----------|
| `allow` | New execution starts anyway |
| `forbid` (default) | New execution is skipped |
| `replace` | Running execution is cancelled, new one starts |

### Can I trigger a job manually?

Yes, via API or CLI:

```bash
# CLI
chronosctl job trigger my-job

# API
curl -X POST http://localhost:8080/api/v1/jobs/my-job/trigger
```

### How do I pass data to my job?

Jobs can include static data in the webhook body:

```json
{
  "webhook": {
    "url": "https://api.example.com/process",
    "method": "POST",
    "body": "{\"type\": \"daily\", \"source\": \"chronos\"}"
  }
}
```

For dynamic data, your webhook handler can query external sources.

### Does Chronos support job dependencies?

Basic dependency chains can be implemented by having one job's webhook trigger the next. For complex DAG workflows, consider [Airflow](/docs/resources/comparison) or our upcoming workflow feature.

---

## Reliability

### What does "at-least-once execution" mean?

Chronos guarantees that every scheduled execution will be attempted at least once, even if:
- A node fails mid-execution
- Network issues occur
- The scheduler restarts

In rare cases (node failure during webhook call), a job might execute twice. Design your handlers to be idempotent.

### How do I make my job handlers idempotent?

```python
# Good: Check if work already done
def handle_report(request):
    report_id = f"report-{date.today()}"
    if report_exists(report_id):
        return {"status": "already_done"}
    generate_report(report_id)
    return {"status": "completed"}

# Bad: No idempotency check
def handle_report(request):
    generate_report()  # May create duplicates
```

### How are failed jobs retried?

Configure retry policy per job:

```json
{
  "retry_policy": {
    "max_attempts": 5,
    "initial_interval": "10s",
    "max_interval": "5m",
    "multiplier": 2.0
  }
}
```

This retries at: 10s, 20s, 40s, 80s, 5m (capped by max_interval).

### What HTTP status codes trigger retries?

- **2xx**: Success, no retry
- **4xx**: Client error, **no retry** (except 429)
- **429**: Rate limited, **retry** with backoff
- **5xx**: Server error, **retry**
- **Timeout**: **Retry**
- **Connection refused**: **Retry**

---

## Operations

### How do I backup Chronos?

**Automatic:** Raft creates periodic snapshots in the data directory.

**Manual export:**
```bash
chronosctl job export > jobs-backup.json
```

**Full backup:**
```bash
tar -czf chronos-backup.tar.gz /var/lib/chronos/
```

### How do I upgrade Chronos?

**Rolling upgrade (recommended):**

1. Upgrade followers first, one at a time
2. Wait for each to rejoin the cluster
3. Upgrade the leader last (triggers failover)

```bash
# On each follower
systemctl stop chronos
# Replace binary
systemctl start chronos
# Wait for "joined cluster" in logs

# Then on leader
systemctl stop chronos
# Replace binary
systemctl start chronos
```

### How do I add a node to an existing cluster?

1. Configure the new node with existing peer addresses
2. Start the node—it will join automatically

```yaml
cluster:
  node_id: chronos-4
  raft:
    address: 10.0.0.4:7000
    peers:
      - 10.0.0.1:7000
      - 10.0.0.2:7000
      - 10.0.0.3:7000
```

### How do I remove a node from the cluster?

```bash
# On the leader
chronosctl cluster remove-peer chronos-4
```

---

## Security

### Does Chronos support authentication?

Yes, when enabled:
- **API keys**: For machine-to-machine access
- **JWT tokens**: For user authentication
- **Basic auth**: For simple setups

```yaml
security:
  auth:
    enabled: true
    api_key_header: X-API-Key
```

### Does Chronos support TLS?

Yes, for both:
- **Inbound** (API server)
- **Internal** (Raft communication)

```yaml
server:
  tls:
    enabled: true
    cert_file: /etc/chronos/tls/server.crt
    key_file: /etc/chronos/tls/server.key
```

### How are webhook credentials stored?

Credentials in job configurations are:
- Encrypted at rest in BadgerDB
- Never logged
- Masked in API responses

For production, use secret references:
```json
{
  "webhook": {
    "auth": {
      "type": "bearer",
      "token_ref": "vault:secret/chronos/api-tokens#my-service"
    }
  }
}
```

---

## Troubleshooting

### Jobs aren't running. What should I check?

1. Is the job enabled? `chronosctl job get <name>`
2. Is this node the leader? `curl http://localhost:8080/health`
3. Is the schedule valid? `chronosctl job validate-schedule "<schedule>"`
4. Check logs for errors

See [Troubleshooting Guide](/docs/resources/troubleshooting) for detailed diagnostics.

### Why do I see "not leader" errors?

Write operations (create/update/delete) must go through the leader. Either:
- Direct requests to the leader node
- Use a load balancer that routes writes to the leader
- The node will forward automatically if configured

### How do I debug webhook failures?

```bash
# Get execution details
chronosctl execution get <job-name> <execution-id>

# View recent failures
chronosctl execution list <job-name> --status failed --limit 5
```

---

## Performance

### How many jobs can Chronos handle?

Benchmarks on a 3-node cluster (4 vCPU, 8GB RAM each):

| Metric | Value |
|--------|-------|
| Jobs | 100,000+ |
| Scheduling throughput | 10,000+ jobs/sec |
| P99 scheduling latency | 5ms |
| Memory per 10k jobs | ~200MB |

### How can I improve performance?

1. **Use SSDs** for the data directory
2. **Increase tick interval** if sub-second precision isn't needed
3. **Tune Raft** timeouts for your network
4. **Add nodes** for read scaling

---

## Getting Help

### Where can I ask questions?

- **GitHub Discussions**: [Ask questions](https://github.com/chronos/chronos/discussions)
- **Discord**: [Real-time chat](https://discord.gg/chronos)
- **Stack Overflow**: Tag with `chronos-cron`

### How do I report a bug?

[Open an issue](https://github.com/chronos/chronos/issues/new?template=bug_report.md) with:
- Chronos version
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs

### How can I contribute?

See our [Contributing Guide](/docs/resources/contributing) for details on:
- Development setup
- Code style
- Pull request process
