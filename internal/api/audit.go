// Package api provides the REST API router.
package api

import (
	"net/http"
	"time"

	"github.com/chronos/chronos/internal/rbac"
	"github.com/go-chi/chi/v5/middleware"
)

// AuditConfig holds audit logging configuration.
type AuditConfig struct {
	// Enabled determines if audit logging is active.
	Enabled bool
	// Logger is the audit logger instance.
	Logger *rbac.AuditLogger
}

// NewAuditMiddleware creates middleware that logs all API requests for audit purposes.
func NewAuditMiddleware(config AuditConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled || config.Logger == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Wrap response writer to capture status code
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			// Process request
			next.ServeHTTP(ww, r)

			// Log the request
			config.Logger.Log(&rbac.AuditEntry{
				Action:    mapMethodToAction(r.Method),
				UserID:    getUserIDForAudit(r),
				Resource:  r.URL.Path,
				Namespace: extractNamespace(r),
				IPAddress: getClientIP(r),
				UserAgent: r.UserAgent(),
				Success:   ww.Status() < 400,
				Details: map[string]interface{}{
					"method":      r.Method,
					"path":        r.URL.Path,
					"query":       r.URL.RawQuery,
					"status":      ww.Status(),
					"duration_ms": time.Since(start).Milliseconds(),
					"bytes":       ww.BytesWritten(),
				},
				Timestamp: start,
			})
		})
	}
}

// mapMethodToAction maps HTTP methods to audit action names.
func mapMethodToAction(method string) string {
	switch method {
	case http.MethodGet:
		return "api_read"
	case http.MethodPost:
		return "api_create"
	case http.MethodPut, http.MethodPatch:
		return "api_update"
	case http.MethodDelete:
		return "api_delete"
	default:
		return "api_request"
	}
}

// getUserIDForAudit extracts user ID for audit logging.
func getUserIDForAudit(r *http.Request) string {
	// Try RBAC user first
	if user := GetRBACUserFromContext(r.Context()); user != nil {
		return user.ID
	}
	// Fall back to API key tenant
	if apiKey := GetAPIKeyFromContext(r.Context()); apiKey != nil {
		return "tenant:" + apiKey.TenantID
	}
	// Fall back to IP
	return "anonymous:" + getClientIP(r)
}

// getClientIP extracts the client IP address.
func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// SecurityAuditLogger provides specialized logging for security events.
type SecurityAuditLogger struct {
	logger *rbac.AuditLogger
}

// NewSecurityAuditLogger creates a new security audit logger.
func NewSecurityAuditLogger(logger *rbac.AuditLogger) *SecurityAuditLogger {
	return &SecurityAuditLogger{logger: logger}
}

// LogAuthAttempt logs an authentication attempt.
func (l *SecurityAuditLogger) LogAuthAttempt(r *http.Request, success bool, reason string) {
	if l.logger == nil {
		return
	}

	entry := &rbac.AuditEntry{
		Action:    "auth_attempt",
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   success,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"path": r.URL.Path,
		},
	}

	if !success && reason != "" {
		entry.Error = reason
	}

	l.logger.Log(entry)
}

// LogAPIKeyCreated logs API key creation.
func (l *SecurityAuditLogger) LogAPIKeyCreated(tenantID, keyID, keyName string, scopes []string) {
	if l.logger == nil {
		return
	}

	l.logger.Log(&rbac.AuditEntry{
		Action:   "api_key_created",
		UserID:   tenantID,
		Resource: keyID,
		Success:  true,
		Details: map[string]interface{}{
			"key_name": keyName,
			"scopes":   scopes,
		},
		Timestamp: time.Now(),
	})
}

// LogAPIKeyRevoked logs API key revocation.
func (l *SecurityAuditLogger) LogAPIKeyRevoked(tenantID, keyID string) {
	if l.logger == nil {
		return
	}

	l.logger.Log(&rbac.AuditEntry{
		Action:    "api_key_revoked",
		UserID:    tenantID,
		Resource:  keyID,
		Success:   true,
		Timestamp: time.Now(),
	})
}

// LogPermissionDenied logs a permission denied event.
func (l *SecurityAuditLogger) LogPermissionDenied(userID, permission, namespace string, r *http.Request) {
	if l.logger == nil {
		return
	}

	l.logger.Log(&rbac.AuditEntry{
		Action:    "permission_denied",
		UserID:    userID,
		Resource:  permission,
		Namespace: namespace,
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   false,
		Details: map[string]interface{}{
			"path":   r.URL.Path,
			"method": r.Method,
		},
		Timestamp: time.Now(),
	})
}

// LogRoleChange logs a role change event.
func (l *SecurityAuditLogger) LogRoleChange(actorID, targetUserID string, oldRoles, newRoles []string) {
	if l.logger == nil {
		return
	}

	l.logger.Log(&rbac.AuditEntry{
		Action:   "role_change",
		UserID:   actorID,
		Resource: targetUserID,
		Success:  true,
		Details: map[string]interface{}{
			"old_roles": oldRoles,
			"new_roles": newRoles,
		},
		Timestamp: time.Now(),
	})
}

// LogSuspiciousActivity logs potentially suspicious activity.
func (l *SecurityAuditLogger) LogSuspiciousActivity(reason string, r *http.Request, details map[string]interface{}) {
	if l.logger == nil {
		return
	}

	entry := &rbac.AuditEntry{
		Action:    "suspicious_activity",
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
		Success:   false,
		Error:     reason,
		Details:   details,
		Timestamp: time.Now(),
	}

	if user := GetRBACUserFromContext(r.Context()); user != nil {
		entry.UserID = user.ID
	}

	l.logger.Log(entry)
}
