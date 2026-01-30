// Package provider implements the policy resource for the Chronos Terraform provider.
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
var _ resource.Resource = &PolicyResource{}
var _ resource.ResourceWithImportState = &PolicyResource{}

// PolicyResource defines the policy resource implementation.
type PolicyResource struct {
	client *Client
}

// PolicyResourceModel describes the policy data model.
type PolicyResourceModel struct {
	ID          types.String   `tfsdk:"id"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	Namespace   types.String   `tfsdk:"namespace"`
	Enabled     types.Bool     `tfsdk:"enabled"`
	Type        types.String   `tfsdk:"type"`
	Priority    types.Int64    `tfsdk:"priority"`
	Rules       []PolicyRule   `tfsdk:"rules"`
	Actions     []PolicyAction `tfsdk:"actions"`
}

// PolicyRule describes a policy rule.
type PolicyRule struct {
	Field    types.String `tfsdk:"field"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
}

// PolicyAction describes a policy action.
type PolicyAction struct {
	Type   types.String `tfsdk:"type"`
	Config types.Map    `tfsdk:"config"`
}

// NewPolicyResource creates the policy resource.
func NewPolicyResource() resource.Resource {
	return &PolicyResource{}
}

// Metadata returns the resource type name.
func (r *PolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

// Schema defines the schema for the resource.
func (r *PolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Chronos scheduling policy.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier for the policy.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the policy.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the policy.",
				Optional:    true,
			},
			"namespace": schema.StringAttribute{
				Description: "Namespace for the policy.",
				Optional:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the policy is enabled.",
				Optional:    true,
			},
			"type": schema.StringAttribute{
				Description: "Policy type (scheduling, resource, security, compliance).",
				Required:    true,
			},
			"priority": schema.Int64Attribute{
				Description: "Policy priority (higher values take precedence).",
				Optional:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"rules": schema.ListNestedBlock{
				Description: "Policy matching rules.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Description: "Field to match (e.g., 'labels.team', 'job.name', 'job.type').",
							Required:    true,
						},
						"operator": schema.StringAttribute{
							Description: "Comparison operator (eq, ne, in, not_in, matches, gt, lt, gte, lte).",
							Required:    true,
						},
						"value": schema.StringAttribute{
							Description: "Value to compare against.",
							Required:    true,
						},
					},
				},
			},
			"actions": schema.ListNestedBlock{
				Description: "Actions to take when the policy matches.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Description: "Action type (set_label, add_resource, set_timeout, add_annotation, notify, block, allow).",
							Required:    true,
						},
						"config": schema.MapAttribute{
							Description: "Action configuration.",
							ElementType: types.StringType,
							Optional:    true,
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *PolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *PolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy := r.modelToAPI(&plan)
	created, err := r.client.CreatePolicy(ctx, policy)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Policy",
			"Could not create policy: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read reads the resource.
func (r *PolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := r.client.GetPolicy(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Policy",
			"Could not read policy: "+err.Error(),
		)
		return
	}

	r.apiToModel(policy, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource.
func (r *PolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy := r.modelToAPI(&plan)
	updated, err := r.client.UpdatePolicy(ctx, plan.ID.ValueString(), policy)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Policy",
			"Could not update policy: "+err.Error(),
		)
		return
	}

	r.apiToModel(updated, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete deletes the resource.
func (r *PolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeletePolicy(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Policy",
			"Could not delete policy: "+err.Error(),
		)
		return
	}
}

// ImportState imports the resource.
func (r *PolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *PolicyResource) modelToAPI(model *PolicyResourceModel) *APIPolicy {
	policy := &APIPolicy{
		Name:        model.Name.ValueString(),
		Description: model.Description.ValueString(),
		Namespace:   model.Namespace.ValueString(),
		Enabled:     model.Enabled.ValueBool(),
		Type:        model.Type.ValueString(),
		Priority:    int(model.Priority.ValueInt64()),
	}

	for _, rule := range model.Rules {
		policy.Rules = append(policy.Rules, APIPolicyRule{
			Field:    rule.Field.ValueString(),
			Operator: rule.Operator.ValueString(),
			Value:    rule.Value.ValueString(),
		})
	}

	for _, action := range model.Actions {
		policy.Actions = append(policy.Actions, APIPolicyAction{
			Type: action.Type.ValueString(),
		})
	}

	return policy
}

func (r *PolicyResource) apiToModel(api *APIPolicy, model *PolicyResourceModel) {
	model.ID = types.StringValue(api.ID)
	model.Name = types.StringValue(api.Name)
	model.Description = types.StringValue(api.Description)
	model.Namespace = types.StringValue(api.Namespace)
	model.Enabled = types.BoolValue(api.Enabled)
	model.Type = types.StringValue(api.Type)
	model.Priority = types.Int64Value(int64(api.Priority))
}

// API types

// APIPolicy represents a policy in the API.
type APIPolicy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Namespace   string            `json:"namespace"`
	Enabled     bool              `json:"enabled"`
	Type        string            `json:"type"`
	Priority    int               `json:"priority"`
	Rules       []APIPolicyRule   `json:"rules"`
	Actions     []APIPolicyAction `json:"actions"`
}

// APIPolicyRule represents a policy rule.
type APIPolicyRule struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// APIPolicyAction represents a policy action.
type APIPolicyAction struct {
	Type   string            `json:"type"`
	Config map[string]string `json:"config,omitempty"`
}
