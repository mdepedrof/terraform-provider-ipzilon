package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ resource.Resource = &IPAddressResource{}
var _ resource.ResourceWithImportState = &IPAddressResource{}

type IPAddressResource struct{ client *client.Client }

func NewIPAddressResource() resource.Resource { return &IPAddressResource{} }

type ipAddressModel struct {
	ID              types.Int64  `tfsdk:"id"`
	SubnetID        types.Int64  `tfsdk:"subnet_id"`
	Address         types.String `tfsdk:"address"`
	Status          types.String `tfsdk:"status"`
	IsAzureReserved types.Bool   `tfsdk:"is_azure_reserved"`
	Hostname        types.String `tfsdk:"hostname"`
	Description     types.String `tfsdk:"description"`
}

func (r *IPAddressResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_address"
}

func (r *IPAddressResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a specific IP address (pre-populated by the subnet). Create marks it as used; destroy releases it back to available.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"subnet_id": schema.Int64Attribute{
				Required:    true,
				Description: "Subnet containing this IP.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"address": schema.StringAttribute{
				Required:    true,
				Description: "IP address to manage (e.g. 10.0.1.5). Must exist in the subnet and be available.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "IP status: available, used, reserved. Defaults to 'used' on create.",
			},
			"is_azure_reserved": schema.BoolAttribute{
				Computed:    true,
				Description: "True for IPs automatically reserved by Azure (.1 gateway, .2/.3 DNS, broadcast).",
			},
			"hostname":    schema.StringAttribute{Optional: true, Computed: true, Description: "Hostname for this IP — use as the semantic name for the address."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
		},
	}
}

func (r *IPAddressResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

func ipFromAPI(ip client.IPAddress) ipAddressModel {
	return ipAddressModel{
		ID:              types.Int64Value(ip.ID),
		SubnetID:        types.Int64Value(ip.SubnetID),
		Address:         types.StringValue(ip.Address),
		Status:          types.StringValue(ip.Status),
		IsAzureReserved: types.BoolValue(ip.IsAzureReserved),
		Hostname:        types.StringPointerValue(ip.Hostname),
		Description:     types.StringPointerValue(ip.Description),
	}
}

func (r *IPAddressResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ipAddressModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Find the pre-existing IP record by exact address within the subnet
	var ips []client.IPAddress
	if err := r.client.Get(
		fmt.Sprintf("/subnets/%d/ips?address=%s", plan.SubnetID.ValueInt64(), plan.Address.ValueString()),
		&ips,
	); err != nil {
		resp.Diagnostics.AddError("Lookup IP failed", err.Error())
		return
	}
	if len(ips) == 0 {
		resp.Diagnostics.AddError("IP not found", fmt.Sprintf("Address %s does not exist in subnet %d.", plan.Address.ValueString(), plan.SubnetID.ValueInt64()))
		return
	}
	ip := ips[0]
	if ip.Status != "available" {
		resp.Diagnostics.AddError("IP not available", fmt.Sprintf("Address %s has status %q (expected available).", ip.Address, ip.Status))
		return
	}

	status := "used"
	if !plan.Status.IsNull() && !plan.Status.IsUnknown() {
		status = plan.Status.ValueString()
	}
	var updated client.IPAddress
	if err := r.client.Patch(fmt.Sprintf("/ips/%d", ip.ID), client.IPAddressUpdate{
		Status:      &status,
		Hostname:    strPtr(plan.Hostname),
		Description: strPtr(plan.Description),
	}, &updated); err != nil {
		resp.Diagnostics.AddError("Mark IP used failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, ipFromAPI(updated))...)
}

func (r *IPAddressResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ipAddressModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var ip client.IPAddress
	if err := r.client.Get(fmt.Sprintf("/ips/%d", state.ID.ValueInt64()), &ip); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read IP failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, ipFromAPI(ip))...)
}

func (r *IPAddressResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ipAddressModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state ipAddressModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	status := plan.Status.ValueString()
	var ip client.IPAddress
	if err := r.client.Patch(fmt.Sprintf("/ips/%d", state.ID.ValueInt64()), client.IPAddressUpdate{
		Status:      &status,
		Hostname:    strPtr(plan.Hostname),
		Description: strPtr(plan.Description),
	}, &ip); err != nil {
		resp.Diagnostics.AddError("Update IP failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, ipFromAPI(ip))...)
}

func (r *IPAddressResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ipAddressModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Release the IP back to the pool (don't delete the record — it was pre-created by populate_all_ips)
	status := "available"
	if err := r.client.Patch(fmt.Sprintf("/ips/%d", state.ID.ValueInt64()), client.IPAddressUpdate{
		Status:      &status,
		Hostname:    nil,
		Description: nil,
	}, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Release IP failed", err.Error())
	}
}

func (r *IPAddressResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
