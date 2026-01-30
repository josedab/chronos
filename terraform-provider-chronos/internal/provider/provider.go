// Package provider implements the Chronos Terraform provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ChronosProvider satisfies various provider interfaces.
var _ provider.Provider = &ChronosProvider{}

// ChronosProvider defines the provider implementation.
type ChronosProvider struct {
	version string
}

// ChronosProviderModel describes the provider data model.
type ChronosProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
	Timeout  types.Int64  `tfsdk:"timeout"`
}

// New creates a new provider instance.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ChronosProvider{
			version: version,
		}
	}
}

// Metadata returns the provider type name.
func (p *ChronosProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "chronos"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *ChronosProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for managing Chronos distributed cron jobs.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "The Chronos API endpoint URL. Can also be set via CHRONOS_ENDPOINT environment variable.",
				Optional:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "API key for authentication. Can also be set via CHRONOS_API_KEY environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"timeout": schema.Int64Attribute{
				Description: "Request timeout in seconds. Defaults to 30.",
				Optional:    true,
			},
		},
	}
}

// Configure prepares the Chronos API client for data sources and resources.
func (p *ChronosProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config ChronosProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve endpoint from config or environment
	endpoint := os.Getenv("CHRONOS_ENDPOINT")
	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}
	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Missing Chronos API Endpoint",
			"The provider cannot create the Chronos API client without an endpoint. "+
				"Set the endpoint value in the configuration or use the CHRONOS_ENDPOINT environment variable.",
		)
		return
	}

	// Resolve API key from config or environment
	apiKey := os.Getenv("CHRONOS_API_KEY")
	if !config.APIKey.IsNull() {
		apiKey = config.APIKey.ValueString()
	}

	// Resolve timeout
	timeout := int64(30)
	if !config.Timeout.IsNull() {
		timeout = config.Timeout.ValueInt64()
	}

	// Create the API client
	client := NewClient(endpoint, apiKey, timeout)

	// Make the client available to resources and data sources
	resp.DataSourceData = client
	resp.ResourceData = client
}

// Resources defines the resources implemented in the provider.
func (p *ChronosProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewJobResource,
		NewDAGResource,
		NewTriggerResource,
		NewNamespaceResource,
		NewWorkflowResource,
		NewPolicyResource,
	}
}

// DataSources defines the data sources implemented in the provider.
func (p *ChronosProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewJobDataSource,
		NewJobsDataSource,
		NewDAGDataSource,
	}
}
