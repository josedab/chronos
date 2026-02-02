# Chronos Cloud Platform

The Chronos Cloud Platform provides a fully managed SaaS offering for running distributed cron jobs without infrastructure management.

## Overview

The cloud platform adds multi-tenant capabilities on top of the open-source Chronos scheduler:

- **Organizations**: Logical groupings for teams or companies
- **Workspaces**: Isolated environments within an organization (dev, staging, prod)
- **Members**: User management with role-based access control
- **Billing**: Subscription management and usage-based pricing

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Control Plane                         │
├─────────────────────────────────────────────────────────┤
│  Organizations  │  Workspaces  │  Members  │  Billing   │
├─────────────────────────────────────────────────────────┤
│                    API Gateway                           │
├─────────────────────────────────────────────────────────┤
│    Workspace 1     │    Workspace 2    │   Workspace N  │
│  ┌─────────────┐   │  ┌─────────────┐  │  ┌──────────┐  │
│  │  Scheduler  │   │  │  Scheduler  │  │  │Scheduler │  │
│  │   Cluster   │   │  │   Cluster   │  │  │ Cluster  │  │
│  └─────────────┘   │  └─────────────┘  │  └──────────┘  │
└─────────────────────────────────────────────────────────┘
```

## Pricing Plans

| Feature | Free | Starter ($29/mo) | Pro ($99/mo) | Enterprise ($499/mo) |
|---------|------|------------------|--------------|----------------------|
| Workspaces | 1 | 3 | 10 | Unlimited |
| Team Members | 3 | 10 | 50 | Unlimited |
| Jobs | 10 | 100 | 1,000 | Unlimited |
| Executions/month | 1,000 | 50,000 | 500,000 | Unlimited |
| DAG Workflows | ❌ | ✅ | ✅ | ✅ |
| Analytics | Basic | Standard | Advanced | Advanced |
| SSO/SAML | ❌ | ❌ | ❌ | ✅ |
| Audit Logs | ❌ | ❌ | ✅ | ✅ |
| SLA | - | 99.5% | 99.9% | 99.99% |
| Support | Community | Email | Priority | Dedicated |

## API Reference

### Organizations

#### Create Organization

```http
POST /api/v1/cloud/organizations
Content-Type: application/json

{
  "name": "Acme Corp",
  "owner_email": "admin@acme.com"
}
```

Response:
```json
{
  "id": "org_abc123",
  "name": "Acme Corp",
  "owner_email": "admin@acme.com",
  "plan": "free",
  "created_at": "2024-01-15T10:00:00Z"
}
```

#### Get Organization

```http
GET /api/v1/cloud/organizations/{orgId}
```

#### List Organizations

```http
GET /api/v1/cloud/organizations
```

#### Update Plan

```http
PUT /api/v1/cloud/organizations/{orgId}/plan
Content-Type: application/json

{
  "plan": "pro"
}
```

### Workspaces

#### Create Workspace

```http
POST /api/v1/cloud/organizations/{orgId}/workspaces
Content-Type: application/json

{
  "name": "Production",
  "environment": "prod"
}
```

#### List Workspaces

```http
GET /api/v1/cloud/organizations/{orgId}/workspaces
```

#### Delete Workspace

```http
DELETE /api/v1/cloud/organizations/{orgId}/workspaces/{wsId}
```

### Members

#### Invite Member

```http
POST /api/v1/cloud/organizations/{orgId}/members
Content-Type: application/json

{
  "email": "user@acme.com",
  "role": "member"
}
```

Roles: `owner`, `admin`, `member`, `viewer`

#### List Members

```http
GET /api/v1/cloud/organizations/{orgId}/members
```

#### Update Member Role

```http
PUT /api/v1/cloud/organizations/{orgId}/members/{memberId}
Content-Type: application/json

{
  "role": "admin"
}
```

#### Remove Member

```http
DELETE /api/v1/cloud/organizations/{orgId}/members/{memberId}
```

### Billing

#### Get Subscription

```http
GET /api/v1/cloud/organizations/{orgId}/subscription
```

#### Get Invoices

```http
GET /api/v1/cloud/organizations/{orgId}/invoices
```

#### Get Usage

```http
GET /api/v1/cloud/organizations/{orgId}/usage?start=2024-01-01&end=2024-01-31
```

## Quota Management

Each plan has resource quotas that are enforced at the API level:

```go
// Check if organization has quota for new workspace
if err := controlPlane.CheckQuota(ctx, orgID, QuotaWorkspaces); err != nil {
    // Returns ErrQuotaExceeded
}
```

Quotas are checked for:
- Workspaces
- Members  
- Jobs per workspace
- Executions per month
- API requests per minute

## Feature Flags

Features can be checked per organization:

```go
if controlPlane.CheckFeature(ctx, orgID, FeatureDAG) {
    // DAG workflows enabled
}
```

Available features:
- `dag_workflows` - Visual DAG workflow builder
- `analytics` - Advanced analytics dashboard
- `sso` - SSO/SAML authentication
- `audit_logs` - Comprehensive audit logging
- `custom_retention` - Custom data retention policies
- `priority_support` - Priority support queue

## Integration

### Route Registration

```go
import (
    "github.com/chronos/chronos/internal/api"
    "github.com/chronos/chronos/internal/cloud"
)

// Create control plane and billing
cp := cloud.NewControlPlane()
billing := cloud.NewBillingService(cp)
handlers := cloud.NewCloudHandlers(cp, billing)

// Create main router
router := api.NewRouter(handler, logger)

// Mount cloud routes
api.MountCloudRoutes(router, handlers.Router())
```

## Security Considerations

1. **Tenant Isolation**: Each workspace runs in complete isolation
2. **API Authentication**: All endpoints require valid API keys
3. **Rate Limiting**: Per-organization rate limits prevent abuse
4. **Encryption**: All data encrypted at rest and in transit
5. **Audit Logging**: All administrative actions are logged
