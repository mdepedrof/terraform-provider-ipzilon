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
	Items     []subnetItem `tfsdk:"items"`
}

type subnetItem struct {
	ID          types.Int64  `tfsdk:"id"`
	NetworkID   types.Int64  `tfsdk:"network_id"`
	Name        types.String `tfsdk:"name"`
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
}

var subnetItemSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id":          schema.Int64Attribute{Computed: true},
		"network_id":  schema.Int64Attribute{Computed: true},
		"name":        schema.StringAttribute{Computed: true},
		"cidr":        schema.StringAttribute{Computed: true},
		"description": schema.StringAttribute{Computed: true},
	},
}

func (d *SubnetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subnets"
}

func (d *SubnetsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List subnets. Provide id (singular) OR network_id.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.Int64Attribute{Optional: true, Description: "Lookup a single subnet by ID."},
			"network_id": schema.Int64Attribute{Optional: true, Description: "List all subnets for a network."},
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

	if !validateFilters(ctx, hasID, hasNet, resp) {
		return
	}

	var items []subnetItem
	if hasID {
		var s client.Subnet
		if err := d.client.Get(fmt.Sprintf("/subnets/%d", cfg.ID.ValueInt64()), &s); err != nil {
			resp.Diagnostics.AddError("Get subnet failed", err.Error())
			return
		}
		items = []subnetItem{subnetToItem(s)}
	} else {
		var subnets []client.Subnet
		if err := d.client.Get(fmt.Sprintf("/networks/%d/subnets", cfg.NetworkID.ValueInt64()), &subnets); err != nil {
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
