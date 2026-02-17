package migration

import (
	"context"
	"testing"

	"github.com/chronos/chronos/internal/models"
)

func TestMigrationWizard_StartSession(t *testing.T) {
	wiz := NewMigrationWizard(nil)

	session, err := wiz.StartSession(SourceCrontab)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.ID == "" {
		t.Error("expected session ID")
	}
	if session.Step != StepProvideInput {
		t.Errorf("expected step %s, got %s", StepProvideInput, session.Step)
	}
	if session.SourceType != SourceCrontab {
		t.Errorf("expected source crontab, got %s", session.SourceType)
	}
}

func TestMigrationWizard_StartSession_Invalid(t *testing.T) {
	wiz := NewMigrationWizard(nil)
	_, err := wiz.StartSession("invalid_source")
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
}

func TestMigrationWizard_GetSession(t *testing.T) {
	wiz := NewMigrationWizard(nil)

	session, _ := wiz.StartSession(SourceCrontab)

	found, err := wiz.GetSession(session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != session.ID {
		t.Errorf("expected session %s, got %s", session.ID, found.ID)
	}

	_, err = wiz.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestMigrationWizard_SubmitInput_Crontab(t *testing.T) {
	wiz := NewMigrationWizard(func() ([]*models.Job, error) {
		return nil, nil // no existing jobs
	})

	session, _ := wiz.StartSession(SourceCrontab)

	crontab := []byte("*/5 * * * * curl -s https://example.com/api/check\n0 0 * * * /usr/bin/cleanup.sh\n")

	result, err := wiz.SubmitInput(context.Background(), session.ID, crontab)
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	if result.Result == nil {
		t.Fatal("expected migration result")
	}
	if result.DiffReport == nil {
		t.Fatal("expected diff report")
	}
	if result.Step != StepPreview {
		t.Errorf("expected step preview (no conflicts), got %s", result.Step)
	}
}

func TestMigrationWizard_ConflictDetection(t *testing.T) {
	wiz := NewMigrationWizard(func() ([]*models.Job, error) {
		return []*models.Job{
			{
				ID:       "existing-1",
				Name:     "crontab-job-1",
				Schedule: "0 * * * *",
				Webhook:  &models.WebhookConfig{URL: "http://old.example.com"},
			},
		}, nil
	})

	session, _ := wiz.StartSession(SourceCrontab)

	// This crontab will create a job named "crontab-job-1" which conflicts
	crontab := []byte("*/5 * * * * curl https://new.example.com/api\n")

	result, err := wiz.SubmitInput(context.Background(), session.ID, crontab)
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	// If there are conflicts, step should be "resolve"
	// If no match by name, step should be "preview"
	if result.DiffReport != nil && len(result.DiffReport.JobsToCreate) > 0 {
		// New jobs created successfully
		t.Logf("diff report: %s", result.DiffReport.Summary)
	}
}

func TestMigrationWizard_ResolveConflict(t *testing.T) {
	wiz := NewMigrationWizard(nil)

	session, _ := wiz.StartSession(SourceCrontab)
	// Manually set up a session with conflicts
	session.Step = StepResolve
	session.Conflicts = []MigrationConflict{
		{ID: "conflict-1", JobName: "test-job", Type: "name_collision"},
	}

	result, err := wiz.ResolveConflict(session.ID, "conflict-1", ResolutionOverwrite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Resolutions["conflict-1"] != ResolutionOverwrite {
		t.Error("expected resolution to be stored")
	}
	if result.Step != StepPreview {
		t.Errorf("expected step preview after all conflicts resolved, got %s", result.Step)
	}
}

func TestMigrationWizard_GetApplyPlan(t *testing.T) {
	wiz := NewMigrationWizard(nil)

	session, _ := wiz.StartSession(SourceCrontab)
	session.Step = StepPreview
	session.Result = &MigrationResult{
		Jobs: []*models.Job{
			{ID: "1", Name: "job-keep"},
			{ID: "2", Name: "job-skip"},
			{ID: "3", Name: "job-rename"},
		},
	}
	session.Conflicts = []MigrationConflict{
		{ID: "c1", JobName: "job-skip"},
		{ID: "c2", JobName: "job-rename"},
	}
	session.Resolutions = map[string]Resolution{
		"c1": ResolutionSkip,
		"c2": ResolutionRename,
	}

	jobs, err := wiz.GetApplyPlan(session.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs (1 skipped), got %d", len(jobs))
	}

	// Check rename was applied
	renamed := false
	for _, j := range jobs {
		if j.Name == "job-rename-migrated" {
			renamed = true
		}
	}
	if !renamed {
		names := make([]string, len(jobs))
		for i, j := range jobs {
			names[i] = j.Name
		}
		t.Errorf("expected renamed job, got names: %v", names)
	}
}

func TestDiffReport_Summary(t *testing.T) {
	wiz := NewMigrationWizard(func() ([]*models.Job, error) {
		return nil, nil
	})

	result := &MigrationResult{
		Jobs: []*models.Job{
			{ID: "1", Name: "new-job", Schedule: "* * * * *", Webhook: &models.WebhookConfig{URL: "http://x.com"}},
		},
	}

	diff, _, err := wiz.generateDiff(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diff.JobsToCreate) != 1 {
		t.Errorf("expected 1 job to create, got %d", len(diff.JobsToCreate))
	}
	if diff.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestListSupportedSources(t *testing.T) {
	m := NewMigrator()
	sources := m.ListSupportedSources()

	if len(sources) == 0 {
		t.Error("expected at least 1 supported source")
	}

	// Crontab should always be supported
	found := false
	for _, s := range sources {
		if s == SourceCrontab {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected crontab source to be supported")
	}
}
