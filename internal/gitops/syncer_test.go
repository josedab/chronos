package gitops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled {
		t.Error("expected disabled by default")
	}
	if cfg.Branch != "main" {
		t.Errorf("expected branch 'main', got %s", cfg.Branch)
	}
	if cfg.Path != "jobs" {
		t.Errorf("expected path 'jobs', got %s", cfg.Path)
	}
}

func TestValidateSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *JobSpec
		wantErr bool
	}{
		{
			name: "valid spec",
			spec: &JobSpec{
				APIVersion: "chronos.io/v1",
				Kind:       "ChronosJob",
				Metadata:   Metadata{Name: "test-job"},
				Spec: JobData{
					Schedule: "* * * * *",
					Webhook:  WebhookSpec{URL: "https://example.com"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing apiVersion",
			spec: &JobSpec{
				Kind:     "ChronosJob",
				Metadata: Metadata{Name: "test-job"},
				Spec: JobData{
					Schedule: "* * * * *",
					Webhook:  WebhookSpec{URL: "https://example.com"},
				},
			},
			wantErr: true,
		},
		{
			name: "wrong kind",
			spec: &JobSpec{
				APIVersion: "chronos.io/v1",
				Kind:       "Job",
				Metadata:   Metadata{Name: "test-job"},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			spec: &JobSpec{
				APIVersion: "chronos.io/v1",
				Kind:       "ChronosJob",
				Metadata:   Metadata{},
			},
			wantErr: true,
		},
		{
			name: "missing schedule",
			spec: &JobSpec{
				APIVersion: "chronos.io/v1",
				Kind:       "ChronosJob",
				Metadata:   Metadata{Name: "test-job"},
				Spec: JobData{
					Webhook: WebhookSpec{URL: "https://example.com"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing webhook URL",
			spec: &JobSpec{
				APIVersion: "chronos.io/v1",
				Kind:       "ChronosJob",
				Metadata:   Metadata{Name: "test-job"},
				Spec: JobData{
					Schedule: "* * * * *",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseJobName(t *testing.T) {
	tests := []struct {
		input     string
		namespace string
		name      string
	}{
		{"my-job", "", "my-job"},
		{"default/my-job", "default", "my-job"},
		{"production/api-sync", "production", "api-sync"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			parts := parseJobName(tt.input)
			if parts.namespace != tt.namespace {
				t.Errorf("expected namespace %q, got %q", tt.namespace, parts.namespace)
			}
			if parts.name != tt.name {
				t.Errorf("expected name %q, got %q", tt.name, parts.name)
			}
		})
	}
}

func TestHashContent(t *testing.T) {
	hash1 := hashContent([]byte("hello world"))
	hash2 := hashContent([]byte("hello world"))
	hash3 := hashContent([]byte("different content"))

	if hash1 != hash2 {
		t.Error("same content should produce same hash")
	}
	if hash1 == hash3 {
		t.Error("different content should produce different hash")
	}
	if len(hash1) != 64 {
		t.Errorf("expected 64 char hex hash, got %d", len(hash1))
	}
}

// mockJobHandler is a test handler.
type mockJobHandler struct {
	jobs    map[string]*JobSpec
	created []string
	updated []string
	deleted []string
}

func newMockHandler() *mockJobHandler {
	return &mockJobHandler{
		jobs:    make(map[string]*JobSpec),
		created: []string{},
		updated: []string{},
		deleted: []string{},
	}
}

func (h *mockJobHandler) CreateJob(ctx context.Context, spec *JobSpec) error {
	name := spec.Metadata.Name
	if spec.Metadata.Namespace != "" {
		name = spec.Metadata.Namespace + "/" + name
	}
	h.jobs[name] = spec
	h.created = append(h.created, name)
	return nil
}

func (h *mockJobHandler) UpdateJob(ctx context.Context, spec *JobSpec) error {
	name := spec.Metadata.Name
	if spec.Metadata.Namespace != "" {
		name = spec.Metadata.Namespace + "/" + name
	}
	h.jobs[name] = spec
	h.updated = append(h.updated, name)
	return nil
}

func (h *mockJobHandler) DeleteJob(ctx context.Context, name, namespace string) error {
	key := name
	if namespace != "" {
		key = namespace + "/" + name
	}
	delete(h.jobs, key)
	h.deleted = append(h.deleted, key)
	return nil
}

func (h *mockJobHandler) GetJob(ctx context.Context, name, namespace string) (*JobSpec, error) {
	key := name
	if namespace != "" {
		key = namespace + "/" + name
	}
	if spec, ok := h.jobs[key]; ok {
		return spec, nil
	}
	return nil, ErrRepoNotFound
}

func (h *mockJobHandler) ListJobs(ctx context.Context, namespace string) ([]*JobSpec, error) {
	var specs []*JobSpec
	for _, spec := range h.jobs {
		if namespace == "" || spec.Metadata.Namespace == namespace {
			specs = append(specs, spec)
		}
	}
	return specs, nil
}

func TestSyncer(t *testing.T) {
	// Create temp directory with job files
	tmpDir, err := os.MkdirTemp("", "gitops-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a job file
	jobYAML := `apiVersion: chronos.io/v1
kind: ChronosJob
metadata:
  name: test-job
  namespace: default
spec:
  schedule: "*/5 * * * *"
  webhook:
    url: https://example.com/webhook
    method: POST
`
	jobPath := filepath.Join(tmpDir, "test-job.yaml")
	if err := os.WriteFile(jobPath, []byte(jobYAML), 0644); err != nil {
		t.Fatalf("failed to write job file: %v", err)
	}

	handler := newMockHandler()
	syncer := NewSyncer(Config{
		Enabled:   true,
		Path:      tmpDir,
		AutoApply: true,
	}, handler)

	// Run sync
	result, err := syncer.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if len(result.Created) != 1 {
		t.Errorf("expected 1 created, got %d", len(result.Created))
	}
	if len(handler.created) != 1 {
		t.Errorf("expected 1 job created in handler, got %d", len(handler.created))
	}

	// Sync again - should be unchanged
	result, err = syncer.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if len(result.Unchanged) != 1 {
		t.Errorf("expected 1 unchanged, got %d", len(result.Unchanged))
	}
}

func TestDryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitops-dryrun")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jobYAML := `apiVersion: chronos.io/v1
kind: ChronosJob
metadata:
  name: dry-run-job
spec:
  schedule: "0 * * * *"
  webhook:
    url: https://example.com
`
	if err := os.WriteFile(filepath.Join(tmpDir, "job.yaml"), []byte(jobYAML), 0644); err != nil {
		t.Fatalf("failed to write job: %v", err)
	}

	handler := newMockHandler()
	syncer := NewSyncer(Config{
		Path:      tmpDir,
		AutoApply: true, // This should be ignored in dry run
	}, handler)

	result, err := syncer.DryRun(context.Background())
	if err != nil {
		t.Fatalf("DryRun failed: %v", err)
	}

	// Dry run shouldn't apply changes
	if len(handler.created) != 0 {
		t.Errorf("dry run should not create jobs, got %d", len(handler.created))
	}

	// But should report pending changes
	if len(result.Created) != 1 {
		t.Errorf("expected 1 pending creation, got %d", len(result.Created))
	}
}
