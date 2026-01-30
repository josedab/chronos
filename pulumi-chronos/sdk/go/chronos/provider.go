// Package chronos provides the Pulumi SDK for the Chronos distributed cron system.
// This package enables infrastructure-as-code management of Chronos resources
// including jobs, workflows, policies, and namespaces.
package chronos

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Provider configures the Chronos provider.
type Provider struct {
	pulumi.ProviderResourceState

	Endpoint pulumi.StringPtrInput
	APIKey   pulumi.StringPtrInput
}

// ProviderArgs contains the arguments for creating a Provider.
type ProviderArgs struct {
	Endpoint pulumi.StringPtrInput
	APIKey   pulumi.StringPtrInput
}

func (ProviderArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*Provider)(nil)).Elem()
}

// NewProvider creates a new Chronos provider.
func NewProvider(ctx *pulumi.Context, name string, args *ProviderArgs, opts ...pulumi.ResourceOption) (*Provider, error) {
	if args == nil {
		args = &ProviderArgs{}
	}

	provider := &Provider{}
	err := ctx.RegisterResource("pulumi:providers:chronos", name, args, provider, opts...)
	if err != nil {
		return nil, err
	}

	return provider, nil
}

// Job represents a Chronos cron job.
type Job struct {
	pulumi.CustomResourceState

	Name        pulumi.StringOutput         `pulumi:"name"`
	Description pulumi.StringPtrOutput      `pulumi:"description"`
	Schedule    pulumi.StringOutput         `pulumi:"schedule"`
	Timezone    pulumi.StringPtrOutput      `pulumi:"timezone"`
	Enabled     pulumi.BoolOutput           `pulumi:"enabled"`
	Namespace   pulumi.StringPtrOutput      `pulumi:"namespace"`
	WebhookURL  pulumi.StringPtrOutput      `pulumi:"webhookUrl"`
	Tags        pulumi.StringMapOutput      `pulumi:"tags"`
	Timeout     pulumi.StringPtrOutput      `pulumi:"timeout"`
	MaxRetries  pulumi.IntPtrOutput         `pulumi:"maxRetries"`
}

// JobArgs contains the arguments for creating a Job.
type JobArgs struct {
	Name        pulumi.StringInput
	Description pulumi.StringPtrInput
	Schedule    pulumi.StringInput
	Timezone    pulumi.StringPtrInput
	Enabled     pulumi.BoolPtrInput
	Namespace   pulumi.StringPtrInput
	WebhookURL  pulumi.StringPtrInput
	Tags        pulumi.StringMapInput
	Timeout     pulumi.StringPtrInput
	MaxRetries  pulumi.IntPtrInput
}

func (JobArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*Job)(nil)).Elem()
}

// NewJob creates a new Chronos job.
func NewJob(ctx *pulumi.Context, name string, args *JobArgs, opts ...pulumi.ResourceOption) (*Job, error) {
	if args == nil {
		return nil, fmt.Errorf("args is required for Job")
	}
	if args.Name == nil {
		return nil, fmt.Errorf("job name is required")
	}
	if args.Schedule == nil {
		return nil, fmt.Errorf("job schedule is required")
	}

	job := &Job{}
	err := ctx.RegisterResource("chronos:index:Job", name, args, job, opts...)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// GetJob retrieves an existing Chronos job by ID.
func GetJob(ctx *pulumi.Context, name string, id pulumi.IDInput, state *JobState, opts ...pulumi.ResourceOption) (*Job, error) {
	job := &Job{}
	err := ctx.ReadResource("chronos:index:Job", name, id, state, job, opts...)
	if err != nil {
		return nil, err
	}
	return job, nil
}

// JobState represents the state of a Job.
type JobState struct {
	Name        pulumi.StringPtrInput
	Description pulumi.StringPtrInput
	Schedule    pulumi.StringPtrInput
	Timezone    pulumi.StringPtrInput
	Enabled     pulumi.BoolPtrInput
	Namespace   pulumi.StringPtrInput
	WebhookURL  pulumi.StringPtrInput
	Tags        pulumi.StringMapInput
	Timeout     pulumi.StringPtrInput
	MaxRetries  pulumi.IntPtrInput
}

func (JobState) ElementType() reflect.Type {
	return reflect.TypeOf((*Job)(nil)).Elem()
}

// Workflow represents a Chronos DAG workflow.
type Workflow struct {
	pulumi.CustomResourceState

	Name        pulumi.StringOutput       `pulumi:"name"`
	Description pulumi.StringPtrOutput    `pulumi:"description"`
	Namespace   pulumi.StringPtrOutput    `pulumi:"namespace"`
	Enabled     pulumi.BoolOutput         `pulumi:"enabled"`
	Steps       WorkflowStepArrayOutput   `pulumi:"steps"`
	Triggers    WorkflowTriggerArrayOutput `pulumi:"triggers"`
}

// WorkflowArgs contains the arguments for creating a Workflow.
type WorkflowArgs struct {
	Name        pulumi.StringInput
	Description pulumi.StringPtrInput
	Namespace   pulumi.StringPtrInput
	Enabled     pulumi.BoolPtrInput
	Steps       WorkflowStepArrayInput
	Triggers    WorkflowTriggerArrayInput
}

func (WorkflowArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*Workflow)(nil)).Elem()
}

// WorkflowStep represents a step in a workflow.
type WorkflowStep struct {
	ID        string   `pulumi:"id"`
	Name      string   `pulumi:"name"`
	Type      string   `pulumi:"type"`
	JobID     *string  `pulumi:"jobId"`
	DependsOn []string `pulumi:"dependsOn"`
	Condition *string  `pulumi:"condition"`
	Timeout   *string  `pulumi:"timeout"`
}

// WorkflowStepInput is an input type for WorkflowStep.
type WorkflowStepInput interface {
	pulumi.Input
	ToWorkflowStepOutput() WorkflowStepOutput
	ToWorkflowStepOutputWithContext(context.Context) WorkflowStepOutput
}

// WorkflowStepArray is an array of WorkflowStep.
type WorkflowStepArray []WorkflowStepInput

// WorkflowStepArrayInput is an input type for WorkflowStepArray.
type WorkflowStepArrayInput interface {
	pulumi.Input
	ToWorkflowStepArrayOutput() WorkflowStepArrayOutput
	ToWorkflowStepArrayOutputWithContext(context.Context) WorkflowStepArrayOutput
}

// WorkflowStepOutput is an output type for WorkflowStep.
type WorkflowStepOutput struct{ *pulumi.OutputState }

func (WorkflowStepOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*WorkflowStep)(nil)).Elem()
}

// WorkflowStepArrayOutput is an output type for WorkflowStepArray.
type WorkflowStepArrayOutput struct{ *pulumi.OutputState }

func (WorkflowStepArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]WorkflowStep)(nil)).Elem()
}

func (WorkflowStepArray) ElementType() reflect.Type {
	return reflect.TypeOf((*[]WorkflowStep)(nil)).Elem()
}

func (i WorkflowStepArray) ToWorkflowStepArrayOutput() WorkflowStepArrayOutput {
	return i.ToWorkflowStepArrayOutputWithContext(context.Background())
}

func (i WorkflowStepArray) ToWorkflowStepArrayOutputWithContext(ctx context.Context) WorkflowStepArrayOutput {
	return pulumi.ToOutputWithContext(ctx, i).(WorkflowStepArrayOutput)
}

// WorkflowTrigger represents a trigger for a workflow.
type WorkflowTrigger struct {
	Type     string  `pulumi:"type"`
	Name     string  `pulumi:"name"`
	Schedule *string `pulumi:"schedule"`
	Enabled  bool    `pulumi:"enabled"`
}

// WorkflowTriggerInput is an input type for WorkflowTrigger.
type WorkflowTriggerInput interface {
	pulumi.Input
	ToWorkflowTriggerOutput() WorkflowTriggerOutput
	ToWorkflowTriggerOutputWithContext(context.Context) WorkflowTriggerOutput
}

// WorkflowTriggerArray is an array of WorkflowTrigger.
type WorkflowTriggerArray []WorkflowTriggerInput

// WorkflowTriggerArrayInput is an input type for WorkflowTriggerArray.
type WorkflowTriggerArrayInput interface {
	pulumi.Input
	ToWorkflowTriggerArrayOutput() WorkflowTriggerArrayOutput
	ToWorkflowTriggerArrayOutputWithContext(context.Context) WorkflowTriggerArrayOutput
}

// WorkflowTriggerOutput is an output type for WorkflowTrigger.
type WorkflowTriggerOutput struct{ *pulumi.OutputState }

func (WorkflowTriggerOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*WorkflowTrigger)(nil)).Elem()
}

// WorkflowTriggerArrayOutput is an output type for WorkflowTriggerArray.
type WorkflowTriggerArrayOutput struct{ *pulumi.OutputState }

func (WorkflowTriggerArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]WorkflowTrigger)(nil)).Elem()
}

// NewWorkflow creates a new Chronos workflow.
func NewWorkflow(ctx *pulumi.Context, name string, args *WorkflowArgs, opts ...pulumi.ResourceOption) (*Workflow, error) {
	if args == nil {
		return nil, fmt.Errorf("args is required for Workflow")
	}
	if args.Name == nil {
		return nil, fmt.Errorf("workflow name is required")
	}

	workflow := &Workflow{}
	err := ctx.RegisterResource("chronos:index:Workflow", name, args, workflow, opts...)
	if err != nil {
		return nil, err
	}

	return workflow, nil
}

// Policy represents a Chronos scheduling policy.
type Policy struct {
	pulumi.CustomResourceState

	Name        pulumi.StringOutput      `pulumi:"name"`
	Description pulumi.StringPtrOutput   `pulumi:"description"`
	Type        pulumi.StringOutput      `pulumi:"type"`
	Namespace   pulumi.StringPtrOutput   `pulumi:"namespace"`
	Enabled     pulumi.BoolOutput        `pulumi:"enabled"`
	Priority    pulumi.IntOutput         `pulumi:"priority"`
	Rules       PolicyRuleArrayOutput    `pulumi:"rules"`
	Actions     PolicyActionArrayOutput  `pulumi:"actions"`
}

// PolicyArgs contains the arguments for creating a Policy.
type PolicyArgs struct {
	Name        pulumi.StringInput
	Description pulumi.StringPtrInput
	Type        pulumi.StringInput
	Namespace   pulumi.StringPtrInput
	Enabled     pulumi.BoolPtrInput
	Priority    pulumi.IntPtrInput
	Rules       PolicyRuleArrayInput
	Actions     PolicyActionArrayInput
}

func (PolicyArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*Policy)(nil)).Elem()
}

// PolicyRule represents a rule in a policy.
type PolicyRule struct {
	Field    string `pulumi:"field"`
	Operator string `pulumi:"operator"`
	Value    string `pulumi:"value"`
}

// PolicyRuleArrayOutput is an output type for PolicyRuleArray.
type PolicyRuleArrayOutput struct{ *pulumi.OutputState }

func (PolicyRuleArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]PolicyRule)(nil)).Elem()
}

// PolicyRuleArrayInput is an input type for PolicyRuleArray.
type PolicyRuleArrayInput interface {
	pulumi.Input
}

// PolicyAction represents an action in a policy.
type PolicyAction struct {
	Type   string            `pulumi:"type"`
	Config map[string]string `pulumi:"config"`
}

// PolicyActionArrayOutput is an output type for PolicyActionArray.
type PolicyActionArrayOutput struct{ *pulumi.OutputState }

func (PolicyActionArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]PolicyAction)(nil)).Elem()
}

// PolicyActionArrayInput is an input type for PolicyActionArray.
type PolicyActionArrayInput interface {
	pulumi.Input
}

// NewPolicy creates a new Chronos policy.
func NewPolicy(ctx *pulumi.Context, name string, args *PolicyArgs, opts ...pulumi.ResourceOption) (*Policy, error) {
	if args == nil {
		return nil, fmt.Errorf("args is required for Policy")
	}
	if args.Name == nil {
		return nil, fmt.Errorf("policy name is required")
	}
	if args.Type == nil {
		return nil, fmt.Errorf("policy type is required")
	}

	policy := &Policy{}
	err := ctx.RegisterResource("chronos:index:Policy", name, args, policy, opts...)
	if err != nil {
		return nil, err
	}

	return policy, nil
}

// Namespace represents a Chronos namespace for multi-tenancy.
type Namespace struct {
	pulumi.CustomResourceState

	Name        pulumi.StringOutput    `pulumi:"name"`
	Description pulumi.StringPtrOutput `pulumi:"description"`
	Labels      pulumi.StringMapOutput `pulumi:"labels"`
	MaxJobs     pulumi.IntPtrOutput    `pulumi:"maxJobs"`
}

// NamespaceArgs contains the arguments for creating a Namespace.
type NamespaceArgs struct {
	Name        pulumi.StringInput
	Description pulumi.StringPtrInput
	Labels      pulumi.StringMapInput
	MaxJobs     pulumi.IntPtrInput
}

func (NamespaceArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*Namespace)(nil)).Elem()
}

// NewNamespace creates a new Chronos namespace.
func NewNamespace(ctx *pulumi.Context, name string, args *NamespaceArgs, opts ...pulumi.ResourceOption) (*Namespace, error) {
	if args == nil {
		return nil, fmt.Errorf("args is required for Namespace")
	}
	if args.Name == nil {
		return nil, fmt.Errorf("namespace name is required")
	}

	namespace := &Namespace{}
	err := ctx.RegisterResource("chronos:index:Namespace", name, args, namespace, opts...)
	if err != nil {
		return nil, err
	}

	return namespace, nil
}

// Client is the Chronos API client for the Pulumi provider.
type Client struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Chronos API client.
func NewClient(endpoint, apiKey string) *Client {
	return &Client{
		endpoint: endpoint,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateJob creates a job via the Chronos API.
func (c *Client) CreateJob(ctx context.Context, job *JobInput) (*JobOutput, error) {
	// Implementation would make HTTP request to Chronos API
	return nil, fmt.Errorf("not implemented - provider must implement dynamic provider interface")
}

// GetJob retrieves a job by ID.
func (c *Client) GetJob(ctx context.Context, id string) (*JobOutput, error) {
	return nil, fmt.Errorf("not implemented - provider must implement dynamic provider interface")
}

// UpdateJob updates an existing job.
func (c *Client) UpdateJob(ctx context.Context, id string, job *JobInput) (*JobOutput, error) {
	return nil, fmt.Errorf("not implemented - provider must implement dynamic provider interface")
}

// DeleteJob deletes a job.
func (c *Client) DeleteJob(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented - provider must implement dynamic provider interface")
}

// JobInput represents input for creating/updating a job.
type JobInput struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Schedule    string            `json:"schedule"`
	Timezone    string            `json:"timezone,omitempty"`
	Enabled     bool              `json:"enabled"`
	Namespace   string            `json:"namespace,omitempty"`
	WebhookURL  string            `json:"webhook_url,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	MaxRetries  int               `json:"max_retries,omitempty"`
}

// JobOutput represents output from the Chronos API for jobs.
type JobOutput struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Schedule    string            `json:"schedule"`
	Timezone    string            `json:"timezone,omitempty"`
	Enabled     bool              `json:"enabled"`
	Namespace   string            `json:"namespace,omitempty"`
	WebhookURL  string            `json:"webhook_url,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
	MaxRetries  int               `json:"max_retries,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// init registers the Chronos resource types with Pulumi.
func init() {
	pulumi.RegisterOutputType(WorkflowStepOutput{})
	pulumi.RegisterOutputType(WorkflowStepArrayOutput{})
	pulumi.RegisterOutputType(WorkflowTriggerOutput{})
	pulumi.RegisterOutputType(WorkflowTriggerArrayOutput{})
	pulumi.RegisterOutputType(PolicyRuleArrayOutput{})
	pulumi.RegisterOutputType(PolicyActionArrayOutput{})
}
