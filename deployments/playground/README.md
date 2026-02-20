# Chronos Playground

Try Chronos locally in 30 seconds:

```bash
cd deployments/playground
docker compose up
```

This starts:
- **Chronos** at http://localhost:8080 (Web UI + API)
- **Echo server** at http://localhost:9090 (webhook target)
- **3 demo jobs** automatically created:
  - `hello-world` — runs every minute
  - `daily-report` — runs at 9 AM daily
  - `cleanup` — runs every 6 hours

## Try It

```bash
# List jobs
curl http://localhost:8080/api/v1/jobs | jq

# Trigger a job manually
curl -X POST http://localhost:8080/api/v1/jobs/<job-id>/trigger

# View executions
curl http://localhost:8080/api/v1/jobs/<job-id>/executions | jq

# Using the CLI
chronosctl --server http://localhost:8080 job list
```

## Clean Up

```bash
docker compose down -v
```
