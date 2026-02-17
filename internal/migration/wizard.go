// Package migration provides an interactive migration wizard for guided migrations.
package migration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/google/uuid"
)

// WizardStep represents a step in the migration wizard.
type WizardStep string

const (
	StepSelectSource WizardStep = "select_source"
	StepProvideInput WizardStep = "provide_input"
	StepValidate     WizardStep = "validate"
	StepPreview      WizardStep = "preview"
	StepResolve      WizardStep = "resolve_conflicts"
	StepApply        WizardStep = "apply"
	StepComplete     WizardStep = "complete"
)

// WizardSession tracks a multi-step migration session.
type WizardSession struct {
	ID          string                `json:"id"`
	Step        WizardStep            `json:"step"`
	SourceType  SourceType            `json:"source_type,omitempty"`
	InputData   []byte                `json:"-"`
	Result      *MigrationResult      `json:"result,omitempty"`
	DiffReport  *DiffReport           `json:"diff_report,omitempty"`
	Conflicts   []MigrationConflict   `json:"conflicts,omitempty"`
	Resolutions map[string]Resolution `json:"resolutions,omitempty"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// MigrationConflict represents a conflict during migration.
type MigrationConflict struct {
	ID          string `json:"id"`
	JobName     string `json:"job_name"`
	Field       string `json:"field"`
	SourceValue string `json:"source_value"`
	TargetValue string `json:"target_value,omitempty"`
	Type        string `json:"type"` // "name_collision", "schedule_unsupported", "webhook_invalid"
	Suggestion  string `json:"suggestion,omitempty"`
}

// Resolution represents how to resolve a conflict.
type Resolution string

const (
	ResolutionSkip      Resolution = "skip"
	ResolutionOverwrite Resolution = "overwrite"
	ResolutionRename    Resolution = "rename"
	ResolutionUseSource Resolution = "use_source"
)

// DiffReport shows what will change during migration.
type DiffReport struct {
	JobsToCreate  []DiffEntry `json:"jobs_to_create"`
	JobsToUpdate  []DiffEntry `json:"jobs_to_update"`
	JobsUnchanged []string    `json:"jobs_unchanged"`
	JobsSkipped   []DiffEntry `json:"jobs_skipped"`
	Summary       string      `json:"summary"`
}

// DiffEntry represents a single change in the diff report.
type DiffEntry struct {
	Name    string            `json:"name"`
	Action  string            `json:"action"` // "create", "update", "skip"
	Changes map[string]Change `json:"changes,omitempty"`
	Reason  string            `json:"reason,omitempty"`
}

// Change represents a field-level change.
type Change struct {
	From string `json:"from,omitempty"`
	To   string `json:"to"`
}

// MigrationWizard orchestrates guided migrations.
type MigrationWizard struct {
	migrator       *Migrator
	sessions       map[string]*WizardSession
	existingJobsFn func() ([]*models.Job, error) // lookup existing jobs
}

// NewMigrationWizard creates a new wizard.
func NewMigrationWizard(existingJobsFn func() ([]*models.Job, error)) *MigrationWizard {
	return &MigrationWizard{
		migrator:       NewMigrator(),
		sessions:       make(map[string]*WizardSession),
		existingJobsFn: existingJobsFn,
	}
}

// StartSession begins a new migration wizard session.
func (w *MigrationWizard) StartSession(sourceType SourceType) (*WizardSession, error) {
	if !w.migrator.SupportsSource(sourceType) {
		return nil, fmt.Errorf("unsupported source type: %s", sourceType)
	}

	session := &WizardSession{
		ID:          uuid.New().String(),
		Step:        StepProvideInput,
		SourceType:  sourceType,
		Resolutions: make(map[string]Resolution),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	w.sessions[session.ID] = session
	return session, nil
}

// GetSession returns a wizard session by ID.
func (w *MigrationWizard) GetSession(sessionID string) (*WizardSession, error) {
	session, ok := w.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// SubmitInput processes input data for the migration.
func (w *MigrationWizard) SubmitInput(ctx context.Context, sessionID string, data []byte) (*WizardSession, error) {
	session, err := w.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	if session.Step != StepProvideInput {
		return nil, fmt.Errorf("session is at step %s, expected %s", session.Step, StepProvideInput)
	}

	session.InputData = data

	// Validate
	converter, ok := w.migrator.converters[session.SourceType]
	if !ok {
		return nil, fmt.Errorf("no converter for source: %s", session.SourceType)
	}

	if err := converter.Validate(data); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Convert
	result, err := w.migrator.Migrate(ctx, session.SourceType, data)
	if err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	session.Result = result

	// Generate diff report
	diff, conflicts, err := w.generateDiff(result)
	if err != nil {
		return nil, fmt.Errorf("generating diff: %w", err)
	}

	session.DiffReport = diff
	session.Conflicts = conflicts

	if len(conflicts) > 0 {
		session.Step = StepResolve
	} else {
		session.Step = StepPreview
	}
	session.UpdatedAt = time.Now()

	return session, nil
}

// ResolveConflict resolves a specific conflict.
func (w *MigrationWizard) ResolveConflict(sessionID, conflictID string, resolution Resolution) (*WizardSession, error) {
	session, err := w.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	if session.Step != StepResolve {
		return nil, fmt.Errorf("session is not at resolve step")
	}

	// Validate conflict exists
	found := false
	for _, c := range session.Conflicts {
		if c.ID == conflictID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("conflict not found: %s", conflictID)
	}

	session.Resolutions[conflictID] = resolution
	session.UpdatedAt = time.Now()

	// Check if all conflicts resolved
	if len(session.Resolutions) >= len(session.Conflicts) {
		session.Step = StepPreview
	}

	return session, nil
}

// GetApplyPlan returns the final list of jobs that will be created/updated.
func (w *MigrationWizard) GetApplyPlan(sessionID string) ([]*models.Job, error) {
	session, err := w.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	if session.Step != StepPreview && session.Step != StepApply {
		return nil, fmt.Errorf("session is not ready for apply (step: %s)", session.Step)
	}

	if session.Result == nil {
		return nil, fmt.Errorf("no migration result available")
	}

	// Filter jobs based on conflict resolutions
	var jobs []*models.Job
	for _, job := range session.Result.Jobs {
		conflictID := w.findConflictForJob(session, job.Name)
		if conflictID != "" {
			resolution := session.Resolutions[conflictID]
			switch resolution {
			case ResolutionSkip:
				continue
			case ResolutionRename:
				job.Name = job.Name + "-migrated"
			}
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// generateDiff compares migration results against existing jobs.
func (w *MigrationWizard) generateDiff(result *MigrationResult) (*DiffReport, []MigrationConflict, error) {
	report := &DiffReport{}
	var conflicts []MigrationConflict

	existingJobs := make(map[string]*models.Job)
	if w.existingJobsFn != nil {
		jobs, err := w.existingJobsFn()
		if err != nil {
			return nil, nil, fmt.Errorf("fetching existing jobs: %w", err)
		}
		for _, j := range jobs {
			existingJobs[j.Name] = j
		}
	}

	for _, job := range result.Jobs {
		existing, exists := existingJobs[job.Name]
		if !exists {
			report.JobsToCreate = append(report.JobsToCreate, DiffEntry{
				Name:   job.Name,
				Action: "create",
				Changes: map[string]Change{
					"schedule": {To: job.Schedule},
					"webhook":  {To: job.Webhook.URL},
				},
			})
		} else {
			changes := make(map[string]Change)
			if existing.Schedule != job.Schedule {
				changes["schedule"] = Change{From: existing.Schedule, To: job.Schedule}
			}
			if existing.Webhook != nil && job.Webhook != nil && existing.Webhook.URL != job.Webhook.URL {
				changes["webhook_url"] = Change{From: existing.Webhook.URL, To: job.Webhook.URL}
			}

			if len(changes) > 0 {
				report.JobsToUpdate = append(report.JobsToUpdate, DiffEntry{
					Name:    job.Name,
					Action:  "update",
					Changes: changes,
				})

				conflicts = append(conflicts, MigrationConflict{
					ID:          uuid.New().String(),
					JobName:     job.Name,
					Field:       "name",
					SourceValue: job.Name,
					TargetValue: existing.Name,
					Type:        "name_collision",
					Suggestion:  "Rename to " + job.Name + "-migrated or overwrite existing",
				})
			} else {
				report.JobsUnchanged = append(report.JobsUnchanged, job.Name)
			}
		}
	}

	// Generate summary
	var parts []string
	if len(report.JobsToCreate) > 0 {
		parts = append(parts, fmt.Sprintf("%d new jobs", len(report.JobsToCreate)))
	}
	if len(report.JobsToUpdate) > 0 {
		parts = append(parts, fmt.Sprintf("%d updates", len(report.JobsToUpdate)))
	}
	if len(report.JobsUnchanged) > 0 {
		parts = append(parts, fmt.Sprintf("%d unchanged", len(report.JobsUnchanged)))
	}
	if len(report.JobsSkipped) > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", len(report.JobsSkipped)))
	}
	report.Summary = strings.Join(parts, ", ")

	return report, conflicts, nil
}

// findConflictForJob finds the conflict ID for a given job name.
func (w *MigrationWizard) findConflictForJob(session *WizardSession, jobName string) string {
	for _, c := range session.Conflicts {
		if c.JobName == jobName {
			return c.ID
		}
	}
	return ""
}

// SupportsSource checks if the migrator supports the given source type.
func (m *Migrator) SupportsSource(source SourceType) bool {
	_, ok := m.converters[source]
	return ok
}

// ListSupportedSources returns all supported source types.
func (m *Migrator) ListSupportedSources() []SourceType {
	sources := make([]SourceType, 0, len(m.converters))
	for s := range m.converters {
		sources = append(sources, s)
	}
	return sources
}
