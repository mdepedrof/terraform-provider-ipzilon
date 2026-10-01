package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var (
	_ datasource.DataSource                   = &NetworksDataSource{}
	_ datasource.DataSourceWithValidateConfig = &NetworksDataSource{}
)

type NetworksDataSource struct{ client *client.Client }

func NewNetworksDataSource() datasource.DataSource { return &NetworksDataSource{} }

type networksModel struct {
	ID      types.Int64   `tfsdk:"id"`
	HubID   types.Int64   `tfsdk:"hub_id"`
	ScopeID types.Int64   `tfsdk:"scope_id"`
	CIDR    types.String  `tfsdk:"cidr"`
	Name    types.String  `tfsdk:"name"`
	Items   []networkItem `tfsdk:"items"`
}

type networkItem struct {
	ID          types.Int64  `tfsdk:"id"`
	ScopeID     types.Int64  `tfsdk:"scope_id"`
	Name        types.String `tfsdk:"name"`
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
}

var networkItemSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id":          schema.Int64Attribute{Computed: true, Description: "Network ID."},
		"scope_id":    schema.Int64Attribute{Computed: true, Description: "ID of the scope the network belongs to."},
		"name":        schema.StringAttribute{Computed: true, Description: "Network name."},
		"cidr":        schema.StringAttribute{Computed: true, Description: "Network CIDR block."},
		"description": schema.StringAttribute{Computed: true, Description: "Free-text description."},
	},
}

func (d *NetworksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_networks"
}

func (d *NetworksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List networks. Provide id (singular), hub_id, scope_id or only name/cidr. Without hub_id and scope_id the lookup is global (IPzilon >= 3.2.0).",
		Attributes: map[string]schema.Attribute{
			"id":       schema.Int64Attribute{Optional: true, Description: "Lookup a single network by ID. Cannot be combined with hub_id or scope_id."},
			"hub_id":   schema.Int64Attribute{Optional: true, Description: "Filter: networks across all scopes of a hub. Cannot be combined with scope_id."},
			"scope_id": schema.Int64Attribute{Optional: true, Description: "Filter: networks directly under a scope. Cannot be combined with hub_id."},
			"cidr":     schema.StringAttribute{Optional: true, Description: "Filter: cidr match (server-side). With hub_id or scope_id it is an exact text match; without them it compares the network, so the value must be a CIDR without host bits."},
			"name":     schema.StringAttribute{Optional: true, Description: "Filter: exact name match (server-side)."},
			"items":    schema.ListNestedAttribute{Computed: true, NestedObject: networkItemSchema, Description: "Matching networks; an empty list when nothing matches."},
		},
	}
}

func (d *NetworksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func networkToItem(n client.Network) networkItem {
	return networkItem{
		ID:          types.Int64Value(n.ID),
		ScopeID:     types.Int64Value(n.ScopeID),
		Name:        types.StringValue(n.Name),
		CIDR:        types.StringValue(n.CIDR),
		Description: types.StringPointerValue(n.Description),
	}
}

func (d *NetworksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg networksModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !cfg.ID.IsNull() && !cfg.ID.IsUnknown()
	hasHub := !cfg.HubID.IsNull() && !cfg.HubID.IsUnknown()
	hasScope := !cfg.ScopeID.IsNull() && !cfg.ScopeID.IsUnknown()

	parentCount := 0
	if hasHub {
		parentCount++
	}
	if hasScope {
		parentCount++
	}

	if hasID && parentCount > 0 {
		resp.Diagnostics.AddError("Conflicting filters", "Provide either id or a parent filter (hub_id/scope_id), not both.")
		return
	}
	if parentCount > 1 {
		resp.Diagnostics.AddError("Conflicting filters", "Provide either hub_id or scope_id, not both.")
		return
	}

	items := []networkItem{}
	if hasID {
		var n client.Network
		if err := d.client.Get(ctx, fmt.Sprintf("/networks/%d", cfg.ID.ValueInt64()), &n); err != nil {
			resp.Diagnostics.AddError("Get network failed", err.Error())
			return
		}
		items = []networkItem{networkToItem(n)}
	} else if hasHub {
		reqURL := hubNetworksURL(cfg.HubID.ValueInt64(), stringFilter(cfg.CIDR), stringFilter(cfg.Name))

		networks, err := client.GetAll[client.Network](ctx, d.client, reqURL)
		if err != nil {
			resp.Diagnostics.AddError("List networks failed", err.Error())
			return
		}
		for _, n := range networks {
			items = append(items, networkToItem(n))
		}
	} else if hasScope {
		reqURL := scopeNetworksURL(cfg.ScopeID.ValueInt64(), stringFilter(cfg.CIDR), stringFilter(cfg.Name))

		networks, err := client.GetAll[client.Network](ctx, d.client, reqURL)
		if err != nil {
			resp.Diagnostics.AddError("List networks failed", err.Error())
			return
		}
		for _, n := range networks {
			items = append(items, networkToItem(n))
		}
	} else {
		const feature = "ipzilon_networks without hub_id or scope_id"
		if !requireGlobalLists(d.client, feature, &resp.Diagnostics) {
			return
		}
		networks, err := client.GetAll[client.Network](ctx, d.client, globalNetworksURL(stringFilter(cfg.CIDR), stringFilter(cfg.Name)))
		if err != nil {
			globalListError(&resp.Diagnostics, "List networks failed", feature, err)
			return
		}
		for _, n := range networks {
			items = append(items, networkToItem(n))
		}
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}

// ValidateConfig checks in plan that cidr is a network address when the
// lookup is global (no id, hub_id or scope_id).
func (d *NetworksDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var cfg networksModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.ID.IsNull() && cfg.HubID.IsNull() && cfg.ScopeID.IsNull() {
		validateGlobalCIDR(ctx, req.Config, "cidr", &resp.Diagnostics)
	}
}
