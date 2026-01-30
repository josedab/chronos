// Package provider implements the Chronos Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TriggerResource{}

// TriggerResource defines the resource implementation.
type TriggerResource struct {
	client *Client
}

// TriggerResourceModel describes the resource data model.
type TriggerResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	JobID   types.String `tfsdk:"job_id"`
	Enabled types.Bool   `tfsdk:"enabled"`
	Config  types.Map    `tfsdk:"config"`
}

// NewTriggerResource creates a new trigger resource.
func NewTriggerResource() resource.Resource {
	return &TriggerResource{}
}

// Metadata returns the resource type name.
func (r *TriggerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trigger"
}

// Schema defines the schema for the resource.
func (r *TriggerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Chronos event trigger.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the trigger.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the trigger.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "Trigger type: kafka, sqs, pubsub, rabbitmq, webhook.",
				Required:    true,
			},
			"job_id": schema.StringAttribute{
				Description: "The ID of the job to trigger.",
				Required:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the trigger is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"config": schema.MapAttribute{
				Description: "Type-specific configuration (e.g., brokers, topic for Kafka).",
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *TriggerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *Client, got: %T", req.ProviderData),
		)
		return
	}
	r.client = client
}

// Create creates the resource.
func (r *TriggerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	trigger := &Trigger{
		Name:    data.Name.ValueString(),
		Type:    data.Type.ValueString(),
		JobID:   data.JobID.ValueString(),
		Enabled: data.Enabled.ValueBool(),
	}

	if !data.Config.IsNull() {
		config := make(map[string]string)
		data.Config.ElementsAs(ctx, &config, false)
		trigger.Config = config
	}

	created, err := r.client.CreateTrigger(ctx, trigger)
	if err != nil {
		resp.Diagnostics.AddError("Error creating trigger", err.Error())
		return
	}

	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state.
func (r *TriggerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	trigger, err := r.client.GetTrigger(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading trigger", err.Error())
		return
	}

	data.Name = types.StringValue(trigger.Name)
	data.Type = types.StringValue(trigger.Type)
	data.JobID = types.StringValue(trigger.JobID)
	data.Enabled = types.BoolValue(trigger.Enabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the resource.
func (r *TriggerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TriggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	trigger := &Trigger{
		Name:    data.Name.ValueString(),
		Type:    data.Type.ValueString(),
		JobID:   data.JobID.ValueString(),
		Enabled: data.Enabled.ValueBool(),
	}

	if !data.Config.IsNull() {
		config := make(map[string]string)
		data.Config.ElementsAs(ctx, &config, false)
		trigger.Config = config
	}

	_, err := r.client.UpdateTrigger(ctx, data.ID.ValueString(), trigger)
	if err != nil {
		resp.Diagnostics.AddError("Error updating trigger", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes the resource.
func (r *TriggerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TriggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteTrigger(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting trigger", err.Error())
		return
	}
}
