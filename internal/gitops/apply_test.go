package gitops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type mockJobLookup struct {
	jobs map[string]*JobSpec
}

func (m *mockJobLookup) GetJobByName(name string) (*JobSpec, bool) {
	j, ok := m.jobs[name]
	return j, ok
}

func TestParseJobSpecs(t *testing.T) {
	yaml := []byte(`
apiVersion: chronos/v1
kind: Job
metadata:
  name: daily-report
  namespace: production
  labels:
    team: analytics
spec:
  schedule: "0 9 * * *"
  timezone: America/New_York
  webhook:
    url: https://api.example.com/reports
    method: POST
  retry:
    maxAttempts: 3
    initialInterval: "1s"
    multiplier: 2.0
  timeout: "5m"
  enabled: true
---
apiVersion: chronos/v1
kind: Job
metadata:
  name: health-check
spec:
  schedule: "*/5 * * * *"
  webhook:
    url: https://api.example.com/health
    method: GET
`)

	specs, err := ParseJobSpecs(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
	if specs[0].Metadata.Name != "daily-report" {
		t.Errorf("expected daily-report, got %s", specs[0].Metadata.Name)
	}
	if specs[1].Spec.Schedule != "*/5 * * * *" {
		t.Errorf("expected schedule */5 * * * *, got %s", specs[1].Spec.Schedule)
	}
}

func TestParseJobSpecsFromFile(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "job1.yaml")
	f2 := filepath.Join(dir, "job2.yml")

	os.WriteFile(f1, []byte(`
metadata:
  name: job-one
spec:
  schedule: "* * * * *"
  webhook:
    url: http://localhost/one
`), 0644)

	os.WriteFile(f2, []byte(`
metadata:
  name: job-two
spec:
  schedule: "0 * * * *"
  webhook:
    url: http://localhost/two
`), 0644)

	specs, err := ParseJobSpecsFromFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs from directory, got %d", len(specs))
	}
}

func TestValidateJobSpec(t *testing.T) {
	t.Run("valid spec", func(t *testing.T) {
		spec := &JobSpec{
			Metadata: Metadata{Name: "test"},
			Spec:     JobData{Schedule: "* * * * *", Webhook: WebhookSpec{URL: "http://x.com"}},
		}
		errs := ValidateJobSpec(spec)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		spec := &JobSpec{
			Spec: JobData{Schedule: "* * * * *", Webhook: WebhookSpec{URL: "http://x.com"}},
		}
		errs := ValidateJobSpec(spec)
		if len(errs) != 1 {
			t.Errorf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})

	t.Run("missing all required", func(t *testing.T) {
		spec := &JobSpec{}
		errs := ValidateJobSpec(spec)
		if len(errs) != 3 {
			t.Errorf("expected 3 errors, got %d: %v", len(errs), errs)
		}
	})
}

func TestComputePlan_Create(t *testing.T) {
	specs := []JobSpec{
		{
			Metadata: Metadata{Name: "new-job"},
			Spec:     JobData{Schedule: "* * * * *", Webhook: WebhookSpec{URL: "http://x.com"}},
		},
	}

	lookup := &mockJobLookup{jobs: map[string]*JobSpec{}}
	plan := ComputePlan(context.Background(), specs, lookup)

	if len(plan.Creates) != 1 {
		t.Fatalf("expected 1 create, got %d", len(plan.Creates))
	}
	if plan.Creates[0].Name != "new-job" {
		t.Errorf("expected new-job, got %s", plan.Creates[0].Name)
	}
}

func TestComputePlan_Update(t *testing.T) {
	specs := []JobSpec{
		{
			Metadata: Metadata{Name: "existing-job"},
			Spec:     JobData{Schedule: "*/10 * * * *", Webhook: WebhookSpec{URL: "http://new.com"}},
		},
	}

	lookup := &mockJobLookup{jobs: map[string]*JobSpec{
		"existing-job": {
			Metadata: Metadata{Name: "existing-job"},
			Spec:     JobData{Schedule: "*/5 * * * *", Webhook: WebhookSpec{URL: "http://old.com"}},
		},
	}}

	plan := ComputePlan(context.Background(), specs, lookup)

	if len(plan.Updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(plan.Updates))
	}
	if _, ok := plan.Updates[0].Changes["schedule"]; !ok {
		t.Error("expected schedule change")
	}
	if _, ok := plan.Updates[0].Changes["webhook.url"]; !ok {
		t.Error("expected webhook.url change")
	}
}

func TestComputePlan_Unchanged(t *testing.T) {
	specs := []JobSpec{
		{
			Metadata: Metadata{Name: "same-job"},
			Spec:     JobData{Schedule: "* * * * *", Webhook: WebhookSpec{URL: "http://x.com", Method: "GET"}},
		},
	}

	lookup := &mockJobLookup{jobs: map[string]*JobSpec{
		"same-job": {
			Metadata: Metadata{Name: "same-job"},
			Spec:     JobData{Schedule: "* * * * *", Webhook: WebhookSpec{URL: "http://x.com", Method: "GET"}},
		},
	}}

	plan := ComputePlan(context.Background(), specs, lookup)

	if len(plan.Unchanged) != 1 {
		t.Fatalf("expected 1 unchanged, got %d", len(plan.Unchanged))
	}
}

func TestComputePlan_ValidationError(t *testing.T) {
	specs := []JobSpec{
		{Metadata: Metadata{Name: ""}, Spec: JobData{}},
	}

	plan := ComputePlan(context.Background(), specs, &mockJobLookup{})
	if len(plan.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(plan.Errors))
	}
}

func TestFormatPlan(t *testing.T) {
	plan := &ApplyPlan{
		Creates:   []ApplyResult{{Name: "new-job", Action: ActionCreate, Changes: map[string]Change{"schedule": {To: "* * * * *"}}}},
		Updates:   []ApplyResult{{Name: "old-job", Action: ActionUpdate, Changes: map[string]Change{"schedule": {From: "0 * * * *", To: "*/5 * * * *"}}}},
		Unchanged: []string{"stable-job"},
		Summary:   "1 to create, 1 to update, 1 unchanged",
	}

	output := FormatPlan(plan)
	if output == "" {
		t.Error("expected non-empty output")
	}
	if !contains(output, "+ new-job") {
		t.Error("expected create marker")
	}
	if !contains(output, "~ old-job") {
		t.Error("expected update marker")
	}
	if !contains(output, "= stable-job") {
		t.Error("expected unchanged marker")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
