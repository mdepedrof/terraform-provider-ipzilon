package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ resource.Resource = &NetworkZoneResource{}
var _ resource.ResourceWithImportState = &NetworkZoneResource{}
var _ resource.ResourceWithModifyPlan = &NetworkZoneResource{}

type NetworkZoneResource struct{ client *client.Client }

func NewNetworkZoneResource() resource.Resource { return &NetworkZoneResource{} }

type networkZoneModel struct {
	ID          types.Int64  `tfsdk:"id"`
	NetworkID   types.Int64  `tfsdk:"network_id"`
	Name        types.String `tfsdk:"name"`
	CIDR        types.String `tfsdk:"cidr"`
	Description types.String `tfsdk:"description"`
}

// networkZoneFromAPI builds the whole state from the API object; it is the
// only place that fills networkZoneModel (Create, Read and Update).
func networkZoneFromAPI(z client.NetworkZone) networkZoneModel {
	return networkZoneModel{
		ID:          types.Int64Value(z.ID),
		NetworkID:   types.Int64Value(z.NetworkID),
		Name:        types.StringValue(z.Name),
		CIDR:        types.StringValue(z.CIDR),
		Description: types.StringPointerValue(z.Description),
	}
}

func (r *NetworkZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_zone"
}

func (r *NetworkZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a network zone: a named block inside a network that groups the subnets it contains. Requires IPzilon >= 3.1.0. Destroying a zone does not delete its subnets.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Zone ID.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"network_id": schema.Int64Attribute{
				Required:    true,
				Description: "Network this zone belongs to.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Zone name, unique within its network (must be lowercase — the server normalizes all strings).",
				Validators:  zoneNameValidators,
			},
			"cidr": schema.StringAttribute{
				Required:      true,
				Description:   "Zone CIDR (IPv4, /8 or smaller), inside the network, smaller than it and not overlapping other zones. Changing it is done in place; IPzilon rejects it if a subnet would be left outside or straddle the zone boundary.",
				PlanModifiers: []planmodifier.String{cidrLimits(true)},
			},
			"description": schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
		},
	}
}

func (r *NetworkZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan reports an IPzilon older than 3.1.0 already in plan, also for a
// zone that does not exist yet (Create would only fail in apply).
func (r *NetworkZoneResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if r.client == nil || req.Plan.Raw.IsNull() {
		return
	}
	requireZones(r.client, "ipzilon_network_zone", &resp.Diagnostics)
}

func (r *NetworkZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !requireZones(r.client, "ipzilon_network_zone", &resp.Diagnostics) {
		return
	}
	var plan networkZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var z client.NetworkZone
	if err := r.client.Post(ctx, fmt.Sprintf("/networks/%d/zones", plan.NetworkID.ValueInt64()), client.NetworkZoneCreate{
		Name:        plan.Name.ValueString(),
		CIDR:        plan.CIDR.ValueString(),
		Description: strPtr(plan.Description),
	}, &z); err != nil {
		zoneAPIError(&resp.Diagnostics, "Create network zone failed", "ipzilon_network_zone", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, networkZoneFromAPI(z))...)
}

func (r *NetworkZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !requireZones(r.client, "ipzilon_network_zone", &resp.Diagnostics) {
		return
	}
	var state networkZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var z client.NetworkZone
	if err := r.client.Get(ctx, fmt.Sprintf("/zones/%d", state.ID.ValueInt64()), &z); err != nil {
		if client.IsNotFound(err) && !zoneRouteMissing(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		zoneAPIError(&resp.Diagnostics, "Read network zone failed", "ipzilon_network_zone", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, networkZoneFromAPI(z))...)
}

func (r *NetworkZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !requireZones(r.client, "ipzilon_network_zone", &resp.Diagnostics) {
		return
	}
	var plan networkZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state networkZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	cidr := plan.CIDR.ValueString()
	var z client.NetworkZone
	if err := r.client.Patch(ctx, fmt.Sprintf("/zones/%d", state.ID.ValueInt64()), client.NetworkZoneUpdate{
		Name:        &name,
		CIDR:        &cidr,
		Description: strPtr(plan.Description),
	}, &z); err != nil {
		zoneAPIError(&resp.Diagnostics, "Update network zone failed", "ipzilon_network_zone", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, networkZoneFromAPI(z))...)
}

func (r *NetworkZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !requireZones(r.client, "ipzilon_network_zone", &resp.Diagnostics) {
		return
	}
	var state networkZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, fmt.Sprintf("/zones/%d", state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete network zone failed", err.Error())
	}
}

func (r *NetworkZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
