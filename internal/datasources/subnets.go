package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ datasource.DataSource = &SubnetsDataSource{}

type SubnetsDataSource struct{ client *client.Client }

func NewSubnetsDataSource() datasource.DataSource { return &SubnetsDataSource{} }

type subnetsModel struct {
	ID        types.Int64  `tfsdk:"id"`
	NetworkID types.Int64  `tfsdk:"network_id"`
	ZoneID    types.Int64  `tfsdk:"zone_id"`
	NoZone    types.Bool   `tfsdk:"no_zone"`
	Items     []subnetItem `tfsdk:"items"`
}

type subnetItem struct {
	ID          types.Int64  `tfsdk:"id"`
	NetworkID   types.Int64  `tfsdk:"network_id"`
	Name        types.String `tfsdk:"name"`
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
	ZoneID      types.Int64  `tfsdk:"zone_id"`
}

var subnetItemSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id":          schema.Int64Attribute{Computed: true},
		"network_id":  schema.Int64Attribute{Computed: true},
		"name":        schema.StringAttribute{Computed: true, Description: "Subnet name."},
		"cidr":        schema.StringAttribute{Computed: true, Description: "Subnet CIDR block."},
		"description": schema.StringAttribute{Computed: true, Description: "Free-text description."},
		"zone_id":     schema.Int64Attribute{Computed: true, Description: "Zone that contains the subnet (null if none; IPzilon >= 3.1.0)."},
	},
}

func (d *SubnetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subnets"
}

func (d *SubnetsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List subnets. Provide id (singular), network_id or zone_id.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.Int64Attribute{Optional: true, Description: "Lookup a single subnet by ID. Cannot be combined with other filters."},
			"network_id": schema.Int64Attribute{Optional: true, Description: "List all subnets for a network."},
			"zone_id":    schema.Int64Attribute{Optional: true, Description: "List the subnets inside this zone (IPzilon >= 3.1.0). network_id is not required."},
			"no_zone":    schema.BoolAttribute{Optional: true, Description: "When true, list only the subnets of network_id outside every zone (IPzilon >= 3.1.0). Requires network_id; cannot be combined with zone_id."},
			"items":      schema.ListNestedAttribute{Computed: true, NestedObject: subnetItemSchema},
		},
	}
}

func (d *SubnetsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func subnetToItem(s client.Subnet) subnetItem {
	return subnetItem{
		ID:          types.Int64Value(s.ID),
		NetworkID:   types.Int64Value(s.NetworkID),
		Name:        types.StringValue(s.Name),
		CIDR:        types.StringValue(s.CIDR),
		Description: types.StringPointerValue(s.Description),
		ZoneID:      types.Int64PointerValue(s.ZoneID),
	}
}

func (d *SubnetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg subnetsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !cfg.ID.IsNull() && !cfg.ID.IsUnknown()
	hasNet := !cfg.NetworkID.IsNull() && !cfg.NetworkID.IsUnknown()
	hasZone := !cfg.ZoneID.IsNull() && !cfg.ZoneID.IsUnknown()
	noZone := cfg.NoZone.ValueBool()

	if !validateFilters(ctx, hasID, hasNet || hasZone || noZone, resp) {
		return
	}
	if hasZone && noZone {
		resp.Diagnostics.AddError("Conflicting filters", "zone_id and no_zone are mutually exclusive.")
		return
	}
	if noZone && !hasNet {
		resp.Diagnostics.AddError("Missing filter", "no_zone requires network_id.")
		return
	}
	if hasZone || noZone {
		if err := d.client.RequireAPIVersion(client.MinZonesAPIVersion, "zone filters in ipzilon_subnets"); err != nil {
			resp.Diagnostics.AddError("IPzilon version not supported", err.Error())
			return
		}
	}

	items := []subnetItem{}
	if hasID {
		var s client.Subnet
		if err := d.client.Get(ctx, fmt.Sprintf("/subnets/%d", cfg.ID.ValueInt64()), &s); err != nil {
			resp.Diagnostics.AddError("Get subnet failed", err.Error())
			return
		}
		items = append(items, subnetToItem(s))
	} else {
		var zoneID *int64
		networkID := cfg.NetworkID.ValueInt64()
		if hasZone {
			z := cfg.ZoneID.ValueInt64()
			zoneID = &z
			if !hasNet {
				// The listing hangs from the network: resolve it from the zone.
				var zone client.NetworkZone
				if err := d.client.Get(ctx, fmt.Sprintf("/zones/%d", z), &zone); err != nil {
					resp.Diagnostics.AddError("Get network zone failed", err.Error())
					return
				}
				networkID = zone.NetworkID
			}
		}
		subnets, err := client.GetAll[client.Subnet](ctx, d.client, networkSubnetsURL(networkID, zoneID, noZone))
		if err != nil {
			resp.Diagnostics.AddError("List subnets failed", err.Error())
			return
		}
		for _, s := range subnets {
			items = append(items, subnetToItem(s))
		}
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
