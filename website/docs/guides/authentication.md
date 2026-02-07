---
sidebar_position: 4
title: Authentication
description: Configure authentication and authorization in Chronos
---

# Authentication & Authorization

Secure your Chronos deployment with authentication, API keys, and role-based access control (RBAC).

## Overview

Chronos supports multiple authentication methods:

| Method | Use Case | Complexity |
|--------|----------|------------|
| API Keys | Machine-to-machine, CI/CD | Low |
| JWT Tokens | User authentication, SSO | Medium |
| Basic Auth | Simple setups, testing | Low |
| OAuth 2.0 / OIDC | Enterprise SSO integration | High |

## Enabling Authentication

By default, authentication is disabled. Enable it in your configuration:

```yaml title="/etc/chronos/chronos.yaml"
security:
  auth:
    enabled: true
```

Once enabled, all API requests require valid credentials.

## API Key Authentication

The simplest method for programmatic access.

### Configuration

```yaml title="/etc/chronos/chronos.yaml"
security:
  auth:
    enabled: true
    api_keys:
      - name: ci-pipeline
        key: "ck_live_abc123..."
        description: "CI/CD pipeline access"
        roles: ["job-manager"]
        
      - name: monitoring
        key: "ck_live_def456..."
        description: "Read-only monitoring access"
        roles: ["viewer"]
        
      - name: admin
        key: "ck_live_xyz789..."
        description: "Full administrative access"
        roles: ["admin"]
```

### Usage

Include the API key in the `X-API-Key` header:

```bash
curl -H "X-API-Key: ck_live_abc123..." \
  http://localhost:8080/api/v1/jobs
```

Or use the `Authorization` header:

```bash
curl -H "Authorization: Bearer ck_live_abc123..." \
  http://localhost:8080/api/v1/jobs
```

### Key Rotation

1. Add a new key with the same roles
2. Update clients to use the new key
3. Remove the old key

```yaml
api_keys:
  - name: ci-pipeline-v2     # New key
    key: "ck_live_new..."
    roles: ["job-manager"]
  - name: ci-pipeline        # Old key (remove after migration)
    key: "ck_live_old..."
    roles: ["job-manager"]
```

### Best Practices

- Use descriptive names for keys
- Assign minimal required roles
- Rotate keys regularly
- Store keys in secret management (Vault, AWS Secrets Manager)
- Use different keys per environment

## JWT Authentication

For user authentication and SSO integration.

### Configuration

```yaml title="/etc/chronos/chronos.yaml"
security:
  auth:
    enabled: true
    jwt:
      enabled: true
      # Option 1: Symmetric key (HMAC)
      secret: "your-256-bit-secret-key"
      algorithm: HS256
      
      # Option 2: Asymmetric keys (RSA/ECDSA)
      # public_key_file: /etc/chronos/jwt-public.pem
      # algorithm: RS256
      
      # JWT validation
      issuer: "https://auth.example.com"
      audience: "chronos"
      claims_mapping:
        username: "sub"
        roles: "chronos_roles"
```

### Token Format

Chronos expects JWTs with these claims:

```json
{
  "sub": "user@example.com",
  "iss": "https://auth.example.com",
  "aud": "chronos",
  "exp": 1706572800,
  "chronos_roles": ["job-manager", "viewer"]
}
```

### Usage

```bash
# Get token from your auth provider
TOKEN=$(curl -X POST https://auth.example.com/oauth/token ...)

# Use token with Chronos
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/jobs
```

### JWKS Support

For dynamic key validation:

```yaml
security:
  auth:
    jwt:
      enabled: true
      jwks_url: "https://auth.example.com/.well-known/jwks.json"
      jwks_refresh_interval: 1h
```

## Basic Authentication

For simple setups and testing.

### Configuration

```yaml title="/etc/chronos/chronos.yaml"
security:
  auth:
    enabled: true
    basic:
      enabled: true
      users:
        - username: admin
          password_hash: "$2a$10$..."  # bcrypt hash
          roles: ["admin"]
        - username: operator
          password_hash: "$2a$10$..."
          roles: ["job-manager"]
```

Generate password hashes:

```bash
# Using htpasswd
htpasswd -nbB admin your-password

# Using chronosctl
chronosctl auth hash-password
```

### Usage

```bash
curl -u admin:your-password \
  http://localhost:8080/api/v1/jobs
```

:::warning
Basic auth transmits credentials with every request. Always use HTTPS in production.
:::

## OAuth 2.0 / OIDC Integration

For enterprise SSO integration.

### Configuration (Generic OIDC)

```yaml title="/etc/chronos/chronos.yaml"
security:
  auth:
    enabled: true
    oidc:
      enabled: true
      issuer: "https://auth.example.com"
      client_id: "chronos-app"
      client_secret: "secret123"
      redirect_url: "https://chronos.example.com/auth/callback"
      scopes: ["openid", "profile", "email"]
      claims_mapping:
        username: "email"
        roles: "groups"
```

### Configuration (Okta)

```yaml
security:
  auth:
    oidc:
      enabled: true
      issuer: "https://your-org.okta.com/oauth2/default"
      client_id: "0oa1234567890"
      client_secret: "your-client-secret"
```

### Configuration (Auth0)

```yaml
security:
  auth:
    oidc:
      enabled: true
      issuer: "https://your-tenant.auth0.com/"
      client_id: "your-client-id"
      client_secret: "your-client-secret"
```

### Configuration (Google)

```yaml
security:
  auth:
    oidc:
      enabled: true
      issuer: "https://accounts.google.com"
      client_id: "xxx.apps.googleusercontent.com"
      client_secret: "your-client-secret"
```

## Role-Based Access Control (RBAC)

Control what authenticated users can do.

### Built-in Roles

| Role | Permissions |
|------|-------------|
| `admin` | Full access to all resources |
| `job-manager` | Create, update, delete jobs |
| `job-operator` | Trigger, enable/disable jobs |
| `viewer` | Read-only access |

### Custom Roles

```yaml title="/etc/chronos/chronos.yaml"
security:
  rbac:
    roles:
      - name: backup-admin
        permissions:
          - resource: jobs
            actions: ["read", "create", "update", "delete"]
            conditions:
              - tag.team: backup
          - resource: executions
            actions: ["read", "cancel"]
            
      - name: developer
        permissions:
          - resource: jobs
            actions: ["read", "trigger"]
          - resource: executions
            actions: ["read"]
```

### Resource Permissions

| Resource | Actions |
|----------|---------|
| `jobs` | `read`, `create`, `update`, `delete`, `trigger`, `enable`, `disable` |
| `executions` | `read`, `cancel` |
| `cluster` | `read`, `manage` |
| `users` | `read`, `create`, `update`, `delete` |
| `roles` | `read`, `create`, `update`, `delete` |

### Namespace-Based Access

Scope access to specific namespaces:

```yaml
security:
  rbac:
    roles:
      - name: team-a-admin
        permissions:
          - resource: jobs
            actions: ["*"]
            namespaces: ["team-a", "team-a-staging"]
```

## CLI Authentication

### Using API Keys

```bash
# Configure API key
chronosctl config set auth.api-key "ck_live_abc123..."

# Or use environment variable
export CHRONOS_API_KEY="ck_live_abc123..."

# Commands now authenticate automatically
chronosctl job list
```

### Using Token

```bash
# Login interactively
chronosctl login

# Or provide token directly
chronosctl config set auth.token "eyJhbG..."
```

### Multiple Profiles

```bash
# Create profiles for different environments
chronosctl config set-context production \
  --server https://chronos.example.com \
  --api-key "ck_live_prod..."

chronosctl config set-context staging \
  --server https://chronos-staging.example.com \
  --api-key "ck_live_staging..."

# Switch between contexts
chronosctl config use-context staging
```

## Web UI Authentication

### Local Login

When using API keys or basic auth, the Web UI shows a login form.

### SSO Integration

With OIDC configured, users are redirected to your identity provider:

1. User visits Web UI
2. Redirected to IdP login page
3. After authentication, redirected back with token
4. Session established in browser

### Session Configuration

```yaml
security:
  session:
    cookie_name: chronos_session
    cookie_secure: true      # Require HTTPS
    cookie_http_only: true   # Prevent XSS
    max_age: 8h              # Session duration
    same_site: strict        # CSRF protection
```

## Audit Logging

Track authentication events:

```yaml
security:
  audit:
    enabled: true
    log_auth_events: true
    log_api_requests: true
```

Audit log entries:

```json
{
  "timestamp": "2026-01-29T12:00:00Z",
  "event": "auth.login.success",
  "user": "admin@example.com",
  "method": "oidc",
  "ip": "10.0.0.1",
  "user_agent": "Mozilla/5.0..."
}
```

## Troubleshooting

### "401 Unauthorized"

1. Check if auth is enabled: `curl http://localhost:8080/health`
2. Verify credentials are correct
3. Check token expiration
4. Review audit logs

### "403 Forbidden"

1. User is authenticated but lacks permission
2. Check role assignments
3. Verify RBAC configuration
4. Check namespace access

### JWT Validation Errors

1. Check token signature (secret/key mismatch)
2. Verify issuer and audience claims
3. Check token expiration
4. Test with JWT debugger (jwt.io)

## Security Best Practices

### Do ✅

- Enable TLS for all traffic
- Use strong, unique API keys
- Implement key rotation
- Apply least-privilege roles
- Monitor failed auth attempts
- Use secret management for credentials

### Don't ❌

- Store secrets in plain text
- Share API keys between environments
- Use overly permissive roles
- Disable auth in production
- Log sensitive credentials

## Next Steps

- [Configuration Reference](/docs/reference/configuration) - All security options
- [Monitoring Guide](/docs/guides/monitoring) - Monitor auth metrics
- [API Reference](/docs/reference/api) - Authenticated API usage
