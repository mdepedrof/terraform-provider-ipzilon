package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ datasource.DataSource = &IPAddressesDataSource{}

type IPAddressesDataSource struct{ client *client.Client }

func NewIPAddressesDataSource() datasource.DataSource { return &IPAddressesDataSource{} }

type ipAddressesModel struct {
	ID       types.Int64   `tfsdk:"id"`
	SubnetID types.Int64   `tfsdk:"subnet_id"`
	Status   types.String  `tfsdk:"status"`
	Items    []ipAddrItem  `tfsdk:"items"`
}

type ipAddrItem struct {
	ID              types.Int64  `tfsdk:"id"`
	SubnetID        types.Int64  `tfsdk:"subnet_id"`
	Address         types.String `tfsdk:"address"`
	Status          types.String `tfsdk:"status"`
	IsAzureReserved types.Bool   `tfsdk:"is_azure_reserved"`
	Hostname        types.String `tfsdk:"hostname"`
	Description     types.String `tfsdk:"description"`
}

var ipAddrItemSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"id":               schema.Int64Attribute{Computed: true},
		"subnet_id":        schema.Int64Attribute{Computed: true},
		"address":          schema.StringAttribute{Computed: true},
		"status":           schema.StringAttribute{Computed: true},
		"is_azure_reserved": schema.BoolAttribute{Computed: true},
		"hostname":         schema.StringAttribute{Computed: true},
		"description":      schema.StringAttribute{Computed: true},
	},
}

func (d *IPAddressesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_addresses"
}

func (d *IPAddressesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List IP addresses. Provide id (singular) OR subnet_id with optional status filter.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.Int64Attribute{Optional: true, Description: "Lookup a single IP by ID."},
			"subnet_id": schema.Int64Attribute{Optional: true, Description: "List IPs for a subnet."},
			"status":    schema.StringAttribute{Optional: true, Description: "Filter by status: available, used, reserved."},
			"items":     schema.ListNestedAttribute{Computed: true, NestedObject: ipAddrItemSchema},
		},
	}
}

func (d *IPAddressesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func ipToItem(ip client.IPAddress) ipAddrItem {
	return ipAddrItem{
		ID:              types.Int64Value(ip.ID),
		SubnetID:        types.Int64Value(ip.SubnetID),
		Address:         types.StringValue(ip.Address),
		Status:          types.StringValue(ip.Status),
		IsAzureReserved: types.BoolValue(ip.IsAzureReserved),
		Hostname:        types.StringPointerValue(ip.Hostname),
		Description:     types.StringPointerValue(ip.Description),
	}
}

func (d *IPAddressesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg ipAddressesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !cfg.ID.IsNull() && !cfg.ID.IsUnknown()
	hasSubnet := !cfg.SubnetID.IsNull() && !cfg.SubnetID.IsUnknown()

	if !validateFilters(ctx, hasID, hasSubnet, resp) {
		return
	}

	var items []ipAddrItem
	if hasID {
		var ip client.IPAddress
		if err := d.client.Get(fmt.Sprintf("/ips/%d", cfg.ID.ValueInt64()), &ip); err != nil {
			resp.Diagnostics.AddError("Get IP failed", err.Error())
			return
		}
		items = []ipAddrItem{ipToItem(ip)}
	} else {
		url := fmt.Sprintf("/subnets/%d/ips", cfg.SubnetID.ValueInt64())
		if !cfg.Status.IsNull() && !cfg.Status.IsUnknown() {
			url += fmt.Sprintf("?status=%s", cfg.Status.ValueString())
		}
		var ips []client.IPAddress
		if err := d.client.Get(url, &ips); err != nil {
			resp.Diagnostics.AddError("List IPs failed", err.Error())
			return
		}
		for _, ip := range ips {
			items = append(items, ipToItem(ip))
		}
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}
