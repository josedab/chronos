---
sidebar_position: 2
title: Secrets
description: Secure secret injection for Chronos jobs
---

# Secret Management

Inject secrets from Vault and cloud providers into job executions.

## Supported Providers

- HashiCorp Vault
- AWS Secrets Manager
- GCP Secret Manager
- Azure Key Vault

## Configuration

```yaml
secrets:
  vault:
    address: https://vault.example.com
    path: secret/chronos
```
