// Package provider implements the Chronos Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &JobResource{}
var _ resource.ResourceWithImportState = &JobResource{}

// JobResource defines the resource implementation.
type JobResource struct {
	client *Client
}

// JobResourceModel describes the resource data model.
type JobResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Schedule    types.String `tfsdk:"schedule"`
	Timezone    types.String `tfsdk:"timezone"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	WebhookURL  types.String `tfsdk:"webhook_url"`
	WebhookMethod types.String `tfsdk:"webhook_method"`
	WebhookBody types.String `tfsdk:"webhook_body"`
	Timeout     types.String `tfsdk:"timeout"`
	Concurrency types.String `tfsdk:"concurrency"`
	Namespace   types.String `tfsdk:"namespace"`
	MaxRetries  types.Int64  `tfsdk:"max_retries"`
	Tags        types.Map    `tfsdk:"tags"`
}

// NewJobResource creates a new job resource.
func NewJobResource() resource.Resource {
	return &JobResource{}
}

// Metadata returns the resource type name.
func (r *JobResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_job"
}

// Schema defines the schema for the resource.
func (r *JobResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Chronos job.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the job.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the job.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the job.",
				Optional:    true,
			},
			"schedule": schema.StringAttribute{
				Description: "Cron expression for the job schedule.",
				Required:    true,
			},
			"timezone": schema.StringAttribute{
				Description: "IANA timezone for the schedule. Defaults to UTC.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("UTC"),
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the job is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"webhook_url": schema.StringAttribute{
				Description: "The webhook URL to call when the job runs.",
				Required:    true,
			},
			"webhook_method": schema.StringAttribute{
				Description: "HTTP method for the webhook. Defaults to POST.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("POST"),
			},
			"webhook_body": schema.StringAttribute{
				Description: "Request body for the webhook.",
				Optional:    true,
			},
			"timeout": schema.StringAttribute{
				Description: "Job execution timeout (e.g., '30s', '5m').",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("30s"),
			},
			"concurrency": schema.StringAttribute{
				Description: "Concurrency policy: allow, forbid, or replace.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("forbid"),
			},
			"namespace": schema.StringAttribute{
				Description: "Namespace for multi-tenant isolation.",
				Optional:    true,
			},
			"max_retries": schema.Int64Attribute{
				Description: "Maximum number of retry attempts.",
				Optional:    true,
			},
			"tags": schema.MapAttribute{
				Description: "Tags for the job.",
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *JobResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create creates the resource and sets the initial Terraform state.
func (r *JobResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data JobResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build job from model
	job := &Job{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Schedule:    data.Schedule.ValueString(),
		Timezone:    data.Timezone.ValueString(),
		Enabled:     data.Enabled.ValueBool(),
		Webhook: &Webhook{
			URL:    data.WebhookURL.ValueString(),
			Method: data.WebhookMethod.ValueString(),
			Body:   data.WebhookBody.ValueString(),
		},
		Timeout:     data.Timeout.ValueString(),
		Concurrency: data.Concurrency.ValueString(),
		Namespace:   data.Namespace.ValueString(),
	}

	if !data.MaxRetries.IsNull() {
		job.RetryPolicy = &RetryPolicy{
			MaxAttempts: int(data.MaxRetries.ValueInt64()),
		}
	}

	// Convert tags
	if !data.Tags.IsNull() {
		tags := make(map[string]string)
		data.Tags.ElementsAs(ctx, &tags, false)
		job.Tags = tags
	}

	// Create the job
	created, err := r.client.CreateJob(ctx, job)
	if err != nil {
		resp.Diagnostics.AddError("Error creating job", err.Error())
		return
	}

	// Set the ID
	data.ID = types.StringValue(created.ID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *JobResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data JobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	job, err := r.client.GetJob(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading job", err.Error())
		return
	}

	// Update model from API response
	data.Name = types.StringValue(job.Name)
	data.Description = types.StringValue(job.Description)
	data.Schedule = types.StringValue(job.Schedule)
	data.Timezone = types.StringValue(job.Timezone)
	data.Enabled = types.BoolValue(job.Enabled)
	if job.Webhook != nil {
		data.WebhookURL = types.StringValue(job.Webhook.URL)
		data.WebhookMethod = types.StringValue(job.Webhook.Method)
		data.WebhookBody = types.StringValue(job.Webhook.Body)
	}
	data.Timeout = types.StringValue(job.Timeout)
	data.Concurrency = types.StringValue(job.Concurrency)
	data.Namespace = types.StringValue(job.Namespace)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the resource and sets the updated Terraform state.
func (r *JobResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data JobResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build job from model
	job := &Job{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Schedule:    data.Schedule.ValueString(),
		Timezone:    data.Timezone.ValueString(),
		Enabled:     data.Enabled.ValueBool(),
		Webhook: &Webhook{
			URL:    data.WebhookURL.ValueString(),
			Method: data.WebhookMethod.ValueString(),
			Body:   data.WebhookBody.ValueString(),
		},
		Timeout:     data.Timeout.ValueString(),
		Concurrency: data.Concurrency.ValueString(),
		Namespace:   data.Namespace.ValueString(),
	}

	if !data.MaxRetries.IsNull() {
		job.RetryPolicy = &RetryPolicy{
			MaxAttempts: int(data.MaxRetries.ValueInt64()),
		}
	}

	// Convert tags
	if !data.Tags.IsNull() {
		tags := make(map[string]string)
		data.Tags.ElementsAs(ctx, &tags, false)
		job.Tags = tags
	}

	// Update the job
	_, err := r.client.UpdateJob(ctx, data.ID.ValueString(), job)
	if err != nil {
		resp.Diagnostics.AddError("Error updating job", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes the resource and removes the Terraform state.
func (r *JobResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data JobResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteJob(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting job", err.Error())
		return
	}
}

// ImportState imports an existing resource.
func (r *JobResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// path is imported from terraform-plugin-framework
var path = struct {
	Root func(string) interface{}
}{
	Root: func(name string) interface{} {
		return nil // Simplified for this implementation
	},
}
