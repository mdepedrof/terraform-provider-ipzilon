package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ datasource.DataSource = &NetworkZonesDataSource{}

type NetworkZonesDataSource struct{ client *client.Client }

func NewNetworkZonesDataSource() datasource.DataSource { return &NetworkZonesDataSource{} }

type networkZonesModel struct {
	ID        types.Int64       `tfsdk:"id"`
	NetworkID types.Int64       `tfsdk:"network_id"`
	Name      types.String      `tfsdk:"name"`
	CIDR      types.String      `tfsdk:"cidr"`
	Items     []networkZoneItem `tfsdk:"items"`
}

type networkZoneItem struct {
	ID          types.Int64  `tfsdk:"id"`
	NetworkID   types.Int64  `tfsdk:"network_id"`
	Name        types.String `tfsdk:"name"`
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
}

var networkZoneItemSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id":          schema.Int64Attribute{Computed: true, Description: "Zone ID."},
		"network_id":  schema.Int64Attribute{Computed: true, Description: "Network the zone belongs to."},
		"name":        schema.StringAttribute{Computed: true, Description: "Zone name (unique within its network)."},
		"cidr":        schema.StringAttribute{Computed: true, Description: "Zone CIDR block."},
		"description": schema.StringAttribute{Computed: true, Description: "Free-text description."},
	},
}

func (d *NetworkZonesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_zones"
}

func (d *NetworkZonesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List network zones by name and/or CIDR without knowing any id (IPzilon >= 3.1.0). " +
			"A zone name is only unique within its network, so a name filter may return several zones; add network_id to narrow it. " +
			"Without filters every zone is returned. Provide id alone to look up a single zone.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.Int64Attribute{Optional: true, Description: "Lookup a single zone by ID. Cannot be combined with other filters."},
			"network_id": schema.Int64Attribute{Optional: true, Description: "Filter: zones of this network (server-side)."},
			"name":       schema.StringAttribute{Optional: true, Description: "Filter: exact name match (server-side)."},
			"cidr":       schema.StringAttribute{Optional: true, Description: "Filter: exact cidr match (server-side)."},
			"items":      schema.ListNestedAttribute{Computed: true, NestedObject: networkZoneItemSchema, Description: "Matching zones, ordered by CIDR."},
		},
	}
}

func (d *NetworkZonesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func networkZoneToItem(z client.NetworkZone) networkZoneItem {
	return networkZoneItem{
		ID:          types.Int64Value(z.ID),
		NetworkID:   types.Int64Value(z.NetworkID),
		Name:        types.StringValue(z.Name),
		CIDR:        types.StringValue(z.CIDR),
		Description: types.StringPointerValue(z.Description),
	}
}

func (d *NetworkZonesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg networkZonesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := d.client.RequireAPIVersion(client.MinZonesAPIVersion, "ipzilon_network_zones"); err != nil {
		resp.Diagnostics.AddError("IPzilon version not supported", err.Error())
		return
	}

	hasID := !cfg.ID.IsNull() && !cfg.ID.IsUnknown()
	if hasID && (!cfg.NetworkID.IsNull() || !cfg.Name.IsNull() || !cfg.CIDR.IsNull()) {
		resp.Diagnostics.AddError("Conflicting filters", "Provide either id or the network_id/name/cidr filters, not both.")
		return
	}

	items := []networkZoneItem{}
	if hasID {
		var z client.NetworkZone
		if err := d.client.Get(ctx, fmt.Sprintf("/zones/%d", cfg.ID.ValueInt64()), &z); err != nil {
			resp.Diagnostics.AddError("Get network zone failed", err.Error())
			return
		}
		items = append(items, networkZoneToItem(z))
	} else {
		var networkID *int64
		if !cfg.NetworkID.IsNull() && !cfg.NetworkID.IsUnknown() {
			n := cfg.NetworkID.ValueInt64()
			networkID = &n
		}
		zones, err := client.GetAll[client.NetworkZone](ctx, d.client, zonesURL(networkID, stringFilter(cfg.Name), stringFilter(cfg.CIDR)))
		if err != nil {
			resp.Diagnostics.AddError("List network zones failed", err.Error())
			return
		}
		for _, z := range zones {
			items = append(items, networkZoneToItem(z))
		}
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
