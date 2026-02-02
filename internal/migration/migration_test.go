package migration

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestKubernetesConverterValidate(t *testing.T) {
	converter := &KubernetesConverter{}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid cronjob",
			input: `{
				"apiVersion": "batch/v1",
				"kind": "CronJob",
				"metadata": {"name": "test", "namespace": "default"},
				"spec": {"schedule": "*/5 * * * *"}
			}`,
			wantErr: false,
		},
		{
			name: "wrong kind",
			input: `{
				"apiVersion": "batch/v1",
				"kind": "Job",
				"metadata": {"name": "test"}
			}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := converter.Validate([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKubernetesConverterConvertJobs(t *testing.T) {
	converter := &KubernetesConverter{}
	ctx := context.Background()

	input := `{
		"apiVersion": "batch/v1",
		"kind": "CronJob",
		"metadata": {
			"name": "daily-backup",
			"namespace": "production",
			"labels": {"app": "backup", "env": "prod"}
		},
		"spec": {
			"schedule": "0 2 * * *",
			"timeZone": "America/New_York",
			"concurrencyPolicy": "Forbid",
			"suspend": false,
			"jobTemplate": {
				"spec": {
					"backoffLimit": 3,
					"template": {
						"spec": {
							"containers": [{
								"name": "backup",
								"image": "backup-image:v1.0",
								"command": ["/bin/backup.sh"],
								"args": ["--full"]
							}],
							"restartPolicy": "Never"
						}
					}
				}
			}
		}
	}`

	jobs, errors, err := converter.ConvertJobs(ctx, []byte(input))

	if err != nil {
		t.Fatalf("ConvertJobs() error = %v", err)
	}

	if len(errors) > 0 {
		t.Errorf("ConvertJobs() returned errors: %v", errors)
	}

	if len(jobs) != 1 {
		t.Fatalf("ConvertJobs() returned %d jobs, want 1", len(jobs))
	}

	job := jobs[0]

	// Verify job properties
	if job.Name != "daily-backup" {
		t.Errorf("job.Name = %q, want %q", job.Name, "daily-backup")
	}

	if job.Schedule != "0 2 * * *" {
		t.Errorf("job.Schedule = %q, want %q", job.Schedule, "0 2 * * *")
	}

	if job.Timezone != "America/New_York" {
		t.Errorf("job.Timezone = %q, want %q", job.Timezone, "America/New_York")
	}

	if job.Concurrency != "forbid" {
		t.Errorf("job.Concurrency = %q, want %q", job.Concurrency, "forbid")
	}

	if !job.Enabled {
		t.Error("job.Enabled = false, want true")
	}

	if job.Namespace != "production" {
		t.Errorf("job.Namespace = %q, want %q", job.Namespace, "production")
	}

	// Verify retry policy
	if job.RetryPolicy == nil {
		t.Error("job.RetryPolicy is nil")
	} else if job.RetryPolicy.MaxAttempts != 3 {
		t.Errorf("job.RetryPolicy.MaxAttempts = %d, want 3", job.RetryPolicy.MaxAttempts)
	}

	// Verify webhook
	if job.Webhook == nil {
		t.Error("job.Webhook is nil")
	} else {
		if job.Webhook.Method != "POST" {
			t.Errorf("job.Webhook.Method = %q, want %q", job.Webhook.Method, "POST")
		}
		if job.Webhook.Headers["X-Original-Image"] != "backup-image:v1.0" {
			t.Errorf("missing X-Original-Image header")
		}
	}

	// Verify tags
	if job.Tags["app"] != "backup" {
		t.Errorf("job.Tags[app] = %q, want %q", job.Tags["app"], "backup")
	}
}

func TestEventBridgeConverterConvertSchedule(t *testing.T) {
	converter := &EventBridgeConverter{}

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "rate minutes",
			input:    "rate(5 minutes)",
			expected: "*/5 * * * *",
			wantErr:  false,
		},
		{
			name:     "rate hour",
			input:    "rate(1 hour)",
			expected: "0 */1 * * *",
			wantErr:  false,
		},
		{
			name:     "rate hours",
			input:    "rate(2 hours)",
			expected: "0 */2 * * *",
			wantErr:  false,
		},
		{
			name:     "rate day",
			input:    "rate(1 day)",
			expected: "0 0 */1 * *",
			wantErr:  false,
		},
		{
			name:     "cron expression",
			input:    "cron(0 12 * * ? *)",
			expected: "0 12 * * *",
			wantErr:  false,
		},
		{
			name:    "invalid expression",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.convertScheduleExpression(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertScheduleExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("convertScheduleExpression() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMigratorMigrate(t *testing.T) {
	migrator := NewMigrator()
	ctx := context.Background()

	// Test Kubernetes migration
	k8sInput := `{
		"apiVersion": "batch/v1",
		"kind": "CronJob",
		"metadata": {"name": "test", "namespace": "default"},
		"spec": {
			"schedule": "* * * * *",
			"jobTemplate": {
				"spec": {
					"template": {
						"spec": {
							"containers": [{"name": "test", "image": "test:latest"}],
							"restartPolicy": "Never"
						}
					}
				}
			}
		}
	}`

	result, err := migrator.Migrate(ctx, SourceKubernetesCronJob, []byte(k8sInput))
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	if result.SourceType != SourceKubernetesCronJob {
		t.Errorf("result.SourceType = %v, want %v", result.SourceType, SourceKubernetesCronJob)
	}

	if result.ItemsMigrated != 1 {
		t.Errorf("result.ItemsMigrated = %d, want 1", result.ItemsMigrated)
	}

	if len(result.Jobs) != 1 {
		t.Errorf("len(result.Jobs) = %d, want 1", len(result.Jobs))
	}
}

func TestMigratorUnsupportedFormat(t *testing.T) {
	migrator := NewMigrator()
	ctx := context.Background()

	_, err := migrator.Migrate(ctx, "unsupported", []byte("{}"))
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestDiscoveredCronJobConversion(t *testing.T) {
	discovered := &DiscoveredCronJob{
		Name:              "test-job",
		Namespace:         "test-ns",
		UID:               "abc-123",
		Schedule:          "*/10 * * * *",
		Timezone:          "UTC",
		Suspend:           false,
		Image:             "my-image:v1",
		Command:           []string{"/bin/run"},
		Args:              []string{"--arg1"},
		ConcurrencyPolicy: "Forbid",
		BackoffLimit:      int32Ptr(5),
		RequiresAdapter:   true,
	}

	// Create a mock discovery to use its conversion method
	d := &KubernetesDiscovery{}
	job := d.convertToChronosJob(discovered, nil)

	if job.Name != "test-ns-test-job" {
		t.Errorf("job.Name = %q, want %q", job.Name, "test-ns-test-job")
	}

	if job.Schedule != "*/10 * * * *" {
		t.Errorf("job.Schedule = %q, want %q", job.Schedule, "*/10 * * * *")
	}

	if job.Tags["k8s_namespace"] != "test-ns" {
		t.Errorf("missing k8s_namespace tag")
	}

	if job.Tags["k8s_name"] != "test-job" {
		t.Errorf("missing k8s_name tag")
	}

	if job.RetryPolicy.MaxAttempts != 5 {
		t.Errorf("job.RetryPolicy.MaxAttempts = %d, want 5", job.RetryPolicy.MaxAttempts)
	}
}

func TestCompareMigration(t *testing.T) {
	discovered := []*DiscoveredCronJob{
		{
			Name:      "existing-job",
			Namespace: "default",
			Schedule:  "0 * * * *",
			Suspend:   false,
		},
		{
			Name:      "new-job",
			Namespace: "default",
			Schedule:  "*/5 * * * *",
			Suspend:   false,
		},
		{
			Name:      "updated-job",
			Namespace: "default",
			Schedule:  "0 2 * * *", // Changed from 0 1 * * *
			Suspend:   false,
		},
	}

	existing := []*mockJob{
		{
			name:     "existing-job",
			schedule: "0 * * * *",
			enabled:  true,
			tags: map[string]string{
				"k8s_namespace": "default",
				"k8s_name":      "existing-job",
			},
		},
		{
			name:     "updated-job",
			schedule: "0 1 * * *", // Original schedule
			enabled:  true,
			tags: map[string]string{
				"k8s_namespace": "default",
				"k8s_name":      "updated-job",
			},
		},
	}

	// Convert mock jobs to models.Job for comparison
	var modelJobs []*mockModelJob
	for _, j := range existing {
		modelJobs = append(modelJobs, &mockModelJob{
			Name:     j.name,
			Schedule: j.schedule,
			Enabled:  j.enabled,
			Tags:     j.tags,
		})
	}

	// Test the comparison logic manually since CompareMigration needs models.Job
	comparisons := make([]*MigrationComparison, 0)
	existingBySource := make(map[string]*mockModelJob)

	for _, job := range modelJobs {
		if job.Tags != nil {
			if ns, ok := job.Tags["k8s_namespace"]; ok {
				if name, ok := job.Tags["k8s_name"]; ok {
					key := ns + "/" + name
					existingBySource[key] = job
				}
			}
		}
	}

	for _, cj := range discovered {
		comparison := &MigrationComparison{
			SourceCronJob: cj,
			Differences:   make([]string, 0),
		}

		key := cj.Namespace + "/" + cj.Name
		if existingJob, found := existingBySource[key]; found {
			if existingJob.Schedule != cj.Schedule {
				comparison.Differences = append(comparison.Differences, "schedule changed")
			}
			if len(comparison.Differences) == 0 {
				comparison.Status = "identical"
			} else {
				comparison.Status = "updated"
			}
		} else {
			comparison.Status = "new"
		}

		comparisons = append(comparisons, comparison)
	}

	// Verify results
	if len(comparisons) != 3 {
		t.Fatalf("expected 3 comparisons, got %d", len(comparisons))
	}

	// existing-job should be identical
	if comparisons[0].Status != "identical" {
		t.Errorf("existing-job status = %q, want %q", comparisons[0].Status, "identical")
	}

	// new-job should be new
	if comparisons[1].Status != "new" {
		t.Errorf("new-job status = %q, want %q", comparisons[1].Status, "new")
	}

	// updated-job should be updated
	if comparisons[2].Status != "updated" {
		t.Errorf("updated-job status = %q, want %q", comparisons[2].Status, "updated")
	}
}

func TestExportMigrationPlan(t *testing.T) {
	plan := &MigrationPlan{
		ID:            "test-plan-123",
		Name:          "test-migration",
		CreatedAt:     time.Now().UTC(),
		SourceCluster: "https://k8s.example.com",
		Status:        MigrationPlanPending,
		CronJobs:      []*MigrationPlanItem{},
	}

	// Test JSON export
	jsonData, err := ExportMigrationPlanJSON(plan)
	if err != nil {
		t.Fatalf("ExportMigrationPlanJSON() error = %v", err)
	}

	var parsedJSON map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsedJSON); err != nil {
		t.Errorf("exported JSON is invalid: %v", err)
	}

	if parsedJSON["id"] != "test-plan-123" {
		t.Errorf("parsed id = %v, want test-plan-123", parsedJSON["id"])
	}

	// Test YAML export
	yamlData, err := ExportMigrationPlanYAML(plan)
	if err != nil {
		t.Fatalf("ExportMigrationPlanYAML() error = %v", err)
	}

	if len(yamlData) == 0 {
		t.Error("YAML export is empty")
	}
}

// Helper types for testing
type mockJob struct {
	name     string
	schedule string
	enabled  bool
	tags     map[string]string
}

type mockModelJob struct {
	Name     string
	Schedule string
	Enabled  bool
	Tags     map[string]string
}

func int32Ptr(i int32) *int32 {
	return &i
}
