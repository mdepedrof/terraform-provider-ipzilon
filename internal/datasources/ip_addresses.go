package datasources

import (
	"context"
	"fmt"
	"net/netip"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var (
	_ datasource.DataSource                   = &IPAddressesDataSource{}
	_ datasource.DataSourceWithValidateConfig = &IPAddressesDataSource{}
)

type IPAddressesDataSource struct{ client *client.Client }

func NewIPAddressesDataSource() datasource.DataSource { return &IPAddressesDataSource{} }

type ipAddressesModel struct {
	ID       types.Int64  `tfsdk:"id"`
	SubnetID types.Int64  `tfsdk:"subnet_id"`
	Status   types.String `tfsdk:"status"`
	Address  types.String `tfsdk:"address"`
	Items    []ipAddrItem `tfsdk:"items"`
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
		"id":                schema.Int64Attribute{Computed: true, Description: "IP record ID. Null for free addresses that have no stored record (IPzilon >= 3.0)."},
		"subnet_id":         schema.Int64Attribute{Computed: true, Description: "Subnet containing this address."},
		"address":           schema.StringAttribute{Computed: true, Description: "IP address."},
		"status":            schema.StringAttribute{Computed: true, Description: "IP status: available, used, or reserved."},
		"is_azure_reserved": schema.BoolAttribute{Computed: true, Description: "True for IPs auto-reserved by Azure (.1/.2/.3/broadcast)."},
		"hostname":          schema.StringAttribute{Computed: true, Description: "Hostname — semantic name for the address."},
		"description":       schema.StringAttribute{Computed: true, Description: "Free-text description."},
	},
}

func (d *IPAddressesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_addresses"
}

func (d *IPAddressesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List IP addresses. Provide id (singular) OR subnet_id with an optional status or address filter. Without status or address the whole subnet is listed (e.g. 65,536 items for a /16, fetched in pages of 1000); set status to limit the cost.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.Int64Attribute{Optional: true, Description: "Lookup a single IP by ID."},
			"subnet_id": schema.Int64Attribute{Optional: true, Description: "List IPs for a subnet."},
			"status":    schema.StringAttribute{Optional: true, Description: "Filter by status: available, used, reserved. Without status the whole subnet is listed (e.g. 65,536 items for a /16, fetched in pages of 1000); set status to limit the cost."},
			"address":   schema.StringAttribute{Optional: true, Description: "Look up one address of subnet_id, occupied or free (a free address is returned with id = null and status = available). The address is the stable way to refer to an IP: its id changes when it is released and occupied again. Requires subnet_id; cannot be combined with id or status. Fails if the address is not in the subnet."},
			"items":     schema.ListNestedAttribute{Computed: true, NestedObject: ipAddrItemSchema, Description: "Matching IP addresses; an empty list when nothing matches (a lookup by address returns exactly one)."},
		},
	}
}

func (d *IPAddressesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req, resp)
}

func ipToItem(ip client.IPAddress) ipAddrItem {
	return ipAddrItem{
		ID:              types.Int64PointerValue(ip.ID),
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

	items := []ipAddrItem{}
	if hasID {
		var ip client.IPAddress
		if err := d.client.Get(ctx, fmt.Sprintf("/ips/%d", cfg.ID.ValueInt64()), &ip); err != nil {
			if client.IsNotFound(err) {
				resp.Diagnostics.AddError("IP not found", fmt.Sprintf("IP %d not found. In IPzilon >= 3.0 released addresses have no record and a re-occupied address gets a new id; look it up by subnet_id instead.", cfg.ID.ValueInt64()))
				return
			}
			resp.Diagnostics.AddError("Get IP failed", err.Error())
			return
		}
		items = []ipAddrItem{ipToItem(ip)}
	} else {
		subnetID := cfg.SubnetID.ValueInt64()
		address := stringFilter(cfg.Address)
		ips, err := client.GetAll[client.IPAddress](ctx, d.client, subnetIPsURL(subnetID, stringFilter(cfg.Status), address))
		if err != nil {
			resp.Diagnostics.AddError("List IPs failed", err.Error())
			return
		}
		// IPzilon answers an address outside the subnet (or malformed) with an
		// empty list; a lookup by address must return exactly that address.
		if address != nil && len(ips) == 0 {
			resp.Diagnostics.AddError("IP address not found", fmt.Sprintf("%s is not an address of subnet %d", *address, subnetID))
			return
		}
		for _, ip := range ips {
			items = append(items, ipToItem(ip))
		}
	}

	cfg.Items = items
	resp.Diagnostics.Append(resp.State.Set(ctx, cfg)...)
}

// subnetIPsURL builds the request URL for GET /subnets/{subnet_id}/ips with the
// optional server-side filters status and address (mutually exclusive,
// validated in ValidateConfig).
func subnetIPsURL(subnetID int64, status, address *string) string {
	q := url.Values{}
	if status != nil {
		q.Set("status", *status)
	}
	if address != nil {
		q.Set("address", *address)
	}

	reqURL := fmt.Sprintf("/subnets/%d/ips", subnetID)
	if encoded := q.Encode(); encoded != "" {
		reqURL += "?" + encoded
	}
	return reqURL
}

// ValidateConfig checks the address filter in plan: an IP address, only with
// subnet_id and never with id or status.
func (d *IPAddressesDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var cfg ipAddressesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.Address.IsNull() {
		return
	}

	if !cfg.Address.IsUnknown() {
		if _, err := netip.ParseAddr(cfg.Address.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("address"), "Invalid address", fmt.Sprintf("'%s' is not a valid IP address", cfg.Address.ValueString()))
		}
	}
	if cfg.SubnetID.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("address"), "Missing filter", "address requires subnet_id")
	}
	if !cfg.ID.IsNull() || !cfg.Status.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("address"), "Conflicting filters", "address cannot be combined with id or status")
	}
}
