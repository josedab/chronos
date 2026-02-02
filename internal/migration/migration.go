// Package migration provides tools for migrating job configurations from other schedulers.
package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chronos/chronos/internal/dag"
	"github.com/chronos/chronos/internal/models"
	"github.com/google/uuid"
)

// Common errors.
var (
	ErrUnsupportedFormat = errors.New("unsupported format")
	ErrInvalidInput      = errors.New("invalid input")
	ErrPartialMigration  = errors.New("partial migration - some items could not be converted")
)

// SourceType represents the source scheduler type.
type SourceType string

const (
	SourceKubernetesCronJob SourceType = "kubernetes_cronjob"
	SourceAirflowDAG        SourceType = "airflow_dag"
	SourceAWSEventBridge    SourceType = "aws_eventbridge"
	SourceTemporalSchedule  SourceType = "temporal_schedule"
	SourceGitHubActions     SourceType = "github_actions"
)

// MigrationResult contains the result of a migration operation.
type MigrationResult struct {
	Success       bool              `json:"success"`
	SourceType    SourceType        `json:"source_type"`
	ItemsTotal    int               `json:"items_total"`
	ItemsMigrated int               `json:"items_migrated"`
	ItemsFailed   int               `json:"items_failed"`
	Jobs          []*models.Job     `json:"jobs,omitempty"`
	Workflows     []*dag.Workflow   `json:"workflows,omitempty"`
	Errors        []*MigrationError `json:"errors,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
}

// MigrationError contains details about a migration failure.
type MigrationError struct {
	ItemName string `json:"item_name"`
	ItemType string `json:"item_type"`
	Error    string `json:"error"`
	Line     int    `json:"line,omitempty"`
}

// Migrator handles migration from various sources.
type Migrator struct {
	converters map[SourceType]Converter
}

// Converter converts source-specific formats to Chronos formats.
type Converter interface {
	ConvertJobs(ctx context.Context, input []byte) ([]*models.Job, []MigrationError, error)
	ConvertWorkflows(ctx context.Context, input []byte) ([]*dag.Workflow, []MigrationError, error)
	Validate(input []byte) error
}

// NewMigrator creates a new migration handler.
func NewMigrator() *Migrator {
	m := &Migrator{
		converters: make(map[SourceType]Converter),
	}

	// Register converters
	m.converters[SourceKubernetesCronJob] = &KubernetesConverter{}
	m.converters[SourceAirflowDAG] = &AirflowConverter{}
	m.converters[SourceAWSEventBridge] = &EventBridgeConverter{}
	m.converters[SourceTemporalSchedule] = &TemporalConverter{}
	m.converters[SourceGitHubActions] = &GitHubActionsConverter{}

	return m
}

// Migrate performs migration from the specified source.
func (m *Migrator) Migrate(ctx context.Context, sourceType SourceType, input []byte) (*MigrationResult, error) {
	converter, ok := m.converters[sourceType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, sourceType)
	}

	if err := converter.Validate(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	result := &MigrationResult{
		SourceType: sourceType,
		Timestamp:  time.Now().UTC(),
		Errors:     make([]*MigrationError, 0),
		Warnings:   make([]string, 0),
	}

	// Convert jobs
	jobs, jobErrors, err := converter.ConvertJobs(ctx, input)
	if err != nil && !errors.Is(err, ErrPartialMigration) {
		return nil, err
	}

	for i := range jobErrors {
		result.Errors = append(result.Errors, &jobErrors[i])
	}
	result.Jobs = jobs
	result.ItemsTotal += len(jobs) + len(jobErrors)
	result.ItemsMigrated += len(jobs)
	result.ItemsFailed += len(jobErrors)

	// Convert workflows
	workflows, workflowErrors, err := converter.ConvertWorkflows(ctx, input)
	if err != nil && !errors.Is(err, ErrPartialMigration) {
		return nil, err
	}

	for i := range workflowErrors {
		result.Errors = append(result.Errors, &workflowErrors[i])
	}
	result.Workflows = workflows
	result.ItemsTotal += len(workflows) + len(workflowErrors)
	result.ItemsMigrated += len(workflows)
	result.ItemsFailed += len(workflowErrors)

	result.Success = result.ItemsFailed == 0

	return result, nil
}

// KubernetesConverter converts Kubernetes CronJob specs.
type KubernetesConverter struct{}

// KubernetesCronJob represents a Kubernetes CronJob spec.
type KubernetesCronJob struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Metadata   struct {
		Name        string            `json:"name" yaml:"name"`
		Namespace   string            `json:"namespace" yaml:"namespace"`
		Labels      map[string]string `json:"labels" yaml:"labels"`
		Annotations map[string]string `json:"annotations" yaml:"annotations"`
	} `json:"metadata" yaml:"metadata"`
	Spec struct {
		Schedule                   string `json:"schedule" yaml:"schedule"`
		TimeZone                   string `json:"timeZone" yaml:"timeZone"`
		ConcurrencyPolicy          string `json:"concurrencyPolicy" yaml:"concurrencyPolicy"`
		Suspend                    bool   `json:"suspend" yaml:"suspend"`
		StartingDeadlineSeconds    *int64 `json:"startingDeadlineSeconds" yaml:"startingDeadlineSeconds"`
		SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit" yaml:"successfulJobsHistoryLimit"`
		FailedJobsHistoryLimit     *int32 `json:"failedJobsHistoryLimit" yaml:"failedJobsHistoryLimit"`
		JobTemplate                struct {
			Spec struct {
				BackoffLimit *int32 `json:"backoffLimit" yaml:"backoffLimit"`
				Template     struct {
					Spec struct {
						Containers []struct {
							Name    string   `json:"name" yaml:"name"`
							Image   string   `json:"image" yaml:"image"`
							Command []string `json:"command" yaml:"command"`
							Args    []string `json:"args" yaml:"args"`
							Env     []struct {
								Name  string `json:"name" yaml:"name"`
								Value string `json:"value" yaml:"value"`
							} `json:"env" yaml:"env"`
						} `json:"containers" yaml:"containers"`
						RestartPolicy string `json:"restartPolicy" yaml:"restartPolicy"`
					} `json:"spec" yaml:"spec"`
				} `json:"template" yaml:"template"`
			} `json:"spec" yaml:"spec"`
		} `json:"jobTemplate" yaml:"jobTemplate"`
	} `json:"spec" yaml:"spec"`
}

func (c *KubernetesConverter) Validate(input []byte) error {
	var cj KubernetesCronJob
	if err := json.Unmarshal(input, &cj); err != nil {
		return err
	}
	if cj.Kind != "CronJob" {
		return fmt.Errorf("expected Kind 'CronJob', got '%s'", cj.Kind)
	}
	return nil
}

func (c *KubernetesConverter) ConvertJobs(ctx context.Context, input []byte) ([]*models.Job, []MigrationError, error) {
	var cj KubernetesCronJob
	if err := json.Unmarshal(input, &cj); err != nil {
		return nil, []MigrationError{{ItemName: "input", Error: err.Error()}}, ErrPartialMigration
	}

	job := &models.Job{
		ID:        uuid.New().String(),
		Name:      cj.Metadata.Name,
		Schedule:  cj.Spec.Schedule,
		Timezone:  cj.Spec.TimeZone,
		Enabled:   !cj.Spec.Suspend,
		Tags:      cj.Metadata.Labels,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Version:   1,
		Namespace: cj.Metadata.Namespace,
	}

	// Map concurrency policy
	switch cj.Spec.ConcurrencyPolicy {
	case "Forbid":
		job.Concurrency = models.ConcurrencyForbid
	case "Replace":
		job.Concurrency = models.ConcurrencyReplace
	default:
		job.Concurrency = models.ConcurrencyAllow
	}

	// Set retry policy from backoffLimit
	if cj.Spec.JobTemplate.Spec.BackoffLimit != nil {
		job.RetryPolicy = &models.RetryPolicy{
			MaxAttempts:     int(*cj.Spec.JobTemplate.Spec.BackoffLimit),
			InitialInterval: models.Duration(time.Second),
			MaxInterval:     models.Duration(time.Minute),
			Multiplier:      2.0,
		}
	}

	// Create placeholder webhook (container jobs need wrapping)
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Containers) > 0 {
		container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		job.Description = fmt.Sprintf("Migrated from K8s CronJob. Original image: %s", container.Image)
		job.Webhook = &models.WebhookConfig{
			URL:    "http://localhost:8080/k8s-job-runner",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":       "application/json",
				"X-Original-Image":   container.Image,
				"X-Migration-Source": "kubernetes-cronjob",
			},
		}
	}

	return []*models.Job{job}, nil, nil
}

func (c *KubernetesConverter) ConvertWorkflows(ctx context.Context, input []byte) ([]*dag.Workflow, []MigrationError, error) {
	// Kubernetes CronJobs are single jobs, not workflows
	return nil, nil, nil
}

// AirflowConverter converts Apache Airflow DAG definitions.
type AirflowConverter struct{}

// AirflowDAGSpec represents an Airflow DAG definition (simplified).
type AirflowDAGSpec struct {
	DagID            string                 `json:"dag_id"`
	Description      string                 `json:"description"`
	Schedule         string                 `json:"schedule_interval"`
	Timezone         string                 `json:"timezone"`
	DefaultArgs      map[string]interface{} `json:"default_args"`
	Catchup          bool                   `json:"catchup"`
	MaxActiveRuns    int                    `json:"max_active_runs"`
	Tasks            []AirflowTask          `json:"tasks"`
	TaskDependencies []TaskDependency       `json:"task_dependencies"`
	IsPaused         bool                   `json:"is_paused"`
}

// AirflowTask represents an Airflow task.
type AirflowTask struct {
	TaskID           string                 `json:"task_id"`
	TaskType         string                 `json:"task_type"` // BashOperator, PythonOperator, HttpOperator, etc.
	Pool             string                 `json:"pool"`
	Retries          int                    `json:"retries"`
	RetryDelay       int                    `json:"retry_delay"`       // seconds
	ExecutionTimeout int                    `json:"execution_timeout"` // seconds
	Params           map[string]interface{} `json:"params"`
	TriggerRule      string                 `json:"trigger_rule"` // all_success, all_failed, one_success, etc.
}

// TaskDependency represents task dependency.
type TaskDependency struct {
	Upstream   string `json:"upstream"`
	Downstream string `json:"downstream"`
}

func (c *AirflowConverter) Validate(input []byte) error {
	var spec AirflowDAGSpec
	return json.Unmarshal(input, &spec)
}

func (c *AirflowConverter) ConvertJobs(ctx context.Context, input []byte) ([]*models.Job, []MigrationError, error) {
	// Airflow DAGs are converted to workflows, not individual jobs
	return nil, nil, nil
}

func (c *AirflowConverter) ConvertWorkflows(ctx context.Context, input []byte) ([]*dag.Workflow, []MigrationError, error) {
	var spec AirflowDAGSpec
	if err := json.Unmarshal(input, &spec); err != nil {
		return nil, []MigrationError{{ItemName: "input", Error: err.Error()}}, ErrPartialMigration
	}

	builder := dag.NewWorkflowBuilder(spec.DagID)
	builder.GetWorkflow().Description = spec.Description

	// Create a trigger node with the schedule
	triggerPos := dag.Position{X: 100, Y: 100}
	triggerConfig := dag.TriggerConfig{
		Type:     "schedule",
		Schedule: spec.Schedule,
		Timezone: spec.Timezone,
	}
	triggerNode, err := builder.AddNode(dag.NodeTypeTrigger, "schedule_trigger", triggerPos, triggerConfig)
	if err != nil {
		return nil, []MigrationError{{ItemName: spec.DagID, Error: err.Error()}}, ErrPartialMigration
	}

	// Create nodes for each task
	taskIDMap := make(map[string]string) // Airflow task_id -> Chronos node ID
	taskIDMap["schedule_trigger"] = triggerNode.ID

	migrationErrors := make([]MigrationError, 0)
	yOffset := 200.0

	for i, task := range spec.Tasks {
		xPos := float64(100 + (i%4)*200)
		yPos := yOffset + float64(i/4)*150

		nodeType := c.mapTaskType(task.TaskType)
		config := c.mapTaskConfig(task)

		node, err := builder.AddNode(nodeType, task.TaskID, dag.Position{X: xPos, Y: yPos}, config)
		if err != nil {
			migrationErrors = append(migrationErrors, MigrationError{
				ItemName: task.TaskID,
				ItemType: "task",
				Error:    err.Error(),
			})
			continue
		}

		taskIDMap[task.TaskID] = node.ID

		// Store trigger rule in metadata
		node.Metadata["trigger_rule"] = task.TriggerRule
		node.Metadata["original_type"] = task.TaskType
	}

	// Create edges based on dependencies
	hasUpstream := make(map[string]bool)
	for _, dep := range spec.TaskDependencies {
		upstreamID, ok1 := taskIDMap[dep.Upstream]
		downstreamID, ok2 := taskIDMap[dep.Downstream]

		if !ok1 || !ok2 {
			continue
		}

		_, err := builder.AddEdge(upstreamID, "output", downstreamID, "input")
		if err != nil {
			migrationErrors = append(migrationErrors, MigrationError{
				ItemName: fmt.Sprintf("%s->%s", dep.Upstream, dep.Downstream),
				ItemType: "edge",
				Error:    err.Error(),
			})
		}
		hasUpstream[dep.Downstream] = true
	}

	// Connect trigger to all root tasks (tasks with no upstream)
	for taskID, nodeID := range taskIDMap {
		if taskID == "schedule_trigger" {
			continue
		}
		if !hasUpstream[taskID] {
			builder.AddEdge(triggerNode.ID, "output", nodeID, "input")
		}
	}

	// Configure workflow settings
	builder.SetSettings(&dag.WorkflowSettings{
		ConcurrencyPolicy: "forbid",
		OnFailure:         "stop",
		EnableLogging:     true,
	})

	workflow, err := builder.Build()
	if err != nil {
		return nil, append(migrationErrors, MigrationError{ItemName: spec.DagID, Error: err.Error()}), ErrPartialMigration
	}

	if len(migrationErrors) > 0 {
		return []*dag.Workflow{workflow}, migrationErrors, ErrPartialMigration
	}

	return []*dag.Workflow{workflow}, nil, nil
}

func (c *AirflowConverter) mapTaskType(airflowType string) dag.NodeType {
	switch airflowType {
	case "BashOperator":
		return dag.NodeTypeScript
	case "PythonOperator", "PythonVirtualenvOperator":
		return dag.NodeTypeScript
	case "SimpleHttpOperator", "HttpSensor":
		return dag.NodeTypeHTTP
	case "EmailOperator":
		return dag.NodeTypeNotification
	case "BranchPythonOperator", "ShortCircuitOperator":
		return dag.NodeTypeCondition
	case "DummyOperator", "EmptyOperator":
		return dag.NodeTypeDelay
	default:
		return dag.NodeTypeJob
	}
}

func (c *AirflowConverter) mapTaskConfig(task AirflowTask) interface{} {
	switch task.TaskType {
	case "BashOperator":
		cmd := ""
		if v, ok := task.Params["bash_command"].(string); ok {
			cmd = v
		}
		return dag.ScriptConfig{
			Language: "bash",
			Code:     cmd,
			Timeout:  fmt.Sprintf("%ds", task.ExecutionTimeout),
		}

	case "PythonOperator":
		code := ""
		if v, ok := task.Params["python_callable"].(string); ok {
			code = v
		}
		return dag.ScriptConfig{
			Language: "python",
			Code:     code,
			Timeout:  fmt.Sprintf("%ds", task.ExecutionTimeout),
		}

	case "SimpleHttpOperator":
		url := ""
		method := "GET"
		if v, ok := task.Params["http_conn_id"].(string); ok {
			url = v
		}
		if v, ok := task.Params["endpoint"].(string); ok {
			url += v
		}
		if v, ok := task.Params["method"].(string); ok {
			method = v
		}
		return dag.JobConfig{
			WebhookURL: url,
			Method:     method,
			Timeout:    fmt.Sprintf("%ds", task.ExecutionTimeout),
		}

	default:
		return map[string]interface{}{
			"original_params": task.Params,
			"migration_note":  fmt.Sprintf("Converted from Airflow %s - manual configuration may be required", task.TaskType),
		}
	}
}

// EventBridgeConverter converts AWS EventBridge rules.
type EventBridgeConverter struct{}

// EventBridgeRule represents an AWS EventBridge rule.
type EventBridgeRule struct {
	Name               string                 `json:"Name"`
	Description        string                 `json:"Description"`
	ScheduleExpression string                 `json:"ScheduleExpression"`
	State              string                 `json:"State"`
	Targets            []EventBridgeTarget    `json:"Targets"`
	EventPattern       map[string]interface{} `json:"EventPattern"`
}

// EventBridgeTarget represents a rule target.
type EventBridgeTarget struct {
	ID      string `json:"Id"`
	Arn     string `json:"Arn"`
	RoleArn string `json:"RoleArn"`
	Input   string `json:"Input"`
}

func (c *EventBridgeConverter) Validate(input []byte) error {
	var rule EventBridgeRule
	return json.Unmarshal(input, &rule)
}

func (c *EventBridgeConverter) ConvertJobs(ctx context.Context, input []byte) ([]*models.Job, []MigrationError, error) {
	var rule EventBridgeRule
	if err := json.Unmarshal(input, &rule); err != nil {
		return nil, []MigrationError{{ItemName: "input", Error: err.Error()}}, ErrPartialMigration
	}

	// Convert EventBridge schedule expression to cron
	schedule, err := c.convertScheduleExpression(rule.ScheduleExpression)
	if err != nil {
		return nil, []MigrationError{{
			ItemName: rule.Name,
			Error:    fmt.Sprintf("unsupported schedule expression: %s", rule.ScheduleExpression),
		}}, ErrPartialMigration
	}

	job := &models.Job{
		ID:          uuid.New().String(),
		Name:        rule.Name,
		Description: rule.Description,
		Schedule:    schedule,
		Enabled:     rule.State == "ENABLED",
		Tags: map[string]string{
			"migration_source": "aws_eventbridge",
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Version:   1,
	}

	// Convert first target to webhook
	if len(rule.Targets) > 0 {
		target := rule.Targets[0]
		job.Webhook = &models.WebhookConfig{
			URL:    c.arnToURL(target.Arn),
			Method: "POST",
			Body:   target.Input,
			Headers: map[string]string{
				"X-Original-Arn":     target.Arn,
				"X-Migration-Source": "aws-eventbridge",
			},
		}
	}

	return []*models.Job{job}, nil, nil
}

func (c *EventBridgeConverter) ConvertWorkflows(ctx context.Context, input []byte) ([]*dag.Workflow, []MigrationError, error) {
	return nil, nil, nil
}

func (c *EventBridgeConverter) convertScheduleExpression(expr string) (string, error) {
	// Handle rate expressions: rate(5 minutes), rate(1 hour), etc.
	rateRegex := regexp.MustCompile(`^rate\((\d+)\s+(minute|minutes|hour|hours|day|days)\)$`)
	if matches := rateRegex.FindStringSubmatch(expr); len(matches) == 3 {
		value, _ := strconv.Atoi(matches[1])
		unit := matches[2]

		switch {
		case strings.HasPrefix(unit, "minute"):
			return fmt.Sprintf("*/%d * * * *", value), nil
		case strings.HasPrefix(unit, "hour"):
			return fmt.Sprintf("0 */%d * * *", value), nil
		case strings.HasPrefix(unit, "day"):
			return fmt.Sprintf("0 0 */%d * *", value), nil
		}
	}

	// Handle cron expressions: cron(0 12 * * ? *)
	cronRegex := regexp.MustCompile(`^cron\((.+)\)$`)
	if matches := cronRegex.FindStringSubmatch(expr); len(matches) == 2 {
		// AWS cron has 6 fields (with year), convert to standard 5-field
		fields := strings.Fields(matches[1])
		if len(fields) == 6 {
			// Remove year field, convert ? to *
			fields = fields[:5]
			for i, f := range fields {
				if f == "?" {
					fields[i] = "*"
				}
			}
			return strings.Join(fields, " "), nil
		}
	}

	return "", fmt.Errorf("cannot convert expression: %s", expr)
}

func (c *EventBridgeConverter) arnToURL(arn string) string {
	// Convert ARN to placeholder URL
	return fmt.Sprintf("https://placeholder.example.com/lambda?arn=%s", arn)
}

// TemporalConverter converts Temporal Schedule definitions.
type TemporalConverter struct{}

// TemporalSchedule represents a Temporal schedule.
type TemporalSchedule struct {
	ScheduleID  string `json:"schedule_id"`
	Description string `json:"description"`
	Spec        struct {
		CronExpressions []string `json:"cron_expressions"`
		Intervals       []struct {
			Every  string `json:"every"`
			Offset string `json:"offset"`
		} `json:"intervals"`
		Timezone string `json:"timezone"`
	} `json:"spec"`
	Action struct {
		Workflow struct {
			Type      string        `json:"type"`
			TaskQueue string        `json:"task_queue"`
			Args      []interface{} `json:"args"`
		} `json:"workflow"`
	} `json:"action"`
	Policies struct {
		CatchupWindow  string `json:"catchup_window"`
		OverlapPolicy  string `json:"overlap_policy"`
		PauseOnFailure bool   `json:"pause_on_failure"`
	} `json:"policies"`
	State struct {
		Paused bool `json:"paused"`
	} `json:"state"`
}

func (c *TemporalConverter) Validate(input []byte) error {
	var sched TemporalSchedule
	return json.Unmarshal(input, &sched)
}

func (c *TemporalConverter) ConvertJobs(ctx context.Context, input []byte) ([]*models.Job, []MigrationError, error) {
	var sched TemporalSchedule
	if err := json.Unmarshal(input, &sched); err != nil {
		return nil, []MigrationError{{ItemName: "input", Error: err.Error()}}, ErrPartialMigration
	}

	// Get schedule expression
	schedule := ""
	if len(sched.Spec.CronExpressions) > 0 {
		schedule = sched.Spec.CronExpressions[0]
	} else if len(sched.Spec.Intervals) > 0 {
		schedule = "@every " + sched.Spec.Intervals[0].Every
	}

	if schedule == "" {
		return nil, []MigrationError{{
			ItemName: sched.ScheduleID,
			Error:    "no schedule expression found",
		}}, ErrPartialMigration
	}

	// Map overlap policy to concurrency
	var concurrency models.ConcurrencyPolicy
	switch sched.Policies.OverlapPolicy {
	case "SCHEDULE_OVERLAP_POLICY_SKIP":
		concurrency = models.ConcurrencyForbid
	case "SCHEDULE_OVERLAP_POLICY_CANCEL_OTHER":
		concurrency = models.ConcurrencyReplace
	default:
		concurrency = models.ConcurrencyAllow
	}

	job := &models.Job{
		ID:          uuid.New().String(),
		Name:        sched.ScheduleID,
		Description: sched.Description,
		Schedule:    schedule,
		Timezone:    sched.Spec.Timezone,
		Enabled:     !sched.State.Paused,
		Concurrency: concurrency,
		Tags: map[string]string{
			"migration_source":   "temporal_schedule",
			"original_workflow":  sched.Action.Workflow.Type,
			"original_taskqueue": sched.Action.Workflow.TaskQueue,
		},
		Webhook: &models.WebhookConfig{
			URL:    "http://localhost:8080/temporal-proxy",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":       "application/json",
				"X-Workflow-Type":    sched.Action.Workflow.Type,
				"X-Task-Queue":       sched.Action.Workflow.TaskQueue,
				"X-Migration-Source": "temporal-schedule",
			},
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Version:   1,
	}

	return []*models.Job{job}, nil, nil
}

func (c *TemporalConverter) ConvertWorkflows(ctx context.Context, input []byte) ([]*dag.Workflow, []MigrationError, error) {
	return nil, nil, nil
}

// GitHubActionsConverter converts GitHub Actions workflow schedules.
type GitHubActionsConverter struct{}

// GitHubActionsWorkflow represents a GitHub Actions workflow.
type GitHubActionsWorkflow struct {
	Name string `json:"name" yaml:"name"`
	On   struct {
		Schedule []struct {
			Cron string `json:"cron" yaml:"cron"`
		} `json:"schedule" yaml:"schedule"`
		WorkflowDispatch map[string]interface{} `json:"workflow_dispatch" yaml:"workflow_dispatch"`
	} `json:"on" yaml:"on"`
	Jobs map[string]struct {
		Name   string   `json:"name" yaml:"name"`
		RunsOn string   `json:"runs-on" yaml:"runs-on"`
		Needs  []string `json:"needs" yaml:"needs"`
		Steps  []struct {
			Name string            `json:"name" yaml:"name"`
			Uses string            `json:"uses" yaml:"uses"`
			Run  string            `json:"run" yaml:"run"`
			With map[string]string `json:"with" yaml:"with"`
		} `json:"steps" yaml:"steps"`
	} `json:"jobs" yaml:"jobs"`
}

func (c *GitHubActionsConverter) Validate(input []byte) error {
	var wf GitHubActionsWorkflow
	return json.Unmarshal(input, &wf)
}

func (c *GitHubActionsConverter) ConvertJobs(ctx context.Context, input []byte) ([]*models.Job, []MigrationError, error) {
	var wf GitHubActionsWorkflow
	if err := json.Unmarshal(input, &wf); err != nil {
		return nil, []MigrationError{{ItemName: "input", Error: err.Error()}}, ErrPartialMigration
	}

	jobs := make([]*models.Job, 0)

	for _, sched := range wf.On.Schedule {
		job := &models.Job{
			ID:          uuid.New().String(),
			Name:        wf.Name,
			Description: fmt.Sprintf("Migrated from GitHub Actions workflow: %s", wf.Name),
			Schedule:    sched.Cron,
			Enabled:     true,
			Tags: map[string]string{
				"migration_source": "github_actions",
			},
			Webhook: &models.WebhookConfig{
				URL:    "https://api.github.com/repos/{owner}/{repo}/actions/workflows/{workflow_id}/dispatches",
				Method: "POST",
				Headers: map[string]string{
					"Accept":               "application/vnd.github+json",
					"X-GitHub-Api-Version": "2022-11-28",
					"X-Migration-Source":   "github-actions",
				},
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			Version:   1,
		}
		jobs = append(jobs, job)
	}

	if len(jobs) == 0 {
		return nil, []MigrationError{{
			ItemName: wf.Name,
			Error:    "no schedule triggers found",
		}}, ErrPartialMigration
	}

	return jobs, nil, nil
}

func (c *GitHubActionsConverter) ConvertWorkflows(ctx context.Context, input []byte) ([]*dag.Workflow, []MigrationError, error) {
	var wf GitHubActionsWorkflow
	if err := json.Unmarshal(input, &wf); err != nil {
		return nil, []MigrationError{{ItemName: "input", Error: err.Error()}}, ErrPartialMigration
	}

	if len(wf.Jobs) <= 1 {
		return nil, nil, nil // Single job doesn't need DAG
	}

	builder := dag.NewWorkflowBuilder(wf.Name)

	// Add trigger
	var schedule string
	if len(wf.On.Schedule) > 0 {
		schedule = wf.On.Schedule[0].Cron
	}

	triggerConfig := dag.TriggerConfig{
		Type:     "schedule",
		Schedule: schedule,
	}
	trigger, _ := builder.AddNode(dag.NodeTypeTrigger, "trigger", dag.Position{X: 100, Y: 100}, triggerConfig)

	// Create job nodes
	jobIDMap := make(map[string]string)
	yPos := 250.0

	for jobName, jobSpec := range wf.Jobs {
		name := jobSpec.Name
		if name == "" {
			name = jobName
		}

		config := map[string]interface{}{
			"runs_on": jobSpec.RunsOn,
			"steps":   len(jobSpec.Steps),
		}

		node, err := builder.AddNode(dag.NodeTypeJob, name, dag.Position{X: 200, Y: yPos}, config)
		if err != nil {
			continue
		}
		jobIDMap[jobName] = node.ID
		yPos += 120
	}

	// Create edges based on 'needs'
	hasUpstream := make(map[string]bool)
	for jobName, jobSpec := range wf.Jobs {
		downstreamID := jobIDMap[jobName]
		for _, need := range jobSpec.Needs {
			upstreamID := jobIDMap[need]
			if upstreamID != "" && downstreamID != "" {
				builder.AddEdge(upstreamID, "output", downstreamID, "input")
				hasUpstream[jobName] = true
			}
		}
	}

	// Connect trigger to root jobs
	for jobName, nodeID := range jobIDMap {
		if !hasUpstream[jobName] {
			builder.AddEdge(trigger.ID, "output", nodeID, "input")
		}
	}

	workflow, err := builder.Build()
	if err != nil {
		return nil, []MigrationError{{ItemName: wf.Name, Error: err.Error()}}, ErrPartialMigration
	}

	return []*dag.Workflow{workflow}, nil, nil
}
