// Package marketplace provides workflow templates for Chronos.
package marketplace

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// WorkflowRegistry manages workflow templates.
type WorkflowRegistry struct {
	templates map[string]*WorkflowTemplate
	community map[string]*WorkflowTemplate
	private   map[string]map[string]*WorkflowTemplate // org -> templates
	mu        sync.RWMutex
}

// NewWorkflowRegistry creates a new workflow registry.
func NewWorkflowRegistry() *WorkflowRegistry {
	r := &WorkflowRegistry{
		templates: make(map[string]*WorkflowTemplate),
		community: make(map[string]*WorkflowTemplate),
		private:   make(map[string]map[string]*WorkflowTemplate),
	}

	// Load built-in workflow templates
	for _, t := range BuiltInWorkflowTemplates() {
		r.templates[t.ID] = t
	}

	return r
}

// GetWorkflowTemplate retrieves a workflow template by ID.
func (r *WorkflowRegistry) GetWorkflowTemplate(id string) (*WorkflowTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if t, ok := r.templates[id]; ok {
		return t, nil
	}
	if t, ok := r.community[id]; ok {
		return t, nil
	}

	return nil, fmt.Errorf("workflow template not found: %s", id)
}

// ListWorkflowTemplates returns all available workflow templates.
func (r *WorkflowRegistry) ListWorkflowTemplates(filter WorkflowFilter) []*WorkflowTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*WorkflowTemplate

	for _, t := range r.templates {
		if r.matchesFilter(t, filter) {
			results = append(results, t)
		}
	}
	for _, t := range r.community {
		if r.matchesFilter(t, filter) {
			results = append(results, t)
		}
	}

	return results
}

func (r *WorkflowRegistry) matchesFilter(t *WorkflowTemplate, f WorkflowFilter) bool {
	if f.Category != "" && t.Category != f.Category {
		return false
	}
	if f.Author != "" && t.Author != f.Author {
		return false
	}
	if f.Verified != nil && t.Verified != *f.Verified {
		return false
	}
	if f.Featured != nil && t.Featured != *f.Featured {
		return false
	}
	if f.MinRating > 0 && t.Rating < f.MinRating {
		return false
	}
	if len(f.Tags) > 0 {
		hasTag := false
		for _, ft := range f.Tags {
			for _, tt := range t.Tags {
				if ft == tt {
					hasTag = true
					break
				}
			}
		}
		if !hasTag {
			return false
		}
	}
	return true
}

// SubmitWorkflowTemplate submits a community template for review.
func (r *WorkflowRegistry) SubmitWorkflowTemplate(template *WorkflowTemplate, authorID string) (*TemplateSubmission, error) {
	if err := r.validateWorkflowTemplate(template); err != nil {
		return nil, err
	}

	submission := &TemplateSubmission{
		ID:          uuid.New().String(),
		Template:    template,
		AuthorID:    authorID,
		Status:      SubmissionPending,
		SubmittedAt: time.Now(),
	}

	return submission, nil
}

func (r *WorkflowRegistry) validateWorkflowTemplate(t *WorkflowTemplate) error {
	if t.Name == "" {
		return errors.New("template name is required")
	}
	if t.Description == "" {
		return errors.New("template description is required")
	}
	if len(t.Steps) == 0 {
		return errors.New("template must have at least one step")
	}

	// Validate step dependencies
	stepIDs := make(map[string]bool)
	for _, s := range t.Steps {
		if s.ID == "" {
			return errors.New("all steps must have an ID")
		}
		stepIDs[s.ID] = true
	}
	for _, s := range t.Steps {
		for _, dep := range s.Dependencies {
			if !stepIDs[dep] {
				return fmt.Errorf("step %s has invalid dependency: %s", s.ID, dep)
			}
		}
	}

	return nil
}

// InstantiateWorkflow creates a workflow instance from a template.
func (r *WorkflowRegistry) InstantiateWorkflow(templateID string, vars map[string]string) (*WorkflowInstance, error) {
	template, err := r.GetWorkflowTemplate(templateID)
	if err != nil {
		return nil, err
	}

	// Validate required variables
	for _, v := range template.Variables {
		if v.Required {
			if _, ok := vars[v.Name]; !ok {
				if v.Default == nil {
					return nil, fmt.Errorf("required variable not provided: %s", v.Name)
				}
				vars[v.Name] = fmt.Sprintf("%v", v.Default)
			}
		}
	}

	// Create instance with substituted values
	instance := &WorkflowInstance{
		ID:         uuid.New().String(),
		TemplateID: templateID,
		Name:       substituteVars(template.Name, vars),
		Steps:      make([]WorkflowStepInstance, len(template.Steps)),
		Variables:  vars,
		CreatedAt:  time.Now(),
	}

	for i, step := range template.Steps {
		configJSON, _ := json.Marshal(step.Config)
		substitutedConfig := substituteVars(string(configJSON), vars)
		var config map[string]interface{}
		json.Unmarshal([]byte(substitutedConfig), &config)

		instance.Steps[i] = WorkflowStepInstance{
			ID:           step.ID,
			Name:         substituteVars(step.Name, vars),
			Type:         step.Type,
			Config:       config,
			Dependencies: step.Dependencies,
		}
	}

	// Increment download count
	r.mu.Lock()
	if t, ok := r.templates[templateID]; ok {
		t.Downloads++
	}
	if t, ok := r.community[templateID]; ok {
		t.Downloads++
	}
	r.mu.Unlock()

	return instance, nil
}

// RateWorkflowTemplate rates a workflow template.
func (r *WorkflowRegistry) RateWorkflowTemplate(templateID string, rating int) error {
	if rating < 1 || rating > 5 {
		return errors.New("rating must be between 1 and 5")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var t *WorkflowTemplate
	if temp, ok := r.templates[templateID]; ok {
		t = temp
	} else if temp, ok := r.community[templateID]; ok {
		t = temp
	} else {
		return fmt.Errorf("template not found: %s", templateID)
	}

	// Update rolling average
	t.Rating = ((t.Rating * float64(t.RatingCount)) + float64(rating)) / float64(t.RatingCount+1)
	t.RatingCount++

	return nil
}
