# ADR-0009: Role-Based Access Control with Teams

## Status

Accepted

## Context

As Chronos adoption grew, access control requirements evolved:

1. **Initial state**: Single admin user, full access to everything
2. **Team growth**: Multiple developers need access with different permissions
3. **Multi-team**: Different teams should manage their own jobs independently
4. **Enterprise**: Compliance requires audit trails and least-privilege access

Requirements:
- **Granular permissions**: Control who can create, read, update, delete, trigger jobs
- **Role abstraction**: Predefined roles for common personas (admin, developer, viewer)
- **Team hierarchy**: Permissions scoped to team membership
- **Namespace isolation**: Jobs in different namespaces have separate access rules
- **Auditability**: Track who did what and when

## Decision

We implemented **Role-Based Access Control (RBAC) with Teams and Namespaces** in `internal/rbac/`:

### Permission Model

50+ granular permissions organized by resource:

```go
// internal/rbac/rbac.go
type Permission string

const (
    // Job permissions
    PermJobCreate  Permission = "job:create"
    PermJobRead    Permission = "job:read"
    PermJobUpdate  Permission = "job:update"
    PermJobDelete  Permission = "job:delete"
    PermJobTrigger Permission = "job:trigger"
    PermJobEnable  Permission = "job:enable"
    PermJobDisable Permission = "job:disable"

    // Execution permissions
    PermExecutionRead   Permission = "execution:read"
    PermExecutionCancel Permission = "execution:cancel"
    PermExecutionReplay Permission = "execution:replay"

    // Admin permissions
    PermAdminCluster Permission = "admin:cluster"
    PermAdminUsers   Permission = "admin:users"
    PermAdminRoles   Permission = "admin:roles"
    PermAdminAudit   Permission = "admin:audit"
    // ...
)
```

### Predefined Roles

```go
func PredefinedRoles() []*Role {
    return []*Role{
        {
            ID:          "admin",
            Name:        "Administrator",
            Description: "Full access to all resources and settings",
            Permissions: []Permission{/* all permissions */},
            IsBuiltIn:   true,
        },
        {
            ID:          "operator",
            Name:        "Operator",
            Description: "Manage jobs and workflows, view executions",
            Permissions: []Permission{
                PermJobCreate, PermJobRead, PermJobUpdate, PermJobDelete,
                PermJobTrigger, PermExecutionRead, PermExecutionCancel,
                // ...
            },
        },
        {
            ID:          "developer",
            Name:        "Developer",
            Description: "Create and manage own jobs, view team jobs",
            Permissions: []Permission{
                PermJobCreate, PermJobRead, PermJobUpdate, PermJobTrigger,
                PermExecutionRead,
            },
        },
        {
            ID:          "viewer",
            Name:        "Viewer",
            Description: "Read-only access to jobs and executions",
            Permissions: []Permission{PermJobRead, PermExecutionRead},
        },
    }
}
```

### Team Membership

```go
type Team struct {
    ID          string       `json:"id"`
    Name        string       `json:"name"`
    Members     []TeamMember `json:"members"`
    Namespaces  []string     `json:"namespaces"`  // Accessible namespaces
}

type TeamMember struct {
    UserID   string    `json:"user_id"`
    RoleID   string    `json:"role_id"`     // Role within this team
    JoinedAt time.Time `json:"joined_at"`
}
```

### Permission Resolution

Permissions are checked hierarchically:

```go
func (m *RBACManager) CheckPermission(ctx context.Context, userID string, permission Permission, namespace string) (bool, error) {
    user, ok := m.users[userID]
    if !ok || user.Status != UserStatusActive {
        return false, fmt.Errorf("user not found or inactive")
    }

    // 1. Check global roles
    for _, roleID := range user.Roles {
        if role := m.roles[roleID]; role != nil {
            if hasPermission(role.Permissions, permission) {
                return true, nil
            }
        }
    }

    // 2. Check namespace-specific permissions
    if perms, ok := user.NamespacePerms[namespace]; ok {
        if hasPermission(perms, permission) {
            return true, nil
        }
    }

    // 3. Check team roles for namespace access
    for teamID, roleID := range user.TeamRoles {
        team := m.teams[teamID]
        if team.ownsNamespace(namespace) {
            if hasPermission(m.roles[roleID].Permissions, permission) {
                return true, nil
            }
        }
    }

    return false, nil
}
```

## Consequences

### Positive

- **Least privilege**: Users have only the permissions they need
- **Self-service**: Team leads manage their own team membership
- **Compliance**: Audit trail of all permission changes
- **Scalability**: Roles reduce per-user configuration
- **Flexibility**: Custom roles for unique requirements

### Negative

- **Complexity**: More moving parts than simple auth
- **Permission explosion**: 50+ permissions can be overwhelming
- **Role sprawl**: Risk of creating too many custom roles
- **Debugging access**: "Why can't I do X?" requires investigation

### API Middleware Integration

```go
// internal/api/router.go
r.Route("/api/v1", func(r chi.Router) {
    r.Use(NewAuthMiddleware(config.AuthConfig))
    r.Use(NewRBACMiddleware(config.RBACConfig))

    r.Route("/jobs", func(r chi.Router) {
        r.With(jobs.Read()).Get("/", handler.ListJobs)
        r.With(jobs.Create()).Post("/", handler.CreateJob)
        r.With(jobs.Update()).Put("/{id}", handler.UpdateJob)
        r.With(jobs.Delete()).Delete("/{id}", handler.DeleteJob)
    })
})
```

### Audit Logging

All RBAC actions are logged:

```go
type AuditEntry struct {
    ID        string                 `json:"id"`
    Action    string                 `json:"action"`
    UserID    string                 `json:"user_id,omitempty"`
    Resource  string                 `json:"resource"`
    Namespace string                 `json:"namespace,omitempty"`
    Details   map[string]interface{} `json:"details,omitempty"`
    IPAddress string                 `json:"ip_address,omitempty"`
    Success   bool                   `json:"success"`
    Timestamp time.Time              `json:"timestamp"`
}
```

### Namespace Quotas

Namespaces have their own resource limits:

```go
type NamespaceQuotas struct {
    MaxJobs              int `json:"max_jobs"`
    MaxExecutionsPerDay  int `json:"max_executions_per_day"`
    MaxConcurrentJobs    int `json:"max_concurrent_jobs"`
    MaxRetentionDays     int `json:"max_retention_days"`
}
```

### Permission Denied Errors

Clear error messages for debugging:

```go
type PermissionDeniedError struct {
    UserID     string
    Permission Permission
    Namespace  string
}

func (e *PermissionDeniedError) Error() string {
    return fmt.Sprintf("permission denied: user %s lacks %s in namespace %s", 
        e.UserID, e.Permission, e.Namespace)
}
```

## References

- [NIST RBAC Model](https://csrc.nist.gov/projects/role-based-access-control)
- [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [AWS IAM](https://docs.aws.amazon.com/IAM/latest/UserGuide/introduction.html)
