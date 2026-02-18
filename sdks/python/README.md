# Chronos Python SDK

Python client for the [Chronos](https://github.com/chronos/chronos) distributed cron system.

## Installation

```bash
pip install chronos-sdk
```

## Quick Start

```python
from chronos_sdk import ChronosClient

client = ChronosClient("http://localhost:8080", api_key="your-key")

# Create a job
job = client.create_job(
    name="daily-report",
    schedule="0 9 * * *",
    webhook_url="https://api.example.com/reports",
    method="POST",
    max_retries=3,
)
print(f"Created job: {job.id}")

# Trigger manually
execution = client.trigger(job.id)
print(f"Execution: {execution.status}")

# List all jobs
for job in client.list_jobs():
    print(f"{job.name}: {job.schedule}")
```

## Features

- Full CRUD for jobs (create, get, list, update, delete)
- Trigger, enable, and disable jobs
- Execution history retrieval
- Namespace support for multi-tenancy
- Typed error handling with `ChronosError`
- Zero dependencies (stdlib only)

## Documentation

- [Chronos Docs](https://chronos.github.io/chronos)
- [SDK Guide](https://chronos.github.io/chronos/docs/guides/sdks)
- [API Reference](https://chronos.github.io/chronos/docs/reference/api)

## License

Apache 2.0
