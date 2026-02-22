---
sidebar_position: 9
title: SDKs
---

# Chronos SDKs

Chronos provides official SDKs for Go, Python, and TypeScript/JavaScript.

## Go SDK

The Go SDK is available at `github.com/chronos/chronos/pkg/sdk`.

```go
package main

import (
    "context"
    "fmt"
    "github.com/chronos/chronos/pkg/sdk"
)

func main() {
    client := sdk.NewClient("http://localhost:8080",
        sdk.WithAPIKey("your-api-key"),
        sdk.WithNamespace("production"),
    )

    // Create a job
    job, err := client.CreateJob(context.Background(), &sdk.JobDefinition{
        Name:       "daily-report",
        Schedule:   "0 9 * * *",
        WebhookURL: "https://api.example.com/reports",
        Method:     "POST",
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("Created job: %s\n", job.ID)
}
```

### Embedded Scheduler

For applications that want to embed scheduling directly:

```go
sched := sdk.New()
sched.NewJob("cleanup").
    Every("hour").
    Handler(func(ctx context.Context, j *sdk.Job) error {
        // Your logic here
        return nil
    }).
    MustBuild()
sched.Start(context.Background())
```

## Python SDK

Install: `pip install chronos-sdk`

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

# Trigger manually
execution = client.trigger(job.id)

# List all jobs
for job in client.list_jobs():
    print(f"{job.name}: {job.schedule}")
```

## TypeScript/JavaScript SDK

Install: `npm install @chronos/sdk`

```typescript
import { ChronosClient } from '@chronos/sdk';

const client = new ChronosClient('http://localhost:8080', {
  apiKey: 'your-key',
  namespace: 'production',
});

// Create a job
const job = await client.createJob({
  name: 'daily-report',
  schedule: '0 9 * * *',
  webhookUrl: 'https://api.example.com/reports',
  method: 'POST',
  maxRetries: 3,
});

// Trigger and check status
const execution = await client.trigger(job.id);
console.log(`Execution ${execution.id}: ${execution.status}`);
```

## Error Handling

All SDKs throw/raise typed errors:

```python
from chronos_sdk import ChronosClient, ChronosError

try:
    client.get_job("nonexistent")
except ChronosError as e:
    print(f"Error {e.status_code}: {e.code} - {e}")
```

```typescript
try {
  await client.getJob('nonexistent');
} catch (e) {
  if (e instanceof ChronosError) {
    console.error(`${e.statusCode}: ${e.code} - ${e.message}`);
  }
}
```
