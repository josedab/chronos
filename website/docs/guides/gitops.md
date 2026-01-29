---
sidebar_position: 7
title: GitOps
description: Manage Chronos jobs with Git-based configuration
---

# GitOps

Manage job configurations through Git repositories.

## Setup

```yaml
gitops:
  enabled: true
  repository: https://github.com/org/chronos-jobs
  branch: main
  sync_interval: 60s
```
