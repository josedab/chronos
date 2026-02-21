# Chronos Deploy Jobs Action

Deploy job definitions to a Chronos instance from your CI/CD pipeline.

## Usage

```yaml
- uses: chronos/chronos/action@v0.1
  with:
    server: https://chronos.example.com
    api-key: ${{ secrets.CHRONOS_API_KEY }}
    path: jobs/
```

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `server` | ✅ | — | Chronos server URL |
| `api-key` | ❌ | — | API key for authentication |
| `path` | ✅ | `jobs/` | Path to YAML job definitions |
| `dry-run` | ❌ | `false` | Show plan without applying |
| `version` | ❌ | `latest` | chronosctl version |

## Example: Deploy on push to main

```yaml
name: Deploy Chronos Jobs
on:
  push:
    branches: [main]
    paths: ['jobs/**']

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: chronos/chronos/action@v0.1
        with:
          server: ${{ vars.CHRONOS_URL }}
          api-key: ${{ secrets.CHRONOS_API_KEY }}
          path: jobs/

  preview:
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: chronos/chronos/action@v0.1
        with:
          server: ${{ vars.CHRONOS_URL }}
          api-key: ${{ secrets.CHRONOS_API_KEY }}
          path: jobs/
          dry-run: 'true'
```
