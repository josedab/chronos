package rbac

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditLogger logs RBAC actions.
type AuditLogger struct {
	entries []AuditEntry
	mu      sync.RWMutex
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger() *AuditLogger {
	return &AuditLogger{
		entries: make([]AuditEntry, 0),
	}
}

// Log records an audit entry.
func (l *AuditLogger) Log(entry *AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	l.entries = append(l.entries, *entry)
}

// Query returns audit entries matching the filter.
func (l *AuditLogger) Query(filter AuditFilter) []AuditEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var results []AuditEntry
	for _, entry := range l.entries {
		if l.matchesFilter(entry, filter) {
			results = append(results, entry)
		}
	}
	return results
}

func (l *AuditLogger) matchesFilter(entry AuditEntry, filter AuditFilter) bool {
	if filter.UserID != "" && entry.UserID != filter.UserID {
		return false
	}
	if filter.Action != "" && entry.Action != filter.Action {
		return false
	}
	if filter.Resource != "" && entry.Resource != filter.Resource {
		return false
	}
	if filter.Namespace != "" && entry.Namespace != filter.Namespace {
		return false
	}
	if !filter.StartTime.IsZero() && entry.Timestamp.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && entry.Timestamp.After(filter.EndTime) {
		return false
	}
	return true
}

// AuditEntry represents an audit log entry.
type AuditEntry struct {
	ID        string                 `json:"id"`
	Action    string                 `json:"action"`
	UserID    string                 `json:"user_id,omitempty"`
	Resource  string                 `json:"resource"`
	Namespace string                 `json:"namespace,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	IPAddress string                 `json:"ip_address,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// AuditFilter filters audit entries.
type AuditFilter struct {
	UserID    string    `json:"user_id,omitempty"`
	Action    string    `json:"action,omitempty"`
	Resource  string    `json:"resource,omitempty"`
	Namespace string    `json:"namespace,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
}
