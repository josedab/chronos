// Package gitops provides declarative job-as-code apply functionality.
package gitops

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ApplyAction describes what will happen to a job during apply.
type ApplyAction string

const (
	ActionCreate    ApplyAction = "create"
	ActionUpdate    ApplyAction = "update"
	ActionUnchanged ApplyAction = "unchanged"
	ActionDelete    ApplyAction = "delete"
	ActionError     ApplyAction = "error"
)

// ApplyResult describes the outcome of applying a single job spec.
type ApplyResult struct {
	Name     string            `json:"name"`
	Action   ApplyAction       `json:"action"`
	Changes  map[string]Change `json:"changes,omitempty"`
	Error    string            `json:"error,omitempty"`
}

// Change describes a field-level diff.
type Change struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ApplyPlan is the full plan for an apply operation.
type ApplyPlan struct {
	Creates   []ApplyResult `json:"creates"`
	Updates   []ApplyResult `json:"updates"`
	Unchanged []string      `json:"unchanged"`
	Errors    []ApplyResult `json:"errors"`
	Summary   string        `json:"summary"`
}

// JobLookup provides access to existing jobs for diff computation.
type JobLookup interface {
	GetJobByName(name string) (*JobSpec, bool)
}

// ParseJobSpecs parses job specs from a YAML file or multi-doc YAML.
func ParseJobSpecs(data []byte) ([]JobSpec, error) {
	var specs []JobSpec
	decoder := yaml.NewDecoder(bytes.NewReader(data))

	for {
		var spec JobSpec
		err := decoder.Decode(&spec)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing YAML: %w", err)
		}
		if spec.Kind == "" {
			spec.Kind = "Job"
		}
		if spec.APIVersion == "" {
			spec.APIVersion = "chronos/v1"
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// ParseJobSpecsFromFile reads and parses job specs from a file or directory.
func ParseJobSpecsFromFile(path string) ([]JobSpec, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("accessing path: %w", err)
	}

	if !info.IsDir() {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading file: %w", err)
		}
		return ParseJobSpecs(data)
	}

	// Directory: read all .yaml and .yml files
	var allSpecs []JobSpec
	err = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("reading %s: %w", p, err)
		}
		specs, err := ParseJobSpecs(data)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", p, err)
		}
		allSpecs = append(allSpecs, specs...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return allSpecs, nil
}

// ValidateJobSpec validates a single job spec.
func ValidateJobSpec(spec *JobSpec) []string {
	var errors []string

	if spec.Metadata.Name == "" {
		errors = append(errors, "metadata.name is required")
	}
	if spec.Spec.Schedule == "" {
		errors = append(errors, "spec.schedule is required")
	}
	if spec.Spec.Webhook.URL == "" {
		errors = append(errors, "spec.webhook.url is required")
	}
	if spec.Kind != "" && spec.Kind != "Job" {
		errors = append(errors, fmt.Sprintf("unsupported kind: %s (expected Job)", spec.Kind))
	}

	return errors
}

// ComputePlan computes the apply plan by diffing specs against existing jobs.
func ComputePlan(_ context.Context, specs []JobSpec, lookup JobLookup) *ApplyPlan {
	plan := &ApplyPlan{}

	for _, spec := range specs {
		errs := ValidateJobSpec(&spec)
		if len(errs) > 0 {
			plan.Errors = append(plan.Errors, ApplyResult{
				Name:   spec.Metadata.Name,
				Action: ActionError,
				Error:  strings.Join(errs, "; "),
			})
			continue
		}

		existing, found := lookup.GetJobByName(spec.Metadata.Name)
		if !found {
			plan.Creates = append(plan.Creates, ApplyResult{
				Name:   spec.Metadata.Name,
				Action: ActionCreate,
				Changes: map[string]Change{
					"schedule": {To: spec.Spec.Schedule},
					"webhook":  {To: spec.Spec.Webhook.URL},
				},
			})
			continue
		}

		// Compute diff
		changes := diffJobSpec(existing, &spec)
		if len(changes) == 0 {
			plan.Unchanged = append(plan.Unchanged, spec.Metadata.Name)
		} else {
			plan.Updates = append(plan.Updates, ApplyResult{
				Name:    spec.Metadata.Name,
				Action:  ActionUpdate,
				Changes: changes,
			})
		}
	}

	// Build summary
	var parts []string
	if len(plan.Creates) > 0 {
		parts = append(parts, fmt.Sprintf("%d to create", len(plan.Creates)))
	}
	if len(plan.Updates) > 0 {
		parts = append(parts, fmt.Sprintf("%d to update", len(plan.Updates)))
	}
	if len(plan.Unchanged) > 0 {
		parts = append(parts, fmt.Sprintf("%d unchanged", len(plan.Unchanged)))
	}
	if len(plan.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("%d errors", len(plan.Errors)))
	}
	plan.Summary = strings.Join(parts, ", ")

	return plan
}

func diffJobSpec(old, new *JobSpec) map[string]Change {
	changes := make(map[string]Change)

	if old.Spec.Schedule != new.Spec.Schedule {
		changes["schedule"] = Change{From: old.Spec.Schedule, To: new.Spec.Schedule}
	}
	if old.Spec.Webhook.URL != new.Spec.Webhook.URL {
		changes["webhook.url"] = Change{From: old.Spec.Webhook.URL, To: new.Spec.Webhook.URL}
	}
	if old.Spec.Webhook.Method != new.Spec.Webhook.Method {
		changes["webhook.method"] = Change{From: old.Spec.Webhook.Method, To: new.Spec.Webhook.Method}
	}
	if old.Spec.Timezone != new.Spec.Timezone {
		changes["timezone"] = Change{From: old.Spec.Timezone, To: new.Spec.Timezone}
	}
	if old.Spec.Timeout != new.Spec.Timeout {
		changes["timeout"] = Change{From: old.Spec.Timeout, To: new.Spec.Timeout}
	}
	if old.Spec.Description != new.Spec.Description {
		changes["description"] = Change{From: old.Spec.Description, To: new.Spec.Description}
	}

	return changes
}

// FormatPlan renders the apply plan as human-readable text.
func FormatPlan(plan *ApplyPlan) string {
	var b strings.Builder

	for _, c := range plan.Creates {
		b.WriteString(fmt.Sprintf("  + %s (create)\n", c.Name))
		for field, ch := range c.Changes {
			b.WriteString(fmt.Sprintf("      %s: %s\n", field, ch.To))
		}
	}
	for _, u := range plan.Updates {
		b.WriteString(fmt.Sprintf("  ~ %s (update)\n", u.Name))
		for field, ch := range u.Changes {
			b.WriteString(fmt.Sprintf("      %s: %q → %q\n", field, ch.From, ch.To))
		}
	}
	for _, name := range plan.Unchanged {
		b.WriteString(fmt.Sprintf("  = %s (unchanged)\n", name))
	}
	for _, e := range plan.Errors {
		b.WriteString(fmt.Sprintf("  ! %s (error: %s)\n", e.Name, e.Error))
	}

	b.WriteString(fmt.Sprintf("\nPlan: %s\n", plan.Summary))
	return b.String()
}
