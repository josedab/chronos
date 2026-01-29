---
sidebar_position: 3
title: FAQ
description: Frequently asked questions about Chronos
---

# FAQ

## General

### What is Chronos?
Chronos is a distributed cron system for reliable job scheduling.

### How is it different from regular cron?
- High availability with automatic failover
- Distributed across multiple nodes
- Built-in monitoring and Web UI
- Retry policies and webhook dispatch

## Operations

### How many nodes do I need?
- Development: 1 node
- Production: 3 nodes (minimum)
- High availability: 5 nodes

### What happens during failover?
Leader election takes ~5 seconds. Jobs may be slightly delayed but won't be lost.

### How do I backup Chronos?
Raft snapshots are created automatically. Export via API for portability.
