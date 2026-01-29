---
sidebar_position: 3
title: Scheduling
description: Cron expressions, intervals, and timezone handling in Chronos
---

# Scheduling

Chronos supports standard cron expressions, special macros, and interval-based scheduling.

## Cron Expression Format

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12 or JAN-DEC)
│ │ │ │ ┌───────────── day of week (0-6 or SUN-SAT)
│ │ │ │ │
│ │ │ │ │
* * * * *
```

## Common Expressions

| Expression | Description | Next Runs |
|------------|-------------|-----------|
| `* * * * *` | Every minute | 12:00, 12:01, 12:02... |
| `0 * * * *` | Every hour | 13:00, 14:00, 15:00... |
| `0 0 * * *` | Every day at midnight | 00:00 daily |
| `0 9 * * *` | Every day at 9 AM | 09:00 daily |
| `0 9 * * 1-5` | Weekdays at 9 AM | Mon-Fri 09:00 |
| `0 0 * * 0` | Every Sunday at midnight | Sunday 00:00 |
| `0 0 1 * *` | First of every month | 1st at 00:00 |
| `0 0 1 1 *` | January 1st at midnight | Jan 1 00:00 |

## Step Values

Use `/` for step values:

| Expression | Description |
|------------|-------------|
| `*/5 * * * *` | Every 5 minutes |
| `*/15 * * * *` | Every 15 minutes |
| `0 */2 * * *` | Every 2 hours |
| `0 0 */3 * *` | Every 3 days |

## Ranges

Use `-` for ranges:

| Expression | Description |
|------------|-------------|
| `0 9-17 * * *` | Every hour from 9 AM to 5 PM |
| `0 0 * * 1-5` | Midnight, Monday through Friday |
| `0 0 1-15 * *` | Midnight, 1st through 15th |

## Lists

Use `,` for multiple values:

| Expression | Description |
|------------|-------------|
| `0 0,12 * * *` | Midnight and noon |
| `0 9 * * 1,3,5` | 9 AM on Mon, Wed, Fri |
| `0 0 1,15 * *` | 1st and 15th of month |

## Special Macros

Chronos supports these convenience macros:

| Macro | Equivalent | Description |
|-------|------------|-------------|
| `@yearly` | `0 0 1 1 *` | Once a year (Jan 1) |
| `@annually` | `0 0 1 1 *` | Same as @yearly |
| `@monthly` | `0 0 1 * *` | First of every month |
| `@weekly` | `0 0 * * 0` | Every Sunday |
| `@daily` | `0 0 * * *` | Every midnight |
| `@midnight` | `0 0 * * *` | Same as @daily |
| `@hourly` | `0 * * * *` | Every hour |

## Interval Scheduling

For intervals not easily expressed as cron:

| Expression | Description |
|------------|-------------|
| `@every 30s` | Every 30 seconds |
| `@every 5m` | Every 5 minutes |
| `@every 1h30m` | Every 90 minutes |
| `@every 6h` | Every 6 hours |

```json
{
  "name": "frequent-check",
  "schedule": "@every 30s"
}
```

:::note Interval vs Cron
- **Cron**: Fixed times (e.g., "at 9:00 AM")
- **Interval**: Fixed duration from last run (e.g., "30 seconds after last run")

Intervals are calculated from when the job finishes, not when it starts.
:::

## Timezones

By default, schedules use UTC. Specify a timezone with IANA names:

```json
{
  "schedule": "0 9 * * *",
  "timezone": "America/New_York"
}
```

### Common Timezones

| IANA Name | Offset | Region |
|-----------|--------|--------|
| `America/New_York` | UTC-5/-4 | US Eastern |
| `America/Los_Angeles` | UTC-8/-7 | US Pacific |
| `America/Chicago` | UTC-6/-5 | US Central |
| `Europe/London` | UTC+0/+1 | UK |
| `Europe/Paris` | UTC+1/+2 | Central Europe |
| `Asia/Tokyo` | UTC+9 | Japan |
| `Asia/Shanghai` | UTC+8 | China |
| `Australia/Sydney` | UTC+10/+11 | Australia East |

### Daylight Saving Time

Chronos handles DST transitions automatically:

- **Spring Forward**: If a scheduled time doesn't exist (e.g., 2:30 AM skipped), the job runs at the next valid time
- **Fall Back**: If a time occurs twice, the job runs once (first occurrence)

```json
{
  "schedule": "0 2 * * *",
  "timezone": "America/New_York"
}
// During DST transition:
// - Spring: 2:00 AM skipped, runs at 3:00 AM
// - Fall: Runs once at first 2:00 AM
```

## Validating Schedules

### Via API

```bash
curl -X POST http://localhost:8080/api/v1/schedules/validate \
  -H "Content-Type: application/json" \
  -d '{
    "schedule": "0 9 * * 1-5",
    "timezone": "America/New_York"
  }'
```

Response:

```json
{
  "valid": true,
  "next_runs": [
    "2026-01-30T14:00:00Z",
    "2026-01-31T14:00:00Z",
    "2026-02-03T14:00:00Z"
  ],
  "description": "At 09:00 AM, Monday through Friday"
}
```

### Via CLI

```bash
chronosctl schedule validate "0 9 * * 1-5" --timezone "America/New_York"
```

## Missed Runs

When Chronos starts after being down, or when a leader election occurs, some scheduled runs may have been missed.

### Missed Run Policies

Configure how to handle missed runs:

```yaml
scheduler:
  missed_run_policy: execute_one
```

| Policy | Behavior |
|--------|----------|
| `ignore` | Skip all missed runs |
| `execute_one` | Execute once for all missed (default) |
| `execute_all` | Execute once per missed run |

### Example

Job scheduled for `0 * * * *` (every hour), Chronos down from 10:00-13:00:

| Policy | Executions |
|--------|------------|
| `ignore` | None |
| `execute_one` | One execution at 13:00 |
| `execute_all` | Three executions (10:00, 11:00, 12:00 catchup) |

## Schedule Examples

### Business Hours

```json
{
  "schedule": "0 9-17 * * 1-5",
  "timezone": "America/New_York"
}
```
*Every hour from 9 AM to 5 PM, Monday through Friday, Eastern Time*

### End of Month

```json
{
  "schedule": "0 0 L * *",
  "timezone": "UTC"
}
```
*Midnight on the last day of every month (L = last)*

:::note
Some cron implementations support `L` for "last". Check if your version supports it.
:::

### Multiple Times Per Day

```json
{
  "schedule": "0 9,13,17 * * *"
}
```
*At 9 AM, 1 PM, and 5 PM daily*

### Quarterly Reports

```json
{
  "schedule": "0 9 1 1,4,7,10 *",
  "timezone": "America/New_York"
}
```
*9 AM on the 1st of January, April, July, October*

## Best Practices

:::tip Spread the Load
Avoid scheduling many jobs at the same time (e.g., `0 0 * * *`). Stagger schedules:

```
0 0 * * *   → Job A
5 0 * * *   → Job B
10 0 * * *  → Job C
```
:::

:::tip Use Specific Times
Prefer specific times over frequent polling:

```
✅ "0 9 * * *"      - Run at 9 AM
❌ "*/1 * * * *"    - Check every minute
```
:::

:::tip Document Complex Schedules
Add descriptions for non-obvious schedules:

```json
{
  "description": "Q1 reporting - 9 AM ET, first Monday of Jan/Apr/Jul/Oct",
  "schedule": "0 9 1-7 1,4,7,10 1"
}
```
:::

## Next Steps

- [Webhook Configuration](/docs/core-concepts/webhooks)
- [Job Configuration](/docs/core-concepts/jobs)
- [API Reference](/docs/reference/api)
