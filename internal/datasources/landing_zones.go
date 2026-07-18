package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ datasource.DataSource = &LandingZonesDataSource{}

type LandingZonesDataSource struct{ client *client.Client }

func NewLandingZonesDataSource() datasource.DataSource { return &LandingZonesDataSource{} }

type landingZonesModel struct {
	ID       types.Int64  `tfsdk:"id"`
	HubID    types.Int64  `tfsdk:"hub_id"`
	ParentID types.Int64  `tfsdk:"parent_id"`
	RootOnly types.Bool   `tfsdk:"root_only"`
	Items    []lzItem     `tfsdk:"items"`
}

type lzItem struct {
	ID          types.Int64  `tfsdk:"id"`
	HubID       types.Int64  `tfsdk:"hub_id"`
	ParentID    types.Int64  `tfsdk:"parent_id"`
	Name        types.String `tfsdk:"name"`
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
}

var lzItemSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id":          schema.Int64Attribute{Computed: true},
		"hub_id":      schema.Int64Attribute{Computed: true},
		"parent_id":   schema.Int64Attribute{Computed: true},
		"name":        schema.StringAttribute{Computed: true, Description: "Landing zone name."},
		"cidr":        schema.StringAttribute{Computed: true, Description: "CIDR block assigned to this landing zone."},
		"description": schema.StringAttribute{Computed: true, Description: "Free-text description."},
	},
}

func (d *LandingZonesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_landing_zones"
}

func (d *LandingZonesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List landing zones. Provide id (singular) OR hub_id with optional parent_id/root_only filter.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.Int64Attribute{Optional: true, Description: "Lookup a single landing zone by ID."},
			"hub_id":    schema.Int64Attribute{Optional: true, Description: "List landing zones for this hub."},
			"parent_id": schema.Int64Attribute{Optional: true, Description: "Filter by parent landing zone ID."},
			"root_only": schema.BoolAttribute{Optional: true, Description: "When true, return only root-level landing zones (parent_id IS NULL)."},
			"items":     schema.ListNestedAttribute{Computed: true, NestedObject: lzItemSchema},
		},
	}
}

func (d *LandingZonesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func lzToItem(lz client.LandingZone) lzItem {
	return lzItem{
		ID:          types.Int64Value(lz.ID),
		HubID:       types.Int64Value(lz.HubID),
		ParentID:    types.Int64PointerValue(lz.ParentID),
		Name:        types.StringValue(lz.Name),
		CIDR:        types.StringPointerValue(lz.CIDR),
		Description: types.StringPointerValue(lz.Description),
	}
}

func (d *LandingZonesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg landingZonesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !cfg.ID.IsNull() && !cfg.ID.IsUnknown()
	hasHub := !cfg.HubID.IsNull() && !cfg.HubID.IsUnknown()

	if !validateFilters(ctx, hasID, hasHub, resp) {
		return
	}

	var items []lzItem
	if hasID {
		var lz client.LandingZone
		if err := d.client.Get(fmt.Sprintf("/landing-zones/%d", cfg.ID.ValueInt64()), &lz); err != nil {
			resp.Diagnostics.AddError("Get landing zone failed", err.Error())
			return
		}
		items = []lzItem{lzToItem(lz)}
	} else {
		url := fmt.Sprintf("/hubs/%d/landing-zones", cfg.HubID.ValueInt64())
		hasParent := !cfg.ParentID.IsNull() && !cfg.ParentID.IsUnknown()
		rootOnly := !cfg.RootOnly.IsNull() && cfg.RootOnly.ValueBool()

		if hasParent && rootOnly {
			resp.Diagnostics.AddError("Conflicting filters", "parent_id and root_only are mutually exclusive.")
			return
		}
		if hasParent {
			url += fmt.Sprintf("?parent_id=%d", cfg.ParentID.ValueInt64())
		} else if rootOnly {
			url += "?root_only=true"
		}

		var lzs []client.LandingZone
		if err := d.client.Get(url, &lzs); err != nil {
			resp.Diagnostics.AddError("List landing zones failed", err.Error())
			return
		}
		for _, lz := range lzs {
			items = append(items, lzToItem(lz))
		}
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
