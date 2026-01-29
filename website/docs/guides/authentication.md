---
sidebar_position: 4
title: Authentication
description: Configure authentication and authorization in Chronos
---

# Authentication

Secure your Chronos deployment with authentication and RBAC.

## API Key Authentication

```yaml
auth:
  enabled: true
  api_keys:
    - name: admin
      key: "your-secret-key"
      roles: ["admin"]
```

See [Configuration Reference](/docs/reference/configuration) for all options.
