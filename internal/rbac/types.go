// Package rbac provides role-based access control for Chronos.
package rbac

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Permission represents a specific permission.
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

	// Workflow permissions
	PermWorkflowCreate Permission = "workflow:create"
	PermWorkflowRead   Permission = "workflow:read"
	PermWorkflowUpdate Permission = "workflow:update"
	PermWorkflowDelete Permission = "workflow:delete"

	// Namespace permissions
	PermNamespaceCreate Permission = "namespace:create"
	PermNamespaceRead   Permission = "namespace:read"
	PermNamespaceUpdate Permission = "namespace:update"
	PermNamespaceDelete Permission = "namespace:delete"

	// Team permissions
	PermTeamCreate Permission = "team:create"
	PermTeamRead   Permission = "team:read"
	PermTeamUpdate Permission = "team:update"
	PermTeamDelete Permission = "team:delete"
	PermTeamInvite Permission = "team:invite"

	// Admin permissions
	PermAdminCluster Permission = "admin:cluster"
	PermAdminUsers   Permission = "admin:users"
	PermAdminRoles   Permission = "admin:roles"
	PermAdminAudit   Permission = "admin:audit"
	PermAdminBilling Permission = "admin:billing"

	// Policy permissions
	PermPolicyCreate Permission = "policy:create"
	PermPolicyRead   Permission = "policy:read"
	PermPolicyUpdate Permission = "policy:update"
	PermPolicyDelete Permission = "policy:delete"
)

// Role represents a predefined or custom role.
type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	IsBuiltIn   bool         `json:"is_built_in"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// PredefinedRoles returns built-in roles.
func PredefinedRoles() []*Role {
	return []*Role{
		{
			ID:          "admin",
			Name:        "Administrator",
			Description: "Full access to all resources and settings",
			IsBuiltIn:   true,
			Permissions: []Permission{
				PermJobCreate, PermJobRead, PermJobUpdate, PermJobDelete, PermJobTrigger, PermJobEnable, PermJobDisable,
				PermExecutionRead, PermExecutionCancel, PermExecutionReplay,
				PermWorkflowCreate, PermWorkflowRead, PermWorkflowUpdate, PermWorkflowDelete,
				PermNamespaceCreate, PermNamespaceRead, PermNamespaceUpdate, PermNamespaceDelete,
				PermTeamCreate, PermTeamRead, PermTeamUpdate, PermTeamDelete, PermTeamInvite,
				PermAdminCluster, PermAdminUsers, PermAdminRoles, PermAdminAudit, PermAdminBilling,
				PermPolicyCreate, PermPolicyRead, PermPolicyUpdate, PermPolicyDelete,
			},
		},
		{
			ID:          "operator",
			Name:        "Operator",
			Description: "Manage jobs and workflows, view executions",
			IsBuiltIn:   true,
			Permissions: []Permission{
				PermJobCreate, PermJobRead, PermJobUpdate, PermJobDelete, PermJobTrigger, PermJobEnable, PermJobDisable,
				PermExecutionRead, PermExecutionCancel, PermExecutionReplay,
				PermWorkflowCreate, PermWorkflowRead, PermWorkflowUpdate, PermWorkflowDelete,
				PermNamespaceRead,
				PermTeamRead,
				PermPolicyRead,
			},
		},
		{
			ID:          "developer",
			Name:        "Developer",
			Description: "Create and manage own jobs, view team jobs",
			IsBuiltIn:   true,
			Permissions: []Permission{
				PermJobCreate, PermJobRead, PermJobUpdate, PermJobTrigger,
				PermExecutionRead,
				PermWorkflowCreate, PermWorkflowRead, PermWorkflowUpdate,
				PermNamespaceRead,
				PermTeamRead,
			},
		},
		{
			ID:          "viewer",
			Name:        "Viewer",
			Description: "Read-only access to jobs and executions",
			IsBuiltIn:   true,
			Permissions: []Permission{
				PermJobRead,
				PermExecutionRead,
				PermWorkflowRead,
				PermNamespaceRead,
				PermTeamRead,
			},
		},
	}
}

// User represents a user in the RBAC system.
type User struct {
	ID             string                  `json:"id"`
	Email          string                  `json:"email"`
	Name           string                  `json:"name"`
	Status         UserStatus              `json:"status"`
	Roles          []string                `json:"roles"`           // Global roles
	TeamRoles      map[string]string       `json:"team_roles"`      // team_id -> role_id
	NamespacePerms map[string][]Permission `json:"namespace_perms"` // namespace -> permissions
	Metadata       map[string]string       `json:"metadata,omitempty"`
	LastLoginAt    *time.Time              `json:"last_login_at,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

// UserStatus represents user account status.
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusPending   UserStatus = "pending"
)

// Team represents a group of users.
type Team struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Slug        string            `json:"slug"`
	Members     []TeamMember      `json:"members"`
	Namespaces  []string          `json:"namespaces"` // Accessible namespaces
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TeamMember represents a team membership.
type TeamMember struct {
	UserID   string    `json:"user_id"`
	RoleID   string    `json:"role_id"`
	JoinedAt time.Time `json:"joined_at"`
}

// Namespace represents a resource namespace for isolation.
type Namespace struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	OwnerTeamID string            `json:"owner_team_id"`
	Quotas      *NamespaceQuotas  `json:"quotas"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// NamespaceQuotas defines resource limits for a namespace.
type NamespaceQuotas struct {
	MaxJobs             int `json:"max_jobs"`
	MaxExecutionsPerDay int `json:"max_executions_per_day"`
	MaxConcurrentJobs   int `json:"max_concurrent_jobs"`
	MaxWorkflows        int `json:"max_workflows"`
	MaxRetentionDays    int `json:"max_retention_days"`
}

// DefaultNamespaceQuotas returns default quota values.
func DefaultNamespaceQuotas() *NamespaceQuotas {
	return &NamespaceQuotas{
		MaxJobs:             100,
		MaxExecutionsPerDay: 10000,
		MaxConcurrentJobs:   10,
		MaxWorkflows:        50,
		MaxRetentionDays:    30,
	}
}

// PermissionDeniedError is returned when permission is denied.
type PermissionDeniedError struct {
	UserID     string
	Permission Permission
	Namespace  string
}

func (e *PermissionDeniedError) Error() string {
	if e.Namespace != "" {
		return fmt.Sprintf("permission denied: user %s lacks %s in namespace %s", e.UserID, e.Permission, e.Namespace)
	}
	return fmt.Sprintf("permission denied: user %s lacks %s", e.UserID, e.Permission)
}

// IsPermissionDenied checks if an error is a permission denied error.
func IsPermissionDenied(err error) bool {
	var pde *PermissionDeniedError
	return errors.As(err, &pde)
}

// ContextKey for storing user in context.
type ContextKey string

const (
	UserContextKey      ContextKey = "rbac_user"
	NamespaceContextKey ContextKey = "rbac_namespace"
)

// GetUserFromContext extracts user from context.
func GetUserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(UserContextKey).(*User)
	return user, ok
}

// WithUser adds user to context.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// WithNamespace adds namespace to context.
func WithNamespace(ctx context.Context, namespace string) context.Context {
	return context.WithValue(ctx, NamespaceContextKey, namespace)
}

// GetNamespaceFromContext extracts namespace from context.
func GetNamespaceFromContext(ctx context.Context) (string, bool) {
	ns, ok := ctx.Value(NamespaceContextKey).(string)
	return ns, ok
}

// Helper functions

func hasPermission(perms []Permission, target Permission) bool {
	for _, p := range perms {
		if p == target {
			return true
		}
	}
	return false
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
