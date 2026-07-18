package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
	"github.com/mdepedrof/terraform-provider-ipzilon/internal/datasources"
	"github.com/mdepedrof/terraform-provider-ipzilon/internal/resources"
)

var _ provider.Provider = &IPzilon{}

type IPzilon struct{ version string }

type ipzilonModel struct {
	APIURL types.String `tfsdk:"api_url"`
	Token  types.String `tfsdk:"token"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &IPzilon{version: version} }
}

func (p *IPzilon) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ipzilon"
	resp.Version = p.version
}

func (p *IPzilon) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage IP address space in IPzilon — an IPAM for Azure and on-premise networks. Supports sites, hubs, landing zones, networks, subnets, and individual IP addresses.",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				Optional:    true,
				Description: "IPzilon API base URL (e.g. https://ipzilon.example.com). Can be set via IPZILON_API_URL environment variable.",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "IPzilon API token (ipam_...). Env: IPZILON_TOKEN.",
			},
		},
	}
}

func (p *IPzilon) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg ipzilonModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiURL := "http://localhost:8000"
	if v := os.Getenv("IPZILON_API_URL"); v != "" {
		apiURL = v
	}
	if !cfg.APIURL.IsNull() && !cfg.APIURL.IsUnknown() {
		apiURL = cfg.APIURL.ValueString()
	}

	token := os.Getenv("IPZILON_TOKEN")
	if !cfg.Token.IsNull() && !cfg.Token.IsUnknown() {
		token = cfg.Token.ValueString()
	}

	if token == "" {
		resp.Diagnostics.AddError("Missing token", "Set token in the provider block or via the IPZILON_TOKEN environment variable.")
		return
	}

	c := client.New(apiURL, token)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *IPzilon) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewHubResource,
		resources.NewLandingZoneResource,
		resources.NewNetworkResource,
		resources.NewSubnetResource,
		resources.NewNextSubnetResource,
		resources.NewLastSubnetResource,
		resources.NewIPAddressResource,
		resources.NewNextIPAddressResource,
	}
}

func (p *IPzilon) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewHubsDataSource,
		datasources.NewLandingZonesDataSource,
		datasources.NewNetworksDataSource,
		datasources.NewSubnetsDataSource,
		datasources.NewIPAddressesDataSource,
	}
}
