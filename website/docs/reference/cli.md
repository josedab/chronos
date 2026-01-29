---
sidebar_position: 2
title: CLI Reference
description: chronosctl command-line interface reference
---

# CLI Reference

The `chronosctl` CLI provides a convenient way to interact with Chronos.

## Installation

```bash
# Installed with Chronos
chronosctl --version
```

## Configuration

```bash
# Set server address
export CHRONOS_SERVER=http://localhost:8080

# Or use flag
chronosctl --server http://localhost:8080 job list
```

## Commands

### Jobs

```bash
# List all jobs
chronosctl job list

# Get job details
chronosctl job get <job-id>

# Create from file
chronosctl job create -f job.yaml

# Update
chronosctl job update <job-id> --schedule "0 9 * * *"

# Delete
chronosctl job delete <job-id>

# Trigger manually
chronosctl job trigger <job-id>

# Enable/Disable
chronosctl job enable <job-id>
chronosctl job disable <job-id>
```

### Executions

```bash
# List executions
chronosctl execution list <job-id>

# Get execution details
chronosctl execution get <job-id> <execution-id>
```

### Cluster

```bash
# Cluster status
chronosctl cluster status

# List nodes
chronosctl cluster nodes
```
