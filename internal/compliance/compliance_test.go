package compliance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDefaultRetentionPolicy(t *testing.T) {
	policy := DefaultRetentionPolicy()

	if policy.ExecutionLogs != 90*24*time.Hour {
		t.Errorf("ExecutionLogs = %v, want 90 days", policy.ExecutionLogs)
	}
	if policy.AuditLogs != 365*24*time.Hour {
		t.Errorf("AuditLogs = %v, want 365 days", policy.AuditLogs)
	}
	if policy.JobHistory != 180*24*time.Hour {
		t.Errorf("JobHistory = %v, want 180 days", policy.JobHistory)
	}
	if policy.DeletedJobRetention != 30*24*time.Hour {
		t.Errorf("DeletedJobRetention = %v, want 30 days", policy.DeletedJobRetention)
	}
}

func TestNewComplianceManager(t *testing.T) {
	config := ComplianceConfig{
		Frameworks:        []Framework{FrameworkSOC2, FrameworkHIPAA},
		RetentionPolicy:   DefaultRetentionPolicy(),
		AuditLogImmutable: true,
	}

	cm := NewComplianceManager(config)

	if cm == nil {
		t.Fatal("NewComplianceManager returned nil")
	}
	if len(cm.controls) == 0 {
		t.Error("expected controls to be loaded")
	}
}

func TestComplianceManager_RecordAuditEvent(t *testing.T) {
	config := ComplianceConfig{
		AuditLogImmutable: true,
	}
	cm := NewComplianceManager(config)

	event := &AuditEvent{
		Actor:    "user@example.com",
		Action:   "job.create",
		Resource: "job",
	}

	err := cm.RecordAuditEvent(event)
	if err != nil {
		t.Fatalf("RecordAuditEvent failed: %v", err)
	}

	if event.ID == "" {
		t.Error("event ID should be set")
	}
	if event.Timestamp.IsZero() {
		t.Error("event timestamp should be set")
	}
	if event.Hash == "" {
		t.Error("event hash should be computed")
	}
}

func TestComplianceManager_CheckPII(t *testing.T) {
	config := ComplianceConfig{
		PIIDetection: PIIDetectionConfig{
			Enabled: true,
		},
	}
	cm := NewComplianceManager(config)

	tests := []struct {
		name      string
		content   string
		expectPII bool
	}{
		{
			name:      "email detected",
			content:   "Contact me at test@example.com",
			expectPII: true,
		},
		{
			name:      "SSN detected",
			content:   "SSN: 123-45-6789",
			expectPII: true,
		},
		{
			name:      "no PII",
			content:   "Hello world",
			expectPII: false,
		},
		{
			name:      "credit card detected",
			content:   "Card: 1234-5678-9012-3456",
			expectPII: true,
		},
		{
			name:      "password detected",
			content:   "password: secretvalue123",
			expectPII: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := cm.CheckPII(tt.content)
			if err != nil {
				t.Fatalf("CheckPII failed: %v", err)
			}
			if result.HasPII != tt.expectPII {
				t.Errorf("HasPII = %v, want %v", result.HasPII, tt.expectPII)
			}
		})
	}
}

func TestComplianceManager_EvaluateControls(t *testing.T) {
	config := ComplianceConfig{
		Frameworks: []Framework{FrameworkSOC2},
	}
	cm := NewComplianceManager(config)

	evidence := NewEvidenceCollector()
	evidence.Set("rbac_enabled", true)
	evidence.Set("auth_required", true)
	evidence.Set("tls_enabled", true)
	evidence.Set("audit_logging_enabled", true)
	evidence.Set("monitoring_enabled", true)
	evidence.Set("version_control_enabled", true)

	ctx := context.Background()
	report, err := cm.EvaluateControls(ctx, evidence)
	if err != nil {
		t.Fatalf("EvaluateControls failed: %v", err)
	}

	if report.ID == "" {
		t.Error("report ID should be set")
	}
	if len(report.Controls) == 0 {
		t.Error("expected control results")
	}
	if report.Summary.TotalControls == 0 {
		t.Error("expected total controls > 0")
	}
}

func TestComplianceManager_GetAuditLog(t *testing.T) {
	config := ComplianceConfig{}
	cm := NewComplianceManager(config)

	// Record some events
	now := time.Now()
	for i := 0; i < 5; i++ {
		cm.RecordAuditEvent(&AuditEvent{
			Actor:    "user@example.com",
			Action:   "job.create",
			Resource: "job",
		})
	}

	filter := AuditFilter{
		StartTime: now.Add(-time.Hour),
		EndTime:   now.Add(time.Hour),
	}

	events, err := cm.GetAuditLog(filter)
	if err != nil {
		t.Fatalf("GetAuditLog failed: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}

func TestComplianceManager_ExportAuditLog(t *testing.T) {
	config := ComplianceConfig{}
	cm := NewComplianceManager(config)

	cm.RecordAuditEvent(&AuditEvent{
		Actor:    "user@example.com",
		Action:   "job.create",
		Resource: "job",
		Outcome:  "success",
	})

	filter := AuditFilter{}

	t.Run("JSON export", func(t *testing.T) {
		data, err := cm.ExportAuditLog(ExportFormatJSON, filter)
		if err != nil {
			t.Fatalf("ExportAuditLog failed: %v", err)
		}

		var events []*AuditEvent
		if err := json.Unmarshal(data, &events); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("expected 1 event, got %d", len(events))
		}
	})

	t.Run("CSV export", func(t *testing.T) {
		data, err := cm.ExportAuditLog(ExportFormatCSV, filter)
		if err != nil {
			t.Fatalf("ExportAuditLog failed: %v", err)
		}

		csv := string(data)
		if !strings.Contains(csv, "ID,Timestamp,Actor,Action") {
			t.Error("CSV should contain header")
		}
		if !strings.Contains(csv, "user@example.com") {
			t.Error("CSV should contain event data")
		}
	})

	t.Run("unsupported format", func(t *testing.T) {
		_, err := cm.ExportAuditLog(ExportFormatPDF, filter)
		if err == nil {
			t.Error("expected error for unsupported format")
		}
	})
}

func TestImmutableAuditLog_Append(t *testing.T) {
	log := NewImmutableAuditLog(true)

	event1 := &AuditEvent{
		ID:   "1",
		Hash: "hash1",
	}
	event2 := &AuditEvent{
		ID:   "2",
		Hash: "hash2",
	}

	log.Append(event1)
	log.Append(event2)

	if event2.PrevHash != "hash1" {
		t.Errorf("PrevHash = %s, want hash1", event2.PrevHash)
	}
}

func TestImmutableAuditLog_Query(t *testing.T) {
	log := NewImmutableAuditLog(false)

	now := time.Now()
	events := []*AuditEvent{
		{ID: "1", Actor: "user1", Action: "create", Timestamp: now},
		{ID: "2", Actor: "user2", Action: "update", Timestamp: now.Add(time.Hour)},
		{ID: "3", Actor: "user1", Action: "delete", Timestamp: now.Add(2 * time.Hour)},
	}

	for _, e := range events {
		log.Append(e)
	}

	tests := []struct {
		name       string
		filter     AuditFilter
		wantCount  int
	}{
		{
			name:      "filter by actor",
			filter:    AuditFilter{Actor: "user1"},
			wantCount: 2,
		},
		{
			name:      "filter by action",
			filter:    AuditFilter{Action: "create"},
			wantCount: 1,
		},
		{
			name:      "filter by time range",
			filter:    AuditFilter{StartTime: now.Add(30 * time.Minute), EndTime: now.Add(3 * time.Hour)},
			wantCount: 2,
		},
		{
			name:      "with limit",
			filter:    AuditFilter{Limit: 2},
			wantCount: 2,
		},
		{
			name:      "with offset",
			filter:    AuditFilter{Offset: 1, Limit: 10},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := log.Query(tt.filter)
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("got %d results, want %d", len(results), tt.wantCount)
			}
		})
	}
}

func TestImmutableAuditLog_VerifyIntegrity(t *testing.T) {
	t.Run("valid chain", func(t *testing.T) {
		log := NewImmutableAuditLog(true)
		log.Append(&AuditEvent{ID: "1", Hash: "hash1"})
		log.Append(&AuditEvent{ID: "2", Hash: "hash2"})

		valid, violations := log.VerifyIntegrity()
		if !valid {
			t.Errorf("expected valid chain, got violations: %v", violations)
		}
	})

	t.Run("broken chain", func(t *testing.T) {
		log := NewImmutableAuditLog(true)
		log.entries = []*AuditEvent{
			{ID: "1", Hash: "hash1"},
			{ID: "2", Hash: "hash2", PrevHash: "wrong"},
		}

		valid, violations := log.VerifyIntegrity()
		if valid {
			t.Error("expected invalid chain")
		}
		if len(violations) == 0 {
			t.Error("expected violations")
		}
	})

	t.Run("non-immutable log", func(t *testing.T) {
		log := NewImmutableAuditLog(false)
		log.Append(&AuditEvent{ID: "1"})

		valid, _ := log.VerifyIntegrity()
		if !valid {
			t.Error("non-immutable log should always be valid")
		}
	})
}

func TestPIIDetector_Check(t *testing.T) {
	detector := NewPIIDetector(PIIDetectionConfig{Enabled: true})

	result, err := detector.Check("Email: test@example.com, SSN: 123-45-6789")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if !result.HasPII {
		t.Error("expected PII to be detected")
	}
	if len(result.Matches) < 2 {
		t.Errorf("expected at least 2 matches, got %d", len(result.Matches))
	}
}

func TestPIIDetector_CheckDisabled(t *testing.T) {
	detector := NewPIIDetector(PIIDetectionConfig{Enabled: false})

	result, err := detector.Check("Email: test@example.com")
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.HasPII {
		t.Error("disabled detector should not detect PII")
	}
}

func TestPIIDetector_Mask(t *testing.T) {
	detector := NewPIIDetector(PIIDetectionConfig{
		Enabled:    true,
		MaskInLogs: true,
	})

	input := "Contact: test@example.com"
	masked := detector.Mask(input)

	if masked == input {
		t.Error("content should be masked")
	}
	if strings.Contains(masked, "test@example.com") {
		t.Error("email should be masked")
	}
}

func TestPIIDetector_MaskDisabled(t *testing.T) {
	detector := NewPIIDetector(PIIDetectionConfig{
		Enabled:    true,
		MaskInLogs: false,
	})

	input := "Contact: test@example.com"
	masked := detector.Mask(input)

	if masked != input {
		t.Error("content should not be masked when MaskInLogs is false")
	}
}

func TestEvidenceCollector(t *testing.T) {
	ec := NewEvidenceCollector()

	t.Run("Set and Get", func(t *testing.T) {
		ec.Set("key1", "value1")
		v, ok := ec.Get("key1")
		if !ok || v != "value1" {
			t.Error("Set/Get failed")
		}
	})

	t.Run("Get non-existent", func(t *testing.T) {
		_, ok := ec.Get("nonexistent")
		if ok {
			t.Error("expected false for non-existent key")
		}
	})

	t.Run("GetBool", func(t *testing.T) {
		ec.Set("bool_true", true)
		ec.Set("bool_false", false)
		ec.Set("not_bool", "string")

		if !ec.GetBool("bool_true") {
			t.Error("GetBool should return true")
		}
		if ec.GetBool("bool_false") {
			t.Error("GetBool should return false")
		}
		if ec.GetBool("not_bool") {
			t.Error("GetBool should return false for non-bool")
		}
		if ec.GetBool("nonexistent") {
			t.Error("GetBool should return false for non-existent")
		}
	})

	t.Run("GetInt", func(t *testing.T) {
		ec.Set("int_val", 42)
		ec.Set("not_int", "string")

		if ec.GetInt("int_val") != 42 {
			t.Error("GetInt should return 42")
		}
		if ec.GetInt("not_int") != 0 {
			t.Error("GetInt should return 0 for non-int")
		}
		if ec.GetInt("nonexistent") != 0 {
			t.Error("GetInt should return 0 for non-existent")
		}
	})
}

func TestGetFrameworkControls(t *testing.T) {
	tests := []struct {
		framework    Framework
		minControls  int
	}{
		{FrameworkSOC2, 4},
		{FrameworkHIPAA, 3},
		{FrameworkGDPR, 3},
		{Framework("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.framework), func(t *testing.T) {
			controls := GetFrameworkControls(tt.framework)
			if len(controls) < tt.minControls {
				t.Errorf("got %d controls, want at least %d", len(controls), tt.minControls)
			}
		})
	}
}

func TestControlStatus(t *testing.T) {
	statuses := []ControlStatus{
		ControlStatusPass,
		ControlStatusFail,
		ControlStatusPending,
		ControlStatusManual,
	}

	expected := []string{"pass", "fail", "pending", "manual"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("ControlStatus %d = %s, want %s", i, s, expected[i])
		}
	}
}

func TestFrameworkConstants(t *testing.T) {
	frameworks := []Framework{
		FrameworkSOC2,
		FrameworkHIPAA,
		FrameworkGDPR,
		FrameworkPCI,
		FrameworkISO27001,
	}

	expected := []string{"soc2", "hipaa", "gdpr", "pci_dss", "iso27001"}

	for i, f := range frameworks {
		if string(f) != expected[i] {
			t.Errorf("Framework %d = %s, want %s", i, f, expected[i])
		}
	}
}

func TestExportFormatConstants(t *testing.T) {
	formats := []ExportFormat{
		ExportFormatJSON,
		ExportFormatCSV,
		ExportFormatPDF,
	}

	expected := []string{"json", "csv", "pdf"}

	for i, f := range formats {
		if string(f) != expected[i] {
			t.Errorf("ExportFormat %d = %s, want %s", i, f, expected[i])
		}
	}
}

func TestConcurrentAuditLogAccess(t *testing.T) {
	log := NewImmutableAuditLog(true)

	done := make(chan bool)

	// Writer
	go func() {
		for i := 0; i < 100; i++ {
			log.Append(&AuditEvent{
				ID:   string(rune('0' + i)),
				Hash: "hash",
			})
		}
		done <- true
	}()

	// Reader
	go func() {
		for i := 0; i < 100; i++ {
			log.Query(AuditFilter{})
		}
		done <- true
	}()

	<-done
	<-done
}
