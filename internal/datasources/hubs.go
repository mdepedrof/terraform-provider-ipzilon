package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ datasource.DataSource = &HubsDataSource{}

type HubsDataSource struct{ client *client.Client }

func NewHubsDataSource() datasource.DataSource { return &HubsDataSource{} }

type hubsModel struct {
	ID     types.Int64 `tfsdk:"id"`
	SiteID types.Int64 `tfsdk:"site_id"`
	Items  []hubItem   `tfsdk:"items"`
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
		"id":            schema.Int64Attribute{Computed: true},
		"site_id":       schema.Int64Attribute{Computed: true},
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
		Description: "List hubs. Provide id (singular lookup) OR site_id (list all hubs for a site).",
		Attributes: map[string]schema.Attribute{
			"id":      schema.Int64Attribute{Optional: true, Description: "Lookup a single hub by ID."},
			"site_id": schema.Int64Attribute{Optional: true, Description: "List all hubs in a site."},
			"items":   schema.ListNestedAttribute{Computed: true, NestedObject: hubItemSchema},
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

	if !validateFilters(ctx, hasID, hasSite, resp) {
		return
	}

	var items []hubItem
	if hasID {
		var h client.Hub
		if err := d.client.Get(fmt.Sprintf("/hubs/%d", cfg.ID.ValueInt64()), &h); err != nil {
			resp.Diagnostics.AddError("Get hub failed", err.Error())
			return
		}
		items = []hubItem{hubToItem(h)}
	} else {
		var hubs []client.Hub
		if err := d.client.Get(fmt.Sprintf("/sites/%d/hubs", cfg.SiteID.ValueInt64()), &hubs); err != nil {
			resp.Diagnostics.AddError("List hubs failed", err.Error())
			return
		}
		for _, h := range hubs {
			items = append(items, hubToItem(h))
		}
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
