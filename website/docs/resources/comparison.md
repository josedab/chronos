---
sidebar_position: 1
title: Comparison
description: How Chronos compares to other scheduling solutions
---

# Chronos vs Alternatives

## Comparison Table

| Feature | Chronos | Airflow | Temporal | Linux Cron |
|---------|---------|---------|----------|------------|
| Dependencies | Zero | Many | External | None |
| HA | Built-in | Requires setup | Built-in | None |
| UI | Included | Included | Included | None |
| Protocol | Multi | Python | SDK | Shell |
| Complexity | Low | High | Medium | Low |

## When to Choose Chronos

- Zero operational overhead
- Simple webhook-based jobs
- Distributed, fault-tolerant scheduling
- Quick setup without external dependencies

## When to Consider Alternatives

- **Airflow**: Complex Python-based DAGs
- **Temporal**: Long-running workflows with state
- **Cron**: Single-machine, shell scripts only
