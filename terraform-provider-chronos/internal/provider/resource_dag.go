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
var _ resource.Resource = &DAGResource{}

// DAGResource defines the resource implementation.
type DAGResource struct {
	client *Client
}

// DAGResourceModel describes the resource data model.
type DAGResourceModel struct {
	ID          types.String   `tfsdk:"id"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	Nodes       []DAGNodeModel `tfsdk:"nodes"`
}

// DAGNodeModel describes a node in the DAG.
type DAGNodeModel struct {
	ID           types.String `tfsdk:"id"`
	JobID        types.String `tfsdk:"job_id"`
	Dependencies types.List   `tfsdk:"dependencies"`
	Condition    types.String `tfsdk:"condition"`
}

// NewDAGResource creates a new DAG resource.
func NewDAGResource() resource.Resource {
	return &DAGResource{}
}

// Metadata returns the resource type name.
func (r *DAGResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dag"
}

// Schema defines the schema for the resource.
func (r *DAGResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Chronos DAG (Directed Acyclic Graph) workflow.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the DAG.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the DAG.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the DAG.",
				Optional:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"nodes": schema.ListNestedBlock{
				Description: "The nodes in the DAG.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "The unique identifier of the node within the DAG.",
							Required:    true,
						},
						"job_id": schema.StringAttribute{
							Description: "The ID of the job to execute for this node.",
							Required:    true,
						},
						"dependencies": schema.ListAttribute{
							Description: "List of node IDs this node depends on.",
							Optional:    true,
							ElementType: types.StringType,
						},
						"condition": schema.StringAttribute{
							Description: "Trigger condition: all_success, all_complete, any_success, none.",
							Optional:    true,
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *DAGResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *DAGResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DAGResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dag := &DAG{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Nodes:       make([]DAGNode, len(data.Nodes)),
	}

	for i, node := range data.Nodes {
		var deps []string
		node.Dependencies.ElementsAs(ctx, &deps, false)
		dag.Nodes[i] = DAGNode{
			ID:           node.ID.ValueString(),
			JobID:        node.JobID.ValueString(),
			Dependencies: deps,
			Condition:    node.Condition.ValueString(),
		}
	}

	created, err := r.client.CreateDAG(ctx, dag)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DAG", err.Error())
		return
	}

	data.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *DAGResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DAGResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dag, err := r.client.GetDAG(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading DAG", err.Error())
		return
	}

	data.Name = types.StringValue(dag.Name)
	data.Description = types.StringValue(dag.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the resource.
func (r *DAGResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DAGResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dag := &DAG{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Nodes:       make([]DAGNode, len(data.Nodes)),
	}

	for i, node := range data.Nodes {
		var deps []string
		node.Dependencies.ElementsAs(ctx, &deps, false)
		dag.Nodes[i] = DAGNode{
			ID:           node.ID.ValueString(),
			JobID:        node.JobID.ValueString(),
			Dependencies: deps,
			Condition:    node.Condition.ValueString(),
		}
	}

	_, err := r.client.UpdateDAG(ctx, data.ID.ValueString(), dag)
	if err != nil {
		resp.Diagnostics.AddError("Error updating DAG", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes the resource.
func (r *DAGResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DAGResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteDAG(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting DAG", err.Error())
		return
	}
}
