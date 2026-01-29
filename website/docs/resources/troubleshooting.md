---
sidebar_position: 2
title: Troubleshooting
description: Common issues and solutions for Chronos
---

# Troubleshooting

## Quick Diagnostics

```bash
# Health check
curl http://localhost:8080/health

# Cluster status
curl http://localhost:8080/api/v1/cluster/status

# View logs
docker logs chronos --tail 100
```

## Common Issues

### Jobs Not Running

1. Check if job is enabled
2. Verify this node is the leader
3. Check schedule syntax

### Webhook Failures

1. Test connectivity to target
2. Check authentication headers
3. Verify SSL certificates

### Cluster Issues

1. Check node connectivity
2. Verify Raft ports are open
3. Ensure odd number of nodes

See the full [Troubleshooting Guide](/docs/resources/faq) for more.
