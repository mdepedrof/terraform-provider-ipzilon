package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ resource.Resource = &NextIPAddressResource{}
var _ resource.ResourceWithImportState = &NextIPAddressResource{}

type NextIPAddressResource struct{ client *client.Client }

func NewNextIPAddressResource() resource.Resource { return &NextIPAddressResource{} }

type nextIPAddressModel struct {
	ID              types.Int64  `tfsdk:"id"`
	SubnetID        types.Int64  `tfsdk:"subnet_id"`
	Hostname        types.String `tfsdk:"hostname"`
	Description     types.String `tfsdk:"description"`
	Address         types.String `tfsdk:"address"`
	Status          types.String `tfsdk:"status"`
	IsAzureReserved types.Bool   `tfsdk:"is_azure_reserved"`
}

// nextIPFromAPI builds the whole state of the resource from the API object. It
// is the only place that fills nextIPAddressModel (Create, Read and Update),
// so no attribute depends on the previous state.
func nextIPFromAPI(ip client.IPAddress) (nextIPAddressModel, error) {
	id, err := ipIDValue(ip.ID)
	if err != nil {
		return nextIPAddressModel{}, err
	}
	return nextIPAddressModel{
		ID:              id,
		SubnetID:        types.Int64Value(ip.SubnetID),
		Hostname:        types.StringPointerValue(ip.Hostname),
		Description:     types.StringPointerValue(ip.Description),
		Address:         types.StringValue(ip.Address),
		Status:          types.StringValue(ip.Status),
		IsAzureReserved: types.BoolValue(ip.IsAzureReserved),
	}, nil
}

// setNextIPState writes the state built from the API object, reporting any
// conversion error as a diagnostic.
func setNextIPState(ctx context.Context, ip client.IPAddress, state *tfsdk.State, diags *diag.Diagnostics) {
	model, err := nextIPFromAPI(ip)
	if err != nil {
		diags.AddError("Read IP failed", err.Error())
		return
	}
	diags.Append(state.Set(ctx, model)...)
}

func (r *NextIPAddressResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_next_ip_address"
}

func (r *NextIPAddressResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Atomically reserves the next available IP in a subnet. The address is assigned by the server.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "IPzilon record ID of the reserved address.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"subnet_id": schema.Int64Attribute{
				Required:    true,
				Description: "Subnet to reserve from.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"hostname":    schema.StringAttribute{Optional: true, Computed: true, Description: "Hostname for this IP — use as the semantic name for the address."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
			"address": schema.StringAttribute{
				Computed:    true,
				Description: "Reserved IP address (computed by server).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "IP status: reserved (default after allocation) or used. To free the address, destroy the resource.",
				Validators:  []validator.String{ipStatusValidator},
			},
			"is_azure_reserved": schema.BoolAttribute{
				Computed:    true,
				Description: "True for IPs automatically reserved by Azure (.1 gateway, .2/.3 DNS, broadcast).",
			},
		},
	}
}

func (r *NextIPAddressResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *NextIPAddressResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nextIPAddressModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var ip client.IPAddress
	if err := r.client.Post(ctx, fmt.Sprintf("/subnets/%d/reserve-ip", plan.SubnetID.ValueInt64()), nil, &ip); err != nil {
		resp.Diagnostics.AddError("Reserve next IP failed", err.Error())
		return
	}
	id, err := ipIDValue(ip.ID)
	if err != nil {
		resp.Diagnostics.AddError("Reserve next IP failed", err.Error())
		return
	}

	// Apply hostname/description/status if provided
	if !plan.Hostname.IsNull() || !plan.Description.IsNull() || !plan.Status.IsNull() {
		if err := r.client.Patch(ctx, fmt.Sprintf("/ips/%d", id.ValueInt64()), client.IPAddressUpdate{
			Hostname:    strPtr(plan.Hostname),
			Description: strPtr(plan.Description),
			Status:      strPtr(plan.Status),
		}, &ip); err != nil {
			resp.Diagnostics.AddError("Set IP metadata failed", err.Error())
			return
		}
	}

	setNextIPState(ctx, ip, &resp.State, &resp.Diagnostics)
}

func (r *NextIPAddressResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nextIPAddressModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var ip client.IPAddress
	if err := r.client.Get(ctx, fmt.Sprintf("/ips/%d", state.ID.ValueInt64()), &ip); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read IP failed", err.Error())
		return
	}
	setNextIPState(ctx, ip, &resp.State, &resp.Diagnostics)
}

func (r *NextIPAddressResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan nextIPAddressModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state nextIPAddressModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var ip client.IPAddress
	if err := r.client.Patch(ctx, fmt.Sprintf("/ips/%d", state.ID.ValueInt64()), client.IPAddressUpdate{
		Hostname:    strPtr(plan.Hostname),
		Description: strPtr(plan.Description),
		Status:      strPtr(plan.Status),
	}, &ip); err != nil {
		resp.Diagnostics.AddError("Update IP failed", err.Error())
		return
	}
	setNextIPState(ctx, ip, &resp.State, &resp.Diagnostics)
}

func (r *NextIPAddressResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state nextIPAddressModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Releases the address: IPzilon 3.0 deletes the stored record and the
	// address becomes free again.
	if err := releaseIP(ctx, r.client, state.ID.ValueInt64()); err != nil {
		resp.Diagnostics.AddError("Release IP failed", err.Error())
	}
}

func (r *NextIPAddressResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
