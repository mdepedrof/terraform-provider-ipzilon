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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/mdepedrof/terraform-provider-ipzilon/internal/client"
)

var _ resource.Resource = &allocZoneResource{}
var _ resource.ResourceWithImportState = &allocZoneResource{}
var _ resource.ResourceWithModifyPlan = &allocZoneResource{}

// allocZoneResource implements ipzilon_next_network_zone and
// ipzilon_last_network_zone: both reserve an empty zone block atomically in
// IPzilon and only differ in the end of the network they search from.
type allocZoneResource struct {
	client    *client.Client
	direction string // "next" or "last"
}

func NewNextNetworkZoneResource() resource.Resource { return &allocZoneResource{direction: "next"} }

type allocZoneModel struct {
	ID           types.Int64  `tfsdk:"id"`
	NetworkID    types.Int64  `tfsdk:"network_id"`
	PrefixLength types.Int64  `tfsdk:"prefix_length"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	CIDR         types.String `tfsdk:"cidr"`
}

// allocZoneFromAPI builds the whole state from the API object; it is the only
// place that fills allocZoneModel (Create, Read and Update). prefix_length is
// derived from the CIDR the API returns.
func allocZoneFromAPI(z client.NetworkZone) (allocZoneModel, error) {
	prefixLength, err := prefixLengthValue(z.CIDR)
	if err != nil {
		return allocZoneModel{}, err
	}
	return allocZoneModel{
		ID:           types.Int64Value(z.ID),
		NetworkID:    types.Int64Value(z.NetworkID),
		PrefixLength: prefixLength,
		Name:         types.StringValue(z.Name),
		Description:  types.StringPointerValue(z.Description),
		CIDR:         types.StringValue(z.CIDR),
	}, nil
}

func setAllocZoneState(ctx context.Context, z client.NetworkZone, state *tfsdk.State, diags *diag.Diagnostics) {
	model, err := allocZoneFromAPI(z)
	if err != nil {
		diags.AddError("Invalid network zone returned by the API", err.Error())
		return
	}
	diags.Append(state.Set(ctx, model)...)
}

func (r *allocZoneResource) typeName() string {
	return "ipzilon_" + r.direction + "_network_zone"
}

func (r *allocZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.direction + "_network_zone"
}

func (r *allocZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	end := "first"
	if r.direction == "last" {
		end = "last"
	}
	resp.Schema = schema.Schema{
		Description: fmt.Sprintf("Atomically reserves the %s empty block of a given prefix length inside a network as a network zone. The block overlaps no zone and no subnet of the network; the CIDR is assigned by the server. Requires IPzilon >= 3.1.0.", end),
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
				Description: "Network to allocate the zone from.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"prefix_length": schema.Int64Attribute{
				Required:    true,
				Description: "Desired prefix length of the zone, 8 to 32 (e.g. 24 for /24).",
				Validators:  prefixLengthValidators,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Zone name, unique within its network (must be lowercase — the server normalizes all strings).",
				Validators:  zoneNameValidators,
			},
			"description": schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
			"cidr": schema.StringAttribute{
				Computed:    true,
				Description: "Assigned CIDR block (computed by server).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *allocZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan reports an IPzilon older than 3.1.0 already in plan.
func (r *allocZoneResource) ModifyPlan(_ context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if r.client == nil || req.Plan.Raw.IsNull() {
		return
	}
	requireZones(r.client, r.typeName(), &resp.Diagnostics)
}

func (r *allocZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !requireZones(r.client, r.typeName(), &resp.Diagnostics) {
		return
	}
	var plan allocZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var z client.NetworkZone
	if err := r.client.Post(ctx,
		fmt.Sprintf("/networks/%d/%s-available-zone", plan.NetworkID.ValueInt64(), r.direction),
		client.AllocateZoneBody{
			PrefixLength: plan.PrefixLength.ValueInt64(),
			Name:         plan.Name.ValueString(),
			Description:  strPtr(plan.Description),
		}, &z,
	); err != nil {
		if summary, detail, ok := allocationErrorDiag(err, plan.PrefixLength.ValueInt64(), "ipzilon_network_zone"); ok {
			resp.Diagnostics.AddError(summary, detail)
			return
		}
		zoneAPIError(&resp.Diagnostics, fmt.Sprintf("Reserve %s network zone failed", r.direction), r.typeName(), err)
		return
	}
	setAllocZoneState(ctx, z, &resp.State, &resp.Diagnostics)
}

func (r *allocZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !requireZones(r.client, r.typeName(), &resp.Diagnostics) {
		return
	}
	var state allocZoneModel
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
		zoneAPIError(&resp.Diagnostics, "Read network zone failed", r.typeName(), err)
		return
	}
	setAllocZoneState(ctx, z, &resp.State, &resp.Diagnostics)
}

// Update changes only name and description: the CIDR was assigned by IPzilon
// and network_id/prefix_length force a replacement.
func (r *allocZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !requireZones(r.client, r.typeName(), &resp.Diagnostics) {
		return
	}
	var plan allocZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var state allocZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()
	var z client.NetworkZone
	if err := r.client.Patch(ctx, fmt.Sprintf("/zones/%d", state.ID.ValueInt64()), client.NetworkZoneUpdate{
		Name:        &name,
		Description: strPtr(plan.Description),
	}, &z); err != nil {
		zoneAPIError(&resp.Diagnostics, "Update network zone failed", r.typeName(), err)
		return
	}
	setAllocZoneState(ctx, z, &resp.State, &resp.Diagnostics)
}

func (r *allocZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !requireZones(r.client, r.typeName(), &resp.Diagnostics) {
		return
	}
	var state allocZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, fmt.Sprintf("/zones/%d", state.ID.ValueInt64())); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete network zone failed", err.Error())
	}
}

func (r *allocZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByID(ctx, req, resp)
}
