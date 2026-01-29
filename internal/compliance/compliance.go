// Package compliance provides compliance and audit capabilities for Chronos.
package compliance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ComplianceConfig configures the compliance module.
type ComplianceConfig struct {
	Frameworks        []Framework       `json:"frameworks"`
	RetentionPolicy   RetentionPolicy   `json:"retention_policy"`
	PIIDetection      PIIDetectionConfig `json:"pii_detection"`
	AuditLogImmutable bool              `json:"audit_log_immutable"`
}

// Framework represents a compliance framework.
type Framework string

const (
	FrameworkSOC2   Framework = "soc2"
	FrameworkHIPAA  Framework = "hipaa"
	FrameworkGDPR   Framework = "gdpr"
	FrameworkPCI    Framework = "pci_dss"
	FrameworkISO27001 Framework = "iso27001"
)

// RetentionPolicy defines data retention rules.
type RetentionPolicy struct {
	ExecutionLogs      time.Duration `json:"execution_logs"`
	AuditLogs          time.Duration `json:"audit_logs"`
	JobHistory         time.Duration `json:"job_history"`
	DeletedJobRetention time.Duration `json:"deleted_job_retention"`
}

// DefaultRetentionPolicy returns default retention periods.
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		ExecutionLogs:       90 * 24 * time.Hour,  // 90 days
		AuditLogs:           365 * 24 * time.Hour, // 1 year
		JobHistory:          180 * 24 * time.Hour, // 180 days
		DeletedJobRetention: 30 * 24 * time.Hour,  // 30 days
	}
}

// PIIDetectionConfig configures PII detection.
type PIIDetectionConfig struct {
	Enabled      bool     `json:"enabled"`
	Patterns     []string `json:"patterns,omitempty"`
	BlockOnDetect bool    `json:"block_on_detect"`
	MaskInLogs   bool     `json:"mask_in_logs"`
}

// ComplianceManager manages compliance features.
type ComplianceManager struct {
	config      ComplianceConfig
	auditLog    *ImmutableAuditLog
	piiDetector *PIIDetector
	controls    map[string]*Control
	findings    []*Finding
	mu          sync.RWMutex
}

// NewComplianceManager creates a compliance manager.
func NewComplianceManager(config ComplianceConfig) *ComplianceManager {
	cm := &ComplianceManager{
		config:      config,
		auditLog:    NewImmutableAuditLog(config.AuditLogImmutable),
		piiDetector: NewPIIDetector(config.PIIDetection),
		controls:    make(map[string]*Control),
		findings:    make([]*Finding, 0),
	}

	// Load controls for enabled frameworks
	for _, framework := range config.Frameworks {
		for _, control := range GetFrameworkControls(framework) {
			cm.controls[control.ID] = control
		}
	}

	return cm
}

// RecordAuditEvent records an audit event.
func (m *ComplianceManager) RecordAuditEvent(event *AuditEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	event.Timestamp = time.Now()
	event.Hash = m.computeEventHash(event)

	return m.auditLog.Append(event)
}

// CheckPII checks content for PII.
func (m *ComplianceManager) CheckPII(content string) (*PIICheckResult, error) {
	return m.piiDetector.Check(content)
}

// EvaluateControls evaluates all compliance controls.
func (m *ComplianceManager) EvaluateControls(ctx context.Context, evidence *EvidenceCollector) (*ComplianceReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	report := &ComplianceReport{
		ID:          uuid.New().String(),
		GeneratedAt: time.Now(),
		Frameworks:  m.config.Frameworks,
		Controls:    make([]ControlResult, 0),
		Findings:    make([]*Finding, 0),
	}

	passedCount := 0
	failedCount := 0

	for _, control := range m.controls {
		result := m.evaluateControl(ctx, control, evidence)
		report.Controls = append(report.Controls, result)

		if result.Status == ControlStatusPass {
			passedCount++
		} else if result.Status == ControlStatusFail {
			failedCount++
			report.Findings = append(report.Findings, &Finding{
				ID:          uuid.New().String(),
				ControlID:   control.ID,
				Severity:    control.Severity,
				Title:       fmt.Sprintf("Control Failed: %s", control.Name),
				Description: result.Evidence,
				Remediation: control.Remediation,
				DetectedAt:  time.Now(),
			})
		}
	}

	report.Summary = ComplianceSummary{
		TotalControls:  len(m.controls),
		PassedControls: passedCount,
		FailedControls: failedCount,
		ComplianceRate: float64(passedCount) / float64(len(m.controls)) * 100,
	}

	return report, nil
}

func (m *ComplianceManager) evaluateControl(ctx context.Context, control *Control, evidence *EvidenceCollector) ControlResult {
	result := ControlResult{
		ControlID:   control.ID,
		ControlName: control.Name,
		Status:      ControlStatusPending,
		EvaluatedAt: time.Now(),
	}

	// Execute control check
	if control.CheckFunc != nil {
		passed, evidenceStr := control.CheckFunc(ctx, evidence)
		result.Evidence = evidenceStr
		if passed {
			result.Status = ControlStatusPass
		} else {
			result.Status = ControlStatusFail
		}
	} else {
		result.Status = ControlStatusManual
		result.Evidence = "Manual verification required"
	}

	return result
}

func (m *ComplianceManager) computeEventHash(event *AuditEvent) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%v",
		event.ID, event.Actor, event.Action, event.Timestamp.Format(time.RFC3339), event.Resource)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// GetAuditLog retrieves audit log entries.
func (m *ComplianceManager) GetAuditLog(filter AuditFilter) ([]*AuditEvent, error) {
	return m.auditLog.Query(filter)
}

// ExportAuditLog exports audit logs for external auditors.
func (m *ComplianceManager) ExportAuditLog(format ExportFormat, filter AuditFilter) ([]byte, error) {
	events, err := m.auditLog.Query(filter)
	if err != nil {
		return nil, err
	}

	switch format {
	case ExportFormatJSON:
		return json.MarshalIndent(events, "", "  ")
	case ExportFormatCSV:
		return m.exportToCSV(events)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

func (m *ComplianceManager) exportToCSV(events []*AuditEvent) ([]byte, error) {
	var result string
	result = "ID,Timestamp,Actor,Action,Resource,Outcome,IP Address\n"
	for _, e := range events {
		result += fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s\n",
			e.ID, e.Timestamp.Format(time.RFC3339), e.Actor, e.Action,
			e.Resource, e.Outcome, e.IPAddress)
	}
	return []byte(result), nil
}

// AuditEvent represents an audit log entry.
type AuditEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Actor       string                 `json:"actor"`
	ActorType   string                 `json:"actor_type"` // "user", "service", "system"
	Action      string                 `json:"action"`
	Resource    string                 `json:"resource"`
	ResourceID  string                 `json:"resource_id"`
	Outcome     string                 `json:"outcome"` // "success", "failure", "denied"
	Details     map[string]interface{} `json:"details,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	Hash        string                 `json:"hash"` // For immutability verification
	PrevHash    string                 `json:"prev_hash,omitempty"`
}

// AuditFilter filters audit events.
type AuditFilter struct {
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Actor      string    `json:"actor,omitempty"`
	Action     string    `json:"action,omitempty"`
	Resource   string    `json:"resource,omitempty"`
	Outcome    string    `json:"outcome,omitempty"`
	Limit      int       `json:"limit,omitempty"`
	Offset     int       `json:"offset,omitempty"`
}

// ExportFormat represents export formats.
type ExportFormat string

const (
	ExportFormatJSON ExportFormat = "json"
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatPDF  ExportFormat = "pdf"
)

// ImmutableAuditLog is a tamper-evident audit log.
type ImmutableAuditLog struct {
	entries   []*AuditEvent
	immutable bool
	mu        sync.RWMutex
}

// NewImmutableAuditLog creates an immutable audit log.
func NewImmutableAuditLog(immutable bool) *ImmutableAuditLog {
	return &ImmutableAuditLog{
		entries:   make([]*AuditEvent, 0),
		immutable: immutable,
	}
}

// Append adds an event to the log.
func (l *ImmutableAuditLog) Append(event *AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Chain hash for immutability
	if l.immutable && len(l.entries) > 0 {
		event.PrevHash = l.entries[len(l.entries)-1].Hash
	}

	l.entries = append(l.entries, event)
	return nil
}

// Query retrieves events matching the filter.
func (l *ImmutableAuditLog) Query(filter AuditFilter) ([]*AuditEvent, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var results []*AuditEvent

	for _, e := range l.entries {
		if !filter.StartTime.IsZero() && e.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && e.Timestamp.After(filter.EndTime) {
			continue
		}
		if filter.Actor != "" && e.Actor != filter.Actor {
			continue
		}
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		if filter.Resource != "" && e.Resource != filter.Resource {
			continue
		}
		if filter.Outcome != "" && e.Outcome != filter.Outcome {
			continue
		}

		results = append(results, e)
	}

	// Apply pagination
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

// VerifyIntegrity verifies the audit log integrity.
func (l *ImmutableAuditLog) VerifyIntegrity() (bool, []string) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if !l.immutable || len(l.entries) == 0 {
		return true, nil
	}

	var violations []string
	for i := 1; i < len(l.entries); i++ {
		if l.entries[i].PrevHash != l.entries[i-1].Hash {
			violations = append(violations, fmt.Sprintf("Chain broken at entry %d", i))
		}
	}

	return len(violations) == 0, violations
}

// PIIDetector detects personally identifiable information.
type PIIDetector struct {
	config   PIIDetectionConfig
	patterns []*regexp.Regexp
}

// NewPIIDetector creates a PII detector.
func NewPIIDetector(config PIIDetectionConfig) *PIIDetector {
	detector := &PIIDetector{
		config:   config,
		patterns: make([]*regexp.Regexp, 0),
	}

	// Default patterns
	defaultPatterns := []string{
		`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`,                    // Email
		`\b\d{3}-\d{2}-\d{4}\b`,                                                  // SSN
		`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,                            // Credit card
		`\b(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b`,      // Phone
		`\b\d{5}(?:-\d{4})?\b`,                                                   // ZIP code
		`(?i)\b(?:password|passwd|pwd|secret|api[_-]?key|token)\s*[:=]\s*\S+`,   // Secrets
	}

	patterns := defaultPatterns
	if len(config.Patterns) > 0 {
		patterns = append(patterns, config.Patterns...)
	}

	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			detector.patterns = append(detector.patterns, re)
		}
	}

	return detector
}

// Check checks content for PII.
func (d *PIIDetector) Check(content string) (*PIICheckResult, error) {
	if !d.config.Enabled {
		return &PIICheckResult{HasPII: false}, nil
	}

	result := &PIICheckResult{
		HasPII:  false,
		Matches: make([]PIIMatch, 0),
	}

	for _, pattern := range d.patterns {
		matches := pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			result.HasPII = true
			result.Matches = append(result.Matches, PIIMatch{
				Type:     d.patternType(pattern.String()),
				Start:    match[0],
				End:      match[1],
				Masked:   d.mask(content[match[0]:match[1]]),
			})
		}
	}

	return result, nil
}

func (d *PIIDetector) patternType(pattern string) string {
	switch {
	case contains(pattern, "email"):
		return "email"
	case contains(pattern, "\\d{3}-\\d{2}-\\d{4}"):
		return "ssn"
	case contains(pattern, "\\d{4}"):
		return "credit_card"
	case contains(pattern, "phone"):
		return "phone"
	case contains(pattern, "password") || contains(pattern, "secret"):
		return "secret"
	default:
		return "pii"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}

func (d *PIIDetector) mask(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + "****" + value[len(value)-2:]
}

// Mask masks PII in content.
func (d *PIIDetector) Mask(content string) string {
	if !d.config.MaskInLogs {
		return content
	}

	masked := content
	for _, pattern := range d.patterns {
		masked = pattern.ReplaceAllStringFunc(masked, func(s string) string {
			return d.mask(s)
		})
	}
	return masked
}

// PIICheckResult contains PII detection results.
type PIICheckResult struct {
	HasPII  bool       `json:"has_pii"`
	Matches []PIIMatch `json:"matches"`
}

// PIIMatch is a single PII match.
type PIIMatch struct {
	Type   string `json:"type"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Masked string `json:"masked"`
}

// Control represents a compliance control.
type Control struct {
	ID          string                                                      `json:"id"`
	Framework   Framework                                                   `json:"framework"`
	Category    string                                                      `json:"category"`
	Name        string                                                      `json:"name"`
	Description string                                                      `json:"description"`
	Severity    string                                                      `json:"severity"` // "critical", "high", "medium", "low"
	Remediation string                                                      `json:"remediation"`
	CheckFunc   func(context.Context, *EvidenceCollector) (bool, string)   `json:"-"`
}

// ControlStatus represents control evaluation status.
type ControlStatus string

const (
	ControlStatusPass    ControlStatus = "pass"
	ControlStatusFail    ControlStatus = "fail"
	ControlStatusPending ControlStatus = "pending"
	ControlStatusManual  ControlStatus = "manual"
)

// ControlResult is the result of a control evaluation.
type ControlResult struct {
	ControlID   string        `json:"control_id"`
	ControlName string        `json:"control_name"`
	Status      ControlStatus `json:"status"`
	Evidence    string        `json:"evidence"`
	EvaluatedAt time.Time     `json:"evaluated_at"`
}

// ComplianceReport is a compliance assessment report.
type ComplianceReport struct {
	ID          string            `json:"id"`
	GeneratedAt time.Time         `json:"generated_at"`
	Frameworks  []Framework       `json:"frameworks"`
	Summary     ComplianceSummary `json:"summary"`
	Controls    []ControlResult   `json:"controls"`
	Findings    []*Finding        `json:"findings"`
}

// ComplianceSummary summarizes compliance status.
type ComplianceSummary struct {
	TotalControls  int     `json:"total_controls"`
	PassedControls int     `json:"passed_controls"`
	FailedControls int     `json:"failed_controls"`
	ComplianceRate float64 `json:"compliance_rate"`
}

// Finding represents a compliance finding.
type Finding struct {
	ID          string    `json:"id"`
	ControlID   string    `json:"control_id"`
	Severity    string    `json:"severity"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Remediation string    `json:"remediation"`
	DetectedAt  time.Time `json:"detected_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Status      string    `json:"status"` // "open", "in_progress", "resolved"
}

// EvidenceCollector collects evidence for compliance checks.
type EvidenceCollector struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

// NewEvidenceCollector creates an evidence collector.
func NewEvidenceCollector() *EvidenceCollector {
	return &EvidenceCollector{
		data: make(map[string]interface{}),
	}
}

// Set sets evidence data.
func (c *EvidenceCollector) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

// Get retrieves evidence data.
func (c *EvidenceCollector) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

// GetBool retrieves a boolean value.
func (c *EvidenceCollector) GetBool(key string) bool {
	v, ok := c.Get(key)
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// GetInt retrieves an integer value.
func (c *EvidenceCollector) GetInt(key string) int {
	v, ok := c.Get(key)
	if !ok {
		return 0
	}
	i, ok := v.(int)
	if ok {
		return i
	}
	return 0
}

// GetFrameworkControls returns controls for a framework.
func GetFrameworkControls(framework Framework) []*Control {
	switch framework {
	case FrameworkSOC2:
		return getSOC2Controls()
	case FrameworkHIPAA:
		return getHIPAAControls()
	case FrameworkGDPR:
		return getGDPRControls()
	default:
		return []*Control{}
	}
}

func getSOC2Controls() []*Control {
	return []*Control{
		{
			ID:          "SOC2-CC6.1",
			Framework:   FrameworkSOC2,
			Category:    "Logical and Physical Access Controls",
			Name:        "Access Control",
			Description: "The entity implements logical access security measures to protect against unauthorized access",
			Severity:    "high",
			Remediation: "Implement RBAC and enforce authentication for all API endpoints",
			CheckFunc: func(ctx context.Context, e *EvidenceCollector) (bool, string) {
				if e.GetBool("rbac_enabled") && e.GetBool("auth_required") {
					return true, "RBAC enabled and authentication required"
				}
				return false, "RBAC or authentication not properly configured"
			},
		},
		{
			ID:          "SOC2-CC6.6",
			Framework:   FrameworkSOC2,
			Category:    "Logical and Physical Access Controls",
			Name:        "Encryption in Transit",
			Description: "The entity implements encryption to protect data in transit",
			Severity:    "high",
			Remediation: "Enable TLS for all network communications",
			CheckFunc: func(ctx context.Context, e *EvidenceCollector) (bool, string) {
				if e.GetBool("tls_enabled") {
					return true, "TLS encryption enabled for all communications"
				}
				return false, "TLS not enabled for all communications"
			},
		},
		{
			ID:          "SOC2-CC7.2",
			Framework:   FrameworkSOC2,
			Category:    "System Operations",
			Name:        "Security Monitoring",
			Description: "The entity monitors system components for anomalies and security incidents",
			Severity:    "medium",
			Remediation: "Enable audit logging and monitoring",
			CheckFunc: func(ctx context.Context, e *EvidenceCollector) (bool, string) {
				if e.GetBool("audit_logging_enabled") && e.GetBool("monitoring_enabled") {
					return true, "Audit logging and monitoring enabled"
				}
				return false, "Audit logging or monitoring not enabled"
			},
		},
		{
			ID:          "SOC2-CC8.1",
			Framework:   FrameworkSOC2,
			Category:    "Change Management",
			Name:        "Change Management",
			Description: "The entity authorizes, documents, and controls changes to infrastructure and software",
			Severity:    "medium",
			Remediation: "Implement version control and change approval workflows",
			CheckFunc: func(ctx context.Context, e *EvidenceCollector) (bool, string) {
				if e.GetBool("version_control_enabled") {
					return true, "Version control and change management in place"
				}
				return false, "Change management not properly implemented"
			},
		},
	}
}

func getHIPAAControls() []*Control {
	return []*Control{
		{
			ID:          "HIPAA-164.312(a)(1)",
			Framework:   FrameworkHIPAA,
			Category:    "Access Control",
			Name:        "Access Control",
			Description: "Implement technical policies and procedures for systems with ePHI",
			Severity:    "critical",
			Remediation: "Implement unique user identification, automatic logoff, and encryption",
			CheckFunc: func(ctx context.Context, e *EvidenceCollector) (bool, string) {
				if e.GetBool("unique_user_ids") && e.GetBool("session_timeout_enabled") {
					return true, "Unique user IDs and session timeouts configured"
				}
				return false, "Access controls not fully implemented"
			},
		},
		{
			ID:          "HIPAA-164.312(b)",
			Framework:   FrameworkHIPAA,
			Category:    "Audit Controls",
			Name:        "Audit Controls",
			Description: "Implement hardware, software, and/or procedural mechanisms to record and examine activity",
			Severity:    "high",
			Remediation: "Enable comprehensive audit logging",
			CheckFunc: func(ctx context.Context, e *EvidenceCollector) (bool, string) {
				if e.GetBool("audit_logging_enabled") && e.GetInt("audit_retention_days") >= 365 {
					return true, "Audit logging enabled with appropriate retention"
				}
				return false, "Audit controls not meeting requirements"
			},
		},
		{
			ID:          "HIPAA-164.312(e)(1)",
			Framework:   FrameworkHIPAA,
			Category:    "Transmission Security",
			Name:        "Transmission Security",
			Description: "Implement technical security measures to guard against unauthorized access to ePHI transmitted over networks",
			Severity:    "critical",
			Remediation: "Implement encryption for data in transit",
			CheckFunc: func(ctx context.Context, e *EvidenceCollector) (bool, string) {
				if e.GetBool("tls_enabled") && e.GetBool("encryption_required") {
					return true, "Transmission security controls in place"
				}
				return false, "Transmission security not properly configured"
			},
		},
	}
}

func getGDPRControls() []*Control {
	return []*Control{
		{
			ID:          "GDPR-Article-5",
			Framework:   FrameworkGDPR,
			Category:    "Data Processing Principles",
			Name:        "Data Minimization",
			Description: "Personal data shall be adequate, relevant and limited to what is necessary",
			Severity:    "high",
			Remediation: "Review and minimize personal data collection",
			CheckFunc: func(ctx context.Context, e *EvidenceCollector) (bool, string) {
				if e.GetBool("data_minimization_policy") {
					return true, "Data minimization policy implemented"
				}
				return false, "Data minimization policy not defined"
			},
		},
		{
			ID:          "GDPR-Article-17",
			Framework:   FrameworkGDPR,
			Category:    "Data Subject Rights",
			Name:        "Right to Erasure",
			Description: "The data subject shall have the right to obtain erasure of personal data",
			Severity:    "high",
			Remediation: "Implement data deletion capabilities",
			CheckFunc: func(ctx context.Context, e *EvidenceCollector) (bool, string) {
				if e.GetBool("data_deletion_enabled") {
					return true, "Data deletion capabilities implemented"
				}
				return false, "Data deletion not properly implemented"
			},
		},
		{
			ID:          "GDPR-Article-32",
			Framework:   FrameworkGDPR,
			Category:    "Security of Processing",
			Name:        "Security Measures",
			Description: "Implement appropriate technical and organisational measures to ensure security",
			Severity:    "critical",
			Remediation: "Implement encryption, access controls, and security monitoring",
			CheckFunc: func(ctx context.Context, e *EvidenceCollector) (bool, string) {
				if e.GetBool("encryption_at_rest") && e.GetBool("encryption_in_transit") && e.GetBool("access_controls") {
					return true, "Security measures implemented"
				}
				return false, "Security measures incomplete"
			},
		},
	}
}
