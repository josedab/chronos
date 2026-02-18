# Chronos JavaScript/TypeScript SDK

TypeScript client for the [Chronos](https://github.com/chronos/chronos) distributed cron system.

## Installation

```bash
npm install @chronos/sdk
```

## Quick Start

```typescript
import { ChronosClient } from '@chronos/sdk';

const client = new ChronosClient('http://localhost:8080', {
  apiKey: 'your-key',
});

// Create a job
const job = await client.createJob({
  name: 'daily-report',
  schedule: '0 9 * * *',
  webhookUrl: 'https://api.example.com/reports',
  method: 'POST',
  maxRetries: 3,
});

// Trigger
const execution = await client.trigger(job.id);
console.log(`Status: ${execution.status}`);

// List jobs
const jobs = await client.listJobs();
jobs.forEach(j => console.log(`${j.name}: ${j.schedule}`));
```

## Features

- Full TypeScript types for all API responses
- Job CRUD, trigger, enable, disable
- Execution history
- Namespace and API key support
- Typed error handling with `ChronosError`
- Zero runtime dependencies (uses native `fetch`)

## License

Apache 2.0
