package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ datasource.DataSource = &NetworksDataSource{}

type NetworksDataSource struct{ client *client.Client }

func NewNetworksDataSource() datasource.DataSource { return &NetworksDataSource{} }

type networksModel struct {
	ID            types.Int64   `tfsdk:"id"`
	HubID         types.Int64   `tfsdk:"hub_id"`
	LandingZoneID types.Int64   `tfsdk:"landing_zone_id"`
	Items         []networkItem `tfsdk:"items"`
}

type networkItem struct {
	ID            types.Int64  `tfsdk:"id"`
	LandingZoneID types.Int64  `tfsdk:"landing_zone_id"`
	Name          types.String `tfsdk:"name"`
	CIDR          types.String `tfsdk:"cidr"`
	Description   types.String `tfsdk:"description"`
}

var networkItemSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id":              schema.Int64Attribute{Computed: true},
		"landing_zone_id": schema.Int64Attribute{Computed: true},
		"name":            schema.StringAttribute{Computed: true},
		"cidr":            schema.StringAttribute{Computed: true},
		"description":     schema.StringAttribute{Computed: true},
	},
}

func (d *NetworksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_networks"
}

func (d *NetworksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List networks. Provide id (singular), hub_id, or landing_zone_id.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.Int64Attribute{Optional: true, Description: "Lookup a single network by ID."},
			"hub_id":          schema.Int64Attribute{Optional: true, Description: "List all networks across all landing zones of a hub."},
			"landing_zone_id": schema.Int64Attribute{Optional: true, Description: "List networks for a specific landing zone."},
			"items":           schema.ListNestedAttribute{Computed: true, NestedObject: networkItemSchema},
		},
	}
}

func (d *NetworksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func networkToItem(n client.Network) networkItem {
	return networkItem{
		ID:            types.Int64Value(n.ID),
		LandingZoneID: types.Int64Value(n.LandingZoneID),
		Name:          types.StringValue(n.Name),
		CIDR:          types.StringValue(n.CIDR),
		Description:   types.StringPointerValue(n.Description),
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
	hasLZ := !cfg.LandingZoneID.IsNull() && !cfg.LandingZoneID.IsUnknown()

	parentCount := 0
	if hasHub {
		parentCount++
	}
	if hasLZ {
		parentCount++
	}

	if hasID && parentCount > 0 {
		resp.Diagnostics.AddError("Conflicting filters", "Provide either id or a parent filter (hub_id/landing_zone_id), not both.")
		return
	}
	if !hasID && parentCount == 0 {
		resp.Diagnostics.AddError("Missing filter", "Provide id, hub_id, or landing_zone_id.")
		return
	}
	if parentCount > 1 {
		resp.Diagnostics.AddError("Conflicting filters", "Provide either hub_id or landing_zone_id, not both.")
		return
	}

	var items []networkItem
	if hasID {
		var n client.Network
		if err := d.client.Get(fmt.Sprintf("/networks/%d", cfg.ID.ValueInt64()), &n); err != nil {
			resp.Diagnostics.AddError("Get network failed", err.Error())
			return
		}
		items = []networkItem{networkToItem(n)}
	} else if hasHub {
		var networks []client.Network
		if err := d.client.Get(fmt.Sprintf("/hubs/%d/networks", cfg.HubID.ValueInt64()), &networks); err != nil {
			resp.Diagnostics.AddError("List networks failed", err.Error())
			return
		}
		for _, n := range networks {
			items = append(items, networkToItem(n))
		}
	} else {
		var networks []client.Network
		if err := d.client.Get(fmt.Sprintf("/landing-zones/%d/networks", cfg.LandingZoneID.ValueInt64()), &networks); err != nil {
			resp.Diagnostics.AddError("List networks failed", err.Error())
			return
		}
		for _, n := range networks {
			items = append(items, networkToItem(n))
		}
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
