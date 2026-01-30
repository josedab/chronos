// Package provider implements additional resources for the Chronos Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &WorkflowResource{}
var _ resource.ResourceWithImportState = &WorkflowResource{}

// WorkflowResource defines the workflow resource implementation.
type WorkflowResource struct {
	client *Client
}

// WorkflowResourceModel describes the workflow data model.
type WorkflowResourceModel struct {
	ID          types.String          `tfsdk:"id"`
	Name        types.String          `tfsdk:"name"`
	Description types.String          `tfsdk:"description"`
	Namespace   types.String          `tfsdk:"namespace"`
	Enabled     types.Bool            `tfsdk:"enabled"`
	Steps       []WorkflowStepModel   `tfsdk:"steps"`
	Triggers    []WorkflowTriggerModel `tfsdk:"triggers"`
	Labels      types.Map             `tfsdk:"labels"`
	Timeout     types.String          `tfsdk:"timeout"`
	RetryPolicy *RetryPolicyModel     `tfsdk:"retry_policy"`
}

// WorkflowStepModel describes a workflow step.
type WorkflowStepModel struct {
	ID           types.String   `tfsdk:"id"`
	Name         types.String   `tfsdk:"name"`
	Type         types.String   `tfsdk:"type"`
	JobID        types.String   `tfsdk:"job_id"`
	DependsOn    types.List     `tfsdk:"depends_on"`
	Condition    types.String   `tfsdk:"condition"`
	Timeout      types.String   `tfsdk:"timeout"`
	OnSuccess    types.List     `tfsdk:"on_success"`
	OnFailure    types.List     `tfsdk:"on_failure"`
	Config       types.Map      `tfsdk:"config"`
}

// WorkflowTriggerModel describes a workflow trigger.
type WorkflowTriggerModel struct {
	Type     types.String `tfsdk:"type"`
	Name     types.String `tfsdk:"name"`
	Schedule types.String `tfsdk:"schedule"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Config   types.Map    `tfsdk:"config"`
}

// RetryPolicyModel describes retry policy.
type RetryPolicyModel struct {
	MaxAttempts     types.Int64   `tfsdk:"max_attempts"`
	InitialInterval types.String  `tfsdk:"initial_interval"`
	MaxInterval     types.String  `tfsdk:"max_interval"`
	Multiplier      types.Float64 `tfsdk:"multiplier"`
}

// NewWorkflowResource creates the workflow resource.
func NewWorkflowResource() resource.Resource {
	return &WorkflowResource{}
}

// Metadata returns the resource type name.
func (r *WorkflowResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow"
}

// Schema defines the schema for the resource.
func (r *WorkflowResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Chronos workflow (DAG-based job orchestration).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier for the workflow.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the workflow.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of what the workflow does.",
				Optional:    true,
			},
			"namespace": schema.StringAttribute{
				Description: "Namespace for the workflow.",
				Optional:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the workflow is enabled.",
				Optional:    true,
			},
			"labels": schema.MapAttribute{
				Description: "Labels to attach to the workflow.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"timeout": schema.StringAttribute{
				Description: "Overall workflow timeout (e.g., '30m', '1h').",
				Optional:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"steps": schema.ListNestedBlock{
				Description: "Steps in the workflow.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Unique identifier for the step.",
							Required:    true,
						},
						"name": schema.StringAttribute{
							Description: "Display name for the step.",
							Required:    true,
						},
						"type": schema.StringAttribute{
							Description: "Type of step (job, http, condition, parallel, delay, notification).",
							Required:    true,
						},
						"job_id": schema.StringAttribute{
							Description: "ID of the job to execute (for job type steps).",
							Optional:    true,
						},
						"depends_on": schema.ListAttribute{
							Description: "Step IDs that must complete before this step.",
							ElementType: types.StringType,
							Optional:    true,
						},
						"condition": schema.StringAttribute{
							Description: "Condition expression for conditional execution.",
							Optional:    true,
						},
						"timeout": schema.StringAttribute{
							Description: "Step timeout.",
							Optional:    true,
						},
						"on_success": schema.ListAttribute{
							Description: "Step IDs to trigger on success.",
							ElementType: types.StringType,
							Optional:    true,
						},
						"on_failure": schema.ListAttribute{
							Description: "Step IDs to trigger on failure.",
							ElementType: types.StringType,
							Optional:    true,
						},
						"config": schema.MapAttribute{
							Description: "Step-specific configuration.",
							ElementType: types.StringType,
							Optional:    true,
						},
					},
				},
			},
			"triggers": schema.ListNestedBlock{
				Description: "Triggers for the workflow.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Description: "Trigger type (cron, webhook, manual, event).",
							Required:    true,
						},
						"name": schema.StringAttribute{
							Description: "Display name for the trigger.",
							Required:    true,
						},
						"schedule": schema.StringAttribute{
							Description: "Cron schedule for cron triggers.",
							Optional:    true,
						},
						"enabled": schema.BoolAttribute{
							Description: "Whether the trigger is enabled.",
							Optional:    true,
						},
						"config": schema.MapAttribute{
							Description: "Trigger-specific configuration.",
							ElementType: types.StringType,
							Optional:    true,
						},
					},
				},
			},
			"retry_policy": schema.SingleNestedBlock{
				Description: "Retry policy for the workflow.",
				Attributes: map[string]schema.Attribute{
					"max_attempts": schema.Int64Attribute{
						Description: "Maximum number of retry attempts.",
						Optional:    true,
					},
					"initial_interval": schema.StringAttribute{
						Description: "Initial retry interval.",
						Optional:    true,
					},
					"max_interval": schema.StringAttribute{
						Description: "Maximum retry interval.",
						Optional:    true,
					},
					"multiplier": schema.Float64Attribute{
						Description: "Backoff multiplier.",
						Optional:    true,
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *WorkflowResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *Client, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// Create creates the resource.
func (r *WorkflowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WorkflowResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create workflow via API
	workflow := r.modelToAPI(&plan)
	created, err := r.client.CreateWorkflow(ctx, workflow)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Workflow",
			"Could not create workflow: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read reads the resource.
func (r *WorkflowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WorkflowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workflow, err := r.client.GetWorkflow(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Workflow",
			"Could not read workflow: "+err.Error(),
		)
		return
	}

	r.apiToModel(workflow, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource.
func (r *WorkflowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WorkflowResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workflow := r.modelToAPI(&plan)
	updated, err := r.client.UpdateWorkflow(ctx, plan.ID.ValueString(), workflow)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Workflow",
			"Could not update workflow: "+err.Error(),
		)
		return
	}

	r.apiToModel(updated, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete deletes the resource.
func (r *WorkflowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WorkflowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteWorkflow(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Workflow",
			"Could not delete workflow: "+err.Error(),
		)
		return
	}
}

// ImportState imports the resource.
func (r *WorkflowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *WorkflowResource) modelToAPI(model *WorkflowResourceModel) *APIWorkflow {
	workflow := &APIWorkflow{
		Name:        model.Name.ValueString(),
		Description: model.Description.ValueString(),
		Namespace:   model.Namespace.ValueString(),
		Enabled:     model.Enabled.ValueBool(),
	}

	// Convert steps
	for _, step := range model.Steps {
		apiStep := APIWorkflowStep{
			ID:        step.ID.ValueString(),
			Name:      step.Name.ValueString(),
			Type:      step.Type.ValueString(),
			JobID:     step.JobID.ValueString(),
			Condition: step.Condition.ValueString(),
			Timeout:   step.Timeout.ValueString(),
		}
		workflow.Steps = append(workflow.Steps, apiStep)
	}

	// Convert triggers
	for _, trigger := range model.Triggers {
		apiTrigger := APIWorkflowTrigger{
			Type:     trigger.Type.ValueString(),
			Name:     trigger.Name.ValueString(),
			Schedule: trigger.Schedule.ValueString(),
			Enabled:  trigger.Enabled.ValueBool(),
		}
		workflow.Triggers = append(workflow.Triggers, apiTrigger)
	}

	return workflow
}

func (r *WorkflowResource) apiToModel(api *APIWorkflow, model *WorkflowResourceModel) {
	model.ID = types.StringValue(api.ID)
	model.Name = types.StringValue(api.Name)
	model.Description = types.StringValue(api.Description)
	model.Namespace = types.StringValue(api.Namespace)
	model.Enabled = types.BoolValue(api.Enabled)
}

// API types

// APIWorkflow represents a workflow in the API.
type APIWorkflow struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Namespace   string               `json:"namespace"`
	Enabled     bool                 `json:"enabled"`
	Steps       []APIWorkflowStep    `json:"steps"`
	Triggers    []APIWorkflowTrigger `json:"triggers"`
}

// APIWorkflowStep represents a workflow step.
type APIWorkflowStep struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	JobID     string `json:"job_id,omitempty"`
	Condition string `json:"condition,omitempty"`
	Timeout   string `json:"timeout,omitempty"`
}

// APIWorkflowTrigger represents a workflow trigger.
type APIWorkflowTrigger struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Schedule string `json:"schedule,omitempty"`
	Enabled  bool   `json:"enabled"`
}
