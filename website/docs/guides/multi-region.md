---
sidebar_position: 6
title: Multi-Region
description: Deploy Chronos across multiple regions with federation
---

# Multi-Region Federation

Deploy Chronos across multiple regions for global availability.

## Configuration

```yaml
federation:
  enabled: true
  mode: replicate
  clusters:
    - name: us-east
      endpoint: https://chronos-east.example.com
```
