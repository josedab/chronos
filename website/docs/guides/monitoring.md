---
sidebar_position: 3
title: Monitoring
description: Monitor Chronos with Prometheus and Grafana
---

# Monitoring

Set up monitoring for your Chronos deployment.

## Prometheus

Chronos exposes metrics at `/metrics`.

```yaml
scrape_configs:
  - job_name: chronos
    static_configs:
      - targets: ['chronos:8080']
```

See [Metrics Reference](/docs/reference/metrics) for all available metrics.
