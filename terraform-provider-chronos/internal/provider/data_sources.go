// Package provider implements the Chronos Terraform provider.
package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &JobDataSource{}

// JobDataSource defines the data source implementation.
type JobDataSource struct {
	client *Client
}

// JobDataSourceModel describes the data source data model.
type JobDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Schedule    types.String `tfsdk:"schedule"`
	Timezone    types.String `tfsdk:"timezone"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	WebhookURL  types.String `tfsdk:"webhook_url"`
	Namespace   types.String `tfsdk:"namespace"`
}

// NewJobDataSource creates a new job data source.
func NewJobDataSource() datasource.DataSource {
	return &JobDataSource{}
}

// Metadata returns the data source type name.
func (d *JobDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_job"
}

// Schema defines the schema for the data source.
func (d *JobDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Chronos job by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The ID of the job to fetch.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the job.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "The description of the job.",
				Computed:    true,
			},
			"schedule": schema.StringAttribute{
				Description: "The cron schedule.",
				Computed:    true,
			},
			"timezone": schema.StringAttribute{
				Description: "The timezone.",
				Computed:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the job is enabled.",
				Computed:    true,
			},
			"webhook_url": schema.StringAttribute{
				Description: "The webhook URL.",
				Computed:    true,
			},
			"namespace": schema.StringAttribute{
				Description: "The namespace.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *JobDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Client, got: %T", req.ProviderData),
		)
		return
	}
	d.client = client
}

// Read refreshes the Terraform state with the latest data.
func (d *JobDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data JobDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	job, err := d.client.GetJob(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading job", err.Error())
		return
	}

	data.Name = types.StringValue(job.Name)
	data.Description = types.StringValue(job.Description)
	data.Schedule = types.StringValue(job.Schedule)
	data.Timezone = types.StringValue(job.Timezone)
	data.Enabled = types.BoolValue(job.Enabled)
	if job.Webhook != nil {
		data.WebhookURL = types.StringValue(job.Webhook.URL)
	}
	data.Namespace = types.StringValue(job.Namespace)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// JobsDataSource defines the data source for listing jobs.
type JobsDataSource struct {
	client *Client
}

// JobsDataSourceModel describes the data source data model.
type JobsDataSourceModel struct {
	Namespace types.String         `tfsdk:"namespace"`
	Jobs      []JobDataSourceModel `tfsdk:"jobs"`
}

// NewJobsDataSource creates a new jobs data source.
func NewJobsDataSource() datasource.DataSource {
	return &JobsDataSource{}
}

// Metadata returns the data source type name.
func (d *JobsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jobs"
}

// Schema defines the schema for the data source.
func (d *JobsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches all Chronos jobs, optionally filtered by namespace.",
		Attributes: map[string]schema.Attribute{
			"namespace": schema.StringAttribute{
				Description: "Filter jobs by namespace.",
				Optional:    true,
			},
			"jobs": schema.ListNestedAttribute{
				Description: "List of jobs.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
						"description": schema.StringAttribute{
							Computed: true,
						},
						"schedule": schema.StringAttribute{
							Computed: true,
						},
						"timezone": schema.StringAttribute{
							Computed: true,
						},
						"enabled": schema.BoolAttribute{
							Computed: true,
						},
						"webhook_url": schema.StringAttribute{
							Computed: true,
						},
						"namespace": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *JobsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Client, got: %T", req.ProviderData),
		)
		return
	}
	d.client = client
}

// Read refreshes the Terraform state with the latest data.
func (d *JobsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data JobsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespace := ""
	if !data.Namespace.IsNull() {
		namespace = data.Namespace.ValueString()
	}

	jobs, err := d.client.ListJobs(ctx, namespace)
	if err != nil {
		resp.Diagnostics.AddError("Error listing jobs", err.Error())
		return
	}

	data.Jobs = make([]JobDataSourceModel, len(jobs))
	for i, job := range jobs {
		data.Jobs[i] = JobDataSourceModel{
			ID:          types.StringValue(job.ID),
			Name:        types.StringValue(job.Name),
			Description: types.StringValue(job.Description),
			Schedule:    types.StringValue(job.Schedule),
			Timezone:    types.StringValue(job.Timezone),
			Enabled:     types.BoolValue(job.Enabled),
			Namespace:   types.StringValue(job.Namespace),
		}
		if job.Webhook != nil {
			data.Jobs[i].WebhookURL = types.StringValue(job.Webhook.URL)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// DAGDataSource defines the data source implementation.
type DAGDataSource struct {
	client *Client
}

// DAGDataSourceModel describes the data source data model.
type DAGDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

// NewDAGDataSource creates a new DAG data source.
func NewDAGDataSource() datasource.DataSource {
	return &DAGDataSource{}
}

// Metadata returns the data source type name.
func (d *DAGDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dag"
}

// Schema defines the schema for the data source.
func (d *DAGDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Chronos DAG by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The ID of the DAG to fetch.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the DAG.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "The description of the DAG.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *DAGDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Client, got: %T", req.ProviderData),
		)
		return
	}
	d.client = client
}

// Read refreshes the Terraform state with the latest data.
func (d *DAGDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DAGDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dag, err := d.client.GetDAG(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading DAG", err.Error())
		return
	}

	data.Name = types.StringValue(dag.Name)
	data.Description = types.StringValue(dag.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
