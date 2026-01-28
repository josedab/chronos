# Troubleshooting Guide

This guide helps diagnose and resolve common issues with Chronos.

## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Startup Issues](#startup-issues)
- [Cluster Issues](#cluster-issues)
- [Job Execution Issues](#job-execution-issues)
- [API Issues](#api-issues)
- [Performance Issues](#performance-issues)
- [Storage Issues](#storage-issues)

---

## Quick Diagnostics

### Health Check

```bash
# Check if Chronos is running
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","leader":true}
```

### Cluster Status

```bash
# Get cluster information
curl http://localhost:8080/api/v1/cluster/status
```

### Logs

```bash
# View recent logs (Docker)
docker logs chronos --tail 100

# View logs (systemd)
journalctl -u chronos -f

# Increase log verbosity
CHRONOS_LOG_LEVEL=debug ./chronos --config chronos.yaml
```

### Metrics

```bash
# Check Prometheus metrics
curl http://localhost:8080/metrics | grep chronos_
```

---

## Startup Issues

### "bind: address already in use"

**Symptom**: Chronos fails to start with port binding error.

**Cause**: Another process is using the configured port.

**Solution**:
```bash
# Find what's using the port
lsof -i :8080
# or
netstat -tlnp | grep 8080

# Either stop the other process or change Chronos port:
# chronos.yaml
server:
  http:
    address: 0.0.0.0:8081
```

### "failed to open badger db"

**Symptom**: Storage initialization fails.

**Cause**: Data directory permissions or corruption.

**Solution**:
```bash
# Check directory permissions
ls -la /var/lib/chronos

# Fix permissions
sudo chown -R chronos:chronos /var/lib/chronos

# If corrupted, backup and recreate
mv /var/lib/chronos /var/lib/chronos.bak
mkdir -p /var/lib/chronos
```

### "configuration file not found"

**Symptom**: Chronos can't find config file.

**Solution**:
```bash
# Specify config path explicitly
./chronos --config /path/to/chronos.yaml

# Or use environment variable
export CHRONOS_CONFIG=/path/to/chronos.yaml
./chronos
```

---

## Cluster Issues

### "no leader elected"

**Symptom**: Cluster has no leader, jobs not executing.

**Causes**:
1. Less than quorum nodes available (need majority)
2. Network connectivity issues between nodes
3. Raft ports blocked by firewall

**Diagnosis**:
```bash
# Check each node's status
curl http://node1:8080/api/v1/cluster/status
curl http://node2:8080/api/v1/cluster/status
curl http://node3:8080/api/v1/cluster/status

# Check Raft metrics
curl http://localhost:8080/metrics | grep raft
```

**Solution**:
```bash
# Verify network connectivity
nc -zv node2 7000  # Raft port
nc -zv node3 7000

# Check firewall rules
sudo iptables -L -n | grep 7000

# Ensure odd number of nodes (3, 5, 7...)
```

### "failed to join cluster"

**Symptom**: New node can't join existing cluster.

**Causes**:
1. Incorrect peer addresses in config
2. Existing cluster doesn't know about new node
3. Node ID conflict

**Solution**:
```yaml
# Ensure peer addresses are correct
cluster:
  node_id: chronos-3  # Must be unique
  raft:
    address: 10.0.0.3:7000
    peers:
      - 10.0.0.1:7000  # Existing nodes
      - 10.0.0.2:7000
```

### Split Brain / Multiple Leaders

**Symptom**: Multiple nodes claim to be leader.

**Cause**: Network partition resolved incorrectly.

**Solution**:
```bash
# Stop all nodes
# Start nodes one by one, starting with the one that has most recent data
# Check Raft term in logs - highest term is most recent

# Nuclear option: Remove Raft state and bootstrap fresh
rm -rf /var/lib/chronos/raft
# Then restart cluster
```

---

## Job Execution Issues

### Jobs Not Running

**Checklist**:

1. **Is the job enabled?**
   ```bash
   curl http://localhost:8080/api/v1/jobs/{id} | jq '.data.enabled'
   ```

2. **Is this node the leader?**
   ```bash
   curl http://localhost:8080/health | jq '.leader'
   ```

3. **Is the schedule correct?**
   ```bash
   # Verify cron expression
   curl http://localhost:8080/api/v1/jobs/{id} | jq '.data.schedule'
   ```

4. **Check next scheduled run**:
   ```bash
   curl http://localhost:8080/api/v1/jobs/{id} | jq '.data.next_run'
   ```

### Webhook Failures

**Symptom**: Jobs execute but webhook fails.

**Diagnosis**:
```bash
# Check execution history
curl http://localhost:8080/api/v1/jobs/{id}/executions | jq '.data[0]'

# Look for:
# - status: "failed"
# - error message
# - HTTP status code
```

**Common causes**:

1. **Target unreachable**:
   ```bash
   # Test connectivity from Chronos host
   curl -v https://api.example.com/endpoint
   ```

2. **SSL/TLS errors**:
   ```bash
   # Check certificate
   openssl s_client -connect api.example.com:443
   ```

3. **Authentication failure**:
   ```bash
   # Verify headers are set correctly
   curl http://localhost:8080/api/v1/jobs/{id} | jq '.data.webhook.headers'
   ```

4. **Timeout**:
   ```yaml
   # Increase job timeout
   timeout: "5m"  # Default might be too short
   ```

### Duplicate Executions

**Symptom**: Same job runs multiple times.

**Causes**:
1. Multiple leaders (split brain) - see cluster issues
2. Concurrency policy set to "allow"
3. Clock skew between nodes

**Solution**:
```yaml
# Set concurrency policy to prevent overlap
concurrency_policy: "forbid"  # or "replace"
```

---

## API Issues

### "unauthorized" (401)

**Cause**: API authentication required but not provided.

**Solution**: Include authentication header if configured.

### "not found" (404)

**Cause**: Job or resource doesn't exist.

**Solution**: Verify the job ID is correct.

### "bad request" (400)

**Cause**: Invalid request body or parameters.

**Solution**: Check API documentation for correct format.
```bash
# Validate JSON
echo '{"name":"test"}' | jq .
```

### Slow API Responses

**Diagnosis**:
```bash
# Time API calls
time curl http://localhost:8080/api/v1/jobs

# Check system resources
top -p $(pgrep chronos)
```

---

## Performance Issues

### High CPU Usage

**Diagnosis**:
```bash
# Profile CPU
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

**Common causes**:
1. Too many jobs scheduling at once
2. Inefficient cron expressions (e.g., `* * * * * *`)
3. Large number of executions being stored

### High Memory Usage

**Diagnosis**:
```bash
# Check memory metrics
curl http://localhost:8080/metrics | grep go_memstats

# Profile heap
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

**Solutions**:
1. Reduce execution history retention
2. Increase BadgerDB value log GC frequency
3. Add memory limits in container/systemd

### Disk Space Issues

**Diagnosis**:
```bash
# Check data directory size
du -sh /var/lib/chronos/*
```

**Solutions**:
```bash
# Trigger BadgerDB garbage collection
# (automatic, but can be tuned)

# Reduce execution history
# Configure max executions per job
```

---

## Storage Issues

### "value log GC didn't result in any cleanup"

**Symptom**: Warning in logs about BadgerDB GC.

**Cause**: Normal when there's little to clean up.

**Solution**: This is usually harmless. If disk usage is high:
```bash
# Compact manually (requires restart)
# Stop Chronos, then:
badger flatten --dir /var/lib/chronos/data
```

### Database Corruption

**Symptom**: Errors reading/writing data.

**Solution**:
```bash
# 1. Stop Chronos
# 2. Backup current data
cp -r /var/lib/chronos /var/lib/chronos.bak

# 3. Try to recover
badger doctor --dir /var/lib/chronos/data

# 4. If unrecoverable, restore from Raft snapshot or peer
# Delete local data, rejoin cluster
rm -rf /var/lib/chronos/data
# Restart - will sync from leader
```

---

## Getting Help

If you can't resolve your issue:

1. **Search existing issues**: [GitHub Issues](https://github.com/chronos/chronos/issues)

2. **Collect diagnostics**:
   ```bash
   # Create diagnostic bundle
   mkdir chronos-diag
   curl http://localhost:8080/health > chronos-diag/health.json
   curl http://localhost:8080/api/v1/cluster/status > chronos-diag/cluster.json
   curl http://localhost:8080/metrics > chronos-diag/metrics.txt
   journalctl -u chronos --since "1 hour ago" > chronos-diag/logs.txt
   tar czf chronos-diag.tar.gz chronos-diag/
   ```

3. **Open an issue** with:
   - Chronos version
   - Deployment type (binary/Docker/Kubernetes)
   - Configuration (sanitized)
   - Diagnostic bundle
   - Steps to reproduce
