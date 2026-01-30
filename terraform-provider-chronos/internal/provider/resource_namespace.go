// Package provider implements the Chronos Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &NamespaceResource{}

// NamespaceResource defines the resource implementation.
type NamespaceResource struct {
	client *Client
}

// NamespaceResourceModel describes the resource data model.
type NamespaceResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	MaxJobs     types.Int64  `tfsdk:"max_jobs"`
	Labels      types.Map    `tfsdk:"labels"`
}

// NewNamespaceResource creates a new namespace resource.
func NewNamespaceResource() resource.Resource {
	return &NamespaceResource{}
}

// Metadata returns the resource type name.
func (r *NamespaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

// Schema defines the schema for the resource.
func (r *NamespaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Chronos namespace for multi-tenant isolation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the namespace.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the namespace.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the namespace.",
				Optional:    true,
			},
			"max_jobs": schema.Int64Attribute{
				Description: "Maximum number of jobs allowed in this namespace.",
				Optional:    true,
			},
			"labels": schema.MapAttribute{
				Description: "Labels for the namespace.",
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *NamespaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *NamespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NamespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := &Namespace{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		MaxJobs:     int(data.MaxJobs.ValueInt64()),
	}

	if !data.Labels.IsNull() {
		labels := make(map[string]string)
		data.Labels.ElementsAs(ctx, &labels, false)
		ns.Labels = labels
	}

	created, err := r.client.CreateNamespace(ctx, ns)
	if err != nil {
		resp.Diagnostics.AddError("Error creating namespace", err.Error())
		return
	}

	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state.
func (r *NamespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NamespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns, err := r.client.GetNamespace(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading namespace", err.Error())
		return
	}

	data.Name = types.StringValue(ns.Name)
	data.Description = types.StringValue(ns.Description)
	data.MaxJobs = types.Int64Value(int64(ns.MaxJobs))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the resource.
func (r *NamespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NamespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := &Namespace{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		MaxJobs:     int(data.MaxJobs.ValueInt64()),
	}

	if !data.Labels.IsNull() {
		labels := make(map[string]string)
		data.Labels.ElementsAs(ctx, &labels, false)
		ns.Labels = labels
	}

	_, err := r.client.UpdateNamespace(ctx, data.ID.ValueString(), ns)
	if err != nil {
		resp.Diagnostics.AddError("Error updating namespace", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes the resource.
func (r *NamespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NamespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteNamespace(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting namespace", err.Error())
		return
	}
}
