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
	_ datasource.DataSource                   = &HubsDataSource{}
	_ datasource.DataSourceWithValidateConfig = &HubsDataSource{}
)

type HubsDataSource struct{ client *client.Client }

func NewHubsDataSource() datasource.DataSource { return &HubsDataSource{} }

type hubsModel struct {
	ID           types.Int64  `tfsdk:"id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	AddressSpace types.String `tfsdk:"address_space"`
	Name         types.String `tfsdk:"name"`
	Items        []hubItem    `tfsdk:"items"`
}

type hubItem struct {
	ID           types.Int64  `tfsdk:"id"`
	SiteID       types.Int64  `tfsdk:"site_id"`
	Name         types.String `tfsdk:"name"`
	AddressSpace types.String `tfsdk:"address_space"`
	Location     types.String `tfsdk:"location"`
	Description  types.String `tfsdk:"description"`
}

var hubItemSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id":            schema.Int64Attribute{Computed: true, Description: "Hub ID."},
		"site_id":       schema.Int64Attribute{Computed: true, Description: "ID of the site the hub belongs to."},
		"name":          schema.StringAttribute{Computed: true, Description: "Hub name."},
		"address_space": schema.StringAttribute{Computed: true, Description: "Hub address space CIDR."},
		"location":      schema.StringAttribute{Computed: true, Description: "Free-text location label."},
		"description":   schema.StringAttribute{Computed: true, Description: "Free-text description."},
	},
}

func (d *HubsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hubs"
}

func (d *HubsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List hubs. Provide id (singular lookup) or any combination of site_id, name and address_space. Without site_id the lookup is global (IPzilon >= 3.2.0).",
		Attributes: map[string]schema.Attribute{
			"id":            schema.Int64Attribute{Optional: true, Description: "Lookup a single hub by ID. Cannot be combined with site_id; name and address_space are ignored."},
			"site_id":       schema.Int64Attribute{Optional: true, Description: "Filter: hubs of this site. Optional: without it the lookup is global (IPzilon >= 3.2.0)."},
			"address_space": schema.StringAttribute{Optional: true, Description: "Filter: address_space match (server-side). With site_id it is an exact text match; without site_id it compares the network, so the value must be a CIDR without host bits."},
			"name":          schema.StringAttribute{Optional: true, Description: "Filter: exact name match (server-side)."},
			"items":         schema.ListNestedAttribute{Computed: true, NestedObject: hubItemSchema, Description: "Matching hubs; an empty list when nothing matches."},
		},
	}
}

func (d *HubsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func hubToItem(h client.Hub) hubItem {
	return hubItem{
		ID:           types.Int64Value(h.ID),
		SiteID:       types.Int64Value(h.SiteID),
		Name:         types.StringValue(h.Name),
		AddressSpace: types.StringPointerValue(h.AddressSpace),
		Location:     types.StringPointerValue(h.Location),
		Description:  types.StringPointerValue(h.Description),
	}
}

func (d *HubsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg hubsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !cfg.ID.IsNull() && !cfg.ID.IsUnknown()
	hasSite := !cfg.SiteID.IsNull() && !cfg.SiteID.IsUnknown()

	if hasID && hasSite {
		resp.Diagnostics.AddError("Conflicting filters", "Provide either id or site_id, not both.")
		return
	}

	items := []hubItem{}
	if hasID {
		var h client.Hub
		if err := d.client.Get(ctx, fmt.Sprintf("/hubs/%d", cfg.ID.ValueInt64()), &h); err != nil {
			resp.Diagnostics.AddError("Get hub failed", err.Error())
			return
		}
		items = []hubItem{hubToItem(h)}
	} else if hasSite {
		reqURL := siteHubsURL(cfg.SiteID.ValueInt64(), stringFilter(cfg.AddressSpace), stringFilter(cfg.Name))

		hubs, err := client.GetAll[client.Hub](ctx, d.client, reqURL)
		if err != nil {
			resp.Diagnostics.AddError("List hubs failed", err.Error())
			return
		}
		for _, h := range hubs {
			items = append(items, hubToItem(h))
		}
	} else {
		const feature = "ipzilon_hubs without site_id"
		if !requireGlobalLists(d.client, feature, &resp.Diagnostics) {
			return
		}
		hubs, err := client.GetAll[client.Hub](ctx, d.client, globalHubsURL(stringFilter(cfg.AddressSpace), stringFilter(cfg.Name)))
		if err != nil {
			globalListError(&resp.Diagnostics, "List hubs failed", feature, err)
			return
		}
		for _, h := range hubs {
			items = append(items, hubToItem(h))
		}
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}

// ValidateConfig checks in plan that address_space is a network address when
// the lookup is global (no id and no site_id).
func (d *HubsDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var cfg hubsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.ID.IsNull() && cfg.SiteID.IsNull() {
		validateGlobalCIDR(ctx, req.Config, "address_space", &resp.Diagnostics)
	}
}
