# ADR-0008: Multi-Tenancy with Tiered Quotas

## Status

Accepted

## Context

Chronos evolved from a single-team tool to a platform serving multiple teams and organizations. This creates requirements for:

- **Isolation**: Teams shouldn't see or affect each other's jobs
- **Resource limits**: Prevent any single tenant from monopolizing capacity
- **Billing**: Track usage for cost allocation or monetization
- **Self-service**: Teams onboard without manual provisioning

We considered two approaches:

1. **Namespace-only isolation**: Logical separation within a single deployment
   - Simpler operations
   - Shared resource pool
   - Softer isolation boundaries

2. **Full multi-tenancy with quotas**: Complete tenant model with usage tracking
   - SaaS-ready architecture
   - Per-tenant billing capability
   - Strict resource governance

## Decision

We implemented **full multi-tenancy with tiered quotas** in `internal/tenant/`:

```go
// internal/tenant/manager.go
type Tenant struct {
    ID          string       `json:"id"`
    Name        string       `json:"name"`
    Slug        string       `json:"slug"`
    Tier        Tier         `json:"tier"`
    Status      TenantStatus `json:"status"`
    Quota       Quota        `json:"quota"`
    Usage       Usage        `json:"usage"`
    BillingID   string       `json:"billing_id,omitempty"`
}

type Tier string

const (
    TierFree       Tier = "free"
    TierStarter    Tier = "starter"
    TierPro        Tier = "pro"
    TierEnterprise Tier = "enterprise"
)
```

### Quota Model

Each tier defines resource limits:

```go
type Quota struct {
    MaxJobs              int      `json:"max_jobs"`
    MaxExecutionsPerDay  int      `json:"max_executions_per_day"`
    MaxDAGs              int      `json:"max_dags"`
    MaxConcurrentRuns    int      `json:"max_concurrent_runs"`
    MaxRetentionDays     int      `json:"max_retention_days"`
    MaxWebhookTimeout    int      `json:"max_webhook_timeout_seconds"`
    Features             []string `json:"features,omitempty"`
}
```

### Tier Defaults

| Resource | Free | Starter | Pro | Enterprise |
|----------|------|---------|-----|------------|
| Max Jobs | 5 | 25 | 100 | Unlimited |
| Executions/Day | 100 | 1,000 | 10,000 | Unlimited |
| Max DAGs | 1 | 5 | 50 | Unlimited |
| Concurrent Runs | 2 | 5 | 20 | 100 |
| Retention Days | 7 | 30 | 90 | 365 |
| Webhook Timeout | 30s | 60s | 300s | 3600s |

## Consequences

### Positive

- **SaaS-ready**: Can offer Chronos as a hosted service with subscription tiers
- **Fair resource sharing**: No tenant can starve others
- **Cost visibility**: Usage tracking enables chargeback/showback
- **Self-service scaling**: Tenants upgrade tiers for more capacity
- **Feature gating**: Enterprise features only for paying customers

### Negative

- **Operational complexity**: Must manage tenant lifecycle
- **Quota enforcement overhead**: Every operation checks quotas
- **Upgrade friction**: Tenants hitting limits need tier upgrades
- **Multi-tenant debugging**: Issues may be tenant-specific

### API Key Authentication

Tenants authenticate via API keys:

```go
type APIKey struct {
    ID          string    `json:"id"`
    TenantID    string    `json:"tenant_id"`
    Name        string    `json:"name"`
    KeyHash     string    `json:"-"`           // bcrypt hash
    KeyPrefix   string    `json:"key_prefix"`  // For identification
    Scopes      []string  `json:"scopes"`
    ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}
```

Keys are formatted as `chron_<uuid>` with the first 12 characters used for fast lookup before full verification.

### Quota Enforcement

```go
func (m *Manager) CheckQuota(ctx context.Context, tenantID string, resource string, delta int) error {
    t, ok := m.tenants[tenantID]
    if !ok {
        return ErrTenantNotFound
    }

    if t.Status != TenantStatusActive {
        return ErrTenantDisabled
    }

    switch resource {
    case "jobs":
        if t.Usage.Jobs+delta > t.Quota.MaxJobs {
            return ErrQuotaExceeded
        }
    case "executions":
        if t.Usage.ExecutionsToday+delta > t.Quota.MaxExecutionsPerDay {
            return ErrQuotaExceeded
        }
    // ...
    }
    return nil
}
```

### Usage Tracking

Every billable action records a usage event:

```go
type UsageEvent struct {
    ID          string    `json:"id"`
    TenantID    string    `json:"tenant_id"`
    Type        string    `json:"type"`
    Quantity    int       `json:"quantity"`
    Timestamp   time.Time `json:"timestamp"`
}
```

Usage events feed into billing integrations (Stripe, internal systems).

### Feature Flags by Tier

```go
func (m *Manager) defaultQuotaForTier(tier Tier) Quota {
    switch tier {
    case TierFree:
        return Quota{
            Features: []string{"basic"},
        }
    case TierPro:
        return Quota{
            Features: []string{"basic", "webhooks", "notifications", "dag", "triggers", "analytics"},
        }
    case TierEnterprise:
        return Quota{
            Features: []string{"basic", "webhooks", "notifications", "dag", "triggers", "analytics", "sso", "audit", "sla"},
        }
    }
}
```

### Daily Usage Reset

Execution counts reset at midnight:

```go
func (m *Manager) ResetDailyUsage(ctx context.Context) error {
    for _, t := range m.tenants {
        t.Usage.ExecutionsToday = 0
        t.Usage.LastUpdated = time.Now()
    }
    return nil
}
```

## References

- [SaaS Multi-Tenancy Patterns](https://docs.aws.amazon.com/whitepapers/latest/saas-tenant-isolation-strategies/saas-tenant-isolation-strategies.html)
- [Stripe Billing](https://stripe.com/billing)
- [Rate Limiting Strategies](https://cloud.google.com/architecture/rate-limiting-strategies-techniques)
