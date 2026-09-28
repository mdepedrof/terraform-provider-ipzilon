package resources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
		Description: "Occupies a specific IP address in a subnet. Destroy releases it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "IPzilon record ID of the occupied address.",
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
				Required:      true,
				Description:   "IP address to occupy (e.g. 10.0.1.5). Must be inside the subnet and not already in use.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{ipAddressValidator{}},
			},
			"status": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "IP status: used (default) or reserved. To free the address, destroy the resource.",
				Validators:  []validator.String{ipStatusValidator},
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

func ipFromAPI(ip client.IPAddress) (ipAddressModel, error) {
	id, err := ipIDValue(ip.ID)
	if err != nil {
		return ipAddressModel{}, err
	}
	return ipAddressModel{
		ID:              id,
		SubnetID:        types.Int64Value(ip.SubnetID),
		Address:         types.StringValue(ip.Address),
		Status:          types.StringValue(ip.Status),
		IsAzureReserved: types.BoolValue(ip.IsAzureReserved),
		Hostname:        types.StringPointerValue(ip.Hostname),
		Description:     types.StringPointerValue(ip.Description),
	}, nil
}

// registerIP occupies a specific address with POST /subnets/{id}/ips (IPzilon
// >= 3.0: free addresses have no record to PATCH) and translates the API
// errors into user-facing diagnostics.
func registerIP(ctx context.Context, c *client.Client, subnetID int64, body client.IPAddressRegister) (client.IPAddress, diag.Diagnostics) {
	var diags diag.Diagnostics
	var ip client.IPAddress
	err := c.Post(ctx, fmt.Sprintf("/subnets/%d/ips", subnetID), body, &ip)

	var apiErr *client.APIError
	switch {
	case err == nil:
		if ip.ID == nil {
			diags.AddError("Create IP failed", "Unexpected response: IP address without id")
		}
	case client.IsConflict(err):
		diags.AddError("IP address in use", fmt.Sprintf("Address %s is already in use in subnet %d.", body.Address, subnetID))
	case errors.As(err, &apiErr) && apiErr.Code == http.StatusBadRequest && strings.Contains(apiErr.Message, "is not within subnet"):
		diags.AddError("IP address outside subnet", fmt.Sprintf("Address %s is not within subnet %d.", body.Address, subnetID))
	case client.IsNotFound(err):
		diags.AddError("Subnet not found", fmt.Sprintf("Subnet %d does not exist.", subnetID))
	default:
		diags.AddError("Create IP failed", err.Error())
	}
	return ip, diags
}

func (r *IPAddressResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ipAddressModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	status := "used"
	if !plan.Status.IsNull() && !plan.Status.IsUnknown() {
		status = plan.Status.ValueString()
	}
	ip, diags := registerIP(ctx, r.client, plan.SubnetID.ValueInt64(), client.IPAddressRegister{
		Address:     plan.Address.ValueString(),
		Status:      status,
		Hostname:    strPtr(plan.Hostname),
		Description: strPtr(plan.Description),
	})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.setState(ctx, ip, &resp.State, &resp.Diagnostics)
}

// setState stores an API IP record in the resource state.
func (r *IPAddressResource) setState(ctx context.Context, ip client.IPAddress, state *tfsdk.State, diags *diag.Diagnostics) {
	model, err := ipFromAPI(ip)
	if err != nil {
		diags.AddError("Read IP failed", err.Error())
		return
	}
	diags.Append(state.Set(ctx, model)...)
}

func (r *IPAddressResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ipAddressModel
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
	r.setState(ctx, ip, &resp.State, &resp.Diagnostics)
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
	if err := r.client.Patch(ctx, fmt.Sprintf("/ips/%d", state.ID.ValueInt64()), client.IPAddressUpdate{
		Status:      &status,
		Hostname:    strPtr(plan.Hostname),
		Description: strPtr(plan.Description),
	}, &ip); err != nil {
		resp.Diagnostics.AddError("Update IP failed", err.Error())
		return
	}
	r.setState(ctx, ip, &resp.State, &resp.Diagnostics)
}

func (r *IPAddressResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ipAddressModel
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

func (r *IPAddressResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
