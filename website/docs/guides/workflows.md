---
sidebar_position: 5
title: Workflows
description: Create DAG-based workflows in Chronos
---

# Workflows

Orchestrate complex job dependencies with DAG-based workflows.

## Example

```yaml
name: data-pipeline
jobs:
  - id: extract
    webhook:
      url: https://api.example.com/extract
  - id: transform
    depends_on: [extract]
    webhook:
      url: https://api.example.com/transform
```
