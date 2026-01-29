package rbac

import (
	"context"
	"testing"
	"time"
)

func TestPredefinedRoles(t *testing.T) {
	roles := PredefinedRoles()

	if len(roles) != 4 {
		t.Errorf("expected 4 predefined roles, got %d", len(roles))
	}

	roleMap := make(map[string]*Role)
	for _, r := range roles {
		roleMap[r.ID] = r
	}

	// Check admin role
	admin := roleMap["admin"]
	if admin == nil {
		t.Fatal("admin role not found")
	}
	if !admin.IsBuiltIn {
		t.Error("admin role should be built-in")
	}
	if !hasPermission(admin.Permissions, PermAdminCluster) {
		t.Error("admin should have cluster admin permission")
	}

	// Check viewer role has limited permissions
	viewer := roleMap["viewer"]
	if viewer == nil {
		t.Fatal("viewer role not found")
	}
	if hasPermission(viewer.Permissions, PermJobDelete) {
		t.Error("viewer should not have job:delete permission")
	}
}

func TestDefaultNamespaceQuotas(t *testing.T) {
	q := DefaultNamespaceQuotas()

	if q.MaxJobs != 100 {
		t.Errorf("expected MaxJobs=100, got %d", q.MaxJobs)
	}
	if q.MaxConcurrentJobs != 10 {
		t.Errorf("expected MaxConcurrentJobs=10, got %d", q.MaxConcurrentJobs)
	}
}

func TestNewRBACManager(t *testing.T) {
	audit := NewAuditLogger()
	m := NewRBACManager(audit)

	// Should have predefined roles loaded
	roles := m.ListRoles(context.Background())
	if len(roles) < 4 {
		t.Errorf("expected at least 4 roles, got %d", len(roles))
	}

	// Should have default namespace
	ns, err := m.GetNamespace(context.Background(), "default")
	if err != nil {
		t.Errorf("failed to get default namespace: %v", err)
	}
	if ns == nil {
		t.Error("default namespace should exist")
	}
}

func TestRBACManager_CreateUser(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	user := &User{
		Email: "test@example.com",
		Name:  "Test User",
	}

	err := m.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if user.ID == "" {
		t.Error("user ID should be generated")
	}
	if user.Status != UserStatusPending {
		t.Errorf("expected pending status, got %s", user.Status)
	}
	if user.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Should fail for duplicate
	err = m.CreateUser(ctx, user)
	if err == nil {
		t.Error("expected error for duplicate user")
	}
}

func TestRBACManager_GetUser(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	user := &User{ID: "user-123", Email: "test@example.com"}
	m.CreateUser(ctx, user)

	found, err := m.GetUser(ctx, "user-123")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if found.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", found.Email)
	}

	_, err = m.GetUser(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}

func TestRBACManager_UpdateUserRoles(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	user := &User{ID: "user-123", Email: "test@example.com"}
	m.CreateUser(ctx, user)

	err := m.UpdateUserRoles(ctx, "user-123", []string{"admin"})
	if err != nil {
		t.Fatalf("failed to update roles: %v", err)
	}

	found, _ := m.GetUser(ctx, "user-123")
	if len(found.Roles) != 1 || found.Roles[0] != "admin" {
		t.Errorf("expected roles [admin], got %v", found.Roles)
	}

	// Should fail for invalid role
	err = m.UpdateUserRoles(ctx, "user-123", []string{"nonexistent"})
	if err == nil {
		t.Error("expected error for invalid role")
	}
}

func TestRBACManager_CheckPermission(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	// Create user with admin role
	user := &User{
		ID:     "admin-user",
		Email:  "admin@example.com",
		Status: UserStatusActive,
		Roles:  []string{"admin"},
	}
	m.CreateUser(ctx, user)

	// Admin should have permission
	allowed, err := m.CheckPermission(ctx, "admin-user", PermJobCreate, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("admin should have job:create permission")
	}

	// Create viewer user
	viewer := &User{
		ID:     "viewer-user",
		Email:  "viewer@example.com",
		Status: UserStatusActive,
		Roles:  []string{"viewer"},
	}
	m.CreateUser(ctx, viewer)

	// Viewer should not have create permission
	allowed, err = m.CheckPermission(ctx, "viewer-user", PermJobCreate, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("viewer should not have job:create permission")
	}

	// Viewer should have read permission
	allowed, err = m.CheckPermission(ctx, "viewer-user", PermJobRead, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("viewer should have job:read permission")
	}
}

func TestRBACManager_CheckPermission_InactiveUser(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	user := &User{
		ID:     "inactive-user",
		Email:  "inactive@example.com",
		Status: UserStatusInactive,
		Roles:  []string{"admin"},
	}
	m.CreateUser(ctx, user)

	_, err := m.CheckPermission(ctx, "inactive-user", PermJobCreate, "")
	if err == nil {
		t.Error("expected error for inactive user")
	}
}

func TestRBACManager_CheckPermissionOrFail(t *testing.T) {
	audit := NewAuditLogger()
	m := NewRBACManager(audit)
	ctx := context.Background()

	user := &User{
		ID:     "test-user",
		Email:  "test@example.com",
		Status: UserStatusActive,
		Roles:  []string{"viewer"},
	}
	m.CreateUser(ctx, user)

	// Should fail for permission not held
	err := m.CheckPermissionOrFail(ctx, "test-user", PermJobDelete, "")
	if err == nil {
		t.Error("expected permission denied error")
	}
	if !IsPermissionDenied(err) {
		t.Error("error should be PermissionDeniedError")
	}

	// Should have audit log entry
	entries := audit.Query(AuditFilter{Action: "permission_denied"})
	if len(entries) == 0 {
		t.Error("expected audit log entry for denied permission")
	}
}

func TestRBACManager_CreateTeam(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	team := &Team{
		Name:        "Engineering",
		Description: "Engineering team",
		Slug:        "engineering",
	}

	err := m.CreateTeam(ctx, team)
	if err != nil {
		t.Fatalf("failed to create team: %v", err)
	}

	if team.ID == "" {
		t.Error("team ID should be generated")
	}

	found, err := m.GetTeam(ctx, team.ID)
	if err != nil {
		t.Fatalf("failed to get team: %v", err)
	}
	if found.Name != "Engineering" {
		t.Errorf("expected name Engineering, got %s", found.Name)
	}
}

func TestRBACManager_AddTeamMember(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	team := &Team{ID: "team-1", Name: "Test Team"}
	m.CreateTeam(ctx, team)

	user := &User{ID: "user-1", Email: "user@example.com", Status: UserStatusActive}
	m.CreateUser(ctx, user)

	err := m.AddTeamMember(ctx, "team-1", "user-1", "developer")
	if err != nil {
		t.Fatalf("failed to add team member: %v", err)
	}

	foundTeam, _ := m.GetTeam(ctx, "team-1")
	if len(foundTeam.Members) != 1 {
		t.Errorf("expected 1 member, got %d", len(foundTeam.Members))
	}
	if foundTeam.Members[0].RoleID != "developer" {
		t.Errorf("expected role developer, got %s", foundTeam.Members[0].RoleID)
	}

	foundUser, _ := m.GetUser(ctx, "user-1")
	if foundUser.TeamRoles["team-1"] != "developer" {
		t.Error("user should have team role")
	}

	// Should fail for duplicate
	err = m.AddTeamMember(ctx, "team-1", "user-1", "developer")
	if err == nil {
		t.Error("expected error for duplicate member")
	}
}

func TestRBACManager_RemoveTeamMember(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	team := &Team{ID: "team-1", Name: "Test Team"}
	m.CreateTeam(ctx, team)

	user := &User{ID: "user-1", Email: "user@example.com", Status: UserStatusActive}
	m.CreateUser(ctx, user)

	m.AddTeamMember(ctx, "team-1", "user-1", "developer")

	err := m.RemoveTeamMember(ctx, "team-1", "user-1")
	if err != nil {
		t.Fatalf("failed to remove team member: %v", err)
	}

	foundTeam, _ := m.GetTeam(ctx, "team-1")
	if len(foundTeam.Members) != 0 {
		t.Errorf("expected 0 members, got %d", len(foundTeam.Members))
	}

	foundUser, _ := m.GetUser(ctx, "user-1")
	if _, exists := foundUser.TeamRoles["team-1"]; exists {
		t.Error("user should not have team role after removal")
	}
}

func TestRBACManager_CreateNamespace(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	ns := &Namespace{
		Name:        "production",
		Description: "Production namespace",
	}

	err := m.CreateNamespace(ctx, ns)
	if err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}

	if ns.ID == "" {
		t.Error("namespace ID should be generated")
	}
	if ns.Quotas == nil {
		t.Error("namespace should have default quotas")
	}

	found, err := m.GetNamespace(ctx, ns.ID)
	if err != nil {
		t.Fatalf("failed to get namespace: %v", err)
	}
	if found.Name != "production" {
		t.Errorf("expected name production, got %s", found.Name)
	}
}

func TestRBACManager_CreateRole(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	role := &Role{
		Name:        "custom-role",
		Description: "A custom role",
		Permissions: []Permission{PermJobRead, PermJobTrigger},
	}

	err := m.CreateRole(ctx, role)
	if err != nil {
		t.Fatalf("failed to create role: %v", err)
	}

	if role.ID == "" {
		t.Error("role ID should be generated")
	}
	if role.IsBuiltIn {
		t.Error("custom role should not be built-in")
	}

	found, err := m.GetRole(ctx, role.ID)
	if err != nil {
		t.Fatalf("failed to get role: %v", err)
	}
	if len(found.Permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(found.Permissions))
	}
}

func TestRBACManager_GetUserPermissions(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	user := &User{
		ID:     "user-1",
		Email:  "user@example.com",
		Status: UserStatusActive,
		Roles:  []string{"viewer"},
	}
	m.CreateUser(ctx, user)

	perms, err := m.GetUserPermissions(ctx, "user-1")
	if err != nil {
		t.Fatalf("failed to get permissions: %v", err)
	}

	// Viewer has: job:read, execution:read, workflow:read, namespace:read, team:read
	if len(perms) < 5 {
		t.Errorf("expected at least 5 permissions, got %d", len(perms))
	}

	// Check specific permission
	hasJobRead := false
	for _, p := range perms {
		if p == PermJobRead {
			hasJobRead = true
			break
		}
	}
	if !hasJobRead {
		t.Error("viewer should have job:read permission")
	}
}

func TestRBACManager_NamespacePermissions(t *testing.T) {
	m := NewRBACManager(nil)
	ctx := context.Background()

	// Create user with namespace-specific permissions
	user := &User{
		ID:     "user-1",
		Email:  "user@example.com",
		Status: UserStatusActive,
		NamespacePerms: map[string][]Permission{
			"prod-ns": {PermJobRead, PermJobTrigger},
		},
	}
	m.CreateUser(ctx, user)

	// Should have permission in specific namespace
	allowed, _ := m.CheckPermission(ctx, "user-1", PermJobTrigger, "prod-ns")
	if !allowed {
		t.Error("user should have job:trigger in prod-ns")
	}

	// Should not have permission in other namespace
	allowed, _ = m.CheckPermission(ctx, "user-1", PermJobTrigger, "other-ns")
	if allowed {
		t.Error("user should not have job:trigger in other-ns")
	}
}

func TestPermissionDeniedError(t *testing.T) {
	err := &PermissionDeniedError{
		UserID:     "user-123",
		Permission: PermJobDelete,
	}

	expected := "permission denied: user user-123 lacks job:delete"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}

	errWithNs := &PermissionDeniedError{
		UserID:     "user-123",
		Permission: PermJobDelete,
		Namespace:  "prod",
	}

	expected = "permission denied: user user-123 lacks job:delete in namespace prod"
	if errWithNs.Error() != expected {
		t.Errorf("expected %q, got %q", expected, errWithNs.Error())
	}
}

func TestAuditLogger(t *testing.T) {
	logger := NewAuditLogger()

	logger.Log(&AuditEntry{
		Action:    "user_login",
		UserID:    "user-1",
		Success:   true,
		Timestamp: time.Now().Add(-1 * time.Hour),
	})
	logger.Log(&AuditEntry{
		Action:    "user_login",
		UserID:    "user-2",
		Success:   true,
		Timestamp: time.Now(),
	})
	logger.Log(&AuditEntry{
		Action:    "job_created",
		UserID:    "user-1",
		Resource:  "job-123",
		Success:   true,
		Timestamp: time.Now(),
	})

	// Query all
	entries := logger.Query(AuditFilter{})
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Filter by user
	entries = logger.Query(AuditFilter{UserID: "user-1"})
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for user-1, got %d", len(entries))
	}

	// Filter by action
	entries = logger.Query(AuditFilter{Action: "job_created"})
	if len(entries) != 1 {
		t.Errorf("expected 1 job_created entry, got %d", len(entries))
	}

	// Filter by time range
	entries = logger.Query(AuditFilter{
		StartTime: time.Now().Add(-30 * time.Minute),
	})
	if len(entries) != 2 {
		t.Errorf("expected 2 entries in last 30 min, got %d", len(entries))
	}
}

func TestContextFunctions(t *testing.T) {
	ctx := context.Background()

	user := &User{ID: "user-1", Email: "test@example.com"}
	ctx = WithUser(ctx, user)
	ctx = WithNamespace(ctx, "test-ns")

	foundUser, ok := GetUserFromContext(ctx)
	if !ok {
		t.Error("user should be in context")
	}
	if foundUser.ID != "user-1" {
		t.Errorf("expected user-1, got %s", foundUser.ID)
	}

	ns, ok := GetNamespaceFromContext(ctx)
	if !ok {
		t.Error("namespace should be in context")
	}
	if ns != "test-ns" {
		t.Errorf("expected test-ns, got %s", ns)
	}
}

func TestUserStatus_Constants(t *testing.T) {
	statuses := []UserStatus{UserStatusActive, UserStatusInactive, UserStatusSuspended, UserStatusPending}
	expected := []string{"active", "inactive", "suspended", "pending"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], s)
		}
	}
}

func TestHasPermission(t *testing.T) {
	perms := []Permission{PermJobRead, PermJobCreate, PermJobUpdate}

	if !hasPermission(perms, PermJobRead) {
		t.Error("should have PermJobRead")
	}
	if hasPermission(perms, PermJobDelete) {
		t.Error("should not have PermJobDelete")
	}
	if hasPermission(nil, PermJobRead) {
		t.Error("nil permissions should not have any permission")
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"a", "b", "c"}

	if !containsString(slice, "b") {
		t.Error("should contain 'b'")
	}
	if containsString(slice, "d") {
		t.Error("should not contain 'd'")
	}
	if containsString(nil, "a") {
		t.Error("nil slice should not contain anything")
	}
}
