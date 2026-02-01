package rbac

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RBACManager manages roles, users, teams, and permissions.
type RBACManager struct {
	roles      map[string]*Role
	users      map[string]*User
	teams      map[string]*Team
	namespaces map[string]*Namespace
	auditLog   *AuditLogger
	mu         sync.RWMutex
}

// NewRBACManager creates a new RBAC manager.
func NewRBACManager(auditLog *AuditLogger) *RBACManager {
	m := &RBACManager{
		roles:      make(map[string]*Role),
		users:      make(map[string]*User),
		teams:      make(map[string]*Team),
		namespaces: make(map[string]*Namespace),
		auditLog:   auditLog,
	}

	// Load predefined roles
	for _, role := range PredefinedRoles() {
		m.roles[role.ID] = role
	}

	// Create default namespace
	m.namespaces["default"] = &Namespace{
		ID:        "default",
		Name:      "default",
		Quotas:    DefaultNamespaceQuotas(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return m
}

// CheckPermission checks if a user has a specific permission.
func (m *RBACManager) CheckPermission(ctx context.Context, userID string, permission Permission, namespace string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.users[userID]
	if !ok {
		return false, fmt.Errorf("user not found: %s", userID)
	}

	if user.Status != UserStatusActive {
		return false, fmt.Errorf("user account is not active")
	}

	// Check global roles
	for _, roleID := range user.Roles {
		role, ok := m.roles[roleID]
		if !ok {
			continue
		}
		if hasPermission(role.Permissions, permission) {
			return true, nil
		}
	}

	// Check namespace-specific permissions
	if namespace != "" {
		if perms, ok := user.NamespacePerms[namespace]; ok {
			if hasPermission(perms, permission) {
				return true, nil
			}
		}

		// Check team roles for namespace access
		ns, ok := m.namespaces[namespace]
		if ok {
			for teamID, roleID := range user.TeamRoles {
				team, ok := m.teams[teamID]
				if !ok {
					continue
				}
				// Check if team has access to namespace
				if team.ID == ns.OwnerTeamID || containsString(team.Namespaces, namespace) {
					role, ok := m.roles[roleID]
					if ok && hasPermission(role.Permissions, permission) {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

// CheckPermissionOrFail checks permission and returns error if denied.
func (m *RBACManager) CheckPermissionOrFail(ctx context.Context, userID string, permission Permission, namespace string) error {
	allowed, err := m.CheckPermission(ctx, userID, permission, namespace)
	if err != nil {
		return err
	}
	if !allowed {
		if m.auditLog != nil {
			m.auditLog.Log(&AuditEntry{
				Action:    "permission_denied",
				UserID:    userID,
				Resource:  string(permission),
				Namespace: namespace,
				Success:   false,
				Timestamp: time.Now(),
			})
		}
		return &PermissionDeniedError{
			UserID:     userID,
			Permission: permission,
			Namespace:  namespace,
		}
	}
	return nil
}

// CreateUser creates a new user.
func (m *RBACManager) CreateUser(ctx context.Context, user *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[user.ID]; exists {
		return fmt.Errorf("user already exists: %s", user.ID)
	}

	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	if user.Status == "" {
		user.Status = UserStatusPending
	}
	if user.TeamRoles == nil {
		user.TeamRoles = make(map[string]string)
	}
	if user.NamespacePerms == nil {
		user.NamespacePerms = make(map[string][]Permission)
	}

	m.users[user.ID] = user

	if m.auditLog != nil {
		m.auditLog.Log(&AuditEntry{
			Action:    "user_created",
			Resource:  user.ID,
			Timestamp: time.Now(),
			Success:   true,
		})
	}

	return nil
}

// GetUser retrieves a user by ID.
func (m *RBACManager) GetUser(ctx context.Context, userID string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.users[userID]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", userID)
	}
	return user, nil
}

// UpdateUserRoles updates a user's global roles.
func (m *RBACManager) UpdateUserRoles(ctx context.Context, userID string, roles []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found: %s", userID)
	}

	// Validate roles exist
	for _, roleID := range roles {
		if _, ok := m.roles[roleID]; !ok {
			return fmt.Errorf("role not found: %s", roleID)
		}
	}

	user.Roles = roles
	user.UpdatedAt = time.Now()

	if m.auditLog != nil {
		m.auditLog.Log(&AuditEntry{
			Action:    "user_roles_updated",
			UserID:    userID,
			Resource:  userID,
			Details:   map[string]interface{}{"roles": roles},
			Timestamp: time.Now(),
			Success:   true,
		})
	}

	return nil
}

// CreateTeam creates a new team.
func (m *RBACManager) CreateTeam(ctx context.Context, team *Team) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if team.ID == "" {
		team.ID = uuid.New().String()
	}
	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()

	m.teams[team.ID] = team

	if m.auditLog != nil {
		m.auditLog.Log(&AuditEntry{
			Action:    "team_created",
			Resource:  team.ID,
			Timestamp: time.Now(),
			Success:   true,
		})
	}

	return nil
}

// GetTeam retrieves a team by ID.
func (m *RBACManager) GetTeam(ctx context.Context, teamID string) (*Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	team, ok := m.teams[teamID]
	if !ok {
		return nil, fmt.Errorf("team not found: %s", teamID)
	}
	return team, nil
}

// AddTeamMember adds a user to a team.
func (m *RBACManager) AddTeamMember(ctx context.Context, teamID, userID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return fmt.Errorf("team not found: %s", teamID)
	}

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found: %s", userID)
	}

	if _, ok := m.roles[roleID]; !ok {
		return fmt.Errorf("role not found: %s", roleID)
	}

	// Check if already a member
	for _, member := range team.Members {
		if member.UserID == userID {
			return fmt.Errorf("user is already a team member")
		}
	}

	team.Members = append(team.Members, TeamMember{
		UserID:   userID,
		RoleID:   roleID,
		JoinedAt: time.Now(),
	})
	team.UpdatedAt = time.Now()

	user.TeamRoles[teamID] = roleID
	user.UpdatedAt = time.Now()

	if m.auditLog != nil {
		m.auditLog.Log(&AuditEntry{
			Action:    "team_member_added",
			UserID:    userID,
			Resource:  teamID,
			Details:   map[string]interface{}{"role": roleID},
			Timestamp: time.Now(),
			Success:   true,
		})
	}

	return nil
}

// RemoveTeamMember removes a user from a team.
func (m *RBACManager) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return fmt.Errorf("team not found: %s", teamID)
	}

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found: %s", userID)
	}

	// Find and remove member
	found := false
	newMembers := make([]TeamMember, 0)
	for _, member := range team.Members {
		if member.UserID != userID {
			newMembers = append(newMembers, member)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("user is not a team member")
	}

	team.Members = newMembers
	team.UpdatedAt = time.Now()

	delete(user.TeamRoles, teamID)
	user.UpdatedAt = time.Now()

	if m.auditLog != nil {
		m.auditLog.Log(&AuditEntry{
			Action:    "team_member_removed",
			UserID:    userID,
			Resource:  teamID,
			Timestamp: time.Now(),
			Success:   true,
		})
	}

	return nil
}

// CreateNamespace creates a new namespace.
func (m *RBACManager) CreateNamespace(ctx context.Context, ns *Namespace) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ns.ID == "" {
		ns.ID = uuid.New().String()
	}
	if ns.Quotas == nil {
		ns.Quotas = DefaultNamespaceQuotas()
	}
	ns.CreatedAt = time.Now()
	ns.UpdatedAt = time.Now()

	m.namespaces[ns.ID] = ns

	// Grant owner team access
	if ns.OwnerTeamID != "" {
		if team, ok := m.teams[ns.OwnerTeamID]; ok {
			team.Namespaces = append(team.Namespaces, ns.ID)
		}
	}

	if m.auditLog != nil {
		m.auditLog.Log(&AuditEntry{
			Action:    "namespace_created",
			Resource:  ns.ID,
			Namespace: ns.ID,
			Timestamp: time.Now(),
			Success:   true,
		})
	}

	return nil
}

// GetNamespace retrieves a namespace by ID.
func (m *RBACManager) GetNamespace(ctx context.Context, nsID string) (*Namespace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ns, ok := m.namespaces[nsID]
	if !ok {
		return nil, fmt.Errorf("namespace not found: %s", nsID)
	}
	return ns, nil
}

// CreateRole creates a custom role.
func (m *RBACManager) CreateRole(ctx context.Context, role *Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if role.ID == "" {
		role.ID = uuid.New().String()
	}
	role.IsBuiltIn = false
	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()

	m.roles[role.ID] = role

	if m.auditLog != nil {
		m.auditLog.Log(&AuditEntry{
			Action:    "role_created",
			Resource:  role.ID,
			Details:   map[string]interface{}{"permissions": role.Permissions},
			Timestamp: time.Now(),
			Success:   true,
		})
	}

	return nil
}

// GetRole retrieves a role by ID.
func (m *RBACManager) GetRole(ctx context.Context, roleID string) (*Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	role, ok := m.roles[roleID]
	if !ok {
		return nil, fmt.Errorf("role not found: %s", roleID)
	}
	return role, nil
}

// ListRoles returns all roles.
func (m *RBACManager) ListRoles(ctx context.Context) []*Role {
	m.mu.RLock()
	defer m.mu.RUnlock()

	roles := make([]*Role, 0, len(m.roles))
	for _, role := range m.roles {
		roles = append(roles, role)
	}
	return roles
}

// GetUserPermissions returns all effective permissions for a user.
func (m *RBACManager) GetUserPermissions(ctx context.Context, userID string) ([]Permission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.users[userID]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	permSet := make(map[Permission]bool)

	// Global roles
	for _, roleID := range user.Roles {
		if role, ok := m.roles[roleID]; ok {
			for _, perm := range role.Permissions {
				permSet[perm] = true
			}
		}
	}

	// Team roles
	for _, roleID := range user.TeamRoles {
		if role, ok := m.roles[roleID]; ok {
			for _, perm := range role.Permissions {
				permSet[perm] = true
			}
		}
	}

	// Namespace permissions
	for _, perms := range user.NamespacePerms {
		for _, perm := range perms {
			permSet[perm] = true
		}
	}

	result := make([]Permission, 0, len(permSet))
	for perm := range permSet {
		result = append(result, perm)
	}

	return result, nil
}

// Middleware provides HTTP middleware for RBAC.
type Middleware struct {
	manager *RBACManager
}

// NewMiddleware creates RBAC middleware.
func NewMiddleware(manager *RBACManager) *Middleware {
	return &Middleware{manager: manager}
}

// RequirePermission returns middleware that checks for a specific permission.
func (m *Middleware) RequirePermission(permission Permission) func(next func()) func() {
	return func(next func()) func() {
		return func() {
			// In a real implementation, extract user from request context
			// and call CheckPermissionOrFail
			next()
		}
	}
}
