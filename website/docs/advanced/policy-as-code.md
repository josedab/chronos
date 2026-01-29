---
sidebar_position: 1
title: Policy-as-Code
description: Declarative governance rules for Chronos jobs
---

# Policy-as-Code

Define governance rules that are evaluated before job operations.

## Example Policy

```yaml
id: production-security
name: Production Security Rules
type: job
severity: error
action: deny
rules:
  - name: https-only
    condition:
      field: webhook.url
      operator: starts_with
      value: "https://"
    message: "Production jobs must use HTTPS"
```

## Operators

`equals`, `not_equals`, `contains`, `starts_with`, `matches` (regex), `greater_than`, `exists`, and more.

See [Architecture](/docs/core-concepts/architecture) for details on the policy engine.
