---
sidebar_position: 2
title: Troubleshooting
description: Common issues and solutions for Chronos
---

# Troubleshooting

This guide covers common issues you may encounter when running Chronos and how to resolve them.

## Quick Diagnostics

Run these commands first to gather diagnostic information:

```bash
# Health check
curl http://localhost:8080/health

# Cluster status
curl http://localhost:8080/api/v1/cluster/status | jq

# Recent logs
docker logs chronos --tail 100 2>&1 | grep -E "(error|warn|fatal)"

# Metrics snapshot
curl -s http://localhost:8080/metrics | grep -E "^chronos_"
```

---

## Job Issues

### Jobs Not Running

**Symptoms:** Jobs are enabled but not executing at scheduled times.

**Diagnostic Steps:**

1. **Verify the job is enabled:**
   ```bash
   chronosctl job get <job-name>
   # Check: enabled: true
   ```

2. **Check if this node is the leader:**
   ```bash
   curl http://localhost:8080/health
   # Response should show "leader": true
   ```

3. **Verify schedule syntax:**
   ```bash
   # Test cron expression
   chronosctl job validate-schedule "0 * * * *"
   ```

4. **Check next_run time:**
   ```bash
   chronosctl job get <job-name> | grep next_run
   ```

**Solutions:**

| Cause | Solution |
|-------|----------|
| Job is disabled | `chronosctl job enable <job-name>` |
| Node is not leader | Query the leader node instead |
| Invalid schedule | Fix cron expression syntax |
| Timezone mismatch | Set correct `timezone` in job config |

### Jobs Running Multiple Times

**Symptoms:** Same job execution triggers multiple times.

**Causes and Solutions:**

1. **Concurrency policy set to `allow`:**
   ```json
   {
     "concurrency": "forbid"  // Change from "allow"
   }
   ```

2. **Clock drift between nodes:**
   - Sync time with NTP
   - Check `chronos_scheduler_tick_duration_seconds` metric

3. **Multiple clusters pointing to same targets:**
   - Verify cluster configuration
   - Check for duplicate deployments

### Jobs Skipped

**Symptoms:** Jobs show status `skipped` in execution history.

**This is normal behavior when:**
- Previous execution is still running (with `concurrency: forbid`)
- Job was disabled during scheduled time

**Check concurrency policy:**
```bash
chronosctl job get <job-name> | grep concurrency
```

**Options:**
- Set `concurrency: allow` if parallel runs are acceptable
- Set `concurrency: replace` to cancel previous and start new
- Increase job timeout if executions are taking too long

---

## Webhook Failures

### Connection Refused

**Error:** `connection refused` or `no route to host`

**Diagnostic Steps:**

```bash
# Test connectivity from Chronos server
curl -v https://api.example.com/endpoint

# Check DNS resolution
nslookup api.example.com

# Test with netcat
nc -zv api.example.com 443
```

**Solutions:**

| Cause | Solution |
|-------|----------|
| Target service down | Check target service health |
| Firewall blocking | Open outbound connections |
| DNS issues | Check DNS configuration |
| Wrong URL | Verify webhook URL in job config |

### SSL/TLS Errors

**Error:** `certificate verify failed` or `SSL handshake error`

**Solutions:**

1. **Self-signed certificates:** Configure CA bundle:
   ```yaml
   dispatcher:
     http:
       tls:
         ca_file: /etc/chronos/ca-bundle.crt
         insecure_skip_verify: false  # Only for testing!
   ```

2. **Expired certificate:** Check target certificate:
   ```bash
   echo | openssl s_client -connect api.example.com:443 2>/dev/null | openssl x509 -noout -dates
   ```

3. **SNI issues:** Ensure target supports SNI

### Timeout Errors

**Error:** `context deadline exceeded` or `timeout`

**Diagnostic Steps:**

```bash
# Check webhook timeout settings
chronosctl job get <job-name> | grep timeout

# Test endpoint response time
time curl -w "%{time_total}\n" -o /dev/null https://api.example.com/endpoint
```

**Solutions:**

1. **Increase timeout:**
   ```json
   {
     "webhook": {
       "timeout": "60s"
     },
     "timeout": "5m"
   }
   ```

2. **Check target service performance**

3. **Add retry policy:**
   ```json
   {
     "retry_policy": {
       "max_attempts": 3,
       "initial_interval": "5s"
     }
   }
   ```

### Authentication Failures

**Error:** `401 Unauthorized` or `403 Forbidden`

**Diagnostic Steps:**

```bash
# Test authentication manually
curl -H "Authorization: Bearer <token>" https://api.example.com/endpoint
```

**Solutions:**

| Cause | Solution |
|-------|----------|
| Expired token | Refresh/rotate credentials |
| Wrong auth type | Check auth configuration format |
| Missing headers | Add required headers to webhook config |

**Correct auth configuration:**

```json
{
  "webhook": {
    "url": "https://api.example.com/endpoint",
    "auth": {
      "type": "bearer",
      "token": "your-token"
    }
  }
}
```

### HTTP 4xx/5xx Errors

**Check execution details:**

```bash
chronosctl execution get <job-name> <execution-id>
```

**Response code meanings:**

| Code | Meaning | Action |
|------|---------|--------|
| 400 | Bad request | Check request body/headers |
| 401 | Unauthorized | Check authentication |
| 403 | Forbidden | Check permissions |
| 404 | Not found | Verify URL path |
| 429 | Rate limited | Add backoff, reduce frequency |
| 500 | Server error | Check target logs |
| 502/503 | Service unavailable | Target service issue |

---

## Cluster Issues

### No Leader Elected

**Symptoms:** All nodes show `leader: false`, jobs not running.

**Diagnostic Steps:**

```bash
# Check each node
for i in 1 2 3; do
  echo "=== Node $i ==="
  curl -s http://chronos-$i:8080/api/v1/cluster/status | jq '.data.state'
done

# Check Raft logs
grep -i "election\|leader\|vote" /var/log/chronos/chronos.log
```

**Common Causes:**

1. **Network partition:**
   ```bash
   # Test connectivity between nodes
   nc -zv chronos-2 7000
   nc -zv chronos-3 7000
   ```

2. **Insufficient nodes for quorum:**
   - 3-node cluster needs 2 nodes for quorum
   - 5-node cluster needs 3 nodes for quorum

3. **Raft port blocked:**
   - Ensure port 7000 (default) is open between nodes

**Solutions:**

```bash
# Restart nodes one at a time
systemctl restart chronos  # On each node, wait for stabilization

# Force bootstrap (last resort, single node)
chronos --raft-bootstrap
```

### Split Brain (Multiple Leaders)

**Symptoms:** Multiple nodes claim to be leader.

**This is a critical situation!**

**Immediate Actions:**

1. **Stop all but one node:**
   ```bash
   # Keep chronos-1, stop others
   ssh chronos-2 systemctl stop chronos
   ssh chronos-3 systemctl stop chronos
   ```

2. **Verify single leader:**
   ```bash
   curl http://chronos-1:8080/health
   ```

3. **Restart other nodes one at a time:**
   ```bash
   ssh chronos-2 systemctl start chronos
   # Wait for it to join as follower
   sleep 30
   ssh chronos-3 systemctl start chronos
   ```

**Root Cause Investigation:**
- Check network connectivity
- Review firewall rules
- Look for asymmetric network issues

### Node Won't Join Cluster

**Error:** `failed to join cluster` or `peer not found`

**Diagnostic Steps:**

```bash
# Check Raft address configuration
grep -A5 "raft:" /etc/chronos/chronos.yaml

# Test connectivity to existing peers
curl http://chronos-1:8080/api/v1/cluster/status
```

**Common Causes:**

| Cause | Solution |
|-------|----------|
| Wrong peer addresses | Update `raft.peers` in config |
| Node ID conflict | Ensure unique `node_id` per node |
| Data directory issues | Clear data dir for fresh join |
| TLS mismatch | Ensure consistent TLS config |

**Fresh join procedure:**

```bash
# On the new node
rm -rf /var/lib/chronos/raft/*  # Clear Raft data
systemctl restart chronos
```

### Slow Failover

**Symptoms:** Leader election takes more than 10 seconds.

**Tune Raft timeouts:**

```yaml
cluster:
  raft:
    heartbeat_timeout: 500ms    # Reduce from default
    election_timeout: 1s        # Reduce from default
    leader_lease_timeout: 400ms
```

**Note:** Lower values mean faster failover but more sensitive to network jitter.

---

## Performance Issues

### High Memory Usage

**Diagnostic Steps:**

```bash
# Check current memory
curl -s http://localhost:8080/metrics | grep process_resident_memory_bytes

# Check job count
curl -s http://localhost:8080/metrics | grep chronos_jobs_total

# Check execution history size
du -sh /var/lib/chronos/
```

**Solutions:**

1. **Enable execution history cleanup:**
   ```yaml
   storage:
     execution_retention: 168h  # Keep 7 days
     cleanup_interval: 1h
   ```

2. **Tune BadgerDB:**
   ```yaml
   storage:
     badger:
       value_log_gc_enabled: true
       gc_interval: 10m
   ```

3. **Reduce concurrent executions:**
   ```yaml
   scheduler:
     max_concurrent_executions: 100
   ```

### Slow Scheduler Ticks

**Symptom:** `chronos_scheduler_tick_duration_seconds` is high.

**Diagnostic Steps:**

```bash
# Check tick duration
curl -s http://localhost:8080/metrics | grep scheduler_tick
```

**Solutions:**

1. **Increase tick interval (trade-off: less precision):**
   ```yaml
   scheduler:
     tick_interval: 5s  # Increase from 1s
   ```

2. **Reduce number of jobs**

3. **Check storage performance:**
   ```bash
   # Benchmark disk
   dd if=/dev/zero of=/var/lib/chronos/test bs=1M count=100 conv=fdatasync
   ```

### High Disk I/O

**Diagnostic Steps:**

```bash
# Check disk usage
iostat -x 1 5

# Check Raft log size
du -sh /var/lib/chronos/raft/
```

**Solutions:**

1. **Configure snapshots:**
   ```yaml
   cluster:
     raft:
       snapshot_interval: 30s
       snapshot_threshold: 8192
   ```

2. **Move data directory to SSD**

---

## Startup Issues

### Failed to Bind Port

**Error:** `listen tcp :8080: bind: address already in use`

**Solution:**

```bash
# Find process using the port
lsof -i :8080

# Kill or stop the conflicting process
kill <pid>

# Or change Chronos port
chronos --http-address 0.0.0.0:8081
```

### Data Directory Permission Denied

**Error:** `open /var/lib/chronos/...: permission denied`

**Solution:**

```bash
# Fix ownership
chown -R chronos:chronos /var/lib/chronos/

# Fix permissions
chmod 755 /var/lib/chronos/
```

### Invalid Configuration

**Error:** `failed to parse config: ...`

**Validate configuration:**

```bash
chronos config validate -f /etc/chronos/chronos.yaml
```

**Common issues:**
- YAML indentation errors
- Invalid duration format (use `1s`, `5m`, `1h`)
- Missing required fields

---

## Recovery Procedures

### Restore from Backup

1. **Stop all nodes:**
   ```bash
   systemctl stop chronos
   ```

2. **Restore data directory:**
   ```bash
   rm -rf /var/lib/chronos/*
   tar -xzf chronos-backup.tar.gz -C /var/lib/chronos/
   ```

3. **Start one node as bootstrap:**
   ```bash
   chronos --raft-bootstrap
   ```

4. **Join other nodes:**
   ```bash
   systemctl start chronos  # On other nodes
   ```

### Reset Cluster State

**⚠️ Warning: This will lose all data!**

```bash
# On all nodes
systemctl stop chronos
rm -rf /var/lib/chronos/*

# Bootstrap first node
chronos --raft-bootstrap

# Join others
systemctl start chronos  # On other nodes
```

### Export/Import Jobs

**Export:**
```bash
chronosctl job export > jobs-backup.json
```

**Import:**
```bash
chronosctl job import < jobs-backup.json
```

---

## Getting Help

If you can't resolve the issue:

1. **Gather diagnostic info:**
   ```bash
   chronosctl debug bundle > chronos-debug.tar.gz
   ```

2. **Check GitHub Issues:** [github.com/chronos/chronos/issues](https://github.com/chronos/chronos/issues)

3. **Ask on Discord:** [discord.gg/chronos](https://discord.gg/chronos)

4. **Community Forum:** [github.com/chronos/chronos/discussions](https://github.com/chronos/chronos/discussions)

When reporting issues, include:
- Chronos version (`chronos --version`)
- OS and architecture
- Configuration (redact secrets)
- Relevant logs
- Steps to reproduce
